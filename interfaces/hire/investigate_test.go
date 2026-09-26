package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func analysisFixture(t *testing.T) (*app, Job, Job) {
	t.Helper()
	a, worker, f := runFixture(t)
	run := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+worker.ID+"/run", f))
	a.cfg.AllowBuild = true
	a.cfg.Ask = filepath.Join(t.TempDir(), "ask")
	writeFixture(t, a.cfg.Ask, "#!/bin/sh\nprintf 'Observed: <script>empty page</script>. Wait for tracking content.\\n'\n", 0700)
	a.cfg.Record = filepath.Join(t.TempDir(), "record")
	writeFixture(t, a.cfg.Record, "#!/bin/sh\nwhile [ \"$1\" != -- ]; do shift; done\nshift\nexec \"$@\"\n", 0700)
	a.cfg.Hire = filepath.Join(t.TempDir(), "hire")
	writeFixture(t, a.cfg.Hire, "#!/bin/sh\ncase \"$1\" in\nbuild) printf '\\nWait for #trackingNum and actual status content.\\n' >> expert/README.md;;\nverify) test -f \"$2/README.md\";;\n*) exit 9;;\nesac\n", 0700)
	return a, worker, run
}
func analyzeFixture(t *testing.T, a *app, run Job) Job {
	t.Helper()
	f := formFor(a)
	f.Set("model", "fixture/model")
	f.Set("question", "Why empty?")
	f.Add("evidence", "report.md")
	return admittedJob(t, a, serveTest(a, "POST", "/jobs/"+run.ID+"/investigate", f))
}
func proposalFixture(t *testing.T, a *app, analysis Job) Job {
	t.Helper()
	f := formFor(a)
	f.Set("model", "fixture/model")
	f.Set("change", "Wait for actual tracking content; preserve acceptance.")
	return admittedJob(t, a, serveTest(a, "POST", "/jobs/"+analysis.ID+"/prepare-fix", f))
}
func TestInvestigationReviewSaveBoundary(t *testing.T) {
	a, worker, run := analysisFixture(t)
	original, _ := os.ReadFile(filepath.Join(worker.Dir, "expert", "README.md"))
	for _, path := range []string{"/jobs/" + run.ID, "/jobs/" + run.ID + "/investigate"} {
		w := serveTest(a, "GET", path, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Analyze issue") {
			t.Fatalf("missing analysis entry: %d %s", w.Code, w.Body.String())
		}
	}
	analysis := analyzeFixture(t, a, run)
	snap, _ := os.ReadFile(filepath.Join(filepath.Dir(analysis.Dir), "snapshot.json"))
	var payload map[string]any
	if json.Unmarshal(snap, &payload) != nil || !strings.Contains(string(snap), "report.md") || strings.Contains(string(snap), "state/") {
		t.Fatal("wrong snapshot")
	}
	body := serveTest(a, "GET", "/jobs/"+analysis.ID+"/review", nil).Body.String()
	if strings.Contains(body, "<script>empty") || !strings.Contains(body, "&lt;script&gt;empty") || !strings.Contains(body, "Prepare fix") {
		t.Fatal("analysis escaped/review missing")
	}
	for _, j := range []Job{analysis} {
		if serveTest(a, "POST", "/jobs/"+j.ID+"/verify", formFor(a)).Code != 409 {
			t.Fatal("analysis bypassed review")
		}
	}
	body = serveTest(a, "GET", "/jobs/"+run.ID, nil).Body.String()
	if !strings.Contains(body, "/jobs/"+analysis.ID+"/review") {
		t.Fatal("run missing retained analysis link")
	}
	proposal := proposalFixture(t, a, analysis)
	if len(localEntries(a)) != 1 {
		t.Fatal("unreviewed proposal entered catalog")
	}
	if serveTest(a, "POST", "/jobs/"+proposal.ID+"/verify", formFor(a)).Code != 409 {
		t.Fatal("proposal bypassed review")
	}
	body = serveTest(a, "GET", "/jobs/"+proposal.ID+"/review", nil).Body.String()
	if !strings.Contains(body, "Proposed") || !strings.Contains(body, "Before") || !strings.Contains(body, "#trackingNum") {
		t.Fatal("missing exact changes")
	}
	f := formFor(a)
	f.Set("review-consent", "yes")
	f.Set("proposal-hash", "stale")
	if serveTest(a, "POST", "/jobs/"+proposal.ID+"/save-revision", f).Code != 409 {
		t.Fatal("stale proposal admitted")
	}
	_, hash, err := definitionSnapshot(a.cfg.Data, filepath.Join(proposal.Dir, "expert"))
	if err != nil {
		t.Fatal(err)
	}
	f = formFor(a)
	f.Set("proposal-hash", hash)
	if serveTest(a, "POST", "/jobs/"+proposal.ID+"/save-revision", f).Code != 422 {
		t.Fatal("unreviewed save admitted")
	}
	f = formFor(a)
	f.Set("proposal-hash", hash)
	f.Set("review-consent", "yes")
	saved := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+proposal.ID+"/save-revision", f))
	if saved.State != "completed" || len(localEntries(a)) != 2 {
		t.Fatal("reviewed revision not saved separately")
	}
	after, _ := os.ReadFile(filepath.Join(worker.Dir, "expert", "README.md"))
	if string(after) != string(original) {
		t.Fatal("original overwritten")
	}
	if _, ok := a.runnable(saved.ID); !ok {
		t.Fatal("saved revision cannot run")
	}
}
func TestInvestigationRejectsUnsafeOrStaleInputs(t *testing.T) {
	a, worker, run := analysisFixture(t)
	secret := filepath.Join(t.TempDir(), "secret")
	writeFixture(t, secret, "private", 0600)
	os.Symlink(secret, filepath.Join(run.Dir, "linked"))
	f := formFor(a)
	f.Set("model", "fixture/model")
	f.Add("evidence", "linked")
	if serveTest(a, "POST", "/jobs/"+run.ID+"/investigate", f).Code != 409 {
		t.Fatal("linked evidence accepted")
	}
	f = formFor(a)
	f.Set("model", "fixture/model")
	f.Add("evidence", "../outside")
	if serveTest(a, "POST", "/jobs/"+run.ID+"/investigate", f).Code != 409 {
		t.Fatal("outside evidence accepted")
	}
	analysis := analyzeFixture(t, a, run)
	proposal := proposalFixture(t, a, analysis)
	_, hash, _ := definitionSnapshot(a.cfg.Data, filepath.Join(proposal.Dir, "expert"))
	writeFixture(t, filepath.Join(worker.Dir, "expert", "README.md"), "changed original", 0600)
	f = formFor(a)
	f.Set("model", "fixture/model")
	f.Set("change", "fix")
	if serveTest(a, "POST", "/jobs/"+analysis.ID+"/prepare-fix", f).Code != 409 {
		t.Fatal("stale baseline authored")
	}
	f = formFor(a)
	f.Set("proposal-hash", hash)
	f.Set("review-consent", "yes")
	if serveTest(a, "POST", "/jobs/"+proposal.ID+"/save-revision", f).Code != 409 {
		t.Fatal("stale baseline saved")
	}
	os.Symlink(secret, filepath.Join(proposal.Dir, "expert", "secret"))
	if _, _, err := definitionSnapshot(a.cfg.Data, filepath.Join(proposal.Dir, "expert")); err == nil {
		t.Fatal("linked definition accepted")
	}
}

func TestDefinitionRootLinkAndModelGate(t *testing.T) {
	a, worker, run := analysisFixture(t)
	f := formFor(a)
	f.Set("model", "fixture/model")
	a.cfg.AllowBuild = false
	if serveTest(a, "POST", "/jobs/"+run.ID+"/investigate", f).Code != 422 {
		t.Fatal("disabled model analysis admitted")
	}
	a.cfg.AllowBuild = true
	dir, err := a.workspace()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(worker.Dir, "expert"), filepath.Join(dir, "expert")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := definitionSnapshot(a.cfg.Data, filepath.Join(dir, "expert")); err == nil {
		t.Fatal("redirected definition root accepted")
	}
}
