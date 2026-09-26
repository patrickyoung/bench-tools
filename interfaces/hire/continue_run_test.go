package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func stoppedRun(t *testing.T, code string, network bool) (*app, Job) {
	t.Helper()
	a, worker, f := runFixture(t)
	writeFixture(t, a.cfg.Agent, "#!/bin/sh\ncat input.txt > report.md\nprintf 'saved' > progress.txt\nexit "+code+"\n", 0700)
	if network {
		f.Set("network", "yes")
	}
	j := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+worker.ID+"/run", f))
	return a, j
}

func TestContinueRunPreservesWorkAndAllowsFurtherTurns(t *testing.T) {
	for _, network := range []bool{false, true} {
		t.Run(map[bool]string{false: "offline", true: "network"}[network], func(t *testing.T) {
			a, j := stoppedRun(t, "2", network)
			page := serveTest(a, "GET", "/jobs/"+j.ID, nil).Body.String()
			if !strings.Contains(page, "Continue run for 50 more turns") || strings.Contains(page, "recovery-consent") {
				t.Fatal("missing normal continuation")
			}
			// Resume must see the previous output/input/work, without restaging it.
			writeFixture(t, a.cfg.Agent, "#!/bin/sh\ntest \"$(cat progress.txt)\" = saved || exit 99\ncmp input.txt report.md || exit 98\nprintf 'continued\\n'\nexit 2\n", 0700)
			f := formFor(a)
			next := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+j.ID+"/continue", f))
			want := append([]string(nil), j.Args...)
			want[11] = "50"
			if next.State != "unfinished" || next.Dir != j.Dir || !reflect.DeepEqual(next.Args, want) {
				t.Fatalf("run not preserved: %+v", next)
			}
			if w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", f); w.Header().Get("Location") != "/jobs/"+next.ID {
				t.Fatal("duplicate repeated execution")
			}
			if w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", formFor(a)); w.Code != 409 {
				t.Fatal("stale continuation admitted")
			}
			if page := serveTest(a, "GET", "/jobs/"+j.ID, nil).Body.String(); strings.Contains(page, "Continue run for 50 more turns") || !strings.Contains(page, "Open latest activity") {
				t.Fatal("stale control shown")
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
				t.Fatal("restart admitted stale run")
			}
			writeFixture(t, a.cfg.Agent, "#!/bin/sh\nprintf 'finished' > report.md\nexit 0\n", 0700)
			final := admittedJob(t, restarted, serveTest(restarted, "POST", "/jobs/"+next.ID+"/continue", formFor(restarted)))
			if final.State != "completed" || final.Dir != j.Dir {
				t.Fatal("repeat continuation failed")
			}
			if w := serveTest(restarted, "GET", "/jobs/"+final.ID+"/output", nil); w.Body.String() != "finished" {
				t.Fatal("continued output unavailable")
			}
			if old, _ := m.Get(j.ID); old.State != "unfinished" || old.Args[11] != "20" {
				t.Fatal("original record changed")
			}
			if w := serveTest(restarted, "POST", "/jobs/"+final.ID+"/continue", formFor(restarted)); w.Code != 409 {
				t.Fatal("successful run resumed")
			}
		})
	}
}

func TestUncertainRunNeedsExplicitReview(t *testing.T) {
	a, j := stoppedRun(t, "125", false)
	page := serveTest(a, "GET", "/jobs/"+j.ID, nil).Body.String()
	if !strings.Contains(page, "recovery-consent") || !strings.Contains(page, "existing effects") {
		t.Fatal("review gate missing")
	}
	before := len(a.jobs.List())
	f := formFor(a)
	if w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", f); w.Code != 409 || len(a.jobs.List()) != before {
		t.Fatal("uncertain run continued without review")
	}
	f.Set("recovery-consent", "yes")
	next := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+j.ID+"/continue", f))
	if next.ExitCode == nil || *next.ExitCode != 125 {
		t.Fatal("outcome rewritten")
	}
	if w := serveTest(a, "POST", "/jobs/"+next.ID+"/continue", formFor(a)); w.Code != 409 {
		t.Fatal("new uncertain outcome skipped review")
	}
}

func TestContinueRunValidation(t *testing.T) {
	a, j := stoppedRun(t, "2", false)
	for _, state := range []string{"running", "unknown", "cancelled", "failed", "completed"} {
		other := j
		other.State = state
		if _, _, err := a.runContinuation(other); err == nil {
			t.Fatalf("accepted %s", state)
		}
	}
	for i := 1; i < len(j.Args); i++ {
		if i == 11 {
			continue
		} // Different bounded original turn counts are valid.
		other := j
		other.Args = append([]string(nil), j.Args...)
		other.Args[i] = "unexpected"
		if i == 9 || i == 17 || i == 19 {
			continue
		} // Model and basenames are validated values, not fixed literals.
		if _, _, err := a.runContinuation(other); err == nil {
			t.Fatalf("accepted changed arg %d", i)
		}
	}
	other := j
	other.Args = append([]string(nil), j.Args...)
	other.Args = append(other.Args[:len(other.Args)-1], "-no-cage", other.Args[len(other.Args)-1])
	if _, _, err := a.runContinuation(other); err == nil {
		t.Fatal("expanded permission")
	}
	f := formFor(a)
	f.Set("csrf", "wrong")
	if w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", f); w.Code != 403 {
		t.Fatal("CSRF bypass")
	}
	a.cfg.AllowRun = false
	if _, _, err := a.runContinuation(j); err == nil {
		t.Fatal("run permission bypass")
	}
	a.cfg.AllowRun = true
	a.cfg.AllowBuild = false
	if _, _, err := a.runContinuation(j); err != nil {
		t.Fatal("run incorrectly requires build permission", err)
	}
	busy, err := a.jobs.Start("new", "Busy", t.TempDir(), []string{"/bin/sh", "-c", "sleep 10"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if w := serveTest(a, "POST", "/jobs/"+j.ID+"/continue", formFor(a)); w.Code != 409 {
		t.Fatal("concurrent run admitted")
	}
	_ = a.jobs.Cancel(busy.ID)
	awaitJob(t, a.jobs, busy.ID)
	if err := os.Remove(filepath.Join(filepath.Dir(j.Dir), "task.txt")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.runContinuation(j); err == nil {
		t.Fatal("lost task accepted")
	}
}

func TestRunDefaultIsFifty(t *testing.T) {
	a, worker, _ := runFixture(t)
	page := serveTest(a, "GET", "/jobs/"+worker.ID+"/run", nil).Body.String()
	if !strings.Contains(page, `name="turns" min="1" max="100" value="50"`) {
		t.Fatal("default is not 50")
	}
}
