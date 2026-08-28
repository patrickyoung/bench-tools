package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCite(t *testing.T, evidence, candidate string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	a := &app{in: strings.NewReader(candidate), out: &stdout, errOut: &stderr}
	if evidence != "" {
		path := filepath.Join(t.TempDir(), "evidence.jsonl")
		if err := os.WriteFile(path, []byte(evidence), 0o600); err != nil {
			t.Fatal(err)
		}
		for i, arg := range args {
			if arg == "$evidence" {
				args[i] = path
			}
		}
	}
	code := a.run(args)
	return code, stdout.String(), stderr.String()
}

func contextLine(ref, url string) string {
	citation := map[string]any{"locator": "source/item"}
	if url != "" {
		citation["url"] = url
	}
	record := map[string]any{
		"kind": "context", "version": 1, "source": "docs",
		"type": "document", "id": "item", "title": "Item",
		"retrieved_at": "2026-08-28T12:00:00Z", "content": "evidence",
		"citation": citation, "ref": ref,
	}
	b, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	return string(b) + "\n"
}

func TestValidCandidatePassesThroughByteForByte(t *testing.T) {
	ref := "ctx:docs:0123456789abcdef0123456789abcdef"
	url := "https://example.test/article_(one)?oldid=7"
	evidence := contextLine(ref, url)
	candidate := "A supported claim. [" + ref + "](" + url + ")\n"
	code, stdout, stderr := runCite(t, evidence, candidate, "$evidence")
	if code != exitYes || stdout != candidate || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestMultipleExactCitationsFromMultipleSourcesPass(t *testing.T) {
	aRef := "ctx:docs:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	bRef := "ctx:wiki:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	aURL := "https://docs.test/a"
	bURL := "https://wiki.test/b"
	evidence := contextLine(aRef, aURL) + contextLine(bRef, bURL)
	candidate := fmt.Sprintf("One [%s](%s), two [%s](%s).", aRef, aURL, bRef, bURL)
	code, stdout, stderr := runCite(t, evidence, candidate, "$evidence")
	if code != exitYes || stdout != candidate || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestCandidateRejectionsLeaveStdoutEmpty(t *testing.T) {
	ref := "ctx:docs:0123456789abcdef0123456789abcdef"
	url := "https://example.test/article"
	evidence := contextLine(ref, url)
	unknown := "ctx:docs:ffffffffffffffffffffffffffffffff"
	for _, tc := range []struct {
		name      string
		candidate string
		want      string
	}{
		{"empty", "", "candidate is empty"},
		{"none", "A claim without a citation.", "no valid citations"},
		{"bare", "A claim " + ref, "must appear exactly"},
		{"wrong-url", "A claim [" + ref + "](https://wrong.test)", "must appear exactly"},
		{"ref-in-title", "A claim [source](" + url + " \"" + ref + "\")", "must appear exactly"},
		{"unknown", "A claim [" + unknown + "](https://example.test)", "unknown ref"},
		{"malformed", "A claim ctx:not-enough", "malformed ctx ref"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runCite(t, evidence, tc.candidate, "$evidence")
			if code != exitNo || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestRefWithoutURLCannotBeCited(t *testing.T) {
	ref := "ctx:docs:0123456789abcdef0123456789abcdef"
	other := "ctx:docs:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	evidence := contextLine(ref, "") + contextLine(other, "https://other.test")
	candidate := "A claim [" + ref + "](https://invented.test)"
	code, stdout, stderr := runCite(t, evidence, candidate, "$evidence")
	if code != exitNo || stdout != "" || !strings.Contains(stderr, "has no citation.url") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestEvidenceWithoutAnyURLIsOrdinaryRejection(t *testing.T) {
	ref := "ctx:docs:0123456789abcdef0123456789abcdef"
	evidence := contextLine(ref, "")
	code, stdout, stderr := runCite(t, evidence, "candidate", "$evidence")
	if code != exitNo || stdout != "" || !strings.Contains(stderr, "no evidence records have citation.url") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestBrokenEvidenceIsError(t *testing.T) {
	ref := "ctx:docs:0123456789abcdef0123456789abcdef"
	good := contextLine(ref, "https://one.test")
	conflict := contextLine(ref, "https://two.test")
	for _, tc := range []struct {
		name     string
		evidence string
		want     string
	}{
		{"invalid-json", "{bad}\n", "invalid JSON"},
		{"wrong-kind", `{"kind":"source","version":1}` + "\n", "kind must be"},
		{"blank-line", good + "\n", "blank JSONL"},
		{"conflict", good + conflict, "conflicting URLs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runCite(t, tc.evidence, "candidate", "$evidence")
			if code != exitErr || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestNoEvidenceIsOrdinaryRejection(t *testing.T) {
	code, stdout, stderr := runCite(t, "", "candidate", "$evidence")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "open evidence") {
		t.Fatalf("missing file: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = runCite(t, "\n", "candidate", "$evidence")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "blank JSONL") {
		t.Fatalf("blank evidence: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	path := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	a := &app{in: strings.NewReader("candidate"), out: &out, errOut: &errOut}
	if code := a.run([]string{path}); code != exitNo || out.Len() != 0 ||
		!strings.Contains(errOut.String(), "no context records") {
		t.Fatalf("empty file: exit=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestCandidateLimitIsAnError(t *testing.T) {
	ref := "ctx:docs:0123456789abcdef0123456789abcdef"
	evidence := contextLine(ref, "https://example.test")
	candidate := strings.Repeat("x", maxCandidateBytes+1)
	code, stdout, stderr := runCite(t, evidence, candidate, "$evidence")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, fmt.Sprint(maxCandidateBytes)) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestUsageVersionAndHelp(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		wantCode int
		wantOut  string
	}{
		{nil, exitErr, ""},
		{[]string{"help"}, exitYes, "cite - verify"},
		{[]string{"version"}, exitYes, "cite " + version},
		{[]string{"a", "b"}, exitErr, ""},
	} {
		code, stdout, _ := runCite(t, "", "", tc.args...)
		if code != tc.wantCode || !strings.Contains(stdout, tc.wantOut) {
			t.Fatalf("%v: exit=%d stdout=%q", tc.args, code, stdout)
		}
	}
}
