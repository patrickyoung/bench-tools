package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Opt in with independently built public programs. The proposer is a fixture;
// no provider calls, credentials, or source-library mutation are involved.
func TestPublicImprove(t *testing.T) {
	bins := os.Getenv("IMPROVE_TOOL_TEST_BIN")
	if bins == "" {
		t.Skip("set IMPROVE_TOOL_TEST_BIN to built improve, record, ask, cage, trail, hire, agent, brief binaries; requires Python and researcher source")
	}
	bins, err := filepath.Abs(bins)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"improve", "record", "ask", "cage", "trail", "hire", "agent", "brief"} {
		if _, err := os.Stat(filepath.Join(bins, name)); err != nil {
			t.Fatal(err)
		}
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	goTool, err = filepath.EvalSymlinks(goTool)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + bins + string(os.PathListSeparator) + filepath.Dir(goTool) + ":/usr/bin:/bin", "HOME=" + home, "TMPDIR=" + root, "LANG=C", "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOENV=off"}
	call := func(want int, input []byte, argv ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Env = env
		cmd.Stdin = bytes.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			if x, ok := err.(*exec.ExitError); ok {
				code = x.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if code != want {
			t.Fatalf("%v: exit %d, want %d\n%s\n%s", argv, code, want, stdout.String(), stderr.String())
		}
		return stdout.Bytes()
	}
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	encode := func(v any) []byte {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	adapter := filepath.Join(root, "tool-building")
	call(0, nil, goTool, "build", "-o", adapter, ".")
	original := filepath.Join(root, "original")
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	const originalInstructions = "Count status values in a JSON array of records.\n"
	write(filepath.Join(original, "AGENTS.md"), []byte(originalInstructions))
	write(filepath.Join(original, "README.md"), []byte("Synthetic status-counting definition; independent Improve cases evaluate the CLI.\n"))
	if err := os.Mkdir(filepath.Join(original, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(original, "bin", "check"), []byte("#!/bin/sh\n# Dormant worker job; the independent Improve oracle evaluates this CLI.\nexit 1\n"))
	if err := os.Chmod(filepath.Join(original, "bin", "check"), 0700); err != nil {
		t.Fatal(err)
	}
	call(0, nil, filepath.Join(bins, "hire"), "verify", original)
	prepared := filepath.Join(root, "prepared")
	call(0, nil, adapter, "prepare", "-source", original, "-out", prepared)
	// Agent reserves tools/ for executable entries. Preparing Go source must
	// preserve the real Hire/Agent definition contract before model authoring.
	call(0, nil, filepath.Join(bins, "hire"), "verify", prepared)
	before, err := inventory(prepared)
	if err != nil {
		t.Fatal(err)
	}
	dev, held := filepath.Join(root, "development.json"), filepath.Join(root, "holdout.json")
	write(dev, encode(testCase{Stdin: `[{"status":"done"},{"status":"blocked"},{"status":"done"}]`, Stdout: "{\"blocked\":1,\"done\":2}\n"}))
	write(held, encode(testCase{Stdin: `[{"status":"queued"},{"status":"review"},{"status":"queued"}]`, Stdout: "{\"queued\":2,\"review\":1}\n"}))
	specBytes := call(0, nil, adapter, "spec", "-source", prepared, "-development", dev, "-holdout", held, "-go", goTool, "-cage", filepath.Join(bins, "cage"), "-record", filepath.Join(bins, "record"), "-offline")
	var spec map[string]any
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		t.Fatal(err)
	}
	spec["repeats"] = 2
	specBytes = encode(spec)
	call(0, specBytes, filepath.Join(bins, "improve"), "-n")
	// Assemble a report-v2 proposal through the actual researcher checker and
	// its public Trail/Ask/Improve readers. This is a synthetic research claim
	// with a real sealed process receipt, not a model-quality evaluation.
	archive := filepath.Join(root, "history")
	researchWork := filepath.Join(root, "research")
	for _, dir := range []string{archive, researchWork} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	const quote = "Offline fixture: count JSON record status values."
	session := filepath.Join(archive, "fixture.jsonl")
	call(0, nil, filepath.Join(bins, "record"), "run", "-f", session, "-ask", filepath.Join(bins, "ask"), "-label", quote, "--", "/bin/echo", "fixture")
	var sequence float64
	for _, line := range bytes.Split(call(0, nil, filepath.Join(bins, "trail"), "show", session), []byte("\n")) {
		if !bytes.Contains(line, []byte(quote)) {
			continue
		}
		var row struct {
			Event struct {
				Seq float64 `json:"seq"`
			} `json:"event"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		sequence = row.Event.Seq
	}
	if sequence == 0 {
		t.Fatal("public Trail did not expose fixture receipt label")
	}
	researchRequest := encode(map[string]any{"version": 1, "question": "Test one small Go status-counting tool.", "sources": []any{map[string]any{"id": "history", "kind": "archive", "path": archive}}, "tools": map[string]string{"trail": filepath.Join(bins, "trail"), "ask": filepath.Join(bins, "ask"), "improve": filepath.Join(bins, "improve")}, "template": spec, "fresh_holdout": true})
	requestPath := filepath.Join(root, "research-request.json")
	write(requestPath, researchRequest)
	sessionBytes, err := os.ReadFile(session)
	if err != nil {
		t.Fatal(err)
	}
	selected := []string{"AGENTS.md", "src/record-summary/main.go"}
	report := map[string]any{"version": 2, "request_sha256": hash(researchRequest), "status": "ready", "summary": "Synthetic contract exercise using a sealed local process receipt.", "hypothesis": "A deterministic Go command can satisfy independent status-count cases.", "change": map[string]any{"kind": "tool", "paths": selected, "instruction": "Implement status counts and document its invocation."}, "citations": []any{map[string]any{"source": "history", "file": "fixture.jsonl", "sha256": hash(sessionBytes), "seq": sequence, "quote": quote}}, "limitations": []string{"Fixture evidence does not establish worker adoption or efficiency."}, "next_inputs": []string{}}
	write(filepath.Join(researchWork, "research.json"), encode(report))
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	checker, err := filepath.Abs("../../../../workers/experiment-researcher/expert/bin/check")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	check := exec.CommandContext(ctx, python, checker, "--assemble")
	check.Dir = researchWork
	check.Env = append(env, "EXPERIMENT_RESEARCH_REQUEST="+requestPath, "PYTHONDONTWRITEBYTECODE=1")
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("researcher assembly: %v\n%s", err, output)
	}
	specBytes, err = os.ReadFile(filepath.Join(researchWork, "experiment.json"))
	if err != nil {
		t.Fatal(err)
	}
	var assembled map[string]any
	if err := json.Unmarshal(specBytes, &assembled); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encode(assembled["mutable"]), encode(selected)) {
		t.Fatal("researcher did not narrow paths")
	}
	assembled["mutable"] = spec["mutable"]
	delete(assembled["settings"].(map[string]any), "research")
	if !bytes.Equal(encode(assembled), encode(spec)) {
		t.Fatal("researcher changed frozen study policy")
	}
	for _, reject := range []bool{false, true} {
		name, want, decision := "supported", 0, "supported"
		if reject {
			name, want, decision = "rejected", 1, "keep_baseline"
			// A fresh invocation freezes a deliberately incompatible held-out
			// oracle. It must not leak into proposal authoring or be repaired.
			write(held, encode(testCase{Stdin: `[]`, Stdout: "wrong independent oracle\n"}))
		}
		out := filepath.Join(root, name)
		var result map[string]any
		if err := json.Unmarshal(call(want, specBytes, filepath.Join(bins, "improve"), "-o", out), &result); err != nil {
			t.Fatal(err)
		}
		if result["decision"] != decision {
			t.Fatalf("unexpected decision: %v", result)
		}
		_, err := os.Stat(filepath.Join(out, "proposal", "source"))
		if (!reject && err != nil) || (reject && !os.IsNotExist(err)) {
			t.Fatalf("export mismatch: %v", err)
		}
		if !reject {
			candidate, err := inventory(filepath.Join(out, "candidate"))
			if err != nil {
				t.Fatal(err)
			}
			exported, err := inventory(filepath.Join(out, "proposal", "source"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encode(candidate), encode(exported)) {
				t.Fatal("export differs from tested source")
			}
		}
		call(0, nil, filepath.Join(bins, "improve"), "verify", "-record", filepath.Join(bins, "record"), out)
		after, err := inventory(prepared)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encode(before), encode(after)) {
			t.Fatal("study modified prepared source")
		}
		write(filepath.Join(out, "candidate", "AGENTS.md"), []byte("tampered\n"))
		call(2, nil, filepath.Join(bins, "improve"), "verify", "-record", filepath.Join(bins, "record"), out)
	}
	unchanged, err := os.ReadFile(filepath.Join(original, "AGENTS.md"))
	if err != nil || string(unchanged) != originalInstructions {
		t.Fatal("original source changed")
	}
	if _, err := os.Stat(filepath.Join(original, "src")); !os.IsNotExist(err) {
		t.Fatal("tool slot leaked into original")
	}
}
