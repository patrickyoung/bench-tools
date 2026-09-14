package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrickyoung/ask/internal/event"
)

func TestInitCreatesSealedRecordWithoutModelOrCurrent(t *testing.T) {
	dir, calls, _ := fake(t, 200, answerWire)
	path := filepath.Join(t.TempDir(), "record.jsonl")
	code, stdout, stderr := exec(t, "not a model message", "init", "-f", path)
	if code != 0 || stdout != path+"\n" || stderr != "" || calls.Load() != 0 {
		t.Fatalf("init: exit=%d out=%q err=%q calls=%d", code, stdout, stderr, calls.Load())
	}
	es := events(t, path)
	if len(es) != 2 || es[0].Type != event.Session || es[1].Type != event.Seal {
		t.Fatalf("initial events=%+v", es)
	}
	h, err := event.As[event.Header](es[0])
	if err != nil || h.Model != "" || h.System != "" || h.ID != "record" {
		t.Fatalf("initialized header=%+v err=%v", h, err)
	}
	if err := event.Check(es); err != nil {
		t.Fatal(err)
	}
	if _, err := event.Current(dir); err == nil {
		t.Fatal("init changed current")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("session permissions: info=%v err=%v", info, err)
	}
	if code, _, stderr := exec(t, "", "replay", "-check", path); code != 0 {
		t.Fatalf("initialized session did not replay: %s", stderr)
	}
}

func TestInitializedSessionAcceptsRecordsAndLaterModelTurns(t *testing.T) {
	_, calls, bodies := fake(t, 200, answerWire)
	path := filepath.Join(t.TempDir(), "tools.jsonl")
	if code, _, stderr := exec(t, "", "init", "-f", path); code != 0 {
		t.Fatal(stderr)
	}
	if code, _, stderr := exec(t, `{"stdout":"AP8=","exit_code":7}`, "note", "-q", "-s", "fixture", "-f", path,
		"-k", "fixture.result/v1", "-json", "-", "-seal"); code != 0 {
		t.Fatal(stderr)
	}
	if code, _, stderr := exec(t, "Observed tool failure; do not retry.", "append", "-q", "-s", "fixture", "-f", path); code != 0 {
		t.Fatal(stderr)
	}
	if calls.Load() != 0 {
		t.Fatal("recording called a model")
	}
	if code, _, stderr := exec(t, "", "-q", "-f", path, "-m", "anthropic/test-model", "Explain the recorded result."); code != 0 {
		t.Fatal(stderr)
	}
	if calls.Load() != 1 || !strings.Contains((*bodies)[0], "Observed tool failure; do not retry.") || strings.Contains((*bodies)[0], "AP8=") {
		t.Fatal("later model turn did not preserve the message/record boundary")
	}
	if err := event.Check(events(t, path)); err != nil {
		t.Fatal(err)
	}
}

func TestInitRefusesOverwriteAndInvalidUsage(t *testing.T) {
	_, calls, _ := fake(t, 200, answerWire)
	path := filepath.Join(t.TempDir(), "kept.jsonl")
	const kept = "existing content\n"
	if err := os.WriteFile(path, []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init"}, {"init", "-f", path}, {"init", "-f", path, "message"},
		{"init", "-f", path + "\nwrong"}, {"init", "-m", "anthropic/test-model", "-f", path},
	} {
		if code, out, _ := exec(t, "", args...); code != 1 || out != "" {
			t.Fatalf("invalid init accepted: %v exit=%d out=%q", args, code, out)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != kept || calls.Load() != 0 {
		t.Fatalf("init changed a file or called model: err=%v calls=%d", err, calls.Load())
	}
}
