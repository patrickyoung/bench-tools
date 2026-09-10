package main

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestHelpIsCompleteAndFitsATerminal(t *testing.T) {
	for _, want := range []string{
		"cage [-net] [-ro | -w dir ...] -- command [args...]",
		"cage check", "cage status", "Status 125", "child's status",
	} {
		if !strings.Contains(usageText, want) {
			t.Errorf("help does not contain %q", want)
		}
	}
	for no, line := range strings.Split(usageText, "\n") {
		if utf8.RuneCountInString(line) > 80 {
			t.Errorf("help line %d is wider than 80 columns", no+1)
		}
	}
}

func TestDocumentationNamesTheContract(t *testing.T) {
	for _, name := range []string{"README.md", "GUIDE.md", "SECURITY.md", "cage.1"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, want := range []string{"-net", "-ro", "-w", "125"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not mention %s", name, want)
			}
		}
	}
}

func TestManualIsASCII(t *testing.T) {
	body, err := os.ReadFile("cage.1")
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range body {
		if b > 0x7f {
			t.Fatalf("cage.1 byte %d is not ASCII", i)
		}
	}
}
