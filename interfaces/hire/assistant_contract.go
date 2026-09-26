package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type assistantItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}
type assistantMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}
type assistantRequest struct {
	Version         int                 `json:"version"`
	Message         string              `json:"message"`
	History         []assistantMessage  `json:"history"`
	Focus           string              `json:"focus"`
	Catalog         []assistantItem     `json:"catalog"`
	Diagnostics     map[string]any      `json:"diagnostics"`
	KnowledgeSource string              `json:"knowledge_source"`
	Context         map[string]any      `json:"context"`
	AllowedTargets  map[string][]string `json:"allowed_targets"`
}
type assistantRole struct {
	Role           string `json:"role"`
	Worker         string `json:"worker"`
	Responsibility string `json:"responsibility"`
}
type assistantAction struct {
	Kind   string            `json:"kind"`
	Label  string            `json:"label"`
	Target string            `json:"target"`
	Fields map[string]string `json:"fields"`
	Roles  []assistantRole   `json:"roles"`
}
type assistantReply struct {
	Version     int               `json:"version"`
	RequestHash string            `json:"request_sha256"`
	Message     string            `json:"message"`
	Question    string            `json:"question"`
	Actions     []assistantAction `json:"actions"`
}

func digestText(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// The UI validates independently of the worker's checker. JSON is only data;
// duplicate keys, unknown fields and trailing values never become instructions.
func strictJSON(b []byte, out any) error {
	scan := json.NewDecoder(bytes.NewReader(b))
	var value func() error
	value = func() error {
		tok, err := scan.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for scan.More() {
				t, e := scan.Token()
				if e != nil {
					return e
				}
				key, ok := t.(string)
				if !ok || seen[key] {
					return fmt.Errorf("duplicate or invalid JSON key")
				}
				seen[key] = true
				if e = value(); e != nil {
					return e
				}
			}
			_, err = scan.Token()
			return err
		case '[':
			for scan.More() {
				if err = value(); err != nil {
					return err
				}
			}
			_, err = scan.Token()
			return err
		default:
			return fmt.Errorf("invalid JSON delimiter")
		}
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := scan.Token(); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func inTargets(req assistantRequest, kind, target string) bool {
	for _, v := range req.AllowedTargets[kind] {
		if target == v {
			return true
		}
	}
	return false
}
func boundedText(s string, n int, required bool) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && utf8.RuneCountInString(s) <= n && (!required || strings.TrimSpace(s) != "")
}
func validateAssistantAction(req assistantRequest, action assistantAction) error {
	if !boundedText(action.Label, 80, true) || action.Fields == nil || action.Roles == nil {
		return fmt.Errorf("incomplete action")
	}
	fields := map[string]int{}
	switch action.Kind {
	case "create_worker":
		fields = map[string]int{"title": 120, "goal": 12000, "reads": 1000, "writes": 1000, "good": 1500, "bad": 1500}
	case "create_team":
		fields = map[string]int{"title": 120, "goal": 12000, "handoffs": 8000, "acceptance": 4000}
	case "revise_worker":
		fields = map[string]int{"change": 12000}
		if action.Target == "" || !inTargets(req, action.Kind, action.Target) {
			return fmt.Errorf("revision target was not selected")
		}
	case "analyze_run":
		fields = map[string]int{"question": 12000}
		if action.Target == "" || !inTargets(req, action.Kind, action.Target) {
			return fmt.Errorf("run target was not selected")
		}
	case "open":
		if action.Target == "" || !inTargets(req, action.Kind, action.Target) {
			return fmt.Errorf("navigation target is unavailable")
		}
	default:
		return fmt.Errorf("unsupported action")
	}
	if action.Kind == "create_worker" || action.Kind == "create_team" {
		if action.Target != "" {
			return fmt.Errorf("creation cannot specify a target")
		}
	}
	if len(fields) != len(action.Fields) {
		return fmt.Errorf("unexpected action fields")
	}
	for k, n := range fields {
		if !boundedText(action.Fields[k], n, true) {
			return fmt.Errorf("invalid %s", k)
		}
	}
	if action.Kind != "create_team" {
		if len(action.Roles) != 0 {
			return fmt.Errorf("unexpected roles")
		}
		return nil
	}
	if len(action.Roles) < 2 || len(action.Roles) > 16 {
		return fmt.Errorf("team needs 2 to 16 roles")
	}
	workers := map[string]bool{}
	for _, item := range req.Catalog {
		if item.Kind == "worker" && strings.HasPrefix(item.Key, "worker:") {
			workers[item.Key] = true
		}
	}
	seen := map[string]bool{}
	for _, r := range action.Roles {
		if !sourceID.MatchString(r.Role) || len(r.Role) > 40 || r.Role == "state" || seen[r.Role] || !workers[r.Worker] || !inTargets(req, "create_team", r.Worker) || !boundedText(r.Responsibility, 2000, true) {
			return fmt.Errorf("invalid team role")
		}
		seen[r.Role] = true
	}
	return nil
}
func decodeAssistantReply(raw, request []byte) (assistantReply, error) {
	var req assistantRequest
	var reply assistantReply
	if len(raw) > 128<<10 || len(request) > 2<<20 || !utf8.Valid(raw) || !utf8.Valid(request) {
		return reply, fmt.Errorf("reply is not bounded UTF-8")
	}
	if err := strictJSON(request, &req); err != nil {
		return reply, err
	}
	if err := strictJSON(raw, &reply); err != nil {
		return reply, err
	}
	if req.Version != 1 || reply.Version != 1 || reply.RequestHash != digestText(request) || !boundedText(reply.Message, 12000, true) || !boundedText(reply.Question, 1000, false) || reply.Actions == nil || len(reply.Actions) > 3 {
		return reply, fmt.Errorf("reply did not match this request")
	}
	// Require every key and reject null throughout the reply. Go zero values
	// otherwise hide omitted targets, questions and nested fields.
	var shape any
	_ = json.Unmarshal(raw, &shape)
	if err := assistantShape(shape); err != nil {
		return reply, err
	}
	// Required keys must exist, even for an empty question or action list.
	var keys map[string]json.RawMessage
	_ = json.Unmarshal(raw, &keys)
	if len(keys) != 5 {
		return reply, fmt.Errorf("incomplete reply")
	}
	for _, act := range reply.Actions {
		if err := validateAssistantAction(req, act); err != nil {
			return reply, err
		}
	}
	return reply, nil
}

func assistantShape(value any) error {
	switch v := value.(type) {
	case nil:
		return fmt.Errorf("null reply value")
	case string:
		if strings.ContainsRune(v, 0) {
			return fmt.Errorf("NUL reply text")
		}
	case []any:
		for _, item := range v {
			if err := assistantShape(item); err != nil {
				return err
			}
		}
	case map[string]any:
		expected := 0
		if _, ok := v["kind"]; ok {
			expected = 5
		}
		if _, ok := v["role"]; ok {
			expected = 3
		}
		if expected > 0 && len(v) != expected {
			return fmt.Errorf("incomplete reply object")
		}
		for _, item := range v {
			if err := assistantShape(item); err != nil {
				return err
			}
		}
	}
	return nil
}
