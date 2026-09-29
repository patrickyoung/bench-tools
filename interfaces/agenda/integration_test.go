package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func publicController(t *testing.T) controller {
	t.Helper()
	binary := os.Getenv("AGENDA_TEST_BIN")
	if binary == "" {
		t.Skip("set AGENDA_TEST_BIN for public executable integration")
	}
	dir := t.TempDir()
	c := controller{Agenda: binary, Root: filepath.Join(dir, "work"), FilesRoot: filepath.Join(dir, "inputs"), EvidenceRoot: filepath.Join(dir, "proof"), AllowWrite: true}
	for _, path := range []string{c.Root, c.FilesRoot, c.EvidenceRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	return c
}

func itemChange() map[string]any {
	return map[string]any{"schema": "agenda.change/v1", "request_id": "item-create", "kind": "item", "id": "human/review", "previous": nil, "by": "Operator", "reason": "Public interface integration", "value": map[string]any{"title": "Review published evidence", "owner": "Morgan", "not_before": "2026-01-01T09:00:00Z", "due_at": "2035-01-01T17:00:00Z", "timezone": "UTC", "basis": "human"}}
}

func dispatchChange(t *testing.T, c controller, change any) []byte {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"name": "agenda_apply", "arguments": map[string]any{"change": change}})
	var out bytes.Buffer
	if err := dispatch(context.Background(), c, raw, &out); err != nil {
		t.Fatalf("apply: %v %s", err, out.String())
	}
	return out.Bytes()
}

func TestPublicAgendaHumanEvidenceAndBothViews(t *testing.T) {
	c := publicController(t)
	dispatchChange(t, c, itemChange())
	proof := []byte("Reviewed selected output and recorded findings.\n")
	sum := sha256.Sum256(proof)
	digest := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(c.FilesRoot, "review.txt"), proof, 0600); err != nil {
		t.Fatal(err)
	}
	report := map[string]any{"schema": "agenda.change/v1", "request_id": "review-complete", "kind": "report", "id": "human/review", "previous": nil, "by": "Morgan", "reason": "Review complete with selected proof", "value": map[string]any{"target": "human/review", "state": "done"}, "evidence": []any{map[string]any{"path": "review.txt", "sha256": digest}}}
	first := dispatchChange(t, c, report)
	again := dispatchChange(t, c, report)
	if !bytes.Equal(first, again) {
		t.Fatal("same proof-bearing request did not preserve retry identity")
	}
	raw, err := c.project(context.Background(), time.Now().UTC().Add(time.Second).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	p, err := readProjection(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Obligations) != 1 || p.Obligations[0].Acceptance != "reported-done" {
		t.Fatalf("human report lost: %s", raw)
	}
	for _, view := range []string{"calendar", "kanban"} {
		var html bytes.Buffer
		if err := render(&html, p, view, false, 5); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(html.String(), "Human-reported completion") {
			t.Fatalf("%s mislabels human completion", view)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(c.EvidenceRoot, digest)); err != nil || !bytes.Equal(raw, proof) {
		t.Fatal("proof stage lost")
	}
}

func TestPublicMCPExecutableRoundTrip(t *testing.T) {
	c := publicController(t)
	mcp, server := os.Getenv("MCP_TEST_BIN"), os.Getenv("MCPSERVE_TEST_BIN")
	if mcp == "" || server == "" {
		t.Skip("set MCP_TEST_BIN and MCPSERVE_TEST_BIN for protocol integration")
	}
	dispatchChange(t, c, itemChange())
	dir := t.TempDir()
	app := filepath.Join(dir, "agenda-ui")
	build := exec.Command("go", "build", "-o", app, ".")
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("standalone build: %v %s", err, raw)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	raw, _ := json.Marshal(manifest(false))
	if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	writeEnabled := false
	call := func(method, request, root, binary string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, mcp, "request", method, "--", server, manifestPath, "--", app, "mcp-dispatch", "--agenda", binary, "--root", root)
		if writeEnabled {
			cmd.Args = append(cmd.Args, "--allow-write")
		}
		cmd.Stdin = strings.NewReader(request)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil && stdout.Len() == 0 {
			t.Fatalf("MCP returned no structured failure: %v %s", err, stderr.String())
		}
		return stdout.Bytes(), err
	}
	listed, err := call("tools/list", `{}`, c.Root, c.Agenda)
	if err != nil || !bytes.Contains(listed, []byte("agenda_project")) || !bytes.Contains(listed, []byte("start_date")) || bytes.Contains(listed, []byte("agenda_apply")) {
		t.Fatalf("discovery mismatch: %v %s", err, listed)
	}
	request := `{"name":"agenda_project","arguments":{"as_of":"2035-01-01T00:00:00Z"}}`
	projected, err := call("tools/call", request, c.Root, c.Agenda)
	if err != nil || !bytes.Contains(projected, []byte("agenda.projection/v1")) || !bytes.Contains(projected, []byte("human/review")) {
		t.Fatalf("missing shared projection: %v %s", err, projected)
	}
	empty := filepath.Join(dir, "empty-root")
	if err = os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	failed := filepath.Join(dir, "unknown-agenda")
	if err = os.WriteFile(failed, []byte("#!/bin/sh\nprintf 'uncertain command outcome' >&2\nexit 125\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, request, root, binary string
		exit                        int
	}{
		{"invalid input", `{"name":"agenda_project","arguments":{"as_of":"not-a-time"}}`, c.Root, c.Agenda, 2},
		{"Agenda failure", request, empty, c.Agenda, 2},
		{"unknown subprocess", request, c.Root, failed, 125},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := call("tools/call", test.request, test.root, test.binary)
			if err == nil {
				t.Fatalf("failed tool appeared successful: %s", raw)
			}
			var result struct {
				IsError    bool `json:"isError"`
				Structured struct {
					Exit int `json:"agenda_exit"`
				} `json:"structuredContent"`
			}
			if err = json.Unmarshal(raw, &result); err != nil || !result.IsError || result.Structured.Exit != test.exit || bytes.Contains(raw, []byte("agenda.projection/v1")) {
				t.Fatalf("lost tool failure: %v %s", err, raw)
			}
		})
	}
	writeEnabled = true
	raw, _ = json.Marshal(manifest(true))
	if err = os.WriteFile(manifestPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	listed, err = call("tools/list", `{}`, c.Root, c.Agenda)
	for _, field := range []string{"agenda_apply", "due_at", "start_date", "needs-attention"} {
		if err != nil || !bytes.Contains(listed, []byte(field)) {
			t.Fatalf("write discovery lacks %s: %v %s", field, err, listed)
		}
	}

}
