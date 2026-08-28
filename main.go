// Command cite verifies literal Markdown citations against Context JSONL.
//
// It does not decide whether evidence supports prose. It proves the smaller,
// mechanical claim that every ctx ref in a candidate is paired with the exact
// URL retrieved for that ref, then passes the candidate through unchanged.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

const version = "0.1.0"

const (
	exitYes = 0
	exitNo  = 1
	exitErr = 2
)

const usageText = `cite - verify Markdown citations against Context evidence

  cite evidence.jsonl       validate stdin, then print it unchanged
  cite version              print the version (-V, --version)
  cite help                 print this summary (-h, --help)

evidence is normalized context/v1 JSONL. A citation is literal Markdown:
  [ctx:source:ref](citation.url)
The label and URL must exactly match one record. Every ctx: occurrence in the
candidate must be such a link, and at least one citation is required.

stdout is the unchanged candidate on success and empty otherwise. Diagnostics
go to stderr. cite verifies citation identity, not whether evidence supports a
claim, whether every claim is cited, or whether a source is authoritative.

exit: 0 valid - 1 rejected candidate or no evidence - 2 error
`

type app struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

func main() {
	os.Exit(newApp().run(os.Args[1:]))
}

func newApp() *app {
	return &app{in: os.Stdin, out: os.Stdout, errOut: os.Stderr}
}

func (a *app) run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.errOut, usageText)
		return exitErr
	}
	if len(args) == 1 {
		switch args[0] {
		case "version", "-V", "--version":
			fmt.Fprintf(a.out, "cite %s\n", version)
			return exitYes
		case "help", "-h", "--help":
			fmt.Fprint(a.out, usageText)
			return exitYes
		}
	}
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(a.errOut, "usage: cite evidence.jsonl")
		return exitErr
	}

	evidence, err := loadEvidence(args[0])
	if err != nil {
		return a.fail(err)
	}
	if len(evidence) == 0 {
		fmt.Fprintln(a.errOut, "cite: no context records in evidence")
		return exitNo
	}
	hasURL := false
	for _, item := range evidence {
		if item.url != "" {
			hasURL = true
			break
		}
	}
	if !hasURL {
		fmt.Fprintln(a.errOut, "cite: no evidence records have citation.url")
		return exitNo
	}
	candidate, err := readBounded(a.in, maxCandidateBytes)
	if err != nil {
		return a.fail(fmt.Errorf("candidate: %w", err))
	}
	problems := checkCandidate(candidate, evidence)
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintf(a.errOut, "cite: %s\n", problem)
		}
		return exitNo
	}
	if _, err := io.Copy(a.out, bytes.NewReader(candidate)); err != nil {
		return a.fail(fmt.Errorf("write output: %w", err))
	}
	return exitYes
}

func (a *app) fail(err error) int {
	fmt.Fprintf(a.errOut, "cite: %v\n", err)
	return exitErr
}

func readBounded(r io.Reader, limit int) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > limit {
		return nil, fmt.Errorf("input exceeds %d bytes", limit)
	}
	return b, nil
}
