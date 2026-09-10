package main

import (
	"encoding/json"
	"testing"
)

func TestVerifierReceiptVersionsPreserveRecoveryAndInterruptedFailure(t *testing.T) {
	for _, kind := range []string{"ply.verifier/v1", "ply.verifier/v2"} {
		t.Run(kind, func(t *testing.T) {
			rejected, accepted := verifierNote("rejected", 1), verifierNote("accepted", 0)
			rejected.Kind, accepted.Kind = kind, kind
			s := build{script: stumbleScript, notes: []noteData{rejected, accepted}}.session()
			if ok, why := s.Teaches(); !ok {
				t.Fatalf("verified recovery did not teach: %s", why)
			}
			first := false
			data, _ := json.Marshal(accepted)
			parsed := &session{}
			if err := parsed.add(event{Seq: 3, Type: kNote, Data: data}, &first); err != nil || len(parsed.Receipts) != 1 || parsed.Verdict() != passed {
				t.Fatalf("receipt did not load: %+v %v", parsed, err)
			}
			for _, receipt := range []verifierReceipt{
				{Outcome: "accepted", ExitCode: 0, Interrupted: true},
				{Outcome: "accepted", ExitCode: 0, OutputIncomplete: true},
				{Outcome: "accepted", ExitCode: 0, Killed: true},
				{Outcome: "accepted", ExitCode: 1},
				{Outcome: "rejected", ExitCode: 0},
			} {
				body, _ := json.Marshal(receipt)
				invalid := noteData{Source: "ply", Kind: kind, Body: body}
				s.Notes = []noteData{accepted, invalid}
				if ok, _ := s.Teaches(); ok {
					t.Fatalf("contradictory outcome taught a lesson: %+v", receipt)
				}
				data, _ := json.Marshal(invalid)
				if err := parsed.add(event{Seq: 4, Type: kNote, Data: data}, &first); err == nil {
					t.Fatalf("contradictory outcome loaded: %+v", receipt)
				}
			}
			interrupted, _ := json.Marshal(verifierReceipt{Outcome: "broken", ExitCode: 0, Interrupted: true})
			s.Notes = append(s.Notes[:1], noteData{Source: "ply", Kind: kind, Body: interrupted})
			if s.Verdict() != failed {
				t.Fatal("a cancelled check retained an earlier accepted verdict")
			}
		})
	}
}
