package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestProtectedInputValidationBeforeEffects(t *testing.T) {
	for _, kind := range []string{"missing", "symlink", "tree-symlink", "hardlink", "fifo", "mutable-alias", "aliased-mutable-link", "state", "control"} {
		t.Run(kind, func(t *testing.T) {
			home, work, control := fixture(t)
			root := filepath.Dir(home)
			input := filepath.Join(work, "input")
			writeFixture(t, input, "original", 0600)
			var err error
			switch kind {
			case "missing":
				input += "missing"
			case "symlink":
				err = os.Symlink(input, input+"link")
				input += "link"
			case "tree-symlink":
				input = filepath.Join(work, "inputs")
				err = os.Mkdir(input, 0700)
				if err == nil {
					err = os.Symlink(filepath.Join(work, "input"), filepath.Join(input, "alias"))
				}
			case "hardlink":
				err = os.Link(input, input+"link")
			case "fifo":
				input += "fifo"
				err = syscall.Mkfifo(input, 0600)
			case "mutable-alias", "aliased-mutable-link":
				err = os.Symlink(root, filepath.Join(work, "link"))
				input = filepath.Join(work, "link", "workspace", "input")
				if kind == "aliased-mutable-link" && err == nil {
					err = os.Symlink(root, filepath.Join(root, "alias"))
					input = filepath.Join(root, "alias", "workspace", "link", "workspace", "input")
				}
			case "state":
				input = filepath.Join(work, "state")
				err = os.Mkdir(input, 0700)
			case "control":
				input = control
				err = os.Mkdir(input, 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			d, err := openDefinition(home, options{work: work, control: control})
			if err != nil {
				if kind == "hardlink" {
					return
				}
				t.Fatal(err)
			}
			if _, err := selectedReadOnly(options{readOnly: []string{input}}, d); err == nil {
				t.Fatal("unsafe protected input accepted")
			}
			if _, err := os.Stat(filepath.Join(control, "runs")); !os.IsNotExist(err) {
				t.Fatal("validation mutated evidence")
			}
		})
	}
}

func TestInputProtectionOperatorCannotBeDisabled(t *testing.T) {
	t.Setenv("AGENT_PROTECT_INPUTS", "1")
	for _, args := range [][]string{{"-protect-inputs=false"}, {"-no-cage"}, {"-no-cage", "-read-only", "x"}} {
		if _, _, err := parse(args); err == nil {
			t.Fatalf("operator protection disabled: %q", args)
		}
	}
	o, _, err := parse(nil)
	if err != nil || !o.protectInputs {
		t.Fatalf("operator protection not selected: %v", err)
	}
}

func TestInputProtectionDefaultsAreFrozenAndMissingOptional(t *testing.T) {
	home, work, control := fixture(t)
	d, err := openDefinition(home, options{work: work, control: control})
	if err != nil {
		t.Fatal(err)
	}
	p, err := selectedReadOnly(options{protectInputs: true}, d)
	if err != nil || len(p) != 0 {
		t.Fatalf("missing optional paths: %v %v", p, err)
	}
	writeFixture(t, filepath.Join(work, "request.md"), "request", 0600)
	if err := os.Mkdir(filepath.Join(work, "inputs"), 0700); err != nil {
		t.Fatal(err)
	}
	p, err = selectedReadOnly(options{protectInputs: true, readOnly: []string{filepath.Join(work, "request.md")}}, d)
	if err != nil || len(p) != 2 {
		t.Fatalf("selection not deduplicated: %v %v", p, err)
	}
}

func TestInputProtectionRefusingCageStopsBeforeRuntime(t *testing.T) {
	for _, kind := range []string{"run", "tick"} {
		t.Run(kind, func(t *testing.T) {
			home, work, control := fixture(t)
			root := filepath.Dir(home)
			writeFixture(t, filepath.Join(home, "HEARTBEAT.md"), "do work", 0600)
			writeFixture(t, filepath.Join(home, "bin/wake"), "#!/bin/sh\ntouch wake-called\nexit 1\n", 0700)
			cage := filepath.Join(root, "cage")
			writeFixture(t, cage, "#!/bin/sh\nexit 2\n", 0700)
			cmd := steeringCommand(t, home, work, control, filepath.Join(root, "steer"))
			writeFixture(t, filepath.Join(root, "steer"), "", 0600)
			cmd.Args[1] = kind
			cmd.Args = append(cmd.Args[:2], append([]string{"-protect-inputs"}, cmd.Args[2:]...)...)
			cmd.Env = append(cmd.Env, "AGENT_CAGE="+cage)
			out, _ := cmd.CombinedOutput()
			if cmd.ProcessState.ExitCode() != 125 || !strings.Contains(string(out), "cannot establish input protection") {
				t.Fatalf("refusal lost: %s %s", cmd.ProcessState, out)
			}
			for _, p := range []string{control, filepath.Join(work, "state"), filepath.Join(work, "wake-called"), filepath.Join(root, "argv")} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatalf("refusal mutated %s", p)
				}
			}
		})
	}
}

func TestFrozenInputProtectionRejectsMutableConfiguration(t *testing.T) {
	home, work, control := fixture(t)
	_ = control
	state := filepath.Join(work, "state")
	temp := filepath.Join(filepath.Dir(home), "tmp")
	for _, dir := range []string{state, temp} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	exe, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(work, "policy.json")
	// An action cannot substitute its own writable grant file, even with valid roots.
	writeFixture(t, path, `{"version":1,"work":`+shellJSON(work)+`,"state":`+shellJSON(state)+`,"temp":`+shellJSON(temp)+`,"cage":`+shellJSON(exe)+`,"check":`+shellJSON(filepath.Join(home, "bin/check"))+`,"network":false,"read_only":[]}`, 0600)
	if _, err := readInputProtection(path); err == nil {
		t.Fatal("mutable authority file accepted")
	}
}
func shellJSON(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestProtectionKeepsExistingWorkspaceRequirement(t *testing.T) {
	home, work, control := fixture(t)
	if err := os.Remove(work); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		if _, err := openDefinition(home, options{work: work, control: control, protectInputs: enabled}); err == nil {
			t.Fatalf("missing workspace accepted with protection=%t", enabled)
		}
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Fatal("validation created workspace")
	}
}

func TestProtectedTemporaryParentRefusedBeforeTransportWrites(t *testing.T) {
	home, work, control := fixture(t)
	root := filepath.Dir(home)
	temp := filepath.Join(root, "protected-temp")
	if err := os.Mkdir(temp, 0700); err != nil {
		t.Fatal(err)
	}
	steering := filepath.Join(root, "steer")
	writeFixture(t, steering, "", 0600)
	cmd := steeringCommand(t, home, work, control, steering)
	cmd.Args = append(cmd.Args[:2], append([]string{"-read-only", temp}, cmd.Args[2:]...)...)
	cmd.Env = append(cmd.Env, "TMPDIR="+temp)
	out, _ := cmd.CombinedOutput()
	if cmd.ProcessState.ExitCode() != 2 || !strings.Contains(string(out), "contains temporary directory") {
		t.Fatalf("unsafe temp accepted: %s %s", cmd.ProcessState, out)
	}
	entries, err := os.ReadDir(temp)
	if err != nil || len(entries) != 0 {
		t.Fatalf("protected input changed: %v %v", entries, err)
	}
	if _, err := os.Stat(control); !os.IsNotExist(err) {
		t.Fatal("validation changed evidence")
	}
}
