package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutomaticInputsAndPriorResultReuse(t *testing.T) {
	input := packetTest(t, "train")
	os.Remove(filepath.Join(input, "train.json"))
	os.RemoveAll(filepath.Join(input, "cases"))
	p, err := admit(input, "train")
	if err != nil || len(p.Cases) != 0 || p.Training.Kind != "knowledge" || p.Training.Scope != "private" {
		t.Fatalf("automatic teaching inputs: %+v %v", p, err)
	}
	writeTest(t, filepath.Join(input, "result.json"), `{"schema":1,"status":"accepted_on_cases"}`, 0600)
	writeTest(t, filepath.Join(input, "cases/old/work/old-output.txt"), "old work is not input", 0644)
	writeTest(t, filepath.Join(input, "evaluation/cases/fresh/goal.md"), "fresh goal", 0644)
	writeTest(t, filepath.Join(input, "evaluation/cases/fresh/case.json"), `{"purpose":"general","outputs":["answer.txt"]}`, 0644)
	writeTest(t, filepath.Join(input, "evaluation/cases/fresh/check"), "#!/bin/sh\nexit 1\n", 0755)
	p, err = admit(input, "eval")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Cases) != 1 || p.Cases[0].ID != "fresh" {
		t.Fatalf("old run results reused as cases: %+v", p.Cases)
	}
	if _, exists := p.Files["cases/old/work/old-output.txt"]; exists {
		t.Fatal("old result admitted as current input")
	}
	bare := t.TempDir()
	p, err = admitRequest(bare, "create", "Build a worker from this command-line outcome.")
	if err != nil || !strings.Contains(string(p.Files["REQUEST.md"].Data), "command-line") {
		t.Fatalf("optional request flag: %v", err)
	}
}

func TestEntryMetadataCannotSelectAuthorityOrEscapeDefinition(t *testing.T) {
	for _, data := range []string{
		`{"entry":"../run","args":[]}`,
		`{"entry":"/bin/sh","args":[]}`,
		`{"entry":"bin/run","args":["{unknown}"]}`,
		`{"entry":"bin/run","args":["{work}/../../source"]}`,
		`{"entry":"bin/run","args":[],"net":true}`,
		`{"entry":"bin/run","args":[],"env":{"AGENT_CAGE":"/bin/true"}}`,
		`{"entry":"bin/run","args":[],"requires":["/bin/sh"]}`,
	} {
		if _, err := parseExecution(map[string]content{"run.json": {[]byte(data), 0644}}); err == nil {
			t.Fatalf("unsafe entry metadata admitted: %s", data)
		}
	}
	e, err := parseExecution(map[string]content{"run.json": {[]byte(`{"entry":"bin/run","args":["{goal}","{work}","literal $(touch marker)","--mode=compact"],"requires":["python3"]}`), 0644}})
	if err != nil || e.Args[2] != "literal $(touch marker)" {
		t.Fatalf("literal argv changed: %+v %v", e, err)
	}
}

func evaluationRun(t *testing.T, scripts map[string]string) *run {
	t.Helper()
	r := fakeRun(t, scripts)
	input := packetTest(t, "train")
	p, err := admit(input, "train")
	if err != nil {
		t.Fatal(err)
	}
	r.packet = p
	r.config.action = "eval"
	r.config.output = t.TempDir()
	r.source = filepath.Join(r.root, "input")
	r.candidate = filepath.Join(r.root, "expert")
	if err := materialize(r.source, p.Files, true); err != nil {
		t.Fatal(err)
	}
	if err := materialize(r.candidate, subtree(p.Files, "expert"), false); err != nil {
		t.Fatal(err)
	}
	source, err := readTree(r.source)
	if err != nil {
		t.Fatal(err)
	}
	r.sourceSHA256 = treeDigest(source)
	if err := os.Mkdir(filepath.Join(r.root, "cases"), 0700); err != nil {
		t.Fatal(err)
	}
	r.evaluationFiles = map[string]content{}
	for name, file := range subtree(p.Files, "cases") {
		r.evaluationFiles["cases/"+name] = file
	}
	r.evaluationRoot = filepath.Join(r.root, "evaluation")
	if err := materialize(r.evaluationRoot, r.evaluationFiles, true); err != nil {
		t.Fatal(err)
	}
	evaluation, err := readTree(r.evaluationRoot)
	if err != nil {
		t.Fatal(err)
	}
	r.evaluationSHA256 = treeDigest(evaluation)
	r.capabilities = []byte("Use instructions and an exact-byte check; no specialist or application required.\n")
	return r
}

func TestExactCapabilityReviewRejectsDeclineAndSourceDrift(t *testing.T) {
	for _, mode := range []string{"approved", "declined", "drift", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			script := `cat > review-input.json; printf '{"approved":true,"reason":"bounded selected implementation"}\n'`
			switch mode {
			case "declined":
				script = `cat > review-input.json; printf '{"approved":false,"reason":"unconditional checker"}\n'`
			case "drift":
				script = `cat > review-input.json; printf '\nchanged after selection\n' >> expert/AGENTS.md; printf '{"approved":true,"reason":"old inspected bytes"}\n'`
			case "malformed":
				script = `cat > review-input.json; printf '{"approved":true}\n'`
			}
			r := evaluationRun(t, map[string]string{"ask": script})
			got := r.reviewEvaluation()
			want := 0
			if mode == "drift" {
				want = 125
			} else if mode != "approved" {
				want = 2
			}
			if got != want {
				t.Fatalf("%s review returned %d, want %d", mode, got, want)
			}
			b, err := os.ReadFile(filepath.Join(r.root, "review-input.json"))
			if err != nil || !strings.Contains(string(b), "definition_sha256") || !strings.Contains(string(b), "evaluation_sha256") {
				t.Fatal("review did not receive exact selection")
			}
		})
	}
}

func TestCheckedSnapshotSurvivesLateWorkerWrites(t *testing.T) {
	r := evaluationRun(t, map[string]string{
		"agent":  `printf 'accepted bytes\n' > answer.txt; (sleep 0.1; printf 'late changed bytes\n' > answer.txt) >/dev/null 2>&1 &`,
		"record": `sleep 0.3; test "$(cat answer.txt)" = 'accepted bytes'`,
		"cage":   "exit 9",
	})
	r.packet.Cases = r.packet.Cases[:1]
	if code := r.evaluate(); code != 0 {
		t.Fatalf("snapshot evaluation: %d", code)
	}
	verdict := r.result.Cases[0]
	raw, _ := os.ReadFile(filepath.Join(verdict.ExecutionWork, "answer.txt"))
	checked, _ := os.ReadFile(filepath.Join(verdict.Work, "answer.txt"))
	published, _ := os.ReadFile(filepath.Join(r.config.output, "cases", verdict.ID, "work", "answer.txt"))
	if string(raw) != "late changed bytes\n" || string(checked) != "accepted bytes\n" || string(published) != string(checked) {
		t.Fatalf("published different bytes than checked: raw=%q checked=%q published=%q", raw, checked, published)
	}
}

func TestExpectedRefusalRequiresIndependentEvidenceAndPreservesExit(t *testing.T) {
	for _, test := range []struct {
		name               string
		child, check, want int
		status             string
		checked            bool
	}{
		{"checked-refusal", 2, 0, 0, "accepted", true},
		{"wrong-refusal-reason", 2, 1, 2, "rejected", true},
		{"broken-refusal-check", 2, 7, 7, "broken", true},
		{"unexpected-success", 0, 0, 2, "unexpected_success", false},
		{"unknown-never-accepted", 125, 0, 125, "incomplete", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := evaluationRun(t, map[string]string{
				"agent":  fmt.Sprintf("printf 'missing capability\\n' > disposition.txt; exit %d", test.child),
				"record": fmt.Sprintf(`test "$TRAIT_EXECUTION_EXIT" = 2; test -f "$TRAIT_EXECUTION_STATUS"; test "$(cat disposition.txt)" = 'missing capability'; exit %d`, test.check),
				"cage":   "exit 9",
			})
			r.packet.Cases = r.packet.Cases[:1]
			r.packet.Cases[0].ExpectedExit = 2
			r.packet.Cases[0].Outputs = nil
			if got := r.evaluate(); got != test.want {
				t.Fatalf("exit %d, want %d", got, test.want)
			}
			c := r.result.Cases[0]
			if c.AgentExit != test.child || c.ExpectedExit != 2 || c.Status != test.status || (c.CheckExit != nil) != test.checked {
				t.Fatalf("incorrect refusal accounting: %+v", c)
			}
		})
	}
}

func TestAdmissionCannotExpectUnknownOrInfrastructureOutcomes(t *testing.T) {
	for _, code := range []int{0, 1, 2, 3, 75, 124, 125, 127, 130} {
		input := packetTest(t, "eval")
		writeTest(t, filepath.Join(input, "cases/transfer/case.json"), fmt.Sprintf(`{"purpose":"transfer","outputs":[],"expected_exit":%d}`, code), 0644)
		_, err := admit(input, "eval")
		if (err == nil) != (code == 2) {
			t.Fatalf("expected exit %d with no outputs: %v", code, err)
		}
	}
}
