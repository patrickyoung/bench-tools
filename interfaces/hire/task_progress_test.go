package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskProgressUsesLiveUpdatesAndFrozenResults(t *testing.T) {
	a := taskFixture(t)
	turn := taskSubmit(t, a, "", "Write a note")
	if turn.Result.Update.Done != "Wrote the document." || len(turn.Result.Artifacts) != 2 {
		t.Fatal("worker update not captured or exposed as a deliverable")
	}
	work := filepath.Join(turn.Job.Dir, "execution")
	update := taskUpdate{Done: "Drafted the slides.", Now: "Making previews.", Next: "Check the images; PDF not started.", Blocked: "<script>Need the event date</script>"}
	b, _ := json.Marshal(update)
	writeFixture(t, filepath.Join(work, "hire-status.json"), string(b), 0600)
	if got := a.taskUpdate(turn.Job, turn.Record, turn.Result); got.Done != "Wrote the document." {
		t.Fatal("later workspace edits changed an earlier result")
	}
	a.jobs.mu.Lock()
	j := a.jobs.jobs[turn.Job.ID]
	j.State, j.ExitCode = "running", nil
	a.jobs.jobs[j.ID] = j
	a.jobs.mu.Unlock()
	taskPhase(filepath.Dir(j.Dir), "Working on your request…")
	w := serveTest(a, "GET", "/work/jobs/"+j.ID+"/status", nil)
	var status struct {
		Active bool
		Update taskUpdate
	}
	if json.Unmarshal(w.Body.Bytes(), &status) != nil || !status.Active || status.Update != update {
		t.Fatalf("live update missing: %s", w.Body.String())
	}
	page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
	if !strings.Contains(page, "PDF not started") || strings.Contains(page, "<script>Need") || !strings.Contains(page, "&lt;script&gt;Need") {
		t.Fatal("worker update lost or not escaped")
	}
	old := j.Started.Add(-time.Second)
	if err := os.Chtimes(filepath.Join(work, "hire-status.json"), old, old); err != nil {
		t.Fatal(err)
	}
	if got := a.taskUpdate(j, turn.Record, turn.Result); got.Now != "Working on your request…" {
		t.Fatal("prior attempt's update appeared live")
	}
	// Restore observed terminal state before fixture cleanup.
	a.jobs.mu.Lock()
	a.jobs.jobs[j.ID] = turn.Job
	a.jobs.mu.Unlock()
}

func TestTaskProgressRejectsInvalidUpdates(t *testing.T) {
	work := t.TempDir()
	for _, raw := range []string{`{`, `{"done": "ok", "command": "execute me"}`, `{"now":"` + strings.Repeat("x", 241) + `"}`, `{"next":"first\nsecond"}`} {
		writeFixture(t, filepath.Join(work, "hire-status.json"), raw, 0600)
		if _, err := readWorkerUpdate(work); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if err := os.Remove(filepath.Join(work, "hire-status.json")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "status.json")
	writeFixture(t, outside, `{"done":"secret"}`, 0600)
	if err := os.Symlink(outside, filepath.Join(work, "hire-status.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := readWorkerUpdate(work); err == nil {
		t.Fatal("followed status symlink")
	}
}
