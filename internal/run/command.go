package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Command runs literal argv, gives the child one Authorization header on file
// descriptor 3, and preserves the child's ordinary process contract.
func Command(ctx context.Context, argv []string, header string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(argv) == 0 {
		return 2, errors.New("missing command")
	}
	read, write, err := os.Pipe()
	if err != nil {
		return 1, err
	}
	if _, err := io.WriteString(write, header); err != nil {
		read.Close()
		write.Close()
		return 1, err
	}
	if err := write.Close(); err != nil {
		read.Close()
		return 1, err
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	cmd.ExtraFiles = []*os.File{read}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		read.Close()
		return 127, err
	}
	_ = read.Close()

	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-signals:
			if s, ok := sig.(syscall.Signal); ok {
				_ = syscall.Kill(-cmd.Process.Pid, s)
			}
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		case <-done:
		}
	}()

	err = cmd.Wait()
	close(done)
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				return 128 + int(status.Signal()), nil
			}
			return status.ExitStatus(), nil
		}
	}
	return 1, fmt.Errorf("wait for %s: %w", argv[0], err)
}
