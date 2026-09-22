package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"time"
)

type Call struct {
	Directory string   `json:"directory"`
	Argv      []string `json:"argv"`
	Exit      int      `json:"exit"`
	Complete  bool     `json:"complete"`
	Cost      *float64 `json:"cost"`
}

func process(ctx context.Context, argv []string, cwd string, in io.Reader, out, errout io.Writer) (int, bool, error) {
	if e := ctx.Err(); e != nil {
		return -1, true, e
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = errout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 3 * time.Second
	if e := cmd.Start(); e != nil {
		return -1, false, e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var e error
	select {
	case e = <-done:
		if ctx.Err() != nil {
			// A stream-copy failure and cancellation can become ready together.
			// Do not leave descendants alive when Wait wins that race.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			return -1, true, ctx.Err()
		}
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		timer := time.NewTimer(3 * time.Second)
		received := false
		select {
		case <-done:
			received = true
			<-timer.C
		case <-timer.C:
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if !received {
			<-done
		}
		return -1, true, ctx.Err()
	}
	if e == nil {
		return 0, false, nil
	}
	var exit *exec.ExitError
	if errors.As(e, &exit) {
		return exit.ExitCode(), false, nil
	}
	return -1, false, e
}
func replay(ctx context.Context, record, directory string, call Call) error {
	// Record owns decoding and integrity verification. Only its public summary
	// and extracted streams are consumed here, never its Ask archive format.
	argv := []string{record, "replay", "-f", filepath.Join(directory, "process.jsonl"), "-json"}
	var out, diag limitedBuffer
	code, interrupted, e := process(ctx, argv, directory, nil, &out, &diag)
	if e != nil || interrupted || code != 0 {
		return fmt.Errorf("Record verification failed for %s", call.Directory)
	}
	var summary struct {
		Intent struct {
			Argv []string `json:"argv"`
		} `json:"intent"`
		Terminal struct {
			Exit        int  `json:"exit"`
			Complete    bool `json:"complete"`
			Started     bool `json:"started"`
			Interrupted bool `json:"interrupted"`
			Signal      int  `json:"signal"`
		} `json:"terminal"`
	}
	if e = json.Unmarshal(out.Bytes(), &summary); e != nil {
		return e
	}
	args := []string{}
	for _, v := range summary.Intent.Argv {
		b, e := base64.StdEncoding.DecodeString(v)
		if e != nil {
			return e
		}
		args = append(args, string(b))
	}
	if !summary.Terminal.Complete || !summary.Terminal.Started || summary.Terminal.Interrupted || summary.Terminal.Signal != 0 || summary.Terminal.Exit != call.Exit || !reflect.DeepEqual(args, call.Argv) {
		return fmt.Errorf("Record outcome or invocation mismatch")
	}
	for _, stream := range []string{"stdin", "stdout", "stderr"} {
		out.Reset()
		diag.Reset()
		a := []string{record, "replay", "-f", filepath.Join(directory, "process.jsonl"), "-stream", stream}
		code, interrupted, e = process(ctx, a, directory, nil, &out, &diag)
		if e != nil || interrupted || code != 0 {
			return fmt.Errorf("Record stream replay failed")
		}
		raw, e := readFile(filepath.Join(directory, stream), 64<<20)
		if e != nil {
			return e
		}
		if !bytes.Equal(out.Bytes(), raw) {
			return fmt.Errorf("Record %s mismatch", stream)
		}
	}
	return nil
}

// Bound output even if the selected replay executable is broken.
type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxJSON {
		return 0, fmt.Errorf("command output exceeds limit")
	}
	return b.Buffer.Write(p)
}

// Cancel the whole command group as soon as either stream crosses its bound.
// Keep the prefix for diagnosis; an interrupted call is never complete evidence.
// Each stream has one os/exec copy goroutine; process waits for it before reads.
type streamLimit struct {
	writer   io.Writer
	cancel   context.CancelFunc
	written  int
	exceeded bool
}

func (w *streamLimit) Write(p []byte) (int, error) {
	remaining := maxJSON - w.written
	if len(p) > remaining {
		w.exceeded = true
		w.cancel()
		n, e := w.writer.Write(p[:remaining])
		w.written += n
		if e != nil {
			return n, e
		}
		return n, fmt.Errorf("command stream exceeds limit")
	}
	n, e := w.writer.Write(p)
	w.written += n
	return n, e
}
