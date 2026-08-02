// hone holds no provider code, no credentials and no session format, for
// the reason brief and ply hold none: the one thing it needs a model for is
// done by running the program that already does that.
//
// It runs `ask -n -f` into a session of its own under ~/.hone/, never the
// conversation you are having — a lesson landing in your current session
// would answer your next question with somebody else's typescript on the
// model's mind. The session is kept, not deleted: a lesson is a claim about
// how work is done here, and `ask replay -check` is what makes the claim
// reviewable a year later.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runAsk puts the evidence through a model and returns what it said, plus
// the id of the session that recorded the call.
func (g *hone) runAsk(ctx context.Context, evidence string) (answer, session string, err error) {
	dir, err := honeDir()
	if err != nil {
		return "", "", err
	}
	session = filepath.Join(dir, "find-"+stamp()+".jsonl")

	args := []string{"-q", "-n", "-f", session}
	if g.model != "" {
		args = append(args, "-m", g.model)
	}
	cmd := exec.CommandContext(ctx, g.askBin, args...)
	cmd.Stdin = strings.NewReader(evidence)
	// The system prompt goes in the environment, not argv: it is long, and
	// argv is world-readable in ps(1) on every machine this will ever run
	// on. The evidence goes on stdin for the same reason plus a better one
	// — a typescript does not respect ARG_MAX.
	cmd.Env = append(os.Environ(), "ASK_SYSTEM="+fmt.Sprintf(systemPrompt, g.max))
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 2 {
			return "", "", fmt.Errorf("the evidence is too large for %s: hone with a wider window (-m)", modelName(g.model))
		}
		return "", "", fmt.Errorf("%s: %s", g.askBin, firstLine(errb.String()))
	}
	return strings.TrimSpace(out.String()), idOf(session), nil
}

// describe writes the line a new skill will be found by, once, when it is
// created. See describePrompt for why this is not optional: brief ranks
// over descriptions and never over bodies, so a skill described as "what
// was learned here" is a skill nothing will ever retrieve.
//
// A failure here is not fatal. A lesson written down under a poor
// description is worth more than a lesson refused, and the description is
// one line in a file the author can edit.
func (g *hone) describe(ctx context.Context, name string, lessons []string) string {
	dir, err := honeDir()
	if err != nil {
		return ""
	}
	args := []string{"-q", "-n", "-f", filepath.Join(dir, "describe-"+stamp()+".jsonl")}
	if g.model != "" {
		args = append(args, "-m", g.model)
	}
	cmd := exec.CommandContext(ctx, g.askBin, args...)
	cmd.Stdin = strings.NewReader("SKILL NAME\n" + name + "\n\nFIRST LESSONS\n" + strings.Join(lessons, "\n"))
	cmd.Env = append(os.Environ(), "ASK_SYSTEM="+describePrompt)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	d := strings.TrimSpace(out.String())
	if i := strings.IndexByte(d, '\n'); i >= 0 {
		d = d[:i]
	}
	// The specification caps a description at 1024 characters and brief
	// lints it. Refusing an over-long one is cheaper than writing a skill
	// that will not load.
	if len(d) > 900 {
		return ""
	}
	return d
}

// verify asks ask whether the session is a record of what happened. A log
// that does not replay is not evidence, and learning from one is the
// failure this whole program is arranged to avoid, arrived at from the
// least interesting direction.
func verify(ctx context.Context, askBin, path string) error {
	cmd := exec.CommandContext(ctx, askBin, "replay", "-check", path)
	var errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = nil, &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s does not replay: %s", filepath.Base(path), firstLine(errb.String()))
	}
	return nil
}

// skillDir decides where a lesson goes: an existing skill of that name, or
// a new one in the first place brief would look.
//
// $BRIEF_PATH is searched here rather than by running `brief path`, and the
// distinction is worth stating. brief owns the skill *format* -- the
// frontmatter, the specification's limits, the lint -- and hone does not
// have a second copy of any of it. $BRIEF_PATH is not a format; it is an
// environment contract like $PATH, and a program that respects $PATH looks
// things up on it rather than shelling out to ask something else where a
// binary is. Doing it here is also what keeps brief optional: without it
// hone writes a skill it cannot lint, which is a narrower hone rather
// than no hone at all.
//
// A name that is already a path is left alone. That is how you write into a
// skill that is not installed anywhere yet.
func skillDir(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) || name == "." || name == ".." {
		return name, nil
	}
	if err := validName(name); err != nil {
		return "", err
	}
	entries := briefPath()
	for _, e := range entries {
		dir := filepath.Join(e, name)
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			return dir, nil // first match wins, the way a path does
		}
	}
	return filepath.Join(entries[0], name), nil
}

// briefPath is $BRIEF_PATH, with brief's default when it is unset: a skill
// in the project shadows the one in your home directory, the same way
// ./bin/foo shadows /usr/bin/foo.
func briefPath() []string {
	if p := os.Getenv("BRIEF_PATH"); p != "" {
		var out []string
		for _, e := range strings.Split(p, string(os.PathListSeparator)) {
			if e != "" {
				out = append(out, e)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{filepath.Join(".claude", "skills")}
	}
	return []string{
		filepath.Join(".claude", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".brief", "skills"),
	}
}

// validName keeps a name a name. -into is often typed from a script, and a
// name is about to become a path: "../../etc" must not be one, and a name
// the specification will not load is a skill that silently never appears.
func validName(name string) error {
	if name == "" {
		return errors.New("a skill needs a name")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return fmt.Errorf("%q is not a skill name: lowercase letters, digits, - and _ (a path with a %c in it names a directory instead)", name, filepath.Separator)
		}
	}
	return nil
}

// lint is hone checking its own work with the program that owns the
// format. Done is a program's opinion, and that applies to what hone
// writes as much as to what ply verifies.
func lint(ctx context.Context, briefBin, dir string) error {
	if briefBin == "" {
		return nil
	}
	var errb bytes.Buffer
	cmd := exec.CommandContext(ctx, briefBin, "lint", dir)
	cmd.Stdout, cmd.Stderr = nil, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return fmt.Errorf("%s", firstLine(errb.String()))
	}
	return nil
}

// tool resolves a program hone depends on, naming what to install rather
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

// optional resolves a program hone can do without, so a missing one is a
// narrower hone rather than a failure.
func optional(env, dflt string) string {
	bin, err := tool(env, dflt, "")
	if err != nil {
		return ""
	}
	return bin
}

func honeDir() (string, error) {
	if d := os.Getenv("HONE_DIR"); d != "" {
		return d, os.MkdirAll(d, 0o700)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(home, ".hone", "lessons")
	return d, os.MkdirAll(d, 0o700)
}

func idOf(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".jsonl")
}

func modelName(spec string) string {
	if spec == "" {
		return "the model"
	}
	return spec
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
