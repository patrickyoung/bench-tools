package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func selectionTestPlan() (taskPlan, []taskChoice) {
	cat := withTaskAdaptations([]taskChoice{
		{Key: "worker:source:presentation", Kind: "worker", Name: "Presentation Designer"},
		{Key: "worker:source:illustrator", Kind: "worker", Name: "Illustrator"},
		{Key: "team:source:studio", Kind: "team", Name: "Studio"},
	})
	p := taskPlan{Message: "I’ll use the presentation designer.", Title: "Slides", Target: cat[0].Key, Brief: "Make the requested slides.", Inputs: []taskInput{}, Selection: &taskSelection{
		Reviewed: []taskCandidate{{cat[0].Key, "reuse", "Existing slide expertise fits."}, {cat[1].Key, "none", "No drawing requested."}, {cat[2].Key, "none", "The comparison pipeline is unnecessary."}}, Roles: []taskMember{},
	}}
	return p, cat
}

func TestSelectionContractAgreesWithCompanion(t *testing.T) {
	check, _ := filepath.Abs("../../workers/bench-hire/expert/task/bin/check")
	for _, tc := range []struct {
		name  string
		edit  func(*taskPlan)
		valid bool
	}{
		{"reuse", func(p *taskPlan) {}, true},
		{"missing", func(p *taskPlan) { p.Selection = nil }, false},
		{"skipped catalog", func(p *taskPlan) { p.Selection.Reviewed = p.Selection.Reviewed[:1] }, false},
		{"generic replacement", func(p *taskPlan) { p.Target = "new:worker"; p.Selection.Gap = "Make new slides." }, false},
		{"actual gap", func(p *taskPlan) {
			p.Target = "new:worker"
			p.Selection.Gap = "A missing unrelated specialty."
			for i := range p.Selection.Reviewed {
				p.Selection.Reviewed[i].Fit = "none"
			}
		}, true},
		{"adapt", func(p *taskPlan) {
			p.Target = "adapt:" + p.Target
			p.Selection.Reviewed[0].Fit = "adapt"
			p.Selection.Gap = "Accept instructional slide input."
		}, true},
		{"silent adaptation", func(p *taskPlan) { p.Selection.Reviewed[0].Fit = "adapt" }, false},
		{"matching existing team", func(p *taskPlan) {
			p.Target = "new:team"
			p.Selection.Reviewed[2].Fit = "reuse"
			p.Selection.Roles = []taskMember{{"writer", p.Selection.Reviewed[0].Target, "Write slides."}, {"reviewer", "new:worker", "Review."}}
			p.Selection.Gap = "A review."
		}, false},
		{"team without members", func(p *taskPlan) { p.Target = "new:team" }, false},
		{"selected members", func(p *taskPlan) {
			p.Target = "new:team"
			p.Selection.Reviewed[1].Fit = "reuse"
			p.Selection.Roles = []taskMember{{"slides", p.Selection.Reviewed[0].Target, "Create the slides."}, {"drawings", p.Selection.Reviewed[1].Target, "Draw the requested illustrations."}}
		}, true},
		{"new members ignore available", func(p *taskPlan) {
			p.Target = "new:team"
			p.Selection.Gap = "Build it all."
			p.Selection.Roles = []taskMember{{"slides", "new:worker", "Slides."}, {"drawings", "new:worker", "Drawings."}}
		}, false},
		{"question", func(p *taskPlan) {
			p.Target = ""
			p.Question = "Which audience?"
			p.Selection = &taskSelection{Reviewed: []taskCandidate{}, Roles: []taskMember{}}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, cat := selectionTestPlan()
			tc.edit(&p)
			request, _ := json.Marshal(map[string]any{"selection_version": 1, "stage": "plan", "message": "Make slides", "source": "/selected", "now": "2026-09-26T12:00:00Z", "history": []any{}, "catalog": taskModelCatalog(cat)})
			p.Hash = digestText(request)
			raw, _ := json.Marshal(p)
			_, err := decodeTaskPlan(raw, request, cat)
			if (err == nil) != tc.valid {
				t.Fatalf("controller validity %v: %v", tc.valid, err)
			}
			dir := t.TempDir()
			writeFixture(t, filepath.Join(dir, "request.json"), string(request), 0600)
			writeFixture(t, filepath.Join(dir, "response.json"), string(raw), 0600)
			cmd := exec.Command("python3", check)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.valid {
				t.Fatalf("companion validity %v: %s %v", tc.valid, out, err)
			}
		})
	}
}

func TestSelectedTeamMembersArePinnedAndCannotBeReplaced(t *testing.T) {
	a := taskFixture(t)
	root := filepath.Join(a.cfg.Data, "workspaces", "selection-test")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	// Public exporter fixture proves each selected library member is actually
	// exported at the caller's revision, not merely mentioned in a build brief.
	exporter := filepath.Join(t.TempDir(), "exporter")
	writeFixture(t, exporter, `#!/usr/bin/python3
import sys
from pathlib import Path
assert sys.argv[2]=='export'
assert sys.argv[5:]==['--ref','`+a.cat.Revision+`','--allow-experimental']
p=Path(sys.argv[4])/'expert';(p/'bin').mkdir(parents=True)
(p/'AGENTS.md').write_text('Existing '+sys.argv[3]+' expertise')
(p/'README.md').write_text('# '+sys.argv[3])
(p/'bin/check').write_text('#!/bin/sh\nexit 0\n');(p/'bin/check').chmod(0o700)
`, 0700)
	p, cat := selectionTestPlan()
	cat[0].ID = "presentation"
	cat[1].ID = "illustrator"
	p.Target = "new:team"
	p.Selection.Reviewed[1].Fit = "reuse"
	p.Selection.Roles = []taskMember{{"slides", cat[0].Key, "Make slides."}, {"drawings", cat[1].Key, "Make illustrations."}}
	rec := taskRecord{Config: a.cfg, Revision: a.cat.Revision, Catalog: cat}
	rec.Config.Python = exporter
	if err := prepareTaskMembers(context.Background(), root, rec, p); err != nil {
		t.Fatal(err)
	}
	if err := validatePreparedTaskMembers(root, rec, p); err != nil {
		t.Fatal(err)
	}
	info := plannedSpecialist(p, cat)
	if len(info.Members) != 2 || !strings.Contains(info.Members[0], "Presentation Designer") {
		t.Fatal(info)
	}
	memberPath := filepath.Join(root, "authoring", "expert", "agents", "slides", "AGENTS.md")
	writeFixture(t, memberPath, "Generic replacement", 0600)
	if err := validatePreparedTaskMembers(root, rec, p); err == nil {
		t.Fatal("replacement silently accepted")
	}
}

func TestSelectionFailureNeverStartsBuildOrExecution(t *testing.T) {
	a := taskFixture(t)
	raw, _ := os.ReadFile(a.cfg.Agent)
	script := strings.Replace(string(raw), "Path('response.json').write_text(json.dumps(reply));sys.exit(0)", "reply.pop('selection',None);Path('response.json').write_text(json.dumps(reply));sys.exit(0)", 1)
	writeFixture(t, a.cfg.Agent, script, 0700)
	turn := taskSubmit(t, a, "", "Make a presentation")
	if turn.Result.Code != 2 || turn.Result.Expert != "" || strings.Contains(a.jobs.Log(turn.Job.ID, "stderr"), "Starting hire") {
		t.Fatal("invalid selection dispatched", turn.Result)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(turn.Job.Dir), "work", "execution")); !os.IsNotExist(err) {
		t.Fatal("execution prepared before selection")
	}
}

func TestWorkTeamBuildUsesSelectedSourcesBeforeExecution(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "preserved"
		if replace {
			name = "replaced"
		}
		t.Run(name, func(t *testing.T) {
			a := taskFixture(t)
			exporter := filepath.Join(t.TempDir(), "export")
			writeFixture(t, exporter, `#!/usr/bin/python3
import sys
from pathlib import Path
assert sys.argv[2]=='export'
assert sys.argv[5:]==['--ref','`+a.cat.Revision+`','--allow-experimental']
p=Path(sys.argv[4])/'expert';(p/'bin').mkdir(parents=True)
(p/'AGENTS.md').write_text('Existing '+sys.argv[3]+' expertise')
(p/'README.md').write_text('# '+sys.argv[3])
(p/'bin/check').write_text('#!/bin/sh\nexit 0\n');(p/'bin/check').chmod(0o700)
`, 0700)
			a.cfg.Python = exporter
			raw, _ := os.ReadFile(a.cfg.Agent)
			seam := " Path('response.json').write_text(json.dumps(reply));sys.exit(0)"
			replacement := ` if r['stage']=='plan':
  reply['selection']['roles']=[dict(role='writer',target='worker:source:writer',responsibility='Write the work.'),dict(role='reviewer',target='worker:source:trial',responsibility='Review the work.')]
  for c in reply['selection']['reviewed']:
   if c['target'] in ['worker:source:writer','worker:source:trial']:c['fit']='reuse'
 Path('response.json').write_text(json.dumps(reply));sys.exit(0)`
			if !strings.Contains(string(raw), seam) {
				t.Fatal("fixture seam changed")
			}
			script := strings.Replace(string(raw), seam, replacement, 1)
			// Exercise real argv dispatch to separately selected member definitions;
			// the executable itself is a deterministic stand-in for public Agent.
			script = strings.Replace(script, "p=Path('request.json')", `if '--member-case' in sys.argv:
 i=sys.argv.index('-C');work=Path(sys.argv[i+1]);work.mkdir(parents=True)
 definition=Path(sys.argv[sys.argv.index('--')-1])
 assert (definition/'AGENTS.md').read_text().startswith('Existing ')
 (work/'result.txt').write_text((definition/'AGENTS.md').read_text());sys.exit(0)
p=Path('request.json')`, 1)
			writeFixture(t, a.cfg.Agent, script, 0700)
			hire := `#!/bin/sh
set -eu
mkdir -p expert/bin
printf 'Team coordinator\n' > expert/AGENTS.md
printf '# Selected writing team\n' > expert/README.md
printf '#!/bin/sh\nexit 0\n' > expert/bin/check
cat > expert/bin/task <<'TASK'
#!/bin/sh
set -eu
team=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
` + shellArg(a.cfg.Agent) + ` run -C "$BENCH_TASK_WORK/members/writer" -m fixture/model -turns 50 "$team/agents/writer" -- --member-case
` + shellArg(a.cfg.Agent) + ` run -C "$BENCH_TASK_WORK/members/reviewer" -m fixture/model -turns 50 "$team/agents/reviewer" -- --member-case
cat "$BENCH_TASK_WORK/members/writer/result.txt" "$BENCH_TASK_WORK/members/reviewer/result.txt" > "$BENCH_TASK_WORK/result.md"
TASK
chmod 700 expert/bin/check expert/bin/task
`
			if replace {
				hire += "printf 'Generic replacement' > expert/agents/writer/AGENTS.md\n"
			}
			writeFixture(t, a.cfg.Hire, hire, 0700)
			turn := taskSubmit(t, a, "", "Build a team document")
			root := filepath.Dir(turn.Job.Dir)
			output, err := os.ReadFile(filepath.Join(root, "work", "execution", "result.md"))
			if replace {
				if turn.Result.Code != 2 || turn.Result.Prepared || !os.IsNotExist(err) {
					t.Fatal("changed worker was executed", turn.Result, err)
				}
			} else {
				if turn.Result.Code != 0 || err != nil || !strings.Contains(string(output), "Existing writer expertise") || !strings.Contains(string(output), "Existing trial expertise") {
					t.Fatal("selected members not executed", turn.Result, err, a.jobs.Log(turn.Job.ID, "stderr"))
				}
				page := serveTest(a, "GET", "/work/"+turn.Record.Thread, nil).Body.String()
				if !strings.Contains(page, "writer: Writer") || !strings.Contains(page, "reviewer: Trial") {
					t.Fatal("assigned roster missing")
				}
			}
		})
	}
}

func TestSelectionProtocolCannotDowngradeAndPreservesExportOutcomes(t *testing.T) {
	p, cat := selectionTestPlan()
	for _, version := range []string{"null", "0", "2", "true", "\"1\""} {
		request := []byte(`{"selection_version":` + version + `}`)
		p.Hash = digestText(request)
		raw, _ := json.Marshal(p)
		if _, err := decodeTaskPlan(raw, request, cat); err == nil {
			t.Fatal("invalid version accepted", version)
		}
	}
	a := taskFixture(t)
	root := filepath.Join(a.cfg.Data, "workspaces", "export-status")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{1, 2, 125, 130, 137} {
		executable := filepath.Join(t.TempDir(), "export")
		writeFixture(t, executable, fmt.Sprintf("#!/bin/sh\nexit %d\n", code), 0700)
		rec := taskRecord{Config: a.cfg, Revision: a.cat.Revision}
		rec.Config.Python = executable
		_, _, err := taskChoiceFiles(context.Background(), root, rec, cat[0], "export")
		if got := taskSelectionStatus(err); got != code {
			t.Fatalf("export exit changed: %d -> %d", code, got)
		}
	}
}
