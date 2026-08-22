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

// verdictSource attributes verifier receipts and composition notes to the
// program that observed them, rather than the model or terminal user.
const verdictSource = "ply"

// maxStalls bounds a reply that neither ran anything nor finished — an
// unterminated fence, twice. A model that cannot close a fence after being
// shown the mistake should not be allowed to turn malformed output into an
// unchecked success.
const maxStalls = 2

type Loop struct {
	Model          Model
	Runner         Runner // the model's reach: the toolbox
	Checker        Runner // the caller's reach: the caller's own PATH
	Check          string // shell command; empty means the model's word is the verdict
	Loaded         string // what ply put in the system prompt, recorded once the log exists
	Cycles         int    // rejected candidates before giving up; 0 unbounded
	Compact        bool   // carry on through a full window by compacting
	Compacts       int    // compactions before giving up; 0 unbounded
	Turns          int    // model turns before giving up; 0 unbounded
	View           *view
	SessionChanged func(string) error // process-boundary notification after compaction
	ContractID     string             // admitted intent contract digest, when a caller supplied one
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
			if l.SessionChanged != nil {
				if err := l.SessionChanged(path); err != nil {
					return last, fmt.Errorf("recording current session: %w", err)
				}
			}
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
		if note != "" {
			if stalls >= maxStalls {
				return last, fmt.Errorf("%w after %d malformed replies", ErrProtocol, stalls+1)
			}
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
		// A check is a verifier, not merely a postcondition. File and code
		// checks can ignore stdin; a question check can judge the exact report
		// that would otherwise be printed to stdout.
		r := l.Checker.RunInput(ctx, l.Check, reply)
		if ctx.Err() != nil {
			return last, ctx.Err()
		}
		l.View.Check(r)
		if err := l.recordVerifier(ctx, "candidate", reply, r); err != nil {
			return reply, err
		}
		if verifierOutcome(r) == "accepted" {
			return reply, nil
		}
		if verifierOutcome(r) == "broken" {
			return reply, checkError(r)
		}
		cycle++
		if l.Cycles > 0 && cycle >= l.Cycles {
			return reply, fmt.Errorf("%w after %d cycles", ErrCycles, cycle)
		}
		msg = rejection(r)
	}
}

func checkError(r Result) error {
	if r.StartError {
		return fmt.Errorf("%w: command interpreter could not start: %s", ErrCheck, firstLine(r.Output))
	}
	if r.Elided > 0 {
		return fmt.Errorf("%w: output exceeded the evidence cap (%d bytes, %d elided)", ErrCheck, r.Total, r.Elided)
	}
	if r.Killed {
		return fmt.Errorf("%w: timed out after %s (exit %d)", ErrCheck, r.Timeout, r.Code)
	}
	return fmt.Errorf("%w: exit %d", ErrCheck, r.Code)
}
