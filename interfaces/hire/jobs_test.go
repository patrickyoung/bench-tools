package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// This helper executes the test binary as a real child without shells, model
// credentials, network requests, or any installed Bench command.
func TestJobProcess(t *testing.T) {
	separator := -1
	for i, arg := range os.Args {
		if arg == "--job-helper" {
			separator = i
			break
		}
	}
	if separator < 0 {
		return
	}
	args := os.Args[separator+1:]
	switch args[0] {
	case "echo":
		input, _ := io.ReadAll(os.Stdin)
		fmt.Printf("args=%s\ninput=%s", strings.Join(args[2:], "|"), input)
		fmt.Fprint(os.Stderr, "progress only\n")
		code, _ := strconv.Atoi(args[1])
		os.Exit(code)
	case "interrupt":
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)
		fmt.Print("ready\n")
		<-signals
		fmt.Fprint(os.Stderr, "cancelled cleanly\n")
		os.Exit(130)
	case "ignore":
		signal.Ignore(os.Interrupt)
		fmt.Print("ready\n")
		time.Sleep(time.Minute)
	case "overflow":
		fmt.Print(strings.Repeat("x", jobLogLimit+4096))
		fmt.Fprint(os.Stderr, "still working\n")
		os.Exit(0)
	case "signal":
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		time.Sleep(time.Second)
	}
	os.Exit(99)
}

func helperArgs(mode string, rest ...string) []string {
	return append([]string{os.Args[0], "-test.run=^TestJobProcess$", "--", "--job-helper", mode}, rest...)
}

func newTestJobs(t *testing.T) (*jobManager, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "private")
	manager, err := newJobManager(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	return manager, root
}

func awaitJob(t *testing.T, manager *jobManager, id string) Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := manager.Get(id)
		if !ok {
			t.Fatal("job disappeared")
		}
		if !job.Active() {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("command did not finish")
	return Job{}
}

func awaitReady(t *testing.T, manager *jobManager, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(manager.Log(id, "stdout"), "ready") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("helper was not ready")
}

func TestJobsExactOutcomesAndLiteralArguments(t *testing.T) {
	for _, test := range []struct {
		code  int
		state string
	}{{0, "completed"}, {2, "unfinished"}, {7, "failed"}, {130, "cancelled"}} {
		t.Run(strconv.Itoa(test.code), func(t *testing.T) {
			m, root := newTestJobs(t)
			literal := "$(touch must-not-exist); 'quoted'"
			argv := helperArgs("echo", strconv.Itoa(test.code), literal)
			job, err := m.Start("build", "Example", root, argv, "private goal")
			if err != nil {
				t.Fatal(err)
			}
			argv[0] = "mutated"
			job = awaitJob(t, m, job.ID)
			if job.State != test.state || job.ExitCode == nil || *job.ExitCode != test.code {
				t.Fatalf("unexpected outcome: %+v", job)
			}
			if job.Args[0] == "mutated" {
				t.Fatal("job aliases caller argv")
			}
			if got := m.Log(job.ID, "stdout"); got != "args="+literal+"\ninput=private goal" {
				t.Fatalf("stdout: %q", got)
			}
			if got := m.Log(job.ID, "stderr"); got != "progress only\n" {
				t.Fatalf("stderr: %q", got)
			}
			data, err := os.ReadFile(filepath.Join(root, "jobs", job.ID, "job.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "private goal") {
				t.Fatal("stdin leaked into job metadata")
			}
			for _, name := range []string{"stdout", "stderr", "job.json"} {
				info, err := os.Stat(filepath.Join(root, "jobs", job.ID, name))
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0600 {
					t.Fatalf("%s permissions: %v", name, info.Mode())
				}
			}
			copy, _ := m.Get(job.ID)
			copy.Args[0] = "another mutation"
			*copy.ExitCode = 999
			again, _ := m.Get(job.ID)
			if again.Args[0] == "another mutation" || *again.ExitCode == 999 {
				t.Fatal("returned job aliases internal state")
			}
		})
	}
}

func TestJobsCancellationAndSingleActiveCommand(t *testing.T) {
	m, root := newTestJobs(t)
	job, err := m.Start("build", "Cancellable", root, helperArgs("interrupt"), "")
	if err != nil {
		t.Fatal(err)
	}
	awaitReady(t, m, job.ID)
	if _, err := m.Start("build", "Second", root, helperArgs("echo", "0"), ""); err == nil {
		t.Fatal("accepted concurrent command")
	}
	if err := m.Cancel("../../escape"); err == nil {
		t.Fatal("accepted invalid ID")
	}
	if err := m.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	job = awaitJob(t, m, job.ID)
	if job.State != "cancelled" || job.ExitCode == nil || *job.ExitCode != 130 {
		t.Fatalf("unexpected cancellation: %+v", job)
	}
	if !strings.Contains(m.Log(job.ID, "stderr"), "cancelled cleanly") {
		t.Fatal("child did not receive graceful cancellation")
	}
}

func TestJobsForcedCancellationIsUnknown(t *testing.T) {
	m, root := newTestJobs(t)
	m.grace = 50 * time.Millisecond
	job, err := m.Start("build", "Uncooperative", root, helperArgs("ignore"), "")
	if err != nil {
		t.Fatal(err)
	}
	awaitReady(t, m, job.ID)
	if err := m.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	job = awaitJob(t, m, job.ID)
	if job.State != "unknown" || job.ExitCode == nil || *job.ExitCode != 137 {
		t.Fatalf("unexpected forced outcome: %+v", job)
	}
}

func TestJobsSignalOutcome(t *testing.T) {
	m, root := newTestJobs(t)
	job, err := m.Start("build", "Signal", root, helperArgs("signal"), "")
	if err != nil {
		t.Fatal(err)
	}
	job = awaitJob(t, m, job.ID)
	if job.ExitCode == nil || *job.ExitCode != 143 {
		t.Fatalf("lost signal outcome: %+v", job)
	}
}

func TestJobsLogsBoundedAndSafe(t *testing.T) {
	m, root := newTestJobs(t)
	job, err := m.Start("build", "Verbose", root, helperArgs("overflow"), "")
	if err != nil {
		t.Fatal(err)
	}
	job = awaitJob(t, m, job.ID)
	if job.State != "completed" {
		t.Fatalf("log cap changed process outcome: %+v", job)
	}
	info, err := os.Stat(filepath.Join(root, "jobs", job.ID, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > jobLogLimit+100 {
		t.Fatalf("unbounded log: %d", info.Size())
	}
	log := m.Log(job.ID, "stdout")
	if len(log) > jobLogTail+100 || !strings.Contains(log, "further output discarded") || !strings.Contains(log, "Earlier output omitted") {
		t.Fatalf("missing bounded-tail markers: length=%d", len(log))
	}
	if m.Log(job.ID, "../../secret") != "" || m.Log("../escape", "stdout") != "" {
		t.Fatal("accepted path traversal")
	}
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("outside secret"), 0600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "jobs", job.ID, "stdout")
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, logPath); err != nil {
		t.Fatal(err)
	}
	if m.Log(job.ID, "stdout") != "" {
		t.Fatal("read symlinked log")
	}
}

func TestJobsRecoveryDoesNotRestart(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	id := strings.Repeat("a", 32)
	if err := os.MkdirAll(filepath.Join(root, "jobs", id), 0700); err != nil {
		t.Fatal(err)
	}
	prior := Job{ID: id, State: "running", Args: []string{"must-not-execute"}, Started: time.Now()}
	data, _ := json.Marshal(prior)
	if err := os.WriteFile(filepath.Join(root, "jobs", id, "job.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := newJobManager(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	job, ok := m.Get(id)
	if !ok || job.State != "unknown" || job.ExitCode != nil || job.Finished.IsZero() {
		t.Fatalf("unsafe recovery: %+v", job)
	}
	if m.active != nil {
		t.Fatal("restarted recovered command")
	}
}

func TestJobsShutdownAndStartFailure(t *testing.T) {
	m, root := newTestJobs(t)
	failed, err := m.Start("build", "Missing", root, []string{filepath.Join(root, "missing-command")}, "")
	if err == nil || failed.State != "failed" || failed.ExitCode != nil {
		t.Fatalf("start error was hidden: %+v %v", failed, err)
	}
	job, err := m.Start("build", "Shutdown", root, helperArgs("interrupt"), "")
	if err != nil {
		t.Fatal(err)
	}
	awaitReady(t, m, job.ID)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	done, _ := m.Get(job.ID)
	if done.Active() || done.ExitCode == nil || *done.ExitCode != 130 {
		t.Fatalf("shutdown left active child: %+v", done)
	}
	if _, err := m.Start("build", "After close", root, helperArgs("echo", "0"), ""); err == nil {
		t.Fatal("started after shutdown")
	}
}

func TestJobsExclusiveDataRoot(t *testing.T) {
	first, root := newTestJobs(t)
	second, err := newJobManager(root)
	if err == nil {
		second.Close()
		t.Fatal("second manager acquired an active data root")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("wrong conflict error: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := newJobManager(root)
	if err != nil {
		t.Fatalf("could not reopen released data root: %v", err)
	}
	defer reopened.Close()
}

func TestJobsRecoveryRejectsUnsafeStorage(t *testing.T) {
	for _, kind := range []string{"directory", "log", "symlink", "metadata"} {
		t.Run(kind, func(t *testing.T) {
			m, root := newTestJobs(t)
			job, err := m.Start("build", "Retained", root, helperArgs("echo", "0"), "")
			if err != nil {
				t.Fatal(err)
			}
			awaitJob(t, m, job.ID)
			if err := m.Close(); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "jobs", job.ID)
			switch kind {
			case "directory":
				err = os.Chmod(dir, 0755)
			case "log":
				err = os.Chmod(filepath.Join(dir, "stdout"), 0644)
			case "metadata":
				err = os.Chmod(filepath.Join(dir, "job.json"), 0644)
			case "symlink":
				path := filepath.Join(dir, "stderr")
				if err = os.Remove(path); err == nil {
					err = os.Symlink("stdout", path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := newJobManager(root)
			if err == nil {
				reopened.Close()
				t.Fatal("accepted unsafe retained storage")
			}
		})
	}
}
