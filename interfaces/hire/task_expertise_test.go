package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExpertiseNoticeDistinguishesReuseAdaptationAndCreation(t *testing.T) {
	p, cat := selectionTestPlan()
	if note := plannedExpertise(p, cat); note.Title != "" {
		t.Fatal("unchanged worker unnecessarily announced as new", note)
	}
	p.Target = "new:team"
	p.Selection.Gap = "We need original drawings and a safety review for this guide."
	p.Selection.Roles = []taskMember{{"slides", cat[0].Key, "Make slides."}, {"art", "adapt:" + cat[1].Key, "Adapt artwork."}, {"safety-review", "new:worker", "Review safety."}}
	note := plannedExpertise(p, cat)
	if note.Why != p.Selection.Gap || len(note.Members) != 3 {
		t.Fatal(note)
	}
	for i, want := range []string{"Using existing", "Adapting existing", "Creating new"} {
		if note.Members[i].Mode != want {
			t.Fatal(note)
		}
	}
	p.Question = "What is the audience?"
	if plannedExpertise(p, cat).Title != "" {
		t.Fatal("question announced as staffed")
	}
}

func TestExpertiseGuidanceReachesHireDuringPreparation(t *testing.T) {
	a := taskFixture(t)
	original, _ := os.ReadFile(a.cfg.Hire)
	script := `#!/bin/sh
set -eu
python3 - "$@" <<'PY'
import json,sys,time
from pathlib import Path
steer=Path(sys.argv[sys.argv.index('-steer')+1])
Path('ready-for-guidance').write_text('ready')
for _ in range(250):
 if steer.read_text().strip():break
 time.sleep(.02)
else:sys.exit(1)
notes=[json.loads(line)['message'] for line in steer.read_text().splitlines()]
Path('received-guidance.txt').write_text('\n'.join(notes))
PY
` + strings.TrimPrefix(string(original), "#!/bin/sh\n")
	writeFixture(t, a.cfg.Hire, script, 0700)
	f := formFor(a)
	f.Set("message", "Create a team for my guide")
	w := serveTest(a, "POST", "/work", f)
	if w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	thread := strings.TrimPrefix(w.Header().Get("Location"), "/work/")
	turn := a.taskTurns(thread)[0]
	root := filepath.Dir(turn.Job.Dir)
	ready := filepath.Join(root, "authoring", "ready-for-guidance")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatal("preparation never reached guidance", a.jobs.Log(turn.Job.ID, "stderr"))
	}
	page := serveTest(a, "GET", "/work/"+thread, nil).Body.String()
	for _, want := range []string{"Meet Cosmic Otter Crew", "Creating new", "Fixture missing capability.", "Add a preference", "Your input is optional.", "I’ll keep going with this recommendation."} {
		if !strings.Contains(page, want) {
			t.Fatal("notice missing", want)
		}
	}
	status := serveTest(a, "GET", "/work/jobs/"+turn.Job.ID+"/status", nil)
	var data struct {
		Specialist taskSpecialist `json:"specialist"`
	}
	if err := json.Unmarshal(status.Body.Bytes(), &data); err != nil || data.Specialist.Expertise.Title == "" {
		t.Fatal("live notice missing", err)
	}
	f = formFor(a)
	f.Set("message", "Keep it beginner-friendly and focus on clear drawings.")
	if w := serveTest(a, "POST", "/work/jobs/"+turn.Job.ID+"/message", f); w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	j := awaitJob(t, a.jobs, turn.Job.ID)
	got, _ := os.ReadFile(filepath.Join(root, "authoring", "received-guidance.txt"))
	if j.State != "completed" || string(got) != f.Get("message") {
		t.Fatal("builder missed optional guidance", j.State, string(got), a.jobs.Log(j.ID, "stderr"))
	}
	if len(a.jobs.List()) != 1 {
		t.Fatal("guidance started another job")
	}
	page = serveTest(a, "GET", "/work/"+thread, nil).Body.String()
	if !strings.Contains(page, "This was the recommendation for this attempt.") {
		t.Fatal("historical notice implies active preparation")
	}
}

func TestExpertiseNoticeEscapesModelText(t *testing.T) {
	a := taskFixture(t)
	turn := taskSubmit(t, a, "", "Make a document") // No optional preference is required.
	if turn.Job.State != "completed" {
		t.Fatal("optional input blocked work")
	}
	info := taskSpecialist{Name: "Specialist", Expertise: taskExpertiseNote{Title: "<script>unsafe</script>", Why: "<img src=x onerror=bad>", Members: []taskExpertiseMember{{"<b>Member</b>", "Creating new"}}}}
	if err := saveTaskJSON(filepath.Join(filepath.Dir(turn.Job.Dir), "specialist.json"), info); err != nil {
		t.Fatal(err)
	}
	page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
	if strings.Contains(page, "<script>unsafe") || strings.Contains(page, "<img src=x") || !strings.Contains(page, "&lt;b&gt;Member&lt;/b&gt;") {
		t.Fatal("unescaped staffing text")
	}
}
