// ply works a goal with a toolbox until a check says it is done.
//
// ask is a model and no loop. brief is a procedure and no model. ply is the
// loop, and it refuses to grow the other two: it has no provider code, no
// credentials, no session format and no catalogue. When it needs a model it
// runs ask; when it needs a procedure it runs brief; and a tool is a
// program, so the toolbox is $PATH.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"

const (
	maxStdin  = 16 << 20 // as much as ask will carry in one message
	spoolOver = 64 << 10 // past this, stdin becomes a file to grep
	maxDepth  = 8        // ply inside ply inside ply: a fork bomb that bills
)

const synopsis = "ply [flags] <goal> | ply tools | ply system | ply version | ply help"

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "tools":
			return toolsCmd(args[1:])
		case "system":
			return systemCmd(args[1:])
		case "version", "-V", "--version":
			fmt.Println("ply " + version)
			return 0
		case "help", "-h", "--help":
			fmt.Print(help)
			return 0
		}
	}
	return work(args)
}

// opts is every knob, in one place, because three verbs share most of them.
type opts struct {
	fs       *flag.FlagSet
	toolbox  *string
	shell    *bool
	check    *string
	force    *bool
	cycles   *int
	turns    *int
	timeout  *time.Duration
	outcap   *int
	dir      *string
	spec     *string
	sys      *string
	skills   list
	file     *string
	quiet    *bool
	compact  *bool
	compacts *int
}

func newOpts(name string) *opts {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o := &opts{
		fs:       fs,
		toolbox:  fs.String("t", os.Getenv("PLY_TOOLS"), "toolbox directory; PATH becomes this alone"),
		shell:    fs.Bool("sh", false, "hand the model every program on PATH"),
		check:    fs.String("check", "", "the goal is done when this shell command exits 0"),
		force:    fs.Bool("B", false, "work the goal even if the check already passes"),
		cycles:   fs.Int("cycles", 5, "failed checks before giving up (0 = unbounded)"),
		turns:    fs.Int("turns", 0, "model turns before giving up (0 = unbounded)"),
		timeout:  fs.Duration("timeout", 2*time.Minute, "per-command timeout"),
		outcap:   fs.Int("cap", 16<<10, "output kept per command, head and tail"),
		dir:      fs.String("C", "", "run commands here"),
		spec:     fs.String("m", "", "provider/model, passed to ask"),
		sys:      fs.String("S", "", "system prompt, replacing the default"),
		file:     fs.String("f", "", "session log to write"),
		quiet:    fs.Bool("q", false, "no typescript on stderr"),
		compact:  fs.Bool("compact", false, "carry on through a full context window"),
		compacts: fs.Int("compactions", 3, "compactions before giving up (0 = unbounded)"),
	}
	fs.Var(&o.skills, "s", "brief skill to append; repeat for more; - picks one")
	return o
}

// box and runner are shared by work, tools and system so that what those
// two print is what work would actually send and run, never a second
// rendering that can drift from it.
func (o *opts) box() (*Box, error) {
	if *o.toolbox == "" && !*o.shell {
		return nil, errors.New("no tools: -t dir gives the model a toolbox (PATH becomes\nthat directory alone), -sh gives it the whole machine")
	}
	return openBox(*o.toolbox, *o.shell)
}

func (o *opts) runner(b *Box, self string, depth int) Runner {
	return Runner{
		Dir:     *o.dir,
		Path:    b.Path(),
		Timeout: *o.timeout,
		Cap:     *o.outcap,
		// A tool that starts another ply is how fan-out, specialists and
		// teams happen here: a program, not a feature. The depth counter is
		// the only thing standing between that and a bill.
		Env: []string{"PLY=" + self, "PLY_DEPTH=" + strconv.Itoa(depth+1)},
	}
}

// checker is that runner with the caller's reach. Everything about running
// a command is shared; only the PATH differs, and it differs because the
// check belongs to whoever typed it.
func (o *opts) checker(r Runner, b *Box) Runner {
	r.Path = b.CheckPath()
	return r
}

func work(args []string) int {
	o := newOpts("ply")
	if err := o.fs.Parse(args); err != nil {
		return usage(err)
	}
	goal := strings.Join(o.fs.Args(), " ")

	depth, err := descend()
	if err != nil {
		return fail(err)
	}
	box, err := o.box()
	if err != nil {
		return fail(err)
	}
	askBin, err := tool("ASK", "ask", "ply runs ask for the model: go install github.com/patrickyoung/ask@latest")
	if err != nil {
		return fail(err)
	}
	if *o.dir != "" {
		if fi, err := os.Stat(*o.dir); err != nil || !fi.IsDir() {
			return fail(fmt.Errorf("-C %s: not a directory", *o.dir))
		}
	}
	if *o.outcap < 512 {
		return fail(fmt.Errorf("-cap %d: too small to be worth reading", *o.outcap))
	}

	data, err := stdinData(*o.quiet)
	if err != nil {
		return fail(err)
	}
	if goal == "" {
		// Piped input alone is the goal, exactly as it is for ask and mu.
		if goal = strings.TrimSpace(string(data)); goal == "" {
			return usage(errors.New("no goal"))
		}
		data = nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// -S replaces the default, and -S "" sends none, as it does for ask.
	// The protocol lives in the default, so replacing it is a real choice:
	// `ply system` prints what you would be dropping, and the manual says
	// to compose with it rather than around it.
	system := prompt(box, *o.dir, *o.check, *o.timeout, *o.outcap)
	o.fs.Visit(func(f *flag.Flag) {
		if f.Name == "S" {
			system = *o.sys
		}
	})
	v := newView(os.Stderr, *o.quiet)
	if len(o.skills) > 0 {
		s, err := brief(ctx, o.skills, goal, v)
		if err != nil {
			return fail(err)
		}
		system += s
	}

	self, err := os.Executable()
	if err != nil {
		self = "ply"
	}
	runner := o.runner(box, self, depth)
	checker := o.checker(runner, box)

	// make's "nothing to be done": a goal already met costs nothing, leaves
	// no session behind, and is safe to put in a hook or a Makefile.
	if *o.check != "" && !*o.force {
		if r := checker.Run(ctx, *o.check); r.Code == 0 {
			v.Check(r)
			v.Note("nothing to do")
			return 0
		} else if ctx.Err() != nil {
			return 130
		}
	}

	// Everything that can fail has failed by here. Only now does ply put a
	// file on the disk: a bad invocation leaves no litter.
	session := *o.file
	if session == "" {
		if session, err = mint(); err != nil {
			return fail(err)
		}
	}
	first, err := spool(goal, data, session)
	if err != nil {
		return fail(err)
	}

	v.Note("%s · %s", session, describe(box, *o.check))
	if underTree(session, *o.dir) {
		v.Note("the session is inside the work tree, so a grep or a find will\n" +
			"     read it back into the conversation it is a record of; -f a path\n" +
			"     outside the tree keeps the record out of the work")
	}
	loop := &Loop{
		Model:    Model{Bin: askBin, Session: session, Spec: *o.spec, System: system},
		Runner:   runner,
		Checker:  checker,
		Check:    *o.check,
		Cycles:   *o.cycles,
		Compact:  *o.compact,
		Compacts: *o.compacts,
		Turns:    *o.turns,
		View:     v,
	}
	answer, err := loop.Run(ctx, first)
	if answer != "" && !v.Shown() {
		fmt.Println(strings.TrimRight(answer, "\n"))
	}
	switch {
	case err == nil:
		return 0
	case errors.Is(err, context.Canceled):
		v.Note("interrupted")
		return 130
	case errors.Is(err, ErrCycles), errors.Is(err, ErrTurns), errors.Is(err, ErrOverflow):
		fmt.Fprintf(os.Stderr, "ply: %v\n", err)
		return 2
	default:
		return fail(err)
	}
}

func toolsCmd(args []string) int {
	o := newOpts("ply tools")
	if err := o.fs.Parse(args); err != nil {
		return usage(err)
	}
	box, err := o.box()
	if err != nil {
		return fail(err)
	}
	fmt.Print(box.Catalogue())
	fmt.Printf("\nPATH=%s\n", box.Path())
	return 0
}

// systemCmd prints what would be sent, which has to include the skills or
// it is a different prompt with the same name. A goal may follow the flags,
// because `-s -` asks brief to choose one for a goal.
func systemCmd(args []string) int {
	o := newOpts("ply system")
	if err := o.fs.Parse(args); err != nil {
		return usage(err)
	}
	box, err := o.box()
	if err != nil {
		return fail(err)
	}
	out := prompt(box, *o.dir, *o.check, *o.timeout, *o.outcap)
	if len(o.skills) > 0 {
		s, err := brief(context.Background(), o.skills, strings.Join(o.fs.Args(), " "), newView(os.Stderr, false))
		if err != nil {
			return fail(err)
		}
		out += s
	}
	fmt.Print(out)
	return 0
}

// brief loads procedures. `-s -` asks the catalogue to choose, and brief
// refuses to guess: nothing matched is an answer, so the run goes on
// without one and stderr says so.
func brief(ctx context.Context, names list, goal string, v *view) (string, error) {
	bin, err := tool("BRIEF", "brief", "-s needs brief: go install github.com/patrickyoung/brief@latest")
	if err != nil {
		return "", err
	}
	var s strings.Builder
	for _, name := range names {
		if name == "-" {
			if name, err = briefFind(ctx, bin, goal); err != nil {
				return "", err
			}
			if name == "" {
				v.Note("brief matched no skill for this goal; continuing without one")
				continue
			}
			v.Note("brief chose %s", name)
		}
		body, err := briefCat(ctx, bin, name)
		if err != nil {
			return "", err
		}
		s.WriteString("\n\nThe procedure below applies to this goal. Follow it.\n\n" + strings.TrimSpace(body) + "\n")
	}
	return s.String(), nil
}

// spool decides what the first message says. Input past spoolOver becomes a
// file the model greps instead of a tax on every request for the rest of
// the run, and it is never silently dropped: too much is an error naming
// the limit, because an answer about the first part of a file presented as
// an answer about the file is what nothing downstream can detect.
func spool(goal string, data []byte, session string) (string, error) {
	if len(data) <= spoolOver {
		return firstMessage(goal, strings.TrimSpace(string(data)), ""), nil
	}
	path := strings.TrimSuffix(session, ".jsonl") + ".stdin"
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("spooling stdin: %w", err)
	}
	return firstMessage(goal, "", path), nil
}

func stdinData(quiet bool) ([]byte, error) {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		return nil, nil
	}
	// A filter reads its input, and an input nobody is writing blocks until
	// somebody does. That is the shell's contract and not a bug — but a
	// silent indefinite wait is a silent surprise, and this family does not
	// have those. So say it, once, and only when it is actually taking a
	// while: a pipe that was ready is never mentioned.
	if !quiet {
		t := time.AfterFunc(time.Second, func() {
			fmt.Fprintln(os.Stderr, "ply: waiting for stdin to end (^D closes it, ^C gives up)")
		})
		defer t.Stop()
	}
	b, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdin+1))
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	if len(b) > maxStdin {
		return nil, fmt.Errorf("piped input is larger than %d MB", maxStdin>>20)
	}
	return b, nil
}

// mint names a session. The format is ask's, because the file is an ask
// session: ply keeps no log of its own, and `ask replay -check` on this
// path proves the entire run.
func mint() (string, error) {
	dir := os.Getenv("PLY_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("no home directory for sessions; set $PLY_DIR")
		}
		dir = filepath.Join(home, ".ply", "sessions")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	var b [4]byte
	rand.Read(b[:])
	return filepath.Join(dir, time.Now().Format("20060102-150405")+"-"+hex.EncodeToString(b[:])+".jsonl"), nil
}

// descend counts how deep this ply is inside another. A toolbox program
// that starts ply is the sub-agent mechanism; without a bound it is also a
// fork bomb with a credit card.
func descend() (int, error) {
	n, _ := strconv.Atoi(os.Getenv("PLY_DEPTH"))
	if n >= maxDepth {
		return 0, fmt.Errorf("ply is %d deep inside itself; refusing to go further", n)
	}
	return n, nil
}

// underTree reports whether the session file sits under the directory
// commands run in. When it does, the model's own transcript is one `grep
// -r` away from being fed back to it — several turns of its own reasoning,
// spent to learn what it already knew, and on a provider that returns
// encrypted reasoning it is unreadable bulk. The default session directory
// is never in the tree; -f and $PLY_DIR can put it there, and neither is
// wrong often enough to refuse, so this is a note rather than an error.
func underTree(session, dir string) bool {
	if dir == "" {
		dir = "."
	}
	d, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	s, err := filepath.Abs(session)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(d, filepath.Dir(s))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func describe(b *Box, check string) string {
	tools := "shell"
	if b.Dir != "" {
		tools = fmt.Sprintf("%d tools", len(b.Tools))
		if b.Shell {
			tools += " + shell"
		}
	}
	if check == "" {
		return tools + " · no check"
	}
	return tools + " · check: " + oneline(check)
}

// list is a repeatable string flag.
type list []string

func (l *list) String() string     { return strings.Join(*l, ",") }
func (l *list) Set(s string) error { *l = append(*l, s); return nil }

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "ply: "+err.Error())
	return 1
}

// usage goes to stdout when it was asked for and to stderr when it was
// earned. A program that prints its help to stderr cannot be piped into
// less by the person who typed -h.
func usage(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(help)
		return 0
	}
	fmt.Fprintf(os.Stderr, "ply: %v\nusage: %s\n", err, synopsis)
	return 1
}
