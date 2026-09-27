package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func spec(args []string) error {
	f := flag.NewFlagSet("spec", flag.ContinueOnError)
	source := f.String("source", "", "prepared source")
	dev := f.String("development", "", "development JSON case")
	held := f.String("holdout", "", "holdout JSON case")
	integration := f.String("integration", "AGENTS.md", "integration file")
	slot := f.String("slot", "src/record-summary", "tool slot")
	offline := f.Bool("offline", false, "explicit deterministic proposer fixture; no model calls")
	model := f.String("model", "", "explicit Hire model")
	effort := f.String("effort", "low", "Hire effort")
	names := []string{"go", "cage", "record", "hire", "ask"}
	flags := map[string]*string{}
	for _, n := range names {
		flags[n] = f.String(n, n, "selected public "+n+" executable")
	}
	if e := f.Parse(args); e != nil {
		return e
	}
	if *source == "" || *dev == "" || *held == "" {
		return errors.New("source, development and holdout are required")
	}
	if !*offline && *model == "" {
		return errors.New("select -offline or explicit -model")
	}
	if *offline && *model != "" {
		return errors.New("-offline cannot select a model")
	}
	s := settings{Slot: *slot, Integration: *integration, Tools: map[string]string{}, Model: *model, Effort: *effort, Fixture: *offline}
	paths, e := surface(s)
	if e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		return e
	}
	deps := []string{exe}
	for _, n := range names {
		if *offline && (n == "hire" || n == "ask") {
			continue
		}
		p, e := exec.LookPath(*flags[n])
		if e != nil {
			return e
		}
		p, e = filepath.Abs(p)
		if e != nil {
			return e
		}
		p, e = filepath.EvalSymlinks(p)
		if e != nil {
			return e
		}
		s.Tools[n] = p
		deps = append(deps, p)
	}
	if !*offline {
		// These are public executable selections, not component imports. Caller
		// wrapper scripts may require additional declared dependencies.
		for _, pair := range [][2]string{{"HIRE_AGENT", "agent"}, {"AGENT_ASK", "ask"}, {"AGENT_PLY", "ply"}, {"AGENT_RECORD", "record"}, {"AGENT_BRIEF", "brief"}, {"AGENT_CAGE", "cage"}} {
			selected := os.Getenv(pair[0])
			if selected == "" {
				selected = pair[1]
			}
			p, e := exec.LookPath(selected)
			if e != nil {
				return fmt.Errorf("pin %s: %w", pair[0], e)
			}
			p, e = filepath.Abs(p)
			if e != nil {
				return e
			}
			p, e = filepath.EvalSymlinks(p)
			if e != nil {
				return e
			}
			present := false
			for _, dep := range deps {
				if dep == p {
					present = true
				}
			}
			if !present {
				deps = append(deps, p)
			}
		}
	}
	src, e := filepath.Abs(*source)
	if e != nil {
		return e
	}
	if e = module(src, s); e != nil {
		return e
	}
	devPath, e := filepath.Abs(*dev)
	if e != nil {
		return e
	}
	heldPath, e := filepath.Abs(*held)
	if e != nil {
		return e
	}
	commands := map[string]any{}
	for _, name := range []string{"propose", "trial", "judge"} {
		commands[name] = map[string]any{"argv": []string{exe, name}}
	}
	return emit(map[string]any{"version": 1, "source": src, "mutable": paths, "development": []caseRef{{"development", "status-count-development", devPath}}, "holdout": []caseRef{{"holdout", "status-count-heldout", heldPath}}, "commands": commands, "dependencies": deps, "record": s.Tools["record"], "settings": s, "repeats": 2, "command_seconds": 600, "max_seconds": 3600})
}

func judge(r request) (int, error) {
	decision, reason, code := "discard", "All candidate cases must pass, every build and test must pass, and at least one baseline case must fail.", 1
	if len(r.Baseline) == 0 || len(r.Baseline) != len(r.Candidate) {
		return 2, errors.New("judge requires matched nonempty samples")
	}
	gap, valid := false, true
	seen := map[string]bool{}
	for i, b := range r.Baseline {
		c := r.Candidate[i]
		key := fmt.Sprintf("%s/%d", b.Case, b.Repeat)
		if seen[key] || b.Case != c.Case || b.Repeat != c.Repeat {
			return 2, errors.New("judge samples not matched")
		}
		seen[key] = true
		for _, s := range []sample{b, c} {
			if s.Observation.Score.BuildExit == nil || s.Observation.Score.TestExit == nil || *s.Observation.Score.BuildExit != 0 || *s.Observation.Score.TestExit != 0 {
				valid = false
			}
		}
		if c.Observation.Score.Accepted == nil || !*c.Observation.Score.Accepted {
			valid = false
		}
		if b.Observation.Score.Accepted == nil {
			valid = false
		} else if !*b.Observation.Score.Accepted {
			gap = true
		}
	}
	if valid && gap {
		decision, reason, code = "keep", "Candidate passed every frozen case; baseline compiled and tested but failed at least one case. This is capability evidence, not a cost or speed claim.", 0
	}
	return code, emit(map[string]any{"version": 1, "decision": decision, "reason": reason, "cost": 0})
}
