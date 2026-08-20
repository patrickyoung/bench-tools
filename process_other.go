//go:build !unix

package main

import (
	"errors"
	"os"
	"os/exec"
)

func relaySignals(*os.Process) func() { return func() {} }

func outcomeFromWait(err error) outcome {
	if err == nil {
		return outcome{code: exitOK}
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return outcome{code: ee.ExitCode()}
	}
	return outcome{code: exitSetup}
}

func reraise(os.Signal) {}
