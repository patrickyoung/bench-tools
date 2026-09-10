package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const processPipeWait = 2 * time.Second

// runCommand runs a CommandContext in an owned process group. Cancellation
// must finish cleaning up that group even if its leader exits and its pipes
// close first: a descendant can redirect its output and ignore the interrupt.
func runCommand(ctx context.Context, cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pgid = 0
	grace := processGrace()
	var once sync.Once
	var cancelErr error
	stop := func() error {
		once.Do(func() { cancelErr = stopProcessGroup(cmd.Process.Pid, grace) })
		return cancelErr
	}
	cmd.Cancel = stop
	// A descendant outside the group can retain a pipe. Closing our copies
	// bounds teardown; process groups are lifecycle management, not a sandbox.
	cmd.WaitDelay = processPipeWait
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if ctx.Err() != nil || errors.Is(err, exec.ErrWaitDelay) {
			_ = stop()
		}
		return err
	case <-ctx.Done():
		// os/exec stops watching context when the leader exits, even if
		// descendants still hold its pipes. Keep this cancellation boundary
		// alive until Wait finishes; a nonzero exit hides ErrWaitDelay.
		_ = stop()
		return <-done
	}
}

// Inner Ply processes must escalate before the outer process can kill them:
// each owns separate groups that its parent cannot reach. Use the existing
// bounded delegation depth to leave a quarter second between those deadlines.
func processGrace() time.Duration {
	depth, _ := strconv.Atoi(os.Getenv("PLY_DEPTH"))
	depth = max(0, min(depth, maxDepth-1))
	return time.Duration(maxDepth-depth) * 250 * time.Millisecond
}

func stopProcessGroup(pid int, grace time.Duration) error {
	err := syscall.Kill(-pid, syscall.SIGINT)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	if err != nil {
		return err
	}
	deadline := time.NewTimer(grace)
	defer deadline.Stop()
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case <-poll.C:
			if errors.Is(syscall.Kill(-pid, 0), syscall.ESRCH) {
				return nil
			}
		case <-deadline.C:
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			return nil
		}
	}
}
