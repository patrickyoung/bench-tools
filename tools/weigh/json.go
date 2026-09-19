package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxBytes     = 8 << 20
	maxDepth     = 64
	maxQuestions = 1024
	maxHeader    = 8192
)

// Check the entire JSON document before decoding any part of its contract.
// Decoder.Token plus UseNumber preserves numeric tokens and catches duplicate
// decoded keys even when their original spellings use different escapes.
func strictJSON(b []byte) error {
	if !utf8.Valid(b) || !validSurrogates(b) {
		return errors.New("invalid JSON text encoding")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := jsonValue(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing or invalid JSON")
	}
	return nil
}

func jsonValue(d *json.Decoder, depth int) error {
	t, err := d.Token()
	if err != nil {
		return errors.New("invalid JSON")
	}
	delim, compound := t.(json.Delim)
	if !compound {
		return nil
	}
	if depth >= maxDepth {
		return errors.New("JSON exceeds depth 64")
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return errors.New("invalid JSON object")
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return errors.New("duplicate or invalid JSON key")
			}
			seen[s] = true
			if err := jsonValue(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := jsonValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	end, err := d.Token()
	if err != nil || (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
		return errors.New("invalid JSON container")
	}
	return nil
}

// encoding/json otherwise silently replaces unmatched UTF-16 surrogates.
func validSurrogates(b []byte) bool {
	inString := false
	for i := 0; i < len(b); i++ {
		if b[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || b[i] != '\\' {
			continue
		}
		i++
		if i >= len(b) {
			return false
		}
		if b[i] != 'u' {
			continue
		}
		if i+4 >= len(b) {
			return false
		}
		n, err := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

func object(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, errors.New("expected JSON object")
	}
	return obj, nil
}

func fields(obj map[string]json.RawMessage, required, optional string) error {
	allowed := map[string]bool{}
	for _, key := range strings.Fields(required + " " + optional) {
		allowed[key] = true
	}
	for key := range obj {
		if !allowed[key] {
			return errors.New("unknown contract field")
		}
	}
	for _, key := range strings.Fields(required) {
		if _, ok := obj[key]; !ok {
			return errors.New("missing required field")
		}
	}
	return nil
}

func cleanText(s string) bool {
	return strings.TrimSpace(s) != "" && !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' })
}

func textValue(raw json.RawMessage) (string, error) {
	var s string
	if json.Unmarshal(raw, &s) != nil || !cleanText(s) {
		return "", errors.New("expected nonempty text")
	}
	return s, nil
}
