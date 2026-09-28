package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type buildContext struct {
	root, source, binary, cage, goTool string
	env                                []string
	records                            []process
	sealed                             map[string]string
}

func newBuild(r request, source string) (*buildContext, error) {
	if e := module(source, r.Settings); e != nil {
		return nil, e
	}
	goTool, e := tool(r.Settings, "go")
	if e != nil {
		return nil, e
	}
	cage, e := tool(r.Settings, "cage")
	if e != nil {
		return nil, e
	}
	root, e := os.MkdirTemp("", "improve-go-")
	if e != nil {
		return nil, e
	}
	root, e = filepath.EvalSymlinks(root)
	if e != nil {
		return nil, e
	}
	b := &buildContext{root: root, source: filepath.Join(root, "source"), binary: filepath.Join(root, "tool"), goTool: goTool, cage: cage}
	if e = copyTree(source, b.source); e != nil {
		os.RemoveAll(root)
		return nil, e
	}
	for _, name := range []string{"tmp", "cache", "modcache", "gopath"} {
		if e = os.Mkdir(filepath.Join(root, name), 0700); e != nil {
			os.RemoveAll(root)
			return nil, e
		}
	}
	b.env = envWith(map[string]string{"TMPDIR": filepath.Join(root, "tmp"), "GOCACHE": filepath.Join(root, "cache"), "GOMODCACHE": filepath.Join(root, "modcache"), "GOPATH": filepath.Join(root, "gopath"), "GOTOOLCHAIN": "local", "GOWORK": "off", "GOPROXY": "off", "GOSUMDB": "off", "CGO_ENABLED": "0", "GOENV": "off", "GOFLAGS": ""})
	return b, nil
}
func (b *buildContext) command(slot string, argv []string, input []byte) (process, error) {
	boundary := []string{b.cage, "-w", b.root, "-r", b.source}
	for path, digest := range b.sealed {
		data, err := os.ReadFile(path)
		if err != nil || hash(data) != digest {
			return process{}, errors.New("compiled executable changed")
		}
		boundary = append(boundary, "-r", path)
	}
	p, e := run(append(append(boundary, "--"), argv...), filepath.Join(b.source, slot), b.env, input)
	for path, digest := range b.sealed {
		data, err := os.ReadFile(path)
		if err != nil || hash(data) != digest {
			return p, errors.New("compiled executable changed")
		}
	}
	b.records = append(b.records, p)
	if e == nil && (p.Exit == 125 || p.Exit == 130 || p.Exit == 137) {
		e = fmt.Errorf("unsafe build/check boundary status %d", p.Exit)
	}
	return p, e
}
func (b *buildContext) compile(slot string) (int, error) {
	p, e := b.command(slot, []string{b.goTool, "list", "-deps", "-json", "."}, nil)
	if e != nil || p.Exit != 0 {
		return p.Exit, e
	}
	d := json.NewDecoder(bytes.NewBufferString(p.Stdout))
	for {
		var item struct {
			Standard   bool
			ImportPath string
		}
		e = d.Decode(&item)
		if e == io.EOF {
			break
		}
		if e != nil {
			return 2, e
		}
		if !item.Standard && item.ImportPath != "example.invalid/bench-tool" {
			return 2, fmt.Errorf("non-stdlib dependency: %s", item.ImportPath)
		}
	}
	p, e = b.command(slot, []string{b.goTool, "build", "-trimpath", "-o", b.binary, "."}, nil)
	if e != nil || p.Exit != 0 {
		return p.Exit, e
	}
	p, e = b.command(slot, []string{b.goTool, "test", "-c", "-o", filepath.Join(b.root, "tool.test"), "."}, nil)
	return p.Exit, e
}
func trial(r request) (int, error) {
	tc, err := readCase(r.Case.File)
	if err != nil {
		return 2, err
	}
	if tc.Exit < 0 || tc.Exit > 255 {
		return 2, errors.New("expected_exit outside 0..255")
	}
	b, e := newBuild(r, r.Source)
	if e != nil {
		return 2, e
	}
	defer os.RemoveAll(b.root)
	buildExit, e := b.compile(r.Settings.Slot)
	if e != nil {
		return 2, e
	}
	buildSeconds := 0.0
	for _, p := range b.records {
		buildSeconds += p.Seconds
	}
	testExit, runExit := -1, -1
	var testSeconds, runSeconds float64
	binaryHash := ""
	accepted := false
	if buildExit == 0 {
		b.sealed = map[string]string{}
		for _, path := range []string{b.binary, filepath.Join(b.root, "tool.test")} {
			data, err := os.ReadFile(path)
			if err != nil {
				return 2, err
			}
			b.sealed[path] = hash(data)
		}
		data, e := os.ReadFile(b.binary)
		if e != nil {
			return 2, e
		}
		binaryHash = hash(data)
		if e = os.WriteFile(filepath.Join(r.Evidence, "tool"), data, 0700); e != nil {
			return 2, e
		}
		p, e := b.command(r.Settings.Slot, []string{filepath.Join(b.root, "tool.test"), "-test.timeout=30s"}, nil)
		if e != nil {
			return 2, e
		}
		testExit = p.Exit
		testSeconds = p.Seconds
		if testExit == 0 {
			p, e = b.command(r.Settings.Slot, []string{b.binary}, []byte(tc.Stdin))
			if e != nil {
				return 2, e
			}
			runExit = p.Exit
			runSeconds = p.Seconds
			accepted = p.Exit == tc.Exit && p.Stdout == tc.Stdout
		}
	}
	inv, e := inventory(r.Source)
	if e != nil {
		return 2, e
	}
	if e = save(filepath.Join(r.Evidence, "build.json"), map[string]any{"version": 1, "source_sha256": r.SourceHash, "source_files": inv, "binary_sha256": binaryHash, "processes": b.records, "confinement": "selected Cage; network denied; private scratch writable; source protected", "caches": "fresh private caches removed after trial"}); e != nil {
		return 2, e
	}
	return 0, emit(map[string]any{"version": 1, "score": map[string]any{"accepted": accepted, "build_exit": buildExit, "test_exit": testExit, "runner_exit": runExit, "build_seconds": buildSeconds, "test_seconds": testSeconds, "run_seconds": runSeconds, "source_sha256": r.SourceHash, "binary_sha256": binaryHash}, "cost": 0})
}

func readCase(path string) (testCase, error) {
	var tc testCase
	raw, e := os.ReadFile(path)
	if e != nil {
		return tc, e
	}
	if len(raw) > 2<<20 {
		return tc, errors.New("case exceeds 2 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, e := d.Token()
	if e != nil || token != json.Delim('{') {
		return tc, errors.New("case must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, e := d.Token()
		if e != nil {
			return tc, e
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return tc, errors.New("duplicate case field")
		}
		seen[name] = true
		var value json.RawMessage
		if e = d.Decode(&value); e != nil {
			return tc, e
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return tc, errors.New("case values must not be null")
		}
		switch name {
		case "stdin":
			e = json.Unmarshal(value, &tc.Stdin)
		case "expected_stdout":
			e = json.Unmarshal(value, &tc.Stdout)
		case "expected_exit":
			e = json.Unmarshal(value, &tc.Exit)
		default:
			return tc, fmt.Errorf("unknown case field %s", name)
		}
		if e != nil {
			return tc, e
		}
	}
	if _, e = d.Token(); e != nil {
		return tc, e
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return tc, errors.New("trailing case JSON")
	}
	if len(seen) != 3 {
		return tc, errors.New("case requires stdin, expected_stdout and expected_exit")
	}
	if tc.Exit < 0 || tc.Exit > 255 {
		return tc, errors.New("expected_exit outside 0..255")
	}
	return tc, nil
}
