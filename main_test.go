package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

var fixtureTime = time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

func runTrail(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	a := &app{out: &stdout, errOut: &stderr, getenv: os.Getenv}
	code := a.run(args)
	return code, stdout.String(), stderr.String()
}

func eventLine(seq int, typ string, data any) []byte {
	b, err := json.Marshal(map[string]any{
		"seq": seq, "time": fixtureTime.Add(time.Duration(seq) * time.Second),
		"type": typ, "data": data,
	})
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}

func headerLine(id string, extra map[string]any) []byte {
	d := map[string]any{"id": id, "ask": "0.1.0", "model": "openai/test", "system": "be terse"}
	for k, v := range extra {
		d[k] = v
	}
	return eventLine(1, "session", d)
}

func writeLog(t *testing.T, dir, name string, lines ...[]byte) string {
	t.Helper()
	path := filepath.Join(dir, name+".jsonl")
	var body []byte
	for _, line := range lines {
		body = append(body, line...)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func recordsFrom(t *testing.T, text string) []map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var out []map[string]any
	for {
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				return out
			}
			t.Fatalf("stdout is not JSONL: %v\n%s", err, text)
		}
		out = append(out, row)
	}
}

func kinds(rows []map[string]any) []string {
	var out []string
	for _, row := range rows {
		out = append(out, row["kind"].(string))
	}
	return out
}

func TestListSummarizesAndDoesNotLetCorruptionHideLaterFiles(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "a", headerLine("a", nil),
		eventLine(2, "user", map[string]any{"text": "hello"}),
		eventLine(3, "request", map[string]any{"model": "test", "digest": "x"}),
		eventLine(4, "assistant", map[string]any{
			"blocks": []any{map[string]any{"type": "text", "text": "hi"}},
			"usage":  map[string]any{"in": 12, "out": 3, "reasoning": 2, "cost": 0.01},
			"model":  "test", "ms": 4,
		}),
		eventLine(5, "note", map[string]any{"source": "ply", "text": "passed"}),
		eventLine(6, "done", map[string]any{"reason": "end"}))
	writeLog(t, dir, "b", headerLine("b", nil), []byte("{bad json}\n"),
		eventLine(3, "done", map[string]any{"reason": "end"}))
	writeLog(t, dir, "c", headerLine("c", nil),
		eventLine(2, "done", map[string]any{"reason": "overflow"}))
	if err := os.Mkdir(filepath.Join(dir, "ignored.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"),
		filepath.Join(dir, "ignored-link.jsonl")); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runTrail(t, "ls", dir)
	if code != exitNo {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitNo, stderr)
	}
	rows := recordsFrom(t, stdout)
	if got := kinds(rows); !slices.Equal(got, []string{"session", "error", "session"}) {
		t.Fatalf("kinds = %v", got)
	}
	first := rows[0]
	if first["id"] != "a" || first["events"] != json.Number("6") ||
		first["users"] != json.Number("1") || first["assistants"] != json.Number("1") ||
		first["outcome"] != "end" {
		t.Errorf("first summary = %#v", first)
	}
	usage := first["usage"].(map[string]any)
	if usage["in"] != json.Number("12") || usage["cost"] != json.Number("0.01") {
		t.Errorf("usage = %#v", usage)
	}
	if !strings.Contains(rows[1]["error"].(string), "corrupt event after seq 1") {
		t.Errorf("error = %#v", rows[1])
	}
	if rows[2]["id"] != "c" {
		t.Errorf("later session was hidden: %#v", rows[2])
	}
}

func TestFindHasOneExplicitSemanticTextDefinition(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "search", headerLine("search", map[string]any{"system": "systemneedle"}),
		eventLine(2, "user", map[string]any{"source": "summary", "blocks": []any{
			map[string]any{"type": "text", "text": "Hello\n\tWORLD"},
			map[string]any{"type": "media", "name": "Invoice.PDF", "media_type": "application/pdf", "data": "YmFzZTY0bmVlZGxl"},
		}}),
		eventLine(3, "assistant", map[string]any{"blocks": []any{
			map[string]any{"type": "reasoning", "text": "Root Cause"},
			map[string]any{"type": "text", "text": "Fixed"},
			map[string]any{"type": "opaque", "provider": "x", "raw": map[string]any{"text": "opaqueneedle"}},
		}, "usage": map[string]any{}, "model": "x", "ms": 1}),
		eventLine(4, "request", map[string]any{"model": "model-label", "effort": "high",
			"system": "requestsystemneedle", "digest": "digestneedle", "schema": map[string]any{"title": "schemaneedle"}}),
		eventLine(5, "note", map[string]any{"source": "ply", "text": "Check Passed"}),
		eventLine(6, "retry", map[string]any{"attempt": 1, "status": 0, "wait_ms": 10, "error": "socket reset"}),
		eventLine(7, "done", map[string]any{"reason": "error", "error": "terminal failure"}))

	for _, tc := range []struct {
		query string
		seq   json.Number
	}{
		{"hello world", "2"}, {"INVOICE.pdf application/PDF", "2"},
		{"root cause", "3"}, {"model-label high", "4"},
		{"check passed", "5"}, {"socket reset", "6"}, {"terminal failure", "7"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			code, stdout, stderr := runTrail(t, "find", tc.query, dir)
			if code != exitYes {
				t.Fatalf("exit = %d: %s", code, stderr)
			}
			rows := recordsFrom(t, stdout)
			if len(rows) != 1 || rows[0]["seq"] != tc.seq {
				t.Fatalf("rows = %#v", rows)
			}
		})
	}
	for _, query := range []string{"systemneedle", "requestsystemneedle", "digestneedle", "schemaneedle", "opaqueneedle", "base64needle"} {
		t.Run("omits-"+query, func(t *testing.T) {
			code, stdout, _ := runTrail(t, "find", query, dir)
			if code != exitNo || stdout != "" {
				t.Fatalf("exit = %d, stdout = %q", code, stdout)
			}
		})
	}
	_, stdout, _ := runTrail(t, "find", "hello world", dir)
	text := recordsFrom(t, stdout)[0]["text"].(string)
	if !strings.Contains(text, "Hello\n\tWORLD") {
		t.Errorf("match text was normalized instead of preserved: %q", text)
	}
}

func TestFindRejectsAQueryThatFoldsToNothing(t *testing.T) {
	code, stdout, stderr := runTrail(t, "find", "\u2003\u2009")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "trail find") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestShowPreservesUnknownEvents(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`{"seq":2,"time":"2026-08-18T12:00:02Z","type":"future/event","data":{"n":12345678901234567890,"nested":[true,"x"]}}` + "\n")
	path := writeLog(t, dir, "future", headerLine("future", nil), raw)
	code, stdout, stderr := runTrail(t, "show", path)
	if code != exitYes {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	rows := recordsFrom(t, stdout)
	if len(rows) != 2 {
		t.Fatalf("rows = %#v", rows)
	}
	eventObject := rows[1]["event"].(map[string]any)
	if eventObject["type"] != "future/event" {
		t.Fatalf("event = %#v", eventObject)
	}
	data := eventObject["data"].(map[string]any)
	if data["n"] != json.Number("12345678901234567890") {
		t.Errorf("large number changed: %#v", data["n"])
	}
}

func TestWindowIsBoundedAndAcceptsFlagsOnEitherSide(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir, "window", headerLine("window", nil),
		eventLine(2, "note", map[string]any{"source": "x", "text": "two"}),
		eventLine(3, "note", map[string]any{"source": "x", "text": "three"}),
		eventLine(4, "note", map[string]any{"source": "x", "text": "four"}),
		eventLine(5, "done", map[string]any{"reason": "end"}))
	for _, args := range [][]string{
		{"window", path, "3", "-before", "1", "-after", "1"},
		{"window", "-before", "1", "-after", "1", path, "3"},
	} {
		code, stdout, stderr := runTrail(t, args...)
		if code != exitYes {
			t.Fatalf("%v: exit = %d: %s", args, code, stderr)
		}
		rows := recordsFrom(t, stdout)
		var seqs []json.Number
		for _, row := range rows {
			seqs = append(seqs, row["event"].(map[string]any)["seq"].(json.Number))
		}
		if !slices.Equal(seqs, []json.Number{"2", "3", "4"}) {
			t.Errorf("%v: seqs = %v", args, seqs)
		}
	}
	code, stdout, stderr := runTrail(t, "window", path, "3")
	if code != exitYes || stderr != "" {
		t.Fatalf("default window: exit %d, stderr %q", code, stderr)
	}
	if rows := recordsFrom(t, stdout); len(rows) != 5 {
		t.Fatalf("default window returned %d events, want 5", len(rows))
	}
	code, stdout, _ = runTrail(t, "window", path, "99")
	if code != exitNo || kinds(recordsFrom(t, stdout))[0] != "error" {
		t.Fatalf("missing seq: exit %d, stdout %s", code, stdout)
	}
}

func TestWindowReportsCorruptionBehindAMissingSequence(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir, "bad", headerLine("bad", nil),
		[]byte("bad\n"), eventLine(3, "done", map[string]any{"reason": "end"}))
	code, stdout, stderr := runTrail(t, "window", path, "3")
	if code != exitNo || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	rows := recordsFrom(t, stdout)
	if got := kinds(rows); !slices.Equal(got, []string{"error", "error"}) {
		t.Fatalf("kinds = %v; stdout = %s", got, stdout)
	}
	if !strings.Contains(rows[1]["error"].(string), "corrupt event") {
		t.Errorf("damage was hidden: %#v", rows)
	}
}

func TestTornTailIsVisibleAndNeverRepaired(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir, "torn", headerLine("torn", nil),
		eventLine(2, "done", map[string]any{"reason": "end"}), []byte(`{"seq":3`))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	infoBefore, _ := os.Stat(path)
	code, stdout, stderr := runTrail(t, "ls", dir)
	if code != exitYes {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	rows := recordsFrom(t, stdout)
	if got := kinds(rows); !slices.Equal(got, []string{"session", "warning"}) {
		t.Fatalf("kinds = %v", got)
	}
	if rows[0]["torn_tail"] != true || !strings.Contains(rows[1]["warning"].(string), "torn") {
		t.Errorf("rows = %#v", rows)
	}
	after, _ := os.ReadFile(path)
	infoAfter, _ := os.Stat(path)
	if !bytes.Equal(before, after) || infoBefore.Mode() != infoAfter.Mode() || !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Error("trail repaired or touched the torn session")
	}
}

func TestLineageReportsOnlyRecordedEdges(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "parent", headerLine("p", nil))
	writeLog(t, dir, "parent-copy", headerLine("p", nil))
	writeLog(t, dir, "summary", headerLine("s", nil))
	child := writeLog(t, dir, "child", headerLine("c", map[string]any{"parent": "p", "summary": "s"}))
	writeLog(t, dir, "grandchild", headerLine("g", map[string]any{"parent": "c"}))
	writeLog(t, dir, "unrelated", headerLine("u", nil))
	writeLog(t, dir, "zzz-bad", headerLine("bad", nil), []byte("bad\n"), headerLine("after", nil))

	code, stdout, stderr := runTrail(t, "lineage", child, dir)
	if code != exitNo {
		t.Fatalf("exit = %d, want damage signal; stderr: %s", code, stderr)
	}
	rows := recordsFrom(t, stdout)
	nodes := make(map[string]map[string]any)
	var edges []string
	for _, row := range rows {
		switch row["kind"] {
		case "node":
			nodes[row["id"].(string)] = row
		case "edge":
			edges = append(edges, fmt.Sprintf("%s-%s->%s", row["from"], row["relation"], row["to"]))
		}
	}
	if got := sortedKeys(nodes); !slices.Equal(got, []string{"c", "g", "p", "s"}) {
		t.Fatalf("nodes = %v", got)
	}
	if nodes["p"]["ambiguous"] != true || len(nodes["p"]["paths"].([]any)) != 2 {
		t.Errorf("copied identity not shown as ambiguous: %#v", nodes["p"])
	}
	if nodes["c"]["target"] != true {
		t.Errorf("target = %#v", nodes["c"])
	}
	slices.Sort(edges)
	want := []string{"c-parent->p", "c-summary->s", "g-parent->c"}
	if !slices.Equal(edges, want) {
		t.Errorf("edges = %v, want %v", edges, want)
	}
	if strings.Contains(stdout, `"id":"u"`) || strings.Contains(stdout, "copy-parent") {
		t.Error("lineage inferred an unrecorded relationship")
	}
}

func TestLineageKeepsAReadableTargetWhenItsTailIsCorrupt(t *testing.T) {
	dir := t.TempDir()
	target := writeLog(t, dir, "child", headerLine("c", map[string]any{"parent": "p"}),
		[]byte("bad\n"), eventLine(3, "done", map[string]any{"reason": "end"}))
	writeLog(t, dir, "parent", headerLine("p", nil))

	code, stdout, stderr := runTrail(t, "lineage", target, dir)
	if code != exitNo || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	rows := recordsFrom(t, stdout)
	if got := kinds(rows); !slices.Equal(got, []string{"error", "node", "node", "edge"}) {
		t.Fatalf("kinds = %v; stdout = %s", got, stdout)
	}
	if rows[1]["id"] != "c" || rows[1]["target"] != true {
		t.Errorf("target = %#v", rows[1])
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func TestCheckDelegatesEveryVerdictToAsk(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("trail and ask target Unix")
	}
	dir := t.TempDir()
	writeLog(t, dir, "bad", headerLine("bad", nil))
	writeLog(t, dir, "good", headerLine("good", nil))
	calls := filepath.Join(t.TempDir(), "calls")
	script := filepath.Join(t.TempDir(), "ask-stub")
	body := "#!/bin/sh\n" +
		"printf '%s|%s|%s\\n' \"$1\" \"$2\" \"$3\" >> \"$CALLS\"\n" +
		"case \"$3\" in *bad.jsonl) echo 'ask: replay divergence' >&2; exit 1;; esac\n" +
		"echo \"ok: $(basename \"$3\") replays exactly\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASK", script)
	t.Setenv("CALLS", calls)
	code, stdout, stderr := runTrail(t, "check", dir)
	if code != exitNo {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	rows := recordsFrom(t, stdout)
	if len(rows) != 2 || rows[0]["ok"] != false || rows[1]["ok"] != true {
		t.Fatalf("checks = %#v", rows)
	}
	got, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("replay|-check|%s\nreplay|-check|%s\n",
		filepath.Join(dir, "bad.jsonl"), filepath.Join(dir, "good.jsonl"))
	if string(got) != want {
		t.Errorf("ask calls = %q, want %q", got, want)
	}
}

func TestCheckOfAnEmptyArchiveDoesNotNeedAsk(t *testing.T) {
	t.Setenv("ASK", filepath.Join(t.TempDir(), "missing-ask"))
	code, stdout, stderr := runTrail(t, "check", t.TempDir())
	if code != exitYes || stdout != "" || stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestAllReadersLeaveTheArchiveByteForByteUntouched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("trail and ask target Unix")
	}
	dir := t.TempDir()
	path := writeLog(t, dir, "one", headerLine("one", nil),
		eventLine(2, "note", map[string]any{"source": "x", "text": "needle"}),
		eventLine(3, "done", map[string]any{"reason": "end"}))
	stub := filepath.Join(t.TempDir(), "ask")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho ok\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASK", stub)
	t.Setenv("ASK_DIR", dir)
	before := fileStateOf(t, path)
	commands := [][]string{
		{"ls", dir}, {"find", "needle", dir}, {"show", path},
		{"window", path, "2"}, {"lineage", path, dir}, {"check", dir},
	}
	for _, args := range commands {
		code, _, stderr := runTrail(t, args...)
		if code != exitYes {
			t.Fatalf("%v: exit %d: %s", args, code, stderr)
		}
	}
	after := fileStateOf(t, path)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("archive changed:\nbefore: %#v\nafter:  %#v", before, after)
	}
}

type fileState struct {
	Body    string
	Mode    os.FileMode
	ModTime time.Time
	Size    int64
}

func fileStateOf(t *testing.T, path string) fileState {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileState{Body: string(body), Mode: info.Mode(), ModTime: info.ModTime(), Size: info.Size()}
}

func TestEventLineLimitRefusesInsteadOfTruncating(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir, "large", headerLine("large", nil),
		eventLine(2, "note", map[string]any{"source": "x", "text": strings.Repeat("z", 300)}))
	s, err := readSessionLimit(path, 256)
	if err == nil || !strings.Contains(err.Error(), "exceeds 256 bytes") {
		t.Fatalf("error = %v", err)
	}
	if len(s.Events) != 1 || s.Events[0].Type != "session" {
		t.Errorf("valid prefix = %#v", s.Events)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("pipe closed") }

func TestOutputFailureIsAnOperationalError(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "one", headerLine("one", nil))
	var stderr bytes.Buffer
	a := &app{out: failWriter{}, errOut: &stderr, getenv: os.Getenv}
	if code := a.run([]string{"ls", dir}); code != exitErr {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "writing stdout") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestUsageVersionAndDefaults(t *testing.T) {
	code, stdout, stderr := runTrail(t)
	if code != exitErr || stdout != "" || !strings.HasPrefix(stderr, "trail -") {
		t.Errorf("no args: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runTrail(t, "help")
	if code != exitYes || stderr != "" || stdout != usageText {
		t.Errorf("help: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, _ = runTrail(t, "version")
	if code != exitYes || stdout != "trail "+version+"\n" {
		t.Errorf("version: exit %d, stdout %q", code, stdout)
	}

	dir := filepath.Join(t.TempDir(), "absent")
	t.Setenv("ASK_DIR", dir)
	code, stdout, stderr = runTrail(t, "ls")
	if code != exitYes || stdout != "" || stderr != "" {
		t.Errorf("absent default archive: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runTrail(t, "ls", dir)
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "reading archive") {
		t.Errorf("absent explicit archive: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
