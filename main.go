// Command may asks a human whether one exact action may proceed.
//
// The action is stdin. With no job, the decision comes from /dev/tty. With a
// job, the request is parked as a file until a human runs may decide.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const version = "0.1.0"

const (
	exitYes    = 0
	exitCheck  = 1
	exitErr    = 2
	exitNo     = 3
	exitParked = 75
	maxAction  = 16 * 1024
)

const usageText = `may - ask a human before one exact action

  printf '%s\n' ACTION | may [JOB]  approve now, or park under JOB
  may pending                       print pending requests as JSONL
  may decide DIGEST                 decide one request at /dev/tty
  may check                         run the offline acceptance check
  may version                       print the version (-V, --version)
  may help                          print this summary (-h, --help)

The action is the exact bytes on stdin and never argv. Without JOB, may asks
y/N on /dev/tty. With JOB, may consumes a matching single-use grant or records
a pending request and exits 75. The decide command is the human side of that
file handoff and also reads only /dev/tty.

State is under ~/.local/state/may. Audit is append-only JSONL. Keep the binary
and state outside model toolboxes and writable sandboxes. There is no flag,
environment variable, config file, or classifier that bypasses the human.

exit: 0 approved - 3 refused/no human - 75 parked - 2 usage/I/O - 1 check
`

type readWriteCloser interface {
	io.Reader
	io.Writer
	io.Closer
}

type app struct {
	in       io.Reader
	out      io.Writer
	errOut   io.Writer
	stateDir string
	now      func() time.Time
	openTTY  func() (readWriteCloser, error)
}

func main() {
	os.Exit(newApp().run(os.Args[1:]))
}

func newApp() *app {
	home := ""
	if current, err := user.Current(); err == nil {
		home = current.HomeDir
	}
	stateDir := ""
	if home != "" {
		stateDir = filepath.Join(home, ".local", "state", "may")
	}
	return &app{
		in:       os.Stdin,
		out:      os.Stdout,
		errOut:   os.Stderr,
		stateDir: stateDir,
		now:      time.Now,
		openTTY:  openControllingTTY,
	}
}

func (a *app) run(args []string) int {
	forceJob := false
	if len(args) > 0 && args[0] == "--" {
		forceJob = true
		args = args[1:]
	}
	if !forceJob && len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			if len(args) != 1 {
				return a.fail(exitErr, errors.New("usage: may help"))
			}
			return a.write([]byte(usageText))
		case "version", "-V", "--version":
			if len(args) != 1 {
				return a.fail(exitErr, errors.New("usage: may version"))
			}
			return a.write([]byte(fmt.Sprintf("may %s\n", version)))
		case "pending":
			if len(args) != 1 {
				return a.fail(exitErr, errors.New("usage: may pending"))
			}
			return a.pending()
		case "decide":
			if len(args) != 2 {
				return a.fail(exitErr, errors.New("usage: may decide DIGEST"))
			}
			return a.decide(args[1])
		case "check":
			if len(args) != 1 {
				return a.fail(exitErr, errors.New("usage: may check"))
			}
			if err := selfCheck(a.out); err != nil {
				return a.fail(exitCheck, fmt.Errorf("check: %w", err))
			}
			return exitYes
		}
	}
	if len(args) > 1 {
		return a.fail(exitErr, errors.New("usage: may [JOB]"))
	}
	if forceJob && len(args) != 1 {
		return a.fail(exitErr, errors.New("usage: may -- JOB"))
	}
	job := ""
	if len(args) == 1 {
		if !forceJob && strings.HasPrefix(args[0], "-") {
			return a.fail(exitErr, fmt.Errorf("unknown option %q", args[0]))
		}
		job = args[0]
		if !utf8.ValidString(job) || strings.TrimSpace(job) == "" {
			return a.fail(exitErr, errors.New("JOB must be non-empty UTF-8"))
		}
	}
	action, err := readAction(a.in)
	if err != nil {
		return a.fail(exitErr, err)
	}
	if job == "" {
		return a.atTerminal(action)
	}
	return a.forJob(job, action)
}

func readAction(r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxAction+1))
	if err != nil {
		return "", fmt.Errorf("read action: %w", err)
	}
	if len(data) > maxAction {
		return "", fmt.Errorf("action exceeds %d bytes", maxAction)
	}
	if !utf8.Valid(data) {
		return "", errors.New("action is not valid UTF-8")
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", errors.New("action contains a NUL byte")
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", errors.New("action is empty")
	}
	return string(data), nil
}

func actionDigest(job, action string) string {
	h := sha256.New()
	_, _ = io.WriteString(h, "may-v1\x00")
	_, _ = io.WriteString(h, job)
	_, _ = io.WriteString(h, "\x00")
	_, _ = io.WriteString(h, action)
	return hex.EncodeToString(h.Sum(nil))
}

func validDigest(s string) bool {
	if len(s) != sha256.Size*2 || strings.ToLower(s) != s {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func (a *app) atTerminal(action string) int {
	if err := a.ensureState(); err != nil {
		return a.fail(exitErr, err)
	}
	digest := actionDigest("", action)
	tty, err := a.openTTY()
	if err != nil {
		if auditErr := a.audit("unavailable", digest, "", action); auditErr != nil {
			return a.fail(exitErr, auditErr)
		}
		return a.fail(exitNo, errors.New("no controlling terminal; action refused"))
	}
	approved, askErr := askHuman(tty, request{Digest: digest, Action: action})
	if askErr != nil {
		return a.fail(exitErr, askErr)
	}
	verdict := "declined"
	code := exitNo
	if approved {
		verdict = "approved"
		code = exitYes
	}
	if err := a.audit(verdict, digest, "", action); err != nil {
		return a.fail(exitErr, err)
	}
	if !approved {
		fmt.Fprintln(a.errOut, "may: declined")
	}
	return code
}

func askHuman(tty readWriteCloser, req request) (bool, error) {
	defer tty.Close()
	if _, err := fmt.Fprintf(tty,
		"may: job %s\nmay: action %s\nmay: digest %s\nmay: allow exactly this action? [y/N] ",
		quotedJob(req.Job), strconv.Quote(req.Action), req.Digest); err != nil {
		return false, fmt.Errorf("write controlling terminal: %w", err)
	}
	answer, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read controlling terminal: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func quotedJob(job string) string {
	if job == "" {
		return "(interactive)"
	}
	return strconv.Quote(job)
}

func (a *app) fail(code int, err error) int {
	fmt.Fprintf(a.errOut, "may: %v\n", err)
	return code
}

func (a *app) write(body []byte) int {
	n, err := a.out.Write(body)
	if err == nil && n != len(body) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return a.fail(exitErr, fmt.Errorf("write stdout: %w", err))
	}
	return exitYes
}

func sortedJSONFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("pending entry %q is not a regular file",
				filepath.Join(dir, entry.Name()))
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}
