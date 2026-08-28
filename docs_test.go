package main

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestHelpDocumentsTheWholeInterface(t *testing.T) {
	for _, want := range []string{
		"context ls", "context query source [query]", "context merge",
		"context check", "CONTEXT_PATH", "exit: 0",
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

func TestDocsNameCommandsEnvironmentAndVersion(t *testing.T) {
	for _, name := range []string{"README.md", "GUIDE.md", "context.1"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, want := range []string{"ls", "query", "merge", "check", "CONTEXT_PATH"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not mention %s", name, want)
			}
		}
	}
	manual, err := os.ReadFile("context.1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manual), version) {
		t.Errorf("manual does not contain version %s", version)
	}
}

func TestManualIsPortableASCII(t *testing.T) {
	body, err := os.ReadFile("context.1")
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range body {
		if b > 0x7f {
			t.Fatalf("context.1 byte %d is not ASCII: %#x", i, b)
		}
	}
}

func TestProtocolDocsNameEveryRequiredRecordField(t *testing.T) {
	body, err := os.ReadFile("CONNECTORS.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, field := range []string{
		"kind", "version", "source", "type", "id", "title",
		"retrieved_at", "content", "citation.locator", "ref",
	} {
		if !strings.Contains(text, field) {
			t.Errorf("CONNECTORS.md does not mention %s", field)
		}
	}
}

func TestREADMEDocumentsWikipediaConnector(t *testing.T) {
	body, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		".context/connectors/wikipedia", "WIKIPEDIA_USER_AGENT", "WIKIPEDIA_API",
		"API:Search", "TextExtracts", "API:Etiquette", "Question: $q",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("README.md does not mention %s", want)
		}
	}
	info, err := os.Stat(".context/connectors/wikipedia")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Error("Wikipedia connector is not executable")
	}
}

func TestDocsComposeCiteAsASeparateFilter(t *testing.T) {
	for _, name := range []string{"README.md", "GUIDE.md"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"cite evidence.jsonl", "supports"} {
			if !strings.Contains(string(body), want) {
				t.Errorf("%s does not mention %s", name, want)
			}
		}
	}
}
