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
	Cmd     string
	Output  string
	Code    int
	Elided  int64 // bytes dropped from the middle of Output
	Total   int64 // bytes the command actually produced
	Killed  bool
	Timeout time.Duration
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
			note = "ply ran only the first command block. Everything after it was deferred and did not run, because it was written before this command's result existed. Read the result before choosing the next action or reporting what happened."
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
}

// Run executes one block as a shell script. Its stdin is the null device —
// os/exec's own default for a nil Stdin — because nothing is typing, and a
// command that waits for input would otherwise hang the loop until the
// timeout, once per turn, forever.
func (r Runner) Run(ctx context.Context, script string) Result {
	ctx, stop := context.WithTimeout(ctx, r.Timeout)
	defer stop()

	out := &capBuf{cap: max(r.Cap/2, 1)}
	cmd := exec.CommandContext(ctx, r.Shell, "-c", script)
	cmd.Dir = r.Dir
	cmd.Stdout, cmd.Stderr = out, out // os/exec serializes writes to one writer
	cmd.Env = append(append(os.Environ(), "PATH="+r.Path), r.Env...)
	// Its own process group, so a timeout reaches what the script started and
	// not just the shell that started it. Interrupt first: a nested Ply then
	// gets a chance to cancel the separate process groups it owns. Escalate
	// after a short grace period so an uncooperative descendant cannot keep a
	// pipe open forever.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	done := make(chan struct{})
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		go func(pid int) {
			select {
			case <-done:
			case <-time.After(750 * time.Millisecond):
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}(cmd.Process.Pid)
		return err
	}
	cmd.WaitDelay = 2 * time.Second

	res := Result{Cmd: script, Timeout: r.Timeout}
	err := cmd.Run()
	close(done)
	res.Output, res.Elided, res.Total = out.String()
	var ee *exec.ExitError
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		res.Killed, res.Code = true, exitTimeout
	case err == nil:
	case errors.As(err, &ee):
		res.Code = ee.ExitCode()
		if res.Code < 0 { // killed by a signal; report it the way a shell does
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
				res.Code = 128 + int(ws.Signal())
			} else {
				res.Code = 128
			}
		}
	default:
		// The selected shell itself failed to start. That is ply's problem,
		// not the model's, but the model still has to see something.
		res.Output, res.Code = err.Error(), 1
	}
	return res
}

// resolveShell turns the operator's interpreter choice into one executable
// before a model is called. A relative path is made absolute before Runner
// sets Cmd.Dir, and symlinks are deliberately left alone: shells may change
// their behaviour according to the name by which they were invoked.
func resolveShell(name string) (string, error) {
	if name == "" {
		return "", errors.New("-shell: empty command interpreter")
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("-shell %q: not executable or not on PATH", name)
	}
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("-shell %q: %w", name, err)
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
	case r.Killed:
		s.WriteString("[ply: killed after " + r.Timeout.String() + "] exit " + strconv.Itoa(r.Code) + "\n")
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
