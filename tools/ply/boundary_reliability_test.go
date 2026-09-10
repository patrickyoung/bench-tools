package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanceledTerminalBoundaryRetainsSealedEvidence(t *testing.T) {
	bin := buildContractAsk(t)
	for _, caged := range []bool{false, true} {
		name := "external_adapter"
		if caged {
			name = "cage"
		}
		t.Run(name, func(t *testing.T) {
			work, _, server := contractEnvironment(t, bin)
			marker := filepath.Join(work, "started")
			// The shell exits with the reserved boundary status while its
			// context is canceled. This is the branch the old early ctx
			// return skipped, despite having observed terminal evidence.
			script := "trap 'printf \"\\377\"; exit 125' INT\ntouch " + shellQuote(marker) + "\nwhile :; do sleep 1; done"
			server.setResponder(func(w http.ResponseWriter, _ contractRequest, n int) {
				if n != 1 {
					t.Error("model continued after a terminal boundary")
				}
				contractResponse(w, "```ply\n"+script+"\n```", 10)
			})
			runner := Runner{Dir: work, Path: os.Getenv("PATH"), Shell: "/bin/sh", Timeout: 10 * time.Second, Cap: 1024}
			loop := Loop{Model: Model{Bin: bin, Session: filepath.Join(t.TempDir(), "session.jsonl")}, Runner: runner, Checker: runner,
				Check: "touch must-not-check", ContractID: "contract-interrupted-boundary", View: newView(io.Discard, true)}
			wantKind := externalActionBoundaryReceiptKind
			if caged {
				fakeMay(t, "spent")
				gate, err := openMayGate(os.Getenv("MAY"), "interrupted-cage")
				if err != nil {
					t.Fatal(err)
				}
				fakeCage(t, "run")
				launcher, err := openCageLauncher(os.Getenv("CAGE"), work, t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				loop.Approval, loop.Runner.Cage = gate, launcher
				wantKind = confinementReceiptKind
			} else {
				digest, err := executableDigest("test adapter", runner.Shell)
				if err != nil {
					t.Fatal(err)
				}
				loop.ActionBoundary = &externalActionBoundary{ExitCode: 125, Path: runner.Shell, SHA256: digest}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := make(chan bool, 1)
			go func() {
				deadline := time.NewTimer(5 * time.Second)
				defer deadline.Stop()
				poll := time.NewTicker(10 * time.Millisecond)
				defer poll.Stop()
				for {
					select {
					case <-poll.C:
						if _, err := os.Stat(marker); err == nil {
							cancel()
							ready <- true
							return
						}
					case <-deadline.C:
						cancel()
						ready <- false
						return
					}
				}
			}()
			answer, err := loop.Run(ctx, "one action")
			if !<-ready {
				t.Fatal("the action never reached its cancellation boundary")
			}
			if answer != "" || !errors.Is(err, ErrConfinement) {
				t.Fatalf("canceled boundary: answer=%q error=%v", answer, err)
			}
			if _, err := os.Stat(filepath.Join(work, "must-not-check")); !os.IsNotExist(err) {
				t.Fatal("verifier ran after the terminal boundary")
			}
			if len(server.requests()) != 1 {
				t.Fatal("terminal boundary caused another model request")
			}
			events := verifiedAskEvents(t, bin, loop.Model.Session)
			if len(events) < 2 || events[len(events)-2].Type != "note" || events[len(events)-1].Type != "seal" {
				t.Fatal("terminal result is not the final sealed record")
			}
			var note struct {
				Source, Kind string
				Body         json.RawMessage
			}
			if err := json.Unmarshal(events[len(events)-2].Data, &note); err != nil || note.Source != "ply" || note.Kind != wantKind {
				t.Fatalf("terminal record kind=%q source=%q error=%v", note.Kind, note.Source, err)
			}
			var body struct {
				ExitCode     int    `json:"exit_code"`
				MayHaveRun   bool   `json:"may_have_run"`
				Output       []byte `json:"output"`
				OutputBytes  int64  `json:"output_bytes"`
				OutputSHA256 string `json:"output_sha256"`
				Detail       string `json:"detail"`
			}
			if err := json.Unmarshal(note.Body, &body); err != nil {
				t.Fatal(err)
			}
			if body.ExitCode != 125 || !body.MayHaveRun || !bytes.Contains(body.Output, []byte{0xff}) || body.OutputBytes != int64(len(body.Output)) || body.OutputSHA256 != digestText(string(body.Output)) {
				t.Fatalf("terminal result lost status, uncertainty, or output bytes: %+v", body)
			}
			if !strings.Contains(body.Detail, "interrupted; action effects may exist") {
				t.Fatalf("terminal receipt hides interruption: %q", body.Detail)
			}
			if caged {
				if len(events) < 4 || events[len(events)-4].Type != "note" || events[len(events)-3].Type != "seal" {
					t.Fatal("confinement result is not adjacent to its sealed approval")
				}
				if err := json.Unmarshal(events[len(events)-4].Data, &note); err != nil || note.Kind != approvalReceiptKindV2 {
					t.Fatal("confinement result is not immediately preceded by its caged approval")
				}
			}
		})
	}
}

func TestProactiveCompactionNoopUsesSessionIdentity(t *testing.T) {
	for _, alias := range []string{"relative", "hardlink", "symlink"} {
		t.Run(alias, func(t *testing.T) {
			dir := t.TempDir()
			original := filepath.Join(dir, "original.jsonl")
			write(t, original, "existing session", 0o600)
			source, returned := original, original
			if alias == "relative" {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				source, err = filepath.Rel(cwd, original)
				if err != nil {
					t.Fatal(err)
				}
			} else if alias == "symlink" {
				source = filepath.Join(dir, "existing-source-link.jsonl")
				if err := os.Symlink(original, source); err != nil {
					t.Fatal(err)
				}
				returned = source
			} else {
				returned = filepath.Join(dir, "same-inode.jsonl")
				if err := os.Link(original, returned); err != nil {
					t.Fatal(err)
				}
			}
			bin := filepath.Join(dir, "ask")
			unexpected := filepath.Join(dir, "unexpected-append")
			write(t, bin, "#!/bin/sh\ncase \"$1\" in\ncompact) printf '%s\\n' "+shellQuote(returned)+";;\nreplay) exit 0;;\n*) touch "+shellQuote(unexpected)+"; exit 1;;\nesac\n", 0o700)
			published := false
			loop := Loop{Model: Model{Bin: bin, Session: source}, Goal: "unchanged goal", View: newView(io.Discard, true), SessionChanged: func(string) error { published = true; return nil }}
			changed, err := loop.compact(context.Background(), 1000)
			if err != nil || changed || published || loop.Model.Session != source {
				t.Fatalf("same-session query became a compaction: changed=%v published=%v session=%q err=%v", changed, published, loop.Model.Session, err)
			}
			if _, err := os.Stat(unexpected); !os.IsNotExist(err) {
				t.Fatal("no-op compaction appended the goal again")
			}
			if _, err := loop.Model.Compact(context.Background()); err == nil {
				t.Fatalf("unconditional recovery accepted unchanged session: %v", err)
			}
		})
	}
}

func TestAskAppendTornSealCannotBecomeVerifiedObservation(t *testing.T) {
	bin := buildContractAsk(t)
	work, control, server := contractEnvironment(t, bin)
	server.setResponder(func(w http.ResponseWriter, _ contractRequest, _ int) {
		contractResponse(w, "Seeded.", 10)
	})
	if code, _, stderr := runPly(t, "-sh", "-C", work, "-checkpoint", control, "seed"); code != 0 {
		t.Fatal(stderr)
	}
	path := strings.TrimSpace(read(t, control))
	model := Model{Bin: bin, Session: path}
	if err := model.Append(context.Background(), "durable-observation-token"); err != nil {
		t.Fatal(err)
	}
	verifiedAskEvents(t, bin, path)
	original := []byte(read(t, path))
	lines := bytes.SplitAfter(original, []byte{'\n'})
	// SplitAfter includes a final empty slice after the trailing newline.
	sealStart := len(original) - len(lines[len(lines)-2])
	for _, damage := range []string{"missing", "unterminated", "partial"} {
		t.Run(damage, func(t *testing.T) {
			var broken []byte
			switch damage {
			case "missing":
				broken = original[:sealStart]
			case "unterminated":
				broken = original[:len(original)-1] // valid seal JSON, missing newline
			case "partial":
				broken = original[:sealStart+17]
			}
			copy := filepath.Join(t.TempDir(), "broken.jsonl")
			write(t, copy, string(broken), 0o600)
			cmd := exec.Command(bin, "replay", "-check", "-json", copy)
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			if err := cmd.Run(); err == nil || out.Len() != 0 || !strings.Contains(stderr.String(), "not immediately sealed") {
				t.Fatalf("torn observation accepted: err=%v out=%q stderr=%q", err, out.String(), stderr.String())
			}
			if err := (Model{Bin: bin, Session: copy}).Append(context.Background(), "must not bless the torn prefix"); err == nil {
				t.Fatal("later append blessed an unsealed observation")
			}
		})
	}
	if len(server.requests()) != 1 {
		t.Fatal("observation repair or inspection called a provider")
	}
}

func TestSteeringDuringApprovalDefersActionAndRequiresFreshGrant(t *testing.T) {
	bin := buildContractAsk(t)
	work, control, server := contractEnvironment(t, bin)
	steer, grants := filepath.Join(t.TempDir(), "steer"), filepath.Join(t.TempDir(), "grants")
	write(t, steer, "", 0o600)
	mayLog := fakeMay(t, "spent")
	realMay := os.Getenv("MAY")
	wrapper := filepath.Join(t.TempDir(), "may-with-new-guidance")
	write(t, wrapper, "#!/bin/sh\nn=$(cat "+shellQuote(grants)+" 2>/dev/null || echo 0)\nn=$((n+1))\nprintf '%s\\n' \"$n\" > "+shellQuote(grants)+"\nif [ \"$n\" = 1 ]; then printf 'approval-time-guidance: inspect state before acting\\n' >> "+shellQuote(steer)+"; fi\nexec "+shellQuote(realMay)+" \"$@\"\n", 0o700)
	t.Setenv("MAY", wrapper)
	const action = "```ply\nprintf x >> effects\n```"
	server.setResponder(func(w http.ResponseWriter, r contractRequest, n int) {
		switch n {
		case 1:
			contractResponse(w, action, 10)
		case 2:
			if _, err := os.Stat(filepath.Join(work, "effects")); !os.IsNotExist(err) {
				t.Error("action ran despite guidance arriving during approval")
			}
			if strings.Count(r.userText(), "approval-time-guidance") != 1 || !strings.Contains(r.userText(), "spent grant was not used") {
				t.Errorf("provider missed deferred-grant evidence: %s", r.userText())
			}
			// A spent but deferred grant must not satisfy -require-action.
			contractResponse(w, "Premature report before an action has run.", 10)
		case 3:
			contractResponse(w, action, 10)
		case 4:
			contractResponse(w, "Completed under the fresh grant.", 10)
		default:
			t.Error("unexpected extra model request")
			contractResponse(w, "Unexpected request.", 10)
		}
	})
	code, out, stderr := runPly(t, "-sh", "-C", work, "-checkpoint", control, "-steer", steer,
		"-may-job", "approval-steering", "-require-action", "-turns", "5", "perform one effect")
	if code != 0 || out != "Completed under the fresh grant.\n" || len(server.requests()) != 4 {
		t.Fatalf("approval steering: exit=%d out=%q stderr=%q", code, out, stderr)
	}
	if read(t, filepath.Join(work, "effects")) != "x" || read(t, grants) != "2\n" {
		t.Fatal("deferred action executed or re-proposed action reused its prior grant")
	}
	if lines := strings.Count(read(t, mayLog), "\n"); lines != 2 {
		t.Fatalf("May received %d exact action envelopes, want two", lines)
	}
	events := verifiedAskEvents(t, bin, strings.TrimSpace(read(t, control)))
	approvals := 0
	for i, e := range events {
		if e.Type != "note" {
			continue
		}
		var note struct {
			Kind string
			Body json.RawMessage
		}
		if json.Unmarshal(e.Data, &note) != nil || note.Kind != approvalReceiptKind {
			continue
		}
		approvals++
		if approvals == 1 {
			if i+3 >= len(events) || events[i+1].Type != "seal" || events[i+2].Type != "user" || events[i+3].Type != "seal" {
				t.Fatal("unused spent approval lacks its durable defer observation")
			}
			var user struct{ Source, Text string }
			if json.Unmarshal(events[i+2].Data, &user) != nil || user.Source != "ply" || !strings.Contains(user.Text, "spent grant was not used") {
				t.Fatal("unused grant did not retain sourced model-visible evidence")
			}
		}
	}
	if approvals != 2 {
		t.Fatalf("sealed approvals=%d, want two", approvals)
	}
}

func TestCappedVerifierRejectionIsAvailableOnResume(t *testing.T) {
	bin := buildContractAsk(t)
	for _, limit := range []string{"turns", "cycles"} {
		t.Run(limit, func(t *testing.T) {
			work, control, server := contractEnvironment(t, bin)
			const observation = "rejection-only-token"
			server.setResponder(func(w http.ResponseWriter, _ contractRequest, n int) {
				if n == 1 {
					contractResponse(w, "First candidate.", 10)
				} else {
					contractResponse(w, "Corrected candidate.", 10)
				}
			})
			check := "if [ -f permit ]; then cat >/dev/null; else printf '%s%s\\n' 'rejection-only-' 'token'; exit 1; fi"
			args := []string{"-sh", "-B", "-C", work, "-checkpoint", control, "-check", check, "-" + limit, "1", "produce a verified answer"}
			code, out, stderr := runPly(t, args...)
			if code != 2 || out != "First candidate.\n" {
				t.Fatalf("capped rejection: exit=%d out=%q stderr=%q", code, out, stderr)
			}
			path := strings.TrimSpace(read(t, control))
			events := verifiedAskEvents(t, bin, path)
			if contractUserCount(events, "ply", observation) != 1 {
				t.Fatal("capped verifier rejection did not enter durable model context")
			}
			// Resume skips the pre-check explicitly. Only the saved observation
			// can tell the model why its preceding candidate was rejected.
			write(t, filepath.Join(work, "permit"), "", 0o600)
			code, out, stderr = runPly(t, "-sh", "-B", "-C", work, "-checkpoint", control,
				"-check", check, "continue from the prior rejection")
			if code != 0 || out != "Corrected candidate.\n" {
				t.Fatalf("rejection resume: exit=%d out=%q stderr=%q", code, out, stderr)
			}
			reqs := server.requests()
			if len(reqs) != 2 || strings.Count(reqs[1].userText(), observation) != 1 {
				t.Fatal("resumed provider did not receive exactly one saved rejection")
			}
			events = verifiedAskEvents(t, bin, path)
			if contractUserCount(events, "ply", observation) != 1 {
				t.Fatal("resume duplicated the rejected verifier result")
			}
			if len(events) < 2 || events[len(events)-2].Type != "note" || events[len(events)-1].Type != "seal" {
				t.Fatal("accepted verifier receipt stopped being terminal and sealed")
			}
			var note struct {
				Kind string
				Body struct{ Outcome string }
			}
			if json.Unmarshal(events[len(events)-2].Data, &note) != nil || note.Kind != verifierReceiptKind || note.Body.Outcome != "accepted" {
				t.Fatal("successful resume lost terminal acceptance evidence")
			}
		})
	}
}
