package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPolicyAllowRecordsCompleteEventChain(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "allow"
	code, stdout, stderr := fixture.run(t, ` { "x" : 1 } `)
	if code != 0 || stdout != "{\"ok\":true}\n" || stderr != "connector progress\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := mustRead(t, fixture.input); got != `{"x":1}` {
		t.Fatalf("connector input=%q", got)
	}
	want := []string{"action.proposal/v1", "action.decision/v1", "action.attempt/v1", "action.sent/v1", "action.result/v1"}
	if got := eventKinds(t, fixture.events); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("event kinds=%v want %v", got, want)
	}
	result := eventBody[resultReceipt](t, fixture.events, "action.result/v1")
	if !result.Sent || result.ExitCode != 0 || result.Outcome != "completed" || string(result.Stdout) != "{\"ok\":true}\n" {
		t.Fatalf("result=%#v", result)
	}
}

func TestVersionFollowsSuiteContract(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.String() != "action 0.1.0\n" || stderr.String() != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestPolicyAllowDoesNotResolveOrInvokeMay(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "allow"
	fixture.may = filepath.Join(fixture.dir, "missing-may")
	code, stdout, stderr := fixture.run(t, `{"x":1}`)
	if code != 0 || stdout != "{\"ok\":true}\n" || stderr != "connector progress\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	decision := eventBody[decisionReceipt](t, fixture.events, "action.decision/v1")
	if decision.MayPath != "" || decision.MayResult != nil {
		t.Fatalf("allow decision unexpectedly resolved May: %#v", decision)
	}
}

func TestReviewParksInMayBeforeConnectorStarts(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "review"
	fixture.mayVerdict = "parked"
	code, stdout, stderr := fixture.run(t, `{"x":1}`)
	if code != 75 || stdout != "" || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(fixture.input); !os.IsNotExist(err) {
		t.Fatalf("connector ran before approval: %v", err)
	}
	want := []string{"action.proposal/v1", "action.decision/v1"}
	if got := eventKinds(t, fixture.events); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("event kinds=%v want %v", got, want)
	}
	decision := eventBody[decisionReceipt](t, fixture.events, "action.decision/v1")
	if decision.Authorized || decision.MayExitCode != 75 || decision.MayResult == nil || decision.MayResult.Verdict != "parked" {
		t.Fatalf("decision=%#v", decision)
	}
	if decision.MayPath == "" || !strings.Contains(decision.MayResult.Action, `"job":"job-1"`) {
		t.Fatalf("decision does not bind May and job: %#v", decision)
	}
}

func TestUnexpectedTerminalAfterSentIsUncertainAndNotReceiptedAsResult(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "allow"
	fixture.connectorExit = "9"
	code, stdout, stderr := fixture.run(t, `{"x":1}`)
	if code != 125 || stdout != "" || !strings.Contains(stderr, "effects may exist") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	want := []string{"action.proposal/v1", "action.decision/v1", "action.attempt/v1", "action.sent/v1"}
	if got := eventKinds(t, fixture.events); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("event kinds=%v want %v", got, want)
	}
}

func TestReceiptFailureAfterEffectIsUncertainAndSuppressesResult(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "allow"
	fixture.askFailKind = "action.result/v1"
	code, stdout, stderr := fixture.run(t, `{"x":1}`)
	if code != 125 || stdout != "" || !strings.Contains(stderr, "record terminal result") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := mustRead(t, fixture.input); got != `{"x":1}` {
		t.Fatalf("connector did not receive effectful request: %q", got)
	}
}

func TestAttemptReceiptFailureBeforeReleaseIsBrokenNotUncertain(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "allow"
	fixture.askFailKind = "action.attempt/v1"
	code, stdout, stderr := fixture.run(t, `{"x":1}`)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "before request release") || strings.Contains(stderr, "effects may exist") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if raw, err := os.ReadFile(fixture.input); err == nil && len(raw) != 0 {
		t.Fatalf("connector received input without attempt receipt: %q", raw)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestDeadlineBeforeReleaseIsBrokenNotUncertain(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "allow"
	fixture.timeout = "10ms"
	fixture.askDelayMS = "50"
	code, stdout, stderr := fixture.run(t, `{"x":1}`)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "deadline before request release") || strings.Contains(stderr, "effects may exist") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestNonObjectInputIsRefusedBeforeProposal(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "allow"
	code, stdout, stderr := fixture.run(t, `[1,2,3]`)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "cannot unmarshal array") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(fixture.events); !os.IsNotExist(err) {
		t.Fatalf("invalid input created events: %v", err)
	}
}

func TestProposalFileSelectsConnectorAndCanonicalInput(t *testing.T) {
	fixture := newFixture(t)
	fixture.policyDecision = "allow"
	proposalPath := filepath.Join(fixture.dir, "proposal.json")
	if err := os.WriteFile(proposalPath, []byte(` {"input":{"z":2,"a":1},"connector":"publish","version":1} `), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := fixture.runProposal(t, proposalPath)
	if code != 0 || stdout != "{\"ok\":true}\n" || stderr != "connector progress\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := mustRead(t, fixture.input); got != `{"a":1,"z":2}` {
		t.Fatalf("connector input=%q", got)
	}
	var inspected bytes.Buffer
	if code := run([]string{"inspect", proposalPath}, strings.NewReader(""), &inspected, &bytes.Buffer{}); code != 0 {
		t.Fatalf("inspect exit=%d", code)
	}
	if got, want := inspected.String(), "{\"version\":1,\"connector\":\"publish\",\"input\":{\"a\":1,\"z\":2}}\n"; got != want {
		t.Fatalf("inspect=%q want %q", got, want)
	}
}

func TestProposalRejectsUnknownFieldsBeforeProposalEvent(t *testing.T) {
	fixture := newFixture(t)
	proposalPath := filepath.Join(fixture.dir, "proposal.json")
	if err := os.WriteFile(proposalPath, []byte(`{"version":1,"connector":"publish","input":{},"command":"bypass"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := fixture.runProposal(t, proposalPath)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unknown field") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(fixture.events); !os.IsNotExist(err) {
		t.Fatalf("invalid proposal created events: %v", err)
	}
}

func TestProposalRejectsDuplicateFields(t *testing.T) {
	fixture := newFixture(t)
	proposalPath := filepath.Join(fixture.dir, "proposal.json")
	if err := os.WriteFile(proposalPath, []byte(`{"version":1,"connector":"publish","connector":"other","input":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", proposalPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.String() != "" || !strings.Contains(stderr.String(), "duplicate JSON field") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

type fixture struct {
	dir, connector, policy, may, ask string
	events, input                    string
	policyDecision                   string
	mayVerdict                       string
	connectorExit                    string
	askFailKind                      string
	askDelayMS                       string
	timeout                          string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	f := &fixture{
		dir: dir, connector: filepath.Join(dir, "publish"), policy: filepath.Join(dir, "policy"),
		may: filepath.Join(dir, "may"), ask: filepath.Join(dir, "ask"), events: filepath.Join(dir, "events.jsonl"),
		input: filepath.Join(dir, "connector-input"), policyDecision: "allow", mayVerdict: "spent", connectorExit: "0",
		timeout: "0",
	}
	writeHelper(t, f.connector, "connector")
	writeHelper(t, f.policy, "policy")
	writeHelper(t, f.may, "may")
	writeHelper(t, f.ask, "ask")
	return f
}

func writeHelper(t *testing.T, path, role string) {
	t.Helper()
	body := fmt.Sprintf("#!/bin/sh\nACTION_TEST_ROLE=%s; export ACTION_TEST_ROLE\nexec %q -test.run=TestHelperProcess -- \"$@\"\n", role, os.Args[0])
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) run(t *testing.T, input string) (int, string, string) {
	t.Helper()
	t.Setenv("ACTION_PATH", f.dir)
	t.Setenv("ACTION_TEST_EVENTS", f.events)
	t.Setenv("ACTION_TEST_INPUT", f.input)
	t.Setenv("ACTION_TEST_POLICY_DECISION", f.policyDecision)
	t.Setenv("ACTION_TEST_MAY_VERDICT", f.mayVerdict)
	t.Setenv("ACTION_TEST_CONNECTOR_EXIT", f.connectorExit)
	t.Setenv("ACTION_TEST_ASK_FAIL_KIND", f.askFailKind)
	t.Setenv("ACTION_TEST_ASK_DELAY_MS", f.askDelayMS)
	var stdout, stderr bytes.Buffer
	code := run([]string{"run", "-job", "job-1", "-policy", f.policy, "-may", f.may, "-record", filepath.Join(f.dir, "session.jsonl"), "-ask", f.ask, "-timeout", f.timeout, "publish"},
		strings.NewReader(input), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func (f *fixture) runProposal(t *testing.T, proposalPath string) (int, string, string) {
	t.Helper()
	t.Setenv("ACTION_PATH", f.dir)
	t.Setenv("ACTION_TEST_EVENTS", f.events)
	t.Setenv("ACTION_TEST_INPUT", f.input)
	t.Setenv("ACTION_TEST_POLICY_DECISION", f.policyDecision)
	t.Setenv("ACTION_TEST_MAY_VERDICT", f.mayVerdict)
	t.Setenv("ACTION_TEST_CONNECTOR_EXIT", f.connectorExit)
	t.Setenv("ACTION_TEST_ASK_FAIL_KIND", f.askFailKind)
	t.Setenv("ACTION_TEST_ASK_DELAY_MS", f.askDelayMS)
	var stdout, stderr bytes.Buffer
	code := run([]string{"run", "-job", "job-1", "-policy", f.policy, "-may", f.may, "-record", filepath.Join(f.dir, "session.jsonl"), "-ask", f.ask, "-proposal", proposalPath},
		strings.NewReader("ignored"), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestHelperProcess(t *testing.T) {
	role := os.Getenv("ACTION_TEST_ROLE")
	if role == "" {
		return
	}
	args := argsAfterSeparator(os.Args)
	switch role {
	case "connector":
		if len(args) != 1 {
			os.Exit(2)
		}
		switch args[0] {
		case "describe":
			fmt.Print(`{"version":1,"name":"publish","description":"publish one exact object","input_schema":{"type":"object"}}`)
		case "run":
			raw, _ := ioReadAll(os.Stdin)
			_ = os.WriteFile(os.Getenv("ACTION_TEST_INPUT"), raw, 0o600)
			fmt.Fprintln(os.Stderr, "connector progress")
			fmt.Println(`{"ok":true}`)
			var code int
			_, _ = fmt.Sscan(os.Getenv("ACTION_TEST_CONNECTOR_EXIT"), &code)
			os.Exit(code)
		default:
			os.Exit(2)
		}
	case "policy":
		raw, _ := ioReadAll(os.Stdin)
		decision := os.Getenv("ACTION_TEST_POLICY_DECISION")
		result := policyResult{Version: 1, ActionSHA256: digest(raw), Decision: decision, Reason: "fixture " + decision}
		encoded, _ := json.Marshal(result)
		fmt.Printf("%s\n", encoded)
		os.Exit(map[string]int{"allow": 0, "deny": 3, "review": 75}[decision])
	case "may":
		raw, _ := ioReadAll(os.Stdin)
		verdict := os.Getenv("ACTION_TEST_MAY_VERDICT")
		job := args[len(args)-1]
		result := mayResult{Version: 1, Job: job, Digest: mayDigest(job, string(raw)), Action: string(raw), Verdict: verdict}
		encoded, _ := json.Marshal(result)
		fmt.Printf("%s\n", encoded)
		os.Exit(map[string]int{"spent": 0, "declined": 3, "parked": 75}[verdict])
	case "ask":
		var delayMS int
		_, _ = fmt.Sscan(os.Getenv("ACTION_TEST_ASK_DELAY_MS"), &delayMS)
		if delayMS > 0 {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
		}
		kind := flagValue(args, "-k")
		if kind == os.Getenv("ACTION_TEST_ASK_FAIL_KIND") {
			os.Exit(1)
		}
		raw, _ := ioReadAll(os.Stdin)
		line, _ := json.Marshal(map[string]any{"kind": kind, "body": json.RawMessage(raw)})
		file, err := os.OpenFile(os.Getenv("ACTION_TEST_EVENTS"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(2)
		}
		_, _ = fmt.Fprintf(file, "%s\n", line)
		_ = file.Close()
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func argsAfterSeparator(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return nil
}

func flagValue(args []string, name string) string {
	for i := range args {
		if args[i] == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func eventKinds(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var out []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		out = append(out, event.Kind)
	}
	return out
}

func eventBody[T any](t *testing.T, path, kind string) T {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event struct {
			Kind string          `json:"kind"`
			Body json.RawMessage `json:"body"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.Kind == kind {
			var body T
			if err := json.Unmarshal(event.Body, &body); err != nil {
				t.Fatal(err)
			}
			return body
		}
	}
	t.Fatalf("event %s not found", kind)
	var zero T
	return zero
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func ioReadAll(r *os.File) ([]byte, error) {
	var out bytes.Buffer
	_, err := out.ReadFrom(r)
	return out.Bytes(), err
}
