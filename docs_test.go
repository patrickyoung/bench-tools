package main

import (
	"os"
	"strings"
	"testing"
)

func TestPublicDocsStateTheContract(t *testing.T) {
	for _, name := range []string{"README.md", "GUIDE.md", "may.1"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, required := range []string{
			"stdin", "/dev/tty", "exact", "pending", "decide", "audit",
			"75", "3", "toolbox", "check",
		} {
			if !strings.Contains(strings.ToLower(text), strings.ToLower(required)) {
				t.Errorf("%s does not mention %q", name, required)
			}
		}
	}
}

func TestSecurityDocsStateTheBoundary(t *testing.T) {
	body, err := os.ReadFile("SECURITY.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(body))
	for _, required := range []string{
		"exact", "stdin", "/dev/tty", "audit", "toolbox", "auto-approve",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("SECURITY.md does not mention %q", required)
		}
	}
}

func TestManPageIsASCII(t *testing.T) {
	body, err := os.ReadFile("may.1")
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range body {
		if b > 0x7f {
			t.Fatalf("may.1 byte %d is non-ASCII: %#x", i, b)
		}
	}
}

func TestProductionHasNoEnvironmentControl(t *testing.T) {
	for _, name := range []string{"main.go", "state.go", "tty_unix.go", "tty_windows.go"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"os.Getenv", "os.LookupEnv"} {
			if strings.Contains(string(body), forbidden) {
				t.Errorf("%s contains environment control %q", name, forbidden)
			}
		}
	}
}
