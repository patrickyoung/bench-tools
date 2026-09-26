package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func steeringCommand(t *testing.T, home, work, control, path string) *exec.Cmd {
	t.Helper()
	root := filepath.Dir(home)
	ply := filepath.Join(root, "ply")
	writeFixture(t, ply, `#!/bin/sh
if [ "$1" = capabilities ]; then
  printf '%s\n' '{"schema":"ply.capabilities/v1","features":{"goal_file":true,"no_delegate":true,"process_recording":"ply.recording/v1","action_boundary_receipt":"ply.action-boundary/v1"}}'
  exit 0
fi
printf '%s\000' "$@" > "$FIXTURE_ROOT/argv"
printf 'answer bytes\n'
printf 'progress bytes\n' >&2
exit 2
`, 0700)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	noop, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "run", "-q", "-C", work, "-evidence", control, "-steer", path, home, "--", "make a draft")
	cmd.Dir = root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + root, "TMPDIR=" + root,
		"AGENT_NATIVE_TEST_PROCESS=1", "AGENT_PLY=" + ply, "AGENT_CAGE=" + noop,
		"AGENT_ASK=" + noop, "AGENT_BRIEF=" + noop, "AGENT_RECORD=" + noop,
		"FIXTURE_ROOT=" + root}
	return cmd
}

func TestSteeringFilePassesLiterallyWithRecordingAndBoundary(t *testing.T) {
	for _, relative := range []bool{false, true} {
		t.Run(map[bool]string{false: "absolute", true: "relative"}[relative], func(t *testing.T) {
			home, work, control := fixture(t)
			root := filepath.Dir(home)
			path := filepath.Join(root, "guidance ' $(touch injection)")
			guidance := "Private new direction.\n"
			writeFixture(t, path, guidance, 0600)
			arg := path
			if relative {
				arg = filepath.Base(path)
			}
			cmd := steeringCommand(t, home, work, control, arg)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, _ := cmd.Output()
			if cmd.ProcessState.ExitCode() != 2 || string(out) != "answer bytes\n" || !strings.Contains(stderr.String(), "progress bytes\n") {
				t.Fatalf("changed process outcome: %s %q %s", cmd.ProcessState, out, &stderr)
			}
			raw, err := os.ReadFile(filepath.Join(root, "argv"))
			if err != nil {
				t.Fatal(err)
			}
			argv := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
			values := map[string]string{}
			for i := 0; i+1 < len(argv); i++ {
				if strings.HasPrefix(argv[i], "-") {
					values[argv[i]] = argv[i+1]
				}
			}
			if values["-steer"] != path || values["-record-dir"] != filepath.Join(control, "recordings") || values["-record"] == "" || values["-action-shell"] == "" || values["-action-boundary-exit"] != "125" {
				t.Fatalf("wrong composition: %q", argv)
			}
			if bytes.Contains(raw, []byte(guidance)) {
				t.Fatal("guidance leaked into argv")
			}
			if _, err := os.Stat(filepath.Join(root, "injection")); !os.IsNotExist(err) {
				t.Fatal("steering path interpreted as shell text")
			}
			if b, _ := os.ReadFile(path); string(b) != guidance {
				t.Fatal("Agent changed controller guidance")
			}
		})
	}
}

func TestUnsafeSteeringRejectedBeforeRuntimeMutation(t *testing.T) {
	for _, kind := range []string{"missing", "directory", "symlink", "fifo", "hardlink", "work", "state", "mutable-symlink"} {
		t.Run(kind, func(t *testing.T) {
			home, work, control := fixture(t)
			root := filepath.Dir(home)
			path := filepath.Join(root, "steering")
			var err error
			switch kind {
			case "directory":
				err = os.Mkdir(path, 0700)
			case "symlink", "hardlink":
				target := filepath.Join(root, "guidance")
				writeFixture(t, target, "", 0600)
				if kind == "symlink" {
					err = os.Symlink(target, path)
				} else {
					err = os.Link(target, path)
				}
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			case "work":
				path = filepath.Join(work, "steering")
				writeFixture(t, path, "", 0600)
			case "state":
				state := filepath.Join(root, "state")
				if err = os.Mkdir(state, 0700); err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(state, "steering")
				writeFixture(t, path, "", 0600)
			case "mutable-symlink":
				writeFixture(t, path, "", 0600)
				err = os.Symlink(root, filepath.Join(work, "escape"))
				path = filepath.Join(work, "escape", "steering")
			}
			if err != nil {
				t.Fatal(err)
			}
			cmd := steeringCommand(t, home, work, control, path)
			if kind == "state" {
				cmd.Args = append(cmd.Args[:2], append([]string{"-state", filepath.Join(root, "state")}, cmd.Args[2:]...)...)
			}
			out, _ := cmd.CombinedOutput()
			if cmd.ProcessState.ExitCode() != 2 || !strings.Contains(string(out), "steering") {
				t.Fatalf("unsafe steering accepted: %s %s", cmd.ProcessState, out)
			}
			for _, absent := range []string{control, filepath.Join(work, "state"), filepath.Join(root, "argv")} {
				if _, err := os.Lstat(absent); !os.IsNotExist(err) {
					t.Fatalf("validation changed %s", absent)
				}
			}
		})
	}
}
