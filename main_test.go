package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func runRules(t *testing.T, cwd string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	a := &app{
		out:    &stdout,
		errOut: &stderr,
		getwd:  func() (string, error) { return cwd, nil },
	}
	code := a.run(args)
	return code, stdout.String(), stderr.String()
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPrintsRootToLeafInDocumentedNameOrder(t *testing.T) {
	root := project(t)
	target := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "AGENTS.md"), "root agents")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "root claude\n")
	writeFile(t, filepath.Join(root, "a", "AGENTS.md"), "a agents\n")
	writeFile(t, filepath.Join(target, "CLAUDE.md"), "b claude")

	code, stdout, stderr := runRules(t, target)
	if code != exitYes {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	want := "root agents\nroot claude\na agents\nb claude"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	for _, text := range []string{"rules: root ", "AGENTS.md", "CLAUDE.md",
		"4 files, 40 bytes"} {
		if !strings.Contains(stderr, text) {
			t.Errorf("stderr does not contain %q:\n%s", text, stderr)
		}
	}
}

func TestListPrintsLogicalPathsAndStillReadsTheSet(t *testing.T) {
	root := project(t)
	target := filepath.Join(root, "sub")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "shared.md"), "shared")
	link := filepath.Join(target, "AGENTS.md")
	if err := os.Symlink(filepath.Join("..", "shared.md"), link); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(target, "CLAUDE.md"), "local")

	code, stdout, stderr := runRules(t, root, "-list", "sub")
	if code != exitYes {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	want := link + "\n" + filepath.Join(target, "CLAUDE.md") + "\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if !strings.Contains(stderr, " -> ") || !strings.Contains(stderr, "shared.md") {
		t.Errorf("symlink provenance missing:\n%s", stderr)
	}
}

func TestCanonicalAliasesAreListedButBodyIsPrintedOnce(t *testing.T) {
	root := project(t)
	target := filepath.Join(root, "sub")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "AGENTS.md")
	claude := filepath.Join(root, "CLAUDE.md")
	nested := filepath.Join(target, "AGENTS.md")
	writeFile(t, agents, "shared")
	if err := os.Symlink("AGENTS.md", claude); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "AGENTS.md"), nested); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runRules(t, target)
	if code != exitYes || stdout != "shared" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if strings.Count(stderr, "body not repeated") != 2 ||
		!strings.Contains(stderr, fmt.Sprintf("alias of %q", agents)) ||
		!strings.Contains(stderr, "3 files, 6 bytes") {
		t.Errorf("alias provenance missing:\n%s", stderr)
	}

	code, stdout, stderr = runRules(t, target, "-list")
	want := agents + "\n" + claude + "\n" + nested + "\n"
	if code != exitYes || stdout != want {
		t.Fatalf("-list: exit = %d, stdout = %q, want %q, stderr = %q",
			code, stdout, want, stderr)
	}
}

func TestFirstLogicalPathMayBeTheSymlink(t *testing.T) {
	root := project(t)
	agents := filepath.Join(root, "AGENTS.md")
	claude := filepath.Join(root, "CLAUDE.md")
	writeFile(t, claude, "shared")
	if err := os.Symlink("CLAUDE.md", agents); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runRules(t, root)
	if code != exitYes || stdout != "shared" ||
		!strings.Contains(stderr, fmt.Sprintf("%q -> %q (6 bytes)", agents, claude)) ||
		!strings.Contains(stderr, fmt.Sprintf("%q (alias of %q; body not repeated)",
			claude, agents)) {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestEqualBodiesInDistinctFilesAreNotDeduplicated(t *testing.T) {
	root := project(t)
	writeFile(t, filepath.Join(root, "AGENTS.md"), "same")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "same")

	code, stdout, stderr := runRules(t, root)
	if code != exitYes || stdout != "same\nsame" ||
		strings.Contains(stderr, "body not repeated") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestNearestGitMarkerDefinesRoot(t *testing.T) {
	outer := project(t)
	writeFile(t, filepath.Join(outer, "AGENTS.md"), "outer")
	inner := filepath.Join(outer, "nested")
	if err := os.MkdirAll(filepath.Join(inner, "deep"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(inner, ".git"), "gitdir: elsewhere\n")
	writeFile(t, filepath.Join(inner, "AGENTS.md"), "inner")

	code, stdout, stderr := runRules(t, filepath.Join(inner, "deep"))
	if code != exitYes || stdout != "inner" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if !strings.Contains(stderr, fmt.Sprintf("root %q", inner)) {
		t.Errorf("wrong root:\n%s", stderr)
	}
}

func TestGitSymlinkIsNotARootMarker(t *testing.T) {
	outer := project(t)
	dir := filepath.Join(outer, "sub")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outer, ".git"), filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(outer, "AGENTS.md"), "outer")

	code, stdout, stderr := runRules(t, dir)
	if code != exitYes || stdout != "outer" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestInstructionSymlinkMayNotEscapeRoot(t *testing.T) {
	root := project(t)
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.md"), "do not read")
	if err := os.Symlink(filepath.Join(outside, "secret.md"),
		filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{nil, {"-list"}} {
		code, stdout, stderr := runRules(t, root, args...)
		if code != exitNo || stdout != "" {
			t.Fatalf("%v: exit = %d, stdout = %q", args, code, stdout)
		}
		if !strings.Contains(stderr, "resolves outside project root") ||
			strings.Contains(stderr, "do not read") {
			t.Errorf("stderr = %q", stderr)
		}
	}
}

func TestAbsoluteInstructionSymlinkInsideRootIsAllowed(t *testing.T) {
	root := project(t)
	target := filepath.Join(root, "policy.md")
	writeFile(t, target, "inside")
	if err := os.Symlink(target, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runRules(t, root)
	if code != exitYes || stdout != "inside" || !strings.Contains(stderr, " -> ") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestDanglingInstructionSymlinkIsAnIOError(t *testing.T) {
	root := project(t)
	if err := os.Symlink("missing", filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runRules(t, root)
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "resolve instruction") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestRefusesSingleFileOverBoundWithoutPartialOutput(t *testing.T) {
	root := project(t)
	writeFile(t, filepath.Join(root, "AGENTS.md"), strings.Repeat("a", maxRules+1))
	code, stdout, stderr := runRules(t, root)
	if code != exitNo || stdout != "" {
		t.Fatalf("exit = %d, stdout bytes = %d", code, len(stdout))
	}
	if !strings.Contains(stderr, fmt.Sprintf("limit is %d bytes", maxRules)) {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRefusesSetOverBoundWithoutPartialOutput(t *testing.T) {
	root := project(t)
	cur := root
	for i := 0; i < 2; i++ {
		writeFile(t, filepath.Join(cur, "AGENTS.md"), strings.Repeat("a", maxRules/2+1))
		cur = filepath.Join(cur, fmt.Sprintf("d%d", i))
		if err := os.Mkdir(cur, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr := runRules(t, cur)
	if code != exitNo || stdout != "" {
		t.Fatalf("exit = %d, stdout bytes = %d", code, len(stdout))
	}
	if !strings.Contains(stderr, fmt.Sprintf("limit is %d bytes", maxRules)) {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestExactSetBoundIsAccepted(t *testing.T) {
	root := project(t)
	target := filepath.Join(root, "sub")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "AGENTS.md"), filepath.Join(root, "CLAUDE.md"),
		filepath.Join(target, "AGENTS.md"), filepath.Join(target, "CLAUDE.md"),
	} {
		writeFile(t, path, strings.Repeat("x", maxRules/4))
	}
	code, stdout, stderr := runRules(t, target)
	if code != exitYes || len(stdout) != maxRules+3 {
		t.Fatalf("exit = %d, stdout bytes = %d, stderr = %q", code, len(stdout), stderr)
	}
	if !strings.Contains(stderr, fmt.Sprintf("4 files, %d bytes", maxRules)) {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestExactSingleFileBoundAndAliasAreAccepted(t *testing.T) {
	root := project(t)
	agents := filepath.Join(root, "AGENTS.md")
	writeFile(t, agents, strings.Repeat("x", maxRules))
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runRules(t, root)
	if code != exitYes || len(stdout) != maxRules ||
		!strings.Contains(stderr, fmt.Sprintf("2 files, %d bytes", maxRules)) {
		t.Fatalf("exit = %d, stdout bytes = %d, stderr = %q", code, len(stdout), stderr)
	}
}

func TestSpecialInstructionEntryIsRefused(t *testing.T) {
	root := project(t)
	if err := os.Mkdir(filepath.Join(root, "AGENTS.md"), 0o700); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runRules(t, root)
	if code != exitNo || stdout != "" || !strings.Contains(stderr, "not a regular file") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestEmptySetSucceeds(t *testing.T) {
	root := project(t)
	code, stdout, stderr := runRules(t, root)
	if code != exitYes || stdout != "" || !strings.Contains(stderr, "0 files, 0 bytes") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestNoProjectIsANegativeResult(t *testing.T) {
	dir := t.TempDir()
	code, stdout, stderr := runRules(t, dir)
	if code != exitNo || stdout != "" || !strings.Contains(stderr, "no project root") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestTargetDirectoryIsCanonicalized(t *testing.T) {
	root := project(t)
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(real, "AGENTS.md"), "real")
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runRules(t, root, "link")
	if code != exitYes || stdout != "real" || !strings.Contains(stderr, real) {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestArgumentsAndMetadata(t *testing.T) {
	root := project(t)
	for _, tc := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"help"}, exitYes, "rules - print"},
		{[]string{"--help"}, exitYes, "rules - print"},
		{[]string{"version"}, exitYes, "rules " + version},
		{[]string{"--version"}, exitYes, "rules " + version},
		{[]string{"-wat"}, exitErr, ""},
		{[]string{"a", "b"}, exitErr, ""},
	} {
		code, stdout, stderr := runRules(t, root, tc.args...)
		if code != tc.code || !strings.Contains(stdout, tc.out) {
			t.Errorf("%v: exit = %d, stdout = %q, stderr = %q", tc.args, code, stdout, stderr)
		}
	}
}

func TestDoubleDashPermitsOptionLikeDirectory(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "-project")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "AGENTS.md"), "option-like")

	code, stdout, stderr := runRules(t, parent, "--", "-project")
	if code != exitYes || stdout != "option-like" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	code, stdout, stderr = runRules(t, parent, "-list", "--", "-project")
	if code != exitYes || stdout != filepath.Join(root, "AGENTS.md")+"\n" {
		t.Fatalf("-list: exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken") }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestOutputFailureIsReported(t *testing.T) {
	root := project(t)
	writeFile(t, filepath.Join(root, "AGENTS.md"), "text")
	var stderr bytes.Buffer
	a := &app{out: brokenWriter{}, errOut: &stderr,
		getwd: func() (string, error) { return root, nil }}
	if code := a.run(nil); code != exitErr || !strings.Contains(stderr.String(), "write stdout") {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
}

func TestShortOutputWriteIsReported(t *testing.T) {
	root := project(t)
	writeFile(t, filepath.Join(root, "AGENTS.md"), "text")
	var stderr bytes.Buffer
	a := &app{out: shortWriter{}, errOut: &stderr,
		getwd: func() (string, error) { return root, nil }}
	if code := a.run(nil); code != exitErr || !strings.Contains(stderr.String(), "short write") {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
}

func TestReadDoesNotMutateRepository(t *testing.T) {
	root := project(t)
	path := filepath.Join(root, "AGENTS.md")
	writeFile(t, path, "unchanged")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runRules(t, root)
	if code != exitYes {
		t.Fatal(stderr)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "unchanged" || before.Mode() != after.Mode() ||
		before.ModTime() != after.ModTime() || before.Size() != after.Size() {
		t.Fatalf("repository changed: before=%v after=%v body=%q", before, after, body)
	}
}

func TestPathFromRoot(t *testing.T) {
	root := filepath.Clean(string(filepath.Separator) + filepath.Join("repo"))
	dirs, err := pathFromRoot(root, filepath.Join(root, "a", "b"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{root, filepath.Join(root, "a"), filepath.Join(root, "a", "b")}
	if !slices.Equal(dirs, want) {
		t.Fatalf("dirs = %#v, want %#v", dirs, want)
	}
	other := filepath.Clean(string(filepath.Separator) + filepath.Join("other"))
	if _, err := pathFromRoot(root, other); !isRefusal(err) {
		t.Fatalf("escape error = %v", err)
	}
}

func TestHelpFitsTerminal(t *testing.T) {
	for i, line := range strings.Split(usageText, "\n") {
		if len(line) > 80 {
			t.Errorf("help line %d is %d columns: %q", i+1, len(line), line)
		}
	}
}
