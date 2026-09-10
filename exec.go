// The wire format is a terminal session. A fenced shell block in the
// model's reply is a command; what it printed and what it exited come back
// as the next message, laid out the way a terminal would have shown it.
// That is the whole protocol, it needs no tool-use API, and it reads
// correctly to both of the things that have to read it.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Languages that mean "shell". The prompt tells the model that a fenced
// shell block is executed and that indenting is how you quote one without
// running it, so this list is generous on purpose: a model that reaches for
// ```sh out of habit must not have its work silently dropped.
var shellLangs = map[string]bool{
	"ply": true, "sh": true, "bash": true, "shell": true, "zsh": true,
}

// exitTimeout is timeout(1)'s number for a command it killed. Borrowed
// rather than invented, because it is already in everyone's fingers.
const exitTimeout = 124

const defaultShell = "/bin/sh"

// Result is one command's outcome.
type Result struct {
	Cmd                   string
	Output                string
	Code                  int
	Elided                int64 // bytes dropped from the middle of Output
	Total                 int64 // command output bytes, or the startup diagnostic size
	Killed                bool
	Interrupted           bool // caller canceled; even an exit 0 cannot establish completion
	OutputIncomplete      bool // inherited output pipes outlived the bounded drain
	StartError            bool // interpreter could not start; never an ordinary verifier rejection
	ConfinementFailed     bool // Cage could not establish or preserve the action boundary
	ConfinementMayHaveRun bool
	ConfinementDetail     string
	Timeout               time.Duration
}

// commands consumes at most one action from a reply. Optional prose may lead
// it; the first complete, nonempty shell block is the action. Everything after
// that block is deferred, because the model could not have seen the command's
// result when it wrote the rest. A reply with no shell block is a final report.
//
// note tells the model when no action could run, or when content after the
// first action was deliberately not applied. This is the observation boundary:
// even a model that emits a whole imagined workflow in one response gets one
// real result before it can choose the next action.
func commands(reply string) (cmds []string, prose, note string) {
	lines := strings.Split(reply, "\n")
	var kept []string
	for i := 0; i < len(lines); i++ {
		fence, lang, ok := openFence(lines[i])
		if !ok {
			kept = append(kept, lines[i])
			continue
		}
		body, end, closed := fenceBody(lines, i+1, fence)
		if !closed {
			if shellLangs[lang] {
				return nil, strings.TrimSpace(reply), fmt.Sprintf("the first command block was never closed, so nothing ran. Close it with a line of %d backticks and nothing else.", len(fence))
			}
			kept = append(kept, lines[i:]...)
			break
		}
		if !shellLangs[lang] {
			kept = append(kept, lines[i:end+1]...) // somebody else's code block; leave it whole
			i = end
			continue
		}

		command := strings.TrimSpace(body)
		if command == "" {
			return nil, strings.TrimSpace(reply), "the first command block was empty, so nothing ran. Send one complete command block or a report with no command block."
		}
		if strings.TrimSpace(strings.Join(lines[end+1:], "\n")) != "" {
			note = "ply ran the first command block successfully through the shell. Everything after it was deferred and did not run, because it was written before this command's result existed. The shell is available: read this result, then send the next required action as one new complete ply block; do not report a deferred block as unavailable."
		}
		return []string{command}, strings.TrimSpace(strings.Join(kept, "\n")), note
	}
	return nil, strings.TrimSpace(strings.Join(kept, "\n")), ""
}

// openFence reports whether a line opens a fenced block, and with what.
// Column zero is required: indenting is the documented way to write shell
// that is not run, and it is the only escape the format has.
func openFence(line string) (fence, lang string, ok bool) {
	n := 0
	for n < len(line) && line[n] == '`' {
		n++
	}
	if n < 3 {
		return "", "", false
	}
	info := strings.TrimSpace(line[n:])
	if strings.Contains(info, "`") {
		return "", "", false
	}
	lang, _, _ = strings.Cut(info, " ")
	return line[:n], strings.ToLower(lang), true
}

// fenceBody collects lines until a closing fence at least as long as the
// opening one, alone on its line.
func fenceBody(lines []string, start int, fence string) (body string, end int, closed bool) {
	for i := start; i < len(lines); i++ {
		if t := strings.TrimRight(lines[i], " \t"); strings.HasPrefix(t, fence) && strings.Trim(t, "`") == "" {
			return strings.Join(lines[start:i], "\n"), i, true
		}
	}
	return "", len(lines), false
}

// Runner holds what every command shares.
type Runner struct {
	Dir     string
	Path    string        // PATH the command runs with
	Shell   string        // resolved command interpreter; called with -c
	Timeout time.Duration // per command
	Cap     int           // bytes of output kept per command
	Env     []string      // extra NAME=VALUE, after the inherited environment
	Cage    *cageLauncher // model actions only; Checker always clears it
}

// Run executes one block as a shell script. Its stdin is the null device —
// os/exec's own default for a nil Stdin — because nothing is typing, and a
// command that waits for input would otherwise hang the loop until the
// timeout, once per turn, forever.
func (r Runner) Run(ctx context.Context, script string) Result {
	return r.run(ctx, script, nil)
}

// RunInput executes a verifier with finite input. Ply's final report is a
// text stream, just like stdout, so terminate it with one newline. The empty
// input used by the pre-check remains empty: before any work there is no
// candidate report to judge.
func (r Runner) RunInput(ctx context.Context, script, input string) Result {
	input = verifierInput(input)
	return r.run(ctx, script, strings.NewReader(input))
}

func verifierInput(input string) string {
	if input == "" {
		return ""
	}
	return strings.TrimRight(input, "\n") + "\n"
}

func (r Runner) run(ctx context.Context, script string, stdin io.Reader) Result {
	ctx, stop := context.WithTimeout(ctx, r.Timeout)
	defer stop()

	out := &capBuf{cap: max(r.Cap/2, 1)}
	argv := []string{r.Shell, "-c", script}
	if r.Cage != nil {
		if err := r.Cage.checkDigest(); err != nil {
			return Result{Cmd: script, Code: cageBoundaryExit,
				StartError: true, ConfinementFailed: true, ConfinementDetail: err.Error(), Timeout: r.Timeout}
		}
		argv = r.Cage.argv(r.Shell, script)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = r.Dir
	cmd.Stdin = stdin
	cmd.Stdout, cmd.Stderr = out, out // os/exec serializes writes to one writer
	cmd.Env = append(append(os.Environ(), "PATH="+r.Path), r.Env...)
	res := Result{Cmd: script, Timeout: r.Timeout}
	err := runCommand(ctx, cmd)
	res.Output, res.Elided, res.Total = out.String()
	res.Interrupted = errors.Is(ctx.Err(), context.Canceled)
	if r.Cage != nil {
		if digestErr := r.Cage.checkDigest(); digestErr != nil {
			res.Code, res.StartError, res.ConfinementFailed = cageBoundaryExit, true, true
			res.ConfinementMayHaveRun = true
			res.ConfinementDetail = digestErr.Error() + "; action effects may exist"
			return res
		}
	}
	var ee *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		res.Killed, res.Code = true, exitTimeout
	case err == nil:
	case errors.Is(err, exec.ErrWaitDelay):
		// The interpreter ran, but a descendant kept an output pipe open
		// past the drain bound. Keep the observed bytes and actual exit;
		// incomplete evidence cannot establish successful verification.
		res.OutputIncomplete = true
		if cmd.ProcessState != nil {
			res.Code = cmd.ProcessState.ExitCode()
		}
	case errors.As(err, &ee):
		res.Code = ee.ExitCode()
		if r.Cage != nil && res.Code == cageBoundaryExit {
			res.ConfinementFailed = true
			res.ConfinementMayHaveRun = true
			res.ConfinementDetail = "Cage returned reserved status 125; the child may have run"
		}
		if res.Code < 0 { // killed by a signal; report it the way a shell does
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
				res.Code = 128 + int(ws.Signal())
			} else {
				res.Code = 128
			}
		}
	case res.Interrupted:
		// CommandContext reports context.Canceled even when the program
		// catches the interrupt and exits 0. Keep that exit status while
		// retaining Interrupted as the separate completion boundary.
		if cmd.ProcessState == nil || !cmd.ProcessState.Success() {
			res.Code = 128 + int(syscall.SIGINT)
		}
	default:
		// The selected shell itself failed to start. That is ply's problem,
		// not the model's, but the model still has to see something.
		res.Output, res.Code, res.StartError = err.Error(), 1, true
		res.Total, res.Elided = int64(len(res.Output)), 0
		if r.Cage != nil {
			res.Code, res.ConfinementFailed = cageBoundaryExit, true
			res.ConfinementDetail = "Cage could not start; action did not run"
			res.Output, res.Total, res.Elided = "", 0, 0
		}
	}
	return res
}

// resolveShell turns the operator's interpreter choice into one executable
// before a model is called. A relative path is made absolute before Runner
// sets Cmd.Dir, and symlinks are deliberately left alone: shells may change
// their behaviour according to the name by which they were invoked.
func resolveShell(name string) (string, error) {
	return resolveShellFlag("-shell", name)
}

func resolveShellFlag(flagName, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%s: empty command interpreter", flagName)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s %q: not executable or not on PATH", flagName, name)
	}
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("%s %q: %w", flagName, name, err)
		}
	}
	return path, nil
}

// Typescript renders a result the way a terminal would have. The command
// echoes with the shell's own prompts, PS1 then PS2, so a multi-line script
// reads as one thing that was typed.
func (r Result) Typescript() string {
	var s strings.Builder
	for i, line := range strings.Split(r.Cmd, "\n") {
		if i == 0 {
			s.WriteString("$ " + line + "\n")
			continue
		}
		s.WriteString("> " + line + "\n")
	}
	if r.Output != "" {
		s.WriteString(r.Output)
		if !strings.HasSuffix(r.Output, "\n") {
			s.WriteString("\n")
		}
	}
	// Silence and success is what a shell shows: nothing. Anything else is
	// news, and news is worth a line.
	switch {
	case r.ConfinementFailed:
		s.WriteString("[ply: " + r.ConfinementDetail + "; stopped] exit " + strconv.Itoa(r.Code) + "\n")
	case r.StartError:
		s.WriteString("[ply: command interpreter could not start] exit " + strconv.Itoa(r.Code) + "\n")
	case r.Killed:
		s.WriteString("[ply: killed after " + r.Timeout.String() + "] exit " + strconv.Itoa(r.Code) + "\n")
	case r.Interrupted:
		s.WriteString("[ply: interrupted; command effects may exist] exit " + strconv.Itoa(r.Code) + "\n")
	case r.OutputIncomplete:
		s.WriteString("[ply: output pipes did not close; observed output is incomplete] exit " + strconv.Itoa(r.Code) + "\n")
	case r.Code != 0:
		s.WriteString("exit " + strconv.Itoa(r.Code) + "\n")
	case r.Output == "":
		s.WriteString("[ply: no output, exit 0]\n")
	}
	if r.Elided > 0 {
		s.WriteString(fmt.Sprintf("[ply: %d bytes of output, %d elided from the middle; narrow it with grep, head or tail]\n", r.Total, r.Elided))
	}
	return s.String()
}

// capBuf keeps the head and the tail of a stream and counts the rest. A
// build fails at the end and a listing matters at the start, so keeping one
// end would always be wrong for half of what a model runs. Memory is
// constant however much a command prints.
type capBuf struct {
	cap  int // per side
	head []byte
	tail []byte
	n    int64
}

func (b *capBuf) Write(p []byte) (int, error) {
	n := len(p)
	b.n += int64(n)
	if room := b.cap - len(b.head); room > 0 {
		take := min(room, len(p))
		b.head = append(b.head, p[:take]...)
		p = p[take:]
	}
	if len(p) > 0 {
		b.tail = append(b.tail, p...)
		if excess := len(b.tail) - b.cap; excess > 0 {
			b.tail = b.tail[:copy(b.tail, b.tail[excess:])]
		}
	}
	return n, nil
}

// String returns the kept output, how many bytes were dropped, and how many
// there were. The marker goes in the text the model reads, never only in a
// counter it cannot see: output that was cut and does not say so is the one
// failure nothing downstream can detect.
func (b *capBuf) String() (string, int64, int64) {
	elided := b.n - int64(len(b.head)) - int64(len(b.tail))
	if elided <= 0 {
		return string(b.head) + string(b.tail), 0, b.n
	}
	return string(b.head) + fmt.Sprintf("\n[ply: %d bytes elided]\n", elided) + string(b.tail), elided, b.n
}
