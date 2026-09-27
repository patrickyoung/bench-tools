package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectedTeamEnvironmentHandlesNewTeams(t *testing.T) {
	a := taskFixture(t)
	writeFixture(t, filepath.Join(filepath.Dir(a.cfg.Agent), "cage"), "#!/bin/sh\nexit 125\n", 0700)
	a.cfg.TeamRunner = filepath.Join(t.TempDir(), "selected-runtime")
	writeFixture(t, a.cfg.TeamRunner, `#!/usr/bin/python3
import os,sys
from pathlib import Path
a=sys.argv[1:]
assert a[:2]==['-net','-w'] and Path(a[2]).resolve()==Path.cwd() and a[3]=='--'
c=a[4:]
if c[0]=='cage':
 assert c[1:]==['-w',a[2],'--','/bin/sh','-c','exit 0']
 print('selected environment member preflight',file=sys.stderr)
 sys.exit(0)
assert c[0].endswith('/expert/bin/task')
print('selected environment team entry',file=sys.stderr)
os.execv(c[0],c)
`, 0700)
	for _, message := range []string{"Create a new team document", "Create a different team document"} {
		turn := taskSubmit(t, a, "", message)
		log := a.jobs.Log(turn.Job.ID, "stderr")
		if turn.Result.Code != 0 || len(turn.Result.Artifacts) == 0 || !strings.Contains(log, "selected environment member preflight") || !strings.Contains(log, "selected environment team entry") {
			t.Fatalf("new team failed in selected environment: %+v\n%s", turn.Result, log)
		}
	}
}

func TestSelectedTeamEnvironmentFailureNeverFallsBack(t *testing.T) {
	a := taskFixture(t)
	a.cfg.TeamRunner = filepath.Join(t.TempDir(), "unavailable-runtime")
	writeFixture(t, a.cfg.TeamRunner, "#!/bin/sh\necho 'selected environment unavailable' >&2\nexit 125\n", 0700)
	turn := taskSubmit(t, a, "", "Create a team document")
	log := a.jobs.Log(turn.Job.ID, "stderr")
	if turn.Result.Code != 125 || turn.Result.Prepared || strings.Contains(log, "Starting hire") {
		t.Fatalf("runtime failure fell back or lost status: %+v\n%s", turn.Result, log)
	}
	if err := os.Remove(a.cfg.TeamRunner); err != nil {
		t.Fatal(err)
	}
	if _, err := taskTeamRunner(a.cfg); err == nil {
		t.Fatal("missing selected runtime fell back to native Cage")
	}
}
