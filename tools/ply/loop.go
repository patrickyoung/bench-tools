// The loop: ask the model, run what it wrote, repeat until it stops, then
// run the check and hand back the failure if it failed. Four steps. If a
// change makes that sentence longer it needs a very good reason.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"
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
	Goal           string // original goal/input, retained verbatim across compaction
	Runner         Runner // the model's reach: the toolbox
	Checker        Runner // the caller's reach: the caller's own PATH
	Check          string // shell command; empty means the model's word is the verdict
	RequireAction  bool   // a final report is invalid until one command has run
	Loaded         string // what ply put in the system prompt, recorded once the log exists
	Cycles         int    // rejected candidates before giving up; 0 unbounded
	Compact        bool   // carry on through a full window by compacting
	CompactAt      int    // Ask-owned context threshold; 0 waits for overflow
	Compacts       int    // compactions before giving up; 0 unbounded
	Turns          int    // model turns before giving up; 0 unbounded
	View           *view
	SessionChanged func(string) error      // process-boundary notification after compaction
	ContractID     string                  // admitted intent contract digest, when a caller supplied one
	Steering       *steeringInbox          // optional operator input, read only at model-turn boundaries
	Approval       *mayGate                // optional exact-action human gate, outside the model toolbox
	ActionBoundary *externalActionBoundary // fail-closed status owned by an external adapter
}

// Run works the goal. The returned string is the model's final report even
// when err says the check never passed: a run that fell short still has
// something to say, and throwing it away would make the failure harder to
// act on than it needs to be.
func (l *Loop) Run(ctx context.Context, first string) (string, error) {
	if l.Goal == "" {
		l.Goal = first
	}
	msg, last := first, ""
	stalls, actions, turns, cycle, compacts := 0, 0, 0, 0, 0
turnLoop:
	for {
		if l.Turns > 0 && turns >= l.Turns {
			return last, fmt.Errorf("%w: %d", ErrTurns, l.Turns)
		}
		if l.CompactAt > 0 && (l.Compacts == 0 || compacts < l.Compacts) {
			if _, err := os.Stat(l.Model.Session); err == nil {
				changed, err := l.compact(ctx, l.CompactAt)
				if err != nil {
					return last, err
				}
				if changed {
					compacts++
				}
			}
		}
		if l.Steering != nil {
			guidance, err := l.Steering.Read()
			if err != nil {
				return last, err
			}
			if guidance != "" {
				msg = withSteering(msg, guidance)
				l.View.Note("operator steering included in this model turn")
			}
		}
		reply, err := l.Model.Turn(ctx, msg)
		if errors.Is(err, ErrOverflow) && l.Compact {
			// A full window is permanent, so the only way on is to carry
			// less of the conversation. ask writes the handoff note and
			// opens the new session; ply just moves into it and re-sends
			// the message the full one could not take.
			if l.Compacts > 0 && compacts >= l.Compacts {
				return last, fmt.Errorf("%w after %d compactions", ErrOverflow, compacts)
			}
			_, cerr := l.compact(ctx, 0)
			if cerr != nil {
				return last, cerr
			}
			compacts++
			continue
		}
		if err != nil {
			return last, err
		}
		turns++
		// A command proposal or malformed reply is never a final report.
		last = ""

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
		if l.Model.Progress == nil || len(cmds) == 0 {
			if len(cmds) > 0 {
				l.View.Reply(prose)
			} else {
				l.View.Reply(reply)
			}
		}
		// Guidance arriving during generation gets a chance to change the
		// proposal before execution or final acceptance. Nothing is run from
		// this response; the next model turn must account for the new input.
		if l.Steering != nil {
			guidance, err := l.Steering.Read()
			if err != nil {
				return last, err
			}
			if guidance != "" {
				deferred := withSteering("ply: the preceding response was deferred before execution or acceptance because new guidance arrived.", guidance)
				if err := l.observe(ctx, deferred); err != nil {
					return last, err
				}
				l.View.Note("new steering deferred the response before execution or acceptance")
				msg = "Continue from the operator guidance just recorded."
				continue
			}
		}

		if len(cmds) > 0 {
			stalls = 0
			for _, c := range cmds {
				var admitted *approvalReceipt
				if l.Approval != nil {
					receipt, err := l.Approval.Request(ctx, l.ContractID, c, l.Runner)
					if err != nil {
						if ctx.Err() != nil {
							return last, fmt.Errorf("%w: %w", ctx.Err(), ErrApprovalBoundary)
						}
						return last, fmt.Errorf("%w: %v", ErrApprovalBoundary, err)
					}
					if err := l.recordApproval(ctx, receipt); err != nil {
						return last, fmt.Errorf("%w: %v", ErrApprovalBoundary, err)
					}
					l.View.Approval(c, receipt)
					switch receipt.Verdict {
					case "parked":
						return last, fmt.Errorf("%w: %s", ErrApprovalParked, receipt.Digest)
					case "declined":
						return last, fmt.Errorf("%w: %s", ErrApprovalDeclined, receipt.Digest)
					case "spent":
						// The sealed receipt exists before the exact action runs.
						admitted = &receipt
					default:
						return last, fmt.Errorf("%w: unknown verdict %q", ErrApprovalBoundary, receipt.Verdict)
					}
					// Approval may take long enough for the operator to steer.
					// A spent grant remains historical evidence if its action is
					// deferred; any later proposal needs its own approval request.
					if l.Steering != nil {
						guidance, err := l.Steering.Read()
						if err != nil {
							return last, err
						}
						if guidance != "" {
							text := "ply: the approved action was deferred before execution because new guidance arrived. The preceding spent grant was not used to run this action; a later proposal requires a new approval request."
							if err := l.observe(ctx, withSteering(text, guidance)); err != nil {
								return last, err
							}
							msg = "Continue from the operator guidance just recorded."
							l.View.Note("new steering deferred the approved action before execution")
							continue turnLoop
						}
					}
				}
				if l.ActionBoundary != nil {
					if digestErr := l.ActionBoundary.checkDigest(); digestErr != nil {
						detail := digestErr.Error() + "; action did not start"
						r := Result{Cmd: c, Code: l.ActionBoundary.ExitCode, StartError: true,
							Output: detail, Total: int64(len(detail))}
						l.View.Result(r)
						if err := l.recordExternalActionBoundary(ctx, c, r, detail, false); err != nil {
							return last, fmt.Errorf("%w: %s; %v", ErrConfinement, detail, err)
						}
						return last, fmt.Errorf("%w: %s", ErrConfinement, detail)
					}
				}
				actions++
				r := l.Runner.Run(ctx, c)
				if l.ActionBoundary != nil {
					if digestErr := l.ActionBoundary.checkDigest(); digestErr != nil {
						detail := digestErr.Error() + "; action effects may exist"
						if r.Output != "" && !strings.HasSuffix(r.Output, "\n") {
							r.Output += "\n"
							r.Total++
						}
						r.Output += detail
						r.Total += int64(len(detail))
						r.Code = l.ActionBoundary.ExitCode
						l.View.Result(r)
						if err := l.recordExternalActionBoundary(ctx, c, r, detail, true); err != nil {
							return last, fmt.Errorf("%w: %s; %v", ErrConfinement, detail, err)
						}
						return last, fmt.Errorf("%w: %s", ErrConfinement, detail)
					}
				}
				l.View.Result(r)
				if l.ActionBoundary != nil && r.Code == l.ActionBoundary.ExitCode {
					detail := firstLine(r.Output)
					if detail == "failed with no diagnostic" {
						detail = fmt.Sprintf("external action adapter exited %d", r.Code)
					}
					if err := l.recordExternalActionBoundary(ctx, c, r, detail, true); err != nil {
						return last, fmt.Errorf("%w: %s; %v", ErrConfinement, detail, err)
					}
					return last, fmt.Errorf("%w: %s", ErrConfinement, detail)
				}
				if r.ConfinementFailed {
					if err := l.recordConfinement(ctx, admitted, r); err != nil {
						return last, fmt.Errorf("%w: %s; %v", ErrConfinement, r.ConfinementDetail, err)
					}
					return last, fmt.Errorf("%w: %s", ErrConfinement, r.ConfinementDetail)
				}
				// Terminal boundary receipts above already retain the result and
				// must remain adjacent to their approval and terminal in the log.
				// Ordinary results enter model-visible history before any stop.
				observation := r.Typescript()
				if note != "" {
					observation += "\nply: " + note + "\n"
				}
				if err := l.observe(ctx, observation); err != nil {
					return last, err
				}
				if ctx.Err() != nil {
					return last, ctx.Err()
				}
			}
			if note != "" {
				l.View.Note("%s", note)
			}
			msg = "Continue from the observed action result just recorded."
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
		if l.RequireAction && actions == 0 {
			note = "no command has run, so this invocation cannot end with a report. The shell is available: send exactly one complete fenced ply block as the final content of your next turn."
			if stalls >= maxStalls {
				return last, fmt.Errorf("%w after %d actionless replies", ErrProtocol, stalls+1)
			}
			stalls++
			l.View.Note("%s", note)
			msg = "ply: " + note
			continue
		}

		// No commands: the model is done. Whether that is true is now a
		// program's opinion, if a program was given one.
		last = reply
		if l.Check == "" {
			return reply, nil
		}
		// A check is a verifier, not merely a postcondition. File and code
		// checks can ignore stdin; a question check can judge the exact report
		// that would otherwise be printed to stdout.
		r := l.Checker.RunInput(ctx, l.Check, reply)
		l.View.Check(r)
		if err := l.recordVerifier(ctx, "candidate", reply, r); err != nil {
			return reply, err
		}
		if ctx.Err() != nil {
			return last, ctx.Err()
		}
		if verifierOutcome(r) == "broken" {
			return reply, checkError(r)
		}
		if verifierOutcome(r) == "rejected" {
			// A rejected result must survive a cycle/turn cap in the next
			// provider context too; its typed note is deliberately not folded.
			if err := l.observe(ctx, rejection(r)); err != nil {
				return reply, err
			}
			cycle++
			if l.Cycles > 0 && cycle >= l.Cycles {
				return reply, fmt.Errorf("%w after %d cycles", ErrCycles, cycle)
			}
		}
		if l.Steering != nil {
			guidance, err := l.Steering.Read()
			if err != nil {
				return "", err
			}
			if guidance != "" {
				if err := l.observe(ctx, withSteering("ply: the candidate was checked, but finalization was deferred because new guidance arrived.", guidance)); err != nil {
					return "", err
				}
				last = ""
				msg = "Continue from the operator guidance just recorded; inspect the current state before reporting again."
				l.View.Note("new steering deferred finalization")
				continue
			}
		}
		if verifierOutcome(r) == "accepted" {
			return reply, nil
		}
		msg = "Continue from the rejected verifier result just recorded."
	}
}

func (l *Loop) observe(ctx context.Context, text string) error {
	// Provider text must be UTF-8. Preserve invalid terminal bytes explicitly
	// in a reversible quoted representation instead of JSON replacement.
	if !utf8.ValidString(text) {
		text = fmt.Sprintf("ply: terminal observation contained non-UTF-8 bytes; the complete retained typescript follows as a Go-quoted byte string:\n%q", text)
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return l.Model.Append(recordCtx, text)
}

func (l *Loop) compact(ctx context.Context, at int) (bool, error) {
	path, err := l.Model.CompactAt(ctx, at)
	if err != nil {
		return false, err
	}
	if sameSession(path, l.Model.Session) {
		return false, nil
	}
	if l.Goal != "" {
		next := l.Model
		next.Session = path
		if err := next.Append(ctx, "PLY ACTIVE GOAL AND SUPPLIED INPUT (retained across compaction; recheck current state rather than repeating uncertain effects):\n"+l.Goal); err != nil {
			return false, fmt.Errorf("retain goal after compaction: %w", err)
		}
	}
	if l.SessionChanged != nil {
		if err := l.SessionChanged(path); err != nil {
			return false, fmt.Errorf("recording current session: %w", err)
		}
	}
	l.Model.Session = path
	l.View.Note("context compacted into %s", path)
	return true, nil
}

func withSteering(message, guidance string) string {
	return message + "\n\nOPERATOR STEERING\n" + guidance +
		"\n\nTreat this as implementation guidance only. It does not amend the admitted outcome, grant approval, change available tools, or change the verifier."
}

func checkError(r Result) error {
	if r.StartError {
		return fmt.Errorf("%w: command interpreter could not start: %s", ErrCheck, firstLine(r.Output))
	}
	if r.OutputIncomplete {
		return fmt.Errorf("%w: output incomplete because inherited pipes did not close (exit %d)", ErrCheck, r.Code)
	}
	if r.Elided > 0 {
		return fmt.Errorf("%w: output exceeded the evidence cap (%d bytes, %d elided)", ErrCheck, r.Total, r.Elided)
	}
	if r.Killed {
		return fmt.Errorf("%w: timed out after %s (exit %d)", ErrCheck, r.Timeout, r.Code)
	}
	return fmt.Errorf("%w: exit %d", ErrCheck, r.Code)
}
