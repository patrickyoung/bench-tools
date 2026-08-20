// Command rules prints the repository instructions that apply at a directory.
//
// It is a bounded reader. Instruction text is stdout, provenance is stderr,
// and prompt composition remains an explicit shell operation.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const version = "0.2.0"

const (
	exitYes  = 0
	exitNo   = 1
	exitErr  = 2
	maxRules = 32 * 1024
)

var instructionNames = []string{"AGENTS.md", "CLAUDE.md"}

const usageText = `rules - print the repository instructions that apply here

  rules [DIR]         print applicable instruction text
  rules -list [DIR]   print applicable instruction paths
  rules version       print the version (-V, --version)
  rules help          print this summary (-h, --help)

The nearest ancestor containing a regular .git file or .git directory is the
project root. Rules walks from that root to DIR. In each directory AGENTS.md
comes before CLAUDE.md. A symlinked instruction is allowed only when its fully
resolved target remains inside the project root.

stdout is instruction text, or one logical path per line with -list. stderr is
provenance. The instruction body set is limited to 32 KiB. Canonical aliases
are listed but their bodies are printed once. Rules validates the complete set
before stdout: a refusal never produces a partial prompt. Empty sets succeed.

exit: 0 success - 1 no project or refused instructions - 2 usage or I/O error
`

type app struct {
	out    io.Writer
	errOut io.Writer
	getwd  func() (string, error)
}

type options struct {
	list bool
	dir  string
}

type instruction struct {
	logical  string
	resolved string
	aliasOf  string
	data     []byte
}

type refusal struct{ message string }

func (e *refusal) Error() string { return e.message }

func main() {
	os.Exit(newApp().run(os.Args[1:]))
}

func newApp() *app {
	return &app{out: os.Stdout, errOut: os.Stderr, getwd: os.Getwd}
}

func (a *app) run(args []string) int {
	if len(args) == 1 {
		switch args[0] {
		case "version", "-V", "--version":
			fmt.Fprintf(a.out, "rules %s\n", version)
			return exitYes
		case "help", "-h", "--help":
			fmt.Fprint(a.out, usageText)
			return exitYes
		}
	}

	opts, err := parseArgs(args)
	if err != nil {
		return a.fail(exitErr, err)
	}
	start, err := a.startPath(opts.dir)
	if err != nil {
		return a.fail(exitErr, err)
	}
	root, err := projectRoot(start)
	if err != nil {
		if isRefusal(err) {
			return a.fail(exitNo, err)
		}
		return a.fail(exitErr, err)
	}
	fmt.Fprintf(a.errOut, "rules: root %s\n", strconv.Quote(root))

	files, total, err := loadInstructions(root, start)
	if err != nil {
		if isRefusal(err) {
			return a.fail(exitNo, err)
		}
		return a.fail(exitErr, err)
	}
	for _, file := range files {
		if file.aliasOf != "" {
			if file.logical == file.resolved {
				fmt.Fprintf(a.errOut, "rules: %s (alias of %s; body not repeated)\n",
					strconv.Quote(file.logical), strconv.Quote(file.aliasOf))
			} else {
				fmt.Fprintf(a.errOut, "rules: %s -> %s (alias of %s; body not repeated)\n",
					strconv.Quote(file.logical), strconv.Quote(file.resolved),
					strconv.Quote(file.aliasOf))
			}
			continue
		}
		if file.logical == file.resolved {
			fmt.Fprintf(a.errOut, "rules: %s (%d bytes)\n",
				strconv.Quote(file.logical), len(file.data))
		} else {
			fmt.Fprintf(a.errOut, "rules: %s -> %s (%d bytes)\n",
				strconv.Quote(file.logical), strconv.Quote(file.resolved),
				len(file.data))
		}
	}
	fmt.Fprintf(a.errOut, "rules: %d files, %d bytes\n", len(files), total)

	var output []byte
	if opts.list {
		output = listOutput(files)
	} else {
		output = textOutput(files)
	}
	if len(output) > 0 {
		n, err := a.out.Write(output)
		if err != nil {
			return a.fail(exitErr, fmt.Errorf("write stdout: %w", err))
		}
		if n != len(output) {
			return a.fail(exitErr, fmt.Errorf("write stdout: %w", io.ErrShortWrite))
		}
	}
	return exitYes
}

func parseArgs(args []string) (options, error) {
	var opts options
	endOptions := false
	if len(args) > 0 && args[0] == "-list" {
		opts.list = true
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "--" {
		endOptions = true
		args = args[1:]
	}
	if len(args) > 1 {
		return opts, errors.New("usage: rules [-list] [DIR]")
	}
	if len(args) == 1 {
		if !endOptions && strings.HasPrefix(args[0], "-") {
			return opts, fmt.Errorf("unknown option %q", args[0])
		}
		opts.dir = args[0]
	}
	return opts, nil
}

func (a *app) startPath(arg string) (string, error) {
	base, err := a.getwd()
	if err != nil {
		return "", fmt.Errorf("current directory: %w", err)
	}
	path := arg
	if path == "" {
		path = base
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", arg, err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make %q absolute: %w", arg, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("inspect %q: %w", path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", path)
	}
	return filepath.Clean(path), nil
}

func (a *app) fail(code int, err error) int {
	fmt.Fprintf(a.errOut, "rules: %v\n", err)
	return code
}

func isRefusal(err error) bool {
	var target *refusal
	return errors.As(err, &target)
}
