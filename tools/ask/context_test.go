package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/patrickyoung/ask/internal/event"
	"github.com/patrickyoung/ask/internal/provider"
)

func TestContextUsesProviderUsageAndPendingMessages(t *testing.T) {
	dir, calls, _ := fake(t, 200, answerWire)
	exec(t, "", "-q", "goal")
	path, _ := event.Current(dir)
	read := func() contextState {
		t.Helper()
		code, stdout, stderr := exec(t, "", "context", "-json", "-limit", "1000", path)
		if code != 0 || stderr != "" {
			t.Fatalf("context: exit=%d err=%q", code, stderr)
		}
		var c contextState
		if err := json.Unmarshal([]byte(stdout), &c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	c := read()
	if c.EstimatedTokens != 15 || c.Basis != "provider_usage" || c.Headroom == nil || *c.Headroom != 985 {
		t.Fatalf("initial context: %+v", c)
	}
	exec(t, "", "note", "-q", "-s", "test", "-f", path, strings.Repeat("not context", 100))
	if next := read(); next.EstimatedTokens != c.EstimatedTokens {
		t.Fatal("note changed context use")
	}
	exec(t, "new observation", "append", "-q", "-s", "ply", "-f", path)
	c = read()
	if c.PendingBytes <= len("new observation") || c.PendingMessages != 1 || c.EstimatedTokens != 15+c.PendingBytes+32 || calls.Load() != 1 {
		t.Fatalf("pending observation not accounted for: %+v calls=%d", c, calls.Load())
	}
}

func TestConditionalCompactSkipsProviderBelowBudget(t *testing.T) {
	dir, calls, _ := fake(t, 200, answerWire)
	exec(t, "", "-q", "goal")
	path, _ := event.Current(dir)
	before, _ := os.ReadFile(path)
	code, stdout, stderr := exec(t, "", "compact", "-q", "-at", "16", path)
	if code != 0 || strings.TrimSpace(stdout) != path || stderr != "" || calls.Load() != 1 || len(sessions(t, dir)) != 1 {
		t.Fatalf("below threshold: exit=%d out=%q err=%q calls=%d", code, stdout, stderr, calls.Load())
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("threshold query changed source")
	}
	code, stdout, stderr = exec(t, "", "compact", "-q", "-at", "15", path)
	if code != 0 || strings.TrimSpace(stdout) == path || calls.Load() != 2 || len(sessions(t, dir)) != 3 {
		t.Fatalf("at threshold: exit=%d out=%q err=%q calls=%d", code, stdout, stderr, calls.Load())
	}
}

func TestConditionalCompactCountsAppendedObservation(t *testing.T) {
	dir, calls, _ := fake(t, 200, answerWire)
	exec(t, "", "-q", "goal")
	path, _ := event.Current(dir)
	exec(t, strings.Repeat("x", 200), "append", "-q", "-s", "ply", "-f", path)
	code, stdout, stderr := exec(t, "", "compact", "-q", "-at", "100", path)
	if code != 0 || strings.TrimSpace(stdout) == path || calls.Load() != 2 {
		t.Fatalf("pending observation did not trigger: exit=%d out=%q err=%q", code, stdout, stderr)
	}
}

func TestConditionalCompactPreservesCurrentWithRelativeDirectory(t *testing.T) {
	dir, _, _ := fake(t, 200, answerWire)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, dir)
	if err != nil {
		t.Fatal(err)
	}
	exec(t, "", "-q", "-d", relative, "goal")
	code, stdout, stderr := exec(t, "", "compact", "-q", "-d", relative, "-at", "15")
	if code != 0 {
		t.Fatal(stderr)
	}
	path := strings.TrimSpace(stdout)
	current, err := event.Current(dir)
	if err != nil || current != path || !filepath.IsAbs(path) {
		t.Fatalf("current=%q compacted=%q err=%v", current, path, err)
	}
}

func TestContextLegacyAndMissingUsageAreExplicit(t *testing.T) {
	log, err := event.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	header(log, "anthropic/legacy", "system")
	log.Append(event.User, event.UserData{Text: "goal"})
	log.Sync()
	c, err := contextUsage(events(t, log.Path()))
	if err != nil || c.Basis != "serialized_bytes" || c.EstimatedTokens <= len("systemgoal") {
		t.Fatalf("without usage: %+v err=%v", c, err)
	}
	log.Append(event.Assistant, event.Turn{Usage: provider.Usage{In: 10, Out: 5, CacheRead: 100, CacheWrite: 20, Reasoning: 2}})
	log.Sync()
	c, err = contextUsage(events(t, log.Path()))
	if err != nil || c.Basis != "legacy_usage_allowance" || c.EstimatedTokens != 137 {
		t.Fatalf("legacy usage: %+v err=%v", c, err)
	}
}

func TestContextAccountsForLatestRequestSystem(t *testing.T) {
	log, err := event.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	header(log, "anthropic/test", "original system")
	log.Append(event.User, event.UserData{Text: "goal"})
	appendRequest := func(system string) {
		log.Sync()
		msgs, err := event.Fold(events(t, log.Path()))
		if err != nil {
			t.Fatal(err)
		}
		log.Append(event.Request, (provider.Request{Model: "test", System: system, Messages: msgs}).Logged())
		log.Sync()
	}
	appendRequest("replacement without usage")
	c, err := contextUsage(events(t, log.Path()))
	if err != nil || c.SystemBytes != len("replacement without usage") || c.EstimatedTokens != c.PendingBytes+32+len("replacement without usage") {
		t.Fatalf("unmeasured replacement system: %+v err=%v", c, err)
	}
	log.Append(event.Assistant, event.Turn{Usage: provider.Usage{In: 10, Out: 5, ContextTokens: 15}})
	appendRequest("next failed request changes system")
	c, err = contextUsage(events(t, log.Path()))
	if err != nil || c.PendingSystemBytes != len("next failed request changes system") || c.EstimatedTokens != 15+c.PendingSystemBytes {
		t.Fatalf("system change after usage: %+v err=%v", c, err)
	}
	if err := event.Check(events(t, log.Path())); err != nil {
		t.Fatal(err)
	}
}

func TestRepeatedCompactionCarriesHandoffAndRecentEvidence(t *testing.T) {
	const handoff = "Goal: publish release after review; constraint: no duplicate sends; unresolved publication effect; job handle: upload-7; next: verify status."
	wire := strings.ReplaceAll(answerWire, "the answer", handoff)
	dir, _, bodies := fake(t, 200, wire)
	exec(t, "", "-q", handoff)
	path, _ := event.Current(dir)
	for i := 0; i < 3; i++ {
		observation := "Observed verifier result " + strconv.Itoa(i) + ": pending; upload-7 status remains uncertain."
		if code, _, stderr := exec(t, observation, "append", "-q", "-s", "ply", "-f", path); code != 0 {
			t.Fatal(stderr)
		}
		before, _ := os.ReadFile(path)
		code, stdout, stderr := exec(t, "", "compact", "-q", path)
		if code != 0 {
			t.Fatal(stderr)
		}
		if after, _ := os.ReadFile(path); string(after) != string(before) {
			t.Fatal("compaction changed source")
		}
		request := (*bodies)[len(*bodies)-1]
		for _, want := range []string{handoff, observation, "Preserve the active goal in full", "repeated compactions", "uncertain external effects"} {
			if !strings.Contains(request, want) {
				t.Fatalf("handoff %d lost %q in request", i, want)
			}
		}
		path = strings.TrimSpace(stdout)
		h, _ := event.As[event.Header](events(t, path)[0])
		if h.Parent == "" || h.Summary == "" {
			t.Fatal("handoff lost provenance")
		}
	}
	for _, name := range sessions(t, dir) {
		if err := event.Check(events(t, filepath.Join(dir, name))); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
