// ply has no provider code, no credentials and no session format, for the
// reason brief has none: the one thing it needs a model for is done by
// running the program that already does that. Five providers, gateway and
// subscription auth, reasoning round-trips, retries and the replay
// invariant all arrive from ask, and stay ask's problem.
package main

import (
	"bytes"
	"context"
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
	Bin       string     // the ask binary
	Session   string     // -f: a thread of ply's own, never the caller's current one
	Spec      string     // -m, empty to let ask decide
	Effort    string     // -effort, empty to let ask and the provider decide
	Verbosity string     // -verbosity, empty to let ask and the provider decide
	System    string     // -S, sent every turn so the log says what shaped it
	Progress  io.Writer  // optional live Ask progress; never the answer stream
	Recording *recording // explicit public compaction artifact handoff
}

// Turn sends text and returns the model's reply. The text goes on stdin
// rather than argv: a turn carries a command's output, and output does not
// respect ARG_MAX.
func (m Model) Turn(ctx context.Context, text string) (string, error) {
	args := []string{"-q", "-f", m.Session}
	if m.Progress != nil {
		args = args[1:]
	}
	if m.Spec != "" {
		args = append(args, "-m", m.Spec)
	}
	if m.Effort != "" {
		args = append(args, "-effort", m.Effort)
	}
	if m.Verbosity != "" {
		args = append(args, "-verbosity", m.Verbosity)
	}
	cmd := exec.CommandContext(ctx, m.Bin, args...)
	cmd.Stdin = strings.NewReader(text)
	// The system prompt goes in the environment, not argv. It is long, it
	// can carry a private procedure that brief just printed, and argv is
	// world-readable in ps(1) on every machine this will ever run on.
	cmd.Env = append(os.Environ(), "ASK_SYSTEM="+m.System)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if m.Progress != nil {
		cmd.Stderr = io.MultiWriter(&errb, m.Progress)
	}
	err := runCommand(ctx, cmd)
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
	return m.CompactAt(ctx, 0)
}

// CompactAt asks Ask to measure context and compact only at the explicit
// token threshold. Zero means unconditional recovery after an overflow.
func (m Model) CompactAt(ctx context.Context, at int) (string, error) {
	args := []string{"compact", "-q"}
	if m.Recording != nil {
		args = append(args, "-json")
	}
	if m.Spec != "" {
		args = append(args, "-m", m.Spec)
	}
	if m.Verbosity != "" {
		args = append(args, "-verbosity", m.Verbosity)
	}
	if at > 0 {
		args = append(args, "-at", strconv.Itoa(at))
	}
	args = append(args, m.Session)
	cmd := exec.CommandContext(ctx, m.Bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := runCommand(ctx, cmd); err != nil {
		return "", fmt.Errorf("%s compact: %s", m.Bin, firstLine(errb.String()))
	}
	path := strings.TrimSpace(out.String())
	if m.Recording != nil {
		var handoff struct{ Source, Summary, Session string }
		if err := json.Unmarshal(out.Bytes(), &handoff); err != nil || !sameSession(handoff.Source, m.Session) {
			return "", fmt.Errorf("%w: invalid Ask compact JSON handoff", ErrRecording)
		}
		path = handoff.Session
		if handoff.Summary != "" {
			if !filepath.IsAbs(handoff.Summary) || filepath.Clean(handoff.Summary) != handoff.Summary || filepath.Dir(handoff.Summary) != filepath.Dir(path) {
				return "", fmt.Errorf("%w: invalid compaction summary path", ErrRecording)
			}
			m.Recording.Sessions = append(m.Recording.Sessions, handoff.Summary)
			if err := m.Recording.note("summary", map[string]any{"source": handoff.Source, "path": handoff.Summary, "session": path}); err != nil {
				return "", err
			}
		} else if !sameSession(path, m.Session) {
			return "", fmt.Errorf("%w: compaction omitted its summary", ErrRecording)
		}
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\r\n\x00") {
		return "", fmt.Errorf("%s compact: expected one clean absolute session path", m.Bin)
	}
	info, err := os.Lstat(path)
	if at > 0 && sameSession(path, m.Session) {
		// Ask may return the unchanged source below its threshold. Preserve
		// an explicitly selected source symlink; fresh handoffs stay regular.
		info, err = os.Stat(path)
	}
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s compact: returned session is not a regular file: %s", m.Bin, path)
	}
	if at == 0 && sameSession(path, m.Session) {
		return "", fmt.Errorf("%s compact: returned the full source session", m.Bin)
	}
	verify := exec.CommandContext(ctx, m.Bin, "replay", "-check", path)
	verify.Stderr = &errb
	if err := runCommand(ctx, verify); err != nil {
		return "", fmt.Errorf("%s compact: returned session did not replay: %s", m.Bin, firstLine(errb.String()))
	}
	return path, nil
}

func sameSession(a, b string) bool {
	aa, ae := filepath.Abs(a)
	bb, be := filepath.Abs(b)
	if ae == nil && be == nil && aa == bb {
		return true
	}
	left, le := os.Stat(a)
	right, re := os.Stat(b)
	return le == nil && re == nil && os.SameFile(left, right)
}

// Append durably records an observed result in the provider-visible history
// without making another model call. Ask owns locking, attribution and seals.
func (m Model) Append(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, m.Bin, "append", "-q", "-s", verdictSource, "-f", m.Session)
	cmd.Stdin = strings.NewReader(text)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := runCommand(ctx, cmd); err != nil {
		return fmt.Errorf("%s append observation: %s", m.Bin, firstLine(errb.String()))
	}
	return nil
}

// Note records human-readable composition metadata such as loaded skill
// names. Executable evidence uses Record below and is not best effort.
func (m Model) Note(ctx context.Context, source, text string) error {
	cmd := exec.CommandContext(ctx, m.Bin, "note", "-q", "-s", source, "-f", m.Session)
	cmd.Stdin = strings.NewReader(text)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := runCommand(ctx, cmd); err != nil {
		return fmt.Errorf("%s note: %s", m.Bin, firstLine(errb.String()))
	}
	return nil
}

// Record appends a typed JSON record and requires Ask to durably seal it.
// Unlike a prose note, this is part of Ply's executable evidence boundary:
// success is not reported until Ask confirms the record and its prefix seal.
func (m Model) Record(ctx context.Context, source, kind string, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal %s record: %w", kind, err)
	}
	cmd := exec.CommandContext(ctx, m.Bin, "note", "-q", "-s", source, "-f", m.Session,
		"-k", kind, "-json", "-", "-seal")
	cmd.Stdin = bytes.NewReader(raw)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := runCommand(ctx, cmd); err != nil {
		return fmt.Errorf("%s sealed note: %s", m.Bin, firstLine(errb.String()))
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
	if err := runCommand(ctx, cmd); err != nil {
		return "", fmt.Errorf("%s cat %s: %s", bin, name, firstLine(errb.String()))
	}
	return out.String(), nil
}

// briefFind is `-s -`: let the catalogue pick. Try Brief's deterministic,
// offline ranker first and spend a selector model call only when it reports no
// match. Brief refuses to guess, and exit 1 from both stages means nothing
// matched — which is an answer, not a failure, so the run goes on without a
// skill and stderr says so.
func briefFind(ctx context.Context, bin, task string) (string, string, error) {
	name, evidence, err := runBriefFind(ctx, bin, []string{"find", "-q"}, task)
	if err == nil {
		return name, "brief: deterministic catalogue match " + name, nil
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 1 {
		return "", "", fmt.Errorf("%s find: %s", bin, firstLine(evidence))
	}

	name, evidence, err = runBriefFind(ctx, bin, []string{"find", "-ask"}, task)
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return "", strings.TrimSpace(evidence), nil
	}
	if err != nil {
		return "", "", fmt.Errorf("%s find -ask: %s", bin, firstLine(evidence))
	}
	return name, strings.TrimSpace(evidence), nil
}

func runBriefFind(ctx context.Context, bin string, args []string, task string) (string, string, error) {
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(task)
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := runCommand(ctx, cmd)
	return strings.TrimSpace(out.String()), strings.TrimSpace(errb.String()), err
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
