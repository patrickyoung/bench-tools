//go:build unix

package main

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func relaySignals(process *os.Process) func() {
	ch := make(chan os.Signal, 4)
	done := make(chan struct{})
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		for {
			select {
			case sig := <-ch:
				_ = process.Signal(sig)
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}

func outcomeFromWait(err error) outcome {
	if err == nil {
		return outcome{code: exitOK}
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return outcome{code: exitSetup}
	}
	status, ok := ee.Sys().(syscall.WaitStatus)
	if !ok {
		return outcome{code: ee.ExitCode()}
	}
	if status.Signaled() {
		return outcome{code: 128 + int(status.Signal()), signal: status.Signal()}
	}
	return outcome{code: status.ExitStatus()}
}

func reraise(sig os.Signal) {
	signal.Reset(sig)
	if unix, ok := sig.(syscall.Signal); ok {
		_ = syscall.Kill(os.Getpid(), unix)
	}
}
