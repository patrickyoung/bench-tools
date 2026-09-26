package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskSpecialistShowsActualDefinitionAndLimitations(t *testing.T) {
	a := taskFixture(t)
	turn := taskSubmit(t, a, "", "Create a note")
	result := turn.Result
	result.Update.Blocked = "Requested illustrations are still missing."
	result.Message = "Your work is ready to review. Tell me what you’d like to change."
	if err := saveTaskJSON(filepath.Join(filepath.Dir(turn.Job.Dir), "result.json"), result); err != nil {
		t.Fatal(err)
	}
	page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
	for _, want := range []string{"ASSIGNED", "Guide", "Created for this conversation", "web research off", "Requested illustrations are still missing.", "Needs attention", "Attach files"} {
		if !strings.Contains(page, want) {
			t.Fatal("missing worker information", want)
		}
	}
	if strings.Contains(page, "Your work is ready to review.") {
		t.Fatal("known missing work hidden by generic success")
	}
	question := taskSubmit(t, a, "", "need question")
	if got := a.taskSpecialist(question.Job, question.Record, question.Result); got.Name != "" {
		t.Fatal("invented assignment before planning")
	}
}

func TestTaskAdaptExistingWorkerRunsHireWithoutChangingOriginal(t *testing.T) {
	a := taskFixture(t)
	first := taskSubmit(t, a, "", "Create a note")
	original, _ := os.ReadFile(filepath.Join(first.Result.Expert, "AGENTS.md"))
	raw, _ := os.ReadFile(a.cfg.Agent)
	from := "target=('previous' if any(c['key']=='previous' for c in r['catalog']) else ('new:team' if 'team' in r['message'] else 'new:worker'))"
	to := "target=('adapt:previous' if any(c['key']=='adapt:previous' for c in r['catalog']) else 'new:worker')"
	if !strings.Contains(string(raw), from) {
		t.Fatal("fixture changed")
	}
	writeFixture(t, a.cfg.Agent, strings.Replace(string(raw), from, to, 1), 0700)
	next := taskSubmit(t, a, first.Record.Thread, "Add illustrations")
	if next.Job.State != "completed" || !strings.Contains(a.jobs.Log(next.Job.ID, "stderr"), "Starting hire") {
		t.Fatal("adaptation did not run public Hire")
	}
	if next.Result.Expert == first.Result.Expert {
		t.Fatal("adapted library original in place")
	}
	after, _ := os.ReadFile(filepath.Join(first.Result.Expert, "AGENTS.md"))
	if string(original) != string(after) {
		t.Fatal("original instructions changed")
	}
	if info := a.taskSpecialist(next.Job, next.Record, next.Result); !strings.Contains(info.Source, "Adapting") {
		t.Fatal("adaptation not explained", info)
	}
}

func TestTaskResearchIsExplicitAndVisible(t *testing.T) {
	a := taskFixture(t)
	raw, _ := os.ReadFile(a.cfg.Agent)
	writeFixture(t, a.cfg.Agent, string(raw)+"\nassert '-net' in sys.argv\n", 0700)
	f := formFor(a)
	f.Set("message", "Research the topic")
	f.Set("research", "yes")
	w := serveTest(a, "POST", "/work", f)
	if w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	thread := strings.TrimPrefix(w.Header().Get("Location"), "/work/")
	turn := a.taskTurns(thread)[0]
	j := awaitJob(t, a.jobs, turn.Job.ID)
	if j.State != "completed" || !turn.Record.Research {
		t.Fatal("explicit research flag not forwarded", a.jobs.Log(j.ID, "stderr"))
	}
	page := serveTest(a, "GET", "/work/"+thread, nil).Body.String()
	if !strings.Contains(page, "Web research enabled") {
		t.Fatal("access hidden")
	}
}

func TestTaskTeamRosterDescribesSelectionNotInventedActivity(t *testing.T) {
	plan := taskPlan{Target: "team:source:design", Message: "Bring together writing and illustration."}
	choice := taskChoice{Key: plan.Target, Name: "Design team", Kind: "team", Members: map[string]member{"writer": {Worker: "editorial-director"}, "illustrator": {Worker: "inkscape-illustrator"}}}
	info := plannedSpecialist(plan, []taskChoice{choice})
	if info.Name != "Design team" || info.Kind != "team" || len(info.Members) != 2 || info.Members[0] != "illustrator: inkscape-illustrator" {
		t.Fatal(info)
	}
}
