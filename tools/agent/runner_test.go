package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("AGENT_NATIVE_TEST_PROCESS") == "1" {
		os.Exit(command(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func fixture(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, work, control := filepath.Join(root, "definition"), filepath.Join(root, "workspace"), filepath.Join(root, "evidence")
	for _, path := range []string{filepath.Join(home, "bin"), work} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture(t, filepath.Join(home, "AGENTS.md"), "Private identity.\n", 0600)
	writeFixture(t, filepath.Join(home, "bin/check"), "#!/bin/sh\nexit 1\n", 0700)
	return home, work, control
}

func writeFixture(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestOutputLimitAppliesThroughIOCopy(t *testing.T) {
	// Embedding bytes.Buffer would promote ReadFrom and bypass Write's limit.
	for _, size := range []int{0, 8, 9, 200000} {
		b := &boundedBuffer{limit: 8}
		// Hide strings.Reader.WriteTo as well, exercising io.Copy's generic path.
		n, err := io.Copy(b, struct{ io.Reader }{strings.NewReader(strings.Repeat("x", size))})
		if err != nil || n != int64(size) || b.Len() != min(8, size) || b.full != (size > 8) {
			t.Fatalf("size=%d consumed=%d retained=%d full=%t err=%v", size, n, b.Len(), b.full, err)
		}
	}
}

func TestRuntimeHasNoAuthoringCommands(t *testing.T) {
	for _, name := range []string{"new", "build", "learn", "amend", "act", "history"} {
		if code := command([]string{name}); code != 2 {
			t.Fatalf("runtime accepted authoring command %s: %d", name, code)
		}
	}
}

func TestDefinitionAndEvidenceSeparation(t *testing.T) {
	home, work, control := fixture(t)
	for _, tc := range []struct{ name, work, state, control string }{
		{"workspace in definition", filepath.Join(home, "bin"), "", control},
		{"state in definition", work, filepath.Join(home, "state"), control},
		{"evidence in workspace", work, "", filepath.Join(work, "evidence")},
		{"evidence contains workspace", work, "", filepath.Dir(work)},
		{"evidence aliases state", work, filepath.Join(work, "state"), filepath.Join(work, "state")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := openDefinition(home, options{work: tc.work, state: tc.state, control: tc.control}); err == nil {
				t.Fatal("accepted overlapping authority roots")
			}
		})
	}
	if _, err := openDefinition(home, options{work: work, control: control}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{control, filepath.Join(home, "work"), filepath.Join(work, "state")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("validation modified %s", path)
		}
	}
}

func TestPortableValidationRefusesControllerSymlinkBeforeMutation(t *testing.T) {
	home, work, control := fixture(t)
	if err := os.Mkdir(control, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(work, filepath.Join(control, "selections")); err != nil {
		t.Fatal(err)
	}
	_, err := openDefinition(home, options{work: work, control: control})
	if err == nil {
		t.Fatal("accepted controller symlink")
	}
	for _, path := range []string{filepath.Join(control, "runs"), filepath.Join(work, "state")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("validation modified %s", path)
		}
	}
}

func TestPrivateFileRefusesSymlinksFIFOsAndOversizedText(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "text")
	writeFixture(t, file, "private", 0600)
	link, fifo := filepath.Join(root, "link"), filepath.Join(root, "fifo")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, fifo, file} {
		if _, err := readRegular(path, 3); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	for _, body := range []string{"bad\x00text", "\xff"} {
		writeFixture(t, file, body, 0600)
		if _, err := readRegular(file, 100); err == nil {
			t.Fatalf("accepted invalid text %q", body)
		}
	}
}

func TestNestedCustomBoundaryIsNeverRebound(t *testing.T) {
	t.Setenv("PLY_DEPTH", "3")
	t.Setenv("PLY_ACTION_SHELL", "/bin/false")
	if _, err := inheritedBoundary(); err == nil {
		t.Fatal("accepted custom parent adapter")
	}
	t.Setenv("PLY_ACTION_SHELL", "/bin/sh")
	if n, err := inheritedBoundary(); err != nil || n != 3 {
		t.Fatalf("ordinary shell depth=%d err=%v", n, err)
	}
	for _, depth := range []string{"-1", "corrupt"} {
		t.Setenv("PLY_DEPTH", depth)
		if _, err := inheritedBoundary(); err == nil {
			t.Fatalf("accepted depth %q", depth)
		}
	}
}

func TestPortableGoalInputsAndExactOutcomes(t *testing.T) {
	home, work, control := fixture(t)
	root := filepath.Dir(home)
	ply := filepath.Join(root, "ply")
	writeFixture(t, ply, `#!/bin/sh
if [ "$1" = capabilities ]; then
  printf '%s\n' '{"schema":"ply.capabilities/v1","features":{"goal_file":true,"no_delegate":true}}'
  exit 0
fi
printf '%s\n' "$@" > "$FIXTURE_ROOT/argv"
while [ "$#" -gt 0 ]; do
  if [ "$1" = -goal-file ]; then cp "$2" "$FIXTURE_ROOT/goal"; fi
  shift
done
cat > "$FIXTURE_ROOT/stdin"
printf 'answer bytes\n'
exit "$FIXTURE_EXIT"
`, 0700)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	noop, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	goal := "private goal with 'quotes' and $(touch must-not-run)"
	for _, mode := range []string{"argv", "file", "stdin", "standing"} {
		for _, status := range []string{"0", "1", "2", "3", "75", "125", "130"} {
			t.Run(mode+"/"+status, func(t *testing.T) {
				_ = os.Remove(filepath.Join(home, "GOAL.md"))
				args := []string{"run", "-q", "-no-cage", "-C", work, "-evidence", control}
				input := "evidence bytes\n"
				switch mode {
				case "argv":
					args = append(args, home, "--", goal)
				case "file":
					path := filepath.Join(root, "goal-file")
					writeFixture(t, path, goal, 0600)
					args = append(args, "-goal-file", path, home)
				case "stdin":
					args = append(args, home)
					input = goal
				case "standing":
					writeFixture(t, filepath.Join(home, "GOAL.md"), goal, 0600)
					args = append(args, home)
				}
				cmd := exec.Command(self, args...)
				// Deliberately minimal: tests never use user credentials or state.
				cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + root, "TMPDIR=" + root,
					"AGENT_NATIVE_TEST_PROCESS=1", "AGENT_PLY=" + ply,
					"AGENT_ASK=" + noop, "AGENT_BRIEF=" + noop,
					"FIXTURE_ROOT=" + root, "FIXTURE_EXIT=" + status}
				cmd.Stdin = strings.NewReader(input)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				out, _ := cmd.Output()
				if got := cmd.ProcessState.ExitCode(); status != strconv.Itoa(got) {
					t.Fatalf("status=%d want=%s stderr=%s", got, status, &stderr)
				}
				if string(out) != "answer bytes\n" {
					t.Fatalf("changed answer %q", out)
				}
				b, _ := os.ReadFile(filepath.Join(root, "goal"))
				if string(b) != goal {
					t.Fatalf("goal=%q", b)
				}
				argv, _ := os.ReadFile(filepath.Join(root, "argv"))
				if bytes.Contains(argv, []byte(goal)) || bytes.Contains(argv, []byte("Private identity")) {
					t.Fatalf("private context leaked into companion argv")
				}
				b, _ = os.ReadFile(filepath.Join(root, "stdin"))
				if (mode == "stdin" && len(b) != 0) || (mode != "stdin" && !bytes.Contains(b, []byte(input))) {
					t.Fatalf("evidence=%q", b)
				}
				left, _ := filepath.Glob(filepath.Join(root, "agent-run.*"))
				if len(left) != 0 {
					t.Fatalf("private transport left after exit: %v", left)
				}
			})
		}
	}
}
