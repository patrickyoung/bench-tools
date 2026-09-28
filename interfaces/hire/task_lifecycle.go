package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const taskRunLimit = 45 * time.Minute

// Retain the controller's stop cause separately from the child's exact status.
// A deadline is not a user cancellation, and a saved draft is not completion.
func retainTaskStop(root string, ctx context.Context, code int) {
	if ctx.Err() == nil || code == 0 {
		return
	}
	var result taskResult
	b, err := os.ReadFile(filepath.Join(root, "result.json"))
	if err == nil {
		_ = json.Unmarshal(b, &result)
	}
	result.Code = code
	deliveryPending := result.GoalStatus == "delivery_pending"
	result.GoalStatus = "interrupted"
	result.StopReason = "The run was interrupted by a stop request or service shutdown."
	if ctx.Err() == context.DeadlineExceeded {
		result.GoalStatus = "timed_out"
		result.StopReason = "The run reached its 45-minute time limit."
	}
	if deliveryPending {
		result.GoalStatus = "delivery_pending"
		_ = saveTaskJSON(filepath.Join(root, "result.json"), result)
		return
	}
	phase, _ := readText(root, "progress.txt", 2048)
	result.Update.Now = strings.TrimSpace(phase)
	if result.Update.Next == "" {
		paths, _ := filepath.Glob(filepath.Join(root, "goal-rounds", "*", "review-response.json"))
		sort.Strings(paths)
		if len(paths) > 0 {
			raw, e := os.ReadFile(paths[len(paths)-1])
			var review taskCompletion
			if e == nil && json.Unmarshal(raw, &review) == nil {
				result.Update.Next = truncateMessage(review.NextGoal, 1000)
			}
		}
	}
	result.Message = "The requested result is not finished. The team, working files and attempt records are retained."
	_ = saveTaskJSON(filepath.Join(root, "result.json"), result)
}
