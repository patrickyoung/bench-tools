package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A session, built the way ask writes one. The tests below are about the
// gate, so they are about exit statuses and one note.
type build struct {
	goal   string
	check  string
	script []string
	notes  []noteData
}

func (b build) session() *session {
	s := &session{ID: "t", Goal: b.goal, Check: b.check, Script: b.script, Notes: b.notes}
	return s
}

func passedNote() noteData {
	return noteData{Source: "ply", Text: passedMark + ":\n\n$ go test ./...\nok\n"}
}

func failedNote() noteData {
	return noteData{Source: "ply", Text: failedMark + ":\n\n$ go test ./...\nFAIL\nexit 1\n"}
}

func verifierNote(outcome string, code int) noteData {
	body, _ := json.Marshal(verifierReceipt{Phase: "candidate", Verifier: "go test ./...", Outcome: outcome, ExitCode: code})
	return noteData{Source: "ply", Kind: verifierReceiptKind, Body: body}
}

// stumbleScript is a command that failed and a command that fixed it, in
// the typescript ply actually writes.
var stumbleScript = []string{
	"$ go test ./...\n# x\nfound packages x and main\nFAIL\nexit 1\n",
	"$ sed -i '' 's/package x/package main/' add.go\n$ go test ./...\nok  \tx\t0.25s\n",
}

// TestTheGate is the whole program's precision story. Three of these four
// runs teach nothing, and that is the point: the corpus stays small enough
// to stay true because almost nothing gets into it.
func TestTheGate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		b       build
		teaches bool
		because string
	}{
		{
			name:    "no verdict at all",
			b:       build{script: stumbleScript},
			teaches: false,
			because: "no check ran",
		},
		{
			name:    "the check never passed",
			b:       build{script: stumbleScript, notes: []noteData{failedNote()}},
			teaches: false,
			because: "never passed",
		},
		{
			name:    "passed, but nothing ever failed",
			b:       build{script: []string{"$ go test ./...\nok\n"}, notes: []noteData{passedNote()}},
			teaches: false,
			because: "nothing ever failed",
		},
		{
			name:    "failed, then passed",
			b:       build{script: stumbleScript, notes: []noteData{passedNote()}},
			teaches: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, why := tc.b.session().Teaches()
			if ok != tc.teaches {
				t.Fatalf("Teaches() = %v, want %v (%s)", ok, tc.teaches, why)
			}
			if !ok && !strings.Contains(why, tc.because) {
				t.Errorf("reason %q does not say %q", why, tc.because)
			}
			if ok && why != "" {
				t.Errorf("a run that teaches gave a reason not to: %q", why)
			}
		})
	}
}

// Only ply signs a verdict. A model can write the words "the check passed"
// in a reply, and taking that for a verdict is exactly the substitution the
// gate exists to refuse -- done is a program's opinion.
func TestOnlyPlySignsAVerdict(t *testing.T) {
	s := build{
		script: stumbleScript,
		notes:  []noteData{{Source: "deploy", Text: passedMark}},
	}.session()
	if s.Verdict() != unjudged {
		t.Fatalf("a note signed %q was read as a verdict", "deploy")
	}
	if ok, _ := s.Teaches(); ok {
		t.Error("a run judged by nobody taught something")
	}
}

// The last verdict wins: a session continued past a failure and judged
// again ended the way it ended the last time a program looked.
func TestTheLastVerdictWins(t *testing.T) {
	s := build{script: stumbleScript, notes: []noteData{failedNote(), passedNote()}}.session()
	if got := s.Verdict(); got != passed {
		t.Fatalf("Verdict() = %v, want passed", got)
	}
}

func TestStructuredVerifierReceiptsDecideVerdict(t *testing.T) {
	s := build{script: stumbleScript, notes: []noteData{
		verifierNote("rejected", 1), verifierNote("accepted", 0),
	}}.session()
	if got := s.Verdict(); got != passed {
		t.Fatalf("Verdict() = %v, want passed", got)
	}
	if ok, why := s.Teaches(); !ok {
		t.Fatalf("sealed receipt recovery did not teach: %s", why)
	}

	forged := build{script: stumbleScript, notes: []noteData{{
		Source: "deploy", Kind: verifierReceiptKind,
		Body: verifierNote("accepted", 0).Body,
	}}}.session()
	if got := forged.Verdict(); got != unjudged {
		t.Fatalf("non-Ply structured receipt decided verdict: %v", got)
	}
}

func TestParseScript(t *testing.T) {
	for _, tc := range []struct {
		name   string
		in     string
		want   []command
		digest string
	}{
		{
			name: "a command that worked prints nothing about it",
			in:   "$ ls\na.go\nb.go\n",
			want: []command{{Cmd: "ls", Output: "a.go\nb.go\n", Failed: false}},
		},
		{
			name: "a nonzero exit is the failure",
			in:   "$ false\nexit 1\n",
			want: []command{{Cmd: "false", Output: "exit 1\n", Failed: true}},
		},
		{
			name: "exit 0 spelled out is not a failure",
			in:   "$ true\n[ply: no output, exit 0]\n",
			want: []command{{Cmd: "true", Output: "[ply: no output, exit 0]\n", Failed: false}},
		},
		{
			name: "a killed command failed",
			in:   "$ sleep 999\n[ply: killed after 2m0s] exit 137\n",
			want: []command{{Cmd: "sleep 999", Output: "[ply: killed after 2m0s] exit 137\n", Failed: true}},
		},
		{
			name: "continuation lines are part of the command",
			in:   "$ cat > x <<'EOF'\n> hello\n> EOF\n[ply: no output, exit 0]\n",
			want: []command{{Cmd: "cat > x <<'EOF'\nhello\nEOF", Output: "[ply: no output, exit 0]\n"}},
		},
		{
			name: "prose with no command in it ran nothing",
			in:   "ply ran the check and it did not pass:\n\nKeep working until it does.",
			want: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := parseScript(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d commands, want %d: %+v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i].Cmd != tc.want[i].Cmd {
					t.Errorf("[%d] Cmd = %q, want %q", i, got[i].Cmd, tc.want[i].Cmd)
				}
				if got[i].Failed != tc.want[i].Failed {
					t.Errorf("[%d] Failed = %v, want %v", i, got[i].Failed, tc.want[i].Failed)
				}
				if got[i].Output != tc.want[i].Output {
					t.Errorf("[%d] Output = %q, want %q", i, got[i].Output, tc.want[i].Output)
				}
			}
		})
	}
}

// A failure with nothing after it is a complaint, not a lesson: the run
// ended there and nobody showed it was fixed.
func TestAStumbleNeedsAFix(t *testing.T) {
	s := build{script: []string{"$ go build ./...\nexit 2\n"}, notes: []noteData{passedNote()}}.session()
	if got := s.Stumbles(); len(got) != 0 {
		t.Fatalf("got %d stumbles, want 0: %+v", len(got), got)
	}
}

// A command that fails four times running is one stumble. Four would put
// four near-identical lessons in front of the next agent, which is how a
// corpus stops being readable.
func TestRepeatedFailuresAreOneStumble(t *testing.T) {
	s := build{
		script: []string{
			"$ go test ./...\nexit 1\n",
			"$ go test ./...\nexit 1\n",
			"$ go test ./...\nexit 1\n",
			"$ go vet ./...\nok\n",
		},
		notes: []noteData{passedNote()},
	}.session()
	got := s.Stumbles()
	if len(got) != 1 {
		t.Fatalf("got %d stumbles, want 1: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Fix, "go vet") {
		t.Errorf("the fix is not what finally worked: %q", got[0].Fix)
	}
}

// Evidence carries the goal, the check and the stumbles, and nothing else.
// Sending the rest would cost the window and invite a lesson grounded in
// the parts of a run that proved nothing.
func TestEvidenceIsOnlyWhatProvedSomething(t *testing.T) {
	s := build{
		goal:   "make the tests pass",
		check:  "go test ./...",
		script: append([]string{"$ ls\nsecret-listing-nobody-needs\n"}, stumbleScript...),
		notes:  []noteData{passedNote()},
	}.session()
	e := s.Evidence()
	for _, want := range []string{"make the tests pass", "go test ./...", "found packages x and main", "package main"} {
		if !strings.Contains(e, want) {
			t.Errorf("evidence is missing %q:\n%s", want, e)
		}
	}
	if strings.Contains(e, "secret-listing-nobody-needs") {
		t.Errorf("evidence carried a command that proved nothing:\n%s", e)
	}
}

func TestCheckIsLiftedFromThePlyPrompt(t *testing.T) {
	system := `You are working through a Unix shell to reach a goal.

Whether you are done is not your judgment. When you stop, ply runs

    go test ./... 2>&1

and the run ends only when that exits zero. If it does not, you will see
what it printed and keep working.
`
	if got := checkOf(system); got != "go test ./... 2>&1" {
		t.Errorf("checkOf = %q", got)
	}
	if got := checkOf("a prompt with no check in it"); got != "" {
		t.Errorf("checkOf invented a check: %q", got)
	}
}

// The joint that was open: a run loads a procedure, stumbles anyway, and
// the log has to say which procedure it was. `brief cat` prints a body
// without its frontmatter, so a skill arrives as anonymous prose -- its
// name reaches the log only because ply records it.
func TestSkillsAreReadBackFromTheLog(t *testing.T) {
	s := build{
		script: stumbleScript,
		notes: []noteData{
			{Source: "ply", Text: "loaded skill web-perf (named)\nloaded skill house (chosen by brief find)\n"},
			passedNote(),
		},
	}.session()

	got := s.Skills()
	if len(got) != 2 {
		t.Fatalf("got %d skills, want 2: %+v", len(got), got)
	}
	if got[0].Name != "web-perf" || got[0].Chosen {
		t.Errorf("[0] = %+v, want web-perf named", got[0])
	}
	if got[1].Name != "house" || !got[1].Chosen {
		t.Errorf("[1] = %+v, want house chosen", got[1])
	}
}

// Only ply's word counts here too. A model writing "loaded skill x" in a
// reply is ordinary; treating it as a record of what was loaded is the
// substitution the whole design refuses.
func TestOnlyPlyReportsWhatWasLoaded(t *testing.T) {
	s := build{
		script: stumbleScript,
		notes:  []noteData{{Source: "deploy", Text: "loaded skill invented (named)"}, passedNote()},
	}.session()
	if got := s.Skills(); len(got) != 0 {
		t.Fatalf("a note signed by something else named a skill: %+v", got)
	}
}

// Naming the procedure changes what a good lesson looks like: the stumble
// happened despite it, so what is missing is a step in it.
func TestEvidenceNamesTheProcedure(t *testing.T) {
	s := build{
		goal:   "make it fast",
		script: stumbleScript,
		notes:  []noteData{{Source: "ply", Text: "loaded skill web-perf (named)"}, passedNote()},
	}.session()
	e := s.Evidence()
	if !strings.Contains(e, "PROCEDURE") || !strings.Contains(e, "web-perf") {
		t.Errorf("evidence does not name the skill the run was following:\n%s", e)
	}
	if !strings.Contains(e, "stumbled anyway") {
		t.Errorf("evidence does not say the stumble happened despite it:\n%s", e)
	}

	// A run that loaded nothing says nothing, rather than saying so at
	// length: an empty section is a sentence the model has to read.
	plain := build{goal: "g", script: stumbleScript, notes: []noteData{passedNote()}}.session()
	if strings.Contains(plain.Evidence(), "PROCEDURE") {
		t.Errorf("evidence invented a procedure:\n%s", plain.Evidence())
	}
}
