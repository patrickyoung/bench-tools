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
	maxStdin     = 16 << 20 // as much as ask will carry in one message
	spoolOver    = 64 << 10 // past this, stdin becomes a file to grep
	maxDepth     = 8        // ply inside ply inside ply: a fork bomb that bills
	defaultTurns = 50       // one invocation must eventually return control
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
	fs         *flag.FlagSet
	toolbox    *string
	shell      *bool
	shellExec  *string
	check      *string
	force      *bool
	requireAct *bool
	cycles     *int
	turns      *int
	timeout    *time.Duration
	outcap     *int
	dir        *string
	spec       *string
	effort     *string
	sys        *string
	skills     list
	file       *string
	sessionOut *string
	quiet      *bool
	compact    *bool
	compacts   *int
	contractID *string
	steer      *string
	mayJob     *string
	cage       *bool
}

func newOpts(name string) *opts {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o := &opts{
		fs:         fs,
		toolbox:    fs.String("t", os.Getenv("PLY_TOOLS"), "toolbox directory; PATH becomes this alone"),
		shell:      fs.Bool("sh", false, "hand the model every program on PATH"),
		shellExec:  fs.String("shell", shellDefault(), "command interpreter; must accept -c"),
		check:      fs.String("check", "", "verifier: candidate stdin; 0 accept, 1 reject, other broken"),
		force:      fs.Bool("B", false, "work the goal even if the check already passes"),
		requireAct: fs.Bool("require-action", false, "refuse a final report until at least one command runs"),
		cycles:     fs.Int("cycles", 5, "rejected candidates before giving up (0 = unbounded)"),
		turns:      fs.Int("turns", defaultTurns, "model turns before giving up (0 = unbounded)"),
		timeout:    fs.Duration("timeout", 2*time.Minute, "per-command timeout"),
		outcap:     fs.Int("cap", 16<<10, "output kept per command, head and tail"),
		dir:        fs.String("C", "", "run commands here"),
		spec:       fs.String("m", "", "provider/model, passed to ask"),
		effort:     fs.String("effort", os.Getenv("PLY_EFFORT"), "reasoning effort, passed to ask"),
		sys:        fs.String("S", "", "system prompt, replacing the default"),
		file:       fs.String("f", "", "session log to write"),
		sessionOut: fs.String("session-out", "", "write the current session path to this file"),
		quiet:      fs.Bool("q", false, "no typescript on stderr"),
		compact:    fs.Bool("compact", false, "carry on through a full context window"),
		compacts:   fs.Int("compactions", 3, "compactions before giving up (0 = unbounded)"),
		contractID: fs.String("contract-id", os.Getenv("PLY_CONTRACT_ID"), "intent contract digest recorded in receipts"),
		steer:      fs.String("steer", "", "append-only operator steering file read between model turns"),
		mayJob:     fs.String("may-job", os.Getenv("PLY_MAY_JOB"), "require exact May approval before every model action"),
		cage:       fs.Bool("cage", false, "confine every approved model action with Cage"),
	}
	fs.Var(&o.skills, "s", "brief skill to compose; repeat for more; - picks one")
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

func (o *opts) runner(b *Box, self string, depth int, shell string, approval *mayGate) Runner {
	r := Runner{
		Dir:     *o.dir,
		Path:    b.Path(),
		Shell:   shell,
		Timeout: *o.timeout,
		Cap:     *o.outcap,
		// A tool that starts another ply is how fan-out, specialists and
		// teams happen here: a program, not a feature. The depth counter is
		// the only thing standing between that and a bill.
		Env: []string{"PLY=" + self, "PLY_DEPTH=" + strconv.Itoa(depth+1), "PLY_SHELL=" + shell},
	}
	// A nested Ply started as an ordinary command should not silently fall
	// back to a different model when the parent selected one with -m.
	if model := strings.TrimSpace(*o.spec); model != "" {
		r.Env = append(r.Env, "ASK_MODEL="+model)
	}
	if effort := strings.TrimSpace(*o.effort); effort != "" {
		r.Env = append(r.Env, "PLY_EFFORT="+effort)
	}
	if contractID := strings.TrimSpace(*o.contractID); contractID != "" {
		r.Env = append(r.Env, "PLY_CONTRACT_ID="+contractID)
	}
	if approval != nil {
		// Nested Ply is still a model-authored action. Inherit the same exact
		// gate rather than silently recovering unrestricted execution.
		r.Env = append(r.Env, "MAY="+approval.Bin, "PLY_MAY_JOB="+approval.Job)
	}
	return r
}

// checker is that runner with the caller's reach. Everything about running
// a command is shared; only the PATH differs, and it differs because the
// check belongs to whoever typed it.
func (o *opts) checker(r Runner, b *Box) Runner {
	r.Path = b.CheckPath()
	r.Cage = nil
	return r
}

func work(args []string) int {
	o := newOpts("ply")
	if err := o.fs.Parse(args); err != nil {
		return usage(err)
	}
	goal := strings.Join(o.fs.Args(), " ")
	if err := o.validate(); err != nil {
		return usage(err)
	}

	depth, err := descend()
	if err != nil {
		return fail(err)
	}
	box, err := o.box()
	if err != nil {
		return fail(err)
	}
	shell, err := resolveShell(*o.shellExec)
	if err != nil {
		return fail(err)
	}
	askBin, err := tool("ASK", "ask", "ply runs ask for the model: go install github.com/patrickyoung/ask@latest")
	if err != nil {
		return fail(err)
	}
	self, err := os.Executable()
	if err != nil {
		self = "ply"
	}
	if *o.dir != "" {
		if fi, err := os.Stat(*o.dir); err != nil || !fi.IsDir() {
			return fail(fmt.Errorf("-C %s: not a directory", *o.dir))
		}
	}
	var approval *mayGate
	if *o.mayJob != "" {
		canonical, err := canonicalWorkDir(*o.dir)
		if err != nil {
			return fail(err)
		}
		*o.dir = canonical
		mayBin, err := tool("MAY", "may", "-may-job needs may: go install github.com/patrickyoung/may@latest")
		if err != nil {
			return fail(err)
		}
		approval, err = openMayGate(mayBin, *o.mayJob)
		if err != nil {
			return fail(err)
		}
	}
	var cageBin string
	var confinement *cageLauncher
	if *o.cage {
		cageBin, err = tool("CAGE", "cage", "-cage needs Cage: go install github.com/patrickyoung/cage@latest")
		if err != nil {
			return fail(err)
		}
		if _, err := executableDigest("Cage", cageBin); err != nil {
			return fail(err)
		}
		probe := *o.file
		if probe == "" {
			dir, dirErr := defaultSessionDir()
			if dirErr != nil {
				return fail(dirErr)
			}
			probe = filepath.Join(dir, "session.jsonl")
		}
		probeTemp := strings.TrimSuffix(probe, ".jsonl") + ".cage-tmp"
		if err := validateCageControlPaths(*o.dir, probeTemp, probe, *o.sessionOut, *o.steer,
			askBin, approval.Bin, cageBin, shell, self); err != nil {
			return fail(err)
		}
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
	var steering *steeringInbox
	if strings.TrimSpace(*o.steer) != "" {
		steering, err = openSteering(*o.steer)
		if err != nil {
			return fail(err)
		}
		defer steering.Close()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// -S replaces the default, and -S "" sends none, as it does for ask.
	// The protocol lives in the default, so replacing it is a real choice:
	// `ply system` prints what you would be dropping, and the manual says
	// to compose with it rather than around it.
	system := prompt(box, shell, *o.dir, *o.check, *o.timeout, *o.outcap, depth, approval != nil)
	if *o.cage {
		system += confinementPrompt()
	}
	o.fs.Visit(func(f *flag.Flag) {
		if f.Name == "S" {
			system = *o.sys
		}
	})
	v := newView(os.Stderr, *o.quiet)
	var skills []loaded
	if len(o.skills) > 0 {
		s, got, err := brief(ctx, o.skills, goal, v)
		if err != nil {
			return fail(err)
		}
		system = composeSystem(system, s, *o.requireAct)
		skills = got
		for _, skill := range skills {
			how := "named"
			if skill.Chosen {
				how = "selected by Brief"
			}
			v.Note("Brief procedure %s loaded (%s)", skill.Name, how)
		}
	} else {
		system = composeSystem(system, "", *o.requireAct)
	}

	runner := o.runner(box, self, depth, shell, approval)
	checker := o.checker(runner, box)

	// make's "nothing to be done": a goal already met costs nothing, leaves
	// no session behind, and is safe to put in a hook or a Makefile.
	var initialCheck *Result
	if *o.check != "" && !*o.force {
		r := checker.RunInput(ctx, *o.check, "")
		v.Check(r)
		// An orchestrator may already have created -f while compiling an
		// intent contract. In that case even a pre-check terminal must become
		// durable evidence in the same session. Plain Ply keeps make's
		// zero-litter fast path when no session exists yet.
		if *o.file != "" {
			if _, statErr := os.Stat(*o.file); statErr == nil {
				model := Model{Bin: askBin, Session: *o.file}
				recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
				err := model.Record(recordCtx, verdictSource, verifierReceiptKind,
					receiptFor(*o.contractID, "baseline", "", checker, r))
				cancel()
				if err != nil {
					return fail(fmt.Errorf("record verifier receipt: %w", err))
				}
			}
		}
		if verifierOutcome(r) == "accepted" {
			v.Note("nothing to do")
			return 0
		} else if ctx.Err() != nil {
			return 130
		} else if verifierOutcome(r) == "broken" {
			return fail(checkError(r))
		}
		initialCheck = &r
	}

	// Everything that can fail has failed by here. Only now does ply put a
	// file on the disk: a bad invocation leaves no litter.
	session := *o.file
	if session == "" {
		if session, err = mint(); err != nil {
			return fail(err)
		}
	}
	if *o.sessionOut != "" {
		if session, err = filepath.Abs(session); err != nil {
			return fail(fmt.Errorf("session path: %w", err))
		}
	}
	if *o.cage {
		cageTemp := strings.TrimSuffix(session, ".jsonl") + ".cage-tmp"
		if err := validateCageControlPaths(*o.dir, cageTemp, session, *o.sessionOut, *o.steer,
			askBin, approval.Bin, cageBin, shell, self); err != nil {
			return fail(err)
		}
		confinement, err = openCageLauncher(cageBin, *o.dir, cageTemp)
		if err != nil {
			return fail(err)
		}
		runner.Cage = confinement
		runner.Env = append(runner.Env, "TMPDIR="+confinement.TempDir)
	}
	first, err := spool(goal, data, session)
	if err != nil {
		return fail(err)
	}
	if initialCheck != nil {
		first = withInitialCheck(first, *initialCheck)
	}
	if err := writeSessionOut(*o.sessionOut, session); err != nil {
		return fail(err)
	}

	v.Note("%s · %s", session, describe(box, shell, *o.check, approval != nil, confinement != nil))
	if underTree(session, *o.dir) {
		v.Note("the session is inside the work tree, so a grep or a find will\n" +
			"     read it back into the conversation it is a record of; -f a path\n" +
			"     outside the tree keeps the record out of the work")
	}
	loop := &Loop{
		Model:         Model{Bin: askBin, Session: session, Spec: *o.spec, Effort: *o.effort, System: system},
		Runner:        runner,
		Checker:       checker,
		Check:         *o.check,
		RequireAction: *o.requireAct,
		Loaded:        skillNote(skills),
		Cycles:        *o.cycles,
		Compact:       *o.compact,
		Compacts:      *o.compacts,
		Turns:         *o.turns,
		View:          v,
		ContractID:    *o.contractID,
		Steering:      steering,
		Approval:      approval,
	}
	if *o.sessionOut != "" {
		loop.SessionChanged = func(path string) error {
			return writeSessionOut(*o.sessionOut, path)
		}
	}
	answer, err := loop.Run(ctx, first)
	if answer != "" && !v.Shown() && !errors.Is(err, ErrApprovalParked) &&
		!errors.Is(err, ErrApprovalDeclined) && !errors.Is(err, ErrApprovalBoundary) &&
		!errors.Is(err, ErrConfinement) {
		fmt.Println(strings.TrimRight(answer, "\n"))
	}
	switch {
	case err == nil:
		return 0
	case errors.Is(err, context.Canceled):
		v.Note("interrupted")
		return 130
	case errors.Is(err, ErrApprovalParked):
		fmt.Fprintf(os.Stderr, "ply: %v\n", err)
		return 75
	case errors.Is(err, ErrApprovalDeclined):
		fmt.Fprintf(os.Stderr, "ply: %v\n", err)
		return 3
	case errors.Is(err, ErrConfinement):
		fmt.Fprintf(os.Stderr, "ply: %v\n", err)
		return cageBoundaryExit
	case errors.Is(err, ErrCycles), errors.Is(err, ErrTurns), errors.Is(err, ErrOverflow), errors.Is(err, ErrProtocol):
		fmt.Fprintf(os.Stderr, "ply: %v\n", err)
		return 2
	default:
		return fail(err)
	}
}

func (o *opts) validate() error {
	switch {
	case *o.cage && strings.TrimSpace(*o.mayJob) == "":
		return errors.New("-cage requires -may-job")
	case *o.cage && strings.TrimSpace(*o.contractID) == "":
		return errors.New("-cage requires -contract-id")
	case *o.cage && *o.compact:
		return errors.New("-cage does not support -compact; start a new explicit invocation instead")
	case *o.cycles < 0:
		return fmt.Errorf("-cycles %d: must be zero or greater", *o.cycles)
	case *o.turns < 0:
		return fmt.Errorf("-turns %d: must be zero or greater", *o.turns)
	case *o.compacts < 0:
		return fmt.Errorf("-compactions %d: must be zero or greater", *o.compacts)
	case *o.timeout <= 0:
		return fmt.Errorf("-timeout %s: must be greater than zero", *o.timeout)
	case *o.outcap < 512:
		return fmt.Errorf("-cap %d: too small to be worth reading", *o.outcap)
	case *o.mayJob != "":
		return validateMayJob(*o.mayJob)
	}
	return nil
}

func canonicalWorkDir(dir string) (string, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("approval working directory: %w", err)
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("approval working directory: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("approval working directory: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("approval working directory: %s is not a directory", resolved)
	}
	return resolved, nil
}

func shellDefault() string {
	if shell := os.Getenv("PLY_SHELL"); shell != "" {
		return shell
	}
	return defaultShell
}

func toolsCmd(args []string) int {
	o := newOpts("ply tools")
	if err := o.fs.Parse(args); err != nil {
		return usage(err)
	}
	if err := o.validate(); err != nil {
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
	if err := o.validate(); err != nil {
		return usage(err)
	}
	box, err := o.box()
	if err != nil {
		return fail(err)
	}
	shell, err := resolveShell(*o.shellExec)
	if err != nil {
		return fail(err)
	}
	depth, _ := strconv.Atoi(os.Getenv("PLY_DEPTH"))
	out := prompt(box, shell, *o.dir, *o.check, *o.timeout, *o.outcap, depth, *o.mayJob != "")
	if *o.cage {
		out += confinementPrompt()
	}
	procedures := ""
	if len(o.skills) > 0 {
		s, _, err := brief(context.Background(), o.skills, strings.Join(o.fs.Args(), " "), newView(os.Stderr, false))
		if err != nil {
			return fail(err)
		}
		procedures = s
	}
	fmt.Print(composeSystem(out, procedures, *o.requireAct))
	return 0
}

// brief loads procedures. `-s -` asks the catalogue to choose, and brief
// refuses to guess: nothing matched is an answer, so the run goes on
// without one and stderr says so.
// loaded is one skill that went into the system prompt, and how it got
// there. The distinction is worth keeping: a skill somebody named is a
// choice, and a skill brief picked is a guess that the run then tested.
type loaded struct {
	Name   string
	Chosen bool // brief find picked it, rather than being named
}

func brief(ctx context.Context, names list, goal string, v *view) (string, []loaded, error) {
	bin, err := tool("BRIEF", "brief", "-s needs brief: go install github.com/patrickyoung/brief@latest")
	if err != nil {
		return "", nil, err
	}
	var s strings.Builder
	var got []loaded
	for _, name := range names {
		chosen := false
		if name == "-" {
			if name, err = briefFind(ctx, bin, goal); err != nil {
				return "", nil, err
			}
			if name == "" {
				v.Note("brief matched no skill for this goal; continuing without one")
				continue
			}
			chosen = true
			v.Note("brief chose %s", name)
		}
		body, err := briefCat(ctx, bin, name)
		if err != nil {
			return "", nil, err
		}
		s.WriteString("\n\nThe procedure below applies to this goal. Follow it.\n\n" + strings.TrimSpace(body) + "\n")
		got = append(got, loaded{Name: name, Chosen: chosen})
	}
	return s.String(), got, nil
}

// skillNote is what ply records about its own composition.
//
// The system prompt reaches the log whole and verbatim, so what shaped a
// run is already provable byte for byte. What is not in it is where any of
// those bytes came from: `brief cat` prints a body without its frontmatter,
// so a skill arrives as anonymous prose and its name lives only on stderr,
// where it dies with the terminal.
//
// That matters to whatever reads the log afterwards. A run that loaded a
// procedure and stumbled anyway is not just a run that stumbled -- it is
// evidence that procedure is incomplete, and hone(1) can only say so if
// the log names it.
//
// It is ply's claim about its own composition, signed by ply, rather than
// something ask asserts about a string it was handed. It is checkable: the
// body is in the header, so a reader can confirm the named skill's text is
// actually there.
func skillNote(got []loaded) string {
	var b strings.Builder
	for _, l := range got {
		how := "named"
		if l.Chosen {
			how = "chosen by brief find"
		}
		fmt.Fprintf(&b, "loaded skill %s (%s)\n", l.Name, how)
	}
	return b.String()
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
	dir, err := defaultSessionDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	var b [4]byte
	rand.Read(b[:])
	return filepath.Join(dir, time.Now().Format("20060102-150405")+"-"+hex.EncodeToString(b[:])+".jsonl"), nil
}

func defaultSessionDir() (string, error) {
	dir := os.Getenv("PLY_DIR")
	if dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("no home directory for sessions; set $PLY_DIR")
	}
	return filepath.Join(home, ".ply", "sessions"), nil
}

// writeSessionOut maintains the optional process-level pointer to the Ask
// session which currently owns the run. It is a control artifact, not a
// second log: one path, replaced atomically, with the conversation still in
// Ask alone.
func writeSessionOut(control, session string) error {
	if control == "" {
		return nil
	}
	control, err := filepath.Abs(control)
	if err != nil {
		return fmt.Errorf("-session-out: %w", err)
	}
	session, err = filepath.Abs(session)
	if err != nil {
		return fmt.Errorf("session path: %w", err)
	}
	if strings.ContainsAny(session, "\r\n") {
		return errors.New("-session-out cannot report a session path containing a newline")
	}
	if control == session {
		return errors.New("-session-out must not name the Ask session itself")
	}
	f, err := os.CreateTemp(filepath.Dir(control), ".ply-session-*")
	if err != nil {
		return fmt.Errorf("-session-out %s: %w", control, err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := io.WriteString(f, session+"\n"); err != nil {
		_ = f.Close()
		return fmt.Errorf("-session-out %s: %w", control, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("-session-out %s: %w", control, err)
	}
	if err := os.Rename(tmp, control); err != nil {
		return fmt.Errorf("-session-out %s: %w", control, err)
	}
	ok = true
	return nil
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

func describe(b *Box, shell, check string, approval, caged bool) string {
	tools := "shell"
	if b.Dir != "" {
		tools = fmt.Sprintf("%d tools", len(b.Tools))
		if b.Shell {
			tools += " + shell"
		}
	}
	tools += " · shell: " + shell
	if approval {
		tools += " · May approval: every action"
	}
	if caged {
		tools += " · Cage: workspace + private temp, no network"
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
