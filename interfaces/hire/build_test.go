package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func unfinishedBuild(t *testing.T, a *app, legacy bool) Job {
	t.Helper()
	a.cfg.AllowBuild = true
	writeFixture(t, a.cfg.Hire, "#!/bin/sh\nprintf '%s\\n' \"$@\"\nmkdir -p expert\nprintf 'saved draft' > expert/README.md\nexit 2\n", 0700)
	dir, err := a.workspace()
	if err != nil {
		t.Fatal(err)
	}
	goal := filepath.Join(filepath.Dir(dir), "brief.txt")
	writeFixture(t, goal, "Finish the original worker", 0600)
	args := buildArgs(a.cfg.Hire, dir, goal, "fixture/model")
	if legacy {
		args = args[:14]
		args[11] = "8"
	}
	j, err := a.jobs.Start("build", "A longer build", dir, args, "")
	if err != nil {
		t.Fatal(err)
	}
	return awaitJob(t, a.jobs, j.ID)
}

func TestBuildContinuationPreservesDraftAndSurvivesRestart(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "checkpoint", true: "legacy"}[legacy], func(t *testing.T) {
			a := testApp(t)
			j := unfinishedBuild(t, a, legacy)
			page := serveTest(a, "GET", "/jobs/"+j.ID, nil).Body.String()
			if !strings.Contains(page, "Continue for 50 more turns") || (legacy && !strings.Contains(page, "no conversation checkpoint")) {
				t.Fatal("missing continuation explanation")
			}
			writeFixture(t, a.cfg.Hire, "#!/bin/sh\ntest \"$(cat expert/README.md)\" = 'saved draft' || exit 99\nprintf '%s\\n' \"$@\"\nexit 2\n", 0700)
			f := formFor(a)
			next := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+j.ID+"/continue", f))
			if next.State != "unfinished" || next.Dir != j.Dir || next.ID == j.ID || next.Args[11] != "50" || next.Args[15] != "build" {
				t.Fatalf("continuation: %+v", next)
			}
			if again := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", f); again.Header().Get("Location") != "/jobs/"+next.ID {
				t.Fatal("duplicate click was not idempotent")
			}
			if w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", formFor(a)); w.Code != 409 {
				t.Fatal("stale page admitted")
			}
			if page := serveTest(a, "GET", "/jobs/"+j.ID, nil).Body.String(); strings.Contains(page, "Continue for 50 more turns") || !strings.Contains(page, "/jobs/"+next.ID) {
				t.Fatal("old page did not link latest")
			}
			if err := a.jobs.Close(); err != nil {
				t.Fatal(err)
			}
			m, err := newJobManager(a.cfg.Data)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			restarted, err := newApp(a.cfg, a.cat, m, "127.0.0.1:8787")
			if err != nil {
				t.Fatal(err)
			}
			if w := serveTest(restarted, "POST", "/jobs/"+j.ID+"/continue", formFor(restarted)); w.Code != 409 {
				t.Fatal("restart admitted stale continuation")
			}
			third := admittedJob(t, restarted, serveTest(restarted, "POST", "/jobs/"+next.ID+"/continue", formFor(restarted)))
			if third.State != "unfinished" || third.Dir != j.Dir {
				t.Fatal("could not continue again")
			}
			if old, _ := m.Get(j.ID); old.State != "unfinished" || old.Args[11] != j.Args[11] {
				t.Fatal("history rewritten")
			}
		})
	}
}

func TestBuildContinuationRejectsOtherOutcomesAndChangedCommands(t *testing.T) {
	a := testApp(t)
	j := unfinishedBuild(t, a, false)
	for _, state := range []string{"running", "unknown", "cancelled", "failed", "completed"} {
		candidate := j
		candidate.State = state
		if _, _, err := a.continuation(candidate); err == nil {
			t.Fatalf("accepted %s", state)
		}
	}
	for _, kind := range []string{"run", "new", "analyze", "assist", "verify"} {
		candidate := j
		candidate.Kind = kind
		if _, _, err := a.continuation(candidate); err == nil {
			t.Fatalf("accepted %s", kind)
		}
	}
	for _, index := range []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15} {
		candidate := j
		candidate.Args = append([]string(nil), j.Args...)
		candidate.Args[index] = "unexpected"
		if _, _, err := a.continuation(candidate); err == nil {
			t.Fatalf("accepted changed argv[%d]", index)
		}
	}
	a.cfg.AllowBuild = false
	if w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", formFor(a)); w.Code != 409 {
		t.Fatal("disabled continuation admitted")
	}
	a.cfg.AllowBuild = true
	f := formFor(a)
	f.Set("csrf", "wrong")
	if w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", f); w.Code != 403 {
		t.Fatal("CSRF admitted")
	}
	if err := os.Remove(filepath.Join(filepath.Dir(j.Dir), "brief.txt")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.continuation(j); err == nil {
		t.Fatal("missing brief accepted")
	}
}

func TestTeamContinuationDoesNotRepeatStaging(t *testing.T) {
	a, f := teamFixture(t)
	draft := admittedJob(t, a, serveTest(a, "POST", "/teams", f))
	writeFixture(t, a.cfg.Hire, "#!/bin/sh\nprintf 'unfinished author'\nexit 2\n", 0700)
	built := teamBuildFixture(t, a, draft)
	if _, _, err := a.continuation(built); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, a.cfg.Python, "#!/bin/sh\nexit 99\n", 0700)
	marker := filepath.Join(built.Dir, "expert", "preserved.txt")
	writeFixture(t, marker, "saved work", 0600)
	continued := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+built.ID+"/continue", formFor(a)))
	if continued.State != "unfinished" || continued.Args[0] != a.cfg.Hire {
		t.Fatalf("repeated team staging: %+v", continued)
	}
	if b, err := os.ReadFile(marker); err != nil || string(b) != "saved work" {
		t.Fatal("draft lost")
	}
	// A separate staging failure with exit 2 is not an unfinished model build.
	a, f = teamFixture(t)
	draft = admittedJob(t, a, serveTest(a, "POST", "/teams", f))
	writeFixture(t, a.cfg.Python, "#!/bin/sh\nexit 2\n", 0700)
	failed := teamBuildFixture(t, a, draft)
	if _, _, err := a.continuation(failed); err == nil {
		t.Fatal("export failure offered model continuation")
	}
}

func TestContinuationRespectsActiveCommand(t *testing.T) {
	a := testApp(t)
	j := unfinishedBuild(t, a, false)
	busy, err := a.jobs.Start("new", "Busy", t.TempDir(), []string{"/bin/sh", "-c", "sleep 10"}, "")
	if err != nil {
		t.Fatal(err)
	}
	w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", formFor(a))
	if w.Code != 409 {
		t.Fatal("overlapping continuation")
	}
	_ = a.jobs.Cancel(busy.ID)
}
