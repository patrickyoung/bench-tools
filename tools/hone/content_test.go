package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contentTurn(body string) event {
	b, _ := json.Marshal(turn{Blocks: []block{{Type: "text", Text: body}}})
	return event{Type: kAssistant, Data: b}
}

func contentNote(candidate, outcome, output string) event {
	code := 0
	if outcome == "rejected" {
		code = 1
	}
	r := contentReceipt{verifierReceipt: verifierReceipt{Phase: "candidate", Verifier: "check --rubric fixed.json", Outcome: outcome, ExitCode: code},
		ContractID: "sha256:fixed-contract", CandidateSHA256: contentDigest(candidate),
		Shell: "/bin/sh", Directory: "/work", TimeoutMS: 30000,
		Output: []byte(output), OutputBytes: int64(len(output)), OutputSHA256: contentDigest(output)}
	r.VerifierSHA256 = contentDigest(r.Shell + "\x00" + r.Verifier)
	b, _ := json.Marshal(r)
	n, _ := json.Marshal(noteData{Source: "ply", Kind: verifierReceiptKind, Body: b})
	return event{Type: kNote, Data: n}
}

func contentEvents() []event {
	return []event{contentTurn("unresolved owner: Jamie"), contentNote("unresolved owner: Jamie\n", "rejected", "The owner is ambiguous.\n"),
		contentTurn("owner: unknown; request clarification"), contentNote("owner: unknown; request clarification\n", "accepted", "Pass.\n")}
}

func changeContentReceipt(e event, change func(*contentReceipt)) event {
	var n noteData
	_ = json.Unmarshal(e.Data, &n)
	var r contentReceipt
	_ = json.Unmarshal(n.Body, &r)
	change(&r)
	n.Body, _ = json.Marshal(r)
	e.Data, _ = json.Marshal(n)
	return e
}

func contentSession(t *testing.T, events []event) (*session, error) {
	t.Helper()
	s := &session{ID: "content", Goal: "Resolve ambiguity using supplied facts.", verified: true}
	first := false
	for i, e := range events {
		e.Seq = i + 1
		if err := s.add(e, &first); err != nil {
			return s, err
		}
	}
	return s, nil
}

func TestChangedContentRecoveryUsesBoundPair(t *testing.T) {
	s, err := contentSession(t, contentEvents())
	if err != nil {
		t.Fatal(err)
	}
	if ok, why := s.Teaches(); !ok || len(s.Stumbles()) != 0 {
		t.Fatalf("content pair required a command or did not teach: %s", why)
	}
	evidence := s.Evidence()
	for _, want := range []string{"CONTENT RECOVERY", "candidate_stdin", "unresolved owner: Jamie", "request clarification", "The owner is ambiguous.", "sha256:fixed-contract"} {
		if !strings.Contains(evidence, want) {
			t.Fatalf("evidence omitted %q: %s", want, evidence)
		}
	}
	if strings.Contains(evidence, "STUMBLE 1") {
		t.Fatal("content repair invented a command stumble")
	}
	s.verified = false
	if ok, _ := s.Teaches(); ok || strings.Contains(s.Evidence(), "CONTENT RECOVERY") {
		t.Fatal("unverified content taught a lesson")
	}
}

func TestContentRecoveryRejectsChangedChecksAndDamagedBindings(t *testing.T) {
	for name, change := range map[string]func(*contentReceipt){
		"candidate digest": func(r *contentReceipt) { r.CandidateSHA256 = contentDigest("other\n") },
		"checker digest":   func(r *contentReceipt) { r.VerifierSHA256 = contentDigest("other") },
		"output digest":    func(r *contentReceipt) { r.OutputSHA256 = contentDigest("other") },
		"output length":    func(r *contentReceipt) { r.OutputBytes++ },
		"changed command": func(r *contentReceipt) {
			r.Verifier = "check --weaker"
			r.VerifierSHA256 = contentDigest(r.Shell + "\x00" + r.Verifier)
		},
		"changed shell": func(r *contentReceipt) {
			r.Shell = "/bin/zsh"
			r.VerifierSHA256 = contentDigest(r.Shell + "\x00" + r.Verifier)
		},
		"changed directory": func(r *contentReceipt) { r.Directory = "/elsewhere" },
		"changed timeout":   func(r *contentReceipt) { r.TimeoutMS++ },
		"changed contract":  func(r *contentReceipt) { r.ContractID = "sha256:another" },
		"missing contract":  func(r *contentReceipt) { r.ContractID = "" },
		"negative timeout":  func(r *contentReceipt) { r.TimeoutMS = -1 },
		"precheck":          func(r *contentReceipt) { r.Phase = "precheck" },
		"partial output":    func(r *contentReceipt) { r.OutputIncomplete = true },
		"interrupted":       func(r *contentReceipt) { r.Interrupted = true },
		"killed":            func(r *contentReceipt) { r.Killed = true },
		"elided":            func(r *contentReceipt) { r.ElidedBytes = 1 },
		"negative elision":  func(r *contentReceipt) { r.ElidedBytes = -1 },
		"start failure":     func(r *contentReceipt) { r.StartError = true },
		"broken":            func(r *contentReceipt) { r.Outcome = "broken"; r.ExitCode = 5 },
	} {
		t.Run(name, func(t *testing.T) {
			events := contentEvents()
			events[3] = changeContentReceipt(events[3], change)
			s, err := contentSession(t, events)
			if err == nil {
				if ok, _ := s.Teaches(); ok {
					t.Fatal("invalid content pair taught a lesson")
				}
			}
		})
	}
}

func TestContentRecoveryRejectsUnprovedTransitions(t *testing.T) {
	for _, name := range []string{"unchanged", "normalization only", "first success", "empty precheck", "partial reject", "partial accept", "intervening broken", "no fresh assistant", "continued after acceptance", "legacy final", "unsupported media"} {
		t.Run(name, func(t *testing.T) {
			e := contentEvents()
			switch name {
			case "unchanged", "normalization only":
				body := "unresolved owner: Jamie"
				if name == "normalization only" {
					body = " \n" + body + "\n\n"
				}
				e[2], e[3] = contentTurn(body), contentNote("unresolved owner: Jamie\n", "accepted", "Pass.\n")
			case "first success":
				e = e[2:]
			case "empty precheck":
				e = append([]event{changeContentReceipt(contentNote("", "rejected", "Fail."), func(r *contentReceipt) { r.Phase = "precheck" })}, e[2:]...)
			case "partial reject", "partial accept":
				i := 0
				if name == "partial accept" {
					i = 2
				}
				var v turn
				_ = json.Unmarshal(e[i].Data, &v)
				v.Partial = true
				e[i].Data, _ = json.Marshal(v)
			case "intervening broken":
				e = append(e[:2], append([]event{changeContentReceipt(e[1], func(r *contentReceipt) { r.Outcome = "broken"; r.ExitCode = 7 })}, e[2:]...)...)
			case "no fresh assistant":
				e = append(e[:2], e[3])
			case "continued after acceptance":
				e = append(e, event{Type: kUser, Data: json.RawMessage(`{"text":"continue"}`)})
			case "legacy final":
				var n noteData
				_ = json.Unmarshal(e[3].Data, &n)
				n.Kind = "ply.verifier/v1"
				e[3].Data, _ = json.Marshal(n)
			case "unsupported media":
				var v turn
				_ = json.Unmarshal(e[2].Data, &v)
				v.Blocks = append(v.Blocks, block{Type: "image"})
				e[2].Data, _ = json.Marshal(v)
			}
			s, err := contentSession(t, e)
			if err == nil {
				if ok, _ := s.Teaches(); ok {
					t.Fatal("unproved content transition taught")
				}
			}
		})
	}
}

func TestContentReceiptRequiresCompleteUnambiguousFields(t *testing.T) {
	var n noteData
	_ = json.Unmarshal(contentEvents()[1].Data, &n)
	for _, name := range []string{"duplicate", "case alias", "missing", "null", "invalid base64", "trailing"} {
		t.Run(name, func(t *testing.T) {
			raw := string(n.Body)
			switch name {
			case "duplicate":
				raw = `{"outcome":"accepted",` + raw[1:]
			case "case alias":
				raw = `{"Outcome":"accepted",` + raw[1:]
			case "missing":
				var fields map[string]any
				_ = json.Unmarshal(n.Body, &fields)
				delete(fields, "output_bytes")
				b, _ := json.Marshal(fields)
				raw = string(b)
			case "null":
				var fields map[string]any
				_ = json.Unmarshal([]byte(raw), &fields)
				fields["output_bytes"] = nil
				b, _ := json.Marshal(fields)
				raw = string(b)
			case "invalid base64":
				var fields map[string]any
				_ = json.Unmarshal(n.Body, &fields)
				fields["output"] = "@bad"
				b, _ := json.Marshal(fields)
				raw = string(b)
			case "trailing":
				raw += `{}`
			}
			if _, ok := readContentReceipt([]byte(raw)); ok {
				t.Fatal("malformed receipt accepted")
			}
		})
	}
}

func TestContentCandidateNormalizationAndBinaryCheckOutput(t *testing.T) {
	v := turn{Blocks: []block{{Type: "reasoning", Text: "private reasoning never sent"}, {Type: "text", Text: " \nfirst\n"}, {Type: "text", Text: "\nsecond\n"}, {Type: "text", Text: " \n"}}}
	if got, ok := candidateInput(v); !ok || got != "first\n\nsecond\n" {
		t.Fatalf("candidate %q %v", got, ok)
	}
	e := contentEvents()
	e[1] = contentNote("unresolved owner: Jamie\n", "rejected", string([]byte{0xff, 0, 10}))
	s, err := contentSession(t, e)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Teaches(); !ok {
		t.Fatal("binary diagnostic lost valid recovery")
	}
	if !strings.Contains(s.Evidence(), `"output_base64": "/wAK"`) {
		t.Fatal("binary bytes were not retained exactly")
	}
}

func writeContentSession(t *testing.T, path string) {
	t.Helper()
	events := []event{{Type: kSession, Data: json.RawMessage(`{"id":"content","model":"m"}`)}, {Type: kUser, Data: json.RawMessage(`{"text":"Resolve ambiguity using supplied facts."}`)}}
	events = append(events, contentEvents()...)
	var b strings.Builder
	for i, e := range events {
		e.Seq = i + 1
		raw, _ := json.Marshal(e)
		b.Write(raw)
		b.WriteByte('\n')
	}
	write(t, path, b.String(), 0o600)
}

func TestWhyContentRequiresReplayAndCallsNoModel(t *testing.T) {
	sessions, _, askdir := sandbox(t, "never call")
	path := filepath.Join(sessions, "content.jsonl")
	writeContentSession(t, path)
	if code, out, err := runHone(t, "-why", path); code != 0 || !strings.Contains(out, "CONTENT RECOVERY") {
		t.Fatalf("why %d %s %s", code, out, err)
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); !os.IsNotExist(err) {
		t.Fatal("why called a model")
	}
	if code, out, _ := runHone(t, "-why", "-no-verify", path); code != 1 || out != "" {
		t.Fatal("no-verify accepted content")
	}
	t.Setenv("FAKE_REPLAY_EXIT", "1")
	if code, out, _ := runHone(t, "-why", path); code != 1 || out != "" {
		t.Fatal("bad replay accepted content")
	}
}

func TestContentProposalRetainsAdmissionAndForgetProvenance(t *testing.T) {
	sessions, skills, _ := sandbox(t, "- Preserve unresolved identity until an explicit mapping is supplied.")
	path := filepath.Join(sessions, "content.jsonl")
	writeContentSession(t, path)
	write(t, filepath.Join(skills, "house", "SKILL.md"), "---\nname: house\ndescription: Resolve ambiguous records.\n---\n\n# House\n", 0o600)
	proposal := filepath.Join(t.TempDir(), "proposal.json")
	if code, _, err := runHone(t, "-into", "house", "-prepare", proposal, path); code != 0 {
		t.Fatalf("prepare %d %s", code, err)
	}
	if code, _, err := runHone(t, "admit", proposal); code != 0 {
		t.Fatalf("admit %d %s", code, err)
	}
	if got := read(t, filepath.Join(skills, "house", "SKILL.md")); !strings.Contains(got, "<!-- hone content ") {
		t.Fatalf("missing source provenance: %s", got)
	}
	if code, _, err := runHone(t, "forget", "content", "house"); code != 0 {
		t.Fatalf("forget %d %s", code, err)
	}
}

func TestContentTornTailCannotRetainEarlierPair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "content.jsonl")
	writeContentSession(t, path)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`{"seq":9,"type":"assistant"`)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := readSession(path)
	if err != nil {
		t.Fatal(err)
	}
	s.verified = true
	if ok, _ := s.Teaches(); ok {
		t.Fatal("unfinished content turn retained an earlier pair")
	}
}

func TestContentRepeatedRejectionsRetainOnlyComparedPair(t *testing.T) {
	events := contentEvents()
	events = append(events[:2], append([]event{contentTurn("owner: still ambiguous"), contentNote("owner: still ambiguous\n", "rejected", "")}, events[2:]...)...)
	s, err := contentSession(t, events)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Teaches(); !ok {
		t.Fatal("changed final pair did not teach")
	}
	evidence := s.Evidence()
	if strings.Contains(evidence, "unresolved owner: Jamie") || !strings.Contains(evidence, "owner: still ambiguous") {
		t.Fatalf("wrong pair retained: %s", evidence)
	}
}
