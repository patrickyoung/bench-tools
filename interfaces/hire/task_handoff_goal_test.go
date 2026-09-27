package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The executable fixture emits the public handoff, not a mocked Go decoder
// return. Every test passes through the ordinary work admission and child job.
const goalTeamReportPython = `
import hashlib,json,os
from pathlib import Path
def write_report(status, files, reason='', next='', question='', message='The current team work is saved.'):
 report=dict(version=1,invocation=os.environ['BENCH_TASK_INVOCATION'],goal_sha256=os.environ['BENCH_TASK_GOAL_SHA256'],definition_sha256=os.environ['BENCH_TASK_DEFINITION_SHA256'],status=status,stage='Reviewing current work',message=message,reason=reason,next=next,question=question,files=[dict(path=p,kind=k,sha256=hashlib.sha256(Path(p).read_bytes()).hexdigest()) for p,k in files])
 temporary=Path('.task-result.tmp')
 temporary.write_text(json.dumps(report))
 temporary.replace('task-result.json')
 return report
`

func setGoalTeamAdapter(t *testing.T, a *app, body string) {
	t.Helper()
	raw, err := os.ReadFile(a.cfg.Hire)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, a.cfg.Hire, string(raw)+"\ncat > expert/bin/task <<'GOAL_TEAM_ADAPTER'\n#!/usr/bin/python3\n"+goalTeamReportPython+"\n"+body+"\nGOAL_TEAM_ADAPTER\nchmod 700 expert/bin/task\n", 0700)
}

func goalExecuteCount(t *testing.T, turn taskTurn) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(goalRoot(turn), "goal-attempts/execute"))
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func assertNoGoalPresentation(t *testing.T, turn taskTurn) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(goalRoot(turn), "present")); !os.IsNotExist(err) {
		t.Fatal("a concrete team stop was sent to the completion model")
	}
}

func TestGoalTeamBlockedSuccessRetainsNestedWorkWithoutRetry(t *testing.T) {
	a := goalFixture(t)
	selectTeamEnvironmentFixture(t, a)
	setGoalTeamAdapter(t, a, `partial=Path('.team/developer');partial.mkdir(parents=True)
(partial/'candidate.py').write_text('print("corrected source")\n')
(partial/'checks.txt').write_text('13 local checks passed; image build unavailable.')
Path('result.md').write_text('Inherited old deliverable, not this result.')
write_report('blocked', [('.team/developer/candidate.py','work'),('.team/developer/checks.txt','evidence')], reason='The selected environment has no image builder.', next='Provide the required image build environment.', message='The source corrections and 13 local checks are saved.')
`)
	turn := taskSubmit(t, a, "", "Finish the team deployment")
	if goalExecuteCount(t, turn) != 1 || turn.Result.Code != 2 || turn.Result.GoalStatus != "blocked" || len(turn.Result.Artifacts) != 0 {
		t.Fatalf("blocked member replayed or inherited files delivered: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
	}
	assertNoGoalPresentation(t, turn)
	for _, name := range []string{"candidate.py", "checks.txt"} {
		if _, err := os.Stat(filepath.Join(goalRoot(turn), "goal-rounds/001/handoff/files/.team/developer", name)); err != nil {
			t.Fatalf("partial work not snapshotted: %s: %v", name, err)
		}
	}
	page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
	for _, text := range []string{"The selected environment has no image builder.", "Provide the required image build environment."} {
		if !strings.Contains(page, text) {
			t.Fatalf("missing concrete stop guidance %q", text)
		}
	}
}

func TestGoalTeamNestedProgressContinuesSameMemberCheckpoint(t *testing.T) {
	a := goalFixture(t)
	selectTeamEnvironmentFixture(t, a)
	member := filepath.Join(t.TempDir(), "member-agent")
	writeFixture(t, member, `#!/usr/bin/python3
import json,sys
from pathlib import Path
assert sys.argv[1:]==['run','-checkpoint','retained-developer','-B']
checkpoint=Path('checkpoint.json')
prior=json.loads(checkpoint.read_text()) if checkpoint.exists() else {'name':'retained-developer','attempt':0}
assert prior['name']=='retained-developer'
prior['attempt']+=1
checkpoint.write_text(json.dumps(prior))
Path('candidate.py').write_text('completed_steps = '+str(prior['attempt'])+'\n')
Path('checks.txt').write_text('Observed member attempt '+str(prior['attempt']))
raise SystemExit(2 if prior['attempt']<5 else 0)
`, 0700)
	setGoalTeamAdapter(t, a, `import subprocess
member=Path('.team/developer');member.mkdir(parents=True,exist_ok=True)
initial=member/'initial-invocation'
if initial.exists():
 assert os.environ['BENCH_TASK_RESUME']=='1'
 assert 'Completion review of current work' in Path(os.environ['BENCH_TASK_FILE']).read_text()
 assert initial.read_text()!=os.environ['BENCH_TASK_INVOCATION']
else:
 assert not os.environ.get('BENCH_TASK_RESUME')
 initial.write_text(os.environ['BENCH_TASK_INVOCATION'])
code=subprocess.run([`+strconv.Quote(member)+`,'run','-checkpoint','retained-developer','-B'],cwd=member).returncode
files=[('.team/developer/candidate.py','work'),('.team/developer/checks.txt','evidence')]
if code==0:
 (member/'result.md').write_text('Finished: checked deployment package')
 files.append(('.team/developer/result.md','deliverable'))
write_report('complete' if code==0 else 'continue', files, next='Finish the remaining source correction.' if code else '')
raise SystemExit(code)
`)
	turn := taskSubmit(t, a, "", "Finish the team deployment")
	if turn.Result.Code != 0 || turn.Result.GoalStatus != "complete" || goalExecuteCount(t, turn) != 5 || goalRounds(t, turn) != 5 || len(turn.Result.Artifacts) != 1 {
		t.Fatalf("nested progress was lost or stopped early: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
	}
	for round := 1; round <= 5; round++ {
		name := filepath.Join(goalRoot(turn), "goal-rounds", "00"+strconv.Itoa(round), "handoff/files/.team/developer/candidate.py")
		content, err := os.ReadFile(name)
		if err != nil || string(content) != "completed_steps = "+strconv.Itoa(round)+"\n" {
			t.Fatalf("round %d lost its immutable source: %q %v", round, content, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(goalRoot(turn), "deliverables/result.md"))
	if err != nil || string(content) != "Finished: checked deployment package" {
		t.Fatalf("nested declared deliverable unavailable: %q %v", content, err)
	}
}

func TestGoalTeamInvalidOrUnsafeHandoffNeverReplays(t *testing.T) {
	for _, scenario := range []struct {
		name string
		code int
		edit string
	}{
		{"missing", 2, "Path('task-result.json').unlink()"},
		{"stale-invocation", 2, "report['invocation']='stale';Path('task-result.json').write_text(json.dumps(report))"},
		{"changed-source", 2, "Path('.team/candidate.py').write_text('changed after report')"},
		{"unsafe-continuation", 125, ""},
		{"interrupted-continuation", 130, ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			a := goalFixture(t)
			selectTeamEnvironmentFixture(t, a)
			setGoalTeamAdapter(t, a, `Path('.team').mkdir()
Path('.team/candidate.py').write_text('partial source')
report=write_report('continue', [('.team/candidate.py','work')], next='Finish the remaining source.')
`+scenario.edit+"\nraise SystemExit("+strconv.Itoa(scenario.code)+")\n")
			turn := taskSubmit(t, a, "", "Finish the team deployment")
			if goalExecuteCount(t, turn) != 1 || turn.Result.Code != scenario.code || turn.Result.GoalStatus != "blocked" {
				t.Fatalf("unsafe handoff replayed or exit lost: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
			}
			assertNoGoalPresentation(t, turn)
		})
	}
}

func TestGoalTeamChangedFeedbackAndEvidenceCannotHideStall(t *testing.T) {
	a := goalFixture(t)
	selectTeamEnvironmentFixture(t, a)
	raw, err := os.ReadFile(a.cfg.Agent)
	if err != nil {
		t.Fatal(err)
	}
	old := "next_goal='Finish the requested result; replace the draft with checked final work.' if status=='continue' else ''"
	new := "next_goal=('Correction wording '+str(r['goal_context']['attempt'])) if status=='continue' else ''"
	if !strings.Contains(string(raw), old) {
		t.Fatal("review fixture seam changed")
	}
	writeFixture(t, a.cfg.Agent, strings.Replace(string(raw), old, new, 1), 0700)
	setGoalTeamAdapter(t, a, `Path('.team').mkdir(exist_ok=True)
Path('.team/candidate.py').write_text('same incomplete source')
Path('.team/checks.txt').write_text('New receipt '+os.environ['BENCH_TASK_INVOCATION'])
write_report('continue', [('.team/candidate.py','work'),('.team/checks.txt','evidence')], next='Try correction '+os.environ['BENCH_TASK_INVOCATION'])
raise SystemExit(2)
`)
	turn := taskSubmit(t, a, "", "Finish the team deployment")
	if goalExecuteCount(t, turn) != 3 || turn.Result.Code != 2 || turn.Result.GoalStatus != "blocked" || !strings.Contains(turn.Result.Message, "unchanged work") {
		t.Fatalf("changing prose hid stalled work: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
	}
	for round := 1; round <= 3; round++ {
		b, err := os.ReadFile(filepath.Join(goalRoot(turn), "goal-rounds", "00"+strconv.Itoa(round), "review-response.json"))
		var review taskCompletion
		if err != nil || json.Unmarshal(b, &review) != nil || review.NextGoal != "Correction wording "+strconv.Itoa(round) {
			t.Fatalf("fixture failed to vary review feedback: %s: %v", b, err)
		}
	}
}

func TestGoalTeamMissingFactIsQuestionWithoutRepeatedExecution(t *testing.T) {
	a := goalFixture(t)
	selectTeamEnvironmentFixture(t, a)
	setGoalTeamAdapter(t, a, `write_report('needs_input', [], question='Which of the supplied venues should appear?', message='The venue selection is the remaining missing fact.')
raise SystemExit(75)
`)
	turn := taskSubmit(t, a, "", "Finish the team invitation")
	if goalExecuteCount(t, turn) != 1 || turn.Result.Code != 75 || turn.Result.GoalStatus != "needs_input" || turn.Result.Question != "Which of the supplied venues should appear?" {
		t.Fatalf("question was lost or repeated: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
	}
	assertNoGoalPresentation(t, turn)
}

func TestGoalTeamInvalidDeliveryPreservesUnsafeObservedExit(t *testing.T) {
	for _, code := range []int{130, 137} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			a := goalFixture(t)
			selectTeamEnvironmentFixture(t, a)
			setGoalTeamAdapter(t, a, `Path('candidate.unsupported').write_text('partial candidate')
write_report('blocked', [('candidate.unsupported','deliverable')], reason='The member was interrupted.', next='Inspect the saved member outcome before preparing fresh work.')
raise SystemExit(`+strconv.Itoa(code)+`)
`)
			turn := taskSubmit(t, a, "", "Finish the team document")
			if goalExecuteCount(t, turn) != 1 || turn.Result.Code != code || turn.Result.GoalStatus != "blocked" || !strings.Contains(turn.Result.Message, "declared deliverables") {
				t.Fatalf("artifact error masked unsafe command outcome: %+v\n%s", turn.Result, a.jobs.Log(turn.Job.ID, "stderr"))
			}
			assertNoGoalPresentation(t, turn)
		})
	}
}
