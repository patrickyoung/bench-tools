package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	maxEvidenceBytes  = 32 << 20
	maxEvidenceLine   = 8 << 20
	maxCandidateBytes = 4 << 20
)

type sourceRef struct {
	ref string
	url string
}

func loadEvidence(path string) (map[string]sourceRef, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open evidence: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat evidence: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("evidence is not a regular file")
	}
	raw, err := readBounded(f, maxEvidenceBytes)
	if err != nil {
		return nil, fmt.Errorf("read evidence: %w", err)
	}
	return parseEvidence(raw)
}

func parseEvidence(raw []byte) (map[string]sourceRef, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), maxEvidenceLine+1)
	refs := make(map[string]sourceRef)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, fmt.Errorf("evidence line %d: blank JSONL record", lineNo)
		}
		ref, err := parseEvidenceRecord(line)
		if err != nil {
			return nil, fmt.Errorf("evidence line %d: %w", lineNo, err)
		}
		if prior, ok := refs[ref.ref]; ok && prior.url != ref.url {
			return nil, fmt.Errorf("evidence line %d: conflicting URLs for ref %q", lineNo, ref.ref)
		}
		refs[ref.ref] = ref
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("evidence record exceeds %d bytes: %w", maxEvidenceLine, err)
	}
	return refs, nil
}

func parseEvidenceRecord(line []byte) (sourceRef, error) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(line, &row); err != nil {
		return sourceRef{}, fmt.Errorf("invalid JSON: %w", err)
	}
	kind, err := textField(row, "kind")
	if err != nil || kind != "context" {
		return sourceRef{}, fmt.Errorf("kind must be %q", "context")
	}
	version, err := numberField(row, "version")
	if err != nil || version != 1 {
		return sourceRef{}, errors.New("version must be 1")
	}
	ref, err := textField(row, "ref")
	if err != nil || !validRef(ref) {
		return sourceRef{}, errors.New("ref must have the form ctx:source:value")
	}
	var citation map[string]json.RawMessage
	if raw, ok := row["citation"]; !ok || json.Unmarshal(raw, &citation) != nil {
		return sourceRef{}, errors.New("citation must be an object")
	}
	url := ""
	if raw, ok := citation["url"]; ok {
		if err := json.Unmarshal(raw, &url); err != nil {
			return sourceRef{}, errors.New("citation.url must be a string")
		}
	}
	return sourceRef{ref: ref, url: url}, nil
}

func textField(row map[string]json.RawMessage, name string) (string, error) {
	raw, ok := row[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func numberField(row map[string]json.RawMessage, name string) (int, error) {
	raw, ok := row[name]
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return value, nil
}

func validRef(ref string) bool {
	parts := strings.Split(ref, ":")
	if len(parts) != 3 || parts[0] != "ctx" || parts[1] == "" || parts[2] == "" {
		return false
	}
	for _, part := range parts[1:] {
		for _, r := range part {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
				r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}
