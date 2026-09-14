package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The root gate requires this test with a fresh independently built Ask.
// A standalone checkout can run it by selecting RECORD_TEST_ASK explicitly.
func TestAskRecordContract(t *testing.T) {
	ask := os.Getenv("RECORD_TEST_ASK")
	if ask == "" {
		t.Skip("set RECORD_TEST_ASK to a freshly built Ask >= 0.3")
	}
	ask, err := filepath.Abs(ask)
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	bin := filepath.Join(work, "record")
	buildArgs := []string{"build", "-o", bin, "."}
	if raceEnabled {
		buildArgs = []string{"build", "-race", "-o", bin, "."}
	}
	build := exec.Command("go", buildArgs...)
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, b)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + work, "TMPDIR=" + work, "RECORD_HELPER=1"}
	run := func(input []byte, code int, args ...string) ([]byte, []byte) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		cmd.Dir = work
		cmd.Stdin = bytes.NewReader(input)
		var out, diagnostic bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &diagnostic
		err := cmd.Run()
		got := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			got = ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		if got != code {
			t.Fatalf("%v: exit %d want %d: %s", args, got, code, diagnostic.Bytes())
		}
		return out.Bytes(), diagnostic.Bytes()
	}
	base := func(file string) []string { return []string{"run", "-ask", ask, "-f", file} }
	child := func(mode string) []string { return []string{"--", helper, "-test.run=^TestProcessHelper$", "--", mode} }
	t.Run("supervisor_group_interrupt_preserves_seal_and_kills_ignoring_child", func(t *testing.T) {
		file := filepath.Join(work, "group-interrupt.jsonl")
		cmd := exec.Command(bin, append(base(file), "-grace", "100ms", "--", "/bin/sh", "-c", "trap '' INT TERM; printf 'ready\\n'; while :; do sleep 60; done")...)
		cmd.Env, cmd.Dir = env, work
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		ready := make(chan error, 1)
		go func() { _, err := bufio.NewReader(stdout).ReadString('\n'); ready <- err }()
		select {
		case err := <-ready:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("recorded child never became ready")
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("ignored interrupt left execution running")
		}
		if cmd.ProcessState.ExitCode() != 137 {
			t.Fatalf("exit %d: %s", cmd.ProcessState.ExitCode(), &diagnostic)
		}
		meta, _ := run(nil, 0, "replay", "-ask", ask, "-f", file, "-json")
		var s summary
		if json.Unmarshal(meta, &s) != nil || !s.Terminal.Complete || !s.Terminal.Interrupted || s.Terminal.Signal != 9 {
			t.Fatalf("interrupted outcome was lost: %s", meta)
		}
	})
	t.Run("old_ask_cannot_turn_init_into_a_model_prompt", func(t *testing.T) {
		old := filepath.Join(work, "old-ask")
		marker := filepath.Join(work, "old-ask-called-model")
		program := "#!/bin/sh\nif [ \"$1\" = help ]; then printf 'usage: ask [prompt] | ask note | ask replay\\n'; exit 0; fi\nprintf unsafe > '" + strings.ReplaceAll(marker, "'", "'\\''") + "'\nexit 1\n"
		if err := os.WriteFile(old, []byte(program), 0700); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(work, "old-ask.jsonl")
		run(nil, 125, "run", "-ask", old, "-f", file, "--", "/bin/true")
		for _, path := range []string{file, marker} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("old Ask was called beyond its safe help boundary")
			}
		}
	})
	for _, code := range []int{0, 1, 2, 75, 125} {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			file := filepath.Join(work, fmt.Sprintf("exit-%d.jsonl", code))
			args := append(base(file), child(fmt.Sprintf("exit:%d", code))...)
			input := []byte{0, 255, '\n', 0xfe}
			out, diag := run(input, code, args...)
			if !bytes.Equal(out, input) || !bytes.Equal(diag, []byte{'e', 0, 255}) {
				t.Fatalf("live streams changed: %x %x", out, diag)
			}
			replayed, errout := run(nil, code, "replay", "-ask", ask, "-f", file)
			if !bytes.Equal(out, replayed) || !bytes.Equal(diag, errout) {
				t.Fatal("replay changed streams")
			}
			stdin, _ := run(nil, 0, "replay", "-ask", ask, "-f", file, "-stream", "stdin")
			if !bytes.Equal(stdin, input) {
				t.Fatal("stdin bytes changed")
			}
			meta, _ := run(nil, 0, "replay", "-ask", ask, "-f", file, "-json")
			var s summary
			if err := json.Unmarshal(meta, &s); err != nil {
				t.Fatal(err)
			}
			if s.Terminal.Exit != code || s.Terminal.StdinDelivered != int64(len(input)) || !s.Terminal.StdinEOF {
				t.Fatalf("bad terminal: %+v", s.Terminal)
			}
		})
	}
	t.Run("snapshots_and_no_reexecution", func(t *testing.T) {
		input := filepath.Join(work, "input.bin")
		output := filepath.Join(work, "output.bin")
		effect := filepath.Join(work, "effect")
		childSession := filepath.Join(work, "child.jsonl")
		file := filepath.Join(work, "files.jsonl")
		if err := os.WriteFile(input, []byte{0, 255, 1}, 0600); err != nil {
			t.Fatal(err)
		}
		if err := askCommand(ask, []string{"init", "-f", childSession}, nil, io.Discard); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(childSession)
		if err != nil {
			t.Fatal(err)
		}
		args := append(base(file), "-input", input, "-output", output, "-session", childSession)
		args = append(args, child("files")...)
		args = append(args, output, effect)
		run(nil, 0, args...)
		os.Remove(input)
		os.Remove(output)
		os.Remove(childSession)
		for i, want := range [][]byte{{0, 255, 1}, {0xfe, 0, 0xff}, before} {
			got, _ := run(nil, 0, "replay", "-ask", ask, "-f", file, "-stream", fmt.Sprintf("artifact:%d", i))
			if !bytes.Equal(got, want) {
				t.Fatalf("artifact %d changed", i)
			}
		}
		run(nil, 0, "replay", "-ask", ask, "-f", file)
		b, _ := os.ReadFile(effect)
		if string(b) != "once\n" {
			t.Fatal("replay executed an effect")
		}
		// Refusing an existing destination must precede the second effect.
		run(nil, 125, args...)
		b, _ = os.ReadFile(effect)
		if string(b) != "once\n" {
			t.Fatal("existing log allowed execution")
		}
	})
	t.Run("corrupt_and_incomplete", func(t *testing.T) {
		source := filepath.Join(work, "exit-0.jsonl")
		raw, _ := os.ReadFile(source)
		lines := bytes.Split(bytes.TrimSpace(raw), []byte{'\n'})
		for label, data := range map[string][]byte{
			"unsealed":         bytes.Join(lines[:len(lines)-1], []byte{'\n'}),
			"missing-terminal": bytes.Join(lines[:len(lines)-2], []byte{'\n'}),
			"changed":          bytes.Replace(raw, []byte(`"complete":true`), []byte(`"complete":false`), 1),
			"reordered":        bytes.Join(append(append(append([][]byte{}, lines[:4]...), lines[6:8]...), lines[4:6]...), []byte{'\n'}),
		} {
			file := filepath.Join(work, label+".jsonl")
			os.WriteFile(file, append(data, '\n'), 0600)
			out, _ := run(nil, 125, "replay", "-ask", ask, "-f", file)
			if len(out) != 0 {
				t.Fatal(label + " emitted unverified bytes")
			}
		}
	})
	t.Run("self_snapshot_alias_refuses_before_execution", func(t *testing.T) {
		file := filepath.Join(work, "self.jsonl")
		alias := filepath.Join(work, "alias.jsonl")
		effect := filepath.Join(work, "self.effect")
		if err := os.Symlink(file, alias); err != nil {
			t.Fatal(err)
		}
		args := append(base(file), "-input", alias)
		args = append(args, child("effect")...)
		args = append(args, effect)
		run(nil, 125, args...)
		if _, err := os.Stat(effect); !os.IsNotExist(err) {
			t.Fatal("self-snapshot allowed execution")
		}
	})
	t.Run("valid_seals_do_not_bless_bad_semantics", func(t *testing.T) {
		file := filepath.Join(work, "bad-offset.jsonl")
		if err := askCommand(ask, []string{"init", "-f", file}, nil, io.Discard); err != nil {
			t.Fatal(err)
		}
		r := &recorder{ask: ask, file: file}
		if err := r.note("intent", intent{Version: 1, Argv: [][]byte{[]byte("literal")}, Cwd: []byte(work)}); err != nil {
			t.Fatal(err)
		}
		if err := r.note("chunk", chunk{"stdout", 1, []byte("wrong")}); err != nil {
			t.Fatal(err)
		}
		if err := askCommand(ask, []string{"replay", "-check", file}, nil, io.Discard); err != nil {
			t.Fatal(err)
		}
		out, _ := run(nil, 125, "replay", "-ask", ask, "-f", file)
		if len(out) != 0 {
			t.Fatal("bad offset emitted output")
		}
	})
	t.Run("signal_and_startup", func(t *testing.T) {
		file := filepath.Join(work, "signal.jsonl")
		run(nil, 143, append(base(file), child("signal")...)...)
		run(nil, 143, "replay", "-ask", ask, "-f", file)
		file = filepath.Join(work, "missing.jsonl")
		run(nil, 127, append(base(file), "--", filepath.Join(work, "nonexistent"))...)
		run(nil, 127, "replay", "-ask", ask, "-f", file)
	})
	t.Run("literal_binary_argv_and_timeout", func(t *testing.T) {
		file := filepath.Join(work, "argv.jsonl")
		values := []string{" a b ", "$(false)", "; touch bad", string([]byte{255, 'x'}), "", "--literal"}
		args := append(base(file), child("argv")...)
		args = append(args, values...)
		out, _ := run(nil, 0, args...)
		var received [][]byte
		if err := json.Unmarshal(out, &received); err != nil {
			t.Fatal(err)
		}
		if len(received) != len(values) {
			t.Fatal("argv length changed")
		}
		for i, value := range values {
			if !bytes.Equal(received[i], []byte(value)) {
				t.Fatalf("argument %d changed", i)
			}
		}
		meta, _ := run(nil, 0, "replay", "-ask", ask, "-f", file, "-json")
		var receipt summary
		if err := json.Unmarshal(meta, &receipt); err != nil {
			t.Fatal(err)
		}
		for i, value := range values {
			if !bytes.Equal(receipt.Intent.Argv[len(receipt.Intent.Argv)-len(values)+i], []byte(value)) {
				t.Fatal("recorded argv changed")
			}
		}
		physical, _ := filepath.EvalSymlinks(work)
		if string(receipt.Intent.Cwd) != physical {
			t.Fatal("cwd is not physical")
		}
		file = filepath.Join(work, "timeout.jsonl")
		args = append(base(file), "-timeout", "100ms")
		args = append(args, child("sleep")...)
		run(nil, 137, args...)
		run(nil, 137, "replay", "-ask", ask, "-f", file)
		meta, _ = run(nil, 0, "replay", "-ask", ask, "-f", file, "-json")
		if err := json.Unmarshal(meta, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Intent.TimeoutNS != int64(100*time.Millisecond) || !receipt.Terminal.Interrupted || receipt.Terminal.Signal != 9 {
			t.Fatal("timeout lost its observed boundary")
		}
	})
	t.Run("streaming_protocol", func(t *testing.T) {
		file := filepath.Join(work, "protocol.jsonl")
		cmd := exec.Command(bin, append(base(file), child("protocol")...)...)
		cmd.Env = env
		cmd.Dir = work
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer cmd.Process.Kill()
		done := make(chan error, 1)
		go func() {
			if _, err := stdin.Write([]byte("ping")); err != nil {
				done <- err
				return
			}
			b := make([]byte, 4)
			_, err := io.ReadFull(stdout, b)
			if err == nil && string(b) != "pong" {
				err = fmt.Errorf("bad reply %q", b)
			}
			stdin.Close()
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("record waited for stdin EOF before protocol reply")
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("%v: %s", err, diagnostic.Bytes())
		}
		run(nil, 0, "check", "-ask", ask, "-f", file)
	})
	t.Run("private_descriptor", func(t *testing.T) {
		secret := []byte("fixture-header-not-for-the-log")
		fd, err := os.CreateTemp(work, "private-")
		if err != nil {
			t.Fatal(err)
		}
		defer fd.Close()
		fd.Write(secret)
		fd.Seek(0, 0)
		file := filepath.Join(work, "private.jsonl")
		args := append(base(file), "-pass-fd", "3", "-label", "resource=fixture")
		cmd := exec.Command(bin, append(args, child("fd")...)...)
		cmd.Env = env
		cmd.Dir = work
		cmd.ExtraFiles = []*os.File{fd}
		out, err := cmd.CombinedOutput()
		if err != nil || string(out) != "authenticated\n" {
			t.Fatalf("%v %s", err, out)
		}
		raw, _ := os.ReadFile(file)
		if bytes.Contains(raw, secret) {
			t.Fatal("private descriptor leaked")
		}
		run(nil, 0, "check", "-ask", ask, "-f", file)
	})
	t.Run("recording_failure_never_earns_success", func(t *testing.T) {
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
		for _, phase := range []string{"intent", "chunk", "terminal"} {
			proxy := filepath.Join(work, "ask-fails-"+phase)
			program := "#!/bin/sh\nexec " + quote(helper) + " -test.run='^TestProcessHelper$' -- ask-proxy " + quote(ask) + " " + quote(phase) + " \"$@\"\n"
			if err := os.WriteFile(proxy, []byte(program), 0700); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(work, "failed-"+phase+".jsonl")
			effect := filepath.Join(work, "failed-"+phase+".effect")
			args := append(base(file), "-ask", proxy)
			args = append(args, child("effect")...)
			args = append(args, effect)
			out, _ := run(nil, 125, args...)
			if phase != "terminal" && len(out) != 0 {
				t.Fatal("unrecorded output escaped")
			}
			b, _ := os.ReadFile(effect)
			if phase == "intent" && len(b) != 0 {
				t.Fatal("execution preceded intent seal")
			}
			if phase != "intent" && string(b) != "once\n" {
				t.Fatal("lost or repeated observed effect")
			}
			replayed, _ := run(nil, 125, "replay", "-ask", ask, "-f", file)
			if len(replayed) != 0 {
				t.Fatal("failed capture was replayed as complete")
			}
		}
	})
	t.Run("abrupt_recorder_death_is_incomplete", func(t *testing.T) {
		file := filepath.Join(work, "killed-recorder.jsonl")
		cmd := exec.Command(bin, append(base(file), child("hold")...)...)
		cmd.Env = env
		cmd.Dir = work
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		defer stdin.Close()
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer cmd.Process.Kill()
		done := make(chan error, 1)
		go func() {
			b := make([]byte, 6)
			_, err := io.ReadFull(stdout, b)
			if err == nil && string(b) != "ready\n" {
				err = fmt.Errorf("bad startup %q", b)
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("recorder did not expose sealed output")
		}
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		cmd.Wait()
		stdin.Close()
		if err := askCommand(ask, []string{"replay", "-check", file}, nil, io.Discard); err != nil {
			t.Fatal(err)
		}
		out, _ := run(nil, 125, "replay", "-ask", ask, "-f", file)
		if len(out) != 0 {
			t.Fatal("sealed prefix was treated as completed execution")
		}
	})
	t.Run("closed_output_pipe_is_recording_failure", func(t *testing.T) {
		file := filepath.Join(work, "closed-output.jsonl")
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		reader.Close()
		defer writer.Close()
		cmd := exec.Command(bin, append(base(file), child("large")...)...)
		cmd.Env = env
		cmd.Dir = work
		cmd.Stdout = writer
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		err = cmd.Run()
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != 125 {
			t.Fatalf("closed output: %v %s", err, diagnostic.Bytes())
		}
		out, _ := run(nil, 125, "replay", "-ask", ask, "-f", file)
		if len(out) != 0 {
			t.Fatal("closed output was treated as complete")
		}
	})
	t.Run("nonzero_exit_cannot_hide_incomplete_output", func(t *testing.T) {
		for _, code := range []string{"0", "1"} {
			file := filepath.Join(work, "lingering-"+code+".jsonl")
			args := append(base(file), child("lingering")...)
			args = append(args, code)
			out, _ := run(nil, 125, args...)
			if string(out) != "early\n" {
				t.Fatalf("unexpected retained prefix %q", out)
			}
			out, _ = run(nil, 125, "replay", "-ask", ask, "-f", file)
			if len(out) != 0 {
				t.Fatal("incomplete output replayed as complete")
			}
		}
	})
	t.Run("large_binary", func(t *testing.T) {
		file := filepath.Join(work, "large.jsonl")
		out, _ := run(nil, 0, append(base(file), child("large")...)...)
		if len(out) != (17 << 20) {
			t.Fatalf("truncated: %d", len(out))
		}
		replay, _ := run(nil, 0, "replay", "-ask", ask, "-f", file)
		if !bytes.Equal(out, replay) {
			t.Fatal("large binary output changed")
		}
	})
}

func TestProcessHelper(t *testing.T) {
	if os.Getenv("RECORD_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(98)
	}
	mode := args[1]
	switch {
	case strings.HasPrefix(mode, "exit:"):
		var code int
		fmt.Sscanf(mode, "exit:%d", &code)
		io.Copy(os.Stdout, os.Stdin)
		os.Stderr.Write([]byte{'e', 0, 255})
		os.Exit(code)
	case mode == "files":
		os.WriteFile(args[2], []byte{0xfe, 0, 0xff}, 0600)
		f, err := os.OpenFile(args[3], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(97)
		}
		f.WriteString("once\n")
		f.Close()
	case mode == "signal":
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
		time.Sleep(time.Second)
		os.Exit(96)
	case mode == "protocol":
		b := make([]byte, 4)
		if _, err := io.ReadFull(os.Stdin, b); err != nil || string(b) != "ping" {
			os.Exit(95)
		}
		os.Stdout.WriteString("pong")
		io.Copy(io.Discard, os.Stdin)
	case mode == "fd":
		b, err := io.ReadAll(os.NewFile(3, "private"))
		if err != nil || string(b) != "fixture-header-not-for-the-log" {
			os.Exit(94)
		}
		os.Stdout.WriteString("authenticated\n")
	case mode == "large":
		os.Stdout.Write(bytes.Repeat([]byte{0, 255, 1, 254}, (17<<20)/4))
	case mode == "argv":
		var values [][]byte
		for _, value := range args[2:] {
			values = append(values, []byte(value))
		}
		json.NewEncoder(os.Stdout).Encode(values)
	case mode == "sleep":
		time.Sleep(10 * time.Second)
	case mode == "hold":
		os.Stdout.WriteString("ready\n")
		io.Copy(io.Discard, os.Stdin)
	case mode == "effect":
		f, err := os.OpenFile(args[2], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(92)
		}
		f.WriteString("once\n")
		f.Close()
		os.Stdout.WriteString("effect complete\n")
	case mode == "lingering":
		child := exec.Command("/bin/sh", "-c", "sleep 5; printf late")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(85)
		}
		os.Stdout.WriteString("early\n")
		var code int
		fmt.Sscanf(args[2], "%d", &code)
		os.Exit(code)
	case mode == "ask-proxy":
		cmd := exec.Command(args[2], args[4:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if args[4] != "note" {
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err != nil {
				os.Exit(91)
			}
		} else {
			stdin, err := cmd.StdinPipe()
			if err != nil {
				os.Exit(90)
			}
			if err := cmd.Start(); err != nil {
				os.Exit(89)
			}
			s := bufio.NewScanner(os.Stdin)
			s.Buffer(make([]byte, 64<<10), 1<<20)
			for s.Scan() {
				var note struct {
					Kind string `json:"kind"`
				}
				json.Unmarshal(s.Bytes(), &note)
				if note.Kind == "record."+args[3]+"/v1" {
					stdin.Close()
					cmd.Wait()
					os.Exit(88)
				}
				if _, err := stdin.Write(append(append([]byte{}, s.Bytes()...), '\n')); err != nil {
					os.Exit(87)
				}
			}
			stdin.Close()
			if err := cmd.Wait(); err != nil {
				os.Exit(86)
			}
		}
	default:
		os.Exit(93)
	}
	os.Exit(0)
}

func TestStrictReceiptRejectsUnknownAndMultipleValues(t *testing.T) {
	for _, raw := range []string{`{"stream":"stdout","offset":0,"data":"AA==","truncated":true}`, `{} {}`} {
		var c chunk
		if strict([]byte(raw), &c) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
