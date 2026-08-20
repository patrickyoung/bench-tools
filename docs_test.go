package main

import (
	"os"
	"strings"
	"testing"
)

func TestPublicDocsStateTheWholeContract(t *testing.T) {
	for _, name := range []string{"README.md", "GUIDE.md", "rules.1"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, required := range []string{
			"AGENTS.md", "CLAUDE.md", ".git", "32 KiB", "canonical",
		} {
			if !strings.Contains(text, required) {
				t.Errorf("%s does not mention %q", name, required)
			}
		}
	}
}

func TestManPageIsASCII(t *testing.T) {
	body, err := os.ReadFile("rules.1")
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range body {
		if b > 0x7f {
			t.Fatalf("rules.1 byte %d is non-ASCII: %#x", i, b)
		}
	}
}

func TestPublicTrustBoundaryIsExplicit(t *testing.T) {
	for _, name := range []string{"README.md", "SECURITY.md", "rules.1"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToLower(string(body)),
			"guidance, not authorization") {
			t.Errorf("%s does not state the trust boundary", name)
		}
	}
}
