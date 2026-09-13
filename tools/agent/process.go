package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Leave process-group ownership with the caller and Ply. This process forwards
// direct signals, waits for the public child and removes its temporary input.
// It neither retries the operation nor interprets its answer as a verdict.
func execute(bin string, args, env []string, in io.Reader, out, stderr io.Writer, dir string) int {
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	cmd := exec.Command(bin, args...)
	cmd.Env, cmd.Dir = env, dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "agent: start %s: %v\n", bin, err)
		return 1
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case sig := <-signals:
			if sig == syscall.SIGHUP {
				sig = syscall.SIGTERM
			}
			_ = cmd.Process.Signal(sig)
		case err := <-done:
			if err == nil {
				return 0
			}
			var e *exec.ExitError
			if errors.As(err, &e) {
				if st, ok := e.Sys().(syscall.WaitStatus); ok && st.Signaled() {
					return 128 + int(st.Signal())
				}
				return e.ExitCode()
			}
			fmt.Fprintf(stderr, "agent: wait %s: %v\n", bin, err)
			return 1
		}
	}
}
