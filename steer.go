package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	maxSteeringLine  = 8 << 10
	maxSteeringBatch = 64 << 10
)

// steeringInbox is deliberately a regular file, not an RPC channel. A
// controller may append while Ply works; Ply reads complete lines only at the
// boundary before an Ask turn, so tool execution and verifier authority stay
// unchanged.
type steeringInbox struct {
	file    *os.File
	offset  int64
	partial []byte
}

func openSteering(path string) (*steeringInbox, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("open steering file: %w", err)
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, errors.New("steering path is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open steering file: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("steering path is not a regular file")
	}
	return &steeringInbox{file: file}, nil
}

func (s *steeringInbox) Close() error { return s.file.Close() }

func (s *steeringInbox) Read() (string, error) {
	info, err := s.file.Stat()
	if err != nil {
		return "", fmt.Errorf("read steering file state: %w", err)
	}
	if info.Size() < s.offset {
		return "", errors.New("steering file was truncated while Ply was running")
	}
	available := info.Size() - s.offset
	if available == 0 {
		return "", nil
	}
	if available > maxSteeringBatch+1 {
		return "", fmt.Errorf("pending steering exceeds %d bytes", maxSteeringBatch)
	}
	buf := make([]byte, available)
	n, err := s.file.ReadAt(buf, s.offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read steering file: %w", err)
	}
	buf = buf[:n]
	s.offset += int64(n)
	all := append(append([]byte(nil), s.partial...), buf...)
	last := bytes.LastIndexByte(all, '\n')
	if last < 0 {
		if len(all) > maxSteeringLine {
			return "", fmt.Errorf("steering line exceeds %d bytes", maxSteeringLine)
		}
		s.partial = all
		return "", nil
	}
	complete := all[:last]
	s.partial = append(s.partial[:0], all[last+1:]...)
	if len(s.partial) > maxSteeringLine {
		return "", fmt.Errorf("steering line exceeds %d bytes", maxSteeringLine)
	}
	if len(complete) > maxSteeringBatch {
		return "", fmt.Errorf("pending steering exceeds %d bytes", maxSteeringBatch)
	}
	lines := bytes.Split(complete, []byte{'\n'})
	accepted := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if len(line) > maxSteeringLine {
			return "", fmt.Errorf("steering line exceeds %d bytes", maxSteeringLine)
		}
		if bytes.IndexByte(line, 0) >= 0 || !utf8.Valid(line) {
			return "", errors.New("steering must be valid UTF-8 without NUL bytes")
		}
		accepted = append(accepted, string(line))
	}
	return strings.Join(accepted, "\n"), nil
}
