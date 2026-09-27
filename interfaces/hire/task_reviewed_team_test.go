package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewedTeamUsesExactFrozenSourceWithoutRebuilding(t *testing.T) {
	a := taskFixture(t)
	first := taskSubmit(t, a, "", "Create a team document")
	if first.Job.State != "completed" {
		t.Fatal(a.jobs.Log(first.Job.ID, "stderr"))
	}
	_, digest, err := definitionSnapshot(a.cfg.Data, first.Result.Expert)
	if err != nil {
		t.Fatal(err)
	}
	a.cfg.ReviewedTeamControllers = digest
	// This host rejects nested Cage. Only explicitly selected source can use
	// the reviewed host coordinator; never change Agent's default action flags.
	writeFixture(t, filepath.Join(filepath.Dir(a.cfg.Agent), "cage"), "#!/bin/sh\nexit 125\n", 0700)
	second := taskSubmit(t, a, first.Record.Thread, "Make it warmer")
	log := a.jobs.Log(second.Job.ID, "stderr")
	if second.Job.State != "completed" || strings.Contains(log, "Starting hire") || !strings.Contains(log, "explicitly reviewed host coordinator") {
		t.Fatalf("reviewed composition failed: %+v\n%s", second.Result, log)
	}
	// A member-check edit invalidates the entire approval, even if the
	// coordinator and public roster have not changed.
	check := filepath.Join(second.Result.Expert, "agents", "writer", "bin", "check")
	f, err := os.OpenFile(check, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\n# changed after review\n")
	_ = f.Close()
	third := taskSubmit(t, a, first.Record.Thread, "Another edit")
	if third.Result.Code != 125 || third.Result.Prepared || strings.Contains(a.jobs.Log(third.Job.ID, "stderr"), "explicitly reviewed host coordinator") {
		t.Fatalf("changed source inherited approval: %+v", third.Result)
	}
}

func TestReviewedTeamFingerprintAdmission(t *testing.T) {
	for _, value := range []string{"", strings.Repeat("a", 64), strings.Repeat("a", 64) + "," + strings.Repeat("b", 64)} {
		if err := validateReviewedTeams(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"true", "*", strings.Repeat("a", 63), strings.Repeat("a", 64) + ",", strings.Repeat("a", 64) + "\n"} {
		if validateReviewedTeams(value) == nil {
			t.Fatalf("unsafe approval accepted: %q", value)
		}
	}
}
