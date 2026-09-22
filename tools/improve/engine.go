package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"time"
	"unicode/utf8"
)

type Observation struct {
	Version int             `json:"version"`
	Score   json.RawMessage `json:"score"`
	Cost    *float64        `json:"cost"`
}
type Change struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type Proposal struct {
	Version    int      `json:"version"`
	Hypothesis string   `json:"hypothesis"`
	Changes    []Change `json:"changes"`
	Cost       *float64 `json:"cost"`
}
type Verdict struct {
	Version  int      `json:"version"`
	Decision string   `json:"decision"`
	Reason   string   `json:"reason"`
	Cost     *float64 `json:"cost"`
}
type Sample struct {
	Case         string      `json:"case"`
	Repeat       int         `json:"repeat"`
	SourceSHA256 string      `json:"source_sha256"`
	InputSHA256  string      `json:"input_sha256"`
	Observation  Observation `json:"observation"`
	Evidence     string      `json:"evidence"`
}
type Comparison struct {
	Version   int             `json:"version"`
	Split     string          `json:"split"`
	Baseline  []Sample        `json:"baseline"`
	Candidate []Sample        `json:"candidate"`
	Settings  json.RawMessage `json:"settings"`
}
type Result struct {
	Version         int      `json:"version"`
	Decision        string   `json:"decision"`
	Reason          string   `json:"reason"`
	SourceSHA256    string   `json:"source_sha256"`
	CandidateSHA256 string   `json:"candidate_sha256,omitempty"`
	Proposal        string   `json:"proposal,omitempty"`
	Trials          int      `json:"trials"`
	Calls           []Call   `json:"calls"`
	KnownCost       float64  `json:"known_cost"`
	TotalCost       *float64 `json:"total_cost"`
}
type Experiment struct {
	spec     Spec
	out      string
	ctx      context.Context
	pins     map[string]Pin
	trees    map[string]map[string]Pin
	original string
	best     string
	result   Result
}

func cost(raw []byte, n *float64) error {
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return e
	}
	if _, ok := fields["cost"]; !ok {
		return fmt.Errorf("missing cost: use null when unknown")
	}
	if n != nil && (math.IsNaN(*n) || math.IsInf(*n, 0) || *n < 0) {
		return fmt.Errorf("invalid cost")
	}
	return nil
}
func (x *Experiment) pin(p string) error {
	v, e := filePin(p)
	if e == nil {
		x.pins[p] = v
	}
	return e
}
func (x *Experiment) guard() error {
	for p, want := range x.pins {
		got, e := filePin(p)
		if e != nil || got != want {
			return fmt.Errorf("pinned input changed: %s", p)
		}
	}
	for p, want := range x.trees {
		got, e := tree(p)
		if e != nil || !reflect.DeepEqual(got, want) {
			return fmt.Errorf("source snapshot changed: %s", p)
		}
	}
	return nil
}
func selected(s Spec) []string {
	p := []string{s.Source, s.Record}
	for _, c := range []Command{s.Commands.Propose, s.Commands.Trial, s.Commands.Judge} {
		p = append(p, c.Argv[0])
	}
	p = append(p, s.Dependencies...)
	for _, rows := range [][]Case{s.Development, s.Holdout} {
		for _, c := range rows {
			p = append(p, c.File)
		}
	}
	return p
}
func prepare(ctx context.Context, s Spec, out string) (*Experiment, error) {
	x := &Experiment{spec: s, out: out, ctx: ctx, pins: map[string]Pin{}, trees: map[string]map[string]Pin{}, result: Result{Version: 1, Decision: "inconclusive", Calls: []Call{}}}
	// Pin before creating any output. Check original dependencies after snapshots.
	original, e := tree(s.Source)
	if e != nil {
		return nil, e
	}
	x.trees[s.Source] = original
	x.result.SourceSHA256 = treeHash(original)
	for _, p := range selected(s)[1:] {
		if e = x.pin(p); e != nil {
			return nil, e
		}
	}
	if e = os.Mkdir(out, 0700); e != nil {
		return nil, e
	}
	x.original = filepath.Join(out, "baseline")
	x.best = x.original
	if e = copyTree(s.Source, x.original, original); e != nil {
		return x, e
	}
	x.trees[x.original] = original
	if e = os.Mkdir(filepath.Join(out, "cases"), 0700); e != nil {
		return x, e
	}
	for split, rows := range [][]Case{x.spec.Development, x.spec.Holdout} {
		for i := range rows {
			c := &rows[i]
			raw, e := readFile(c.File, maxJSON)
			if e != nil {
				return x, e
			}
			dest := filepath.Join(out, "cases", fmt.Sprintf("%d-%d", split, i))
			if e = writeNew(dest, raw, 0600); e != nil {
				return x, e
			}
			c.File = dest
			if e = x.pin(dest); e != nil {
				return x, e
			}
		}
	}
	if e = save(filepath.Join(out, "spec.json"), x.spec); e != nil {
		return x, e
	}
	if e = x.pin(filepath.Join(out, "spec.json")); e != nil {
		return x, e
	}
	if e = save(filepath.Join(out, "pins.json"), map[string]any{"files": x.pins, "sources": x.trees}); e != nil {
		return x, e
	}
	if e = x.pin(filepath.Join(out, "pins.json")); e != nil {
		return x, e
	}
	return x, x.guard()
}
func (x *Experiment) invoke(name string, c Command, request any) ([]byte, int, error) {
	if e := x.guard(); e != nil {
		return nil, 2, e
	}
	if e := x.ctx.Err(); e != nil {
		return nil, 1, e
	}
	dir := filepath.Join(x.out, name)
	if e := os.Mkdir(dir, 0700); e != nil {
		return nil, 2, e
	}
	for _, p := range []string{"work", "evidence"} {
		if e := os.Mkdir(filepath.Join(dir, p), 0700); e != nil {
			return nil, 2, e
		}
	}
	raw := marshal(request)
	if len(raw) > maxJSON {
		return nil, 2, fmt.Errorf("command input exceeds limit")
	}
	if e := writeNew(filepath.Join(dir, "stdin"), raw, 0600); e != nil {
		return nil, 2, e
	}
	stdout, e := os.OpenFile(filepath.Join(dir, "stdout"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, 2, e
	}
	defer stdout.Close()
	stderr, e := os.OpenFile(filepath.Join(dir, "stderr"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, 2, e
	}
	defer stderr.Close()
	argv := append([]string{x.spec.Record, "run", "-f", filepath.Join(dir, "process.jsonl"), "-grace", "2s", "--"}, c.Argv...)
	idx := len(x.result.Calls)
	x.result.Calls = append(x.result.Calls, Call{Directory: name, Argv: c.Argv, Exit: -1})
	ctx, cancel := context.WithTimeout(x.ctx, time.Duration(x.spec.CommandSeconds)*time.Second)
	defer cancel()
	fmt.Fprintln(os.Stderr, "improve:", name)
	outBound := &streamLimit{writer: stdout, cancel: cancel}
	errBound := &streamLimit{writer: stderr, cancel: cancel}
	code, interrupted, e := process(ctx, argv, dir, bytes.NewReader(raw), outBound, errBound)
	x.result.Calls[idx].Exit = code
	if outBound.exceeded || errBound.exceeded {
		return nil, 2, fmt.Errorf("command stream exceeds %d bytes; no retry: %s", maxJSON, name)
	}
	if e != nil || interrupted {
		return nil, 1, fmt.Errorf("%w; no retry: %s", errIncomplete, name)
	}
	if e = x.guard(); e != nil {
		return nil, 2, e
	}
	for _, stream := range []string{"stdout", "stderr"} {
		if _, e := readFile(filepath.Join(dir, stream), maxJSON); e != nil {
			return nil, 2, e
		}
	}
	checkCtx, stop := context.WithTimeout(x.ctx, 30*time.Second)
	defer stop()
	if e = replay(checkCtx, x.spec.Record, dir, x.result.Calls[idx]); e != nil {
		return nil, 2, e
	}
	retained, e := tree(dir)
	if e != nil {
		return nil, 2, e
	}
	x.trees[dir] = retained
	x.result.Calls[idx].Complete = true
	output, e := readFile(filepath.Join(dir, "stdout"), maxJSON)
	return output, code, e
}
func (x *Experiment) setCost(n *float64) { x.result.Calls[len(x.result.Calls)-1].Cost = n }
func (x *Experiment) trial(source string, c Case, repeat int, name string) (Sample, error) {
	sourcePins := x.trees[source]
	trialSource := filepath.Join(x.out, name+"-source")
	if e := copyTree(source, trialSource, sourcePins); e != nil {
		return Sample{}, e
	}
	x.trees[trialSource] = sourcePins
	request := map[string]any{"version": 1, "source": trialSource, "source_sha256": treeHash(sourcePins), "case": c, "repeat": repeat, "settings": x.spec.Settings, "work": filepath.Join(x.out, name, "work"), "evidence": filepath.Join(x.out, name, "evidence")}
	x.result.Trials++
	raw, code, e := x.invoke(name, x.spec.Commands.Trial, request)
	if e != nil {
		return Sample{}, e
	}
	if code != 0 {
		return Sample{}, fmt.Errorf("trial adapter failed (exit %d); job failure must be a scored observation", code)
	}
	var o Observation
	if e = decode(raw, &o); e != nil {
		return Sample{}, e
	}
	if o.Version != 1 || len(o.Score) == 0 || bytes.Equal(o.Score, []byte("null")) {
		return Sample{}, fmt.Errorf("invalid trial observation")
	}
	if e = cost(raw, o.Cost); e != nil {
		return Sample{}, e
	}
	x.setCost(o.Cost)
	return Sample{c.ID, repeat, treeHash(sourcePins), x.pins[c.File].SHA256, o, filepath.Join(x.out, name)}, nil
}
func (x *Experiment) compare(split string, cases []Case) (Verdict, error) {
	data := Comparison{Version: 1, Split: split, Settings: x.spec.Settings, Baseline: []Sample{}, Candidate: []Sample{}}
	for r := 0; r < x.spec.Repeats; r++ {
		for i, c := range cases {
			arms := []string{"baseline", "candidate"}
			if r%2 == 1 {
				arms[0], arms[1] = arms[1], arms[0]
			}
			for _, arm := range arms {
				source := x.original
				if arm == "candidate" {
					source = x.best
				}
				sample, e := x.trial(source, c, r, fmt.Sprintf("%s-%d-%d-%s", split, r, i, arm))
				if e != nil {
					return Verdict{}, e
				}
				if arm == "baseline" {
					data.Baseline = append(data.Baseline, sample)
				} else {
					data.Candidate = append(data.Candidate, sample)
				}
			}
		}
	}
	if e := save(filepath.Join(x.out, split+"-scores.json"), data); e != nil {
		return Verdict{}, e
	}
	if e := x.pin(filepath.Join(x.out, split+"-scores.json")); e != nil {
		return Verdict{}, e
	}
	raw, code, e := x.invoke(split+"-judge", x.spec.Commands.Judge, data)
	if e != nil {
		return Verdict{}, e
	}
	var v Verdict
	if e = decode(raw, &v); e != nil {
		return v, e
	}
	if v.Version != 1 || v.Reason == "" || !((v.Decision == "keep" && code == 0) || (v.Decision == "discard" && code == 1)) {
		return v, fmt.Errorf("judge decision/exit mismatch")
	}
	if e = cost(raw, v.Cost); e != nil {
		return v, e
	}
	x.setCost(v.Cost)
	if e = save(filepath.Join(x.out, split+"-decision.json"), v); e != nil {
		return v, e
	}
	if e = x.pin(filepath.Join(x.out, split+"-decision.json")); e != nil {
		return v, e
	}
	return v, nil
}
func (x *Experiment) execute() error {
	diagnostics := []Sample{}
	for r := 0; r < x.spec.Repeats; r++ {
		for i, c := range x.spec.Development {
			v, e := x.trial(x.original, c, r, fmt.Sprintf("diagnostic-%d-%d", r, i))
			if e != nil {
				return e
			}
			diagnostics = append(diagnostics, v)
		}
	}
	texts := map[string]string{}
	for _, p := range x.spec.Mutable {
		b, e := readFile(filepath.Join(x.original, p), maxJSON)
		if e != nil {
			return e
		}
		texts[p] = string(b)
	}
	request := map[string]any{"version": 1, "source": x.original, "source_sha256": x.result.SourceSHA256, "files": texts, "development": x.spec.Development, "observations": diagnostics, "settings": x.spec.Settings, "work": filepath.Join(x.out, "propose", "work"), "evidence": filepath.Join(x.out, "propose", "evidence")}
	raw, code, e := x.invoke("propose", x.spec.Commands.Propose, request)
	if e != nil {
		return e
	}
	if code == 1 {
		if len(bytes.TrimSpace(raw)) > 0 {
			var none Proposal
			if e = decode(raw, &none); e != nil {
				return e
			}
			if none.Version != 1 || len(none.Changes) != 0 {
				return fmt.Errorf("invalid no-proposal response")
			}
			if e = cost(raw, none.Cost); e != nil {
				return e
			}
			x.setCost(none.Cost)
		}
		x.result.Decision = "keep_baseline"
		x.result.Reason = "proposer returned no proposal"
		return nil
	}
	if code != 0 {
		return fmt.Errorf("proposer failed (exit %d)", code)
	}
	var p Proposal
	if e = decode(raw, &p); e != nil {
		return e
	}
	if p.Version != 1 || p.Hypothesis == "" || len(p.Changes) == 0 || len(p.Changes) > len(texts) {
		return fmt.Errorf("invalid proposal")
	}
	if e = cost(raw, p.Cost); e != nil {
		return e
	}
	x.setCost(p.Cost)
	seen := map[string]bool{}
	changed := false
	for _, c := range p.Changes {
		old, ok := texts[c.Path]
		if !ok || seen[c.Path] || !utf8.ValidString(c.Content) {
			return fmt.Errorf("proposal changed unselected file")
		}
		seen[c.Path] = true
		changed = changed || old != c.Content
	}
	if !changed {
		x.result.Decision = "keep_baseline"
		x.result.Reason = "proposal did not change source"
		return nil
	}
	candidate := filepath.Join(x.out, "candidate")
	if e = copyTree(x.original, candidate, x.trees[x.original]); e != nil {
		return e
	}
	for _, c := range p.Changes {
		path := filepath.Join(candidate, c.Path)
		if e = os.WriteFile(path, []byte(c.Content), 0600); e != nil {
			return e
		}
	}
	pins, e := tree(candidate)
	if e != nil {
		return e
	}
	x.trees[candidate] = pins
	x.best = candidate
	x.result.CandidateSHA256 = treeHash(pins)
	if e = save(filepath.Join(x.out, "candidate.json"), map[string]any{"hypothesis": p.Hypothesis, "sha256": x.result.CandidateSHA256, "changes": p.Changes}); e != nil {
		return e
	}
	if e = x.pin(filepath.Join(x.out, "candidate.json")); e != nil {
		return e
	}
	v, e := x.compare("development", x.spec.Development)
	if e != nil {
		return e
	}
	if v.Decision != "keep" {
		x.result.Decision = "keep_baseline"
		x.result.Reason = "development: " + v.Reason
		return nil
	}
	if e = save(filepath.Join(x.out, "freeze.json"), map[string]any{"candidate_sha256": x.result.CandidateSHA256, "before_holdout": true}); e != nil {
		return e
	}
	if e = x.pin(filepath.Join(x.out, "freeze.json")); e != nil {
		return e
	}
	v, e = x.compare("holdout", x.spec.Holdout)
	if e != nil {
		return e
	}
	if v.Decision != "keep" {
		x.result.Decision = "keep_baseline"
		x.result.Reason = "holdout: " + v.Reason
		return nil
	}
	if e = x.guard(); e != nil {
		return e
	}
	proposal := filepath.Join(x.out, "proposal")
	if e = os.Mkdir(proposal, 0700); e != nil {
		return e
	}
	if e = copyTree(candidate, filepath.Join(proposal, "source"), pins); e != nil {
		return e
	}
	if e = save(filepath.Join(proposal, "manifest.json"), map[string]any{"version": 1, "baseline_sha256": x.result.SourceSHA256, "candidate_sha256": x.result.CandidateSHA256, "comparison": "../holdout-decision.json", "source": "source", "status": "supported proposal; not applied"}); e != nil {
		return e
	}
	x.result.Decision = "supported"
	x.result.Reason = v.Reason
	x.result.Proposal = "proposal/source"
	return nil
}
func (x *Experiment) finish() (err error) {
	wroteResult := false
	defer func() {
		if err != nil {
			x.result.Decision = "invalid_evidence"
			x.result.Reason = "could not finalize retained evidence: " + err.Error()
			if x.result.Proposal != "" {
				_ = os.RemoveAll(filepath.Join(x.out, "proposal"))
			}
			x.result.Proposal = ""
			if wroteResult {
				temp := filepath.Join(x.out, "invalid-result.json")
				if writeNew(temp, marshal(x.result), 0600) == nil {
					_ = os.Rename(temp, filepath.Join(x.out, "result.json"))
				}
			}
		}
	}()
	if x.result.Decision == "supported" || x.result.Decision == "keep_baseline" {
		if e := x.guard(); e != nil {
			return e
		}
	}
	known := 0.0
	complete := true
	for _, c := range x.result.Calls {
		if c.Cost == nil {
			complete = false
		} else {
			known += *c.Cost
			if math.IsInf(known, 0) {
				return fmt.Errorf("total cost exceeds numeric range")
			}
		}
	}
	x.result.KnownCost = known
	if complete {
		x.result.TotalCost = &known
	}
	if e := save(filepath.Join(x.out, "result.json"), x.result); e != nil {
		return e
	}
	wroteResult = true
	pins, e := artifacts(x.out)
	if e != nil {
		return e
	}
	return save(filepath.Join(x.out, "inventory.json"), Inventory{1, pins, x.result.Calls})
}
