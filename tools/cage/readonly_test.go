package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadOnlyPolicyAndWritableAncestorAnchors(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work, scratch := filepath.Join(root, "work"), filepath.Join(root, "scratch")
	input := filepath.Join(work, "nested", "inputs")
	for _, dir := range []string{input, scratch} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMPDIR", scratch)
	opts, err := parseOptions([]string{"-w", work, "-r", input, "-r", input, "--", "true"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := makePolicy(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.reads, []string{input}) || !reflect.DeepEqual(protectedAncestors(p), []string{work, filepath.Join(work, "nested")}) {
		t.Fatalf("incorrect protected policy: %+v %v", p, protectedAncestors(p))
	}
	if _, err := parseOptions([]string{"-r"}); err == nil {
		t.Fatal("missing path accepted")
	}
}

func TestProtectedInputRejectsUnsafeContentsAndConflictingRoots(t *testing.T) {
	root := t.TempDir()
	work, scratch := filepath.Join(root, "work"), filepath.Join(root, "scratch")
	if err := os.MkdirAll(work, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", scratch)
	input := filepath.Join(work, "input")
	if err := os.WriteFile(input, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{filepath.Join(work, "missing"), scratch, work} {
		if _, err := makePolicy(options{writes: []string{work}, reads: []string{name}}); err == nil {
			t.Fatalf("accepted conflicting/missing %s", name)
		}
	}
	link := filepath.Join(work, "link")
	if err := os.Symlink(input, link); err != nil {
		t.Fatal(err)
	}
	if _, err := makePolicy(options{writes: []string{work}, reads: []string{link}}); err == nil {
		t.Fatal("accepted symlink")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(input, link); err != nil {
		t.Fatal(err)
	}
	if _, err := makePolicy(options{writes: []string{work}, reads: []string{input}}); err == nil || !strings.Contains(err.Error(), "link") {
		t.Fatalf("hardlink accepted: %v", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if _, err := makePolicy(options{writes: []string{work}, reads: []string{input}}); err != nil {
		t.Fatal(err)
	}
}

func TestProtectedInputRejectsMutableParentThroughStableAlias(t *testing.T) {
	root := t.TempDir()
	work, scratch := filepath.Join(root, "work"), filepath.Join(root, "scratch")
	for _, path := range []string{filepath.Join(work, "original"), scratch} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(work, "original", "input"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(work, "original"), filepath.Join(work, "mutable")); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "stable")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", scratch)
	_, err := makePolicy(options{writes: []string{work}, reads: []string{filepath.Join(alias, "work", "mutable", "input")}})
	if err == nil || !strings.Contains(err.Error(), "mutable symlink") {
		t.Fatalf("mutable alias accepted: %v", err)
	}
}
