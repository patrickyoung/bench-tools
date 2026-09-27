package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repairTeamFixture(t *testing.T) *app {
	t.Helper()
	a := goalFixture(t)
	raw, _ := os.ReadFile(a.cfg.Hire)
	writeFixture(t, a.cfg.Hire, strings.Replace(string(raw), "Team result: ", "Finished: ", 1), 0700)
	return a
}
func TestTeamBuildRepairsMissingMemberGuideBeforeExecution(t *testing.T) {
	a := repairTeamFixture(t)
	raw, _ := os.ReadFile(a.cfg.Hire)
	writeFixture(t, a.cfg.Hire, string(raw)+`
if [ ! -f repair-started ]; then
 touch repair-started
 rm expert/agents/writer/README.md
else
 /usr/bin/python3 - "$@" <<'VERIFY'
import sys
from pathlib import Path
a=sys.argv
assert a[a.index('-checkpoint')+1]=='build'
assert 'team member writer is missing README.md' in Path(a[a.index('-goal-file')+1]).read_text()
VERIFY
fi
`, 0700)
	turn := taskSubmit(t, a, "", "Finish the team document")
	if turn.Result.Code != 0 || !turn.Result.Prepared || turn.Result.GoalStatus != "complete" {
		t.Fatalf("repair failed: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
	}
	attempts, _ := os.ReadDir(filepath.Join(goalRoot(turn), "goal-attempts/prepare"))
	if len(attempts) != 2 {
		t.Fatalf("got %d preparation attempts", len(attempts))
	}
	if _, err := os.Stat(filepath.Join(turn.Result.Expert, "agents/writer/README.md")); err != nil {
		t.Fatal(err)
	}
}
func TestTeamBuildPersistentValidationStopsBeforeExecution(t *testing.T) {
	a := repairTeamFixture(t)
	raw, _ := os.ReadFile(a.cfg.Hire)
	writeFixture(t, a.cfg.Hire, string(raw)+"\nrm expert/agents/writer/README.md\n", 0700)
	turn := taskSubmit(t, a, "", "Finish the team document")
	attempts, _ := os.ReadDir(filepath.Join(goalRoot(turn), "goal-attempts/prepare"))
	if turn.Result.Code != 2 || turn.Result.Prepared || len(attempts) != 3 || !strings.Contains(turn.Result.Message, "writer is missing README.md") {
		t.Fatalf("bad stop: %+v; attempts %d", turn.Result, len(attempts))
	}
	executed, _ := filepath.Glob(filepath.Join(goalRoot(turn), "goal-attempts/execute/*"))
	if len(executed) != 0 {
		t.Fatal("invalid roster executed")
	}
}
func TestTeamRepairPreservesUnsafeBuildExit(t *testing.T) {
	a := repairTeamFixture(t)
	raw, _ := os.ReadFile(a.cfg.Hire)
	writeFixture(t, a.cfg.Hire, string(raw)+"\nif [ -f repair-started ]; then exit 125; fi\ntouch repair-started\nrm expert/agents/writer/README.md\n", 0700)
	turn := taskSubmit(t, a, "", "Finish the team document")
	attempts, _ := os.ReadDir(filepath.Join(goalRoot(turn), "goal-attempts/prepare"))
	if turn.Result.Code != 125 || turn.Result.Prepared || len(attempts) != 2 {
		t.Fatalf("unsafe retry: %+v; %d", turn.Result, len(attempts))
	}
}
func TestSavedTeamPreparationRequiresObservedBuildAndNoExecution(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "selected-members.json"), "[]", 0600)
	p := filepath.Join(root, "goal-attempts/prepare/a/outcome.json")
	for _, raw := range []string{`{"started":"1","state":"started"}`, `{"started":"1","state":"observed","exit_code":125}`, `{"started":"1","state":"observed","exit_code":130}`} {
		writeFixture(t, p, raw, 0600)
		if _, err := savedTeamPreparationCode(root); err == nil {
			t.Fatal("unsafe receipt accepted", raw)
		}
	}
	writeFixture(t, p, `{"started":"1","state":"observed","exit_code":0}`, 0600)
	if code, err := savedTeamPreparationCode(root); err != nil || code != 0 {
		t.Fatal(code, err)
	}
	writeFixture(t, filepath.Join(root, "goal-attempts/execute/a/outcome.json"), `{"state":"started"}`, 0600)
	if _, err := savedTeamPreparationCode(root); err == nil {
		t.Fatal("execution replay admitted")
	}
}
func TestTeamValidationStillRejectsChangedReuseAfterRepair(t *testing.T) {
	// The controller must reapply the original independent validation, never
	// treat a successful repair process as proof that immutable members survived.
	a := taskFixture(t)
	work, err := a.workspace()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(work)
	rec := taskRecord{Config: a.cfg}
	plan := taskPlan{Target: "new:team", Selection: &taskSelection{Roles: []taskMember{{Role: "writer", Target: "worker:source:writer"}}}}
	dest := filepath.Join(root, "authoring/expert/agents/writer")
	for _, name := range []string{"AGENTS.md", "README.md", "bin/check"} {
		writeFixture(t, filepath.Join(dest, name), "original", 0700)
	}
	_, hash, err := definitionSnapshot(rec.Config.Data, dest)
	if err != nil {
		t.Fatal(err)
	}
	if err = saveTaskJSON(filepath.Join(root, "selected-members.json"), []preparedTaskMember{{Role: "writer", Hash: hash}}); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(dest, "AGENTS.md"), "replacement", 0700)
	if code, err := repairPreparedTaskMembers(context.Background(), root, root, rec, plan); code != 2 || err == nil {
		t.Fatal("changed reuse accepted", code, err)
	}
}

func TestSavedTeamBuildRecoveryKeepsRosterAndSkipsPlanning(t *testing.T) {
	a := repairTeamFixture(t)
	raw, _ := os.ReadFile(a.cfg.Hire)
	writeFixture(t, a.cfg.Hire, string(raw)+"\nrm expert/agents/writer/README.md\n", 0700)
	first := taskSubmit(t, a, "", "Finish the team document")
	if first.Result.Code != 2 || first.Result.Prepared {
		t.Fatal(first.Result)
	}
	writeFixture(t, a.cfg.Hire, string(raw), 0700)
	next := continueFixtureTask(t, a, first)
	if next.Result.Code != 0 || next.Result.Kind != "team" || !next.Result.Prepared || next.Result.Expert != first.Result.Expert || next.Result.Flash.ID != first.Result.Flash.ID {
		t.Fatalf("recovery changed team or failed: %+v\n%s", next.Result, a.jobs.Log(next.Job.ID, "stderr"))
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(next.Job.Dir), "plan")); !os.IsNotExist(err) {
		t.Fatal("team recovery replanned")
	}
	if strings.Contains(a.jobs.Log(next.Job.ID, "stderr"), "Starting moniker") {
		t.Fatal("renamed team")
	}
}
