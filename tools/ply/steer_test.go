package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSteeringReadsOnlyCompleteAppendedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "steer")
	if err := os.WriteFile(path, []byte("first\npartial"), 0o600); err != nil {
		t.Fatal(err)
	}
	inbox, err := openSteering(path)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Close()
	if got, err := inbox.Read(); err != nil || got != "first" {
		t.Fatalf("first read = %q, %v", got, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(" line\nsecond\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if got, err := inbox.Read(); err != nil || got != "partial line\nsecond" {
		t.Fatalf("second read = %q, %v", got, err)
	}
}

func TestSteeringRejectsFIFOWithoutBlocking(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mkfifo is a Unix interface")
	}
	path := filepath.Join(t.TempDir(), "steer.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := openSteering(path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("FIFO error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("opening a steering FIFO blocked")
	}
}

func TestSteeringRejectsMalformedOrUnboundedInput(t *testing.T) {
	for name, body := range map[string][]byte{
		"nul":       {'b', 'a', 'd', 0, '\n'},
		"utf8":      {0xff, '\n'},
		"long-line": []byte(strings.Repeat("x", maxSteeringLine+1) + "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "steer")
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			inbox, err := openSteering(path)
			if err != nil {
				t.Fatal(err)
			}
			defer inbox.Close()
			if _, err := inbox.Read(); err == nil {
				t.Fatal("malformed steering was accepted")
			}
		})
	}
}

func TestWithSteeringPreservesAuthorityBoundary(t *testing.T) {
	got := withSteering("tool output", "focus on the parser")
	for _, want := range []string{"tool output", "OPERATOR STEERING", "focus on the parser", "does not amend", "grant approval", "change the verifier"} {
		if !strings.Contains(got, want) {
			t.Errorf("steered turn missing %q: %s", want, got)
		}
	}
}
