package main

import (
	"encoding/json"
	"testing"
)

func TestInitialPlyCheckIsGoalAndStumble(t *testing.T) {
	check := "$ go test ./...\nFAIL baseline\nexit 1\n"
	body := "make the tests pass\n\n" + initialCheckStart + "\n\n" +
		check + initialCheckEnd + "\n\nKeep working until the check passes."
	b, err := json.Marshal(userData{Text: body})
	if err != nil {
		t.Fatal(err)
	}
	s := &session{Notes: []noteData{passedNote()}}
	first := true
	if err := s.add(event{Type: kUser, Data: b}, &first); err != nil {
		t.Fatal(err)
	}
	if s.Goal != "make the tests pass" {
		t.Errorf("goal = %q", s.Goal)
	}
	if len(s.Script) != 1 || s.Script[0] != check {
		t.Fatalf("initial script = %#v, want the pre-check transcript", s.Script)
	}

	// The first successful command is the demonstrated fix, exactly as it
	// is for a failure returned after any later check.
	s.Script = append(s.Script, "$ edit broken.go\n[ply: no output, exit 0]\n")
	if ok, why := s.Teaches(); !ok {
		t.Fatalf("failed pre-check followed by a fix did not teach: %s", why)
	}
	got := s.Stumbles()
	if len(got) != 1 || got[0].Cmd != "go test ./..." || got[0].Fix != "$ edit broken.go" {
		t.Fatalf("stumbles = %+v", got)
	}
}

func TestIncompleteInitialCheckMarkerStaysInGoal(t *testing.T) {
	body := "a literal example\n\n" + initialCheckStart + "\n\n$ false\nexit 1\n"
	goal, script := splitInitialCheck(body)
	if goal != body || script != "" {
		t.Fatalf("incomplete marker became evidence: goal=%q script=%q", goal, script)
	}
}

func TestAppendedInitialCheckWinsOverAnExampleInTheGoal(t *testing.T) {
	example := initialCheckStart + "\n\n$ false\nexit 1\n" + initialCheckEnd
	goal := "explain this example:\n" + example
	real := "$ go test ./...\nprinted " + initialCheckEnd + "\nexit 1\n"
	body := goal + "\n\n" + initialCheckStart + "\n\n" + real +
		initialCheckEnd + "\n\nKeep working until the check passes."

	gotGoal, gotScript := splitInitialCheck(body)
	if gotGoal != goal || gotScript != real {
		t.Fatalf("goal=%q\nscript=%q", gotGoal, gotScript)
	}
}
