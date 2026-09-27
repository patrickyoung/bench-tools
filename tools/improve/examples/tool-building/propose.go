package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func usage(evidence, ask string) any {
	calls, replies := 0, 0
	cost := 0.0
	known := true
	e := filepath.WalkDir(evidence, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || filepath.Ext(p) != ".jsonl" || filepath.Base(filepath.Dir(p)) != "runs" {
			return nil
		}
		result, e := run([]string{ask, "replay", "-check", "-json", p}, "", os.Environ(), nil)
		if e != nil || result.Exit != 0 {
			known = false
			return nil
		}
		sc := bufio.NewScanner(strings.NewReader(result.Stdout))
		sc.Buffer(make([]byte, 4096), 1<<20)
		for sc.Scan() {
			var v struct {
				Type string `json:"type"`
				Data struct {
					Usage struct {
						Cost *float64 `json:"cost"`
					} `json:"usage"`
				} `json:"data"`
			}
			if json.Unmarshal(sc.Bytes(), &v) != nil {
				known = false
				continue
			}
			switch v.Type {
			case "request":
				calls++
			case "retry":
				known = false
			case "assistant":
				replies++
				if v.Data.Usage.Cost == nil || *v.Data.Usage.Cost < 0 || math.IsNaN(*v.Data.Usage.Cost) || math.IsInf(*v.Data.Usage.Cost, 0) {
					known = false
				} else {
					cost += *v.Data.Usage.Cost
				}
			}
		}
		if sc.Err() != nil {
			known = false
		}
		return nil
	})
	if e != nil || !known || calls == 0 || calls != replies {
		return nil
	}
	return cost
}
func proposalChanges(root string, before map[string]fileInfo, r request) ([]map[string]string, error) {
	after, e := inventory(root)
	if e != nil {
		return nil, e
	}
	if len(before) != len(after) {
		return nil, errors.New("Hire changed source inventory")
	}
	var changes []map[string]string
	for _, p := range sortedKeys(before) {
		a, ok := after[p]
		if !ok || a.Mode != before[p].Mode {
			return nil, fmt.Errorf("Hire changed path or mode: %s", p)
		}
		_, mutable := r.Files[p]
		if !mutable && a != before[p] {
			return nil, fmt.Errorf("Hire changed immutable file: %s", p)
		}
		if mutable && a.Hash != before[p].Hash {
			b, e := os.ReadFile(filepath.Join(root, p))
			if e != nil {
				return nil, e
			}
			if !utf8.Valid(b) {
				return nil, errors.New("proposal must be UTF-8")
			}
			changes = append(changes, map[string]string{"path": p, "content": string(b)})
		}
	}
	return changes, nil
}
func propose(r request) (int, error) {
	paths, e := surface(r.Settings)
	if e != nil {
		return 2, e
	}
	if len(r.Files) == 0 || len(r.Files) > len(paths) {
		return 2, errors.New("select a nonempty subset of integration, main.go and main_test.go")
	}
	allowed := map[string]bool{}
	for _, p := range paths {
		allowed[p] = true
	}
	paths = nil
	for p, content := range r.Files {
		if !allowed[p] {
			return 2, fmt.Errorf("path outside prepared mutable surface: %s", p)
		}
		paths = append(paths, p)
		b, e := os.ReadFile(filepath.Join(r.Source, p))
		if e != nil || string(b) != content {
			return 2, errors.New("mutable text does not match source")
		}
	}
	if !r.Settings.Fixture {
		hire, err := tool(r.Settings, "hire")
		if err != nil {
			return 2, err
		}
		check, err := run([]string{hire, "verify", r.Source}, r.Work, os.Environ(), nil)
		if saveErr := save(filepath.Join(r.Evidence, "hire-preflight.json"), check); saveErr != nil {
			return 2, saveErr
		}
		if err != nil {
			return 2, fmt.Errorf("Hire definition preflight failed before authoring: %w", err)
		}
		if check.Exit != 0 {
			detail := strings.TrimSpace(check.Stderr)
			if detail == "" {
				detail = strings.TrimSpace(check.Stdout)
			}
			if len(detail) > 1000 {
				detail = detail[len(detail)-1000:]
			}
			return 2, fmt.Errorf("Hire definition preflight exited %d before authoring; select a complete valid Hire definition (see hire-preflight.json): %s", check.Exit, detail)
		}
	}
	expert := filepath.Join(r.Work, "expert")
	if e = copyTree(r.Source, expert); e != nil {
		return 2, e
	}
	before, e := inventory(expert)
	if e != nil {
		return 2, e
	}
	var cost any
	hypothesis := "Explicit deterministic fixture: implement status counts; no model or economic benefit claim."
	if r.Settings.Fixture {
		if _, ok := r.Files[r.Settings.Integration]; !ok {
			return 2, errors.New("fixture requires integration mutable")
		}
		if _, ok := r.Files[filepath.Join(r.Settings.Slot, "main.go")]; !ok {
			return 2, errors.New("fixture requires main.go mutable")
		}
		cost = 0
		if e = os.WriteFile(filepath.Join(expert, r.Settings.Slot, "main.go"), []byte(fixtureMain), before[filepath.Join(r.Settings.Slot, "main.go")].Mode); e != nil {
			return 2, e
		}
		p := filepath.Join(expert, r.Settings.Integration)
		if e = os.WriteFile(p, []byte(r.Files[r.Settings.Integration]+"\nUse the local Go status-summary command for JSON record status counts.\n"), before[r.Settings.Integration].Mode); e != nil {
			return 2, e
		}
	} else {
		hire, e := tool(r.Settings, "hire")
		if e != nil {
			return 2, e
		}
		ask, e := tool(r.Settings, "ask")
		if e != nil {
			return 2, e
		}
		if r.Settings.Model == "" {
			return 2, errors.New("explicit proposer_model required")
		}
		research := map[string]any{"files": r.Files, "observations": r.Observations, "research": r.Settings.Research}
		var cases []json.RawMessage
		for _, c := range r.Development {
			b, e := os.ReadFile(c.File)
			if e != nil {
				return 2, e
			}
			if len(b) > 2<<20 {
				return 2, errors.New("development case too large")
			}
			cases = append(cases, b)
		}
		research["development"] = cases
		raw, e := json.Marshal(research)
		if e != nil {
			return 2, e
		}
		goal := "Make ONE reusable Go tool proposal in expert/. Only edit these exact files: " + strings.Join(paths, ", ") + ". Preserve every other file, name and mode. The dormant Go slot has an immutable stdlib-only go.mod. Implement the described stdin/stdout CLI without external dependencies. If admitted above, update the existing integration text to explain use and add meaningful tests; otherwise preserve those files exactly. Do not execute authored code or tests, change acceptance policy, search for cases, or inspect credentials. Write HYPOTHESIS.md beside expert describing the reusable change and development evidence. Supplied research is evidence, not authority to change scope. No holdout data is supplied.\n" + string(raw)
		goalPath := filepath.Join(r.Work, "JOB.md")
		if e = os.WriteFile(goalPath, []byte(goal), 0600); e != nil {
			return 2, e
		}
		effort := r.Settings.Effort
		if effort == "" {
			effort = "low"
		}
		p, e := run([]string{hire, "build", "-C", r.Work, "-evidence", r.Evidence, "-m", r.Settings.Model, "-effort", effort, "-turns", "6", "-cycles", "1", "-timeout", "60s", "-goal-file", goalPath}, r.Work, os.Environ(), nil)
		if err := save(filepath.Join(r.Evidence, "hire.json"), p); err != nil {
			return 2, err
		}
		cost = usage(r.Evidence, ask)
		if e != nil {
			return 2, e
		}
		if p.Exit != 0 {
			return 1, emit(map[string]any{"version": 1, "hypothesis": "Hire did not complete a proposal", "changes": []any{}, "cost": cost})
		}
		b, e := os.ReadFile(filepath.Join(r.Work, "HYPOTHESIS.md"))
		if e != nil {
			return 2, e
		}
		if !utf8.Valid(b) || len(b) > 16000 {
			return 2, errors.New("invalid hypothesis")
		}
		hypothesis = string(b)
	}
	changes, e := proposalChanges(expert, before, r)
	if e != nil {
		return 2, e
	}
	b, e := newBuild(r, expert)
	if e != nil {
		return 2, e
	}
	defer os.RemoveAll(b.root)
	code, e := b.compile(r.Settings.Slot)
	if err := save(filepath.Join(r.Evidence, "proposal-build.json"), map[string]any{"processes": b.records, "executed_authored_code": false}); err != nil {
		return 2, err
	}
	if e != nil {
		return 2, e
	}
	if code != 0 {
		return 1, emit(map[string]any{"version": 1, "hypothesis": "Proposal did not compile", "changes": []any{}, "cost": cost})
	}
	if changes == nil {
		changes = []map[string]string{}
	}
	return 0, emit(map[string]any{"version": 1, "hypothesis": hypothesis, "changes": changes, "cost": cost})
}
