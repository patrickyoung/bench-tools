package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func runContext(t *testing.T, dir, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	a := &app{
		in: strings.NewReader(stdin), out: &stdout, errOut: &stderr,
		getenv: func(name string) string {
			if name == "CONTEXT_PATH" {
				return dir
			}
			return ""
		},
		homeDir: os.UserHomeDir,
		command: execCommand,
	}
	code := a.run(args)
	return code, stdout.String(), stderr.String()
}

var execCommand = func(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func writeConnector(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func record(source, id, title, content string) string {
	return fmt.Sprintf(`{"kind":"context","version":1,"source":%q,"type":"document","id":%q,"title":%q,"retrieved_at":"2026-08-28T12:00:00Z","content":%s,"citation":{"locator":%q}}`,
		source, id, title, content, source+"/"+id)
}

func decodeRows(t *testing.T, output string) []map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(output))
	dec.UseNumber()
	var rows []map[string]any
	for {
		var row map[string]any
		if err := dec.Decode(&row); errors.Is(err, io.EOF) {
			return rows
		} else if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
}

func TestListDescribesConnectorsInNameOrderAndShadows(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeConnector(t, first, "zeta", `
case "$1" in
describe) printf '%s\n' '{"kind":"source","version":1,"name":"zeta","description":"Zeta search"}' ;;
*) exit 2 ;;
esac
`)
	writeConnector(t, first, "alpha", `
case "$1" in
describe) printf '%s\n' '{"kind":"source","version":1,"name":"alpha","description":"First alpha"}' ;;
*) exit 2 ;;
esac
`)
	writeConnector(t, second, "alpha", `
case "$1" in
describe) printf '%s\n' '{"kind":"source","version":1,"name":"alpha","description":"Wrong alpha"}' ;;
*) exit 2 ;;
esac
`)
	if err := os.WriteFile(filepath.Join(first, "notes"), []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runContext(t, first+string(os.PathListSeparator)+second, "", "ls")
	if code != exitYes || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	rows := decodeRows(t, stdout)
	if len(rows) != 2 || rows[0]["name"] != "alpha" || rows[1]["name"] != "zeta" {
		t.Fatalf("rows=%#v", rows)
	}
	if rows[0]["description"] != "First alpha" {
		t.Fatalf("shadowing failed: %#v", rows[0])
	}
}

func TestQueryPassesExactStdinAndAddsStableCitationRef(t *testing.T) {
	dir := t.TempDir()
	writeConnector(t, dir, "handbook", `
case "$1" in
describe) exit 2 ;;
query)
  q=$(cat)
  [ "$q" = 'paid leave policy?' ] || { echo "wrong query: $q" >&2; exit 2; }
  printf '%s\n' '{"kind":"context","version":1,"source":"handbook","type":"document","id":"leave-7","title":"Paid leave","retrieved_at":"2026-08-28T12:00:00Z","content":{"text":"Twenty days","format":"text"},"citation":{"locator":"handbook.md#leave","url":"https://example.test/leave"},"provider_rank":3}'
  ;;
*) exit 2 ;;
esac
`)

	code, stdout, stderr := runContext(t, dir, "paid leave policy?", "query", "handbook")
	if code != exitYes || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	rows := decodeRows(t, stdout)
	wantRef := citationRef("handbook", "leave-7")
	if len(rows) != 1 || rows[0]["ref"] != wantRef || rows[0]["provider_rank"] != json.Number("3") {
		t.Fatalf("row=%#v", rows)
	}
	content := rows[0]["content"].(map[string]any)
	if content["text"] != "Twenty days" {
		t.Fatalf("structured content lost: %#v", content)
	}
	retrieval := rows[0]["retrieval"].(map[string]any)
	if retrieval["query"] != "paid leave policy?" {
		t.Fatalf("retrieval query = %#v", retrieval["query"])
	}
	connector := retrieval["connector"].(map[string]any)
	rawConnector, err := os.ReadFile(filepath.Join(dir, "handbook"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(rawConnector)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if connector["name"] != "handbook" || connector["sha256"] != wantDigest {
		t.Fatalf("connector provenance = %#v, want handbook %s", connector, wantDigest)
	}

	code, stdout2, _ := runContext(t, dir, "", "query", "handbook", "paid leave policy?")
	if code != exitYes || stdout2 != stdout {
		t.Fatalf("argv query differs: exit=%d output=%q", code, stdout2)
	}
}

func TestQueryOwnsRetrievalStampAndRequiresTextQuery(t *testing.T) {
	dir := t.TempDir()
	writeConnector(t, dir, "docs", `
[ "$1" = query ] || exit 2
cat >/dev/null
printf '%s\n' '{"kind":"context","version":1,"source":"docs","type":"document","id":"1","title":"Doc","retrieved_at":"2026-08-28T12:00:00Z","content":"text","citation":{"locator":"docs/1"},"retrieval":{"query":"forged","connector":{"name":"docs","sha256":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}'
`)
	code, stdout, stderr := runContext(t, dir, "real", "query", "docs")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "stamped by context") {
		t.Fatalf("reserved retrieval: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = runContext(t, dir, string([]byte{0xff}), "query", "docs")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "valid UTF-8") {
		t.Fatalf("binary query: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestQueryNoResultIsOrdinaryAndBrokenOutputIsAtomic(t *testing.T) {
	dir := t.TempDir()
	writeConnector(t, dir, "empty", `
[ "$1" = query ] || exit 2
cat >/dev/null
exit 1
`)
	code, stdout, _ := runContext(t, dir, "anything", "query", "empty")
	if code != exitNo || stdout != "" {
		t.Fatalf("exit=%d stdout=%q", code, stdout)
	}

	writeConnector(t, dir, "broken", `
[ "$1" = query ] || exit 2
cat >/dev/null
printf '%s\n' '`+record("broken", "ok", "Good", `"first"`)+`'
printf '%s\n' '{bad json}'
`)
	code, stdout, stderr := runContext(t, dir, "anything", "query", "broken")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "record 2") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestQueryRejectsConnectorClaimingAnotherSource(t *testing.T) {
	dir := t.TempDir()
	writeConnector(t, dir, "glean", `
[ "$1" = query ] || exit 2
cat >/dev/null
printf '%s\n' '`+record("genie", "1", "Wrong", `"x"`)+`'
`)
	code, stdout, stderr := runContext(t, dir, "anything", "query", "glean")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, `source must be "glean"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestMergeIsStableDeduplicatesAndRejectsConflict(t *testing.T) {
	a := record("glean", "a", "A", `{"text":"one"}`)
	b := record("genie", "b", "B", `{"columns":["n"],"rows":[[4]]}`)
	code, stdout, stderr := runContext(t, t.TempDir(), a+"\n"+b+"\n"+a+"\n", "merge")
	if code != exitYes || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	rows := decodeRows(t, stdout)
	got := []string{rows[0]["source"].(string), rows[1]["source"].(string)}
	if !reflect.DeepEqual(got, []string{"glean", "genie"}) || len(rows) != 2 {
		t.Fatalf("rows=%#v", rows)
	}

	conflict := record("glean", "a", "A changed", `{"text":"two"}`)
	code, stdout, stderr = runContext(t, t.TempDir(), a+"\n"+conflict+"\n", "merge")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "conflicting records") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestCheckRequiresNormalizedCitationRef(t *testing.T) {
	raw := record("docs", "1", "Doc", `"text"`)
	code, stdout, stderr := runContext(t, t.TempDir(), raw+"\n", "check")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "ref is required") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	normalized, err := normalizeRecord([]byte(raw), "", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runContext(t, t.TempDir(), string(normalized)+"\n", "check")
	if code != exitYes || stdout != "" || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestMissingSourceAndEmptyStreamsUseExitOne(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		stdin string
	}{
		{[]string{"ls"}, ""},
		{[]string{"query", "missing", "q"}, ""},
		{[]string{"merge"}, ""},
		{[]string{"check"}, ""},
	} {
		code, stdout, _ := runContext(t, t.TempDir(), tc.stdin, tc.args...)
		if code != exitNo || stdout != "" {
			t.Fatalf("%v: exit=%d stdout=%q", tc.args, code, stdout)
		}
	}
}

func TestBoundedBufferReportsOverflowWithoutGrowingPastLimit(t *testing.T) {
	b := newBoundedBuffer(4)
	n, err := b.Write([]byte("abcdef"))
	if err != nil || n != 6 || !b.exceeded || string(b.bytes()) != "abcd" {
		t.Fatalf("n=%d err=%v exceeded=%v bytes=%q", n, err, b.exceeded, b.bytes())
	}
}

func TestRetrievalStampCannotExpandPastStreamLimit(t *testing.T) {
	var raw strings.Builder
	for i := range 33 {
		raw.WriteString(record("docs", fmt.Sprintf("%d", i), "Doc", `"x"`))
		raw.WriteByte('\n')
	}
	stamp := &retrievalStamp{
		Query: strings.Repeat("q", maxQueryBytes),
		Connector: connectorStamp{
			Name: "docs", SHA256: "sha256:" + strings.Repeat("a", 64),
		},
	}
	if _, err := normalizeRecords([]byte(raw.String()), "docs", true, stamp); err == nil ||
		!strings.Contains(err.Error(), "normalized output exceeds") {
		t.Fatalf("expanded normalization error = %v", err)
	}
}
