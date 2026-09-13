package boundary

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// ReadObject rejects truncated input, trailing values, and duplicate members.
// A contradictory task state or request identity is not a trustworthy record.
func ReadObject(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("JSON exceeds byte limit")
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return nil, errors.New("expected one JSON object")
	}
	if err := CheckJSON(b); err != nil {
		return nil, err
	}
	return b, nil
}

func CheckJSON(b []byte) error {
	if !utf8.Valid(b) {
		return errors.New("JSON must be valid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 128 {
			return errors.New("JSON nesting exceeds limit")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return errors.New("duplicate or invalid JSON member")
				}
				seen[name] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return errors.New("unterminated JSON object")
			}
		case json.Delim('['):
			for d.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return errors.New("unterminated JSON array")
			}
		case json.Delim('}'), json.Delim(']'):
			return errors.New("unexpected JSON delimiter")
		}
		return nil
	}
	if err := value(0); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}
