// Command trail reads ask session logs and writes selected records as JSONL.
//
// It is a reader, not a session service. The files remain ask's, replay
// remains ask's assertion, and stdout stays data another program can consume.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const version = "0.1.0"

const (
	exitYes = 0
	exitNo  = 1
	exitErr = 2
)

var commands = []string{
	"ls", "find", "show", "window", "lineage", "check", "version", "help",
}

const usageText = `trail - read ask session logs as JSONL

  trail ls [dir]                         summarize an archive
  trail find query [dir]                 find semantic text
  trail show session                     print every event
  trail window [-before n] [-after n] session seq
                                         print one bounded event window
  trail lineage session [dir]            print recorded lineage
  trail check [dir]                      run ask replay -check on each file
  trail version                          print the version (-V, --version)
  trail help                             print this summary (-h, --help)

stdout is JSONL: session, match, event, node, edge, check, warning, or error
records. Fatal diagnostics go to stderr.

find folds whitespace and Unicode case for matching. It searches user and
assistant text, reasoning, attachment names and media types, notes, retry and
terminal errors, and request model/effort labels. It never searches base64
media, opaque provider state, digests, schemas, or system prompts.

a session is a path when that path exists, otherwise a bare id under ASK_DIR.
An archive defaults to ASK_DIR, then ~/.ask/sessions. lineage reports only
parent and summary ids recorded by ask; it never infers that a copy is a fork.

env: ASK_DIR  default archive and bare-id directory
     ASK      ask binary used only by check (default: ask on PATH)
exit: 0 result/success - 1 no match, damage, or failed check - 2 error
`

type app struct {
	out    io.Writer
	errOut io.Writer
	getenv func(string) string
}

func main() {
	os.Exit(newApp().run(os.Args[1:]))
}

func newApp() *app {
	return &app{out: os.Stdout, errOut: os.Stderr, getenv: os.Getenv}
}

func (a *app) run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.errOut, usageText)
		return exitErr
	}
	switch args[0] {
	case "ls":
		return a.cmdList(args[1:])
	case "find":
		return a.cmdFind(args[1:])
	case "show":
		return a.cmdShow(args[1:])
	case "window":
		return a.cmdWindow(args[1:])
	case "lineage":
		return a.cmdLineage(args[1:])
	case "check":
		return a.cmdCheck(args[1:])
	case "version", "-V", "--version":
		fmt.Fprintf(a.out, "trail %s\n", version)
		return exitYes
	case "help", "-h", "--help":
		fmt.Fprint(a.out, usageText)
		return exitYes
	default:
		fmt.Fprintf(a.errOut, "trail: unknown command %q\n", args[0])
		fmt.Fprintf(a.errOut, "trail: commands are %s\n",
			strings.Join(commands, ", "))
		return exitErr
	}
}

func (a *app) cmdList(args []string) int {
	if len(args) > 1 {
		return a.usage("trail ls [dir]")
	}
	dir, explicit := a.archiveArg(args)
	files, err := archiveFiles(dir, explicit)
	if err != nil {
		return a.fail(err)
	}
	records := newRecords(a.out)
	bad := false
	for _, path := range files {
		s, rerr := readSession(path)
		if rerr != nil {
			records.write(errorFor(path, s, rerr))
			bad = true
			continue
		}
		rec, err := summarize(s)
		if err != nil {
			records.write(errorFor(path, s, err))
			bad = true
			continue
		}
		records.write(rec)
		if s.Torn {
			records.write(warningFor(s, "torn final line ignored"))
		}
	}
	return a.finish(records, noIf(bad))
}

func (a *app) cmdFind(args []string) int {
	if len(args) < 1 || len(args) > 2 || strings.TrimSpace(args[0]) == "" {
		return a.usage("trail find query [dir]")
	}
	query := foldForMatch(args[0])
	if query == "" {
		return a.usage("trail find query [dir]")
	}
	dir, explicit := a.archiveArg(args[1:])
	files, err := archiveFiles(dir, explicit)
	if err != nil {
		return a.fail(err)
	}
	records := newRecords(a.out)
	found, bad := false, false
	for _, path := range files {
		s, rerr := readSession(path)
		for _, e := range s.Events {
			text, terr := semanticText(e)
			if terr != nil {
				rerr = errors.Join(rerr, terr)
				break
			}
			if text != "" && strings.Contains(foldForMatch(text), query) {
				records.write(matchRecord{Kind: "match", Session: s.ID(), Path: s.Path,
					Seq: e.Seq, Time: e.Time, Type: e.Type, Text: text})
				found = true
			}
		}
		if rerr != nil {
			records.write(errorFor(path, s, rerr))
			bad = true
		} else if s.Torn {
			records.write(warningFor(s, "torn final line ignored"))
		}
	}
	if bad || !found {
		return a.finish(records, exitNo)
	}
	return a.finish(records, exitYes)
}

func (a *app) cmdShow(args []string) int {
	if len(args) != 1 {
		return a.usage("trail show session")
	}
	path, err := a.resolveSession(args[0])
	if err != nil {
		return a.fail(err)
	}
	s, rerr := readSession(path)
	records := newRecords(a.out)
	for _, e := range s.Events {
		records.write(eventRecord{
			Kind: "event", Session: s.ID(), Path: s.Path, Event: e.Raw,
		})
	}
	if rerr != nil {
		records.write(errorFor(path, s, rerr))
		return a.finish(records, exitNo)
	}
	if s.Torn {
		records.write(warningFor(s, "torn final line ignored"))
	}
	return a.finish(records, exitYes)
}

func (a *app) cmdWindow(args []string) int {
	before, after, pos, err := parseWindowArgs(args)
	if err != nil || len(pos) != 2 {
		return a.usage("trail window [-before n] [-after n] session seq")
	}
	seq, err := strconv.Atoi(pos[1])
	if err != nil || seq < 1 {
		return a.usage("trail window [-before n] [-after n] session seq")
	}
	path, err := a.resolveSession(pos[0])
	if err != nil {
		return a.fail(err)
	}
	s, rerr := readSession(path)
	idx := -1
	for i, e := range s.Events {
		if e.Seq == seq {
			idx = i
			break
		}
	}
	records := newRecords(a.out)
	if idx < 0 {
		records.write(errorRecord{Kind: "error", Session: s.ID(), Path: path,
			Error: fmt.Sprintf("no event at seq %d", seq)})
		if rerr != nil {
			records.write(errorFor(path, s, rerr))
		} else if s.Torn {
			records.write(warningFor(s, "torn final line ignored"))
		}
		return a.finish(records, exitNo)
	}
	lo, hi := max(0, idx-before), min(len(s.Events), idx+after+1)
	for _, e := range s.Events[lo:hi] {
		records.write(eventRecord{
			Kind: "event", Session: s.ID(), Path: s.Path, Event: e.Raw,
		})
	}
	if rerr != nil {
		records.write(errorFor(path, s, rerr))
		return a.finish(records, exitNo)
	}
	if s.Torn {
		records.write(warningFor(s, "torn final line ignored"))
	}
	return a.finish(records, exitYes)
}

func (a *app) cmdLineage(args []string) int {
	if len(args) < 1 || len(args) > 2 {
		return a.usage("trail lineage session [dir]")
	}
	path, err := a.resolveSession(args[0])
	if err != nil {
		return a.fail(err)
	}
	target, rerr := readSession(path)
	h, herr := target.header()
	if herr != nil {
		return a.fail(errors.Join(rerr, herr))
	}
	dir, explicit := "", false
	if len(args) == 2 {
		dir, explicit = args[1], true
	} else if filepath.IsAbs(args[0]) ||
		strings.ContainsRune(args[0], os.PathSeparator) {
		dir, explicit = filepath.Dir(path), true
	} else {
		dir = a.askDir()
	}
	files, err := archiveFiles(dir, explicit)
	if err != nil {
		return a.fail(err)
	}
	status, err := writeLineage(a.out, h.ID, files)
	if err != nil {
		return a.fail(fmt.Errorf("writing stdout: %w", err))
	}
	return status
}

func (a *app) cmdCheck(args []string) int {
	if len(args) > 1 {
		return a.usage("trail check [dir]")
	}
	dir, explicit := a.archiveArg(args)
	files, err := archiveFiles(dir, explicit)
	if err != nil {
		return a.fail(err)
	}
	if len(files) == 0 {
		return exitYes
	}
	ask := a.getenv("ASK")
	if ask == "" {
		ask = "ask"
	}
	if !strings.ContainsRune(ask, os.PathSeparator) {
		if _, err := exec.LookPath(ask); err != nil {
			return a.fail(fmt.Errorf("finding ASK %q: %w", ask, err))
		}
	}
	records := newRecords(a.out)
	bad := false
	for _, path := range files {
		cmd := exec.Command(ask, "replay", "-check", path)
		stdout, err := cmd.Output()
		if err == nil {
			records.write(checkRecord{Kind: "check", Session: fileID(path), Path: path,
				OK: true, Detail: strings.TrimSpace(string(stdout))})
			continue
		}
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return a.fail(fmt.Errorf("running %s: %w", ask, err))
		}
		detail := strings.TrimSpace(string(ee.Stderr))
		if detail == "" {
			detail = strings.TrimSpace(string(stdout))
		}
		records.write(checkRecord{Kind: "check", Session: fileID(path), Path: path,
			OK: false, Error: detail})
		bad = true
	}
	return a.finish(records, noIf(bad))
}

func (a *app) archiveArg(args []string) (string, bool) {
	if len(args) == 1 {
		return args[0], true
	}
	return a.askDir(), false
}

func (a *app) askDir() string {
	if dir := a.getenv("ASK_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".ask", "sessions")
	}
	return filepath.Join(home, ".ask", "sessions")
}

func (a *app) resolveSession(arg string) (string, error) {
	if info, err := os.Stat(arg); err == nil && info.Mode().IsRegular() {
		return arg, nil
	}
	path := filepath.Join(a.askDir(), strings.TrimSuffix(arg, ".jsonl")+".jsonl")
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return path, nil
	}
	return "", fmt.Errorf("no session %q in %s", arg, a.askDir())
}

func (a *app) usage(synopsis string) int {
	fmt.Fprintf(a.errOut, "usage: %s\n", synopsis)
	return exitErr
}

func (a *app) fail(err error) int {
	fmt.Fprintln(a.errOut, "trail:", err)
	return exitErr
}

type records struct {
	enc *json.Encoder
	err error
}

func newRecords(w io.Writer) *records {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &records{enc: enc}
}

func (r *records) write(value any) {
	if r.err == nil {
		r.err = r.enc.Encode(value)
	}
}

func (a *app) finish(r *records, status int) int {
	if r.err != nil {
		return a.fail(fmt.Errorf("writing stdout: %w", r.err))
	}
	return status
}

func noIf(no bool) int {
	if no {
		return exitNo
	}
	return exitYes
}

func parseWindowArgs(args []string) (
	before, after int,
	positional []string,
	err error,
) {
	before, after = 3, 3
	// flag stops at the first positional argument, while the review's public
	// synopsis permitted flags after seq. Scan once so both orders work.
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-before", "-after":
			if i+1 >= len(args) {
				return 0, 0, nil, errors.New("missing flag value")
			}
			n, convErr := strconv.Atoi(args[i+1])
			if convErr != nil || n < 0 {
				return 0, 0, nil, errors.New("window bounds must be non-negative integers")
			}
			if args[i] == "-before" {
				before = n
			} else {
				after = n
			}
			i++
		case "--":
			positional = append(positional, args[i+1:]...)
			return before, after, positional, nil
		default:
			if strings.HasPrefix(args[i], "-") {
				return 0, 0, nil, fmt.Errorf("unknown flag %s", args[i])
			}
			positional = append(positional, args[i])
		}
	}
	return before, after, positional, nil
}
