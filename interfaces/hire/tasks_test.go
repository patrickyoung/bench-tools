package main

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "delivery" {
		os.Exit(deliveryProcess(os.Args[2]))
	}
	if len(os.Args) == 3 && os.Args[1] == "task" {
		os.Exit(taskProcess(os.Args[2]))
	}
	os.Exit(m.Run())
}
func taskFixture(t *testing.T) *app {
	t.Helper()
	a := testApp(t)
	a.cfg.AllowBuild = true
	a.cfg.AllowRun = true
	a.cfg.Model = "fixture/model"
	tools := t.TempDir()
	a.cfg.Agent = filepath.Join(tools, "agent")
	a.cfg.Hire = filepath.Join(tools, "hire")
	a.cfg.Moniker = filepath.Join(tools, "moniker")
	writeFixture(t, a.cfg.Moniker, `#!/usr/bin/python3
import json,uuid
n=uuid.uuid4().hex
print(json.dumps(dict(id=n,name='Cosmic Otter Crew '+n[:6],slug='cosmic-otter-crew-'+n[:6],theme='playful')))
`, 0700)
	// Executable fixtures implement only the public process seams; never call a model.
	writeFixture(t, a.cfg.Agent, `#!/usr/bin/python3
import hashlib,json,os,sys
from pathlib import Path
p=Path('request.json')
if p.exists():
 raw=p.read_bytes();r=json.loads(raw);h=hashlib.sha256(raw).hexdigest()
 if r['stage']=='plan':
  question='What date should I use?' if r['message']=='need question' else ''
  target=('previous' if any(c['key']=='previous' for c in r['catalog']) else ('new:team' if 'team' in r['message'] else 'new:worker'))
  reply=dict(request_sha256=h,message='I can help.',question=question,title='A useful document',target='' if question else target,brief='Create the requested document.',inputs=[dict(name='facts.txt',text=r['message'],binding='DOCUMENT_INPUT')])
  if r.get('selection_version')==1:
   reviewed=[dict(target=c['key'],fit=('adapt' if target=='adapt:'+c['key'] else 'reuse' if target==c['key'] else 'none'),reason='Fixture capability assessment.') for c in r['catalog'] if not c['key'].startswith('adapt:')]
   roles=[dict(role=k,target='new:worker',responsibility='Prepare '+k) for k in ['writer','reviewer']] if target=='new:team' else []
   reply['selection']=dict(reviewed=[] if question else reviewed,gap='' if question else 'Fixture missing capability.',roles=[] if question else roles)
 else: reply=dict(request_sha256=h,message='Here is your work.' if r['execution']['exit_code']==0 else 'I need more information to finish.')
 Path('response.json').write_text(json.dumps(reply));sys.exit(0)
assert Path(os.environ['DOCUMENT_INPUT']).parent != Path.cwd()
assert Path('facts.txt').read_text()==Path(os.environ['DOCUMENT_INPUT']).read_text()
assert '-B' in sys.argv and sys.argv[sys.argv.index('-turns')+1]=='50'
Path('result.md').write_text('Finished: '+Path('facts.txt').read_text())
Path('draft.html').write_text('<script>alert(1)</script>draft')
Path('hire-status.json').write_text(json.dumps(dict(done='Wrote the document.',now='',next='',blocked='')))
`, 0700)
	writeFixture(t, a.cfg.Hire, `#!/bin/sh
set -eu
mkdir -p expert/bin
printf 'Instructions\n' > expert/AGENTS.md
printf '# Guide\nRead facts.txt and write result.md.\n' > expert/README.md
printf '#!/bin/sh\ntest -s result.md\n' > expert/bin/check
printf '#!/bin/sh\nprintf "Team result" > "$BENCH_TASK_WORK/result.md"\n' > expert/bin/task
chmod 700 expert/bin/check expert/bin/task
if [ -d expert/agents ]; then
 for role in writer reviewer; do
  mkdir -p "expert/agents/$role/bin"
  cp expert/AGENTS.md expert/README.md "expert/agents/$role/"
  cp expert/bin/check "expert/agents/$role/bin/check"
 done
fi
`, 0700)
	writeFixture(t, filepath.Join(tools, "cage"), "#!/bin/sh\nwhile [ \"$1\" != -- ]; do shift; done\nshift\nexec \"$@\"\n", 0700)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
	a.cfg.Assistant = t.TempDir()
	for _, f := range []string{"AGENTS.md", "README.md", "bin/check"} {
		writeFixture(t, filepath.Join(a.cfg.Assistant, "task", f), "fixture profile", 0700)
	}
	return a
}
func taskSubmit(t *testing.T, a *app, thread, message string) taskTurn {
	t.Helper()
	f := formFor(a)
	f.Set("thread", thread)
	f.Set("message", message)
	w := serveTest(a, "POST", "/work", f)
	if w.Code != 303 {
		t.Fatalf("send failed: %d %s", w.Code, w.Body.String())
	}
	id := strings.TrimPrefix(w.Header().Get("Location"), "/work/")
	turns := a.taskTurns(id)
	j := awaitJob(t, a.jobs, turns[len(turns)-1].Job.ID)
	return taskTurn{Job: j, Record: turns[len(turns)-1].Record, Result: a.taskResult(j)}
}
func TestWorkConversationBuildExecuteRefine(t *testing.T) {
	a := taskFixture(t)
	first := taskSubmit(t, a, "", "Write a welcome note")
	if first.Job.State != "completed" || len(first.Result.Artifacts) != 2 {
		t.Fatalf("no finished work: %+v\n%s", first, a.jobs.Log(first.Job.ID, "stderr"))
	}
	original, _ := os.ReadFile(filepath.Join(filepath.Dir(first.Job.Dir), "deliverables", "result.md"))
	second := taskSubmit(t, a, first.Record.Thread, "Make it warmer")
	if second.Job.State != "completed" || second.Job.Dir == first.Job.Dir {
		t.Fatal("refinement failed")
	}
	old, _ := os.ReadFile(filepath.Join(filepath.Dir(first.Job.Dir), "deliverables", "result.md"))
	if string(old) != string(original) {
		t.Fatal("old output overwritten")
	}
	page := serveTest(a, "GET", "/work/"+first.Record.Thread, nil).Body.String()
	for _, text := range []string{"Write a welcome note", "Make it warmer", "Here is your work", "Download"} {
		if !strings.Contains(page, text) {
			t.Fatalf("missing %s", text)
		}
	}
	for i, f := range first.Result.Artifacts {
		w := serveTest(a, "GET", "/work/jobs/"+first.Job.ID+"/files/"+string(rune('0'+i))+"?preview=1", nil)
		if w.Code != 200 {
			t.Fatal("missing download")
		}
		if f.Name == "draft.html" && !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") {
			t.Fatal("active HTML inline")
		}
	}
	if w := serveTest(a, "GET", "/work/jobs/"+first.Job.ID+"/files/99", nil); w.Code != 404 {
		t.Fatal("unlisted artifact")
	}
	if err := a.jobs.Close(); err != nil {
		t.Fatal(err)
	}
	m, e := newJobManager(a.cfg.Data)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	restarted, e := newApp(a.cfg, a.cat, m, "127.0.0.1:8787")
	if e != nil {
		t.Fatal(e)
	}
	if len(restarted.taskTurns(first.Record.Thread)) != 2 {
		t.Fatal("lost conversation")
	}
}
func TestWorkQuestionTeamAndDuplicate(t *testing.T) {
	a := taskFixture(t)
	question := taskSubmit(t, a, "", "need question")
	if question.Result.Question == "" || question.Result.Expert != "" {
		t.Fatal("question incorrectly dispatched")
	}
	f := formFor(a)
	f.Set("message", "Build a team document")
	w := serveTest(a, "POST", "/work", f)
	if w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	if again := serveTest(a, "POST", "/work", f); again.Header().Get("Location") != w.Header().Get("Location") {
		t.Fatal("duplicate conversation")
	}
	turns := a.taskTurns(strings.TrimPrefix(w.Header().Get("Location"), "/work/"))
	j := awaitJob(t, a.jobs, turns[0].Job.ID)
	result := a.taskResult(j)
	if j.State != "completed" || result.Kind != "team" || len(result.Artifacts) != 1 {
		t.Fatalf("team adapter failed: %+v %s", result, a.jobs.Log(j.ID, "stderr"))
	}
}
func TestTaskPlanValidationAndArtifactBoundary(t *testing.T) {
	req := []byte(`{}`)
	p := taskPlan{Hash: digestText(req), Message: "Ready", Title: "Work", Target: "new:worker", Brief: "Do it", Inputs: []taskInput{}}
	b, _ := json.Marshal(p)
	if _, e := decodeTaskPlan(b, req, nil); e != nil {
		t.Fatal(e)
	}
	for _, binding := range []string{"RUN_INPUT", "AGENT_INPUT", "PATH", "x_INPUT"} {
		p.Inputs = []taskInput{{Name: "facts.txt", Text: "data", Binding: binding}}
		b, _ = json.Marshal(p)
		if _, e := decodeTaskPlan(b, req, nil); e == nil {
			t.Fatal("unsafe binding", binding)
		}
	}
	p.Inputs = []taskInput{{Name: "../escape", Text: "data"}}
	b, _ = json.Marshal(p)
	if _, e := decodeTaskPlan(b, req, nil); e == nil {
		t.Fatal("unsafe filename")
	}
	work := t.TempDir()
	dest := t.TempDir()
	private := filepath.Join(t.TempDir(), "secret.txt")
	writeFixture(t, private, "private", 0600)
	_ = os.Symlink(private, filepath.Join(work, "result.txt"))
	if _, e := collectTaskArtifacts(work, dest, nil); e == nil {
		t.Fatal("artifact symlink followed")
	}
}
func TestTaskProfileContract(t *testing.T) {
	check, e := filepath.Abs("../../workers/bench-hire/expert/task/bin/check")
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	req := map[string]any{"stage": "plan", "message": "Make something", "source": "/selected", "now": "2026-09-26T12:00:00Z", "history": []any{}, "catalog": []any{}}
	raw, _ := json.Marshal(req)
	writeFixture(t, filepath.Join(dir, "request.json"), string(raw), 0600)
	reply := taskPlan{Hash: digestText(raw), Message: "Of course", Question: "What date?", Inputs: []taskInput{}}
	b, _ := json.Marshal(reply)
	writeFixture(t, filepath.Join(dir, "response.json"), string(b), 0600)
	cmd := exec.Command("python3", check)
	cmd.Dir = dir
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("profile rejected question %s %v", out, e)
	}
	if _, e := decodeTaskPlan(b, raw, nil); e != nil {
		t.Fatal("UI disagrees with companion", e)
	}
}
func TestTaskHTTPEnablementAndCSRF(t *testing.T) {
	a := taskFixture(t)
	f := url.Values{"csrf": {"wrong"}, "nonce": {randomID()}, "message": {"Do work"}}
	if w := serveTest(a, "POST", "/work", f); w.Code != 403 {
		t.Fatal("CSRF bypass")
	}
	f.Set("csrf", a.csrf)
	a.cfg.AllowRun = false
	if w := serveTest(a, "POST", "/work", f); w.Code != 403 || len(a.jobs.List()) != 0 {
		t.Fatal("run enablement bypass")
	}
}

func TestTaskQuestionRetainsWorkAndUnfinishedBuildContinues(t *testing.T) {
	a := taskFixture(t)
	first := taskSubmit(t, a, "", "Create a note")
	question := taskSubmit(t, a, first.Record.Thread, "need question")
	if question.Result.Question == "" {
		t.Fatal("missing question")
	}
	third := taskSubmit(t, a, first.Record.Thread, "The date is October 10")
	if third.Record.Previous != filepath.Dir(first.Job.Dir) || third.Job.State != "completed" {
		t.Fatal("question lost earlier work")
	}
	first.Result.Prepared = false
	if err := saveTaskJSON(filepath.Join(filepath.Dir(third.Job.Dir), "result.json"), first.Result); err != nil {
		t.Fatal(err)
	}
	next := taskSubmit(t, a, first.Record.Thread, "Continue preparing")
	if !strings.Contains(a.jobs.Log(next.Job.ID, "stderr"), "Starting hire") || !next.Result.Prepared {
		t.Fatal("unfinished expertise was run without preparation")
	}
	found := false
	for _, c := range a.taskChoices() {
		if c.Path == next.Result.Expert {
			found = true
		}
	}
	if !found {
		t.Fatal("prepared specialist was not retained for reuse")
	}
}

func TestTaskExplicitMessageAfterInterruptedOutcome(t *testing.T) {
	a := taskFixture(t)
	first := taskSubmit(t, a, "", "Create a note")
	a.jobs.mu.Lock()
	j := a.jobs.jobs[first.Job.ID]
	j.State = "unknown"
	j.ExitCode = nil
	a.jobs.jobs[j.ID] = j
	a.jobs.mu.Unlock()
	before := len(a.jobs.List())
	if w := serveTest(a, "GET", "/work/"+first.Record.Thread, nil); w.Code != 200 || len(a.jobs.List()) != before {
		t.Fatal("reading retried interrupted work")
	}
	next := taskSubmit(t, a, first.Record.Thread, "Make a fresh local version")
	if next.Job.State != "completed" || next.Job.Dir == first.Job.Dir {
		t.Fatal("explicit message could not start fresh work")
	}
	found := false
	for _, msg := range next.Record.History {
		if strings.Contains(msg.Text, "outcome was not observed") {
			found = true
		}
	}
	if !found {
		t.Fatal("lost uncertainty warning")
	}
}
