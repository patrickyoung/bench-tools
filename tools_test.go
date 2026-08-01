package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenBoxKeepsOnlyPrograms(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "deploy"), "#!/bin/sh\n# push to staging\ntrue\n", 0o755)
	write(t, filepath.Join(dir, "notes.txt"), "not a program\n", 0o644)
	write(t, filepath.Join(dir, ".hidden"), "#!/bin/sh\n", 0o755)
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A toolbox is mostly symlinks to programs that live elsewhere.
	if err := os.Symlink("/bin/echo", filepath.Join(dir, "echo")); err != nil {
		t.Fatal(err)
	}

	b, err := openBox(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range b.Tools {
		names = append(names, tool.Name)
	}
	if got := strings.Join(names, ","); got != "deploy,echo" {
		t.Errorf("tools = %s, want deploy,echo", got)
	}
	if b.Tools[0].Synopsis != "push to staging" {
		t.Errorf("synopsis = %q, want the first comment after the shebang", b.Tools[0].Synopsis)
	}
	if b.Tools[1].Synopsis != "" {
		t.Errorf("a compiled program invented a synopsis: %q", b.Tools[1].Synopsis)
	}
}

func TestOpenBoxRefusesAnEmptyToolbox(t *testing.T) {
	if _, err := openBox(t.TempDir(), false); err == nil {
		t.Fatal("an empty toolbox was accepted; the model would have no way to act")
	}
	if _, err := openBox(filepath.Join(t.TempDir(), "nope"), false); err == nil {
		t.Fatal("a missing toolbox was accepted")
	}
	f := filepath.Join(t.TempDir(), "file")
	write(t, f, "x", 0o644)
	if _, err := openBox(f, false); err == nil {
		t.Fatal("a file was accepted as a toolbox")
	}
}

// TestPathIsTheCapabilitySet: the three modes, and the one thing that makes
// -t worth having — without -sh, the toolbox is the whole PATH.
func TestPathIsTheCapabilitySet(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "tool"), "#!/bin/sh\n", 0o755)
	t.Setenv("PATH", "/usr/bin:/bin")

	only, err := openBox(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if only.Path() != dir {
		t.Errorf("PATH = %q, want the toolbox alone (%q)", only.Path(), dir)
	}

	both, err := openBox(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := dir + ":/usr/bin:/bin"; both.Path() != want {
		t.Errorf("PATH = %q, want %q: the toolbox comes first", both.Path(), want)
	}

	shell, err := openBox("", true)
	if err != nil {
		t.Fatal(err)
	}
	if shell.Path() != "/usr/bin:/bin" {
		t.Errorf("PATH = %q, want it unchanged", shell.Path())
	}
}

// TestCatalogueIsLevelOne: names and one line each. The instructions for a
// program are `name -h`, and running it is level three; neither needed
// building, because that is how programs have always worked.
func TestCatalogueIsLevelOne(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "deploy"), "#!/usr/bin/env bash\n\n# ship it to staging\necho\n", 0o755)
	write(t, filepath.Join(dir, "zz"), "#!/bin/sh\necho\n", 0o755)
	b, err := openBox(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	cat := b.Catalogue()
	for _, want := range []string{"deploy", "ship it to staging", "zz", "nothing else"} {
		if !strings.Contains(cat, want) {
			t.Errorf("catalogue is missing %q:\n%s", want, cat)
		}
	}
	if strings.Contains(cat, "echo") {
		t.Errorf("the catalogue leaked a program's body:\n%s", cat)
	}
	shell, _ := openBox("", true)
	if c := shell.Catalogue(); !strings.Contains(c, "every program") {
		t.Errorf("-sh catalogue = %q", c)
	}
}

func TestSynopsisIsOneCleanLine(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct{ body, want string }{
		"first comment wins":    {"#!/bin/sh\n# the good one\n# the second\n", "the good one"},
		"blank lines skipped":   {"#!/bin/sh\n\n\n#   spaced   \n", "spaced"},
		"code ends the search":  {"#!/bin/sh\nset -e\n# too late\n", ""},
		"no shebang, no claim":  {"# a comment\n", ""},
		"control chars dropped": {"#!/bin/sh\n# clean\x1b[31m red\n", "clean[31m red"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-"))
			write(t, p, c.body, 0o755)
			if got := firstComment(p); got != c.want {
				t.Errorf("firstComment = %q, want %q", got, c.want)
			}
		})
	}
}
