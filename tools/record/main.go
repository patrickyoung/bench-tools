// Record retains one process boundary as sealed Ask notes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

const version = "0.1.0"

const help = `usage: record run -f SESSION [flags] -- COMMAND [ARG ...]
       record replay -f SESSION [-stream NAME | -json]
       record check -f SESSION
       record help | version

run creates a new Ask session and records one literal command invocation.
  -f FILE         new session file; never overwrites or uses Ask current
  -ask COMMAND    Ask executable (default: ask; requires Ask 0.3 or later)
  -input FILE     snapshot a regular input file before execution (repeatable)
  -output FILE    snapshot a regular output file after execution (repeatable)
  -session FILE   snapshot and verify a child Ask session afterwards (repeatable)
  -pass-fd N      pass an inherited private descriptor, without reading it
  -label TEXT     explicitly selected non-secret context (repeatable)
  -timeout D      execution limit, e.g. 30s (default: no limit)
  -grace D        signal cleanup before group kill (default: 1s)

replay verifies Ask seals and the entire process receipt before writing bytes.
  -f FILE         recorded session
  -ask COMMAND    Ask executable (default: ask)
  -stream NAME    extract stdin, stdout, stderr, or artifact:N to stdout
  -json           emit the verified invocation and terminal receipt as JSON

check verifies without emitting recorded streams. No replay command executes
the recorded process, refreshes credentials, or reads the original artifacts.
Run and replay preserve child exits; signals use 128 + signal. Recorder errors
and incomplete evidence are 125. Extraction, JSON, and check succeed with 0.
Only observed bytes are evidence. See README for pipe and credential limits.
`

type stringsFlag []string

func (v *stringsFlag) String() string     { return fmt.Sprint([]string(*v)) }
func (v *stringsFlag) Set(s string) error { *v = append(*v, s); return nil }

func flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(os.Stderr)
	f.Usage = func() { fmt.Fprint(f.Output(), help) }
	return f
}

func main() {
	// Turn a closed consumer pipe into a checked write error so the recorder
	// can stop capture and report 125 instead of dying before finalization.
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
	os.Exit(mainCode(os.Args[1:]))
}

func mainCode(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, help)
		return 125
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, help)
		return 0
	case "version", "--version":
		fmt.Fprintln(os.Stdout, "record "+version)
		return 0
	case "run":
		return runCLI(args[1:])
	case "replay", "check":
		return replayCLI(args[0], args[1:])
	default:
		return fail(fmt.Errorf("unknown command %q", args[0]))
	}
}

func fail(err error) int { fmt.Fprintln(os.Stderr, "record:", err); return 125 }

func parse(f *flag.FlagSet, args []string) error {
	if err := f.Parse(args); err != nil {
		return err
	}
	return nil
}

func writeAll(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return err
}

var errIncomplete = errors.New("incomplete recording; effects may exist; do not infer permission to retry")
