package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

var testApp string
var testArgvApp string

const fixtureEntry = `#!/bin/sh
set -eu
helper > "$BUNDLE_CONTROL/helper.txt"
printf '%s\n' "$1" > mode.txt
printf '%s\n' "$BUNDLE_ROOT" "$BUNDLE_CONTROL" "$BUNDLE_STATE" "$BUNDLE_GOAL_FILE" "$BUNDLE_WORKSPACE" "$BUNDLE_INPUT" "$BUNDLE_WORK" "$BUNDLE_OUTPUT" "$(pwd -P)" > paths.txt
cat > evidence.txt
goal=$(cat "$BUNDLE_GOAL_FILE")
case "$goal" in
  block) echo $$ > adapter.pid; while :; do sleep 1; done ;;
  status-*) printf '\000\377artifact'; printf 'child diagnostic\n' >&2; exit "${goal#status-}" ;;
esac
cp "$BUNDLE_GOAL_FILE" "$BUNDLE_OUTPUT/result.txt"
cat "$BUNDLE_OUTPUT/result.txt"
`

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "bundle-test-build-")
	if err != nil {
		panic(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		panic(err)
	}
	testApp = filepath.Join(root, "app")
	appDir, prefix := fixtureSource(root)
	if err := build([]string{"-o", testApp, "-runtime", prefix, appDir}); err != nil {
		os.RemoveAll(root)
		panic(err)
	}
	// The build directory and destination name do not enter the distributable.
	if err := build([]string{"-o", testApp + "-again", "-runtime", prefix, appDir}); err != nil {
		panic(err)
	}
	first, _ := os.ReadFile(testApp)
	again, _ := os.ReadFile(testApp + "-again")
	if !bytes.Equal(first, again) {
		os.RemoveAll(root)
		panic("nonreproducible native bundle build")
	}
	testArgvApp = filepath.Join(root, "argv-app")
	put(filepath.Join(appDir, "bin/entry"), []byte(fixtureArgvEntry), 0755)
	argvDefinition := application{Schema: 1, Name: "argv-fixture", Description: "literal argv fixture", Interface: "argv", Entry: "bin/entry", Files: []string{"bin/entry"}, Requires: []string{"helper", "cat"}}
	b, _ := json.Marshal(argvDefinition)
	put(filepath.Join(appDir, "app.json"), b, 0644)
	if err := build([]string{"-o", testArgvApp, "-runtime", prefix, appDir}); err != nil {
		os.RemoveAll(root)
		panic(err)
	}
	// The deliverable has no runtime reliance on its build inputs.
	os.RemoveAll(appDir)
	os.RemoveAll(prefix)
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

func put(name string, b []byte, mode os.FileMode) {
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		panic(err)
	}
	if err := os.WriteFile(name, b, 0600); err != nil {
		panic(err)
	}
	if err := os.Chmod(name, mode); err != nil {
		panic(err)
	}
}

func fixtureSource(root string) (string, string) {
	appDir, prefix := filepath.Join(root, "app source"), filepath.Join(root, "runtime prefix")
	put(filepath.Join(appDir, "bin/entry"), []byte(fixtureEntry), 0755)
	a := application{Schema: 1, Name: "fixture", Description: "offline contract fixture", Entry: "bin/entry", Files: []string{"bin/entry"}, Requires: []string{"helper", "cat"}, Followup: true, Resume: true}
	b, _ := json.Marshal(a)
	put(filepath.Join(appDir, "app.json"), b, 0644)
	tool := []byte("#!/bin/sh\nprintf 'bundled helper\\n'\n")
	dir := filepath.Join(prefix, "lib/bench-tools/helper")
	put(filepath.Join(dir, "bin/helper"), tool, 0755)
	put(filepath.Join(dir, "LICENSE"), []byte("fixture license\n"), 0644)
	receipt := map[string]any{"schema": 1, "name": "helper", "commands": []string{"helper"}, "files": []fileRecord{{Path: "bin/helper", SHA256: digest(tool), Mode: 0755}, {Path: "LICENSE", SHA256: digest([]byte("fixture license\n")), Mode: 0644}}}
	b, _ = json.Marshal(receipt)
	put(filepath.Join(dir, "package.json"), b, 0644)
	return appDir, prefix
}

func fixture(t *testing.T) (string, []string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + root, "XDG_STATE_HOME=" + filepath.Join(root, "state")}
	return root, env
}

func runApp(t *testing.T, root string, env []string, input []byte, args ...string) (int, []byte, []byte) {
	t.Helper()
	return runExecutable(t, testApp, root, env, input, args...)
}

func runExecutable(t *testing.T, executable, root string, env []string, input []byte, args ...string) (int, []byte, []byte) {
	t.Helper()
	cmd := exec.Command(executable, args...)
	cmd.Dir, cmd.Env = root, env
	cmd.Stdin = bytes.NewReader(input)
	var out, diagnostics bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostics
	err := cmd.Run()
	code := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return code, out.Bytes(), diagnostics.Bytes()
}

func mustRun(t *testing.T, root string, env []string, input []byte, args ...string) []byte {
	t.Helper()
	code, out, diagnostics := runApp(t, root, env, input, args...)
	if code != 0 {
		t.Fatalf("%v: exit %d: %s", args, code, diagnostics)
	}
	return out
}

func paths(t *testing.T, work string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(work, "working", "paths.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestWorkspacePreparedInputsAndLayout(t *testing.T) {
	for _, selector := range []string{"-w", "-o"} {
		t.Run(selector, func(t *testing.T) {
			root, env := fixture(t)
			work := filepath.Join(root, "job with spaces")
			input := filepath.Join(work, "input", "brief.txt")
			put(input, []byte("selected original\x00\xff"), 0644)
			// Ordinary caller-created directories need not have a 0700 mode.
			if err := os.Chmod(filepath.Dir(input), 0755); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"BUNDLE_WORKSPACE", "BUNDLE_INPUT", "BUNDLE_WORK", "BUNDLE_OUTPUT"} {
				env = append(env, key+"=stale-inherited-path")
			}
			mustRun(t, root, env, nil, selector, work, "first result")
			p := paths(t, work)
			want := []string{work, filepath.Join(work, "input"), filepath.Join(work, "working"), filepath.Join(work, "output"), filepath.Join(work, "working")}
			if strings.Join(p[4:], "\n") != strings.Join(want, "\n") {
				t.Fatalf("wrong workspace selectors/cwd: %q", p[4:])
			}
			mustRun(t, root, env, nil, "-c", work, "revised result")
			mustRun(t, root, env, nil, "-resume", work)
			b, err := os.ReadFile(input)
			if err != nil || string(b) != "selected original\x00\xff" {
				t.Fatalf("staged input changed: %q, %v", b, err)
			}
			b, err = os.ReadFile(filepath.Join(work, "output", "result.txt"))
			if err != nil || string(b) != "revised result" {
				t.Fatalf("missing deliverable: %q, %v", b, err)
			}
			entries, err := os.ReadDir(work)
			if err != nil || len(entries) != 3 {
				t.Fatalf("workspace root polluted: %v, %v", entries, err)
			}
			entries, err = os.ReadDir(filepath.Join(work, "output"))
			if err != nil || len(entries) != 1 || entries[0].Name() != "result.txt" {
				t.Fatalf("scratch leaked into output: %v, %v", entries, err)
			}
		})
	}
	root, env := fixture(t)
	work := filepath.Join(root, "empty")
	if err := os.Mkdir(work, 0755); err != nil {
		t.Fatal(err)
	}
	mustRun(t, root, env, nil, "-w", work, "empty prepared root")
	for _, args := range [][]string{{"-w", work, "-o", work, "goal"}, {"-w", work, "-c", work, "goal"}, {"-w", "", "goal"}} {
		if code, _, _ := runApp(t, root, env, nil, args...); code != 2 {
			t.Fatalf("accepted conflicting/empty selector: %q (%d)", args, code)
		}
	}
}

func TestWorkspaceRefusalsBeforeGoalAdmission(t *testing.T) {
	for _, name := range []string{"input", "working", "output"} {
		for _, mutation := range []string{"missing", "symlink", "file"} {
			t.Run(name+"-"+mutation, func(t *testing.T) {
				root, env := fixture(t)
				work := filepath.Join(root, "job")
				mustRun(t, root, env, nil, "-w", work, "first")
				p := paths(t, work)
				binding := filepath.Join(filepath.Dir(p[1]), "run.json")
				before, _ := os.ReadFile(binding)
				selected := filepath.Join(work, name)
				if err := os.Rename(selected, selected+"-saved"); err != nil {
					t.Fatal(err)
				}
				switch mutation {
				case "symlink":
					if err := os.Symlink(selected+"-saved", selected); err != nil {
						t.Fatal(err)
					}
				case "file":
					put(selected, []byte("not a directory"), 0600)
				}
				for _, args := range [][]string{{"-c", work, "second"}, {"-resume", work}} {
					if code, _, _ := runApp(t, root, env, nil, args...); code == 0 {
						t.Fatal("accepted changed layout")
					}
				}
				after, _ := os.ReadFile(binding)
				if !bytes.Equal(before, after) {
					t.Fatal("admitted goal before rejecting layout")
				}
				if mutation == "missing" {
					if _, err := os.Lstat(selected); !os.IsNotExist(err) {
						t.Fatal("silently recreated missing directory")
					}
				}
			})
		}
	}
	for _, name := range []string{"working", "output", "unrelated", "input-link", "input-file"} {
		t.Run("prepared-"+name, func(t *testing.T) {
			root, env := fixture(t)
			work := filepath.Join(root, "job")
			if err := os.Mkdir(work, 0755); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "input-link":
				if err := os.Symlink(root, filepath.Join(work, "input")); err != nil {
					t.Fatal(err)
				}
			case "input-file":
				put(filepath.Join(work, "input"), []byte("preserve"), 0600)
			default:
				if err := os.Mkdir(filepath.Join(work, name), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if code, _, _ := runApp(t, root, env, nil, "-w", work, "goal"); code == 0 {
				t.Fatal("adopted unrelated or incomplete workspace")
			}
			entries, err := os.ReadDir(work)
			if err != nil || len(entries) != 1 {
				t.Fatalf("refusal modified staged root: %v, %v", entries, err)
			}
		})
	}
}

func TestFreshWorkspaceClaimAcrossPackageLocks(t *testing.T) {
	// Different packages do not share a run lock. Exercise their shared layout
	// claim directly: exactly one may succeed, leaving every required folder.
	for attempt := 0; attempt < 40; attempt++ {
		work := t.TempDir()
		if attempt%2 == 0 {
			put(filepath.Join(work, "input", "source.txt"), []byte("preserve"), 0644)
		}
		start := make(chan struct{})
		results := make(chan error, 4)
		var wg sync.WaitGroup
		for i := 0; i < cap(results); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				results <- prepareWorkspace(work, true)
			}()
		}
		close(start)
		wg.Wait()
		close(results)
		accepted := 0
		for err := range results {
			if err == nil {
				accepted++
			}
		}
		if accepted != 1 {
			t.Fatalf("concurrent fresh admissions: got %d, want 1", accepted)
		}
		if err := prepareWorkspace(work, false); err != nil {
			t.Fatalf("successful admission lost its layout: %v", err)
		}
		if attempt%2 == 0 {
			b, err := os.ReadFile(filepath.Join(work, "input", "source.txt"))
			if err != nil || string(b) != "preserve" {
				t.Fatalf("concurrent admission changed staged input: %q, %v", b, err)
			}
		}
	}
}

func TestAppStreamsAndContinuation(t *testing.T) {
	root, env := fixture(t)
	work := filepath.Join(root, "output with spaces")
	goal := "-- $(touch BAD); `touch BAD2`\nquoted \"text\""
	evidence := []byte("evidence\x00\xff")
	if out := mustRun(t, root, env, evidence, "-w", work, "--", goal); string(out) != goal {
		t.Fatalf("changed goal: %q", out)
	}
	b, _ := os.ReadFile(filepath.Join(work, "working", "evidence.txt"))
	if !bytes.Equal(b, evidence) {
		t.Fatal("stdin not preserved")
	}
	p := paths(t, work)
	if overlaps(p[0], p[1]) || overlaps(work, p[1]) || overlaps(p[2], p[1]) {
		t.Fatal("mixed authority roots")
	}
	for _, name := range []string{"BAD", "BAD2"} {
		if _, err := os.Stat(filepath.Join(work, name)); !os.IsNotExist(err) {
			t.Fatal("evaluated goal as shell")
		}
	}
	if code, _, _ := runApp(t, root, env, nil, "-w", work, "another"); code == 0 {
		t.Fatal("reused existing output")
	}
	if out := mustRun(t, root, env, nil, "-c", work, "follow-up"); string(out) != "follow-up" {
		t.Fatal("lost follow-up")
	}
	if old, _ := os.ReadFile(p[3]); string(old) != goal {
		t.Fatal("overwrote original goal")
	}
	if out := mustRun(t, root, env, []byte("resume evidence"), "-resume", work); string(out) != "follow-up" {
		t.Fatal("resume changed goal")
	}
	b, _ = os.ReadFile(filepath.Join(work, "working", "mode.txt"))
	if string(b) != "resume\n" {
		t.Fatal("lost resume operation")
	}
	if code, _, _ := runApp(t, root, env, nil, "-resume", work, "new goal"); code != 2 {
		t.Fatal("resume accepted a new goal")
	}
	if code, _, _ := runApp(t, root, env, nil, "-c", filepath.Join(root, "missing"), "goal"); code == 0 {
		t.Fatal("follow created a run")
	}
	if out := mustRun(t, root, env, []byte("stdin goal\n"), "-w", filepath.Join(root, "piped")); string(out) != "stdin goal\n" {
		t.Fatal("changed stdin goal")
	}
	b, _ = os.ReadFile(filepath.Join(root, "piped", "working", "evidence.txt"))
	if len(b) != 0 {
		t.Fatal("goal forwarded twice")
	}
	for _, code := range []int{0, 1, 2, 3, 75, 125, 130} {
		actual, out, errout := runApp(t, root, env, nil, "-w", filepath.Join(root, fmt.Sprintf("exit-%d", code)), fmt.Sprintf("status-%d", code))
		if actual != code || !bytes.Equal(out, []byte("\x00\xffartifact")) || !bytes.Contains(errout, []byte("child diagnostic\n")) {
			t.Fatalf("status/stream mismatch: %d %d %q %s", code, actual, out, errout)
		}
	}
}

func TestAppRejectsChangedCacheAndRequest(t *testing.T) {
	root, env := fixture(t)
	work := filepath.Join(root, "output")
	mustRun(t, root, env, nil, "-w", work, "first")
	p := paths(t, work)
	put(p[3], []byte("tampered"), 0600)
	if code, _, _ := runApp(t, root, env, nil, "-resume", work); code != 125 {
		t.Fatal("accepted changed retained goal")
	}
	if code, _, _ := runApp(t, root, env, nil, "-c", work, "next"); code != 125 {
		t.Fatal("accepted follow-up over changed retained goal")
	}
	put(filepath.Join(p[0], "bin/entry"), []byte("#!/bin/sh\necho tampered\n"), 0755)
	if code, _, _ := runApp(t, root, env, nil, "-w", filepath.Join(root, "next"), "goal"); code != 125 {
		t.Fatal("executed modified cache")
	}
	if _, err := os.Stat(filepath.Join(root, "next")); !os.IsNotExist(err) {
		t.Fatal("made output before cache validation")
	}
}

func TestAppOutputContainmentAndInfo(t *testing.T) {
	root, env := fixture(t)
	var m manifest
	if err := json.Unmarshal(mustRun(t, root, env, nil, "-info"), &m); err != nil {
		t.Fatal(err)
	}
	if m.App.Name != "fixture" {
		t.Fatal("wrong embedded manifest")
	}
	if len(m.LauncherSHA256) != 64 {
		t.Fatal("launcher source is not pinned")
	}
	if _, err := os.Stat(filepath.Join(root, "state")); !os.IsNotExist(err) {
		t.Fatal("info executed extraction")
	}
	work := filepath.Join(root, "output")
	mustRun(t, root, env, nil, "-w", work, "first")
	p := paths(t, work)
	bad := filepath.Join(p[0], "bad-output")
	if code, _, _ := runApp(t, root, env, nil, "-w", bad, "goal"); code == 0 {
		t.Fatal("output overlaps package")
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatal("invalid output poisoned package")
	}
	mustRun(t, root, env, nil, "-resume", work)
	mustRun(t, root, env, nil, "automatic output")
	auto, _ := filepath.Glob(filepath.Join(root, "fixture-*"))
	if len(auto) != 1 {
		t.Fatal("default output not retained")
	}
}

func TestAppRelativePATHAndRestrictiveUmask(t *testing.T) {
	root, env := fixture(t)
	env[0] = "PATH=.:relative:/usr/bin:/bin"
	work := filepath.Join(root, "output")
	// Set the umask in a child shell, never mutate the test process's umask.
	cmd := exec.Command("/bin/sh", "-c", `umask 077; exec "$@"`, "sh", testApp, "-w", work, "first")
	cmd.Env, cmd.Dir = env, root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restrictive umask: %v %s", err, b)
	}
	put(filepath.Join(work, "working", "cat"), []byte("#!/bin/sh\ntouch hijacked\nexit 42\n"), 0755)
	mustRun(t, root, env, nil, "-c", work, "next")
	if _, err := os.Stat(filepath.Join(work, "working", "hijacked")); !os.IsNotExist(err) {
		t.Fatal("relative PATH selected worker-created command")
	}
}

func TestAppConcurrentRunAndKilledController(t *testing.T) {
	for _, kill := range []bool{false, true} {
		t.Run(fmt.Sprintf("kill=%v", kill), func(t *testing.T) {
			root, env := fixture(t)
			work := filepath.Join(root, "output")
			cmd := exec.Command(testApp, "-w", work, "block")
			cmd.Dir, cmd.Env = root, env
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Kill()
			var pid int
			deadline := time.Now().Add(8 * time.Second)
			for time.Now().Before(deadline) {
				b, err := os.ReadFile(filepath.Join(work, "working", "adapter.pid"))
				if err == nil {
					pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
					if pid > 0 {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid == 0 {
				t.Fatal("adapter did not start")
			}
			defer syscall.Kill(-pid, syscall.SIGKILL)
			if code, _, _ := runApp(t, root, env, nil, "-c", work, "next"); code != 75 {
				t.Fatalf("concurrent launch not refused: %d", code)
			}
			if kill {
				cmd.Process.Kill()
				cmd.Wait()
				if code, _, errout := runApp(t, root, env, nil, "-c", work, "next"); code != 125 || !bytes.Contains(errout, []byte("no confirmed end")) {
					t.Fatalf("crash fence missing: %d %s", code, errout)
				}
			} else {
				cmd.Process.Signal(syscall.SIGTERM)
				if err := cmd.Wait(); err == nil || cmd.ProcessState.ExitCode() != 143 {
					t.Fatalf("signal outcome: %v %v", err, cmd.ProcessState)
				}
				if err := syscall.Kill(pid, 0); err == nil {
					t.Fatal("adapter survived cancellation")
				}
				mustRun(t, root, env, nil, "-c", work, "after interruption")
			}
		})
	}
}

func TestBuildRefusals(t *testing.T) {
	for _, mutation := range []string{"traversal", "source-link", "receipt-change", "extra-file", "unsupported-binary", "output-exists", "missing-entry"} {
		t.Run(mutation, func(t *testing.T) {
			root, _ := filepath.EvalSymlinks(t.TempDir())
			app, prefix := fixtureSource(root)
			output := filepath.Join(root, "new-app")
			switch mutation {
			case "traversal", "missing-entry":
				var a application
				b, _ := os.ReadFile(filepath.Join(app, "app.json"))
				json.Unmarshal(b, &a)
				if mutation == "traversal" {
					a.Files = []string{"../outside"}
				} else {
					a.Entry = "bin/missing"
				}
				b, _ = json.Marshal(a)
				put(filepath.Join(app, "app.json"), b, 0644)
			case "source-link":
				os.Rename(filepath.Join(app, "bin/entry"), filepath.Join(app, "real"))
				os.Symlink("../real", filepath.Join(app, "bin/entry"))
			case "receipt-change":
				put(filepath.Join(prefix, "lib/bench-tools/helper/bin/helper"), []byte("changed"), 0755)
			case "extra-file":
				put(filepath.Join(prefix, "lib/bench-tools/helper/private"), []byte("extra"), 0600)
			case "unsupported-binary":
				dir := filepath.Join(prefix, "lib/bench-tools/helper")
				var r map[string]any
				b, _ := os.ReadFile(filepath.Join(dir, "package.json"))
				json.Unmarshal(b, &r)
				r["files"].([]any)[0].(map[string]any)["sha256"] = digest([]byte("not a native executable"))
				b, _ = json.Marshal(r)
				put(filepath.Join(dir, "package.json"), b, 0644)
				put(filepath.Join(dir, "bin/helper"), []byte("not a native executable"), 0755)
			case "output-exists":
				put(output, []byte("preserve me"), 0600)
			}
			if err := build([]string{"-o", output, "-runtime", prefix, app}); err == nil {
				t.Fatal("accepted invalid build")
			}
			if mutation == "output-exists" {
				b, _ := os.ReadFile(output)
				if string(b) != "preserve me" {
					t.Fatal("overwrote existing output")
				}
			}
		})
	}
}

func TestArchiveTraversalAndLinkRefusals(t *testing.T) {
	for _, r := range []fileRecord{{Path: "../outside", Mode: 0755}, {Path: "runtime/bin/helper", Mode: 0777, Link: "/bin/sh"}, {Path: "runtime/bin/helper", Mode: 0777, Link: "../../../outside"}} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		m := manifest{Schema: 1, App: application{Schema: 1, Name: "bad", Entry: "bin/entry", Files: []string{"bin/entry"}}, Files: []fileRecord{r}}
		data, _ := json.Marshal(m)
		w, _ := z.Create("manifest.json")
		w.Write(data)
		w, _ = z.Create(r.Path)
		w.Write([]byte(r.Link))
		z.Close()
		if _, err := openArchive(b.Bytes()); err == nil {
			t.Fatalf("accepted unsafe archive: %+v", r)
		}
	}
}
