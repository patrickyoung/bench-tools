// Command context retrieves evidence through executable source connectors.
//
// It owns one small wire contract, not the providers behind it. Connectors may
// be written in any language; context finds one, gives it a query, validates
// its JSONL, adds stable citation references, and prints the records.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const version = "0.1.0"

const (
	exitYes            = 0
	exitNo             = 1
	exitErr            = 2
	maxDiagnosticBytes = 1 << 20
)

const usageText = `context - retrieve cited external context through connectors

  context ls                     list available source connectors as JSONL
  context query source [query]   query one source; query may come from stdin
  context merge                  validate and combine context JSONL on stdin
  context check                  validate context JSONL on stdin
  context version                print the version (-V, --version)
  context help                   print this summary (-h, --help)

a connector is an executable named for its source. context runs it with
describe to read one source record, or query with the exact query on stdin.
Connectors live on CONTEXT_PATH, searched left to right like PATH.

stdout is JSONL and nothing else. query and merge emit context/v1 records
with stable ref fields for citations. Diagnostics go to stderr.

env: CONTEXT_PATH  connector directories (default .context/connectors,
       ~/.context/connectors)
exit: 0 result/clean - 1 no connector, context, or input - 2 error
`

type app struct {
	in      io.Reader
	out     io.Writer
	errOut  io.Writer
	getenv  func(string) string
	homeDir func() (string, error)
	command func(string, ...string) *exec.Cmd
}

func main() {
	os.Exit(newApp().run(os.Args[1:]))
}

func newApp() *app {
	return &app{
		in: os.Stdin, out: os.Stdout, errOut: os.Stderr,
		getenv: os.Getenv, homeDir: os.UserHomeDir, command: exec.Command,
	}
}

func (a *app) run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.errOut, usageText)
		return exitErr
	}
	switch args[0] {
	case "ls":
		return a.cmdList(args[1:])
	case "query":
		return a.cmdQuery(args[1:])
	case "merge":
		return a.cmdMerge(args[1:])
	case "check":
		return a.cmdCheck(args[1:])
	case "version", "-V", "--version":
		fmt.Fprintf(a.out, "context %s\n", version)
		return exitYes
	case "help", "-h", "--help":
		fmt.Fprint(a.out, usageText)
		return exitYes
	default:
		fmt.Fprintf(a.errOut, "context: unknown command %q\n", args[0])
		fmt.Fprintln(a.errOut, "context: commands are ls, query, merge, check, version, help")
		return exitErr
	}
}

func (a *app) cmdList(args []string) int {
	if len(args) != 0 {
		return a.usage("context ls")
	}
	connectors, err := a.connectors()
	if err != nil {
		return a.fail(err)
	}
	if len(connectors) == 0 {
		return exitNo
	}
	var answer bytes.Buffer
	for _, c := range connectors {
		raw, stderr, code, err := a.invoke(c.path, "describe", nil)
		a.relay(c.name, stderr)
		if err != nil || code != 0 {
			if err == nil {
				err = fmt.Errorf("describe exited %d", code)
			}
			return a.fail(fmt.Errorf("source %s: %w", c.name, err))
		}
		line, err := normalizeSource(raw, c.name)
		if err != nil {
			return a.fail(fmt.Errorf("source %s: %w", c.name, err))
		}
		answer.Write(line)
		answer.WriteByte('\n')
	}
	if _, err := answer.WriteTo(a.out); err != nil {
		return a.fail(fmt.Errorf("write output: %w", err))
	}
	return exitYes
}

func (a *app) cmdQuery(args []string) int {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(a.errOut)
	fs.Usage = func() { fmt.Fprintln(a.errOut, "usage: context query source [query]") }
	if err := fs.Parse(args); err != nil || fs.NArg() < 1 || fs.NArg() > 2 {
		if err == nil {
			fs.Usage()
		}
		return exitErr
	}
	name := fs.Arg(0)
	path, err := a.connector(name)
	if err != nil {
		if errors.Is(err, errNotFound) {
			fmt.Fprintf(a.errOut, "context: source %q not found\n", name)
			return exitNo
		}
		return a.fail(err)
	}
	var query []byte
	if fs.NArg() == 2 {
		query = []byte(fs.Arg(1))
	} else {
		query, err = readBounded(a.in, maxQueryBytes)
		if err != nil {
			return a.fail(fmt.Errorf("query: %w", err))
		}
	}
	if len(bytes.TrimSpace(query)) == 0 {
		return a.usage("context query source [query]")
	}
	if len(query) > maxQueryBytes {
		return a.fail(fmt.Errorf("query exceeds %d bytes", maxQueryBytes))
	}
	raw, stderr, code, runErr := a.invoke(path, "query", query)
	a.relay(name, stderr)
	if code == exitNo && runErr != nil && len(bytes.TrimSpace(raw)) == 0 {
		return exitNo
	}
	if runErr != nil || code != exitYes {
		if runErr == nil {
			runErr = fmt.Errorf("query exited %d", code)
		}
		return a.fail(fmt.Errorf("source %s: %w", name, runErr))
	}
	records, err := normalizeRecords(raw, name, true)
	if err != nil {
		return a.fail(fmt.Errorf("source %s: %w", name, err))
	}
	if len(records) == 0 {
		return a.fail(fmt.Errorf("source %s: exit 0 with no context", name))
	}
	return a.writeRecords(records)
}

func (a *app) cmdMerge(args []string) int {
	if len(args) != 0 {
		return a.usage("context merge")
	}
	raw, err := readBounded(a.in, maxStreamBytes)
	if err != nil {
		return a.fail(err)
	}
	records, err := normalizeRecords(raw, "", true)
	if err != nil {
		return a.fail(err)
	}
	if len(records) == 0 {
		return exitNo
	}
	merged, err := mergeRecords(records)
	if err != nil {
		return a.fail(err)
	}
	return a.writeRecords(merged)
}

func (a *app) cmdCheck(args []string) int {
	if len(args) != 0 {
		return a.usage("context check")
	}
	raw, err := readBounded(a.in, maxStreamBytes)
	if err != nil {
		return a.fail(err)
	}
	records, err := normalizeRecords(raw, "", false)
	if err != nil {
		return a.fail(err)
	}
	if len(records) == 0 {
		return exitNo
	}
	if _, err := mergeRecords(records); err != nil {
		return a.fail(err)
	}
	return exitYes
}

func (a *app) invoke(path, verb string, stdin []byte) ([]byte, []byte, int, error) {
	cmd := a.command(path, verb)
	cmd.Stdin = bytes.NewReader(stdin)
	stdout := newBoundedBuffer(maxStreamBytes)
	stderr := newBoundedBuffer(maxDiagnosticBytes)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if stdout.exceeded {
		err = fmt.Errorf("output exceeds %d bytes", maxStreamBytes)
	}
	if stderr.exceeded && err == nil {
		err = fmt.Errorf("diagnostics exceed %d bytes", maxDiagnosticBytes)
	}
	if err == nil {
		return stdout.bytes(), stderr.bytes(), 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return stdout.bytes(), stderr.bytes(), ee.ExitCode(), err
	}
	return stdout.bytes(), stderr.bytes(), -1, err
}

type boundedBuffer struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func newBoundedBuffer(limit int) *boundedBuffer {
	return &boundedBuffer{limit: limit}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	written := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) < remaining {
			remaining = len(p)
		}
		_, _ = b.buf.Write(p[:remaining])
	}
	if len(p) > remaining {
		b.exceeded = true
	}
	return written, nil
}

func (b *boundedBuffer) bytes() []byte { return b.buf.Bytes() }

func (a *app) relay(source string, b []byte) {
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if line != "" {
			fmt.Fprintf(a.errOut, "context: %s: %s\n", source, line)
		}
	}
}

func (a *app) usage(line string) int {
	fmt.Fprintf(a.errOut, "usage: %s\n", line)
	return exitErr
}

func (a *app) fail(err error) int {
	fmt.Fprintf(a.errOut, "context: %v\n", err)
	return exitErr
}

func (a *app) writeRecords(records [][]byte) int {
	var answer bytes.Buffer
	for _, record := range records {
		answer.Write(record)
		answer.WriteByte('\n')
	}
	if _, err := answer.WriteTo(a.out); err != nil {
		return a.fail(fmt.Errorf("write output: %w", err))
	}
	return exitYes
}
