package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type retrievalStamp struct {
	Query     string         `json:"query"`
	Connector connectorStamp `json:"connector"`
}

type connectorStamp struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

const (
	maxQueryBytes  = 1 << 20
	maxLineBytes   = 8 << 20
	maxStreamBytes = 32 << 20
)

func normalizeRecords(raw []byte, wantSource string, addRef bool, stamp *retrievalStamp) ([][]byte, error) {
	lines, err := splitJSONL(raw)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, 0, len(lines))
	total := 0
	for i, line := range lines {
		normalized, err := normalizeRecord(line, wantSource, addRef, stamp)
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", i+1, err)
		}
		if len(normalized) > maxLineBytes {
			return nil, fmt.Errorf("record %d exceeds %d bytes after normalization", i+1, maxLineBytes)
		}
		total += len(normalized) + 1 // writeRecords adds one newline per record
		if total > maxStreamBytes {
			return nil, fmt.Errorf("normalized output exceeds %d bytes", maxStreamBytes)
		}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeRecord(line []byte, wantSource string, addRef bool, stamp *retrievalStamp) ([]byte, error) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(line, &row); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	kind, err := stringField(row, "kind")
	if err != nil || kind != "context" {
		return nil, fmt.Errorf("kind must be %q", "context")
	}
	version, err := intField(row, "version")
	if err != nil || version != 1 {
		return nil, fmt.Errorf("version must be 1")
	}
	source, err := stringField(row, "source")
	if err != nil || !validName(source) {
		return nil, fmt.Errorf("source must be a valid name")
	}
	if wantSource != "" && source != wantSource {
		return nil, fmt.Errorf("source must be %q", wantSource)
	}
	id, err := stringField(row, "id")
	if err != nil || strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id must be a nonempty string")
	}
	typeName, err := stringField(row, "type")
	if err != nil || !validName(typeName) {
		return nil, fmt.Errorf("type must be a valid name")
	}
	title, err := stringField(row, "title")
	if err != nil || strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("title must be a nonempty string")
	}
	retrieved, err := stringField(row, "retrieved_at")
	if err != nil {
		return nil, fmt.Errorf("retrieved_at must be an RFC3339 string")
	}
	if _, err := time.Parse(time.RFC3339, retrieved); err != nil {
		return nil, fmt.Errorf("retrieved_at must be RFC3339: %w", err)
	}
	content, ok := row["content"]
	if !ok || bytes.Equal(bytes.TrimSpace(content), []byte("null")) {
		return nil, fmt.Errorf("content is required and may not be null")
	}
	var citation map[string]json.RawMessage
	if rawCitation, ok := row["citation"]; !ok || json.Unmarshal(rawCitation, &citation) != nil {
		return nil, fmt.Errorf("citation must be an object")
	}
	locator, err := stringField(citation, "locator")
	if err != nil || strings.TrimSpace(locator) == "" {
		return nil, fmt.Errorf("citation.locator must be a nonempty string")
	}
	wantRef := citationRef(source, id)
	if rawRef, ok := row["ref"]; ok {
		var got string
		if json.Unmarshal(rawRef, &got) != nil || got != wantRef {
			return nil, fmt.Errorf("ref does not match source and id")
		}
	} else if addRef {
		row["ref"], _ = json.Marshal(wantRef)
	} else {
		return nil, fmt.Errorf("ref is required")
	}
	if stamp != nil {
		if _, exists := row["retrieval"]; exists {
			return nil, fmt.Errorf("retrieval is stamped by context and must not be emitted by a connector")
		}
		row["retrieval"], _ = json.Marshal(stamp)
	} else if raw, exists := row["retrieval"]; exists {
		if err := validateRetrieval(raw); err != nil {
			return nil, err
		}
	}
	return json.Marshal(row)
}

func validateRetrieval(raw json.RawMessage) error {
	var stamp retrievalStamp
	if err := json.Unmarshal(raw, &stamp); err != nil {
		return fmt.Errorf("retrieval must be an object")
	}
	if strings.TrimSpace(stamp.Query) == "" {
		return fmt.Errorf("retrieval.query must be a nonempty string")
	}
	if !validName(stamp.Connector.Name) {
		return fmt.Errorf("retrieval.connector.name must be a valid name")
	}
	const prefix = "sha256:"
	digest := strings.TrimPrefix(stamp.Connector.SHA256, prefix)
	if len(digest) != sha256.Size*2 || prefix+digest != stamp.Connector.SHA256 {
		return fmt.Errorf("retrieval.connector.sha256 must be a sha256 digest")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("retrieval.connector.sha256 must be a sha256 digest")
	}
	return nil
}

func mergeRecords(records [][]byte) ([][]byte, error) {
	seen := make(map[string][]byte)
	var out [][]byte
	for _, record := range records {
		var row map[string]json.RawMessage
		_ = json.Unmarshal(record, &row)
		ref, _ := stringField(row, "ref")
		if prior, ok := seen[ref]; ok {
			if !bytes.Equal(prior, record) {
				return nil, fmt.Errorf("conflicting records for ref %q", ref)
			}
			continue
		}
		seen[ref] = record
		out = append(out, record)
	}
	return out, nil
}

func citationRef(source, id string) string {
	sum := sha256.Sum256([]byte(source + "\x00" + id))
	return "ctx:" + source + ":" + hex.EncodeToString(sum[:16])
}

func splitJSONL(raw []byte) ([][]byte, error) {
	if len(raw) > maxStreamBytes {
		return nil, fmt.Errorf("input exceeds %d bytes", maxStreamBytes)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes+1)
	var lines [][]byte
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, fmt.Errorf("blank JSONL record")
		}
		lines = append(lines, bytes.Clone(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("record exceeds %d bytes: %w", maxLineBytes, err)
	}
	return lines, nil
}

func stringField(row map[string]json.RawMessage, name string) (string, error) {
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

func intField(row map[string]json.RawMessage, name string) (int, error) {
	raw, ok := row[name]
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	var number json.Number
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&number); err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	value, err := strconv.Atoi(number.String())
	if err != nil || number.String() != strconv.Itoa(value) {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return value, nil
}
