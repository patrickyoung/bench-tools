package main

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestHelpDocumentsTheWholeInterface(t *testing.T) {
	wants := []string{
		"trail ls [dir]",
		"trail find query [dir]",
		"trail show session",
		"trail window [-before n] [-after n] session seq",
		"trail lineage session [dir]",
		"trail check [dir]",
		"ASK_DIR",
		"ASK ",
		"exit: 0",
	}
	for _, want := range wants {
		if !strings.Contains(usageText, want) {
			t.Errorf("help does not contain %q", want)
		}
	}
	for no, line := range strings.Split(usageText, "\n") {
		if utf8.RuneCountInString(line) > 80 {
			t.Errorf("help line %d is wider than 80 columns: %q", no+1, line)
		}
	}
}

func TestREADMEAndManualNameEveryCommand(t *testing.T) {
	for _, name := range []string{"README.md", "GUIDE.md", "trail.1"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, command := range []string{
			"ls", "find", "show", "window", "lineage", "check",
		} {
			if !strings.Contains(text, command) {
				t.Errorf("%s does not mention %s", name, command)
			}
		}
	}
}

func TestManualIsPortableASCII(t *testing.T) {
	body, err := os.ReadFile("trail.1")
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range body {
		if b > 0x7f {
			t.Fatalf("trail.1 byte %d is not ASCII: %#x", i, b)
		}
	}
}
