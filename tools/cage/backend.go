package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

const launcher = `printf x >&3 || exit 125
exec 3>&-
exec "$@"`

var setupTimeout = 5 * time.Second

type backend interface {
	status() statusRecord
	readyByte() byte
	command(policy, []string) (*exec.Cmd, func(), error)
}

type unavailableBackend struct {
	name        string
	detail      string
	alternative string
}

func (b unavailableBackend) status() statusRecord {
	return statusRecord{
		Backend: b.name, Available: false, Complete: false,
		Filesystem: "unavailable", Network: "unavailable",
		Detail: b.detail, Alternative: b.alternative,
	}
}

func (b unavailableBackend) readyByte() byte { return 0 }

func (b unavailableBackend) command(policy, []string) (*exec.Cmd, func(), error) {
	return nil, func() {}, errors.New(b.detail)
}

func runBackend(b backend, p policy, child []string, s streams) outcome {
	status := b.status()
	if !status.Available {
		fmt.Fprintf(s.err, "cage: confinement setup: %s\n", status.Detail)
		if status.Alternative != "" {
			fmt.Fprintf(s.err, "cage: alternative: %s\n", status.Alternative)
		}
		return outcome{code: exitSetup}
	}
	readyR, readyW, err := os.Pipe()
	if err != nil {
		fmt.Fprintln(s.err, "cage: confinement setup: readiness pipe:", err)
		return outcome{code: exitSetup}
	}
	defer readyR.Close()
	cmd, cleanup, err := b.command(p, child)
	if err != nil {
		readyW.Close()
		fmt.Fprintln(s.err, "cage: confinement setup:", err)
		return outcome{code: exitSetup}
	}
	defer cleanup()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = s.in, s.out, s.err
	cmd.ExtraFiles = append(cmd.ExtraFiles, readyW)
	if err := cmd.Start(); err != nil {
		readyW.Close()
		fmt.Fprintln(s.err, "cage: confinement setup:", err)
		return outcome{code: exitSetup}
	}
	readyW.Close()
	stopSignals := relaySignals(cmd.Process)
	type readyResult struct {
		mark byte
		err  error
	}
	ready := make(chan readyResult, 1)
	go func() {
		var mark [1]byte
		_, err := io.ReadFull(readyR, mark[:])
		ready <- readyResult{mark: mark[0], err: err}
	}()
	var proof readyResult
	select {
	case proof = <-ready:
	case <-time.After(setupTimeout):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		stopSignals()
		fmt.Fprintf(s.err, "cage: confinement setup: backend gave no readiness proof within %s\n", setupTimeout)
		return outcome{code: exitSetup}
	}
	waitErr := cmd.Wait()
	stopSignals()
	if proof.err != nil || proof.mark != b.readyByte() {
		if waitErr == nil {
			waitErr = proof.err
		}
		fmt.Fprintln(s.err, "cage: confinement setup: backend did not establish the boundary:", waitErr)
		return outcome{code: exitSetup}
	}
	return outcomeFromWait(waitErr)
}

func shellCommand(path string, prefix, child []string) *exec.Cmd {
	args := append([]string{}, prefix...)
	args = append(args, "/bin/sh", "-c", launcher, "cage-launch")
	args = append(args, child...)
	return exec.Command(path, args...)
}
