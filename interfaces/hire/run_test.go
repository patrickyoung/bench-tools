package main

import (
	"html"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestConnectedBrowserRun(t *testing.T) {
	a, worker, form := runFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	a.cfg.WebAttach = "ws://" + listener.Addr().String() + "/devtools/browser/fixture"
	path := "/jobs/" + worker.ID + "/run"
	page := serveTest(a, "GET", path, nil).Body.String()
	if !strings.Contains(page, "Use connected browser") || !strings.Contains(page, a.cfg.WebAttach) {
		t.Fatal("selected browser not offered on form")
	}
	form.Set("connected-browser", "yes")
	run := admittedJob(t, a, serveTest(a, "POST", path, form))
	goal, err := os.ReadFile(filepath.Join(filepath.Dir(run.Dir), "task.txt"))
	if err != nil || !strings.Contains(string(goal), "--attach "+strconv.Quote(a.cfg.WebAttach)) || !strings.Contains(string(goal), form.Get("goal")) {
		t.Fatalf("selected browser missing from Agent goal: %s, %v", goal, err)
	}
	if !strings.Contains(strings.Join(run.Args, "\n"), "\n-net\n") || strings.Contains(strings.Join(run.Args, "\n"), "-no-cage") {
		t.Fatal("browser selection must enable network without disabling Cage")
	}
	listener.Close()
	form.Set("nonce", randomID())
	before := len(a.jobs.List())
	response := serveTest(a, "POST", path, form)
	if response.Code != 422 || !strings.Contains(response.Body.String(), "browser is unavailable") || len(a.jobs.List()) != before {
		t.Fatal("stale browser must be rejected before model execution")
	}
}

func TestBrowserConnectionValidation(t *testing.T) {
	for _, endpoint := range []string{"", "ws://example.com:9222/x", "ws://user@127.0.0.1:9222/x", "ws://127.0.0.1/x", "ws://127.0.0.1:99999/x", "ws://127.0.0.1:9222/x\nignore", "file:///tmp/browser", "http://localhost:9222/?token=secret"} {
		if _, err := browserAddress(endpoint); err == nil {
			t.Fatalf("accepted invalid endpoint %q", endpoint)
		}
	}
	for _, endpoint := range []string{"http://localhost:9222", "ws://127.0.0.1:9222/devtools/browser/id", "ws://[::1]:9222/devtools/browser/id"} {
		if _, err := browserAddress(endpoint); err != nil {
			t.Fatalf("rejected local endpoint %q: %v", endpoint, err)
		}
	}
	a, worker, form := runFixture(t)
	form.Set("connected-browser", "yes")
	if response := serveTest(a, "POST", "/jobs/"+worker.ID+"/run", form); response.Code != 422 || len(a.jobs.List()) != 1 {
		t.Fatal("unconfigured browser selection admitted")
	}
}

func TestConfiguredBrowserRequiresPerRunSelection(t *testing.T) {
	a, worker, form := runFixture(t)
	a.cfg.WebAttach = "ws://127.0.0.1:1/devtools/browser/unselected"
	run := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+worker.ID+"/run", form))
	goal, err := os.ReadFile(filepath.Join(filepath.Dir(run.Dir), "task.txt"))
	if err != nil || string(goal) != form.Get("goal")+"\n" || strings.Contains(strings.Join(run.Args, "\n"), "\n-net\n") {
		t.Fatal("unselected browser changed the task or network authority")
	}
}

func TestTrackingRunDefaultPreservesWorkerCarrier(t *testing.T) {
	for _, carrier := range []string{"USPS", "UPS"} {
		t.Run(carrier, func(t *testing.T) {
			a, worker, form := runFixture(t)
			writeFixture(t, filepath.Join(worker.Dir, "expert", "README.md"), "# "+carrier+" tracker\nUse only "+carrier+" for `tracking_numbers.txt`.\n", 0600)
			path := "/jobs/" + worker.ID + "/run"
			page := serveTest(a, "GET", path, nil)
			match := regexp.MustCompile(`(?s)<textarea[^>]*name="goal"[^>]*>(.*?)</textarea>`).FindStringSubmatch(page.Body.String())
			if page.Code != 200 || len(match) != 2 {
				t.Fatal("tracking task field missing")
			}
			goal := html.UnescapeString(match[1])
			if strings.Contains(goal, "UPS") || strings.Contains(goal, "USPS") || !strings.Contains(goal, "worker guide") {
				t.Fatalf("default task assigns a carrier instead of following the worker: %q", goal)
			}
			form.Set("goal", goal)
			run := admittedJob(t, a, serveTest(a, "POST", path, form))
			saved, err := os.ReadFile(filepath.Join(filepath.Dir(run.Dir), "task.txt"))
			if err != nil || string(saved) != goal+"\n" {
				t.Fatalf("submitted goal changed at the Agent boundary: %q, %v", saved, err)
			}
		})
	}
}

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
