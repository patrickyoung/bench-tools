package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

const verifierReceiptKind = "ply.verifier/v2"

// verifierReceipt is the machine-readable evidence for one verifier run.
// The candidate remains in the assistant event and the verifier output is
// repeated here so the receipt is useful on its own; the digests bind both.
// Version 2 encodes captured output as base64, preserving arbitrary process
// bytes that JSON strings in version 1 would replace with U+FFFD.
type verifierReceipt struct {
	ContractID       string `json:"contract_id,omitempty"`
	Phase            string `json:"phase"`
	CandidateSHA256  string `json:"candidate_sha256"`
	Verifier         string `json:"verifier"`
	VerifierSHA256   string `json:"verifier_sha256"`
	Shell            string `json:"shell"`
	Directory        string `json:"directory"`
	Outcome          string `json:"outcome"`
	ExitCode         int    `json:"exit_code"`
	Killed           bool   `json:"killed,omitempty"`
	Interrupted      bool   `json:"interrupted,omitempty"`
	OutputIncomplete bool   `json:"output_incomplete,omitempty"`
	StartError       bool   `json:"start_error,omitempty"`
	TimeoutMS        int64  `json:"timeout_ms"`
	Output           []byte `json:"output,omitempty"`
	OutputSHA256     string `json:"output_sha256"`
	OutputBytes      int64  `json:"output_bytes"`
	ElidedBytes      int64  `json:"elided_bytes,omitempty"`
}

func digestText(s string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s)))
}

func receiptFor(contractID, phase, candidate string, checker Runner, r Result) verifierReceipt {
	outcome := verifierOutcome(r)
	return verifierReceipt{
		ContractID: contractID, Phase: phase,
		CandidateSHA256: digestText(verifierInput(candidate)),
		Verifier:        r.Cmd, VerifierSHA256: digestText(checker.Shell + "\x00" + r.Cmd),
		Shell: checker.Shell, Directory: checker.Dir,
		Outcome: outcome, ExitCode: r.Code, Killed: r.Killed, Interrupted: r.Interrupted, StartError: r.StartError,
		OutputIncomplete: r.OutputIncomplete,
		TimeoutMS:        r.Timeout.Milliseconds(), Output: []byte(r.Output),
		OutputSHA256: digestText(r.Output), OutputBytes: r.Total, ElidedBytes: r.Elided,
	}
}

func verifierOutcome(r Result) string {
	if r.Killed || r.Interrupted || r.OutputIncomplete || r.StartError || r.Elided > 0 || r.Code != 0 && r.Code != 1 {
		return "broken"
	}
	if r.Code == 1 {
		return "rejected"
	}
	return "accepted"
}

func (l *Loop) recordVerifier(ctx context.Context, phase, candidate string, r Result) error {
	if l.Model.Session == "" {
		return fmt.Errorf("record verifier receipt: no Ask session")
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := l.Model.Record(ctx, verdictSource, verifierReceiptKind,
		receiptFor(l.ContractID, phase, candidate, l.Checker, r)); err != nil {
		return fmt.Errorf("record verifier receipt: %w", err)
	}
	return nil
}
