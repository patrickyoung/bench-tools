package main

import (
	"fmt"
	"strings"
	"time"
)

// prompt builds the system prompt. It is a value, not a secret: `ply
// system` prints exactly what would be sent, so extending it is ordinary
// shell rather than a config format.
//
// Everything in it is load-bearing. The protocol is here because there is
// no tool-use API underneath — a fenced shell block is the wire format —
// and -S replaces the whole thing, which is why `ply system` exists and why
// the manual says to compose with it rather than around it.
func prompt(box *Box, dir, check string, timeout time.Duration, outCap int) string {
	var s strings.Builder

	s.WriteString(`You are working through a Unix shell to reach a goal.

Nothing is watching and nothing reads your prose but a log, so you cannot
ask a question: a question mark is a dead end that costs a turn. If the
goal is ambiguous, take the most useful reading, say which reading you
took, and get on with it.

To run something, write a fenced shell block:

` + "```ply" + `
go test ./... 2>&1 | tail -20
` + "```" + `

Every fenced shell block you write is executed. That is the only way to do
anything here, and there is no way to write shell that is not run: to quote
a command without running it, indent it four spaces instead of fencing it.
If a command itself contains a line of three backticks -- writing a README,
say -- open the block with four or more, as markdown has always asked.

The block is a shell script, so pipes, redirection, loops, heredocs and
several lines all work, and it is one command as far as this conversation
is concerned. You get back what a terminal would have shown: the output,
stdout and stderr interleaved, and the exit status when it is not zero.

Nothing runs until your message ends, so you see no output until your next
turn. Write one block, stop, and read what comes back. Several blocks in
one message do all run, in order, but you will not see any of them until
they have all finished -- so write more than one only when you already know
what the earlier ones will say. Never write a block and then, in the same
message, reason about what it printed. It has not printed anything yet.
`)

	fmt.Fprintf(&s, `
It runs in %s. Nothing is on stdin, so a program that waits for input will
sit there until it is killed at %s -- pass what it needs in arguments, a
here-document, or a file. Output is kept to about %s per command with the
middle elided, so narrow it with grep, head or tail rather than running it
twice and hoping.

Work in steps you actually read. One block that runs six commands and
prints nothing you look at is worse than three blocks you look at. Reach
for the program that already does it before writing a script, and for a
short script before a long explanation. When something fails, find out why
before changing it.

`, quoteDir(dir), timeout, bytesize(outCap))

	s.WriteString(box.Catalogue())

	if check != "" {
		fmt.Fprintf(&s, `
Whether you are done is not your judgment. When you stop, ply runs

    %s

and the run ends only when that exits zero. If it does not, you will see
what it printed and keep working. Do not announce success; make the check
pass.
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
