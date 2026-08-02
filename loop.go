// The loop: ask the model, run what it wrote, repeat until it stops, then
// run the check and hand back the failure if it failed. Four steps. If a
// change makes that sentence longer it needs a very good reason.
package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// verdictSource stamps the note ply writes when the check reaches a
// terminal answer. It is the program's name because that is who ran the
// command: a reader, and hone(1), can tell at a glance that nobody typed
// this and no model wrote it.
const verdictSource = "ply"

// maxStalls bounds a reply that neither ran anything nor finished — an
// unterminated fence, twice. Past that the reply is taken at face value,
// because a model that cannot close a fence will not learn to on the third
// telling and the run should end somewhere a human can read.
const maxStalls = 2

type Loop struct {
	Model    Model
	Runner   Runner // the model's reach: the toolbox
	Checker  Runner // the caller's reach: the caller's own PATH
	Check    string // shell command; empty means the model's word is the verdict
	Loaded   string // what ply put in the system prompt, recorded once the log exists
	Cycles   int    // failed checks before giving up; 0 unbounded
	Compact  bool   // carry on through a full window by compacting
	Compacts int    // compactions before giving up; 0 unbounded
	Turns    int    // model turns before giving up; 0 unbounded
	View     *view
}

// verdict records how the run ended, in the session, where ply has always
// said everything worth recording goes. Until it did, a session held every
// command that ran and nothing about whether the work was done — so a run
// that passed and a run that gave up were the same shape on disk, and
// nothing reading the log afterwards could tell them apart.
//
// It is a note rather than a message because the run is over and it is
// addressed to a later reader. It is written at exactly the two points the
// check reaches a terminal answer, and nowhere else: a failing check that
// the loop carries on from is already in the conversation as the rejection
// the model was handed, and recording it twice would say it happened twice.
//
// Best effort. A run that did the work and then could not write a line
// about it did the work, and the exit status still says so.
func (l *Loop) verdict(ctx context.Context, r Result) {
	if l.Model.Session == "" {
		return
	}
	// context.Canceled would fail the write for an interruption that has
	// nothing to do with the verdict; the check already ran and this is
	// what it said.
	if err := l.Model.Note(context.WithoutCancel(ctx), verdictSource, verdictText(r)); err != nil {
		l.View.Note("could not record the verdict: %v", err)
	}
}

// verdictText is the typescript with a line saying what it decided. The
// typescript alone would leave a reader to infer the verdict from an exit
// status that a passing command does not print, which is the ambiguity
// this whole thing exists to remove.
func verdictText(r Result) string {
	outcome := "the check passed"
	if r.Code != 0 {
		outcome = "the check did not pass"
	}
	return outcome + ":\n\n" + r.Typescript()
}

// Run works the goal. The returned string is the model's final report even
// when err says the check never passed: a run that fell short still has
// something to say, and throwing it away would make the failure harder to
// act on than it needs to be.
func (l *Loop) Run(ctx context.Context, first string) (string, error) {
	msg, last := first, ""
	stalls, turns, cycle, compacts := 0, 0, 0, 0
	for {
		if l.Turns > 0 && turns >= l.Turns {
			return last, fmt.Errorf("%w: %d", ErrTurns, l.Turns)
		}
		reply, err := l.Model.Turn(ctx, msg)
		if errors.Is(err, ErrOverflow) && l.Compact {
			// A full window is permanent, so the only way on is to carry
			// less of the conversation. ask writes the handoff note and
			// opens the new session; ply just moves into it and re-sends
			// the message the full one could not take.
			path, cerr := l.Model.Compact(ctx)
			if cerr != nil {
				return last, cerr
			}
			l.View.Note("context was full; compacted into %s", path)
			l.Model.Session = path
			compacts++
			if l.Compacts > 0 && compacts >= l.Compacts {
				return last, fmt.Errorf("%w after %d compactions", ErrOverflow, compacts)
			}
			continue
		}
		if err != nil {
			return last, err
		}
		turns++
		last = reply

		// The first turn is what creates the session file -- ask mints it,
		// because ask owns the log -- so this is the earliest moment there
		// is anything to write a note in. Written here rather than at the
		// end so that a run which dies mid-way still says what it was
		// following.
		if l.Loaded != "" {
			if err := l.Model.Note(ctx, verdictSource, l.Loaded); err != nil {
				l.View.Note("could not record what was loaded: %v", err)
			}
			l.Loaded = ""
		}

		// The prose goes to the typescript; the commands in it do not,
		// because each is about to appear under a real prompt with what it
		// printed. A reply that ends the run is shown whole.
		cmds, prose, note := commands(reply)
		if len(cmds) > 0 {
			l.View.Reply(prose)
		} else {
			l.View.Reply(reply)
		}

		if len(cmds) > 0 {
			stalls = 0
			var b strings.Builder
			for _, c := range cmds {
				r := l.Runner.Run(ctx, c)
				if ctx.Err() != nil {
					return last, ctx.Err()
				}
				l.View.Result(r)
				b.WriteString(r.Typescript())
				b.WriteString("\n")
			}
			if note != "" {
				l.View.Note("%s", note)
				b.WriteString("ply: " + note + "\n")
			}
			msg = b.String()
			continue
		}
		if note != "" && stalls < maxStalls {
			stalls++
			l.View.Note("%s", note)
			msg = "ply: " + note
			continue
		}

		// No commands: the model is done. Whether that is true is now a
		// program's opinion, if a program was given one.
		if l.Check == "" {
			return reply, nil
		}
		r := l.Checker.Run(ctx, l.Check)
		if ctx.Err() != nil {
			return last, ctx.Err()
		}
		l.View.Check(r)
		if r.Code == 0 {
			l.verdict(ctx, r)
			return reply, nil
		}
		cycle++
		if l.Cycles > 0 && cycle >= l.Cycles {
			l.verdict(ctx, r)
			return reply, fmt.Errorf("%w after %d cycles", ErrCycles, cycle)
		}
		msg = rejection(r)
	}
}
