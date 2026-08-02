// The gate.
//
// A lesson is worth keeping only if it would have changed what happened,
// and that is decided here, by arithmetic on exit statuses that are already
// in the log. A model is used to *word* a lesson and never to decide there
// is one, because a model asked whether a transcript contains a lesson will
// always find one.
//
// Three cases, and only the third teaches:
//
//   - a run that never failed — everything worked first try, so there is no
//     counterfactual and nothing to write down;
//   - a run that never passed — the last thing tried might be wrong, and a
//     lesson drawn from it is a wrong answer with the standing of a
//     precedent, which is worse than no lesson at all;
//   - a run that failed and then passed. The lesson is the difference.
//
// That third case is the whole corpus. It is why it stays small enough to
// stay true, and it is the one thing this family can do that a chat log
// cannot: `ply -check` is a program's opinion, recorded, and `ask replay
// -check` proves it months later.
package main

import (
	"fmt"
	"strings"
)

// verdict is what a program decided about a run.
type verdict int

const (
	unjudged verdict = iota // no check ran: nobody's opinion but the model's
	failed
	passed
)

func (v verdict) String() string {
	switch v {
	case passed:
		return "passed"
	case failed:
		return "failed"
	}
	return "unjudged"
}

// These are the words ply writes into its notes. They are a wire format
// between two programs, which is why they are pinned here and tested
// against a real run rather than matched loosely.
const (
	passedMark = "the check passed"
	failedMark = "the check did not pass"
	skillMark  = "loaded skill "
)

// Skills is what ply put in the system prompt, by name.
//
// The system prompt reaches the log whole, so what shaped a run is provable
// byte for byte -- but `brief cat` prints a body without its frontmatter,
// so a skill arrives as anonymous prose and its name would otherwise live
// only on stderr. ply records it, and this reads it back.
//
// It is what closes the loop. A run that loaded a procedure and stumbled
// anyway is not merely a run that stumbled: it is evidence that procedure
// is incomplete, and the lesson belongs to it rather than to wherever
// somebody happened to point -into.
func (s *session) Skills() []loadedSkill {
	var out []loadedSkill
	for _, n := range s.Notes {
		if n.Source != "ply" {
			continue
		}
		for _, line := range strings.Split(n.Text, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, skillMark) {
				continue
			}
			rest := strings.TrimPrefix(line, skillMark)
			name, how, _ := strings.Cut(rest, " ")
			if name == "" {
				continue
			}
			out = append(out, loadedSkill{Name: name, Chosen: strings.Contains(how, "chosen")})
		}
	}
	return out
}

// loadedSkill is one procedure the run was following. Chosen distinguishes
// a skill somebody named from one brief picked: a wrong guess by brief is
// itself worth knowing about, because the run then tested it.
type loadedSkill struct {
	Name   string
	Chosen bool
}

// Verdict reads how the run ended. Only a note ply signed counts: the
// verdict is the one thing in a session that a program decided, and taking
// the model's word for it instead is exactly the substitution this refuses.
func (s *session) Verdict() verdict {
	v := unjudged
	for _, n := range s.Notes {
		if n.Source != "ply" {
			continue
		}
		switch {
		case strings.Contains(n.Text, passedMark):
			v = passed
		case strings.Contains(n.Text, failedMark):
			v = failed
		}
	}
	return v
}

// stumble is a command that failed and what happened next: the output it
// produced, and the commands that ran after it until one succeeded. It is
// the contrastive pair — what did not work beside what did — and it is the
// only part of a session a lesson is ever grounded in.
type stumble struct {
	Cmd    string // the command that failed
	Output string // what it printed, including the exit line
	Fix    string // the commands that followed, up to and including the one that worked
}

// command is one entry in a ply typescript: the command, what it printed,
// and whether it worked. The typescript is a shell transcript by design —
// `$` for a command, `>` for its continuation lines — so reading it back is
// reading a shell session, which is what it was written to be.
type command struct {
	Cmd    string
	Output string
	Failed bool
}

// parseScript reads a ply typescript back into commands. Anything that is
// not a typescript — a plain message, a rejection's prose — yields nothing,
// which is correct: no commands ran in it.
func parseScript(s string) []command {
	var cmds []command
	var cur *command
	var out strings.Builder

	flush := func() {
		if cur != nil {
			// Splitting on "\n" yields a final empty field for text that
			// ends in one, which would put a blank line on the end of every
			// command's output and so into every piece of evidence sent.
			cur.Output = strings.TrimRight(out.String(), "\n")
			if cur.Output != "" {
				cur.Output += "\n"
			}
			cmds = append(cmds, *cur)
		}
		cur, out = nil, strings.Builder{}
	}
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "$ "):
			flush()
			cur = &command{Cmd: line[2:]}
		case cur == nil:
			// Prose before any command: a rejection, a goal, a note.
		case strings.HasPrefix(line, "> "):
			cur.Cmd += "\n" + line[2:]
		default:
			if failedLine(line) {
				cur.Failed = true
			}
			out.WriteString(line)
			out.WriteString("\n")
		}
	}
	flush()
	return cmds
}

// failedLine recognises the two ways a ply typescript says a command did
// not work. Silence and success is what a shell shows — nothing — so the
// absence of one of these lines is the success case, and that asymmetry is
// the reason this is a function with a comment rather than a regexp inline.
func failedLine(line string) bool {
	if strings.HasPrefix(line, "exit ") && line != "exit 0" {
		return true
	}
	// "[ply: killed after 2m0s] exit 137"
	return strings.HasPrefix(line, "[ply: killed after ")
}

// Stumbles finds every place the run went wrong and what it did about it.
//
// The fix is the commands between a failure and the next success, joined —
// which is the smallest thing that can honestly be called what was done
// about it. A failure with nothing after it is dropped: the run ended there
// and nobody showed it was fixed, so it is a complaint, not a lesson.
func (s *session) Stumbles() []stumble {
	var all []command
	for _, sc := range s.Script {
		all = append(all, parseScript(sc)...)
	}
	var out []stumble
	for i, c := range all {
		if !c.Failed {
			continue
		}
		// A command that fails again and again is one stumble, not four:
		// the fix is whatever finally worked, and repeating the complaint
		// would put four near-identical lessons in front of the model.
		if i > 0 && all[i-1].Failed {
			continue
		}
		var fix []string
		for _, n := range all[i+1:] {
			fix = append(fix, "$ "+n.Cmd)
			if !n.Failed {
				break
			}
		}
		if len(fix) == 0 {
			continue // nothing came after; the run ended on this
		}
		out = append(out, stumble{Cmd: c.Cmd, Output: c.Output, Fix: strings.Join(fix, "\n")})
	}
	return out
}

// Teaches is the gate. The reason for the message is that "nothing to
// learn" is an ordinary answer here — most runs are — and a filter that
// exits 1 without saying which of three quite different things happened
// makes a loop over an archive impossible to debug.
func (s *session) Teaches() (bool, string) {
	switch v := s.Verdict(); v {
	case unjudged:
		return false, "no check ran, so nothing judged it but the model. " +
			"Run it again with ply -check, and the verdict lands in the log"
	case failed:
		return false, "the check never passed, so there is no evidence the " +
			"last thing tried was right"
	}
	if len(s.Stumbles()) == 0 {
		return false, "the check passed and nothing ever failed: the run " +
			"had nothing to teach because it needed nothing"
	}
	return true, ""
}

// Evidence is what hone sends a model to be worded: the goal, the command
// that judged the run, and the stumbles. Nothing else in a session is
// evidence of anything, and sending the rest would cost the window and
// invite a lesson grounded in the parts that proved nothing.
func (s *session) Evidence() string {
	var b strings.Builder
	fmt.Fprintf(&b, "GOAL\n%s\n\n", strings.TrimSpace(s.Goal))
	if s.Check != "" {
		fmt.Fprintf(&b, "CHECK (this passed, so the work was done)\n%s\n\n", s.Check)
	}
	// Naming the procedure the run was already following changes what a
	// good lesson looks like: the stumble happened *despite* it, so what is
	// missing is a step in that procedure rather than a fact about the
	// world. Only the name is sent -- the body is a skill somebody else
	// wrote and hone has no business quoting it back.
	if sk := s.Skills(); len(sk) > 0 {
		var names []string
		for _, l := range sk {
			names = append(names, l.Name)
		}
		fmt.Fprintf(&b, "PROCEDURE\nThe agent was already following the skill %s, and stumbled anyway.\nSo what is missing is most likely a step in it.\n\n",
			strings.Join(names, " and "))
	}
	for i, st := range s.Stumbles() {
		fmt.Fprintf(&b, "STUMBLE %d\nthis failed:\n$ %s\n\nit printed:\n%s\nthen this was done, and worked:\n%s\n\n",
			i+1, st.Cmd, strings.TrimRight(st.Output, "\n"), st.Fix)
	}
	return b.String()
}
