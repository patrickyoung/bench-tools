package main

import (
	"testing"
	"time"
)

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
	} {
		t.Run(name, func(t *testing.T) {
			if got := verifierOutcome(result); got != "broken" {
				t.Fatalf("outcome = %q, want broken", got)
			}
		})
	}
}
