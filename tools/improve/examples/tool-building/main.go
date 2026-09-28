// tool-building is a caller-selected Improve adapter, not an Improve runtime.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const moduleText = "module example.invalid/bench-tool\n\ngo 1.20\n"
const dormantMain = "package main\n\nimport (\"fmt\"; \"os\")\nfunc main() { fmt.Fprintln(os.Stderr, \"dormant tool slot: not implemented\"); os.Exit(2) }\n"
const dormantTest = "package main\n\nimport \"testing\"\nfunc TestSlotExists(t *testing.T) {}\n"
const fixtureMain = `package main
import ("encoding/json"; "fmt"; "os")
// Summarize status counts from a JSON array of record objects.
func main() {
 var rows []struct { Status string ` + "`json:\"status\"`" + ` }
 if err := json.NewDecoder(os.Stdin).Decode(&rows); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(2) }
 counts := map[string]int{}
 for _, row := range rows { counts[row.Status]++ }
 if err := json.NewEncoder(os.Stdout).Encode(counts); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(2) }
}
`

type settings struct {
	Slot        string            `json:"slot"`
	Integration string            `json:"integration"`
	Tools       map[string]string `json:"tools"`
	Model       string            `json:"proposer_model"`
	Effort      string            `json:"effort"`
	Fixture     bool              `json:"fixture"`
	Research    json.RawMessage   `json:"research,omitempty"`
}
type caseRef struct {
	ID     string `json:"id"`
	Family string `json:"family"`
	File   string `json:"file"`
}
type request struct {
	Version      int               `json:"version"`
	Source       string            `json:"source"`
	SourceHash   string            `json:"source_sha256"`
	Files        map[string]string `json:"files"`
	Development  []caseRef         `json:"development"`
	Observations json.RawMessage   `json:"observations"`
	Settings     settings          `json:"settings"`
	Work         string            `json:"work"`
	Evidence     string            `json:"evidence"`
	Case         caseRef           `json:"case"`
	Repeat       int               `json:"repeat"`
	Split        string            `json:"split"`
	Baseline     []sample          `json:"baseline"`
	Candidate    []sample          `json:"candidate"`
}
type sample struct {
	Case        string `json:"case"`
	Repeat      int    `json:"repeat"`
	Observation struct {
		Score struct {
			Accepted  *bool `json:"accepted"`
			BuildExit *int  `json:"build_exit"`
			TestExit  *int  `json:"test_exit"`
		} `json:"score"`
	} `json:"observation"`
}
type testCase struct {
	Stdin  string `json:"stdin"`
	Stdout string `json:"expected_stdout"`
	Exit   int    `json:"expected_exit"`
}
type fileInfo struct {
	Hash string      `json:"sha256"`
	Mode fs.FileMode `json:"mode"`
}
type process struct {
	Argv    []string `json:"argv"`
	Exit    int      `json:"exit"`
	Seconds float64  `json:"seconds"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
}
type bounded struct {
	buffer   bytes.Buffer
	overflow bool
}

func (b *bounded) Write(p []byte) (int, error) {
	n := len(p)
	if b.buffer.Len()+n > 1<<20 {
		b.overflow = true
		p = p[:max(0, (1<<20)-b.buffer.Len())]
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}
func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func emit(v any) error     { return json.NewEncoder(os.Stdout).Encode(v) }
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if len(b) > 2<<20 {
		return errors.New("JSON exceeds 2 MiB")
	}
	return decode(b, v)
}
func decode(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func rel(p string) error {
	if p == "" || filepath.IsAbs(p) || filepath.Clean(p) != p || p == "." || p == ".." || strings.HasPrefix(p, ".."+string(filepath.Separator)) || strings.ContainsAny(p, "\x00\r\n\\") {
		return fmt.Errorf("unsafe relative path %q", p)
	}
	return nil
}
func inventory(root string) (map[string]fileInfo, error) {
	out := map[string]fileInfo{}
	var total int64
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == root {
			return nil
		}
		r, _ := filepath.Rel(root, p)
		if e = rel(r); e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink: %s", r)
		}
		if d.IsDir() {
			return nil
		}
		i, e := d.Info()
		if e != nil {
			return e
		}
		if !i.Mode().IsRegular() {
			return fmt.Errorf("not regular: %s", r)
		}
		total += i.Size()
		if len(out) >= 4096 || i.Size() > 16<<20 || total > 128<<20 {
			return errors.New("source inventory limit")
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		out[r] = fileInfo{hash(b), i.Mode().Perm()}
		return nil
	})
	return out, e
}
func copyTree(src, dst string) error {
	inv, e := inventory(src)
	if e != nil {
		return e
	}
	if e = os.Mkdir(dst, 0700); e != nil {
		return e
	}
	for p, i := range inv {
		target := filepath.Join(dst, p)
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		b, e := os.ReadFile(filepath.Join(src, p))
		if e != nil {
			return e
		}
		if e = os.WriteFile(target, b, i.Mode); e != nil {
			return e
		}
		if e = os.Chmod(target, i.Mode); e != nil {
			return e
		}
	}
	return nil
}
func surface(s settings) ([]string, error) {
	if e := rel(s.Slot); e != nil {
		return nil, e
	}
	if e := rel(s.Integration); e != nil {
		return nil, e
	}
	if s.Integration == s.Slot || strings.HasPrefix(s.Integration, s.Slot+"/") {
		return nil, errors.New("integration must be outside the tool slot")
	}
	return []string{s.Integration, filepath.Join(s.Slot, "main.go"), filepath.Join(s.Slot, "main_test.go")}, nil
}
func module(root string, s settings) error {
	files, e := inventory(filepath.Join(root, s.Slot))
	if e != nil {
		return e
	}
	if len(files) != 3 {
		return errors.New("tool slot must contain only go.mod, main.go, main_test.go")
	}
	for _, p := range []string{"go.mod", "main.go", "main_test.go"} {
		if _, ok := files[p]; !ok {
			return fmt.Errorf("missing tool slot %s", p)
		}
	}
	b, e := os.ReadFile(filepath.Join(root, s.Slot, "go.mod"))
	if e != nil {
		return e
	}
	if string(b) != moduleText {
		return errors.New("tool module changed")
	}
	return nil
}
func tool(s settings, n string) (string, error) {
	p := s.Tools[n]
	if p == "" {
		return "", fmt.Errorf("select tools.%s", n)
	}
	p, e := exec.LookPath(p)
	if e != nil {
		return "", e
	}
	return filepath.Abs(p)
}
func run(argv []string, dir string, env []string, input []byte) (process, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 480*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	c.WaitDelay = 2 * time.Second
	c.Dir = dir
	c.Env = env
	c.Stdin = bytes.NewReader(input)
	var out, errout bounded
	c.Stdout = &out
	c.Stderr = &errout
	start := time.Now()
	err := c.Run()
	p := process{argv, 0, time.Since(start).Seconds(), out.buffer.String(), errout.buffer.String()}
	if err != nil {
		var x *exec.ExitError
		if !errors.As(err, &x) {
			return p, err
		}
		p.Exit = x.ExitCode()
		if p.Exit < 0 {
			return p, fmt.Errorf("process terminated by signal: %s", err)
		}
	}
	if ctx.Err() != nil {
		return p, ctx.Err()
	}
	if out.overflow || errout.overflow {
		return p, errors.New("process output exceeded 1 MiB")
	}
	return p, nil
}
func save(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func envWith(values map[string]string) []string {
	var out []string
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		if _, ok := values[k]; !ok {
			out = append(out, v)
		}
	}
	for k, v := range values {
		out = append(out, k+"="+v)
	}
	return out
}
func prepare(args []string) error {
	f := flag.NewFlagSet("prepare", flag.ContinueOnError)
	src := f.String("source", "", "source tree")
	out := f.String("out", "", "new prepared copy")
	integration := f.String("integration", "AGENTS.md", "existing integration file")
	slot := f.String("slot", "src/record-summary", "new dormant Go slot")
	if e := f.Parse(args); e != nil {
		return e
	}
	s := settings{Slot: *slot, Integration: *integration}
	paths, e := surface(s)
	if e != nil {
		return e
	}
	a, e := filepath.Abs(*src)
	if e != nil {
		return e
	}
	a, e = filepath.EvalSymlinks(a)
	if e != nil {
		return e
	}
	b, e := filepath.Abs(*out)
	if e != nil {
		return e
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(b))
	if e != nil {
		return fmt.Errorf("output parent must exist: %w", e)
	}
	b = filepath.Join(parent, filepath.Base(b))
	if *src == "" || *out == "" || b == a || strings.HasPrefix(b, a+string(filepath.Separator)) {
		return errors.New("select source and a new output outside source")
	}
	inv, e := inventory(a)
	if e != nil {
		return e
	}
	if _, ok := inv[*integration]; !ok {
		return errors.New("integration file must already exist")
	}
	text, e := os.ReadFile(filepath.Join(a, *integration))
	if e != nil || !utf8.Valid(text) {
		return errors.New("integration must be UTF-8")
	}
	if _, e = os.Lstat(filepath.Join(a, *slot)); !os.IsNotExist(e) {
		return errors.New("tool slot must not exist")
	}
	if e = copyTree(a, b); e != nil {
		return e
	}
	dir := filepath.Join(b, *slot)
	if e = os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	for p, v := range map[string]string{"go.mod": moduleText, "main.go": dormantMain, "main_test.go": dormantTest} {
		if e = os.WriteFile(filepath.Join(dir, p), []byte(v), 0644); e != nil {
			return e
		}
	}
	return emit(map[string]any{"version": 1, "source": b, "mutable": paths, "slot": *slot})
}
func main() {
	code, e := dispatch(os.Args[1:])
	if e != nil {
		fmt.Fprintln(os.Stderr, "tool-building:", e)
		code = 2
	}
	os.Exit(code)
}
func dispatch(args []string) (int, error) {
	if len(args) == 0 {
		return 2, errors.New("usage: tool-building prepare|propose|trial|judge|spec")
	}
	switch args[0] {
	case "prepare":
		return 0, prepare(args[1:])
	case "spec":
		return 0, spec(args[1:])
	}
	b, e := io.ReadAll(io.LimitReader(os.Stdin, (2<<20)+1))
	if e != nil {
		return 2, e
	}
	if len(b) > 2<<20 {
		return 2, errors.New("request exceeds 2 MiB")
	}
	var r request
	if e = decode(b, &r); e != nil {
		return 2, e
	}
	if r.Version != 1 {
		return 2, errors.New("request version must be 1")
	}
	switch args[0] {
	case "propose":
		return propose(r)
	case "trial":
		return trial(r)
	case "judge":
		return judge(r)
	}
	return 2, errors.New("unknown command")
}
func sortedKeys(m map[string]fileInfo) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
