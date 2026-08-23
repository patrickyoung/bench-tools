package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

func confinementPrompt() string {
	return `

Every model-authored action runs inside Cage after exact May approval. The
real workspace and one private temporary directory are writable, host network
access is denied, and host files remain readable. Ask, May, Brief, and the
configured verifier run outside Cage. Cage status 125 is reserved for a
confinement failure and stops the invocation.
`
}

// prompt builds the system prompt. It is a value, not a secret: `ply
// system` prints exactly what would be sent, so extending it is ordinary
// shell rather than a config format.
//
// Everything in it is load-bearing. The protocol is here because there is
// no tool-use API underneath — a fenced shell block is the wire format —
// and -S replaces the whole thing, which is why `ply system` exists and why
// the manual says to compose with it rather than around it.
func prompt(box *Box, shell, dir, check string, timeout time.Duration, outCap, depth int, approval bool) string {
	return promptWithCheckShell(box, shell, shell, dir, check, timeout, outCap, depth, approval)
}

func promptWithCheckShell(box *Box, actionShell, checkShell, dir, check string, timeout time.Duration, outCap, depth int, approval bool) string {
	var s strings.Builder

	fmt.Fprintf(&s, `You are working through a Unix shell to reach the user's goal.

A reply is only a report: it does not create files, run programs, or change
the system. When the goal asks for an answer, review, or diagnosis, inspect
the relevant evidence and report it without making unrequested changes. When
it asks for an artifact or a change, use the available programs to make that
effect real. Do not stop at advice, sample content, or an inventory while the
requested effect remains undone.

Inspect before changing state, preserve unrelated work, and prefer reversible
operations for broad goals. If ambiguity risks destruction or scope expansion,
finish the safe work and report the exact remaining decision instead of
guessing.

Nothing is watching and nothing reads your prose but a log, so you cannot
ask a question: a question mark is a dead end that costs a turn. If the
goal is ambiguous, take the most useful reading, say which reading you
took, and get on with it.

To run something, write a fenced shell block:

`+"```ply"+`
go test ./... 2>&1 | tail -20
`+"```"+`

An action turn contains exactly one nonempty fenced shell block, as its final
content. Ply executes that block and returns its terminal result before you
continue. A turn with no shell block is your final report and ends the run.
There is no way to quote a fenced shell block without running it: indent it
four spaces instead. If a command itself contains a line of three backticks --
writing a README, say -- open the block with four or more, as markdown has
always asked.

%s

Nothing runs until your message ends, so you see no output until your next
turn. End the turn at the closing fence, then read what comes back. Ply runs
only the first complete command block in a turn. It defers every later block
and any text after the first, tells you that it did so, and returns the first
command's real result. An empty or unfinished first block runs nothing and is
returned for correction. Put steps that need no observation in one shell
script. When a later step depends on output, send the first command now and
wait. Never predict or report what a command printed before you receive it.
`, interpreterPrompt(actionShell, checkShell))

	if approval {
		s.WriteString(`
Every model-authored action is subject to an exact May approval before it
runs. If those exact bytes are not already granted, Ply stops this invocation
without executing them. Approval changes neither the goal nor the tool grant;
it is only authority for the one proposed shell action.
`)
	}

	if actionShell == checkShell {
		fmt.Fprintf(&s, `
It runs in %s on %s.`, quoteDir(dir), platformName())
	} else {
		fmt.Fprintf(&s, `
Ply starts the action interpreter in %s. That interpreter may execute elsewhere;
Ply does not know its target platform, programs, or final working directory.`, quoteDir(dir))
	}
	fmt.Fprintf(&s, ` Nothing is on stdin, so a program that waits for input
will sit there until it is killed at %s -- pass what it needs in arguments,
a here-document, or a file. Output is kept to about %s per command with the
middle elided, so narrow it with grep, head or tail rather than running it
twice and hoping.

Work in steps you actually read. One block that runs six commands and
prints nothing you look at is worse than three blocks you look at. Reach
for the program that already does it before writing a script, and for a
short script before a long explanation. When something fails, find out why
before changing it. Use command -v and a program's documented read-only help
form to discover capabilities and syntax. Do not guess -h, --help, or GNU/BSD
option meanings: a Unix program may treat an option-looking word as an operand.
After changing state, inspect the result with an independent command before
you stop.

`, timeout, bytesize(outCap))

	s.WriteString(box.Catalogue())
	if depth == 0 && canDelegate(box) && !approval {
		fmt.Fprintf(&s, `
If and only if the goal explicitly asks for subagents, delegation, or parallel
agent work, another ordinary ply process is a subagent. Start each one with:

    %s "one independent, bounded task; return a concise evidence-backed summary"

Announce the delegated job names in prose before the command block. Run at most
three at once, and make the complete fan-out fit this command's %s
timeout. Give every child all context it needs; skills do not carry over.
First make a private run directory with

    umask 077
    base=${PLY_DIR:-${TMPDIR:-/tmp}}
    mkdir -p "$base"
    run=$(mktemp -d "$base/ply-team.XXXXXX")

Replace NNN with a stable numeric task index in each child command. Redirect
that child's stdout and stderr to matching NNN.out and NNN.err files. Preserve
failure under set -e with `+"`"+`rc=0; child ... || rc=$?`+"`"+`, then atomically publish the
status through NNN.rc.tmp and mv it to NNN.rc. Wait for every child, then read
results in task order, not completion order. A missing or nonzero status is a
visible failure, not a result. Keep child typescripts out of this context;
their indexed sessions and stderr files are the evidence.

Delegate read-heavy exploration, tests, triage, and review. You remain the
sole writer and synthesizer in this working tree; the configured check still
decides done. For truly independent
writes, use disjoint worktrees. Synthesize the child summaries yourself;
never pass their conclusion through unchanged.
`, subagentCommand(box), timeout)
	}

	if check != "" {
		fmt.Fprintf(&s, `
Whether you are done is not your judgment. When you stop, ply runs

    %s

with your final report on its standard input. Exit 0 accepts the work; exit
1 rejects it, and you will see what the check printed and keep working. Any
other exit means the checker itself is broken and stops the run. File and
code checks may simply ignore stdin. Do not announce success; make the check
accept the work.
`, indentAfterFirst(check, "    "))
	} else {
		s.WriteString(`
Nothing checks this work but you. Before you stop, run the command that
would show somebody else you are right, and put what it printed in your
answer.
`)
	}

	s.WriteString(`
When the work is done, reply with no fenced block at all. That reply ends
the run and is the only thing that reaches stdout, so write it for whoever
asked for this goal: what you did, the evidence, and anything you could not
finish. No preamble, no sign-off, no restating the goal back.
`)
	return s.String()
}

func interpreterPrompt(actionShell, checkShell string) string {
	if actionShell == checkShell {
		return fmt.Sprintf(`Each block runs as %s -c SCRIPT, not under the operator's interactive shell.
The same interpreter runs the check, if one is configured, and PLY_SHELL names
it. Fence labels are protocol markers; bash or zsh on a fence does not select
another interpreter. Write syntax this interpreter accepts. POSIX shell syntax
is the portable baseline. Pipes, redirection, loops, heredocs and several lines
all work when the interpreter supports them, and the block is one command as
far as this conversation is concerned. You get back what a terminal would have
shown: stdout and stderr interleaved, and the exit status when it is not zero.`, shellQuote(actionShell))
	}
	return fmt.Sprintf(`Each block runs as %s -c SCRIPT, not under the operator's interactive shell.
PLY_ACTION_SHELL names that action interpreter. The configured check, if any,
runs separately as %s -c CHECK; -shell and PLY_SHELL name its interpreter.
Fence labels are protocol markers; bash or zsh on a fence does not select
another interpreter. Write syntax the action interpreter accepts. POSIX shell
syntax is the portable baseline. Pipes, redirection, loops, heredocs and several
lines all work when the interpreter supports them, and the block is one command
as far as this conversation is concerned. You get back what a terminal would
have shown: stdout and stderr interleaved, and the exit status when it is not
zero.`, shellQuote(actionShell), shellQuote(checkShell))
}

// composeSystem keeps a procedure close to the goal while making the action
// wire format the final instruction the model reads. A long skill changes how
// work is done; it must not accidentally hide how Ply executes that work.
func composeSystem(base, procedure string, requireAction bool) string {
	var s strings.Builder
	s.WriteString(base)
	s.WriteString(procedure)
	if procedure != "" {
		s.WriteString(`

PLY ACTION PROTOCOL REMINDER
The procedure above changes how to do the work, not how actions run. To use a
program, end the turn with exactly one complete fenced ply block. Ply returns
that command's real result on the next turn. Do not claim the shell or a
deferred command is unavailable; send the next required block after reading
the result. A report with no block ends the run.
`)
	}
	if requireAction {
		s.WriteString(`

This invocation requires real tool interaction. At least one command must run
before a final report is accepted. If the goal requests an artifact or system
change, continue using command blocks until that effect exists and has been
inspected; merely describing the intended command is not progress.
`)
	}
	return s.String()
}

func platformName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}

func canDelegate(box *Box) bool {
	if box.Shell {
		return true
	}
	needed := map[string]bool{"mkdir": false, "mktemp": false, "mv": false}
	for _, tool := range box.Tools {
		if _, ok := needed[tool.Name]; ok {
			needed[tool.Name] = true
		}
	}
	for _, found := range needed {
		if !found {
			return false
		}
	}
	return true
}

func subagentCommand(box *Box) string {
	args := []string{`"$PLY"`}
	if box.Dir != "" {
		args = append(args, "-t", shellQuote(box.Dir))
	}
	if box.Shell {
		args = append(args, "-sh")
	}
	return strings.Join(append(args, "-turns", "12", "-C", ".", "-f", `"$run/NNN.jsonl"`, "--"), " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// firstMessage is the goal, and whatever was piped in with it. Large input
// is spooled to a file and named rather than carried in context every turn,
// so a big log becomes something to grep instead of a tax on every request.
func firstMessage(goal, stdin, spool string) string {
	switch {
	case stdin == "" && spool == "":
		return goal
	case spool != "":
		return goal + "\n\nThe input for this goal is in " + spool + " -- it was too\nlarge to put here. Read what you need out of it."
	default:
		return goal + "\n\n<stdin>\n" + stdin + "\n</stdin>"
	}
}

const (
	initialCheckStart = "[ply: initial check did not pass]"
	initialCheckEnd   = "[ply: end initial check]"
)

// withInitialCheck keeps the pre-check where both audiences can see it: in
// the first message the model receives, and therefore in the replayable Ask
// session. The markers let hone separate the goal from the terminal
// transcript without inventing a second record of the check.
func withInitialCheck(first string, r Result) string {
	return first + "\n\n" + initialCheckStart + "\n\n" + r.Typescript() +
		initialCheckEnd + "\n\nKeep working until the check passes."
}

// rejection is what a failed check says to the model. It is a typescript
// like any other tool result, because that is what it is: a command ran and
// this is what it printed.
func rejection(r Result) string {
	return "ply ran the check and it did not pass:\n\n" + r.Typescript() +
		"\nKeep working until it does."
}

func quoteDir(d string) string {
	if d == "" || d == "." {
		return "the current directory"
	}
	return d
}

func indentAfterFirst(s, pad string) string {
	return strings.ReplaceAll(s, "\n", "\n"+pad)
}

func bytesize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%dMB", n>>20)
	case n >= 1<<10:
		return fmt.Sprintf("%dKB", n>>10)
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}
