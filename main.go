// hone reads a session that a program judged and writes down what it
// teaches.
//
// The whole program is four steps: read the log, check it earned a lesson,
// have one worded, write it where brief will find it. If a change makes
// that sentence longer, it needs a very good reason.
//
//	ask     the model      -- no tools, no loop
//	brief   the procedure  -- no model, no loop
//	ply     the loop       -- no model, no procedure
//	hone    the lesson     -- no store, no retrieval, no format
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const version = "0.2.0"

// maxLessons bounds what one run can add. It is small on purpose: a run
// that appears to teach five things has usually taught one thing and four
// restatements of it, and the corpus that stays useful is the one that
// stays readable.
const maxLessons = 3

type hone struct {
	askBin   string
	briefBin string
	model    string
	max      int
	into     string
	dry      bool
	quiet    bool
	verify   bool
	proposal string
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "forget":
			return cmdForget(args[1:])
		case "prompt":
			return cmdPrompt(args[1:])
		case "show":
			return cmdShowProposal(args[1:])
		case "admit":
			return cmdAdmit(args[1:])
		case "version", "-V", "--version":
			fmt.Printf("hone %s\n", version)
			return 0
		case "help", "-h", "--help":
			fmt.Print(usageText)
			return 0
		}
		if v := nearVerb(args[0]); v != "" {
			fmt.Fprintf(os.Stderr, "hone: unknown command %q -- did you mean %q?\n", args[0], v)
			return 2
		}
	}
	return cmdHone(args)
}

var verbs = []string{"forget", "prompt", "show", "admit", "version", "help"}

// nearVerb catches a mistyped command before it is read as a session path.
// Unlike ask, everything here is a path, so a word that is not a file and
// is one edit from a verb is almost certainly the verb.
func nearVerb(word string) string {
	if word == "" || word[0] == '-' || strings.ContainsRune(word, filepath.Separator) {
		return ""
	}
	if _, err := os.Stat(word); err == nil {
		return "" // it is a file; it was meant as one
	}
	for _, v := range verbs {
		if distance(word, v) == 1 {
			return v
		}
	}
	return ""
}

func cmdHone(args []string) int {
	fs := flag.NewFlagSet("hone", flag.ContinueOnError)
	var (
		into    = fs.String("into", "", "fold the lessons into this skill instead of printing them")
		mspec   = fs.String("m", "", "provider/model for the wording (default: the session's own)")
		max     = fs.Int("n", maxLessons, "most lessons to take from one run")
		dry     = fs.Bool("N", false, "say what would be learned, write nothing")
		quiet   = fs.Bool("q", false, "no progress on stderr")
		noVfy   = fs.Bool("no-verify", false, "skip the replay check on the session")
		expl    = fs.Bool("why", false, "print the evidence a lesson would be drawn from, and stop")
		prepare = fs.String("prepare", "", "write an exact reviewed-learning proposal, not the skill")
		dirArg  = fs.String("d", askDir(), "session directory")
	)
	usage(fs, "hone [flags] [session ...]")
	if err := fs.Parse(args); err != nil {
		return usageCode(fs, err)
	}
	if *max < 1 {
		return fail(fmt.Errorf("-n %d: a run teaches at least one thing or none of them", *max))
	}
	if *prepare != "" && *into == "" {
		return fail(errors.New("-prepare needs -into SKILL"))
	}
	if *prepare != "" && (*dry || *expl) {
		return fail(errors.New("-prepare cannot be combined with -N or -why"))
	}
	if *prepare != "" && *noVfy {
		return fail(errors.New("-prepare always replay-verifies its source session"))
	}
	if *prepare != "" {
		if err := checkProposalDestination(*prepare); err != nil {
			return fail(err)
		}
	}

	// Flags stop at the first path, so `hone s.jsonl -into x` reads the
	// flag as a session and fails three frames later saying it cannot stat
	// "-into". Say the actual thing instead.
	for _, a := range fs.Args() {
		if strings.HasPrefix(a, "-") && a != "-" {
			return fail(fmt.Errorf("%s came after a session, where it is a filename, not a flag: put flags first", a))
		}
	}

	paths, err := sessions(*dirArg, fs.Args())
	if err != nil {
		return fail(err)
	}
	if *prepare != "" && len(paths) != 1 {
		return fail(errors.New("-prepare accepts exactly one session"))
	}

	g := &hone{model: *mspec, max: *max, into: *into, dry: *dry, quiet: *quiet, verify: !*noVfy, proposal: *prepare}
	if *expl {
		return explain(paths)
	}
	if g.askBin, err = tool("ASK", "ask", "hone has no model of its own"); err != nil {
		return fail(err)
	}
	g.briefBin = optional("BRIEF", "brief")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	learned := 0
	for _, p := range paths {
		n, code := g.one(ctx, p)
		if code == 2 {
			return 2
		}
		learned += n
	}
	if learned == 0 {
		return 1 // no: an ordinary answer, and the common one
	}
	return 0
}

// one works a single session. It returns how many lessons it produced and
// an exit code, where 2 is the only one that stops a batch: "this run
// taught nothing" must never end a loop over an archive, and "ask is not
// installed" must never be mistaken for it.
func (g *hone) one(ctx context.Context, path string) (int, int) {
	s, err := readSession(path)
	if err != nil {
		return 0, fail(err)
	}
	if g.verify {
		if err := verify(ctx, g.askBin, path); err != nil {
			g.say("%v", err)
			g.say("a log that is not a record of what happened is not evidence")
			return 0, 1
		}
	}
	if ok, why := s.Teaches(); !ok {
		g.say("%s: %s", s.ID, why)
		return 0, 1
	}
	// Before the model is called, not after: a run teaches once, and a
	// second pass over an archive should cost nothing at all.
	into := g.into
	if into == "-" {
		// The skill the run was following. brief refuses to guess and so
		// does this: no skill, or more than one, and there is no single
		// place the lesson belongs.
		sk := s.Skills()
		switch len(sk) {
		case 1:
			into = sk[0].Name
			g.say("%s: the run was following %s -- the lesson belongs to it", s.ID, into)
		case 0:
			g.say("%s: the run loaded no skill, so -into - names nothing. "+
				"Run it with ply -s, or name a skill", s.ID)
			return 0, 1
		default:
			var names []string
			for _, l := range sk {
				names = append(names, l.Name)
			}
			g.say("%s: the run loaded %s, so -into - is ambiguous. Name one",
				s.ID, strings.Join(names, " and "))
			return 0, 1
		}
	}

	dir := ""
	if into != "" && (!g.dry || g.proposal != "") {
		var err error
		if dir, err = skillDir(into); err != nil {
			return 0, fail(err)
		}
		if taught(dir, s.ID) {
			g.say("%s: already learned from, in %s -- hone forget %s %s to learn it again",
				s.ID, into, s.ID, into)
			return 0, 1
		}
	}

	n := len(s.Stumbles())
	g.say("%s: %d stumble(s), check passed -- asking %s what it teaches", s.ID, n, modelName(g.model))

	reply, by, err := g.runAsk(ctx, s.Evidence())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0, 130
		}
		return 0, fail(err)
	}
	lessons := parse(reply, g.max)
	if len(lessons) == 0 {
		g.say("%s: nothing worth keeping · ask replay -check %s", s.ID, by)
		return 0, 1
	}
	if g.proposal != "" {
		return g.prepare(ctx, g.proposal, dir, into, path, s, lessons, by)
	}

	if into == "" || g.dry {
		for _, l := range lessons {
			fmt.Println(l)
			fmt.Println(mark(s.ID, by))
		}
		g.say("%s: %d lesson(s) · ask replay -check %s", s.ID, len(lessons), by)
		return len(lessons), 0
	}
	return g.write(ctx, dir, s, lessons, by)
}

func (g *hone) write(ctx context.Context, dir string, s *session, lessons []string, by string) (int, int) {
	name := filepath.Base(strings.TrimRight(dir, string(filepath.Separator)))

	added, held, err := add(dir, name, lessons, s.ID, by, func() string {
		g.say("%s is new; writing the line brief will find it by", name)
		return g.describe(ctx, name, lessons)
	})
	if err != nil {
		return 0, fail(err)
	}
	if added == 0 {
		g.say("%s: %d lesson(s), all already there", s.ID, len(lessons))
		return 0, 1
	}
	// hone checks its own work with the program that owns the format.
	if err := lint(ctx, g.briefBin, dir); err != nil {
		g.say("brief lint has something to say about %s: %v", name, err)
	}
	fmt.Printf("%s: %d lesson(s) added (%d total)\n", filepath.Join(dir, "SKILL.md"), added, held)
	g.say("%s · ask replay -check %s", s.ID, by)
	return added, 0
}

func cmdForget(args []string) int {
	fs := flag.NewFlagSet("forget", flag.ContinueOnError)
	quiet := fs.Bool("q", false, "no progress on stderr")
	usage(fs, "hone forget <session-id> <skill> [skill ...]")
	if err := fs.Parse(args); err != nil {
		return usageCode(fs, err)
	}
	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: hone forget <session-id> <skill> [skill ...]")
		return 2
	}
	id, targets := fs.Arg(0), fs.Args()[1:]

	total := 0
	for _, t := range targets {
		dir, err := skillDir(t)
		if err != nil {
			return fail(err)
		}
		path := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			return fail(fmt.Errorf("no skill named %q on %s", t, strings.Join(briefPath(), string(os.PathListSeparator))))
		}
		n, err := forget(path, id)
		if err != nil {
			return fail(err)
		}
		if n > 0 && !*quiet {
			fmt.Fprintf(os.Stderr, "hone: %s: forgot %d lesson(s) from %s\n", path, n, id)
		}
		total += n
	}
	if total == 0 {
		return 1
	}
	return 0
}

func cmdPrompt(args []string) int {
	fs := flag.NewFlagSet("prompt", flag.ContinueOnError)
	max := fs.Int("n", maxLessons, "most lessons to take from one run")
	usage(fs, "hone prompt [flags]")
	if err := fs.Parse(args); err != nil {
		return usageCode(fs, err)
	}
	fmt.Printf(systemPrompt+"\n", *max)
	return 0
}

// explain prints what would be sent and stops. A lesson is a claim, and
// being able to see the evidence before paying for the claim is the
// difference between a tool you trust and one you audit afterwards.
func explain(paths []string) int {
	found := false
	for _, p := range paths {
		s, err := readSession(p)
		if err != nil {
			return fail(err)
		}
		ok, why := s.Teaches()
		if !ok {
			fmt.Fprintf(os.Stderr, "hone: %s: %s\n", s.ID, why)
			continue
		}
		fmt.Print(s.Evidence())
		found = true
	}
	if !found {
		return 1
	}
	return 0
}

// sessions resolves what to read: the paths given, or the current
// conversation when none are. Naming a directory reads every session in it,
// oldest first, which is what a batch over an archive wants.
func sessions(dir string, args []string) ([]string, error) {
	if len(args) == 0 {
		p, err := current(dir)
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	}
	var out []string
	for _, a := range args {
		fi, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			out = append(out, a)
			continue
		}
		ents, err := os.ReadDir(a)
		if err != nil {
			return nil, err
		}
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
				out = append(out, filepath.Join(a, e.Name()))
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no sessions to read")
	}
	return out, nil
}

func current(dir string) (string, error) {
	if target, err := os.Readlink(filepath.Join(dir, "current")); err == nil {
		p := filepath.Join(dir, target)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %w (name a session, or -d its directory)", dir, err)
	}
	newest, best := "", time.Time{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		fi, err := e.Info()
		if err == nil && (newest == "" || fi.ModTime().After(best)) {
			newest, best = e.Name(), fi.ModTime()
		}
	}
	if newest == "" {
		return "", fmt.Errorf("no sessions in %s", dir)
	}
	return filepath.Join(dir, newest), nil
}

func askDir() string {
	if d := os.Getenv("ASK_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ask/sessions"
	}
	return filepath.Join(home, ".ask", "sessions")
}

func (g *hone) say(format string, a ...any) {
	if g.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, "hone: "+format+"\n", a...)
}

func usage(fs *flag.FlagSet, synopsis string) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s\n", synopsis)
		fs.PrintDefaults()
	}
}

// usageCode keeps requested help on stdout and misuse on stderr, so
// `hone -h | less` works and a misuse leaves stdout empty for whatever was
// parsing it.
func usageCode(fs *flag.FlagSet, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(usageText)
		return 0
	}
	return 2
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "hone: %v\n", err)
	return 2
}

func stamp() string { return time.Now().UTC().Format("20060102-150405.000000") }

// distance is Levenshtein, bounded by the two words being command names.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(min(cur[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
