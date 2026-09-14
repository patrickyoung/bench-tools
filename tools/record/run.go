package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func runCLI(args []string) int {
	f := flags("run")
	file := f.String("f", "", "new recording session")
	ask := f.String("ask", "ask", "Ask executable")
	timeout := f.Duration("timeout", 0, "execution limit")
	grace := f.Duration("grace", time.Second, "signal cleanup grace before killing the child group")
	var inputs, outputs, sessions, passFDs, labels stringsFlag
	f.Var(&inputs, "input", "snapshot input file")
	f.Var(&outputs, "output", "snapshot output file")
	f.Var(&sessions, "session", "snapshot child Ask session")
	f.Var(&passFDs, "pass-fd", "private inherited descriptor")
	f.Var(&labels, "label", "non-secret context")
	if err := parse(f, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 125
	}
	if *file == "" || f.NArg() == 0 || *timeout < 0 || *grace < 0 || strings.ContainsAny(*file, "\x00\r\n") {
		return fail(errors.New("run requires -f FILE and a literal command; timeout must be nonnegative"))
	}
	abs, err := filepath.Abs(*file)
	if err != nil {
		return fail(err)
	}
	askPath, err := exec.LookPath(*ask)
	if err != nil {
		return fail(err)
	}
	askPath, err = filepath.Abs(askPath)
	if err != nil {
		return fail(err)
	}
	// Older Ask versions treat unrecognized words as model prompts. Probe a
	// universally supported read-only verb before attempting the new init verb.
	var askHelp bytes.Buffer
	if err := askCommand(askPath, []string{"help"}, nil, &askHelp); err != nil {
		return fail(err)
	}
	if !strings.Contains(askHelp.String(), "ask init ") || !strings.Contains(askHelp.String(), "  -jsonl ") {
		return fail(errors.New("selected Ask lacks model-free initialization or streaming notes; use Ask 0.3 or later"))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fail(err)
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return fail(err)
	}
	in := intent{Version: 1, Cwd: []byte(cwd), Labels: labels, TimeoutNS: int64(*timeout), GraceNS: int64(*grace)}
	for _, arg := range f.Args() {
		in.Argv = append(in.Argv, []byte(arg))
	}
	var extras []*os.File
	seen := map[int]bool{}
	for _, value := range passFDs {
		n, e := strconv.Atoi(value)
		if e != nil || n < 3 || n > 1024 || seen[n] {
			return fail(fmt.Errorf("invalid or repeated private descriptor %q", value))
		}
		seen[n] = true
		fd := os.NewFile(uintptr(n), "private descriptor")
		if _, e := fd.Stat(); e != nil {
			return fail(e)
		}
		for len(extras) <= n-3 {
			extras = append(extras, nil)
		}
		extras[n-3] = fd
		in.PrivateFDs = append(in.PrivateFDs, n)
	}
	for _, group := range []struct {
		phase string
		paths []string
	}{{"input", inputs}, {"output", outputs}, {"session", sessions}} {
		for _, path := range group.paths {
			p, e := filepath.Abs(path)
			if e != nil {
				return fail(e)
			}
			if p == abs {
				return fail(errors.New("a recording session cannot snapshot itself"))
			}
			in.Artifacts = append(in.Artifacts, artifact{fmt.Sprintf("artifact:%d", len(in.Artifacts)), []byte(p), group.phase})
		}
	}
	selected, selectionErr := exec.LookPath(f.Arg(0))
	if selectionErr == nil {
		selected, selectionErr = filepath.Abs(selected)
		if selectionErr == nil {
			in.Executable = []byte(selected)
			in.ExecutableSHA256, selectionErr = fingerprint(selected)
		}
	}
	// Only Ask creates and writes the log. Existing destinations fail before
	// any process is started; no current pointer or model is involved.
	if err := askCommand(askPath, []string{"init", "-f", abs}, nil, io.Discard); err != nil {
		return fail(err)
	}
	r := &recorder{ask: askPath, file: abs, streams: map[string]*streamDigest{}}
	if err := r.startNotes(); err != nil {
		return fail(err)
	}
	defer r.closeNotes()
	for _, name := range []string{"stdin", "stdout", "stderr"} {
		r.streams[name] = newDigest()
	}
	for _, a := range in.Artifacts {
		r.streams[a.Name] = newDigest()
	}
	if err := r.note("intent", in); err != nil {
		return fail(err)
	}
	for _, a := range in.Artifacts {
		if a.Phase == "input" {
			if err := r.snapshot(a); err != nil {
				return fail(err)
			}
		}
	}
	outcome := terminal{Complete: true}
	if selectionErr != nil {
		outcome.Exit, outcome.StartError = 127, selectionErr.Error()
	} else {
		outcome = r.execute(selected, f.Args(), cwd, extras, *timeout, *grace)
	}
	for _, a := range in.Artifacts {
		if a.Phase != "input" {
			if err := r.snapshot(a); err != nil {
				outcome.Complete, outcome.Problem = false, err.Error()
				break
			}
		}
	}
	r.mu.Lock()
	outcome.Streams = map[string]commitment{}
	for name, stream := range r.streams {
		outcome.Streams[name] = stream.commit()
	}
	captureErr := r.err
	r.mu.Unlock()
	if captureErr != nil {
		return fail(fmt.Errorf("%w: %v", errIncomplete, captureErr))
	}
	if err := r.note("terminal", outcome); err != nil {
		return fail(fmt.Errorf("%w: %v", errIncomplete, err))
	}
	if err := r.closeNotes(); err != nil {
		return fail(err)
	}
	if !outcome.Complete {
		return fail(fmt.Errorf("%w: %s", errIncomplete, outcome.Problem))
	}
	if outcome.StartError != "" {
		fmt.Fprintln(os.Stderr, "record:", outcome.StartError)
	}
	return outcome.Exit
}

func fingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("executable is not a regular file: %s", path)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (r *recorder) snapshot(a artifact) error {
	f, err := os.Open(string(a.Path))
	if err != nil {
		return err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return err
	}
	if !before.Mode().IsRegular() {
		return fmt.Errorf("snapshot requires a regular file: %s", a.Path)
	}
	logInfo, err := os.Stat(r.file)
	if err != nil {
		return err
	}
	if os.SameFile(before, logInfo) {
		return errors.New("a recording session cannot snapshot itself through an alias")
	}
	reader := io.Reader(io.LimitReader(f, before.Size()+1))
	if a.Phase == "session" {
		// Verify a private copy, then retain those same bytes. Checking the live
		// file and reopening it would introduce a check/use race.
		tmp, err := os.CreateTemp("", "record-child-*.jsonl")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()
		if _, err := io.Copy(tmp, reader); err != nil {
			return err
		}
		if err := tmp.Sync(); err != nil {
			return err
		}
		if err := askCommand(r.ask, []string{"replay", "-check", tmp.Name()}, nil, io.Discard); err != nil {
			return err
		}
		if _, err := tmp.Seek(0, 0); err != nil {
			return err
		}
		reader = tmp
	}
	buf := make([]byte, chunkSize)
	if _, err := io.CopyBuffer(captureWriter{r, a.Name, io.Discard}, reader, buf); err != nil {
		return err
	}
	after, err := f.Stat()
	if err != nil {
		return err
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("snapshot changed while reading: %s", a.Path)
	}
	return nil
}

func (r *recorder) execute(selected string, argv []string, cwd string, extras []*os.File, timeout, grace time.Duration) terminal {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if timeout > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, timeout)
		defer stop()
	}
	cmd := exec.CommandContext(ctx, selected, argv[1:]...)
	cmd.Args[0] = argv[0]
	cmd.Dir, cmd.ExtraFiles = cwd, extras
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	// Own the output pipes so a nonzero child exit cannot hide a failed drain
	// behind exec.ExitError (Cmd.Wait gives the process error precedence).
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return terminal{Exit: 126, Complete: true, StartError: err.Error()}
	}
	defer stdoutR.Close()
	defer stdoutW.Close()
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return terminal{Exit: 126, Complete: true, StartError: err.Error()}
	}
	defer stderrR.Close()
	defer stderrW.Close()
	cmd.Stdout, cmd.Stderr = stdoutW, stderrW
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return terminal{Exit: 126, Complete: true, StartError: err.Error()}
	}
	defer stdinR.Close()
	defer stdinW.Close()
	cmd.Stdin = stdinR
	r.kill = cancel
	if err := cmd.Start(); err != nil {
		return terminal{Exit: 126, Complete: true, StartError: err.Error()}
	}
	stdinR.Close()
	stdoutW.Close()
	stderrW.Close()
	drained := make(chan error, 2)
	go func() { _, err := io.Copy(captureWriter{r, "stdout", os.Stdout}, stdoutR); drained <- err }()
	go func() { _, err := io.Copy(captureWriter{r, "stderr", os.Stderr}, stderrR); drained <- err }()
	var interrupted atomic.Bool
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	done := make(chan struct{})
	signalDone := make(chan struct{})
	go func() {
		defer close(signalDone)
		for {
			select {
			case s := <-signals:
				interrupted.Store(true)
				_ = syscall.Kill(-cmd.Process.Pid, s.(syscall.Signal))
				// Finish group cleanup even when its leader exits first. A
				// supervisor may kill this recorder after its own grace period.
				deadline := time.NewTimer(grace)
				poll := time.NewTicker(10 * time.Millisecond)
				defer deadline.Stop()
				defer poll.Stop()
				for {
					select {
					case <-poll.C:
						if errors.Is(syscall.Kill(-cmd.Process.Pid, 0), syscall.ESRCH) {
							return
						}
					case <-deadline.C:
						_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
						return
					}
				}
			case <-done:
				return
			}
		}
	}()
	// A blocked reader must not keep a command alive after that command has
	// closed stdin. Hold inputMu only around capture/write, never while reading
	// the caller's pipe. Finalization excludes every later input observation.
	var inputMu sync.Mutex
	var delivered int64
	var eof bool
	var inputErr error
	go func() {
		defer stdinW.Close()
		buf := make([]byte, 32<<10)
		for {
			n, readErr := os.Stdin.Read(buf)
			inputMu.Lock()
			select {
			case <-done:
				inputMu.Unlock()
				return
			default:
			}
			if n > 0 {
				if err := r.capture("stdin", buf[:n]); err != nil {
					inputErr = err
					inputMu.Unlock()
					return
				}
				written, err := stdinW.Write(buf[:n])
				delivered += int64(written)
				if err != nil {
					inputMu.Unlock()
					return
				} // child closed its pipe; unread input is not execution evidence
			}
			if readErr != nil {
				eof = errors.Is(readErr, io.EOF)
				if !eof {
					inputErr = readErr
					cancel()
				}
				inputMu.Unlock()
				return
			}
			inputMu.Unlock()
		}
	}()
	err = cmd.Wait()
	close(done)
	<-signalDone
	stdinW.Close()
	inputMu.Lock()
	outcome := terminal{Started: true, Complete: true, StdinEOF: eof, StdinDelivered: delivered, Interrupted: interrupted.Load() || ctx.Err() != nil}
	if inputErr != nil {
		outcome.Complete, outcome.Problem = false, inputErr.Error()
	}
	inputMu.Unlock()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for remaining := 2; remaining > 0; {
		select {
		case drainErr := <-drained:
			remaining--
			if drainErr != nil {
				outcome.Complete = false
				if outcome.Problem == "" {
					outcome.Problem = drainErr.Error()
				}
			}
		case <-deadline.C:
			outcome.Complete = false
			outcome.Problem = "output pipes remained open after the two-second drain bound"
			stdoutR.Close()
			stderrR.Close()
		}
	}
	if cmd.ProcessState != nil {
		outcome.Exit = cmd.ProcessState.ExitCode()
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			outcome.Signal = int(ws.Signal())
			outcome.Exit = 128 + outcome.Signal
		}
	}
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		outcome.Complete, outcome.Problem = false, err.Error()
	}
	return outcome
}
