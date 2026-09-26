package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func localEntries(a *app) []entry {
	var out []entry
	for _, e := range a.catalog().Workers {
		if e.Local {
			out = append(out, e)
		}
	}
	return out
}

func localFixture(t *testing.T, a *app, kind, code string) Job {
	t.Helper()
	dir, err := a.workspace()
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(dir, "expert", "README.md"), "# UPS guide\n<untrusted> source text", 0600)
	j, err := a.jobs.Start(kind, "UPS Tracker", dir, helperArgs("echo", code, "fixture"), "")
	if err != nil {
		t.Fatal(err)
	}
	return awaitJob(t, a.jobs, j.ID)
}

func verifyLocal(t *testing.T, a *app, j Job) Job {
	t.Helper()
	return admittedJob(t, a, serveTest(a, "POST", "/jobs/"+j.ID+"/verify", formFor(a)))
}

func TestLocalWorkersDiscoverVerifySearchAndRecover(t *testing.T) {
	a := testApp(t)
	build := localFixture(t, a, "build", "2")
	if len(localEntries(a)) != 0 {
		t.Fatal("unfinished build listed")
	}
	verified := verifyLocal(t, a, build)
	entries := localEntries(a)
	if len(entries) != 1 || entries[0].Name() != "UPS Tracker" || entries[0].Status != "experimental" || entries[0].Exportable() {
		t.Fatalf("wrong local entry: %+v", entries)
	}
	worker := entries[0]
	for _, path := range []string{"/workers?q=UPS", "/workers?q=UPS", worker.URL()} {
		w := serveTest(a, "GET", path, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "UPS Tracker") {
			t.Fatalf("worker missing at %s", path)
		}
	}
	body := serveTest(a, "GET", worker.URL(), nil).Body.String()
	if !strings.Contains(body, "&lt;untrusted&gt;") || strings.Contains(body, `action="/exports"`) || !strings.Contains(body, "/jobs/"+verified.ID) {
		t.Fatal("local guide was unsafe or exposed a source export")
	}
	if !strings.Contains(serveTest(a, "GET", "/jobs/"+build.ID, nil).Body.String(), worker.URL()) {
		t.Fatal("result missing worker link")
	}
	verifyLocal(t, a, verified)
	if got := localEntries(a); len(got) != 1 || got[0].ID != worker.ID {
		t.Fatal("reverification duplicated or renamed worker")
	}
	if err := a.jobs.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := newJobManager(a.cfg.Data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { recovered.Close() })
	a.jobs = recovered
	if got := localEntries(a); len(got) != 1 || got[0].ID != worker.ID {
		t.Fatal("worker not recovered")
	}
	if err := os.Remove(filepath.Join(build.Dir, "expert", "README.md")); err != nil {
		t.Fatal(err)
	}
	if len(localEntries(a)) != 0 {
		t.Fatal("missing result listed")
	}
	if w := serveTest(a, "GET", worker.URL(), nil); w.Code != 404 {
		t.Fatal("missing local guide served")
	}
}

func TestLocalWorkersOnlyCompletedAuthoredResults(t *testing.T) {
	for _, tc := range []struct {
		kind, code string
		count      int
	}{
		{"build", "0", 1}, {"build", "1", 0}, {"build", "2", 0}, {"build", "125", 0}, {"build", "130", 0}, {"new", "0", 0}, {"export", "0", 0},
	} {
		t.Run(tc.kind+tc.code, func(t *testing.T) {
			a := testApp(t)
			j := localFixture(t, a, tc.kind, tc.code)
			if got := len(localEntries(a)); got != tc.count {
				t.Fatalf("got %d local entries", got)
			}
			if tc.kind == "new" || tc.kind == "export" {
				verifyLocal(t, a, j)
				want := 0
				if tc.kind == "new" {
					want = 1
				}
				if len(localEntries(a)) != want {
					t.Fatal("verification confused authored and exported results")
				}
			}
		})
	}
}

func TestLocalWorkersLatestFailureAndUnsafeGuide(t *testing.T) {
	a := testApp(t)
	j := localFixture(t, a, "build", "0")
	worker := localEntries(a)[0]
	guide := filepath.Join(j.Dir, "expert", "README.md")
	if err := os.Remove(guide); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private")
	writeFixture(t, outside, "SECRET OUTSIDE ROOT", 0600)
	if err := os.Symlink(outside, guide); err != nil {
		t.Fatal(err)
	}
	if len(localEntries(a)) != 0 {
		t.Fatal("symlink guide escaped data root")
	}
	os.Remove(guide)
	writeFixture(t, guide, "restored", 0600)
	writeFixture(t, a.cfg.Hire, "#!/bin/sh\nexit 1\n", 0700)
	failed := verifyLocal(t, a, j)
	if failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Fatal("fixture did not fail")
	}
	if len(localEntries(a)) != 0 {
		t.Fatal("older success hid latest failure")
	}
	if w := serveTest(a, "GET", worker.URL(), nil); w.Code != 404 {
		t.Fatal("stale worker remained accessible")
	}
}
