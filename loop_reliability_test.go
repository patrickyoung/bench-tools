package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInterruptedObservationSurvivesWithoutAnotherModelTurn(t *testing.T) {
	for _, verifier := range []bool{false, true} {
		t.Run(strconv.FormatBool(verifier), func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "started")
			script := "touch " + shellQuote(marker) + "; sleep 30"
			reply := "```ply\n" + script + "\n```"
			if verifier {
				reply = "candidate report"
			}
			bin, dir := fakeAsk(t, reply)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := make(chan struct{})
			go func() {
				defer close(ready)
				for i := 0; i < 500; i++ {
					if _, err := os.Stat(marker); err == nil {
						cancel()
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
				cancel()
			}()
			runner := Runner{Dir: dir, Path: os.Getenv("PATH"), Shell: "/bin/sh", Timeout: 10 * time.Second, Cap: 1024}
			loop := Loop{Model: Model{Bin: bin, Session: filepath.Join(dir, "session")}, Runner: runner, Checker: runner, View: newView(io.Discard, true)}
			if verifier {
				loop.Check = script
			}
			answer, err := loop.Run(ctx, "goal")
			<-ready
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v, want cancellation", err)
			}
			if _, err := os.Stat(marker); err != nil {
				t.Fatal("fixture command never started")
			}
			if got := strings.TrimSpace(read(t, filepath.Join(dir, "n"))); got != "1" {
				t.Fatalf("model calls=%s, want exactly one", got)
			}
			input := read(t, filepath.Join(dir, "stdin.log"))
			argv := read(t, filepath.Join(dir, "argv.log"))
			if verifier {
				if !strings.Contains(argv, verifierReceiptKind) || !strings.Contains(input, `"interrupted":true`) || !strings.Contains(input, `"outcome":"broken"`) {
					t.Fatalf("interrupted verifier lost its sealed result:\n%s\n%s", argv, input)
				}
			} else if answer != "" || !strings.Contains(argv, "append -q -s ply") || !strings.Contains(input, "interrupted;") || !strings.Contains(input, "effects may exist") {
				t.Fatalf("interrupted action lost its observation or became an answer: %q\n%s\n%s", answer, argv, input)
			}
		})
	}
}

func TestFailedObservationStopsBeforeAnotherModelOrVerifier(t *testing.T) {
	work, _, askdir := sandbox(t, "```ply\nprintf actual-result\n```", "unchecked report")
	t.Setenv("FAKE_ASK_APPEND_EXIT", "1")
	check := filepath.Join(work, "checked")
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-B", "-check", "touch "+shellQuote(check), "goal")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "append observation") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if read(t, filepath.Join(askdir, "n")) != "1\n" {
		t.Fatal("model continued after failed recording")
	}
	if _, err := os.Stat(check); !os.IsNotExist(err) {
		t.Fatal("verifier ran after failed recording")
	}
}

func TestBinaryActionObservationIsExplicitAndReversible(t *testing.T) {
	work, _, askdir := sandbox(t, "```ply\nprintf '\\377'\n```", "finished")
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "goal")
	if code != 0 || stdout != "finished\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	input := read(t, filepath.Join(askdir, "stdin.log"))
	if !strings.Contains(input, "non-UTF-8") || !strings.Contains(input, `\xff`) || strings.Contains(input, "\ufffd") {
		t.Fatalf("binary evidence was silently replaced: %q", input)
	}
}

func TestSteeringDuringCheckDefersFinalization(t *testing.T) {
	work, _, askdir := sandbox(t, "old report", "updated report")
	steer := filepath.Join(t.TempDir(), "steer")
	write(t, steer, "", 0o600)
	marker := filepath.Join(work, "checked")
	check := "if [ ! -e " + shellQuote(marker) + " ]; then touch " + shellQuote(marker) + "; printf 'include parser detail\\n' >> " + shellQuote(steer) + "; fi"
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-B", "-steer", steer, "-check", check, "goal")
	if code != 0 || stdout != "updated report\n" || !strings.Contains(stderr, "deferred finalization") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := read(t, filepath.Join(askdir, "stdin.log")); strings.Count(got, "\ninclude parser detail\n") != 1 {
		t.Fatalf("guidance not delivered exactly once: %s", got)
	}
}

func TestSIGTERMStopsOwnedActionAndRecordsInterruption(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "action.pid")
	work, _, askdir := sandbox(t, "```ply\necho $$ > "+shellQuote(pidfile)+"; sleep 30\n```")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "-sh", "-C", work, "goal")
	cmd.Env = append(os.Environ(), "PLY_TEST_PROGRAM=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	var pid int
	for i := 0; i < 500; i++ {
		if body, err := os.ReadFile(pidfile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(body)))
			if pid > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("action did not start")
	}
	defer func() { _ = syscall.Kill(-pid, syscall.SIGKILL) }()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != 130 {
			t.Fatalf("termination=%v, want exit 130", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("SIGTERM did not complete cleanup")
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("owned action remains alive: %v", err)
	}
	if input := read(t, filepath.Join(askdir, "stdin.log")); !strings.Contains(input, "interrupted;") || !strings.Contains(input, "effects may exist") {
		t.Fatalf("SIGTERM observation missing: %s", input)
	}
}
