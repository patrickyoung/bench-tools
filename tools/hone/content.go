package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// These fields are a public Ply v2 receipt, not an import of Ply's code.
// Version 1 output was a JSON string and cannot bind arbitrary process bytes.
type contentReceipt struct {
	verifierReceipt
	ContractID      string `json:"contract_id,omitempty"`
	CandidateSHA256 string `json:"candidate_sha256"`
	VerifierSHA256  string `json:"verifier_sha256"`
	Shell           string `json:"shell"`
	Directory       string `json:"directory"`
	TimeoutMS       int64  `json:"timeout_ms"`
	Output          []byte `json:"output,omitempty"`
	OutputSHA256    string `json:"output_sha256"`
	OutputBytes     int64  `json:"output_bytes"`
}

type checkedContent struct {
	Candidate string
	Receipt   contentReceipt
}

type contentRecovery struct {
	Rejected checkedContent
	Accepted checkedContent
}

func contentDigest(s string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s)))
}

// Ask's ordinary text output joins nonblank text blocks with a blank line,
// trims surrounding whitespace, and excludes reasoning. Ply terminates a
// nonempty report with one newline for the verifier. The receipt must agree;
// another output mode or unrecognized attachment supplies no guessed bytes.
func candidateInput(t turn) (string, bool) {
	if t.Partial {
		return "", false
	}
	var parts []string
	for _, b := range t.Blocks {
		switch b.Type {
		case "text":
			if strings.TrimSpace(b.Text) != "" {
				parts = append(parts, strings.Trim(b.Text, "\n"))
			}
		case "reasoning", "opaque":
		default:
			return "", false
		}
	}
	s := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if s == "" {
		return "", false
	}
	return s + "\n", true
}

// Only the new content path requires this complete binding. Old command
// recoveries still accept their documented v1 and minimal legacy receipts.
func readContentReceipt(raw json.RawMessage) (contentReceipt, bool) {
	var r contentReceipt
	// Reject duplicate top-level contract fields instead of silently choosing
	// the last digest or outcome. Values are flat fields in the v2 contract.
	d := json.NewDecoder(bytes.NewReader(raw))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return r, false
	}
	fields := make(map[string]json.RawMessage)
	seen := make(map[string]bool)
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return r, false
		}
		name, ok := key.(string)
		if !ok || seen[strings.ToLower(name)] {
			return r, false
		}
		seen[strings.ToLower(name)] = true
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(value, []byte("null")) {
			return r, false
		}
		fields[name] = value
	}
	if _, err := d.Token(); err != nil {
		return r, false
	}
	if _, err := d.Token(); err != io.EOF {
		return r, false
	}
	for _, key := range []string{"phase", "candidate_sha256", "verifier", "verifier_sha256", "shell", "directory", "outcome", "exit_code", "timeout_ms", "output_sha256", "output_bytes"} {
		if fields[key] == nil {
			return r, false
		}
	}
	if json.Unmarshal(raw, &r) != nil || !r.validOutcome() || r.Phase != "candidate" ||
		r.Outcome == "broken" || r.Verifier == "" || r.Shell == "" || r.TimeoutMS < 0 ||
		r.VerifierSHA256 != contentDigest(r.Shell+"\x00"+r.Verifier) ||
		r.OutputBytes != int64(len(r.Output)) || r.OutputSHA256 != contentDigest(string(r.Output)) {
		return r, false
	}
	return r, true
}

func sameContentCheck(a, b contentReceipt) bool {
	return a.VerifierSHA256 == b.VerifierSHA256 && a.Shell == b.Shell &&
		a.Verifier == b.Verifier && a.Directory == b.Directory &&
		a.TimeoutMS == b.TimeoutMS && a.ContractID == b.ContractID
}

func (s *session) resetContent() {
	s.candidate = ""
	s.rejectedContent = nil
	s.contentRepair = nil
}

func (s *session) contentVerdict(n noteData) {
	r, ok := readContentReceipt(n.Body)
	if n.Kind != verifierReceiptKind || !ok || s.candidate == "" ||
		r.CandidateSHA256 != contentDigest(s.candidate) {
		s.resetContent()
		return
	}
	current := checkedContent{Candidate: s.candidate, Receipt: r}
	s.candidate = "" // one receipt consumes one assistant candidate
	s.contentRepair = nil
	if r.Outcome == "rejected" {
		s.rejectedContent = &current
		return
	}
	if before := s.rejectedContent; before != nil && sameContentCheck(before.Receipt, r) &&
		before.Candidate != current.Candidate {
		s.contentRepair = &contentRecovery{Rejected: *before, Accepted: current}
	}
	s.rejectedContent = nil
}

// Evidence contains the exact two stdin values and their complete check
// outputs. Candidate prose is evidence, never an instruction to Hone.
func (r contentRecovery) evidence() string {
	type result struct {
		Candidate       string  `json:"candidate_stdin"`
		CandidateSHA256 string  `json:"candidate_sha256"`
		Outcome         string  `json:"outcome"`
		ExitCode        int     `json:"exit_code"`
		OutputText      *string `json:"output_text,omitempty"`
		OutputBase64    []byte  `json:"output_base64,omitempty"`
		OutputSHA256    string  `json:"output_sha256"`
	}
	convert := func(c checkedContent) result {
		r := result{Candidate: c.Candidate, CandidateSHA256: c.Receipt.CandidateSHA256,
			Outcome: c.Receipt.Outcome, ExitCode: c.Receipt.ExitCode, OutputSHA256: c.Receipt.OutputSHA256}
		if utf8.Valid(c.Receipt.Output) {
			text := string(c.Receipt.Output)
			r.OutputText = &text
		} else {
			r.OutputBase64 = c.Receipt.Output
		}
		return r
	}
	data := struct {
		Verifier       string `json:"verifier"`
		VerifierSHA256 string `json:"verifier_sha256"`
		Shell          string `json:"shell"`
		Directory      string `json:"directory"`
		TimeoutMS      int64  `json:"timeout_ms"`
		ContractID     string `json:"contract_id,omitempty"`
		Rejected       result `json:"rejected"`
		Accepted       result `json:"accepted"`
	}{r.Accepted.Receipt.Verifier, r.Accepted.Receipt.VerifierSHA256,
		r.Accepted.Receipt.Shell, r.Accepted.Receipt.Directory, r.Accepted.Receipt.TimeoutMS,
		r.Accepted.Receipt.ContractID, convert(r.Rejected), convert(r.Accepted)}
	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b)
}
