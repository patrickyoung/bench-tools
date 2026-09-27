package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func coordinatorFixture(t *testing.T) *app {
	t.Helper()
	a := taskFixture(t)
	a.cfg.Data, _ = filepath.EvalSymlinks(a.cfg.Data)
	a.cfg.GoalMode = true
	a.cfg.TeamCoordinator = filepath.Join(t.TempDir(), "manage")
	a.cfg.TeamQueue = filepath.Join(a.cfg.Data, "workspaces", "team-runs")
	if err := os.MkdirAll(a.cfg.TeamQueue, 0700); err != nil {
		t.Fatal(err)
	}
	// Real public client subprocess fixture: no Agent execution, no paid model.
	writeFixture(t, a.cfg.TeamCoordinator, `#!/usr/bin/python3
import hashlib,json,os,sys
from pathlib import Path
args=sys.argv[1:];assert args.pop(0)=='team';op=args.pop(0)
log=Path(__file__).with_name('calls');log.open('a').write(op+'\n')
def arg(k):return args[args.index(k)+1]
if op=='validate':
 d=Path(arg('-definition'));r=json.loads((d/'team.json').read_text());assert r['schema']=='bench.team/v1'
 print(json.dumps(dict(schema='bench.team.validation/v1',valid=True,steps=[dict(id=s['id'],worker=s['worker']) for s in r['steps']])));sys.exit(0)
if op=='preflight':
 root=Path(arg('-C'));root.mkdir(parents=True,exist_ok=True)
 result=dict(schema='bench.team.preflight/v1',status='passed',coverage='first_prepare_only',run=str(root),request_id=arg('-id'),message='First preparation passed; no specialist ran.',repairable=False,execution=dict(state='done',exit=0,signal=0))
 control=Path(__file__).with_name('preflight-control.json')
 if control.exists():
  values=json.loads(control.read_text());result.update(values.pop(0));control.write_text(json.dumps(values)) if values else control.unlink()
 print(json.dumps(result));sys.exit(0 if result['status']=='passed' else 125 if result['status']=='unknown' else 2)
if op=='admit':
 root=Path(arg('-C'));root.mkdir(parents=True,exist_ok=True)
 state=dict(schema='bench.team.status/v1',run=str(root),request_id=arg('-id'),status='ready',phase='prepare',worker='writer',revision=0,question='',question_id=None,resumable=True,message='Team admitted; waiting for the host worker.',attempts_used=0,attempts_limit=48,messages=[])
 (root/'status.json').write_text(json.dumps(state));(root/'argv.json').write_text(json.dumps(args));print(json.dumps(state));sys.exit(0)
root=Path(args[0]);state=json.loads((root/'status.json').read_text())
if op=='status':print(json.dumps(state))
elif op=='result':
 state['schema']='bench.team.result/v1';state['files']=json.loads((root/'files.json').read_text());print(json.dumps(state))
elif op=='events':pass
elif op=='send':
 msg=json.load(sys.stdin);dest=root/(msg['id']+'.json')
 if dest.exists():
  assert json.loads(dest.read_text())==msg,'message ID has different bytes'
 else:
  assert msg['revision']==state['revision'],'stale goal revision'
  assert msg['question_id']==state['question_id'],'stale question'
  assert state['status'] not in ['unknown','cancelled','complete'],'unsafe state'
  dest.write_text(json.dumps(msg));state['messages'].append(dict(id=msg['id'],status='queued'));(root/'status.json').write_text(json.dumps(state))
 print(json.dumps(dict(id=msg['id'],status='queued',run=str(root))))
elif op=='cancel':
 state['status']='cancelling';state['message']='Cancellation requested.';(root/'status.json').write_text(json.dumps(state));print(json.dumps(dict(status='cancellation_requested',run=str(root))))
else:raise AssertionError('UI may not drive '+op)
`, 0700)
	// Preserve the fixture members, but replace generated runtime with a recipe.
	raw, _ := os.ReadFile(a.cfg.Hire)
	writeFixture(t, a.cfg.Hire, string(raw)+`python3 - <<'PY'
import json
from pathlib import Path
steps=[]
for role in ['writer','reviewer']:
 steps.append(dict(id=role,worker='agents/'+role,goal='Do '+role,prepare='bin/prepare-'+role,inspect='bin/inspect-'+role,input_limits=dict(files=64,bytes=104857600,file_bytes=26214400)))
 for kind in ['prepare','inspect']:
  f=Path('expert/bin/'+kind+'-'+role);f.write_text('#!/bin/sh\nexit 0\n');f.chmod(0o700)
Path('expert/team.json').write_text(json.dumps(dict(schema='bench.team/v1',title='Saved team',steps=steps,check='bin/check')))
Path('expert/bin/task').unlink()
PY
`, 0700)
	return a
}
func coordinatorRun(t *testing.T, a *app) (taskTurn, taskTeamReference) {
	t.Helper()
	turn := taskSubmit(t, a, "", "Build a team for a checked document")
	if turn.Job.State != "completed" {
		t.Fatalf("admission failed: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
	}
	ref, e := readTaskTeamReference(turn.Job, turn.Record)
	if e != nil {
		t.Fatal(e)
	}
	return turn, ref
}
func setCoordinatorState(t *testing.T, ref taskTeamReference, patch map[string]any) {
	t.Helper()
	p := filepath.Join(ref.Run, "status.json")
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	v := map[string]any{}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	for k, x := range patch {
		v[k] = x
	}
	if e = saveTaskJSON(p, v); e != nil {
		t.Fatal(e)
	}
}
func completeCoordinator(t *testing.T, ref taskTeamReference) {
	t.Helper()
	p := filepath.Join(ref.Run, "delivery", "files", "result.md")
	writeFixture(t, p, "Checked result", 0600)
	files := []map[string]any{{"path": p, "name": "result.md", "kind": "deliverable", "size": 14, "sha256": digestText([]byte("Checked result"))}}
	if e := saveTaskJSON(filepath.Join(ref.Run, "files.json"), files); e != nil {
		t.Fatal(e)
	}
	setCoordinatorState(t, ref, map[string]any{"status": "complete", "phase": "final", "message": "All selected contributions passed their checks.", "resumable": false})
}
func TestCoordinatorAdmissionDoesNotExecuteOrReviewTeam(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	turns := a.taskTurns(turn.Record.Thread)
	if len(turns) != 1 || !turns[0].Job.Active() || turns[0].Result.GoalStatus != "ready" {
		t.Fatalf("admission confused with completion: %+v", turns)
	}
	root := filepath.Dir(turn.Job.Dir)
	for _, name := range []string{"goal-attempts/execute", "present", "goal-rounds"} {
		if _, e := os.Stat(filepath.Join(root, name)); !os.IsNotExist(e) {
			t.Fatal("second execution/review loop created", name)
		}
	}
	if _, e := os.Stat(filepath.Join(root, "authoring", "expert", "bin/task")); !os.IsNotExist(e) {
		t.Fatal("generated orchestration survived")
	}
	b, _ := os.ReadFile(filepath.Join(root, "build.txt"))
	if !strings.Contains(string(b), "Do not author bin/task") || strings.Contains(string(b), "task-result.json") {
		t.Fatal("old runtime contract still authored")
	}
	// Exact request identity and selected queue persist before any worker starts.
	b, _ = os.ReadFile(filepath.Join(ref.Run, "argv.json"))
	if !strings.Contains(string(b), ref.ID) || strings.Contains(string(b), a.cfg.Agent) {
		t.Fatal("wrong identity or host tools passed to runtime", string(b))
	}
	if _, e := a.taskResumeInfo(turn.Job, turn.Record); e == nil {
		t.Fatal("UI can resume coordinator")
	}
}
func TestCoordinatorSurvivesUIRestartWithoutReplay(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	before, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
	if e := a.jobs.Close(); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
	if string(before) != string(after) {
		t.Fatal("UI shutdown called coordinator")
	}
	jobs, e := newJobManager(a.cfg.Data)
	if e != nil {
		t.Fatal(e)
	}
	defer jobs.Close()
	recovered, e := newApp(a.cfg, a.cat, jobs, "127.0.0.1:8787")
	if e != nil {
		t.Fatal(e)
	}
	setCoordinatorState(t, ref, map[string]any{"status": "running", "phase": "agent", "message": "Writer is running."})
	turns := recovered.taskTurns(turn.Record.Thread)
	if !turns[0].Job.Active() || turns[0].Progress != "Writer is running." {
		t.Fatalf("lost independent run %+v", turns[0])
	}
	log, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
	if strings.Count(string(log), "admit\n") != 1 || strings.Contains(string(log), "resume\n") || strings.Contains(string(log), "work\n") {
		t.Fatal("replayed run", string(log))
	}
}
func TestCoordinatorBoundReplyDuplicateConflictAndCancellation(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	setCoordinatorState(t, ref, map[string]any{"status": "needs_input", "question": "Which date?", "question_id": "q1", "revision": 2, "message": "Waiting for the date."})
	page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
	if !strings.Contains(page, `name="team-question" value="q1"`) || !strings.Contains(page, `name="team-revision" value="2"`) {
		t.Fatal("question binding absent")
	}
	f := formFor(a)
	f.Set("team-revision", "2")
	f.Set("team-question", "q1")
	f.Set("message", "October 4")
	path := "/work/jobs/" + turn.Job.ID + "/message"
	for i := 0; i < 2; i++ {
		w := serveTest(a, "POST", path, f)
		if w.Code != 303 {
			t.Fatalf("reply %d %s", w.Code, w.Body.String())
		}
	}
	notes, e := readTaskMessages(filepath.Dir(turn.Job.Dir))
	if e != nil || len(notes) != 1 {
		t.Fatal("duplicate UI note", notes, e)
	}
	f.Set("message", "October 9")
	if w := serveTest(a, "POST", path, f); w.Code != 409 {
		t.Fatal("conflicting ID accepted", w.Code)
	}
	f = formFor(a)
	f.Set("team-revision", "1")
	f.Set("team-question", "q1")
	f.Set("message", "old answer")
	if w := serveTest(a, "POST", path, f); w.Code != 409 {
		t.Fatal("stale revision accepted")
	}
	f.Set("team-revision", "2")
	f.Set("team-question", "q0")
	if w := serveTest(a, "POST", path, f); w.Code != 409 {
		t.Fatal("stale question accepted")
	}
	if w := serveTest(a, "POST", "/work/jobs/"+turn.Job.ID+"/stop", formFor(a)); w.Code != 303 {
		t.Fatal("cancel failed", w.Body.String())
	}
	if a.jobs.active != nil {
		t.Fatal("UI started execution on reply/cancel")
	}
}
func TestCoordinatorAttachmentsUseSavedPaths(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	f := formFor(a)
	f.Set("team-revision", "0")
	f.Set("message", "Use this version")
	w := uploadTask(t, a, "/work/jobs/"+turn.Job.ID+"/message", f, uploadFixture{"updated.txt", []byte("new source")})
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	raw, e := os.ReadFile(filepath.Join(ref.Run, f.Get("nonce")+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var payload struct{ Attachments []struct{ Path, Name string } }
	if json.Unmarshal(raw, &payload) != nil || len(payload.Attachments) != 1 {
		t.Fatal("files not forwarded", string(raw))
	}
	selected := payload.Attachments[0]
	if selected.Name != "updated.txt" || !within(filepath.Join(a.cfg.Data, "attachments", turn.Record.Thread), selected.Path) {
		t.Fatal("unbound attachment", selected)
	}
}
func TestCoordinatorAcceptedResultImportsOnlyVerifiedDelivery(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	completeCoordinator(t, ref)
	turns := a.taskTurns(turn.Record.Thread)
	if turns[0].Result.GoalStatus != "complete" || len(turns[0].Result.Artifacts) != 1 {
		t.Fatalf("no delivery %+v", turns[0].Result)
	}
	if got := serveTest(a, "GET", "/work/jobs/"+turn.Job.ID+"/files/0", nil); got.Code != 200 || got.Body.String() != "Checked result" {
		t.Fatal("download", got.Code, got.Body.String())
	}
	writeFixture(t, filepath.Join(ref.Run, "delivery", "files", "result.md"), "tampered", 0600)
	turns = a.taskTurns(turn.Record.Thread)
	if turns[0].Result.GoalStatus != "delivery_pending" || !strings.Contains(turns[0].Result.Update.Blocked, "bytes changed") {
		t.Fatalf("tamper not detected %+v", turns[0].Result)
	}
}
func TestCoordinatorUnknownAndUnavailableNeverContinue(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	setCoordinatorState(t, ref, map[string]any{"status": "unknown", "message": "Worker effect is unknown.", "resumable": false, "execution": map[string]any{"state": "unknown", "exit": 125}})
	got := a.taskTurns(turn.Record.Thread)[0]
	if got.Job.State != "unknown" || got.Recovery != "" || got.Result.Code != 125 {
		t.Fatalf("unknown weakened %+v", got)
	}
	if w := serveTest(a, "POST", "/work/jobs/"+turn.Job.ID+"/continue", formFor(a)); w.Code != 409 {
		t.Fatal("unknown continued", w.Code)
	}
	os.Remove(filepath.Join(ref.Run, "status.json"))
	got = a.taskTurns(turn.Record.Thread)[0]
	if got.CoordinatorError == "" || got.Recovery != "" || got.Result.GoalStatus != "unavailable" {
		t.Fatalf("missing status misclassified %+v", got)
	}
}
func TestCoordinatorRecipeCannotDropSelectedRole(t *testing.T) {
	a := coordinatorFixture(t)
	raw, _ := os.ReadFile(a.cfg.Hire)
	raw = []byte(strings.Replace(string(raw), "steps=steps,check=", "steps=steps[:1],check=", 1))
	writeFixture(t, a.cfg.Hire, string(raw), 0700)
	turn := taskSubmit(t, a, "", "Build a team")
	if turn.Job.State == "completed" {
		t.Fatal("dropped selected member admitted")
	}
	log, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
	if strings.Contains(string(log), "admit\n") {
		t.Fatal("bad roster executed", string(log))
	}
}
func TestCoordinatorObserverOnlyQueuesDelivery(t *testing.T) {
	a := coordinatorFixture(t)
	_, ref := coordinatorRun(t, a)
	completeCoordinator(t, ref)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.observeTaskTeams(ctx); close(done) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("observer did not stop")
	}
	log, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
	if strings.Contains(string(log), "work\n") || strings.Contains(string(log), "resume\n") {
		t.Fatal("UI owns goal loop")
	}
}

func TestCoordinatorAttachmentNonceCannotReplaceBytes(t *testing.T) {
	a := coordinatorFixture(t)
	turn, _ := coordinatorRun(t, a)
	f := formFor(a)
	f.Set("team-revision", "0")
	f.Set("message", "Use this version")
	path := "/work/jobs/" + turn.Job.ID + "/message"
	if w := uploadTask(t, a, path, f, uploadFixture{"version.txt", []byte("first")}); w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := uploadTask(t, a, path, f, uploadFixture{"version.txt", []byte("second")}); w.Code != 409 {
		t.Fatal("same message replaced its file", w.Code)
	}
}
func TestCoordinatorProjectionKeepsPinnedExecutable(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	a.cfg.TeamCoordinator = filepath.Join(t.TempDir(), "replacement")
	writeFixture(t, a.cfg.TeamCoordinator, "#!/bin/sh\nexit 99\n", 0700)
	setCoordinatorState(t, ref, map[string]any{"status": "running", "message": "Original coordinator is still working."})
	got := a.taskTurns(turn.Record.Thread)[0]
	if got.Coordinator == nil || got.CoordinatorError != "" {
		t.Fatal("replaced admitted coordinator", got.CoordinatorError)
	}
}
func TestCoordinatorCanReusePreparedRecipe(t *testing.T) {
	a := coordinatorFixture(t)
	first, ref := coordinatorRun(t, a)
	completeCoordinator(t, ref)
	a.taskTurns(first.Record.Thread)
	// A follow-up chooses the previous team through the existing planner fixture.
	// Replacing Hire with failure proves the recipe is reused unchanged.
	writeFixture(t, a.cfg.Hire, "#!/bin/sh\nexit 98\n", 0700)
	next := taskSubmit(t, a, first.Record.Thread, "Edit the previous result")
	if next.Job.State != "completed" {
		t.Fatalf("valid existing recipe rebuilt: %+v %s", next.Result, a.jobs.Log(next.Job.ID, "stderr"))
	}
	if _, e := readTaskTeamReference(next.Job, next.Record); e != nil {
		t.Fatal(e)
	}
}
func TestCoordinatorResultRejectsUnboundPath(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	completeCoordinator(t, ref)
	outside := filepath.Join(a.cfg.Data, "outside.md")
	writeFixture(t, outside, "Checked result", 0600)
	files := []map[string]any{{"path": outside, "name": "result.md", "kind": "deliverable", "size": 14, "sha256": digestText([]byte("Checked result"))}}
	saveTaskJSON(filepath.Join(ref.Run, "files.json"), files)
	got := a.taskTurns(turn.Record.Thread)[0]
	if got.Result.GoalStatus != "delivery_pending" || len(got.Result.Artifacts) != 0 {
		t.Fatal("unbound file imported", got.Result)
	}
}

func TestCoordinatorClientBoundsBothProcessStreams(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "coordinator")
			writeFixture(t, path, "#!/usr/bin/python3\nimport sys\nsys."+stream+".write('x'*(2*1024*1024))\n", 0700)
			raw, code, err := taskTeamCommand(context.Background(), config{TeamCoordinator: path}, nil, "status", "unused")
			if err == nil || code != 125 || len(raw) != 0 || !strings.Contains(err.Error(), "bounds") {
				t.Fatalf("unbounded stream retained: code%d bytes%d err%v", code, len(raw), err)
			}
		})
	}
}
func TestCoordinatorShowsQueuedHostAndExactInputWait(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	got := a.taskTurns(turn.Record.Thread)[0]
	if !strings.Contains(got.Progress, "Waiting for the host worker") {
		t.Fatal("unserviced queue invisible", got.Progress)
	}
	setCoordinatorState(t, ref, map[string]any{"status": "needs_input", "question": "Which date?", "question_id": "q1", "message": "Waiting for date.", "resumable": true})
	got = a.taskTurns(turn.Record.Thread)[0]
	if got.Result.Code != 75 || got.Job.Active() {
		t.Fatalf("input wait misclassified %+v", got.Result)
	}
	page := serveTest(a, "GET", "/jobs/"+turn.Job.ID, nil).Body.String()
	if !strings.Contains(page, "Independent team status") || !strings.Contains(page, "Which date?") {
		t.Fatal("actual blocker absent from activity details")
	}
}

func TestCoordinatorQueuesPrivateDeliveryWithoutMemberWork(t *testing.T) {
	a := coordinatorFixture(t)
	a.cfg.Plonk = filepath.Join(t.TempDir(), "plonk")
	a.cfg.PlonkURL = "http://127.0.0.1:1"
	a.cfg.PlonkTokenFile = filepath.Join(t.TempDir(), "connection")
	writeFixture(t, a.cfg.PlonkTokenFile, "fixture", 0600)
	writeFixture(t, a.cfg.Plonk, `#!/usr/bin/python3
import json,sys
from pathlib import Path
assert sys.argv[1]=='publish'
Path(__file__).with_name('calls').open('a').write(json.dumps(sys.argv)+'\n')
print(json.dumps(dict(slug='private-result',versionId='version-1')))
`, 0700)
	turn, ref := coordinatorRun(t, a)
	completeCoordinator(t, ref)
	projected := a.taskTurns(turn.Record.Thread)[0]
	if projected.Result.GoalStatus != "delivery_pending" {
		t.Fatal("publication prematurely complete")
	}
	started, err := a.queueTeamDelivery(turn.Job, turn.Record, projected.Result)
	if err != nil || !started {
		t.Fatal(started, err)
	}
	var delivery Job
	for _, j := range a.jobs.List() {
		if j.Kind == "delivery" {
			delivery = j
			break
		}
	}
	delivery = awaitJob(t, a.jobs, delivery.ID)
	if delivery.State != "completed" {
		t.Fatal(delivery, a.jobs.Log(delivery.ID, "stderr"))
	}
	if a.deliveryState(turn.Record.Thread).TaskID != turn.Job.ID {
		t.Fatal("no private receipt")
	}
	if _, err = a.queueTeamDelivery(turn.Job, turn.Record, projected.Result); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.Plonk), "calls"))
	if strings.Count(string(calls), "\n") != 1 {
		t.Fatal("duplicate private publication", string(calls))
	}
	if got := a.taskTurns(turn.Record.Thread)[0]; got.Result.GoalStatus != "complete" {
		t.Fatal("delivery receipt not reflected", got.Result)
	}
}
func TestCoordinatorPreparationRepairBindsNewSnapshot(t *testing.T) {
	a := coordinatorFixture(t)
	hire, _ := os.ReadFile(a.cfg.Hire)
	writeFixture(t, a.cfg.Hire, string(hire)+"\nexit 2\n", 0700)
	first := taskSubmit(t, a, "", "Build a team")
	if first.Job.ExitCode == nil || *first.Job.ExitCode != 2 {
		t.Fatalf("fixture not parked %+v", first.Result)
	}
	writeFixture(t, a.cfg.Hire, string(hire), 0700)
	w := serveTest(a, "POST", "/work/jobs/"+first.Job.ID+"/continue", formFor(a))
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	turns := a.taskTurns(first.Record.Thread)
	next := turns[len(turns)-1]
	job := awaitJob(t, a.jobs, next.Job.ID)
	rec, err := a.taskRecord(job)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "completed" {
		t.Fatal(job, a.jobs.Log(job.ID, "stderr"))
	}
	ref, err := readTaskTeamReference(job, rec)
	if err != nil {
		t.Fatal("new snapshot lacks coordinator binding", err)
	}
	if ref.ID != filepath.Base(filepath.Dir(first.Job.Dir)) {
		t.Fatal("repair changed original run identity")
	}
	if _, err = readTaskTeamReference(first.Job, first.Record); !os.IsNotExist(err) {
		t.Fatal("earlier snapshot rewritten", err)
	}
}

func TestCoordinatorIncludesUpdatesUploadedDuringPreparation(t *testing.T) {
	a := coordinatorFixture(t)
	hire, _ := os.ReadFile(a.cfg.Hire)
	gate := `printf ready > waiting-for-update
for i in $(seq 1 400); do [ -f release-preparation ] && break; sleep .02; done
test -f release-preparation
`
	writeFixture(t, a.cfg.Hire, strings.Replace(string(hire), "set -eu\n", "set -eu\n"+gate, 1), 0700)
	f := formFor(a)
	f.Set("message", "Build a team")
	w := serveTest(a, "POST", "/work", f)
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	thread := strings.TrimPrefix(w.Header().Get("Location"), "/work/")
	turn := a.taskTurns(thread)[0]
	author := filepath.Join(filepath.Dir(turn.Job.Dir), "authoring")
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if _, e := os.Stat(filepath.Join(author, "waiting-for-update")); e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, e := os.Stat(filepath.Join(author, "waiting-for-update")); e != nil {
		t.Fatal("preparation did not wait")
	}
	update := formFor(a)
	update.Set("message", "Use the revised title from this file.")
	w = uploadTask(t, a, "/work/jobs/"+turn.Job.ID+"/message", update, uploadFixture{"title.txt", []byte("New title")})
	if w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	writeFixture(t, filepath.Join(author, "release-preparation"), "go", 0600)
	job := awaitJob(t, a.jobs, turn.Job.ID)
	if job.State != "completed" {
		t.Fatal(job, a.jobs.Log(job.ID, "stderr"))
	}
	root := filepath.Dir(job.Dir)
	stages, _ := filepath.Glob(filepath.Join(root, "team-admission-*", "team-goal.txt"))
	if len(stages) != 1 {
		t.Fatal("missing admission stage", stages)
	}
	goal, _ := os.ReadFile(stages[0])
	if !strings.Contains(string(goal), "Use the revised title") {
		t.Fatal("preparation note lost", string(goal))
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(stages[0]), "team-originals", "attachment-01.txt"))
	if err != nil || string(source) != "New title" {
		t.Fatal("preparation attachment lost", string(source), err)
	}
}

func TestCoordinatorPreflightRepairsKnownFailureBeforeAdmission(t *testing.T) {
	a := coordinatorFixture(t)
	control := []map[string]any{{"status": "failed", "repairable": true, "message": "wrong step", "diagnostic": "assignment.step is an object", "execution": map[string]any{"state": "failed", "exit": 1, "signal": 0}}}
	if err := saveTaskJSON(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "preflight-control.json"), control); err != nil {
		t.Fatal(err)
	}
	turn, _ := coordinatorRun(t, a)
	root := filepath.Dir(turn.Job.Dir)
	repairs, _ := filepath.Glob(filepath.Join(root, "team-build-repairs", "*.txt"))
	if len(repairs) != 1 {
		t.Fatal("expected one bounded Hire repair", repairs)
	}
	correction, _ := os.ReadFile(repairs[0])
	if !strings.Contains(string(correction), "assignment.step is an object") {
		t.Fatal("repair missed real diagnostic")
	}
	// Re-entry semantics must reach both the initial public Hire goal and its
	// bounded correction. This guards prompt propagation, not model compliance.
	initial, err := os.ReadFile(filepath.Join(root, "build.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for name, goal := range map[string][]byte{"initial": initial, "repair": correction} {
		for _, instruction := range []string{
			"same retained member work directory",
			"Existing valid native inputs are not an error",
			"Text-only feedback is valid when amendments is empty",
			"stale output fails its native check",
			"files.name is the destination path",
		} {
			if !strings.Contains(string(goal), instruction) {
				t.Fatalf("%s Hire goal lost continuation contract: %s", name, instruction)
			}
		}
	}
	calls, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
	if strings.Count(string(calls), "preflight\n") != 2 || strings.Count(string(calls), "admit\n") != 1 || strings.Index(string(calls), "admit\n") < strings.LastIndex(string(calls), "preflight\n") {
		t.Fatal("admitted before successful preflight", string(calls))
	}
	receipts, _ := filepath.Glob(filepath.Join(root, "team-preflights", "*", "response.json"))
	if len(receipts) != 2 {
		t.Fatal("failed preflight evidence lost", receipts)
	}
}

func TestCoordinatorPreflightNeverRepairsUnsafeOrUnboundOutcomes(t *testing.T) {
	cases := []struct {
		name  string
		patch map[string]any
		code  int
	}{
		{"unknown", map[string]any{"status": "unknown", "repairable": false}, 125},
		{"interrupted", map[string]any{"status": "failed", "repairable": true, "execution": map[string]any{"state": "failed", "exit": 130, "signal": 0}}, 130},
		{"signal", map[string]any{"status": "failed", "repairable": true, "execution": map[string]any{"state": "failed", "exit": 137, "signal": 9}}, 137},
		{"missing-execution", map[string]any{"status": "failed", "repairable": true, "execution": nil}, 125},
		{"no-authority", map[string]any{"status": "failed", "repairable": false, "execution": map[string]any{"state": "failed", "exit": 2, "signal": 0}}, 125},
		{"unbound", map[string]any{"request_id": "other-request"}, 125},
		{"coverage", map[string]any{"coverage": "invented"}, 125},
		{"contradictory-success", map[string]any{"execution": map[string]any{"state": "failed", "exit": 130, "signal": 0}}, 125},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := coordinatorFixture(t)
			if err := saveTaskJSON(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "preflight-control.json"), []map[string]any{c.patch}); err != nil {
				t.Fatal(err)
			}
			turn := taskSubmit(t, a, "", "Build a team")
			if turn.Job.ExitCode == nil || *turn.Job.ExitCode != c.code {
				t.Fatalf("wrong stop: %+v %s", turn.Job, a.jobs.Log(turn.Job.ID, "stderr"))
			}
			calls, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
			if strings.Count(string(calls), "preflight\n") != 1 || strings.Contains(string(calls), "admit\n") {
				t.Fatal("unsafe outcome replayed", string(calls))
			}
			repairs, _ := filepath.Glob(filepath.Join(filepath.Dir(turn.Job.Dir), "team-build-repairs", "*.txt"))
			if len(repairs) != 0 {
				t.Fatal("unsafe outcome sent to Hire repair", repairs)
			}
		})
	}
}

func TestCoordinatorPreflightProofBindsAdmittedBytes(t *testing.T) {
	a := coordinatorFixture(t)
	turn, _ := coordinatorRun(t, a)
	root := filepath.Dir(turn.Job.Dir)
	stages, _ := filepath.Glob(filepath.Join(root, "team-admission-*", "team-goal.txt"))
	if len(stages) != 1 {
		t.Fatal("missing admission stage", stages)
	}
	inputs, goal := filepath.Join(filepath.Dir(stages[0]), "team-originals"), stages[0]
	var proof taskTeamPreflightProof
	b, _ := os.ReadFile(filepath.Join(root, "team-preflight.json"))
	if err := json.Unmarshal(b, &proof); err != nil {
		t.Fatal(err)
	}
	if err := verifyTaskTeamPreflight(root, proof.Definition, inputs, goal); err != nil {
		t.Fatal(err)
	}
	if err := verifyTaskTeamPreflight(root, "changed", inputs, goal); err == nil {
		t.Fatal("changed definition accepted")
	}
	original, _ := os.ReadFile(goal)
	writeFixture(t, goal, string(original)+"changed", 0600)
	if err := verifyTaskTeamPreflight(root, proof.Definition, inputs, goal); err == nil {
		t.Fatal("changed goal accepted")
	}
	writeFixture(t, goal, string(original), 0600)
	writeFixture(t, filepath.Join(inputs, "late.txt"), "late update", 0600)
	if err := verifyTaskTeamPreflight(root, proof.Definition, inputs, goal); err == nil {
		t.Fatal("late input accepted without preflight")
	}
}

func TestCoordinatorShowsActualEscapedRuntimeDiagnostic(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	setCoordinatorState(t, ref, map[string]any{"status": "blocked", "message": "Prepare stopped with exact exit 1.", "diagnostic": "wrong step <script>alert(1)</script>", "resumable": false, "last_execution": map[string]any{"state": "failed", "exit": 1}})
	page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
	if !strings.Contains(page, "wrong step &lt;script&gt;") || strings.Contains(page, "<script>alert(1)</script>") || strings.Contains(page, "See why this run stopped") {
		t.Fatal("runtime failure absent, unsafe, or points to unrelated authoring log")
	}
}

func TestCoordinatorLateInputCanBeRecheckedWithoutReusingFailedStage(t *testing.T) {
	a := coordinatorFixture(t)
	script, _ := os.ReadFile(a.cfg.TeamCoordinator)
	marker := filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "change-input-once")
	writeFixture(t, marker, "yes", 0600)
	injected := ` marker=Path(__file__).with_name('change-input-once')
 if marker.exists():
  (Path(arg('-definition')).parent.parent/'work/execution/late.txt').write_text('Late supplied input')
  marker.unlink()
`
	writeFixture(t, a.cfg.TeamCoordinator, strings.Replace(string(script), " print(json.dumps(result));sys.exit", injected+" print(json.dumps(result));sys.exit", 1), 0700)
	turn, _ := coordinatorRun(t, a)
	root := filepath.Dir(turn.Job.Dir)
	stages, _ := filepath.Glob(filepath.Join(root, "team-admission-*", "team-goal.txt"))
	if len(stages) != 2 {
		t.Fatal("expected retained failed capture plus checked capture", stages)
	}
	if len(a.taskTurns(turn.Record.Thread)) != 1 {
		t.Fatal("required user continuation for known local input change")
	}
	calls, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
	if strings.Count(string(calls), "admit\n") != 1 || strings.Count(string(calls), "preflight\n") != 2 {
		t.Fatal("wrong admission/preflight count", string(calls))
	}
}

func TestCoordinatorChangingInputsStopAtRecheckBoundBeforeAdmission(t *testing.T) {
	a := coordinatorFixture(t)
	script, _ := os.ReadFile(a.cfg.TeamCoordinator)
	injected := ` (Path(arg('-definition')).parent.parent/'work/execution/late.txt').write_text(log.read_text())
`
	writeFixture(t, a.cfg.TeamCoordinator, strings.Replace(string(script), " print(json.dumps(result));sys.exit", injected+" print(json.dumps(result));sys.exit", 1), 0700)
	turn := taskSubmit(t, a, "", "Build a team")
	if turn.Job.ExitCode == nil || *turn.Job.ExitCode != 2 || !strings.Contains(turn.Result.Message, "inputs kept changing") {
		t.Fatalf("wrong bounded stop: %+v", turn.Result)
	}
	if _, err := readTaskTeamReference(turn.Job, turn.Record); !os.IsNotExist(err) {
		t.Fatal("changing inputs admitted", err)
	}
	calls, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.TeamCoordinator), "calls"))
	if strings.Count(string(calls), "preflight\n") != 3 || strings.Contains(string(calls), "admit\n") {
		t.Fatal("unbounded recheck or early admission", string(calls))
	}
}

func TestCoordinatorInterleavedDeliveryDoesNotHideExplicitCancellation(t *testing.T) {
	a := coordinatorFixture(t)
	turn, ref := coordinatorRun(t, a)
	completeCoordinator(t, ref)
	result := a.taskTurns(turn.Record.Thread)[0].Result
	start := func(thread string, args []string) Job {
		t.Helper()
		dir, err := a.workspace()
		if err != nil {
			t.Fatal(err)
		}
		if err = saveTaskJSON(filepath.Join(filepath.Dir(dir), "delivery-request.json"), deliveryRecord{Thread: thread, TaskID: turn.Job.ID}); err != nil {
			t.Fatal(err)
		}
		job, err := a.jobs.Start("delivery", "Delivery", dir, args, "")
		if err != nil {
			t.Fatal(err)
		}
		return job
	}
	cancelled := start(turn.Record.Thread, helperArgs("interrupt"))
	awaitReady(t, a.jobs, cancelled.ID)
	if err := a.jobs.Cancel(cancelled.ID); err != nil {
		t.Fatal(err)
	}
	if got := awaitJob(t, a.jobs, cancelled.ID); got.State != "cancelled" {
		t.Fatal("cancellation not observed", got)
	}
	other := start("different-conversation", helperArgs("echo", "0"))
	if got := awaitJob(t, a.jobs, other.ID); got.State != "completed" {
		t.Fatal(got)
	}
	before := len(a.jobs.List())
	handled, err := a.queueTeamDelivery(turn.Job, turn.Record, result)
	if err != nil || !handled || len(a.jobs.List()) != before {
		t.Fatal("unrelated newer delivery hid explicit cancellation", handled, err, a.jobs.List())
	}
}

func TestCoordinatorOlderCompletionCannotRollBackNewerDelivery(t *testing.T) {
	a := coordinatorFixture(t)
	a.cfg.Plonk = filepath.Join(t.TempDir(), "plonk")
	a.cfg.PlonkURL = "http://127.0.0.1:1"
	a.cfg.PlonkTokenFile = filepath.Join(t.TempDir(), "connection")
	writeFixture(t, a.cfg.PlonkTokenFile, "fixture", 0600)
	writeFixture(t, a.cfg.Plonk, `#!/usr/bin/python3
import json,sys
from pathlib import Path
assert sys.argv[1]=='publish'
Path(__file__).with_name('calls').open('a').write(json.dumps(sys.argv)+'\n')
print(json.dumps(dict(slug='private-result',versionId=sys.argv[sys.argv.index('-request-id')+1])))
`, 0700)
	deliver := func(turn taskTurn) taskResult {
		t.Helper()
		ref, err := readTaskTeamReference(turn.Job, turn.Record)
		if err != nil {
			t.Fatal(err)
		}
		completeCoordinator(t, ref)
		turns := a.taskTurns(turn.Record.Thread)
		current := turns[len(turns)-1]
		started, err := a.queueTeamDelivery(turn.Job, turn.Record, current.Result)
		if err != nil || !started {
			t.Fatal(started, err)
		}
		var delivery Job
		for _, job := range a.jobs.List() {
			if job.Kind == "delivery" {
				delivery = job
				break
			}
		}
		if got := awaitJob(t, a.jobs, delivery.ID); got.State != "completed" {
			t.Fatal(got, a.jobs.Log(got.ID, "stderr"))
		}
		return current.Result
	}
	first, _ := coordinatorRun(t, a)
	firstResult := deliver(first)
	second := taskSubmit(t, a, first.Record.Thread, "Use this team for the revised document")
	if second.Job.State != "completed" {
		t.Fatal(second.Job, a.jobs.Log(second.Job.ID, "stderr"))
	}
	deliver(second)
	if a.deliveryState(first.Record.Thread).TaskID != second.Job.ID {
		t.Fatal("fixture lacks newer delivery")
	}
	before := len(a.jobs.List())
	handled, err := a.queueTeamDelivery(first.Job, first.Record, firstResult)
	if err != nil || !handled || len(a.jobs.List()) != before {
		t.Fatal("historical completion queued an old publication", handled, err)
	}
	if a.deliveryState(first.Record.Thread).TaskID != second.Job.ID {
		t.Fatal("newer delivery receipt rolled back")
	}
	calls, _ := os.ReadFile(filepath.Join(filepath.Dir(a.cfg.Plonk), "calls"))
	if strings.Count(string(calls), "\n") != 2 {
		t.Fatal("publication was replayed", string(calls))
	}
}
