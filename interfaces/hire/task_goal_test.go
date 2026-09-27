package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func goalFixture(t *testing.T) *app {
	t.Helper()
	a := taskFixture(t)
	a.cfg.GoalMode = true
	raw, err := os.ReadFile(a.cfg.Agent)
	if err != nil {
		t.Fatal(err)
	}
	old := " else: reply=dict(request_sha256=h,message='Here is your work.' if r['execution']['exit_code']==0 else 'I need more information to finish.')"
	check, err := filepath.Abs("../../workers/bench-hire/expert/task/bin/check")
	if err != nil {
		t.Fatal(err)
	}
	replacement := ` else:
  code=r['execution']['exit_code']
  content=(Path(r['artifacts_dir'])/'result.md').read_text()
  status='complete' if code==0 and content.startswith('Finished:') else 'continue' if code in (0,2) else 'blocked'
  if r['message']=='ask input': status='needs_input'
  reply=dict(request_sha256=h,message='Here is your checked work.' if status=='complete' else 'The result still needs correction.',status=status,next_goal='Finish the requested result; replace the draft with checked final work.' if status=='continue' else '',question='Which of the two supplied venues should I use?' if status=='needs_input' else '')
  Path('response.json').write_text(json.dumps(reply))
  import subprocess
  assert subprocess.run(['/usr/bin/python3', ` + strconv.Quote(check) + `]).returncode==0
  assert p.read_bytes()==raw
`
	if !strings.Contains(string(raw), old) {
		t.Fatal("fixture seam changed")
	}
	writeFixture(t, a.cfg.Agent, strings.Replace(string(raw), old, replacement, 1), 0700)
	return a
}
func appendGoalWorker(t *testing.T, a *app, script string) {
	t.Helper()
	b, err := os.ReadFile(a.cfg.Agent)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, a.cfg.Agent, string(b)+"\n"+script, 0700)
}
func goalRoot(turn taskTurn) string { return filepath.Dir(turn.Job.Dir) }
func goalRounds(t *testing.T, turn taskTurn) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(goalRoot(turn), "goal-rounds"))
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}
func TestGoalContinuesUnfinishedAndCorrectsAcceptedDraft(t *testing.T) {
	a := goalFixture(t)
	appendGoalWorker(t, a, `state=Path('runtime');state.mkdir(exist_ok=True)
counter=state/'count';n=int(counter.read_text())+1 if counter.exists() else 1;counter.write_text(str(n))
assert sys.argv[sys.argv.index('-checkpoint')+1]=='task'
assert '-B' in sys.argv and '-compact' in sys.argv
(state/('argv-'+str(n)+'.json')).write_text(json.dumps(sys.argv))
if n>1: assert 'Completion review of current work' in Path(sys.argv[sys.argv.index('-goal-file')+1]).read_text()
if n<3: Path('result.md').write_text('Partial draft')
sys.exit(2 if n==1 else 0)
`)
	turn := taskSubmit(t, a, "", "Finish this document")
	if turn.Result.GoalStatus != "complete" || turn.Result.Code != 0 || goalRounds(t, turn) != 3 || len(a.taskTurns(turn.Record.Thread)) != 1 {
		t.Fatalf("goal failed: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
	}
	b, _ := os.ReadFile(filepath.Join(goalRoot(turn), "goal-rounds/001/deliverables/result.md"))
	if string(b) != "Partial draft" {
		t.Fatal("lost original unfinished version")
	}
	b, _ = os.ReadFile(filepath.Join(goalRoot(turn), "deliverables/result.md"))
	if !strings.HasPrefix(string(b), "Finished:") {
		t.Fatal("final not delivered")
	}
	attempts, _ := os.ReadDir(filepath.Join(goalRoot(turn), "goal-attempts/execute"))
	if len(attempts) != 3 {
		t.Fatal("missing exact attempt receipts")
	}
	page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
	if strings.Contains(page, "Try again with saved work") {
		t.Fatal("completed goal asks for retry")
	}
}
func TestGoalStopsOnNoProgressAndUnsafeOutcomes(t *testing.T) {
	for _, code := range []int{2, 1, 3, 75, 125, 130, 137} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			a := goalFixture(t)
			appendGoalWorker(t, a, "Path('result.md').write_text('Partial draft')\nsys.exit("+strconv.Itoa(code)+")\n")
			turn := taskSubmit(t, a, "", "Finish this document")
			want := 1
			if code == 2 {
				want = 3
			}
			if goalRounds(t, turn) != want || turn.Result.Code != code || turn.Result.GoalStatus != "blocked" {
				t.Fatalf("unsafe replay or lost status: %+v", turn.Result)
			}
			page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
			if strings.Contains(page, "Try again with saved work") || strings.Contains(page, "Continue this work") {
				t.Fatal("generic retry offered for concrete blocker")
			}
		})
	}
}
func TestGoalAsksOnlyConcreteMissingInput(t *testing.T) {
	a := goalFixture(t)
	turn := taskSubmit(t, a, "", "ask input")
	if turn.Result.GoalStatus != "needs_input" || turn.Result.Question == "" || turn.Result.Code != 75 || goalRounds(t, turn) != 1 {
		t.Fatalf("missing question: %+v", turn.Result)
	}
}
func TestGoalPreparationContinuesCheckpoint(t *testing.T) {
	a := goalFixture(t)
	b, _ := os.ReadFile(a.cfg.Hire)
	writeFixture(t, a.cfg.Hire, strings.Replace(string(b), "set -eu", "set -eu\nif [ ! -f saved-attempt ]; then printf partial > saved-attempt; exit 2; fi", 1), 0700)
	turn := taskSubmit(t, a, "", "Finish this document")
	attempts, _ := os.ReadDir(filepath.Join(goalRoot(turn), "goal-attempts/prepare"))
	if turn.Result.Code != 0 || len(attempts) != 2 {
		t.Fatalf("preparation did not continue: %+v", turn.Result)
	}
}
func TestGoalCompletionProtocolRejectsFalseSuccess(t *testing.T) {
	req := []byte(`{}`)
	good := taskCompletion{Hash: digestText(req), Message: "Checked.", Status: "complete"}
	for _, c := range []int{1, 2, 3, 75, 125, 130, 137} {
		b, _ := json.Marshal(good)
		if _, err := decodeTaskCompletion(b, req, c); err == nil {
			t.Fatal("nonzero completed", c)
		}
	}
	for _, status := range []string{"complete", "continue", "needs_input", "blocked"} {
		p := good
		p.Status = status
		if status == "continue" {
			p.NextGoal = "Repair missing item."
		}
		if status == "needs_input" {
			p.Question = "Which venue?"
		}
		b, _ := json.Marshal(p)
		if _, err := decodeTaskCompletion(b, req, 0); err != nil {
			t.Fatal(err)
		}
		p.Hash = "wrong"
		b, _ = json.Marshal(p)
		if _, err := decodeTaskCompletion(b, req, 0); err == nil {
			t.Fatal("unbound reply")
		}
	}
	good.Question = " "
	b, _ := json.Marshal(good)
	if _, err := decodeTaskCompletion(b, req, 0); err == nil {
		t.Fatal("nonempty inactive field")
	}
}
func TestGoalCancelledContextDoesNotInvoke(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, _, _ := taskCheckpointCommand(ctx, dir, "prepare", dir, taskRecord{Config: config{GoalMode: true}}, "missing-command")
	if code != 130 {
		t.Fatal(code)
	}
	if _, err := os.Stat(filepath.Join(dir, "goal-attempts")); !os.IsNotExist(err) {
		t.Fatal("invoked after cancellation")
	}
}
func TestGoalTeamResumeCapabilityIsExact(t *testing.T) {
	for _, raw := range []string{`{"version":1,"resume":true,"local_only":true}`, `{"version":1,"resume":true}`, `{"version":2,"resume":true,"local_only":true}`, `{"version":1,"resume":true,"local_only":true,"external":true}`} {
		dir := t.TempDir()
		writeFixture(t, filepath.Join(dir, "task-runtime.json"), raw, 0600)
		if taskTeamResumable(dir) != (raw == `{"version":1,"resume":true,"local_only":true}`) {
			t.Fatal(raw)
		}
	}
}

func TestGoalTeamContinuationRequiresDeclaredContract(t *testing.T) {
	for _, supported := range []bool{false, true} {
		t.Run(strconv.FormatBool(supported), func(t *testing.T) {
			a := goalFixture(t)
			raw, _ := os.ReadFile(a.cfg.Hire)
			marker := ""
			if supported {
				marker = "printf '%s\\n' '{\"version\":1,\"resume\":true,\"local_only\":true}' > expert/task-runtime.json\n"
				selectTeamEnvironmentFixture(t, a)
			}
			script := string(raw) + "\n" + marker + `cat > expert/bin/task <<'GOALTEAM'
#!/usr/bin/python3
import os
from pathlib import Path
state=Path('runtime');state.mkdir(exist_ok=True)
count=state/'count';n=int(count.read_text())+1 if count.exists() else 1
count.write_text(str(n))
if n==1:
 assert not os.environ.get('BENCH_TASK_RESUME')
 Path('result.md').write_text('Partial draft')
else:
 assert os.environ['BENCH_TASK_RESUME']=='1'
 assert 'Completion review of current work' in Path(os.environ['BENCH_TASK_FILE']).read_text()
 Path('result.md').write_text('Finished: team result')
raise SystemExit(2 if n==1 else 0)
GOALTEAM
chmod 700 expert/bin/task
`
			writeFixture(t, a.cfg.Hire, script, 0700)
			turn := taskSubmit(t, a, "", "Finish the team document")
			if supported {
				log := a.jobs.Log(turn.Job.ID, "stderr")
				if turn.Result.Code != 0 || goalRounds(t, turn) != 2 || strings.Count(log, "selected environment team entry") != 2 {
					t.Fatalf("team continuation failed: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
				}
			} else if turn.Result.Code != 2 || goalRounds(t, turn) != 1 || turn.Result.GoalStatus != "blocked" {
				t.Fatalf("unsupported team replayed: %+v", turn.Result)
			}
		})
	}
}
