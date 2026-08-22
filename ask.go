// ply has no provider code, no credentials and no session format, for the
// reason brief has none: the one thing it needs a model for is done by
// running the program that already does that. Five providers, gateway and
// subscription auth, reasoning round-trips, retries and the replay
// invariant all arrive from ask, and stay ask's problem.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Outcomes that are not infrastructure failures and must not be retried or
// reported as breakage.
var (
	ErrOverflow = errors.New("context window is full")
	ErrCycles   = errors.New("the check never passed")
	ErrTurns    = errors.New("turn limit reached")
	ErrProtocol = errors.New("the command protocol stalled")
)

// ErrCheck is infrastructure failure, not an ordinary rejected candidate.
// Main therefore maps it to exit 1 rather than Ply's exit-2 "not done".
var ErrCheck = errors.New("the check is broken")

// Model is one conversation, held in an ask session file. Every turn is one
// `ask` process appending to it, so the file is the whole record: the
// commands are in the assistant messages, their output is in the user
// messages, and `ask replay -check` proves the run.
type Model struct {
	Bin     string // the ask binary
	Session string // -f: a thread of ply's own, never the caller's current one
	Spec    string // -m, empty to let ask decide
	Effort  string // -effort, empty to let ask and the provider decide
	System  string // -S, sent every turn so the log says what shaped it
}

// Turn sends text and returns the model's reply. The text goes on stdin
// rather than argv: a turn carries a command's output, and output does not
// respect ARG_MAX.
func (m Model) Turn(ctx context.Context, text string) (string, error) {
	args := []string{"-q", "-f", m.Session}
	if m.Spec != "" {
		args = append(args, "-m", m.Spec)
	}
	if m.Effort != "" {
		args = append(args, "-effort", m.Effort)
	}
	cmd := exec.CommandContext(ctx, m.Bin, args...)
	cmd.Stdin = strings.NewReader(text)
	// The system prompt goes in the environment, not argv. It is long, it
	// can carry a private procedure that brief just printed, and argv is
	// world-readable in ps(1) on every machine this will ever run on.
	cmd.Env = append(os.Environ(), "ASK_SYSTEM="+m.System)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		// ask's exit 2 is the whole reason it has one: a full context window
		// fails identically forever, so the loop stops instead of paying for
		// the same permanent error until somebody notices.
		if ee.ExitCode() == 2 {
			return "", ErrOverflow
		}
		return "", fmt.Errorf("%s: %s", m.Bin, firstLine(errb.String()))
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", m.Bin, err)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// Compact starts a fresh session from a model-written handoff note and
// returns its path. It is what a full context window costs now: one
// summarizing call and a conversation that carries the work instead of the
// transcript. ask owns the mechanism, as it owns the log — ply only knows
// when to reach for it.
func (m Model) Compact(ctx context.Context) (string, error) {
	args := []string{"-q", "compact", m.Session}
	if m.Spec != "" {
		args = append(args, "-m", m.Spec)
	}
	cmd := exec.CommandContext(ctx, m.Bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s compact: %s", m.Bin, firstLine(errb.String()))
	}
	path := strings.TrimSpace(out.String())
	if path == "" {
		return "", fmt.Errorf("%s compact: no session on stdout", m.Bin)
	}
	return path, nil
}

// Note records the check's verdict in the conversation, which is where ply
// has always said everything worth recording goes. It did not go there: the
// verdict lived on stderr and in an exit status, so a session held every
// command that ran and nothing about whether the work was done, and a run
// that passed and a run that gave up were the same shape on disk.
//
// It is a note rather than a message because of who it is for. A failing
// check becomes a user message — the model has to act on it, and does. A
// passing check is addressed to nobody, because the run is over; it is a
// record for whoever reads the session later, which includes hone(1),
// which refuses to learn from a run that will not say how it ended.
//
// Best effort, and deliberately so: a run that did the work and then could
// not write a line about it did the work. The failure is worth a word on
// stderr and nothing more.
func (m Model) Note(ctx context.Context, source, text string) error {
	cmd := exec.CommandContext(ctx, m.Bin, "note", "-q", "-s", source, "-f", m.Session)
	cmd.Stdin = strings.NewReader(text)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s note: %s", m.Bin, firstLine(errb.String()))
	}
	return nil
}

// brief runs the catalogue. ply knows a skill by name and nothing else
// about it; if ply needs a procedure, it runs brief, exactly as brief runs
// ask when it needs a model.
func briefCat(ctx context.Context, bin, name string) (string, error) {
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "cat", name)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s cat %s: %s", bin, name, firstLine(errb.String()))
	}
	return out.String(), nil
}

// briefFind is `-s -`: let the catalogue pick. brief refuses to guess, and
// exit 1 means nothing matched — which is an answer, not a failure, so the
// run goes on without a skill and stderr says so.
func briefFind(ctx context.Context, bin, task string) (string, error) {
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "find", "-ask", "-q", task)
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s find: %s", bin, firstLine(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// tool resolves a program ply depends on, naming what to install rather
// than reporting a bare ENOENT from three frames down.
func tool(env, dflt, why string) (string, error) {
	bin := dflt
	if v := os.Getenv(env); v != "" {
		bin = v
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("%s: not on PATH (%s; $%s names another)", bin, why, env)
	}
	return path, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "failed with no diagnostic"
	}
	return s
}
