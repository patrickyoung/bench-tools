package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/patrickyoung/ask/internal/event"
)

func TestStreamedNotesAcknowledgeSealedRecordsWithoutModel(t *testing.T) {
	_, calls, _ := fake(t, 200, answerWire)
	file := filepath.Join(t.TempDir(), "stream.jsonl")
	if code, _, errout := exec(t, "", "init", "-f", file); code != 0 {
		t.Fatal(errout)
	}
	input := "{\"kind\":\"process.intent/v1\",\"body\":{\"argv\":[\"echo\"]}}\n{\"kind\":\"process.output/v1\",\"body\":{\"bytes\":\"AP8=\"}}\n"
	code, out, errout := exec(t, input, "note", "-f", file, "-s", "fixture", "-jsonl", "-", "-seal")
	if code != 0 || errout != "" || out != "{\"seq\":3}\n{\"seq\":5}\n" {
		t.Fatalf("%d %q %s", code, out, errout)
	}
	es := events(t, file)
	if err := event.Check(es); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("streamed notes called a model")
	}
	for _, i := range []int{2, 4} {
		n, err := event.As[event.NoteData](es[i])
		if err != nil || n.Source != "fixture" || !json.Valid(n.Body) || es[i+1].Type != event.Seal {
			t.Fatalf("bad record: %+v %v", n, err)
		}
	}
}

func TestStreamedNotesStopAtInvalidLineAndKeepAcknowledgedPrefix(t *testing.T) {
	file := filepath.Join(t.TempDir(), "stream.jsonl")
	if code, _, errout := exec(t, "", "init", "-f", file); code != 0 {
		t.Fatal(errout)
	}
	good := "{\"kind\":\"fixture/v1\",\"body\":null}\n"
	code, out, _ := exec(t, good+"{\"kind\":\"bad\"}\n"+good, "note", "-f", file, "-s", "fixture", "-jsonl", "-", "-seal")
	if code != 1 || out != "{\"seq\":3}\n" {
		t.Fatalf("%d %q", code, out)
	}
	es := events(t, file)
	if len(es) != 4 {
		t.Fatalf("continued after failure: %d events", len(es))
	}
	if err := event.Check(es); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(file)
	for _, input := range []string{"", "{}\n", "{\"kind\":\"fixture/v1\",\"body\":{},\"extra\":true}\n", good[:len(good)-1] + " {}\n"} {
		if code, _, _ := exec(t, input, "note", "-f", file, "-s", "fixture", "-jsonl", "-", "-seal"); code != 1 {
			t.Fatal("invalid stream accepted")
		}
	}
	after, _ := os.ReadFile(file)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid first line changed the session")
	}
}
