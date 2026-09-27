package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if log := os.Getenv("TOOL_BUILDING_FAKE_HIRE_LOG"); log != "" {
		data, _ := json.Marshal(os.Args[1:])
		_ = os.WriteFile(log, data, 0600)
		if len(os.Args) > 1 && os.Args[1] == "verify" {
			_, _ = os.Stderr.WriteString("missing README.md\n")
			os.Exit(1)
		}
		os.Exit(99)
	}
	os.Exit(m.Run())
}

func TestRealProposerRefusesInvalidDefinitionBeforeBuild(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	work := filepath.Join(root, "work")
	evidence := filepath.Join(root, "evidence")
	for _, p := range []string{source, work, evidence} {
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(source, "AGENTS.md"), []byte("original"), 0644); e != nil {
		t.Fatal(e)
	}
	log := filepath.Join(root, "calls.json")
	t.Setenv("TOOL_BUILDING_FAKE_HIRE_LOG", log)
	r := request{Version: 1, Source: source, Work: work, Evidence: evidence, Files: map[string]string{"AGENTS.md": "original"}, Settings: settings{Slot: "src/record-summary", Integration: "AGENTS.md", Model: "unused", Tools: map[string]string{"hire": os.Args[0]}}}
	code, e := propose(r)
	if code != 2 || e == nil || !strings.Contains(e.Error(), "before authoring") {
		t.Fatalf("code=%d err=%v", code, e)
	}
	var argv []string
	if e = readJSON(log, &argv); e != nil {
		t.Fatal(e)
	}
	if len(argv) != 2 || argv[0] != "verify" || argv[1] != source {
		t.Fatalf("unexpected public calls: %v", argv)
	}
	var receipt process
	if e = readJSON(filepath.Join(evidence, "hire-preflight.json"), &receipt); e != nil {
		t.Fatal(e)
	}
	if receipt.Exit != 1 || !strings.Contains(receipt.Stderr, "README") {
		t.Fatal("preflight outcome not retained")
	}
	if _, e = os.Stat(filepath.Join(work, "expert")); !os.IsNotExist(e) {
		t.Fatal("authoring copy created before failed preflight")
	}
	data, _ := os.ReadFile(filepath.Join(source, "AGENTS.md"))
	if string(data) != "original" {
		t.Fatal("source changed")
	}
}

func TestOutputSubprocess(t *testing.T) {
	if os.Getenv("TOOL_BUILDING_OUTPUT_CHILD") == "1" {
		_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), 2<<20))
		os.Exit(0)
	}
	env := append(os.Environ(), "TOOL_BUILDING_OUTPUT_CHILD=1")
	p, e := run([]string{os.Args[0], "-test.run=^TestOutputSubprocess$"}, "", env, nil)
	if e == nil || !strings.Contains(e.Error(), "exceeded") || len(p.Stdout) != 1<<20 {
		t.Fatalf("output cap: bytes=%d err=%v", len(p.Stdout), e)
	}
}
func TestPreparePreservesOriginal(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "source")
	out := filepath.Join(base, "prepared")
	os.Mkdir(src, 0700)
	os.WriteFile(filepath.Join(src, "AGENTS.md"), []byte("Original guidance.\n"), 0600)
	before, e := inventory(src)
	if e != nil {
		t.Fatal(e)
	}
	if e = prepare([]string{"-source", src, "-out", out}); e != nil {
		t.Fatal(e)
	}
	after, _ := inventory(src)
	if len(after) != 1 || before["AGENTS.md"] != after["AGENTS.md"] {
		t.Fatal("original changed")
	}
	s := settings{Slot: "src/record-summary", Integration: "AGENTS.md"}
	if e = module(out, s); e != nil {
		t.Fatal(e)
	}
	i, _ := os.Stat(filepath.Join(out, "AGENTS.md"))
	if i.Mode().Perm() != 0600 {
		t.Fatal("integration mode changed")
	}
	if e = prepare([]string{"-source", src, "-out", out}); e == nil {
		t.Fatal("reused destination")
	}
}
func TestUnsafeSurfacesAndSource(t *testing.T) {
	for _, p := range []string{"", ".", "../escape", "/absolute", "a/../b", "a\nb", "a\\b"} {
		if rel(p) == nil {
			t.Fatalf("allowed %q", p)
		}
	}
	root := t.TempDir()
	os.Symlink("/tmp", filepath.Join(root, "escape"))
	if _, e := inventory(root); e == nil {
		t.Fatal("source symlink accepted")
	}
}
func TestScopeModesAndInventory(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"AGENTS.md", "immutable"} {
		os.WriteFile(filepath.Join(root, p), []byte("before"), 0644)
	}
	before, _ := inventory(root)
	r := request{Files: map[string]string{"AGENTS.md": "before"}}
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("after"), 0644)
	changes, e := proposalChanges(root, before, r)
	if e != nil || len(changes) != 1 {
		t.Fatalf("valid: %v %v", changes, e)
	}
	os.Chmod(filepath.Join(root, "AGENTS.md"), 0600)
	if _, e = proposalChanges(root, before, r); e == nil {
		t.Fatal("mode change accepted")
	}
	os.Chmod(filepath.Join(root, "AGENTS.md"), 0644)
	os.WriteFile(filepath.Join(root, "immutable"), []byte("after"), 0644)
	if _, e = proposalChanges(root, before, r); e == nil {
		t.Fatal("immutable changed")
	}
	os.WriteFile(filepath.Join(root, "new"), []byte("new"), 0644)
	if _, e = proposalChanges(root, before, r); e == nil {
		t.Fatal("new file accepted")
	}
}
func TestJudgeRequiresValidPairedCapability(t *testing.T) {
	valid := `{"version":1,"baseline":[{"case":"one","repeat":0,"observation":{"score":{"accepted":false,"build_exit":0,"test_exit":0}}}],"candidate":[{"case":"one","repeat":0,"observation":{"score":{"accepted":true,"build_exit":0,"test_exit":0}}}]}`
	for _, tc := range []struct {
		name, raw string
		want      int
	}{{"valid", valid, 0}, {"baseline build broken", strings.Replace(valid, `"build_exit":0`, `"build_exit":1`, 1), 1}, {"missing baseline accepted", strings.Replace(valid, `"accepted":false,`, "", 1), 1}, {"candidate rejected", strings.Replace(valid, `"accepted":true`, `"accepted":false`, 1), 1}, {"different case", strings.Replace(valid, `"case":"one"`, `"case":"other"`, 1), 2}} {
		t.Run(tc.name, func(t *testing.T) {
			var r request
			if e := json.Unmarshal([]byte(tc.raw), &r); e != nil {
				t.Fatal(e)
			}
			code, _ := judge(r)
			if code != tc.want {
				t.Fatalf("code=%d want=%d", code, tc.want)
			}
		})
	}
}
func TestBoundedDoesNotExposeReaderFrom(t *testing.T) {
	var b bounded
	if _, ok := any(&b).(io.ReaderFrom); ok {
		t.Fatal("ReaderFrom bypasses cap")
	}
}
func TestCompiledSealRejectsReplacement(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "tool")
	os.WriteFile(bin, []byte("original"), 0700)
	b := buildContext{root: root, source: root, sealed: map[string]string{bin: hash([]byte("original"))}}
	os.WriteFile(bin, []byte("replacement"), 0700)
	if _, e := b.command(".", []string{"unused"}, nil); e == nil || !strings.Contains(e.Error(), "changed") {
		t.Fatalf("replacement not refused: %v", e)
	}
}

func TestCaseRequiresExactOracle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case.json")
	for _, raw := range []string{`{}`, `null`, `{"stdin":"","expected_stdout":"","expected_exit":null}`, `{"stdin":"","expected_stdout":"","expected_exit":0,"expected_stdout":"x"}`, `{"stdin":"","expected_stdout":"","expected_exit":0,"extra":1}`, `{"stdin":"","expected_stdout":"","expected_exit":256}`} {
		if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := readCase(path); e == nil {
			t.Fatalf("accepted malformed oracle %s", raw)
		}
	}
	os.WriteFile(path, []byte(`{"stdin":"","expected_stdout":"","expected_exit":0}`), 0600)
	if _, e := readCase(path); e != nil {
		t.Fatal(e)
	}
}
