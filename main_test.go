package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAsk stands in for the model. It is a program, like the real one, so
// the test exercises the same path production does: argv, stdin, stdout,
// and an exit status. Replies are handed out in order.
func fakeAsk(t *testing.T, replies ...string) (bin, dir string) {
	t.Helper()
	dir = t.TempDir()
	for i, r := range replies {
		write(t, filepath.Join(dir, "reply."+itoa(i+1)), r, 0o644)
	}
	bin = filepath.Join(dir, "ask")
	write(t, bin, `#!/bin/sh
d=`+dir+`
echo "$@" >> "$d/argv.log"
printf '%s' "$ASK_SYSTEM" > "$d/system"
cat >> "$d/stdin.log"
n=$(cat "$d/n" 2>/dev/null || echo 0); n=$((n+1)); echo "$n" > "$d/n"
[ -f "$d/reply.$n" ] && cat "$d/reply.$n"
exit ${FAKE_ASK_EXIT:-0}
`, 0o755)
	return bin, dir
}

func itoa(n int) string { return string(rune('0' + n)) }

// runPly calls the program in process, with the streams a filter's contract
// is written about pointed somewhere a test can read them.
func runPly(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	out, err1 := os.Create(filepath.Join(dir, "out"))
	errf, err2 := os.Create(filepath.Join(dir, "err"))
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	so, se := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errf
	code = run(args)
	os.Stdout, os.Stderr = so, se
	out.Close()
	errf.Close()
	return code, read(t, filepath.Join(dir, "out")), read(t, filepath.Join(dir, "err"))
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// sandbox points ply's sessions somewhere disposable and gives it a model.
func sandbox(t *testing.T, replies ...string) (workdir, plydir, askdir string) {
	t.Helper()
	bin, askdir := fakeAsk(t, replies...)
	plydir = t.TempDir()
	t.Setenv("ASK", bin)
	t.Setenv("PLY_DIR", plydir)
	return t.TempDir(), plydir, askdir
}

// TestTheLoopRunsWhatTheModelWrites is the whole program in one test: a
// reply with a block runs it, the output comes back, and a reply without
// one ends the run and is the answer.
func TestTheLoopRunsWhatTheModelWrites(t *testing.T) {
	work, _, askdir := sandbox(t,
		"I will look first.\n\n```ply\necho hello > made.txt\n```",
		"Wrote made.txt.",
	)
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "make a file")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "Wrote made.txt." {
		t.Errorf("stdout = %q, want the final reply alone", got)
	}
	if _, err := os.Stat(filepath.Join(work, "made.txt")); err != nil {
		t.Errorf("the command did not run in -C's directory: %v", err)
	}
	// The output goes back as the next message, as a typescript.
	if sent := read(t, filepath.Join(askdir, "stdin.log")); !strings.Contains(sent, "$ echo hello > made.txt") {
		t.Errorf("the model was not shown what ran:\n%s", sent)
	}
}

// TestCheckDecidesDone: exit 0 is a program's opinion, so `ply ... && push`
// means something. The model claiming success does not end the run.
func TestCheckDecidesDone(t *testing.T) {
	work, _, askdir := sandbox(t,
		"All done!", // a lie, and the check catches it
		"```ply\ntouch built\n```",
		"Built it.",
	)
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-check", "test -f "+filepath.Join(work, "built"), "build it")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "Built it." {
		t.Errorf("stdout = %q", got)
	}
	sent := read(t, filepath.Join(askdir, "stdin.log"))
	if !strings.Contains(sent, "check and it did not pass") {
		t.Errorf("the model was never told the check failed:\n%s", sent)
	}
}

// TestTheCheckIsNotScopedByTheToolbox: the toolbox exists to aim the model.
// The check is the caller's own program, and scoping it to three symlinks
// would mean `go test ./...` needed a toolbox holding go, git and a linker.
func TestTheCheckIsNotScopedByTheToolbox(t *testing.T) {
	work, _, _ := sandbox(t, "done")
	box := t.TempDir()
	write(t, filepath.Join(box, "onlytool"), "#!/bin/sh\n# the model's one tool\n", 0o755)

	// `env` is on the machine and not in the toolbox: a real check needs the
	// machine, and a model command must not.
	code, _, stderr := runPly(t, "-t", box, "-C", work, "-check", "env >/dev/null", "goal")
	if code != 0 {
		t.Fatalf("exit = %d, want 0: the check could not reach its own PATH\n%s", code, stderr)
	}
	// The toolbox still comes first, so a program in it can be the check.
	if code, _, _ := runPly(t, "-t", box, "-C", work, "-check", "onlytool", "goal"); code != 0 {
		t.Errorf("a toolbox program could not be used as the check: exit %d", code)
	}
}

// TestUnverifiedIsExitTwo: a supervisor has to tell "did not work" from
// "broke", or it retries the wrong one.
func TestUnverifiedIsExitTwo(t *testing.T) {
	work, _, _ := sandbox(t, "done", "done", "done", "done", "done", "done", "done")
	code, stdout, _ := runPly(t, "-sh", "-C", work, "-cycles", "2", "-check", "false", "impossible")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if strings.TrimSpace(stdout) != "done" {
		t.Errorf("stdout = %q: a run that fell short still has something to say", stdout)
	}
}

// TestNothingToBeDone: make's oldest manners. A goal already met costs no
// model call and leaves no session behind, which is what makes ply safe in
// a hook, a loop, or a Makefile.
func TestNothingToBeDone(t *testing.T) {
	work, plydir, askdir := sandbox(t, "should never be asked")
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-check", "true", "already true")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing: no work was done", stdout)
	}
	if !strings.Contains(stderr, "nothing to do") {
		t.Errorf("stderr = %q, want it to say so", stderr)
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); err == nil {
		t.Error("the model was called for a goal that was already met")
	}
	if ents, _ := os.ReadDir(plydir); len(ents) != 0 {
		t.Errorf("left %d files behind: %v", len(ents), ents)
	}
	// -B is for when you meant it anyway.
	if code, _, _ := runPly(t, "-sh", "-C", work, "-B", "-check", "true", "again"); code != 0 {
		t.Fatalf("-B exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); err != nil {
		t.Error("-B did not work the goal")
	}
}

// TestBadInvocationLeavesNoLitter: a filter must not create a session for a
// call that was never going to run.
func TestBadInvocationLeavesNoLitter(t *testing.T) {
	work, plydir, _ := sandbox(t, "x")
	for _, args := range [][]string{
		{"-C", work, "no toolbox and no -sh"},
		{"-sh", "-C", filepath.Join(work, "nope"), "bad -C"},
		{"-sh", "-t", filepath.Join(work, "nope"), "bad -t"},
		{"-sh", "-C", work, "-cap", "10", "silly cap"},
		{"-sh", "-C", work},
	} {
		code, stdout, _ := runPly(t, args...)
		if code != 1 {
			t.Errorf("%v: exit = %d, want 1", args, code)
		}
		if stdout != "" {
			t.Errorf("%v: wrote %q to stdout", args, stdout)
		}
	}
	if ents, _ := os.ReadDir(plydir); len(ents) != 0 {
		t.Errorf("a refused invocation left %v behind", ents)
	}
}

// TestContextFullIsExitTwo: ask's exit 2 exists so a retry loop can stop.
// ply is that loop.
func TestContextFullIsExitTwo(t *testing.T) {
	work, _, _ := sandbox(t, "unused")
	t.Setenv("FAKE_ASK_EXIT", "2")
	if code, _, _ := runPly(t, "-sh", "-C", work, "goal"); code != 2 {
		t.Errorf("exit = %d, want 2 for a full context window", code)
	}
}

func TestModelFailureIsExitOne(t *testing.T) {
	work, _, _ := sandbox(t, "unused")
	t.Setenv("FAKE_ASK_EXIT", "1")
	if code, _, _ := runPly(t, "-sh", "-C", work, "goal"); code != 1 {
		t.Errorf("exit = %d, want 1 for a broken provider", code)
	}
}

// TestSystemPromptTravelsInTheEnvironment: it is long, it can carry a
// private procedure brief just printed, and argv is world-readable in ps(1).
func TestSystemPromptTravelsInTheEnvironment(t *testing.T) {
	work, _, askdir := sandbox(t, "done")
	runPly(t, "-sh", "-C", work, "goal")
	if sys := read(t, filepath.Join(askdir, "system")); !strings.Contains(sys, "fenced shell block") {
		t.Errorf("ASK_SYSTEM did not carry the protocol:\n%s", sys)
	}
	if argv := read(t, filepath.Join(askdir, "argv.log")); strings.Contains(argv, "-S") {
		t.Errorf("the system prompt went through argv: %s", argv)
	}
}

// TestStdinIsTheGoalOrRidesWithIt, per ask and mu both.
func TestGoalComesFromArgvOrStdin(t *testing.T) {
	work, _, askdir := sandbox(t, "done")
	runPly(t, "-sh", "-C", work, "the goal in argv")
	if sent := read(t, filepath.Join(askdir, "stdin.log")); !strings.Contains(sent, "the goal in argv") {
		t.Errorf("the goal never reached the model:\n%s", sent)
	}
}

func TestDepthIsBounded(t *testing.T) {
	work, _, _ := sandbox(t, "done")
	t.Setenv("PLY_DEPTH", "8")
	code, _, stderr := runPly(t, "-sh", "-C", work, "goal")
	if code != 1 || !strings.Contains(stderr, "deep inside itself") {
		t.Errorf("exit = %d, stderr = %q; a ply that starts ply must have a floor", code, stderr)
	}
}

// ---- the documentation is part of the program ----

func flags(t *testing.T) []string {
	t.Helper()
	var names []string
	newOpts("ply").fs.VisitAll(func(f *flag.Flag) { names = append(names, f.Name) })
	return names
}

func verbs() []string { return []string{"tools", "system", "version", "help"} }

func TestHelpFitsEightyColumns(t *testing.T) {
	for i, line := range strings.Split(help, "\n") {
		if len(line) > 80 {
			t.Errorf("help line %d is %d columns:\n%s", i+1, len(line), line)
		}
	}
}

func TestEveryFlagAndVerbIsDocumented(t *testing.T) {
	man := read(t, "ply.1")
	readme := read(t, "README.md")
	for _, f := range flags(t) {
		if !strings.Contains(help, "-"+f) {
			t.Errorf("flag -%s is not in ply help", f)
		}
		if !strings.Contains(man, "-"+f) {
			t.Errorf("flag -%s is not in ply.1", f)
		}
	}
	// Guard the whole as well as the parts: a flag-level check stays green
	// while an entire verb goes undocumented.
	for _, v := range verbs() {
		for name, doc := range map[string]string{"ply help": help, "ply.1": man, "README.md": readme} {
			if !strings.Contains(doc, "ply "+v) {
				t.Errorf("verb %q is not in %s", v, name)
			}
		}
	}
	for _, env := range []string{"PLY_TOOLS", "PLY_DIR", "PLY_DEPTH", "ASK", "BRIEF", "NO_COLOR"} {
		if !strings.Contains(help, env) {
			t.Errorf("%s is not in ply help", env)
		}
		if !strings.Contains(man, env) {
			t.Errorf("%s is not in ply.1", env)
		}
	}
}

func TestManPageIsPlainAsciiAndCurrent(t *testing.T) {
	man := read(t, "ply.1")
	if !strings.Contains(man, version) {
		t.Errorf("ply.1 does not carry version %s", version)
	}
	for i, r := range man {
		if r > 127 {
			t.Fatalf("ply.1 has a non-ASCII rune %q at byte %d", r, i)
		}
	}
}

// TestHelpGoesToStdoutMisuseToStderr: a program that prints its help to
// stderr cannot be piped into less by the person who typed -h.
func TestHelpGoesToStdoutMisuseToStderr(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		code, stdout, stderr := runPly(t, arg)
		if code != 0 || !strings.HasPrefix(stdout, "ply") || stderr != "" {
			t.Errorf("%s: exit %d, stdout %d bytes, stderr %q", arg, code, len(stdout), stderr)
		}
	}
	code, stdout, stderr := runPly(t, "-nosuchflag", "goal")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "usage:") {
		t.Errorf("misuse: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestVersionIsOneLine(t *testing.T) {
	for _, arg := range []string{"version", "-V", "--version"} {
		code, stdout, _ := runPly(t, arg)
		if code != 0 || strings.TrimSpace(stdout) != "ply "+version {
			t.Errorf("%s: exit %d, stdout %q", arg, code, stdout)
		}
	}
}

// TestSystemVerbPrintsWhatWouldBeSent: it is a value, not a secret, so
// extending it is ordinary shell rather than a config format.
func TestSystemAndToolsVerbsShowTheRealThing(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "deploy"), "#!/bin/sh\n# push it\n", 0o755)

	code, stdout, _ := runPly(t, "system", "-t", dir, "-check", "make test")
	if code != 0 || !strings.Contains(stdout, "deploy") || !strings.Contains(stdout, "make test") {
		t.Errorf("ply system: exit %d, missing the toolbox or the check:\n%s", code, stdout)
	}
	code, stdout, _ = runPly(t, "tools", "-t", dir)
	if code != 0 || !strings.Contains(stdout, "push it") || !strings.Contains(stdout, "PATH="+dir) {
		t.Errorf("ply tools: exit %d:\n%s", code, stdout)
	}
	if code, _, stderr := runPly(t, "tools"); code != 1 || !strings.Contains(stderr, "no tools") {
		t.Errorf("ply tools with no toolbox: exit %d, %q", code, stderr)
	}
}
