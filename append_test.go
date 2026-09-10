package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrickyoung/ask/internal/event"
)

func TestAppendPersistsObservationWithoutCallingModel(t *testing.T) {
	dir, calls, bodies := fake(t, 200, answerWire)
	if code, _, stderr := exec(t, "", "-q", "original goal"); code != 0 {
		t.Fatal(stderr)
	}
	path, _ := event.Current(dir)
	before, _ := os.ReadFile(path)
	const observation = "observed: file changed; publication outcome uncertain\n"
	code, stdout, stderr := exec(t, observation, "append", "-q", "-s", "ply", "-f", path)
	if code != 0 || stdout != "" || stderr != "" || calls.Load() != 1 {
		t.Fatalf("append: exit=%d out=%q err=%q calls=%d", code, stdout, stderr, calls.Load())
	}
	after, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatal("append modified the existing session prefix")
	}
	es := events(t, path)
	user, _ := event.As[event.UserData](es[len(es)-2])
	if es[len(es)-2].Type != event.User || user.Text != observation || user.Source != "ply" || !user.Sealed || es[len(es)-1].Type != event.Seal {
		t.Fatalf("observation not recorded durably: %+v", user)
	}
	if err := event.Check(es); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := exec(t, "", "-q", "-f", path, "Continue from the observation."); code != 0 {
		t.Fatal(stderr)
	}
	var req struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte((*bodies)[1]), &req); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, m := range req.Messages {
		if strings.Contains(string(m.Content), strings.TrimSpace(observation)) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("provider saw observation %d times: %s", count, (*bodies)[1])
	}
	if err := event.Check(events(t, path)); err != nil {
		t.Fatal(err)
	}
	current, _ := event.Current(dir)
	if current != path {
		t.Fatal("append moved current")
	}
}

func TestAppendRejectsInvalidInputBeforeMutation(t *testing.T) {
	dir, calls, _ := fake(t, 200, answerWire)
	exec(t, "", "-q", "goal")
	path, _ := event.Current(dir)
	before, _ := os.ReadFile(path)
	for _, tc := range []struct {
		input string
		args  []string
	}{
		{"observation", nil},
		{"observation", []string{"-s", "two words"}},
		{"observation", []string{"-s", "two\rwords"}},
		{"", []string{"-s", "ply"}},
		{string([]byte{0xff}), []string{"-s", "ply"}},
		{strings.Repeat("x", maxAttachment+1), []string{"-s", "ply"}},
	} {
		args := append([]string{"append", "-f", path}, tc.args...)
		if code, out, _ := exec(t, tc.input, args...); code != 1 || out != "" {
			t.Fatalf("invalid append succeeded: %v", tc.args)
		}
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) || calls.Load() != 1 {
		t.Fatal("invalid append changed session or called provider")
	}
	missing := filepath.Join(t.TempDir(), "missing.jsonl")
	if code, _, _ := exec(t, "observation", "append", "-s", "ply", "-f", missing); code != 1 {
		t.Fatal("append created missing session")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("failed append left a file")
	}
}

func TestAppendHonorsLockAndRejectsUnsealedObservation(t *testing.T) {
	dir, _, _ := fake(t, 200, answerWire)
	exec(t, "", "-q", "goal")
	path, _ := event.Current(dir)
	log, _, err := event.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := exec(t, "observation", "append", "-s", "ply", "-f", path); code != 1 || !strings.Contains(stderr, "in use") {
		t.Fatalf("append bypassed writer lock: exit %d: %s", code, stderr)
	}
	log.Append(event.User, event.UserData{Text: "unsealed result", Source: "ply", Sealed: true})
	log.Close()
	before, _ := os.ReadFile(path)
	if code, _, stderr := exec(t, "observation", "append", "-s", "ply", "-f", path); code != 1 || !strings.Contains(stderr, "not immediately sealed") {
		t.Fatalf("append accepted unsealed prefix: exit %d: %s", code, stderr)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid prefix was changed")
	}
	if code, _, _ := exec(t, "", "replay", "-check", path); code != 1 {
		t.Fatal("replay accepted unsealed observation")
	}
}

func TestAppendCarriesExactEvidenceManifest(t *testing.T) {
	dir, _, _ := fake(t, 200, answerWire)
	exec(t, "", "-q", "goal")
	path, _ := event.Current(dir)
	text := " \n" + `{"kind":"context","version":1,"source":"docs","type":"document","id":"1","title":"Doc","ref":"ctx:docs:x","retrieved_at":"2026-08-28T12:00:00Z","content":"evidence","citation":{"locator":"x"}}` + "\n"
	if code, _, stderr := exec(t, text, "append", "-q", "-s", "ply", "-f", path); code != 0 {
		t.Fatal(stderr)
	}
	es := events(t, path)
	u, _ := event.As[event.UserData](es[len(es)-2])
	if u.Text != text || u.Evidence == nil || u.Evidence.Offset != 2 {
		t.Fatalf("exact message/manifest not preserved: %+v", u)
	}
	if err := event.Check(es); err != nil {
		t.Fatal(err)
	}
}
