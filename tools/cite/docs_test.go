package main

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestHelpDocumentsTheWholeInterface(t *testing.T) {
	for _, want := range []string{
		"cite evidence.jsonl", "cite version", "cite help",
		"[ctx:source:ref](citation.url)", "exit: 0",
	} {
		if !strings.Contains(usageText, want) {
			t.Errorf("help does not contain %q", want)
		}
	}
	for no, line := range strings.Split(usageText, "\n") {
		if utf8.RuneCountInString(line) > 80 {
			t.Errorf("help line %d exceeds 80 columns: %q", no+1, line)
		}
	}
}

func TestDocsCoverInterfaceLimitsAndScope(t *testing.T) {
	for _, name := range []string{"README.md", "GUIDE.md", "SECURITY.md", "cite.1"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, want := range []string{"evidence", "candidate", "citation", "support"} {
			if !strings.Contains(strings.ToLower(text), want) {
				t.Errorf("%s does not mention %s", name, want)
			}
		}
	}
	for _, name := range []string{"README.md", "GUIDE.md", "cite.1"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"4 MiB", "32 MiB", "8 MiB"} {
			if !strings.Contains(string(body), want) {
				t.Errorf("%s does not mention %s", name, want)
			}
		}
	}
}

func TestManualVersionAndPortableASCII(t *testing.T) {
	body, err := os.ReadFile("cite.1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), version) {
		t.Errorf("manual does not contain version %s", version)
	}
	for i, b := range body {
		if b > 0x7f {
			t.Fatalf("cite.1 byte %d is not ASCII: %#x", i, b)
		}
	}
}
