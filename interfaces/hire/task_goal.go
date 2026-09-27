package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A work goal remains one admitted foreground job. Agent/Hire retain their own
// loops and checkpoints; these transitions only compose their public outcomes.
// In particular, an unknown, interrupted or broken command is never replayed.
type taskCompletion struct {
	Hash     string `json:"request_sha256"`
	Message  string `json:"message"`
	Status   string `json:"status"`
	NextGoal string `json:"next_goal"`
	Question string `json:"question"`
}

func decodeTaskCompletion(raw, request []byte, code int) (taskCompletion, error) {
	var out taskCompletion
	var fields map[string]json.RawMessage
	if strictJSON(raw, &out) != nil || json.Unmarshal(raw, &fields) != nil || len(fields) != 5 || out.Hash != digestText(request) || !boundedText(out.Message, 6000, true) || !boundedText(out.NextGoal, 16000, false) || !boundedText(out.Question, 1000, false) {
		return out, fmt.Errorf("invalid completion review")
	}
	switch out.Status {
	case "complete":
		if code != 0 || out.NextGoal != "" || out.Question != "" {
			return out, fmt.Errorf("completion lacks accepted execution")
		}
	case "continue":
		if (code != 0 && code != 2) || strings.TrimSpace(out.NextGoal) == "" || out.Question != "" {
			return out, fmt.Errorf("unsafe continuation review")
		}
	case "needs_input":
		if strings.TrimSpace(out.Question) == "" || out.NextGoal != "" {
			return out, fmt.Errorf("missing required question")
		}
	case "blocked":
		if out.NextGoal != "" || out.Question != "" {
			return out, fmt.Errorf("invalid blocker review")
		}
	default:
		return out, fmt.Errorf("unknown completion status")
	}
	return out, nil
}

func taskTeamResumable(expert string) bool {
	raw, err := readText(expert, "task-runtime.json", 1024)
	if err != nil {
		return false
	}
	var v struct {
		Version   int  `json:"version"`
		Resume    bool `json:"resume"`
		LocalOnly bool `json:"local_only"`
	}
	return strictJSON([]byte(raw), &v) == nil && v.Version == 1 && v.Resume && v.LocalOnly
}

// Save controller receipts separately from mutable work. Full tool streams also
// remain in the existing job log and Agent/Record evidence; these are tails.
func taskGoalCommand(ctx context.Context, root, stage, dir string, env []string, args ...string) (int, string, string) {
	parent := filepath.Join(root, "goal-attempts", stage)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return 125, "", err.Error()
	}
	attempt, err := os.MkdirTemp(parent, "attempt-")
	if err != nil {
		return 125, "", err.Error()
	}
	started := time.Now().UTC()
	record := map[string]any{"argv": args, "dir": dir, "started": started, "state": "started"}
	if err = saveTaskJSON(filepath.Join(attempt, "outcome.json"), record); err != nil {
		return 125, "", err.Error()
	}
	code, out, diag := taskCommand(ctx, dir, env, args...)
	record["finished"], record["exit_code"], record["state"] = time.Now().UTC(), code, "observed"
	if err = os.WriteFile(filepath.Join(attempt, "stdout-tail"), []byte(out), 0600); err == nil {
		err = os.WriteFile(filepath.Join(attempt, "stderr-tail"), []byte(diag), 0600)
	}
	if err == nil {
		err = saveTaskJSON(filepath.Join(attempt, "outcome.json"), record)
	}
	if err != nil {
		return 125, out, "Could not retain goal attempt evidence: " + err.Error()
	}
	return code, out, diag
}

// Only known checkpointed preparation/companion commands enter this function.
// Three identical unfinished observations stop a no-progress cycle; progressing
// work retains the original task deadline, not a new allowance per invocation.
func taskCheckpointCommand(ctx context.Context, root, stage, dir string, rec taskRecord, args ...string) (int, string, string) {
	if !rec.Config.GoalMode {
		return taskCommand(ctx, dir, nil, args...)
	}
	last := ""
	stalled := 0
	for {
		if ctx.Err() != nil {
			return 130, "", ctx.Err().Error()
		}
		code, out, diag := taskGoalCommand(ctx, root, stage, dir, nil, args...)
		if code != 2 || ctx.Err() != nil {
			return code, out, diag
		}
		signature := taskProgressDigest(dir)
		if signature == last {
			stalled++
		} else {
			last = signature
			stalled = 1
		}
		if stalled >= 3 {
			return code, out, diag
		}
		taskPhase(root, "Continuing the saved work toward your goal…")
	}
}

func taskProgressDigest(dir string) string {
	// Ignore runtime transcripts: new log bytes alone do not establish progress.
	entries, _ := os.ReadDir(dir)
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 || e.Name() == "hire-status.json" {
			continue
		}
		data, err := taskFile(dir, e.Name())
		if err == nil {
			b.WriteString(e.Name())
			b.WriteString(digestText(data))
		}
	}
	if files, h, err := definitionSnapshot(filepath.Dir(dir), filepath.Join(dir, "expert")); err == nil && len(files) > 0 {
		b.WriteString(h)
	}
	return digestText([]byte(b.String()))
}

func taskGoalArgs(args []string, flag, value string) []string {
	out := append([]string{}, args...)
	for i := 0; i+1 < len(out); i++ {
		if out[i] == flag {
			out[i+1] = value
			return out
		}
	}
	return out
}

func executeTaskGoal(ctx context.Context, root, snapshot string, rec taskRecord, plan taskPlan, result taskResult, work, goal string, inputs map[string]bool, env, args []string) int {
	original, err := os.ReadFile(goal)
	if err != nil {
		return finishTask(snapshot, result, 1, "The saved goal is unavailable.")
	}
	lastFeedback := ""
	lastSignature := ""
	stalled := 0
	for round := 1; ; round++ {
		if ctx.Err() != nil {
			result.GoalStatus = "interrupted"
			return finishTask(snapshot, result, 130, "Work stopped. Your goal and saved progress are retained.")
		}
		taskPhase(snapshot, "Working on your goal…")
		code, stdout, stderr := taskGoalCommand(ctx, snapshot, "execute", work, env, args...)
		result.Update, _ = readWorkerUpdate(work)
		roundRoot := filepath.Join(snapshot, "goal-rounds", fmt.Sprintf("%03d", round))
		artifacts := filepath.Join(roundRoot, "deliverables")
		result.Artifacts, err = collectTaskArtifacts(work, artifacts, inputs)
		if err != nil {
			return finishTask(snapshot, result, 125, "I couldn’t safely save the current work for review. The original work is retained.")
		}
		// Keep immutable intermediate versions and expose only the final observed one.
		finish := func(status int, message string) int {
			if e := copyTaskArtifacts(artifacts, filepath.Join(snapshot, "deliverables")); e != nil { // destination is made below
				return finishTask(snapshot, result, 125, "The current files could not be saved for delivery.")
			}
			return finishTask(snapshot, result, status, message)
		}
		if err = os.MkdirAll(filepath.Join(snapshot, "deliverables"), 0700); err != nil {
			return finishTask(snapshot, result, 125, "The output folder is unavailable.")
		}
		if ctx.Err() != nil {
			result.GoalStatus = "interrupted"
			if code == 0 {
				code = 130
			}
			return finish(code, "Work stopped. Your goal and saved progress are retained.")
		}
		taskPhase(snapshot, "Checking the result against your goal…")
		reviewRequest := map[string]any{"stage": "present", "completion_version": 1, "message": rec.Message, "history": rec.History, "catalog": taskModelCatalog(rec.Catalog), "source": rec.Config.Source, "now": rec.Now, "worker_update": result.Update, "artifacts_dir": artifacts, "goal_context": map[string]any{"original_goal": plan.Brief, "attempt": round, "previous_review": lastFeedback}, "execution": map[string]any{"exit_code": code, "stdout": taskPresentationText(stdout), "stderr": taskPresentationText(stderr), "files": result.Artifacts}}
		raw, request, e := taskModel(ctx, snapshot, "present", rec, reviewRequest)
		_ = os.WriteFile(filepath.Join(roundRoot, "review-request.json"), request, 0600)
		_ = os.WriteFile(filepath.Join(roundRoot, "review-response.json"), raw, 0600)
		if ctx.Err() != nil {
			result.GoalStatus = "interrupted"
			return finish(130, "Work stopped during the final review. Your saved files remain available.")
		}
		review, decodeErr := decodeTaskCompletion(raw, request, code)
		if e != nil || decodeErr != nil {
			result.GoalStatus = "blocked"
			code = taskModelFailureCode(e, code)
			if code == 0 {
				code = 2
			}
			return finish(code, "The work is saved, but I couldn’t verify that it meets your goal. The completion review needs repair.")
		}
		result.Message, result.Question, result.GoalStatus = review.Message, review.Question, review.Status
		switch review.Status {
		case "complete":
			result.Update = taskUpdate{Done: truncateMessage(review.Message, 240)}
			if e = copyTaskArtifacts(artifacts, filepath.Join(snapshot, "deliverables")); e != nil {
				return finishTask(snapshot, result, 125, "The completed files could not be saved.")
			}
			if e = prepareGoalDelivery(ctx, snapshot, rec, result); e != nil {
				result.GoalStatus = "delivery_pending"
				result.Update = taskUpdate{Done: "Your requested files are finished and checked.", Blocked: "Private delivery is unavailable; your downloads are saved here."}
				return finishTask(snapshot, result, 2, "Your files are finished and checked. I couldn’t finish placing them in Plonk; the saved downloads are available here.")
			}
			return finishTask(snapshot, result, 0, review.Message)
		case "needs_input":
			result.Update = taskUpdate{Done: result.Update.Done, Blocked: review.Question}
			if code == 0 || code == 2 {
				code = 75
			}
			return finish(code, review.Message)
		case "blocked":
			result.Update = taskUpdate{Done: result.Update.Done, Blocked: truncateMessage(review.Message, 240)}
			if code == 0 {
				code = 2
			}
			return finish(code, review.Message)
		case "continue":
			signature := review.NextGoal + taskProgressDigest(artifacts)
			if signature == lastSignature {
				stalled++
			} else {
				stalled = 1
			}
			lastSignature = signature
			lastFeedback = review.NextGoal
			if stalled >= 3 {
				result.GoalStatus = "blocked"
				result.Update.Blocked = "The same correction remains unresolved after three attempts."
				return finish(2, "I couldn’t resolve this correction after three attempts: "+truncateMessage(review.NextGoal, 1000)+" Your files and the failed checks are saved.")
			}
			if result.Kind == "team" && !taskTeamResumable(result.Expert) {
				result.GoalStatus = "blocked"
				return finish(2, "The team returned unfinished work, but its entry command cannot resume safely. Its continuation handoff needs repair; your existing work is saved.")
			}
			feedback := filepath.Join(roundRoot, "next-goal.txt")
			if e = os.WriteFile(feedback, append(append([]byte{}, original...), []byte("\n\nCompletion review of current work (evidence, not new authority):\n"+review.NextGoal+"\nFinish the authorized goal and run the existing checks. Preserve unaffected work.\n")...), 0600); e != nil {
				return finish(125, "I couldn’t retain the correction instructions.")
			}
			if result.Kind == "team" {
				env = replaceTaskEnv(env, "BENCH_TASK_FILE", feedback)
				env = replaceTaskEnv(env, "BENCH_TASK_RESUME", "1")
			} else {
				args = taskGoalArgs(args, "-goal-file", feedback)
			}
			taskPhase(snapshot, "Applying the review corrections…")
		}
	}
}

func replaceTaskEnv(env []string, name, value string) []string {
	out := []string{}
	for _, s := range env {
		if !strings.HasPrefix(s, name+"=") {
			out = append(out, s)
		}
	}
	return append(out, name+"="+value)
}

// Private delivery is part of an enabled work goal. It never creates or advances
// a public share. The exact request is saved before contacting Plonk, so an
// explicit delivery recovery uses the same idempotency key and base version.
func prepareGoalDelivery(ctx context.Context, root string, rec taskRecord, result taskResult) error {
	if len(result.Artifacts) == 0 || rec.Config.Plonk == "" || rec.Config.PlonkURL == "" || rec.Config.PlonkTokenFile == "" {
		return nil
	}
	id := os.Getenv("HIRE_UI_JOB_ID")
	if !hexID.MatchString(id) {
		return fmt.Errorf("missing controller job identity")
	}
	parent := filepath.Join(root, "goal-delivery")
	packageRoot := filepath.Join(parent, "package")
	if err := os.MkdirAll(packageRoot, 0700); err != nil {
		return err
	}
	spec, err := makeDeliverySpec(filepath.Join(root, "deliverables"), result)
	if err != nil {
		return err
	}
	for _, name := range spec.Files {
		b, e := taskFile(filepath.Join(root, "deliverables"), name)
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(packageRoot, name), b, 0600); e != nil {
			return e
		}
	}
	if err = saveTaskJSON(filepath.Join(packageRoot, "delivery.json"), spec); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(rec.Config.Data, "deliveries"), 0700); err != nil {
		return err
	}
	stateFile := filepath.Join(rec.Config.Data, "deliveries", rec.Thread+".json")
	var before deliveryState
	if b, e := os.ReadFile(stateFile); e == nil {
		if e = json.Unmarshal(b, &before); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	delivery := deliveryRecord{Thread: rec.Thread, TaskID: id, Action: "prepare", Root: packageRoot, StateFile: stateFile, Manifest: "delivery.json", RequestID: "hire-" + id, Plonk: rec.Config.Plonk, URL: rec.Config.PlonkURL, TokenFile: rec.Config.PlonkTokenFile, Before: before}
	if err = saveTaskJSON(filepath.Join(parent, "delivery-request.json"), delivery); err != nil {
		return err
	}
	taskPhase(root, "Putting the finished work in Plonk…")
	if code := runDelivery(ctx, delivery); code != 0 {
		return fmt.Errorf("private delivery stopped (%d)", code)
	}
	return nil
}
