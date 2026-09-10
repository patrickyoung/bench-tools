// Command cage confines one child process using the host kernel.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const version = "0.1.0"

const (
	exitOK    = 0
	exitCheck = 1
	exitUsage = 2
	exitSetup = 125
)

const usageText = `cage - confine one command with the host kernel

  cage [-net] [-ro | -w dir ...] -- command [args...]
  cage check
  cage status
  cage version

The child may read the host filesystem. By default it may write only the
current directory and the temporary directory, and cannot reach host networks.
-ro removes the current-directory write. One or more -w flags replace it with
the named writable directories. The temporary directory remains writable.

stdin, stdout, stderr, and the child's status pass through. Status 125 means
the confinement could not be established; Cage fails closed and does not run
the child. Status 2 is Cage usage. Other statuses belong to the child.
`

type streams struct {
	in  io.Reader
	out io.Writer
	err io.Writer
}

type outcome struct {
	code   int
	signal os.Signal
}

type options struct {
	network  bool
	readOnly bool
	writes   []string
	command  []string
}

type policy struct {
	network bool
	writes  []string
	temp    string
	cwd     string
}

type confinementError struct{ err error }

func (e confinementError) Error() string { return e.err.Error() }

func (e confinementError) Unwrap() error { return e.err }

type statusRecord struct {
	Kind        string `json:"kind"`
	Platform    string `json:"platform"`
	Backend     string `json:"backend"`
	Available   bool   `json:"available"`
	Complete    bool   `json:"complete"`
	Filesystem  string `json:"filesystem"`
	Network     string `json:"network"`
	Detail      string `json:"detail,omitempty"`
	Alternative string `json:"alternative,omitempty"`
}

func main() {
	s := streams{in: os.Stdin, out: os.Stdout, err: os.Stderr}
	r := run(os.Args[1:], s)
	if r.signal != nil {
		reraise(r.signal)
	}
	os.Exit(r.code)
}

func run(args []string, s streams) outcome {
	if handled, result := internalCommand(args, s); handled {
		return result
	}
	if len(args) == 0 {
		fmt.Fprint(s.err, usageText)
		return outcome{code: exitUsage}
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(s.out, usageText)
		return outcome{code: exitOK}
	case "version", "-V", "--version":
		fmt.Fprintf(s.out, "cage %s\n", version)
		return outcome{code: exitOK}
	case "status":
		if len(args) != 1 {
			return usage(s, "cage status")
		}
		return writeStatus(s)
	case "check":
		if len(args) != 1 {
			return usage(s, "cage check")
		}
		return runCheck(s)
	}
	opts, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(s.err, "cage:", err)
		return usage(s, "cage [-net] [-ro | -w dir ...] -- command [args...]")
	}
	p, err := makePolicy(opts)
	if err != nil {
		var setup confinementError
		if errors.As(err, &setup) {
			fmt.Fprintln(s.err, "cage: confinement setup:", err)
			return outcome{code: exitSetup}
		}
		fmt.Fprintln(s.err, "cage:", err)
		return outcome{code: exitUsage}
	}
	return runBackend(nativeBackend(), p, opts.command, s)
}

func usage(s streams, synopsis string) outcome {
	fmt.Fprintln(s.err, "usage:", synopsis)
	return outcome{code: exitUsage}
}

func parseOptions(args []string) (options, error) {
	var o options
	for len(args) > 0 {
		switch args[0] {
		case "--":
			args = args[1:]
			o.command = args
			args = nil
		case "-net":
			o.network = true
			args = args[1:]
		case "-ro":
			o.readOnly = true
			args = args[1:]
		case "-w":
			if len(args) < 2 {
				return o, errors.New("-w needs a directory")
			}
			o.writes = append(o.writes, args[1])
			args = args[2:]
		default:
			if strings.HasPrefix(args[0], "-") {
				return o, fmt.Errorf("unknown flag %q", args[0])
			}
			o.command = args
			args = nil
		}
	}
	if o.readOnly && len(o.writes) > 0 {
		return o, errors.New("-ro and -w cannot be used together")
	}
	if len(o.command) == 0 {
		return o, errors.New("no command")
	}
	return o, nil
}

func makePolicy(o options) (policy, error) {
	cwd, err := canonicalDir(".")
	if err != nil {
		return policy{}, confinementError{fmt.Errorf("current directory: %w", err)}
	}
	temp, err := canonicalDir(os.TempDir())
	if err != nil {
		return policy{}, confinementError{fmt.Errorf("temporary directory: %w", err)}
	}
	p := policy{network: o.network, cwd: cwd, temp: temp}
	paths := o.writes
	if !o.readOnly && len(paths) == 0 {
		paths = []string{cwd}
	}
	for _, path := range paths {
		real, err := canonicalDir(path)
		if err != nil {
			return policy{}, fmt.Errorf("writable directory %q: %w", path, err)
		}
		p.writes = append(p.writes, real)
	}
	p.writes = minimalRoots(append(p.writes, temp))
	return p, nil
}

func canonicalDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("not a directory")
	}
	return filepath.Clean(real), nil
}

func minimalRoots(paths []string) []string {
	var out []string
	for _, path := range paths {
		duplicate := false
		for _, have := range out {
			if path == have {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, path)
		}
	}
	return out
}

func writeStatus(s streams) outcome {
	record := nativeBackend().status()
	record.Kind = "cage-status"
	record.Platform = runtime.GOOS
	enc := json.NewEncoder(s.out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(record); err != nil {
		fmt.Fprintln(s.err, "cage: writing status:", err)
		return outcome{code: exitSetup}
	}
	return outcome{code: exitOK}
}
