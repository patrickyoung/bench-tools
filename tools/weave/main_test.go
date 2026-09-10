package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func row(id string, needs ...string) string {
	if needs == nil {
		needs = []string{}
	}
	b, _ := json.Marshal(map[string]any{"id": id, "needs": needs, "input": map[string]any{"operation": "review"}})
	return string(b)
}
func obs(task, state string) string {
	var t struct{ ID string }
	_ = fields([]byte(task), map[string]any{"id": &t.ID})
	b, _ := json.Marshal(map[string]any{"id": t.ID, "task_sha256": fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(task))), "state": state, "evidence": []map[string]string{{"kind": "check", "ref": "run/check.jsonl#1"}}})
	return string(b)
}
func call(t *testing.T, tasks, observations string) (int, string, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "tasks.jsonl")
	if err := os.WriteFile(p, []byte(tasks), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := run([]string{p}, strings.NewReader(observations), &out, &stderr)
	return code, out.String(), stderr.String()
}

func TestForkJoinAndExactBytes(t *testing.T) {
	a := ` {"id":"a","needs":[],"input":null,"extension":9007199254740993} `
	b, c, d := row("b", "a"), row("c", "a"), row("d", "b", "c")
	graph := strings.Join([]string{d, c, a, b}, "\r\n") + "\r\n"
	for _, tc := range []struct{ states, want string }{
		{"", a + "\n"}, {obs(a, "accepted"), c + "\n" + b + "\n"},
		{obs(a, "accepted") + "\n" + obs(b, "running"), c + "\n"},
		{obs(a, "accepted") + "\n" + obs(b, "accepted") + "\n" + obs(c, "accepted"), d + "\n"},
		{obs(a, "rejected"), ""}, {obs(a, "unknown"), ""},
	} {
		code, out, err := call(t, graph, tc.states)
		if code != 0 || out != tc.want || err != "" {
			t.Fatalf("code=%d out=%q err=%s want=%q", code, out, err, tc.want)
		}
	}
}

func TestRejectsInvalidSnapshotsBeforeOutput(t *testing.T) {
	a, b := row("a"), row("b", "a")
	for name, tc := range map[string][2]string{
		"late malformed":           {a + "\n{bad", ""},
		"duplicate task":           {a + "\n" + a, ""},
		"missing dependency":       {a + "\n" + row("b", "missing"), ""},
		"cycle":                    {a + "\n" + row("b", "c") + "\n" + row("c", "b"), ""},
		"duplicate dependency":     {a + "\n" + row("b", "a", "a"), ""},
		"self":                     {row("a", "a"), ""},
		"absent input":             {`{"id":"a","needs":[]}`, ""},
		"null needs":               {`{"id":"a","needs":null,"input":{}}`, ""},
		"blank line":               {a + "\n\n", ""},
		"array":                    {`[]`, ""},
		"null":                     {`null`, ""},
		"invalid utf8":             {a + "\n" + string([]byte{0xff}), ""},
		"duplicate JSON field":     {`{"id":"a","id":"b","needs":[],"input":{}}`, ""},
		"escaped duplicate field":  {`{"id":"a","\u0069d":"b","needs":[],"input":{}}`, ""},
		"nested duplicate":         {`{"id":"a","needs":[],"input":{"x":1,"x":2}}`, ""},
		"two values":               {a + " {}", ""},
		"unknown observed task":    {a, obs(row("other"), "accepted")},
		"stale task":               {a + " ", obs(a, "accepted")},
		"duplicate observed":       {a, obs(a, "running") + "\n" + obs(a, "accepted")},
		"invalid state":            {a, obs(a, "done")},
		"bad trailing observation": {a, obs(a, "accepted") + "\n{"},
		"unaccepted prerequisite":  {a + "\n" + b, obs(b, "accepted")},
		"missing evidence":         {a, strings.ReplaceAll(obs(a, "accepted"), `[{"kind":"check","ref":"run/check.jsonl#1"}]`, `[]`)},
		"bad evidence":             {a, strings.ReplaceAll(obs(a, "accepted"), `"ref":"run/check.jsonl#1"`, `"ref":""`)},
	} {
		t.Run(name, func(t *testing.T) {
			code, out, err := call(t, tc[0], tc[1])
			if code != 1 || out != "" || err == "" {
				t.Fatalf("code=%d out=%q err=%q", code, out, err)
			}
		})
	}
}

func TestCaseSensitiveFields(t *testing.T) {
	a := `{"id":"a","ID":"not-a","needs":[],"input":{}}`
	code, out, err := call(t, a, strings.TrimSuffix(obs(a, "running"), "}")+`,"State":"accepted"}`)
	if code != 0 || out != "" || err != "" {
		t.Fatalf("%d %q %s", code, out, err)
	}
	code, _, _ = call(t, `{"ID":"a","needs":[],"input":{}}`, "")
	if code != 1 {
		t.Fatal("case alias admitted")
	}
}

func TestEmptyAndCompleteAreValidEmptyProjections(t *testing.T) {
	for _, pair := range [][2]string{{"", ""}, {row("a"), obs(row("a"), "accepted")}} {
		code, out, err := call(t, pair[0], pair[1])
		if code != 0 || out != "" || err != "" {
			t.Fatalf("%d %q %s", code, out, err)
		}
	}
}

func TestLimitsAndLongGraph(t *testing.T) {
	for _, s := range []string{
		strings.Repeat(" ", maxLine+1),
		strings.Repeat(" ", maxBytes+1),
		strings.Repeat(row("a")+"\n", maxTasks+1),
		`{"id":"a","needs":[],"input":` + strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65) + "}",
	} {
		code, out, _ := call(t, s, "")
		if code != 1 || out != "" {
			t.Fatalf("limit: %d", code)
		}
	}
	var graph strings.Builder
	for i := 0; i < maxTasks; i++ {
		var deps []string
		if i > 0 {
			deps = []string{fmt.Sprint(i - 1)}
		}
		fmt.Fprintln(&graph, row(fmt.Sprint(i), deps...))
	}
	code, out, err := call(t, graph.String(), "")
	if code != 0 || out != row("0")+"\n" {
		t.Fatalf("long graph: %d %s", code, err)
	}
}

type brokenIO struct{}

func (brokenIO) Read([]byte) (int, error)  { return 0, errors.New("read failed") }
func (brokenIO) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func TestOperatingFailures(t *testing.T) {
	var errout bytes.Buffer
	p := filepath.Join(t.TempDir(), "tasks")
	_ = os.WriteFile(p, []byte(row("a")), 0600)
	if got := run([]string{p}, brokenIO{}, io.Discard, &errout); got != 2 {
		t.Fatal(got)
	}
	if got := run([]string{p}, strings.NewReader(""), brokenIO{}, &errout); got != 2 {
		t.Fatal(got)
	}
	if got := run([]string{p + "missing"}, strings.NewReader(""), io.Discard, &errout); got != 2 {
		t.Fatal(got)
	}
	for _, args := range [][]string{nil, {"-"}, {"--oops"}, {p, p}} {
		var out bytes.Buffer
		if got := run(args, strings.NewReader(""), &out, &errout); got != 2 || out.Len() != 0 {
			t.Fatal(args, got)
		}
	}
}

func TestThreeHundredTasksKeepAssociation(t *testing.T) {
	var tasks, observations strings.Builder
	for i := 0; i < 300; i++ {
		r := row(fmt.Sprintf("t-%03d", i))
		fmt.Fprintln(&tasks, r)
		if i%3 == 0 {
			fmt.Fprintln(&observations, obs(r, "accepted"))
		}
	}
	code, out, err := call(t, tasks.String(), observations.String())
	if code != 0 {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 200 {
		t.Fatal(len(lines))
	}
	for _, line := range lines {
		var v task
		_ = json.Unmarshal([]byte(line), &v)
		var i int
		_, _ = fmt.Sscanf(v.ID, "t-%03d", &i)
		if i%3 == 0 {
			t.Fatal(v.ID)
		}
	}
}

func TestDoubleDashTreatsTaskFilenameLiterally(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, name := range []string{"-tasks.jsonl", "-", "help"} {
		if err := os.WriteFile(name, []byte(row("a")), 0600); err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		if code := run([]string{"--", name}, strings.NewReader(""), &out, &stderr); code != 0 || out.String() != row("a")+"\n" {
			t.Fatalf("name=%q code=%d out=%q stderr=%q", name, code, out.String(), stderr.String())
		}
	}
}

func TestUnicodeEscapesPreserveUnambiguousIdentity(t *testing.T) {
	for name, text := range map[string]string{
		"lone high surrogate":       `"\ud800"`,
		"lone low surrogate":        `"\udfff"`,
		"high followed by ordinary": `"\ud800\u0061"`,
		"high followed by high":     `"\ud800\ud800"`,
		"high followed by text":     `"\ud800x"`,
		"reversed pair":             `"\udc00\ud800"`,
	} {
		t.Run(name, func(t *testing.T) {
			for _, task := range []string{
				`{"id":` + text + `,"needs":[],"input":null}`,
				`{"id":"a","needs":[],"input":{"extension":` + text + `}}`,
			} {
				if code, out, diagnostic := call(t, task, ""); code != 1 || out != "" || diagnostic == "" {
					t.Fatalf("code=%d out=%q diagnostic=%q", code, out, diagnostic)
				}
			}
		})
	}
	for _, id := range []string{`"\ud83d\ude00"`, `"\uD800\uDC00"`, `"\\ud800"`, `"�"`} {
		task := `{"id":` + id + `,"needs":[],"input":"escaped quote: \" and slash: \\"}`
		if code, out, diagnostic := call(t, task, ""); code != 0 || out != task+"\n" {
			t.Fatalf("valid unicode: code=%d out=%q diagnostic=%q", code, out, diagnostic)
		}
		if code, out, diagnostic := call(t, task, obs(task, "accepted")); code != 0 || out != "" {
			t.Fatalf("decoded identity: code=%d out=%q diagnostic=%q", code, out, diagnostic)
		}
	}
	for _, count := range []int{64, 65} {
		task := `{"id":"` + strings.Repeat(`\ud83d\ude00`, count) + `","needs":[],"input":null}`
		code, _, diagnostic := call(t, task, "")
		if (count == 64 && code != 0) || (count == 65 && code != 1) {
			t.Fatalf("decoded ID bytes=%d: code=%d diagnostic=%q", count*4, code, diagnostic)
		}
	}
	a := row("a")
	bad := strings.Replace(obs(a, "accepted"), `"run/check.jsonl#1"`, `"\ud800"`, 1)
	if code, out, _ := call(t, a, bad); code != 1 || out != "" {
		t.Fatal("unpaired surrogate admitted in evidence")
	}
}

func TestNestingCountsEmptyAndNonemptyContainersEqually(t *testing.T) {
	for _, container := range [][2]string{{"[", "]"}, {`{"x":`, "}"}} {
		for _, leaf := range []string{"0", "[]", "{}"} {
			for _, depth := range []int{64, 65} {
				// The task object is level one; an empty leaf adds one level.
				wraps := depth - 1
				if leaf != "0" {
					wraps--
				}
				input := strings.Repeat(container[0], wraps) + leaf + strings.Repeat(container[1], wraps)
				task := `{"id":"a","needs":[],"input":` + input + `}`
				code, out, diagnostic := call(t, task, "")
				if depth == 64 && (code != 0 || out != task+"\n") {
					t.Fatalf("depth=%d leaf=%s: code=%d diagnostic=%q", depth, leaf, code, diagnostic)
				}
				if depth == 65 && (code != 1 || out != "") {
					t.Fatalf("depth=%d leaf=%s: code=%d out=%q", depth, leaf, code, out)
				}
			}
		}
	}
}

func TestTrailingCarriageReturnKeepsDigestAcrossProjection(t *testing.T) {
	task := row("a") + "\r"
	for _, terminator := range []string{"", "\r\n"} {
		code, ready, diagnostic := call(t, task+terminator, "")
		if code != 0 || ready != task+"\r\n" {
			t.Fatalf("code=%d ready=%q diagnostic=%q", code, ready, diagnostic)
		}
		code, out, diagnostic := call(t, ready, obs(task, "accepted"))
		if code != 0 || out != "" {
			t.Fatalf("digest changed on projection: code=%d diagnostic=%q", code, diagnostic)
		}
	}
}

func TestLineLimitExcludesOnlyTheTerminator(t *testing.T) {
	a := row("a")
	task := a + strings.Repeat(" ", maxLine-len(a))
	for _, end := range []string{"", "\n", "\r\n"} {
		if code, out, diagnostic := call(t, task+end, ""); code != 0 || out != task+"\n" {
			t.Fatalf("line at limit: code=%d diagnostic=%q", code, diagnostic)
		}
		if code, out, _ := call(t, task+" "+end, ""); code != 1 || out != "" {
			t.Fatalf("line over limit: code=%d", code)
		}
	}
}
