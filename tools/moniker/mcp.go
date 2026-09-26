package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"unicode/utf8"
)

const mcpInputLimit = 4096

type mcpText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type mcpResult struct {
	Content           []mcpText   `json:"content"`
	StructuredContent *nameRecord `json:"structuredContent,omitempty"`
	IsError           bool        `json:"isError,omitempty"`
}

func mcpCommand(args []string, in io.Reader, out, diag io.Writer) int {
	fs := flag.NewFlagSet("moniker mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", "", "absolute private registry")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(out, help)
			return 0
		}
		return problem(diag, err, 2)
	}
	if fs.NArg() != 1 {
		return problem(diag, "mcp needs -dir ABS_REGISTRY and one dispatcher method", 2)
	}
	if err := registryArgument(*dir); err != nil {
		return problem(diag, err, 2)
	}
	if fs.Arg(0) != "tools/call" {
		_ = json.NewEncoder(out).Encode(map[string]any{"code": -32601, "message": "method not implemented by moniker"})
		return 1
	}
	result := mcpResult{}
	theme, err := callTheme(in)
	if err == nil {
		var name nameRecord
		name, err = generate(*dir, theme)
		if err == nil {
			result.Content = []mcpText{{Type: "text", Text: name.Name}}
			result.StructuredContent = &name
		}
	}
	if err != nil {
		result.Content = []mcpText{{Type: "text", Text: err.Error()}}
		result.IsError = true
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		return problem(diag, err, 1)
	}
	return 0
}

func callTheme(in io.Reader) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(in, mcpInputLimit+1))
	if err != nil {
		return "", fmt.Errorf("read tool parameters: %w", err)
	}
	if len(raw) > mcpInputLimit {
		return "", fmt.Errorf("tool parameters exceed %d bytes", mcpInputLimit)
	}
	params, err := strictObject(raw)
	if err != nil {
		return "", err
	}
	for key := range params {
		if key != "name" && key != "arguments" && key != "_meta" {
			return "", fmt.Errorf("unknown tool parameter %q", key)
		}
	}
	// Protocol metadata is bounded with the request but grants no authority.
	if raw, present := params["_meta"]; present {
		if _, err := strictObject(raw); err != nil {
			return "", fmt.Errorf("_meta: %w", err)
		}
	}
	var name string
	if json.Unmarshal(params["name"], &name) != nil || name != "generate_team_name" {
		return "", fmt.Errorf("tool name must be generate_team_name")
	}
	theme := "playful"
	if raw, present := params["arguments"]; present {
		arguments, err := strictObject(raw)
		if err != nil {
			return "", fmt.Errorf("arguments: %w", err)
		}
		for key, raw := range arguments {
			if key != "theme" {
				return "", fmt.Errorf("unknown tool argument %q", key)
			}
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &theme) != nil {
				return "", fmt.Errorf("theme must be a string")
			}
		}
	}
	if _, ok := themes[theme]; !ok {
		return "", fmt.Errorf("theme must be playful, space or nature")
	}
	return theme, nil
}

// Decode each parameter object explicitly to reject
// duplicate keys as well as trailing JSON; ordinary struct decoding permits them.
func strictObject(raw []byte) (map[string]json.RawMessage, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("parameters must be UTF-8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("parameters must be one JSON object")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("invalid JSON object: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("invalid JSON key")
		}
		if _, present := fields[key]; present {
			return nil, fmt.Errorf("duplicate JSON key %q", key)
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, fmt.Errorf("invalid JSON value: %w", err)
		}
		fields[key] = value
	}
	if _, err := d.Token(); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("parameters must not contain trailing data")
	}
	return fields, nil
}
