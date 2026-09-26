package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskCommandPreservesForcedKill(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ignore-interrupt")
	writeFixture(t, file, "#!/usr/bin/python3\nimport signal,time\nsignal.signal(signal.SIGINT,signal.SIG_IGN)\ntime.sleep(10)\n", 0700)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	code, _, _ := taskCommand(ctx, t.TempDir(), nil, file)
	if code != 137 {
		t.Fatalf("forced kill was reported as %d instead of 137", code)
	}
}

func TestTaskContinuationEarlyFailureKeepsPreviousDownloads(t *testing.T) {
	a := taskFixture(t)
	raw, _ := os.ReadFile(a.cfg.Agent)
	writeFixture(t, a.cfg.Agent, string(raw)+"\nsys.exit(2)\n", 0700)
	first := taskSubmit(t, a, "", "Write a note")
	if first.Job.State != "unfinished" || len(first.Result.Artifacts) == 0 {
		t.Fatal("fixture did not save partial work")
	}
	if err := os.Rename(first.Result.Expert, first.Result.Expert+"-unavailable"); err != nil {
		t.Fatal(err)
	}
	next := continueFixtureTask(t, a, first)
	if len(next.Result.Artifacts) != 0 {
		t.Fatal("new empty snapshot advertised previous files")
	}
	if w := serveTest(a, "GET", "/work/jobs/"+first.Job.ID+"/files/0", nil); w.Code != 200 {
		t.Fatal("original download was lost")
	}
	if err := os.Rename(first.Result.Expert+"-unavailable", first.Result.Expert); err != nil {
		t.Fatal(err)
	}
	followup := taskSubmit(t, a, first.Record.Thread, "Use the saved draft")
	if followup.Record.Previous != filepath.Dir(first.Job.Dir) {
		t.Fatal("refinement lost the earlier files after an empty attempt")
	}
}

func continueFixtureTask(t *testing.T, a *app, prior taskTurn) taskTurn {
	t.Helper()
	w := serveTest(a, "POST", "/work/jobs/"+prior.Job.ID+"/continue", formFor(a))
	if w.Code != 303 {
		t.Fatalf("continue %d: %s", w.Code, w.Body.String())
	}
	turns := a.taskTurns(prior.Record.Thread)
	last := turns[len(turns)-1]
	last.Job = awaitJob(t, a.jobs, last.Job.ID)
	last.Result = a.taskResult(last.Job)
	return last
}
func TestTaskContinueUsesCheckpointAndKeepsEachAttempt(t *testing.T) {
	a := taskFixture(t)
	raw, err := os.ReadFile(a.cfg.Agent)
	if err != nil {
		t.Fatal(err)
	}
	old := "Path('result.md').write_text('Finished: '+Path('facts.txt').read_text())"
	replacement := `assert sys.argv[sys.argv.index('-checkpoint')+1]=='task'
assert '-net' not in sys.argv and '-no-cage' not in sys.argv
step=Path('saved-step')
n=int(step.read_text())+1 if step.exists() else 1
step.write_text(str(n))
Path('result.md').write_text('Partial '+str(n))
if n<3: sys.exit(2)
Path('result.md').write_text('Finished from saved step '+str(n))`
	if !strings.Contains(string(raw), old) {
		t.Fatal("fixture changed")
	}
	writeFixture(t, a.cfg.Agent, strings.Replace(string(raw), old, replacement, 1), 0700)
	first := taskSubmit(t, a, "", "Create a note")
	if first.Job.State != "unfinished" {
		t.Fatalf("first attempt: %+v", first.Job)
	}
	root := filepath.Dir(first.Job.Dir)
	original, _ := os.ReadFile(filepath.Join(root, "deliverables/result.md"))
	page := serveTest(a, "GET", "/work/"+first.Record.Thread, nil).Body.String()
	if !strings.Contains(page, "Continue working →") {
		t.Fatal("no recovery control")
	}
	second := continueFixtureTask(t, a, first)
	if second.Job.State != "unfinished" || second.Record.Resume.Root != root {
		t.Fatal("did not preserve checkpoint workspace")
	}
	third := continueFixtureTask(t, a, second)
	if third.Job.State != "completed" {
		t.Fatalf("continuation failed: %s", a.jobs.Log(third.Job.ID, "stderr"))
	}
	final, _ := os.ReadFile(filepath.Join(filepath.Dir(third.Job.Dir), "deliverables/result.md"))
	if string(final) != "Finished from saved step 3" {
		t.Fatalf("saved work lost: %q", final)
	}
	after, _ := os.ReadFile(filepath.Join(root, "deliverables/result.md"))
	if string(original) != string(after) {
		t.Fatal("earlier output changed")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(second.Job.Dir), "plan")); !os.IsNotExist(err) {
		t.Fatal("continuation replanned")
	}
	if first.Job.Dir == second.Job.Dir || second.Job.Dir == third.Job.Dir {
		t.Fatal("attempt records collided")
	}
	page = serveTest(a, "GET", "/work/"+first.Record.Thread, nil).Body.String()
	if strings.Contains(page, "/continue\"") {
		t.Fatal("old continue controls remained after success")
	}
	before := len(a.jobs.List())
	if w := serveTest(a, "POST", "/work/jobs/"+first.Job.ID+"/continue", formFor(a)); w.Code != 409 {
		t.Fatal("stale attempt restarted")
	}
	if w := serveTest(a, "POST", "/work/jobs/"+third.Job.ID+"/continue", formFor(a)); w.Code != 409 {
		t.Fatal("completed attempt restarted")
	}
	if len(a.jobs.List()) != before {
		t.Fatal("rejected action started a job")
	}
}

func TestTaskContinueBuildUsesExistingAuthoring(t *testing.T) {
	a := taskFixture(t)
	raw, err := os.ReadFile(a.cfg.Hire)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte("\nif [ ! -f built-once ]; then touch built-once; exit 2; fi\n")...)
	writeFixture(t, a.cfg.Hire, string(raw), 0700)
	first := taskSubmit(t, a, "", "Create a note")
	if first.Job.State != "unfinished" || first.Result.Prepared {
		t.Fatal("expected unfinished authoring")
	}
	next := continueFixtureTask(t, a, first)
	if next.Job.State != "completed" || !next.Result.Prepared {
		t.Fatalf("build continuation: %+v %s", next.Job, a.jobs.Log(next.Job.ID, "stderr"))
	}
	if next.Result.Expert != first.Result.Expert {
		t.Fatal("recreated definition instead of continuing")
	}
}

func TestTaskUncertainAttemptOffersFreshRecovery(t *testing.T) {
	a := taskFixture(t)
	first := taskSubmit(t, a, "", "Create a note")
	a.jobs.mu.Lock()
	j := a.jobs.jobs[first.Job.ID]
	j.State = "unknown"
	j.ExitCode = nil
	a.jobs.jobs[j.ID] = j
	a.jobs.mu.Unlock()
	before := len(a.jobs.List())
	page := serveTest(a, "GET", "/work/"+first.Record.Thread, nil).Body.String()
	if !strings.Contains(page, "Try again with saved work") || strings.Contains(page, "/continue\"") {
		t.Fatal("uncertain work offered replay or no recovery")
	}
	if w := serveTest(a, "POST", "/work/jobs/"+first.Job.ID+"/continue", formFor(a)); w.Code != 409 {
		t.Fatal("uncertain action replayed")
	}
	if len(a.jobs.List()) != before {
		t.Fatal("reading or denied continuation ran work")
	}
	f := formFor(a)
	f.Set("thread", first.Record.Thread)
	f.Set("retry-job", first.Job.ID)
	f.Set("message", "Continue from the saved work")
	w := serveTest(a, "POST", "/work", f)
	if w.Code != 303 {
		t.Fatalf("fresh recovery denied: %d %s", w.Code, w.Body.String())
	}
	turns := a.taskTurns(first.Record.Thread)
	next := turns[len(turns)-1]
	job := awaitJob(t, a.jobs, next.Job.ID)
	if job.State != "completed" || next.Record.Resume != nil || next.Record.Previous != filepath.Dir(first.Job.Dir) {
		t.Fatal("fresh recovery did not preserve earlier results")
	}
	f = formFor(a)
	f.Set("thread", first.Record.Thread)
	f.Set("retry-job", first.Job.ID)
	f.Set("message", "Continue")
	if w := serveTest(a, "POST", "/work", f); w.Code != 409 {
		t.Fatal("stale retry admitted")
	}
}
