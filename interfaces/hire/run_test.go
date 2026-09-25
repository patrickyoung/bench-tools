package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runFixture(t *testing.T) (*app, Job, url.Values) {
	t.Helper()
	a := testApp(t)
	j := localFixture(t, a, "build", "0")
	writeFixture(t, filepath.Join(j.Dir, "expert", "bin", "check"), "#!/bin/sh\nexit 0\n", 0700)
	a.cfg.AllowRun = true
	a.cfg.Agent = filepath.Join(t.TempDir(), "fake agent")
	writeFixture(t, a.cfg.Agent, "#!/bin/sh\ncat input.txt > report.md\nprintf 'worker ran\\n'\n", 0700)
	f := formFor(a)
	f.Set("goal", "Report the supplied inputs")
	f.Set("model", "fixture/model")
	f.Set("turns", "20")
	f.Set("input-name", "input.txt")
	f.Set("input", "<script>literal input</script>")
	f.Set("output", "report.md")
	f.Set("run-consent", "yes")
	return a, j, f
}
func TestWorkerRunAndResult(t *testing.T) {
	a, worker, f := runFixture(t)
	if w := serveTest(a, "GET", "/jobs/"+worker.ID+"/run", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "Review the worker") {
		t.Fatal("run form missing")
	}
	run := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+worker.ID+"/run", f))
	if run.Kind != "run" || run.Dir == worker.Dir || within(worker.Dir, run.Dir) || within(run.Dir, worker.Dir) {
		t.Fatal("run reused definition workspace")
	}
	if run.Args[len(run.Args)-1] != resultPath(worker) {
		t.Fatal("wrong definition")
	}
	for _, arg := range run.Args {
		if arg == "-net" || strings.Contains(arg, "literal input") {
			t.Fatal("network default or private-input boundary lost")
		}
	}
	if jobLabel(run) != "Run completed" {
		t.Fatal("wrong run outcome")
	}
	body := serveTest(a, "GET", "/jobs/"+run.ID, nil).Body.String()
	if !strings.Contains(body, "&lt;script&gt;literal input") || strings.Contains(body, "Verify structure") {
		t.Fatal("result page unsafe or treated output as definition")
	}
	download := serveTest(a, "GET", "/jobs/"+run.ID+"/output", nil)
	if download.Code != 200 || download.Body.String() != f.Get("input") || !strings.Contains(download.Header().Get("Content-Disposition"), "report.md") {
		t.Fatal("download failed")
	}
	if w := serveTest(a, "POST", "/jobs/"+run.ID+"/verify", formFor(a)); w.Code != 409 {
		t.Fatal("run result verified as definition")
	}
	if err := os.Remove(filepath.Join(run.Dir, "report.md")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private.txt")
	writeFixture(t, outside, "private", 0600)
	os.Symlink(outside, filepath.Join(run.Dir, "report.md"))
	if w := serveTest(a, "GET", "/jobs/"+run.ID+"/output", nil); w.Code != 404 {
		t.Fatal("result symlink escaped")
	}
}
func TestRunValidationAndExplicitNetwork(t *testing.T) {
	a, worker, f := runFixture(t)
	path := "/jobs/" + worker.ID + "/run"
	a.cfg.AllowRun = false
	if w := serveTest(a, "POST", path, f); w.Code != 403 {
		t.Fatal("disabled run admitted")
	}
	a.cfg.AllowRun = true
	for _, tc := range []struct{ key, value string }{{"run-consent", ""}, {"model", ""}, {"turns", "0"}, {"turns", "101"}, {"input-name", "../escape"}, {"input-name", "state"}, {"output", "/tmp/output"}, {"output", "input.txt"}} {
		old := f.Get(tc.key)
		f.Set(tc.key, tc.value)
		w := serveTest(a, "POST", path, f)
		if w.Code != 422 || !strings.Contains(w.Body.String(), "literal input") || len(a.jobs.List()) != 1 {
			t.Fatalf("invalid %s admitted: %d", tc.key, w.Code)
		}
		f.Set(tc.key, old)
	}
	f.Set("network", "yes")
	run := admittedJob(t, a, serveTest(a, "POST", path, f))
	if !strings.Contains(strings.Join(run.Args, "\n"), "\n-net\n") {
		t.Fatal("selected network omitted")
	}
	again := serveTest(a, "POST", path, f)
	if again.Code != 303 || again.Header().Get("Location") != "/jobs/"+run.ID || len(a.jobs.List()) != 2 {
		t.Fatal("duplicate submission repeated run")
	}
}
