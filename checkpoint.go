package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const maxCheckpoint = 16 << 10

// checkpointLease serializes whole Ply invocations that share a checkpoint.
// Ask locks each append, but a conversation is more than one append: without
// this lock, two loops could take turns writing individually valid nonsense.
type checkpointLease struct {
	Path string
	lock *os.File
}

// acquireCheckpoint turns one durable pointer into the current Ask session.
// A missing pointer mints a session name; work publishes it before the first
// model turn through the existing -session-out path. The separate lock inode
// stays stable while the pointer itself is atomically replaced on compaction.
func acquireCheckpoint(path string) (*checkpointLease, string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("-checkpoint: %w", err)
	}
	if bytes.ContainsAny([]byte(abs), "\r\n") {
		return nil, "", errors.New("-checkpoint path contains a newline")
	}
	parent := filepath.Dir(abs)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return nil, "", fmt.Errorf("-checkpoint parent is not a directory: %s", parent)
	}
	lockPath := abs + ".lock"
	if info, err := os.Lstat(lockPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, "", fmt.Errorf("-checkpoint lock is not a regular file: %s", lockPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("-checkpoint lock: %w", err)
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("-checkpoint lock: %w", err)
	}
	lease := &checkpointLease{Path: abs, lock: lock}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, "", fmt.Errorf("checkpoint is already in use: %s", abs)
		}
		return nil, "", fmt.Errorf("-checkpoint lock: %w", err)
	}
	session, err := readCheckpoint(abs)
	if err != nil {
		_ = lease.Close()
		return nil, "", err
	}
	if session == "" {
		session, err = mint()
		if err != nil {
			_ = lease.Close()
			return nil, "", fmt.Errorf("-checkpoint session: %w", err)
		}
	}
	return lease, session, nil
}

func (c *checkpointLease) Close() error {
	if c == nil || c.lock == nil {
		return nil
	}
	err := syscall.Flock(int(c.lock.Fd()), syscall.LOCK_UN)
	closeErr := c.lock.Close()
	c.lock = nil
	if err != nil {
		return err
	}
	return closeErr
}

func readCheckpoint(path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read checkpoint: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("checkpoint is not a regular file: %s", path)
	}
	if info.Size() == 0 || info.Size() > maxCheckpoint {
		return "", fmt.Errorf("checkpoint has invalid size: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read checkpoint: %w", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' || bytes.Count(b, []byte{'\n'}) != 1 || bytes.ContainsRune(b, '\r') {
		return "", fmt.Errorf("checkpoint must contain one absolute session path and newline: %s", path)
	}
	session := string(b[:len(b)-1])
	if !filepath.IsAbs(session) || filepath.Clean(session) != session {
		return "", fmt.Errorf("checkpoint session path is not clean and absolute: %s", path)
	}
	if session == path {
		return "", fmt.Errorf("checkpoint must not name itself: %s", path)
	}
	if sessionInfo, statErr := os.Lstat(session); statErr == nil {
		if sessionInfo.Mode()&os.ModeSymlink != 0 || !sessionInfo.Mode().IsRegular() {
			return "", fmt.Errorf("checkpoint session is not a regular file: %s", session)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("checkpoint session: %w", statErr)
	}
	return session, nil
}
