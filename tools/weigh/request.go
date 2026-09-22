package main

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"
)

type question struct {
	Type     string
	Text     json.RawMessage
	Options  map[string]json.RawMessage
	Levels   []json.RawMessage
	Criteria map[string]json.RawMessage
}

type request struct {
	State     json.RawMessage
	Questions map[string]question
}

type nativeQuestion struct {
	Type         string          `json:"type"`
	Instructions json.RawMessage `json:"instructions"`
	Criteria     any             `json:"criteria,omitempty"`
}

func parseRequest(raw []byte) (request, error) {
	var result request
	if err := strictJSON(raw); err != nil {
		return result, err
	}
	obj, err := object(raw)
	if err != nil {
		return result, errors.New("request must be an object")
	}
	if err := fields(obj, "version state questions", ""); err != nil {
		return result, err
	}
	if string(obj["version"]) != "1" {
		return result, errors.New("request version must be 1")
	}
	state := obj["state"]
	if len(state) == 0 || (state[0] != '"' && state[0] != '{' && state[0] != '[') {
		return result, errors.New("state must be a string, object, or array")
	}
	result.State = state
	qs, err := object(obj["questions"])
	if err != nil || len(qs) == 0 || len(qs) > maxQuestions {
		return result, errors.New("questions must contain 1 to 1024 named questions")
	}
	result.Questions = make(map[string]question, len(qs))
	for id, raw := range qs {
		if !validID(id) {
			return result, errors.New("question IDs must be nonempty text without controls (at most 256 bytes)")
		}
		q, err := parseQuestion(raw)
		if err != nil {
			return result, err // Do not echo private IDs or question contents.
		}
		result.Questions[id] = q
	}
	return result, nil
}

func validID(s string) bool {
	return len(s) <= 256 && strings.TrimSpace(s) != "" && !strings.ContainsFunc(s, unicode.IsControl)
}

func parseQuestion(raw json.RawMessage) (question, error) {
	var q question
	obj, err := object(raw)
	if err != nil {
		return q, errors.New("each question must be an object")
	}
	if err := fields(obj, "type question", "options levels criteria"); err != nil {
		return q, err
	}
	q.Type, err = textValue(obj["type"])
	if err != nil {
		return q, errors.New("question type must be choice, score, or probability")
	}
	q.Text = obj["question"]
	if !description(q.Text) {
		return q, errors.New("question must be nonempty text, an object, or an array")
	}
	switch q.Type {
	case "choice":
		if err := fields(obj, "type question options", ""); err != nil {
			return q, err
		}
		opts, err := object(obj["options"])
		if err != nil || len(opts) < 2 || len(opts) > 255 {
			return q, errors.New("choice requires 2 to 255 named options")
		}
		for id, raw := range opts {
			if !validID(id) {
				return q, errors.New("choice option IDs must be nonempty text without controls (at most 256 bytes)")
			}
			if string(raw) != "null" && !description(raw) {
				return q, errors.New("choice descriptions must be nonempty text, objects, arrays, or null")
			}
		}
		q.Options = opts
	case "score":
		if err := fields(obj, "type question levels", ""); err != nil {
			return q, err
		}
		if json.Unmarshal(obj["levels"], &q.Levels) != nil || len(q.Levels) < 2 || len(q.Levels) > 10 {
			return q, errors.New("score requires 2 to 10 ordered descriptions")
		}
		for _, level := range q.Levels {
			if !description(level) {
				return q, errors.New("score levels must be nonempty text, objects, or arrays")
			}
		}
	case "probability":
		if err := fields(obj, "type question", "criteria"); err != nil {
			return q, err
		}
		if raw, ok := obj["criteria"]; ok {
			criteria, err := object(raw)
			if err != nil || fields(criteria, "true false", "") != nil || !description(criteria["true"]) || !description(criteria["false"]) {
				return q, errors.New("probability criteria must contain exactly true and false descriptions (text, object, or array)")
			}
			q.Criteria = criteria
		}
	default:
		return q, errors.New("question type must be choice, score, or probability")
	}
	return q, nil
}

func (r request) native(model string) ([]byte, error) {
	qs := make(map[string]nativeQuestion, len(r.Questions))
	for id, q := range r.Questions {
		n := nativeQuestion{Type: q.Type, Instructions: q.Text}
		switch q.Type {
		case "choice":
			n.Criteria = q.Options
		case "score":
			n.Criteria = q.Levels
		case "probability":
			n.Type = "noul"
			if q.Criteria != nil {
				n.Criteria = q.Criteria
			}
		}
		qs[id] = n
	}
	return json.Marshal(struct {
		Model     string                    `json:"model"`
		State     json.RawMessage           `json:"state"`
		Questions map[string]nativeQuestion `json:"questions"`
	}{model, r.State, qs})
}

// The whole document has already passed strictJSON. Structured descriptions
// are provider data, not another schema; keep their numeric literals intact.
func description(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	if raw[0] == '{' || raw[0] == '[' {
		return true
	}
	_, err := textValue(raw)
	return err == nil
}
