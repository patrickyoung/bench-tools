package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskStopCausePreservesExactOutcomeAndRemainingWork(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		root := t.TempDir()
		result := taskResult{Kind: "team", Prepared: true, Code: 137, GoalStatus: "interrupted"}
		if err := saveTaskJSON(filepath.Join(root, "result.json"), result); err != nil {
			t.Fatal(err)
		}
		taskPhase(root, "Checking the result against your goal…")
		writeFixture(t, filepath.Join(root, "goal-rounds/003/review-response.json"), `{"next_goal":"Finish the independent review."}`, 0600)
		ctx, cancel := context.WithCancel(context.Background())
		if timeout {
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		} else {
			cancel()
		}
		defer cancel()
		retainTaskStop(root, ctx, 137)
		raw, _ := os.ReadFile(filepath.Join(root, "result.json"))
		var got taskResult
		if json.Unmarshal(raw, &got) != nil || got.Code != 137 || got.Update.Next != "Finish the independent review." || !strings.Contains(got.Update.Now, "Checking") {
			t.Fatalf("lost outcome: %+v", got)
		}
		if timeout && (got.GoalStatus != "timed_out" || !strings.Contains(got.StopReason, "45-minute")) {
			t.Fatal(got)
		}
		if !timeout && (got.GoalStatus != "interrupted" || strings.Contains(got.StopReason, "time limit")) {
			t.Fatal(got)
		}
	}
}

func TestTaskBlockedRecoverySelectsRuntimeEvidenceWithoutReplay(t *testing.T) {
	a := goalFixture(t)
	appendGoalWorker(t, a, "Path('result.md').write_text('Partial draft')\nsys.exit(2)\n")
	first := taskSubmit(t, a, "", "Make a document")
	page := serveTest(a, "GET", "/work/"+first.Record.Thread, nil).Body.String()
	if !strings.Contains(page, "Resolve the blocker") || !strings.Contains(page, "See why this run stopped") {
		t.Fatal("missing stop explanation or recovery")
	}
	form := formFor(a)
	form.Set("thread", first.Record.Thread)
	form.Set("retry-job", first.Job.ID)
	form.Set("message", "Inspect and resolve this blocker")
	response := serveTest(a, "POST", "/work", form)
	if response.Code != 303 {
		t.Fatal(response.Code)
	}
	turns := a.taskTurns(first.Record.Thread)
	next := turns[len(turns)-1]
	awaitJob(t, a.jobs, next.Job.ID)
	if next.Record.Resume != nil || next.Record.RecoveryRoot != filepath.Dir(first.Job.Dir) {
		t.Fatal("recovery lost saved runtime or replayed checkpoint")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(next.Job.Dir), "plan/request.json"))
	if err != nil || !strings.Contains(string(raw), `"stopped_work"`) || !strings.Contains(string(raw), "read-only diagnosis") {
		t.Fatalf("missing evidence selection: %v", err)
	}
}

func TestTaskDeadlineDuringDeliveryDoesNotReopenFinishedWork(t *testing.T) {
	root := t.TempDir()
	original := taskResult{GoalStatus: "delivery_pending", Message: "Your files are finished and checked.", Code: 2}
	if err := saveTaskJSON(filepath.Join(root, "result.json"), original); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	retainTaskStop(root, ctx, 2)
	raw, _ := os.ReadFile(filepath.Join(root, "result.json"))
	var got taskResult
	if json.Unmarshal(raw, &got) != nil || got.GoalStatus != "delivery_pending" || got.Message != original.Message || !strings.Contains(got.StopReason, "45-minute") {
		t.Fatal(got)
	}
}
