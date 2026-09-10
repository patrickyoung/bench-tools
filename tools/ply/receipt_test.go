package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestVerifierReceiptPreservesBinaryOutput(t *testing.T) {
	output := []byte{0xff, 0x00, 'x', '\n'}
	r := Result{Cmd: "binary-check", Output: string(output), Code: 0, Total: int64(len(output))}
	receipt := receiptFor("", "candidate", "answer", Runner{Shell: "/bin/sh"}, r)
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var decoded verifierReceipt
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if verifierReceiptKind != "ply.verifier/v2" || !bytes.Contains(raw, []byte(`"output":"/wB4Cg=="`)) {
		t.Fatalf("receipt version or encoding = %s %s", verifierReceiptKind, raw)
	}
	if !bytes.Equal(decoded.Output, output) || decoded.OutputSHA256 != digestText(string(decoded.Output)) || decoded.OutputBytes != int64(len(decoded.Output)) {
		t.Fatalf("binary output did not round-trip: %+v", decoded)
	}
}

func TestVerifierReceiptBindsExactExecution(t *testing.T) {
	checker := Runner{Shell: "/bin/zsh", Dir: "/work"}
	r := Result{Cmd: "judge --strict", Output: "why\n", Code: 1, Total: 4, Timeout: 30 * time.Second}
	receipt := receiptFor("sha256:contract", "candidate", "answer\n\n", checker, r)
	if receipt.ContractID != "sha256:contract" || receipt.Phase != "candidate" || receipt.Outcome != "rejected" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if receipt.CandidateSHA256 != digestText("answer\n") {
		t.Fatalf("candidate digest = %s", receipt.CandidateSHA256)
	}
	if receipt.VerifierSHA256 != digestText("/bin/zsh\x00judge --strict") || receipt.OutputSHA256 != digestText("why\n") {
		t.Fatalf("receipt digests = %+v", receipt)
	}
	if receipt.TimeoutMS != 30000 || receipt.OutputBytes != 4 {
		t.Fatalf("receipt execution fields = %+v", receipt)
	}
}

func TestVerifierInfrastructureCannotBecomeAcceptanceOrRejection(t *testing.T) {
	for name, result := range map[string]Result{
		"interpreter start":  {Code: 1, StartError: true},
		"truncated evidence": {Code: 0, Total: 1000, Elided: 488},
		"unexpected status":  {Code: 7},
		"timeout":            {Code: exitTimeout, Killed: true},
		"cancelled zero":     {Code: 0, Killed: true},
		"cancelled reject":   {Code: 1, Killed: true},
		"interrupted zero":   {Code: 0, Interrupted: true},
		"interrupted reject": {Code: 1, Interrupted: true},
		"incomplete output":  {Code: 0, OutputIncomplete: true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := verifierOutcome(result); got != "broken" {
				t.Fatalf("outcome = %q, want broken", got)
			}
		})
	}
}
