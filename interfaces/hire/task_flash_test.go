package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlashTeamStaysInConversationAndKeepsIdentity(t *testing.T) {
	a := taskFixture(t)
	first := taskSubmit(t, a, "", "Create a team document")
	if first.Result.Flash == nil || first.Result.Kind != "team" || first.Job.State != "completed" {
		t.Fatal("flash team not prepared", first.Result, a.jobs.Log(first.Job.ID, "stderr"))
	}
	name := first.Result.Flash.Name
	for _, c := range a.taskChoices() {
		if c.Path == first.Result.Expert {
			t.Fatal("temporary team leaked into reusable library")
		}
	}
	page := serveTest(a, "GET", "/work/"+first.Record.Thread, nil).Body.String()
	if !strings.Contains(page, "Meet "+name) || !strings.Contains(page, "Flash team · kept with this conversation") {
		t.Fatal("temporary identity missing")
	}
	second := taskSubmit(t, a, first.Record.Thread, "Make the wording warmer")
	if second.Job.State != "completed" || second.Result.Flash == nil || *second.Result.Flash != *first.Result.Flash {
		t.Fatal("follow-up lost flash identity", second.Result, a.jobs.Log(second.Job.ID, "stderr"))
	}
	for _, c := range a.taskChoices() {
		if c.Path == second.Result.Expert {
			t.Fatal("refined flash team leaked into library")
		}
	}
	page = serveTest(a, "GET", "/work/"+first.Record.Thread, nil).Body.String()
	if !strings.Contains(page, name) {
		t.Fatal("team renamed by generated README")
	}
}

func TestFlashNamingReservationIsStableAndErrorsStopDispatch(t *testing.T) {
	a := taskFixture(t)
	root := filepath.Join(a.cfg.Data, "workspaces", "flash-naming")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	rec := taskRecord{Config: a.cfg}
	plan := taskPlan{Target: "new:team"}
	first, err := prepareFlashTeam(context.Background(), root, rec, plan)
	if err != nil {
		t.Fatal(err)
	}
	// A persisted successful reservation is reused without calling the tool.
	writeFixture(t, a.cfg.Moniker, "#!/bin/sh\nexit 125\n", 0700)
	next, err := prepareFlashTeam(context.Background(), root, rec, plan)
	if err != nil || *next != *first {
		t.Fatal("reservation rerolled", err)
	}
	turn := taskSubmit(t, a, "", "Create another team")
	if turn.Result.Code != 125 || turn.Result.Prepared || strings.Contains(a.jobs.Log(turn.Job.ID, "stderr"), "Starting hire") {
		t.Fatal("naming failure dispatched build or changed outcome", turn.Result)
	}
}

func TestFlashNamingRejectsInvalidOutput(t *testing.T) {
	a := taskFixture(t)
	root := filepath.Join(a.cfg.Data, "workspaces", "bad-name")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, a.cfg.Moniker, `#!/bin/sh
printf '%s\n' '{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"A name","slug":"../../escape","theme":"playful"}'
`, 0700)
	if _, err := prepareFlashTeam(context.Background(), root, taskRecord{Config: a.cfg}, taskPlan{Target: "new:team"}); err == nil {
		t.Fatal("invalid name accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "flash-team.json")); !os.IsNotExist(err) {
		t.Fatal("bad reservation persisted")
	}
}

func TestFlashTeamsRemainSelectableOnlyInTheirConversation(t *testing.T) {
	flash := &taskFlashTeam{ID: strings.Repeat("a", 32), Name: "Cosmic Otter Crew", Slug: "cosmic-otter-crew", Theme: "playful"}
	turns := []taskTurn{{Job: Job{State: "completed"}, Result: taskResult{Flash: flash, Prepared: true, Expert: "/selected/first", Kind: "team"}}, {Job: Job{State: "completed"}, Result: taskResult{Prepared: true, Expert: "/selected/solo", Kind: "worker"}}}
	choices := withConversationFlashTeams([]taskChoice{{Key: "previous", Path: "/selected/solo", Kind: "worker"}}, turns)
	if len(choices) != 2 || choices[1].Flash != flash || choices[1].Path != "/selected/first" {
		t.Fatal("earlier team was forgotten", choices)
	}
	if got := withConversationFlashTeams(nil, nil); len(got) != 0 {
		t.Fatal("team leaked to another conversation")
	}
	if got := withConversationFlashTeams([]taskChoice{{Key: "previous", Path: "/selected/first"}}, turns); len(got) != 1 {
		t.Fatal("duplicate current team")
	}
}

// Explicit executable integration: real naming reservations, fake model-backed
// authoring/execution, isolated state, no user configuration or paid model.
func TestPublicMonikerFlashTeam(t *testing.T) {
	command := os.Getenv("BENCH_UI_MONIKER")
	if command == "" {
		t.Skip("select an absolute Moniker executable")
	}
	if !filepath.IsAbs(command) {
		t.Fatal("Moniker must be explicitly selected")
	}
	a := taskFixture(t)
	a.cfg.Moniker = command
	one := taskSubmit(t, a, "", "Create a team document")
	two := taskSubmit(t, a, "", "Create another team document")
	if one.Job.State != "completed" || two.Job.State != "completed" {
		t.Fatal("real naming integration failed", a.jobs.Log(one.Job.ID, "stderr"), a.jobs.Log(two.Job.ID, "stderr"))
	}
	if one.Result.Flash == nil || two.Result.Flash == nil || one.Result.Flash.Name == two.Result.Flash.Name || one.Result.Flash.ID == two.Result.Flash.ID {
		t.Fatal("names not unique", one.Result.Flash, two.Result.Flash)
	}
	reservations, err := os.ReadDir(filepath.Join(a.cfg.Data, "team-names"))
	if err != nil || len(reservations) != 2 {
		t.Fatal("reservations not retained", reservations, err)
	}
}
