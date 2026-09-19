package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("HIRE_TEST_PROCESS") == "1" {
		os.Exit(command(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func invokeTest(t *testing.T, root string, args ...string) (string, string, int) {
	t.Helper()
	return invokeTestEnv(t, root, nil, args...)
}

func invokeTestEnv(t *testing.T, root string, env []string, args ...string) (string, string, int) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + root, "TMPDIR=" + root, "HIRE_TEST_PROCESS=1", "HIRE_AGENT=" + filepath.Join(root, "agent")}
	cmd.Env = append(cmd.Env, env...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run()
	return stdout.String(), stderr.String(), cmd.ProcessState.ExitCode()
}

func TestBuildWeighOptIn(t *testing.T) {
	for _, tc := range []struct {
		name, value, normalized string
		set                     bool
	}{
		{name: "unset", normalized: "0"},
		{name: "empty", set: true, normalized: "0"},
		{name: "off", value: "0", set: true, normalized: "0"},
		{name: "on", value: "1", set: true, normalized: "1"},
		{name: "invalid", value: "yes", set: true},
		{name: "whitespace", value: " 1", set: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			work := filepath.Join(root, "workspace")
			if err := os.Mkdir(work, 0700); err != nil {
				t.Fatal(err)
			}
			stub := "#!/bin/sh\nprintf '%s' \"${BENCH_WEIGH-unset}\" > \"$HOME/agent-weigh\"\nexit 0\n"
			if err := os.WriteFile(filepath.Join(root, "agent"), []byte(stub), 0700); err != nil {
				t.Fatal(err)
			}
			var env []string
			if tc.set {
				env = []string{"BENCH_WEIGH=" + tc.value}
			}
			_, stderr, code := invokeTestEnv(t, root, env, "build", "-C", work, "-evidence", filepath.Join(root, "evidence"), "--", "Build a reviewer")
			body, err := os.ReadFile(filepath.Join(root, "agent-weigh"))
			if tc.normalized == "" {
				if code != 2 || !strings.Contains(stderr, "BENCH_WEIGH") || !os.IsNotExist(err) {
					t.Fatalf("invalid opt-in launched Agent or wrong outcome: code=%d stderr=%s marker=%q err=%v", code, stderr, body, err)
				}
				return
			}
			if code != 0 || err != nil || string(body) != tc.normalized {
				t.Fatalf("child policy: code=%d stderr=%s setting=%q err=%v", code, stderr, body, err)
			}
		})
	}
}

func TestNewDefinitionWithoutRunner(t *testing.T) {
	root := t.TempDir()
	definition := filepath.Join(root, "definition")
	out, stderr, code := invokeTest(t, root, "new", definition, "A private specialist")
	if code != 0 || out != definition+"/AGENTS.md\n" {
		t.Fatalf("new: code=%d out=%q stderr=%s", code, out, stderr)
	}
	for _, name := range []string{"AGENTS.md", "GOAL.md", "bin/check"} {
		if _, err := os.Stat(filepath.Join(definition, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"work", "state", ".agent"} {
		if _, err := os.Lstat(filepath.Join(definition, name)); !os.IsNotExist(err) {
			t.Fatalf("portable definition contains %s", name)
		}
	}
	_, _, code = invokeTest(t, root, "new", definition, "Overwrite it")
	if code == 0 {
		t.Fatal("overwrote an existing definition")
	}
}

func TestNewLegacyHome(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	_, stderr, code := invokeTest(t, root, "new", "-home", home)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	for _, name := range []string{"work/actions", "state/kv", ".agent/runs"} {
		if _, err := os.Stat(filepath.Join(home, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuilderDoesNotOfferRuntimeCommands(t *testing.T) {
	for _, cmd := range []string{"run", "tick", "specialist", "serve"} {
		_, _, code := invokeTest(t, t.TempDir(), cmd)
		if code != 2 {
			t.Fatalf("%s: got %d", cmd, code)
		}
	}
}

func TestBuildUsesOnlyPublicAgent(t *testing.T) {
	root := t.TempDir()
	agent := filepath.Join(root, "agent")
	stub := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HOME/argv\"\ncat >/dev/null\nprintf 'unfinished answer\\n'\nexit 2\n"
	if err := os.WriteFile(agent, []byte(stub), 0700); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "workspace")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := invokeTest(t, root, "build", "-C", work, "-evidence", filepath.Join(root, "evidence"), "-m", "fixture/model", "--", "Build a reviewer")
	if code != 2 || out != "unfinished answer\n" {
		t.Fatalf("code=%d out=%q stderr=%s", code, out, stderr)
	}
	argv, err := os.ReadFile(filepath.Join(root, "argv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(argv), "run\n-C\n") || !strings.Contains(string(argv), "fixture/model\n") {
		t.Fatalf("bad Agent invocation: %s", argv)
	}
	remaining, _ := filepath.Glob(filepath.Join(root, "hire-definition.*"))
	if len(remaining) != 0 {
		t.Fatalf("private builder transport left after failed run: %v", remaining)
	}
}
