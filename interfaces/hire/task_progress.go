package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Optional worker communication is display-only. It never changes the observed
// command outcome, checks, selected permissions or continuation eligibility.
type taskUpdate struct {
	Done    string `json:"done"`
	Now     string `json:"now"`
	Next    string `json:"next"`
	Blocked string `json:"blocked"`
}

const taskCommunication = `
Keep the user briefly informed while you work. At the start and after meaningful progress, atomically replace hire-status.json in the work folder with a JSON object containing exactly four short plain-text strings: done, now, next, blocked. Each is at most 240 characters (empty if none). Say what you actually finished, what you are doing, what remains or has not started, and a concrete blocker if any. Use everyday language and task-specific details, not tool names, paths, logs or reasoning. A draft is not a checked final result. Update again before stopping. This is a small status note, not a deliverable or an acceptance check; spend your effort doing the work.
`

func readWorkerUpdate(work string) (taskUpdate, error) {
	var update taskUpdate
	// readText refuses symlinks and caps reads, including partially written data.
	raw, err := readText(work, "hire-status.json", 2048)
	if err != nil {
		return update, err
	}
	if err = strictJSON([]byte(raw), &update); err != nil {
		return taskUpdate{}, err
	}
	var fields map[string]any
	if json.Unmarshal([]byte(raw), &fields) != nil || len(fields) != 4 {
		return taskUpdate{}, fmt.Errorf("incomplete worker update")
	}
	for _, v := range fields {
		if _, ok := v.(string); !ok {
			return taskUpdate{}, fmt.Errorf("invalid worker update field")
		}
	}
	for _, text := range []string{update.Done, update.Now, update.Next, update.Blocked} {
		if !boundedText(text, 240, false) || strings.ContainsAny(text, "\n\r") {
			return taskUpdate{}, fmt.Errorf("invalid worker update")
		}
	}
	return update, nil
}

func taskPhase(root, message string) {
	_ = os.WriteFile(filepath.Join(root, "progress.txt"), []byte(message), 0600)
}

func (a *app) taskUpdate(j Job, rec taskRecord, result taskResult) taskUpdate {
	if !j.Active() {
		update := result.Update // Immutable copy for this attempt, never live state.
		if update.Done == "" && len(result.Artifacts) > 0 {
			update.Done = "Your available files are saved below."
		}
		if update.Next == "" && j.State != "completed" {
			update.Next = "Finish the remaining work and check the result."
		}
		return update
	}
	root := filepath.Dir(j.Dir)
	rel, _ := filepath.Rel(a.cfg.Data, filepath.Join(root, "progress.txt"))
	phase, _ := readText(a.cfg.Data, rel, 2048)
	update := taskUpdate{Now: phase, Next: "Prepare the right help, then do the work."}
	if phase == "" {
		update.Now = "Understanding your request…"
	} else if strings.Contains(phase, "prepar") || strings.Contains(phase, "Prepar") || phase == "Putting the right expertise together…" {
		update.Done, update.Next = "The work is planned.", "Make your requested result, then check it."
		planned := root
		if rec.Resume != nil {
			planned = rec.Resume.Root
		}
		if text, err := readText(planned, "plan-summary.txt", 2048); err == nil && strings.TrimSpace(text) != "" {
			update.Next = text
		}
	} else if phase == "Working on your request…" {
		update.Done, update.Next = "Your specialist is ready.", "Finish the work, then check it."
		runtime := root
		if rec.Resume != nil {
			runtime = rec.Resume.Root
		}
		work := filepath.Join(runtime, "work", "execution")
		// A previous attempt's note must not appear to be a new live update.
		if info, err := os.Lstat(filepath.Join(work, "hire-status.json")); err == nil && !info.ModTime().Before(j.Started) {
			if reported, err := readWorkerUpdate(work); err == nil && reported != (taskUpdate{}) {
				return reported
			}
		}
	} else if phase == "Checking and gathering your work…" {
		update.Done, update.Next = "The worker has returned its work.", "Show what’s ready and anything still unfinished."
	}
	return update
}
