// The loop: ask the model, run what it wrote, repeat until it stops, then
// run the check and hand back the failure if it failed. Four steps. If a
// change makes that sentence longer it needs a very good reason.
package main

import (
	"context"
	"fmt"
	"strings"
)

// maxStalls bounds a reply that neither ran anything nor finished — an
// unterminated fence, twice. Past that the reply is taken at face value,
// because a model that cannot close a fence will not learn to on the third
// telling and the run should end somewhere a human can read.
const maxStalls = 2

type Loop struct {
	Model   Model
	Runner  Runner // the model's reach: the toolbox
	Checker Runner // the caller's reach: the caller's own PATH
	Check   string // shell command; empty means the model's word is the verdict
	Cycles  int    // failed checks before giving up; 0 unbounded
	Turns   int    // model turns before giving up; 0 unbounded
	View    *view
}

// Run works the goal. The returned string is the model's final report even
// when err says the check never passed: a run that fell short still has
// something to say, and throwing it away would make the failure harder to
// act on than it needs to be.
func (l *Loop) Run(ctx context.Context, first string) (string, error) {
	msg, last := first, ""
	stalls, turns, cycle := 0, 0, 0
	for {
		if l.Turns > 0 && turns >= l.Turns {
			return last, fmt.Errorf("%w: %d", ErrTurns, l.Turns)
		}
		reply, err := l.Model.Turn(ctx, msg)
		if err != nil {
			return last, err
		}
		turns++
		last = reply

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
			return reply, nil
		}
		cycle++
		if l.Cycles > 0 && cycle >= l.Cycles {
			return reply, fmt.Errorf("%w after %d cycles", ErrCycles, cycle)
		}
		msg = rejection(r)
	}
}
