package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func emit(v any) {
	if e := json.NewEncoder(os.Stdout).Encode(v); e != nil {
		panic(e)
	}
}
func TestChild(t *testing.T) {
	if os.Getenv("IMPROVE_FIXTURE") != "1" {
		return
	}
	at := 0
	for i, v := range os.Args {
		if v == "--" {
			at = i + 1
			break
		}
	}
	if at == 0 {
		return
	}
	args := os.Args[at:]
	mode := args[0]
	if mode == "record" {
		verb := args[1]
		file := ""
		stream := ""
		start := 0
		for i, v := range args {
			if v == "-f" {
				file = args[i+1]
			}
			if v == "-stream" {
				stream = args[i+1]
			}
			if v == "--" {
				start = i + 1
				break
			}
		}
		if verb == "run" {
			input, _ := io.ReadAll(os.Stdin)
			cmd := exec.Command(args[start], args[start+1:]...)
			cmd.Stdin = bytes.NewReader(input)
			var out, errout bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errout
			err := cmd.Run()
			code := 0
			if err != nil {
				code = cmd.ProcessState.ExitCode()
			}
			av := []string{}
			for _, v := range args[start:] {
				av = append(av, base64.StdEncoding.EncodeToString([]byte(v)))
			}
			doc := map[string]any{"intent": map[string]any{"argv": av}, "terminal": map[string]any{"complete": true, "started": true, "exit": code, "signal": 0, "interrupted": false}, "streams": map[string][]byte{"stdin": input, "stdout": out.Bytes(), "stderr": errout.Bytes()}}
			_ = os.WriteFile(file, marshal(doc), 0600)
			_, _ = os.Stdout.Write(out.Bytes())
			_, _ = os.Stderr.Write(errout.Bytes())
			os.Exit(code)
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			os.Exit(125)
		}
		var doc struct {
			Intent   any               `json:"intent"`
			Terminal any               `json:"terminal"`
			Streams  map[string][]byte `json:"streams"`
		}
		_ = json.Unmarshal(raw, &doc)
		if stream != "" {
			_, _ = os.Stdout.Write(doc.Streams[stream])
		} else {
			emit(map[string]any{"intent": doc.Intent, "terminal": doc.Terminal})
		}
		os.Exit(0)
	}
	raw, _ := io.ReadAll(os.Stdin)
	scenario := os.Getenv("IMPROVE_SCENARIO")
	switch mode {
	case "propose":
		var request struct {
			Files       map[string]string `json:"files"`
			Development []Case            `json:"development"`
		}
		_ = json.Unmarshal(raw, &request)
		if bytes.Contains(raw, []byte("reserved-secret")) {
			fmt.Fprintln(os.Stderr, "leaked holdout")
			os.Exit(2)
		}
		if scenario == "none" {
			os.Exit(1)
		}
		path := "AGENTS.md"
		if scenario == "unauthorized" {
			path = "bin/check"
		}
		if scenario == "traversal" {
			path = "../outside"
		}
		content := "precise"
		if scenario == "duplicate" {
			content = request.Files[path]
		}
		emit(map[string]any{"version": 1, "hypothesis": "use a precise method", "changes": []map[string]any{{"path": path, "content": content}}, "cost": 0.5})
	case "trial":
		var request struct {
			Source string `json:"source"`
			Case   Case   `json:"case"`
		}
		_ = json.Unmarshal(raw, &request)
		b, _ := os.ReadFile(filepath.Join(request.Source, "AGENTS.md"))
		candidate := string(b) == "precise"
		if scenario == "timeout" {
			time.Sleep(30 * time.Second)
		}
		if scenario == "finalization" {
			cwd, _ := os.Getwd()
			// A selected command leaves a reserved output name occupied. The
			// controller must return JSON even when final inventory fails.
			_ = os.Mkdir(filepath.Join(filepath.Dir(cwd), "inventory.json"), 0700)
		}
		if scenario == "broken" {
			os.Exit(3)
		}
		if scenario == "mutate" {
			_ = os.WriteFile(filepath.Join(request.Source, "bin/check"), []byte("changed"), 0700)
		}
		if scenario == "prior-evidence" {
			cwd, _ := os.Getwd()
			prior := filepath.Join(filepath.Dir(cwd), "diagnostic-0-0", "stdout")
			if filepath.Base(cwd) != "diagnostic-0-0" {
				_ = os.WriteFile(prior, []byte("changed"), 0600)
			}
		}
		if scenario == "dependency" {
			_ = os.WriteFile(os.Getenv("IMPROVE_DEPENDENCY"), []byte("changed"), 0600)
		}
		if scenario == "missing-cost" {
			fmt.Print(`{"version":1,"score":{"correct":true}}`)
			os.Exit(0)
		}
		if scenario == "malformed" {
			fmt.Print(`{"version":1,"version":1,"score":{},"cost":0}`)
			os.Exit(0)
		}
		correct := true
		cost := 5.0
		if candidate {
			cost = 3
		}
		if candidate && ((scenario == "reject-dev") || (scenario == "reject-holdout" && request.Case.Family == "reserved-secret")) {
			correct = false
		}
		var dollars any = cost
		if scenario == "unknown" {
			dollars = nil
		}
		emit(map[string]any{"version": 1, "score": map[string]any{"correct": correct}, "cost": dollars})
	case "judge":
		var c Comparison
		_ = json.Unmarshal(raw, &c)
		keep := true
		for i, b := range c.Candidate {
			var score struct {
				Correct bool `json:"correct"`
			}
			_ = json.Unmarshal(b.Observation.Score, &score)
			if !score.Correct || b.Observation.Cost == nil || c.Baseline[i].Observation.Cost == nil || *b.Observation.Cost >= *c.Baseline[i].Observation.Cost {
				keep = false
			}
		}
		decision := "keep"
		code := 0
		if !keep {
			decision = "discard"
			code = 1
		}
		if scenario == "judge-mismatch" {
			code = 1
		}
		emit(Verdict{1, decision, "fixture gate", new(float64)})
		os.Exit(code)
	default:
		os.Exit(2)
	}
	os.Exit(0)
}
func fixture(t *testing.T) (Spec, string) {
	t.Helper()
	t.Setenv("IMPROVE_FIXTURE", "1")
	// These helpers exit hundreds of times; keep race checking without its
	// default one-second post-exit delay on every fixture subprocess.
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "source")
	if err = os.MkdirAll(filepath.Join(source, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(source, "AGENTS.md"), []byte("plain"), 0644)
	_ = os.WriteFile(filepath.Join(source, "bin/check"), []byte("original check"), 0755)
	dev := filepath.Join(base, "dev.json")
	held := filepath.Join(base, "held.json")
	_ = os.WriteFile(dev, []byte(`{"input":"development"}`), 0600)
	_ = os.WriteFile(held, []byte(`{"input":"reserved-secret"}`), 0600)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(base, "record")
	_ = os.WriteFile(record, []byte("#!/bin/sh\nexec '"+strings.ReplaceAll(self, "'", "'\\''")+"' -test.run=TestChild -- record \"$@\"\n"), 0700)
	command := func(mode string) Command { return Command{[]string{self, "-test.run=TestChild", "--", mode}} }
	dep := filepath.Join(base, "dependency")
	_ = os.WriteFile(dep, []byte("fixed evaluator"), 0600)
	t.Setenv("IMPROVE_DEPENDENCY", dep)
	s := Spec{Version: 1, Source: source, Mutable: []string{"AGENTS.md"}, Development: []Case{{"dev", "direct", dev}}, Holdout: []Case{{"held", "reserved-secret", held}}, Commands: Commands{command("propose"), command("trial"), command("judge")}, Dependencies: []string{dep}, Record: record, Repeats: 2, CommandSeconds: 10, MaxSeconds: 60}
	return s, filepath.Join(base, "run")
}
func executeFixture(t *testing.T, s Spec, dest string) (Result, int) {
	t.Helper()
	var out, errout bytes.Buffer
	code := run([]string{"-o", dest}, bytes.NewReader(marshal(s)), &out, &errout)
	var r Result
	if out.Len() > 0 {
		if e := json.Unmarshal(out.Bytes(), &r); e != nil {
			t.Fatal(e, out.String())
		}
	}
	if code == 2 && out.Len() == 0 {
		t.Fatalf("preflight failed: %s", errout.String())
	}
	return r, code
}
func TestSupportedAndOfflineVerification(t *testing.T) {
	s, dest := fixture(t)
	r, code := executeFixture(t, s, dest)
	if code != 0 || r.Decision != "supported" || r.Trials != 10 || r.TotalCost == nil || *r.TotalCost != 42.5 {
		t.Fatalf("unexpected result: %+v code %d", r, code)
	}
	b, _ := os.ReadFile(filepath.Join(s.Source, "AGENTS.md"))
	if string(b) != "plain" {
		t.Fatal("source overwritten")
	}
	b, _ = os.ReadFile(filepath.Join(dest, "proposal/source/AGENTS.md"))
	if string(b) != "precise" {
		t.Fatal("wrong exported candidate")
	}
	i, _ := os.Stat(filepath.Join(dest, "proposal/source/bin/check"))
	if i.Mode().Perm() != 0755 {
		t.Fatal("mode changed")
	}
	if e := verify(context.Background(), dest, s.Record); e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(filepath.Join(dest, "candidate/AGENTS.md"), []byte("tampered"), 0644)
	if e := verify(context.Background(), dest, s.Record); e == nil {
		t.Fatal("accepted changed candidate")
	}
}
func TestRejectAndUnknown(t *testing.T) {
	for _, scenario := range []string{"reject-dev", "reject-holdout", "unknown", "duplicate", "none"} {
		t.Run(scenario, func(t *testing.T) {
			s, d := fixture(t)
			t.Setenv("IMPROVE_SCENARIO", scenario)
			r, code := executeFixture(t, s, d)
			if code != 1 || r.Decision != "keep_baseline" {
				t.Fatalf("%d %+v", code, r)
			}
			if _, e := os.Stat(filepath.Join(d, "proposal")); !os.IsNotExist(e) {
				t.Fatal("exported rejected proposal")
			}
			if scenario != "reject-holdout" {
				if _, e := os.Stat(filepath.Join(d, "holdout-scores.json")); !os.IsNotExist(e) {
					t.Fatal("spent holdout")
				}
			}
			if scenario == "unknown" && (r.TotalCost != nil || r.KnownCost != 0.5) {
				t.Fatal("unknown billing treated as zero")
			}
			if e := verify(context.Background(), d, s.Record); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestBrokenEvidence(t *testing.T) {
	for _, scenario := range []string{"unauthorized", "traversal", "mutate", "dependency", "prior-evidence", "missing-cost", "malformed", "broken", "judge-mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			s, d := fixture(t)
			t.Setenv("IMPROVE_SCENARIO", scenario)
			r, code := executeFixture(t, s, d)
			if code != 2 || r.Decision != "invalid_evidence" {
				t.Fatalf("%d %+v", code, r)
			}
			if _, e := os.Stat(filepath.Join(d, "proposal")); !os.IsNotExist(e) {
				t.Fatal("exported invalid evidence")
			}
		})
	}
}
func TestPlanNoWritesAndNoCalls(t *testing.T) {
	s, d := fixture(t)
	t.Setenv("IMPROVE_SCENARIO", "broken")
	var out, errout bytes.Buffer
	if code := run([]string{"-n", "-o", d}, bytes.NewReader(marshal(s)), &out, &errout); code != 0 {
		t.Fatal(errout.String())
	}
	if _, e := os.Stat(d); !os.IsNotExist(e) {
		t.Fatal("plan wrote files")
	}
	if !strings.Contains(out.String(), `"max_worker_trials": 10`) {
		t.Fatal(out.String())
	}
}
func TestValidation(t *testing.T) {
	s, d := fixture(t)
	tests := []struct {
		name   string
		change func(*Spec)
	}{
		{"mutable escape", func(s *Spec) { s.Mutable = []string{"../AGENTS.md"} }},
		{"mutable missing", func(s *Spec) { s.Mutable = []string{"missing"} }},
		{"overlap family", func(s *Spec) { s.Holdout[0].Family = s.Development[0].Family }},
		{"overlap bytes", func(s *Spec) { s.Holdout[0].File = s.Development[0].File }},
		{"zero cases", func(s *Spec) { s.Holdout = nil }},
		{"limits", func(s *Spec) { s.Repeats = 11 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Spec
			_ = json.Unmarshal(marshal(s), &c)
			tt.change(&c)
			if _, e := validateSpec(marshal(c)); e == nil {
				t.Fatal("accepted invalid spec")
			}
		})
	}
	for _, raw := range []string{`{"version":1,"version":1}`, `{"Version":1}`, `{} {}`, `null`} {
		if _, e := validateSpec([]byte(raw)); e == nil {
			t.Fatal("accepted ambiguous JSON")
		}
	}
	if _, e := outputPath(filepath.Join(s.Source, "run"), selected(s)); e == nil {
		t.Fatal("accepted output inside source")
	}
	_ = os.Mkdir(d, 0700)
	if _, e := outputPath(d, selected(s)); e == nil {
		t.Fatal("accepted existing output")
	}
	link := filepath.Join(filepath.Dir(d), "link")
	_ = os.Symlink(s.Source, link)
	s.Source = link
	if _, e := validateSpec(marshal(s)); e == nil {
		t.Fatal("accepted symlink source")
	}
}
func TestTimeoutNeverRetries(t *testing.T) {
	s, d := fixture(t)
	s.CommandSeconds = 1
	t.Setenv("IMPROVE_SCENARIO", "timeout")
	r, code := executeFixture(t, s, d)
	if code != 1 || r.Decision != "inconclusive" || len(r.Calls) != 1 || r.TotalCost != nil {
		t.Fatalf("%d %+v", code, r)
	}
	if e := verify(context.Background(), d, s.Record); e == nil {
		t.Fatal("accepted incomplete call")
	}
}

func TestArtifactAndCostBounds(t *testing.T) {
	s, dest := fixture(t)
	if e := save(dest, strings.Repeat("x", maxJSON)); e == nil {
		t.Fatal("wrote unreadable oversized artifact")
	}
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("oversized save wrote a file")
	}
	if e := os.Mkdir(dest, 0700); e != nil {
		t.Fatal(e)
	}
	huge := 1e308
	x := &Experiment{spec: s, out: dest, result: Result{Version: 1, Decision: "invalid_evidence", Calls: []Call{{Cost: &huge}, {Cost: &huge}}}}
	if e := x.finish(); e == nil {
		t.Fatal("accepted cost overflow")
	}
}

func TestSourceCopyAdmission(t *testing.T) {
	s, dest := fixture(t)
	// Each individual source is legal, but its immutable copies cannot fit.
	f, e := os.Create(filepath.Join(s.Source, "padding"))
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(41 << 20); e != nil {
		t.Fatal(e)
	}
	f.Close()
	for _, args := range [][]string{{"-n"}, {"-o", dest}} {
		var out, diag bytes.Buffer
		if code := run(args, bytes.NewReader(marshal(s)), &out, &diag); code != 2 || !strings.Contains(diag.String(), "planned source copies") {
			t.Fatalf("%d %s", code, diag.String())
		}
		if _, e = os.Stat(dest); !os.IsNotExist(e) {
			t.Fatal("admission created output")
		}
	}
}

func TestStudyLargerThanSingleTree(t *testing.T) {
	s, dest := fixture(t)
	f, e := os.Create(filepath.Join(s.Source, "padding"))
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(11 << 20); e != nil {
		t.Fatal(e)
	}
	f.Close()
	r, code := executeFixture(t, s, dest)
	if code != 0 || r.Decision != "supported" {
		t.Fatalf("%d %+v", code, r)
	}
	if e = verify(context.Background(), dest, s.Record); e != nil {
		t.Fatal(e)
	}
}

func TestFinalizationFailureEmitsTerminalJSON(t *testing.T) {
	s, dest := fixture(t)
	t.Setenv("IMPROVE_SCENARIO", "finalization")
	r, code := executeFixture(t, s, dest)
	if code != 2 || r.Decision != "invalid_evidence" || r.Proposal != "" || !strings.Contains(r.Reason, "finalize") {
		t.Fatalf("%d %+v", code, r)
	}
	if _, e := os.Stat(filepath.Join(dest, "proposal")); !os.IsNotExist(e) {
		t.Fatal("left supported proposal")
	}
	raw, e := os.ReadFile(filepath.Join(dest, "result.json"))
	if e != nil || !bytes.Equal(raw, marshal(r)) {
		t.Fatal("retained result disagrees with stdout")
	}
}

func TestStreamOverflowCancelsBeforeFollowingWork(t *testing.T) {
	// Exercise the actual process-group cancellation, for both output streams.
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var output bytes.Buffer
			bound := &streamLimit{writer: &output, cancel: cancel}
			out, diag := io.Writer(io.Discard), io.Writer(io.Discard)
			if stream == "stdout" {
				out = bound
			} else {
				diag = bound
			}
			redirect := ""
			if stream == "stderr" {
				redirect = " >&2"
			}
			// The old post-exit size check would retain all bytes and create marker.
			script := "dd if=/dev/zero bs=1048576 count=3" + redirect + "; sleep 5; touch marker"
			_, interrupted, _ := process(ctx, []string{"/bin/sh", "-c", script}, dir, nil, out, diag)
			if !bound.exceeded || output.Len() != maxJSON || !interrupted {
				t.Fatalf("overflow not bounded: %d %v %v", output.Len(), bound.exceeded, interrupted)
			}
			if _, e := os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(e) {
				t.Fatal("command continued after overflow")
			}
		})
	}
}

func TestOversizedCallLedgerRejectedBeforeExecution(t *testing.T) {
	s, dest := fixture(t)
	for i := 0; i < 27; i++ {
		for _, c := range []*Command{&s.Commands.Trial, &s.Commands.Propose, &s.Commands.Judge} {
			c.Argv = append(c.Argv, strings.Repeat("x", 8000))
		}
	}
	for _, args := range [][]string{{"-n"}, {"-o", dest}} {
		var out, diag bytes.Buffer
		code := run(args, bytes.NewReader(marshal(s)), &out, &diag)
		if code != 2 || !strings.Contains(diag.String(), "planned call ledger") || out.Len() != 0 {
			t.Fatalf("%d %s", code, diag.String())
		}
		if _, e := os.Stat(dest); !os.IsNotExist(e) {
			t.Fatal("admission wrote output")
		}
	}
}

func TestInventoryExcludedFromItsOwnAggregateBound(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < artifactFiles; i++ {
		if e := os.WriteFile(filepath.Join(dir, fmt.Sprint(i)), nil, 0600); e != nil {
			t.Fatal(e)
		}
	}
	before, e := artifacts(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "inventory.json"), []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	after, e := artifacts(dir)
	if e != nil || !bytes.Equal(marshal(before), marshal(after)) {
		t.Fatalf("inventory changed admission: %v", e)
	}
}
