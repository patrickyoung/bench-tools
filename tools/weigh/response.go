package main

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
)

const probabilityTolerance = 1e-6

// The native endpoint has returned scores/probabilities separately rounded to
// hundredths. This is an explicit compatibility allowance, not a claim that
// the provider guarantees precision. More precise values get no such allowance.
func roundingBounds(raw json.RawMessage, value float64, maximum float64) (float64, float64) {
	radius := 0.0
	if hundredth(raw) {
		radius = 0.005
	}
	return math.Max(0, value-radius), math.Min(maximum, value+radius)
}

// Inspect the literal, since float64 would erase distinctions such as
// 0.9 versus 0.90000000000000001 before deciding whether rounding is allowed.
func hundredth(raw json.RawMessage) bool {
	text, exponent := string(raw), 0
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		var err error
		exponent, err = strconv.Atoi(text[i+1:])
		if err != nil {
			return false
		}
		text = text[:i]
	}
	fraction := 0
	if i := strings.IndexByte(text, '.'); i >= 0 {
		fraction = len(text) - i - 1
	}
	trailing := 0
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '0' {
			trailing++
		} else if text[i] != '.' && text[i] != '-' {
			return exponent >= fraction-trailing-2
		}
	}
	return true // Zero is an exact hundredth regardless of its spelling.
}

type answer struct {
	Type          string                     `json:"type"`
	Value         json.RawMessage            `json:"value"`
	Probabilities map[string]json.RawMessage `json:"probabilities,omitempty"`
}

type metadata struct {
	RequestID  string                     `json:"request_id,omitempty"`
	Provider   string                     `json:"provider,omitempty"`
	Usage      json.RawMessage            `json:"usage,omitempty"`
	Confidence map[string]json.RawMessage `json:"confidence,omitempty"`
}

type result struct {
	Version int `json:"version"`
	Model   struct {
		Requested string `json:"requested"`
		Reported  string `json:"reported"`
	} `json:"model"`
	Answers  map[string]answer `json:"answers"`
	Metadata *metadata         `json:"metadata,omitempty"`
}

func number(raw json.RawMessage, min, max float64) (float64, error) {
	if len(raw) == 0 || raw[0] == '"' || raw[0] == 'n' {
		return 0, errors.New("expected a finite number in range")
	}
	n, err := strconv.ParseFloat(string(raw), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < min || n > max {
		return 0, errors.New("expected a finite number in range")
	}
	return n, nil
}

func parseResponse(raw []byte, requested string, req request) ([]byte, error) {
	if err := strictJSON(raw); err != nil {
		return nil, err
	}
	obj, err := object(raw)
	if err != nil {
		return nil, err
	}
	if err := fields(obj, "model answers", "id provider usage"); err != nil {
		return nil, err
	}
	var result result
	result.Version = 1
	result.Model.Requested = requested
	result.Model.Reported, err = textValue(obj["model"])
	if err != nil {
		return nil, errors.New("missing or invalid reported model")
	}
	answers, err := object(obj["answers"])
	if err != nil || len(answers) != len(req.Questions) {
		return nil, errors.New("answer IDs do not match requested questions")
	}
	meta := metadata{Confidence: map[string]json.RawMessage{}}
	result.Answers = make(map[string]answer, len(answers))
	for id, q := range req.Questions {
		raw, exists := answers[id]
		if !exists {
			return nil, errors.New("answer IDs do not match requested questions")
		}
		a, confidence, err := parseAnswer(raw, q)
		if err != nil {
			return nil, err
		}
		result.Answers[id] = a
		if confidence != nil {
			meta.Confidence[id] = confidence
		}
	}
	for _, field := range []string{"id", "provider"} {
		if value, ok := obj[field]; ok {
			s, err := textValue(value)
			if err != nil {
				return nil, errors.New("invalid provider metadata")
			}
			if field == "id" {
				meta.RequestID = s
			} else {
				meta.Provider = s
			}
		}
	}
	if usage, ok := obj["usage"]; ok {
		if err := validateUsage(usage); err != nil {
			return nil, err
		}
		meta.Usage = usage
	}
	if meta.RequestID != "" || meta.Provider != "" || meta.Usage != nil || len(meta.Confidence) != 0 {
		result.Metadata = &meta
	}
	return json.Marshal(result)
}

func parseAnswer(raw json.RawMessage, q question) (answer, json.RawMessage, error) {
	a := answer{Type: q.Type}
	obj, err := object(raw)
	if err != nil {
		return a, nil, err
	}
	var kind string
	if json.Unmarshal(obj["type"], &kind) != nil {
		return a, nil, errors.New("missing answer type")
	}
	expected := q.Type
	if expected == "probability" {
		expected = "noul"
	}
	if kind != expected {
		return a, nil, errors.New("answer type does not match requested question")
	}
	var confidence json.RawMessage
	if raw, ok := obj["confidence"]; ok {
		if _, err := number(raw, 0, 1); err != nil {
			return a, nil, errors.New("invalid provider confidence")
		}
		confidence = raw
	}
	switch q.Type {
	case "choice":
		if err := fields(obj, "type choice probabilities", "confidence"); err != nil {
			return a, nil, err
		}
		var choice string
		if json.Unmarshal(obj["choice"], &choice) != nil {
			return a, nil, errors.New("invalid choice value")
		}
		if _, ok := q.Options[choice]; !ok {
			return a, nil, errors.New("choice is outside requested support")
		}
		support := make(map[string]bool, len(q.Options))
		for id := range q.Options {
			support[id] = true
		}
		a.Probabilities, err = distribution(obj["probabilities"], support)
		if err != nil {
			return a, nil, err
		}
		a.Value = obj["choice"]
	case "score":
		if err := fields(obj, "type score probabilities", "confidence legend"); err != nil {
			return a, nil, err
		}
		score, err := number(obj["score"], 0, float64(len(q.Levels)-1))
		if err != nil {
			return a, nil, errors.New("score is outside requested level range")
		}
		support := make(map[string]bool, len(q.Levels))
		for i := range q.Levels {
			support[strconv.Itoa(i)] = true
		}
		a.Probabilities, err = distribution(obj["probabilities"], support)
		if err != nil {
			return a, nil, err
		}
		minimum, maximum := expectationBounds(a.Probabilities)
		scoreLow, scoreHigh := roundingBounds(obj["score"], score, float64(len(q.Levels)-1))
		if scoreHigh < minimum-probabilityTolerance || scoreLow > maximum+probabilityTolerance {
			return a, nil, errors.New("score does not match distribution expectation")
		}
		if legend, ok := obj["legend"]; ok {
			levels, err := object(legend)
			if err != nil || len(levels) != len(q.Levels) {
				return a, nil, errors.New("score legend does not match requested levels")
			}
			for i, level := range q.Levels {
				var got string
				if json.Unmarshal(levels[strconv.Itoa(i)], &got) != nil || got != level {
					return a, nil, errors.New("score legend does not match requested levels")
				}
			}
		}
		a.Value = obj["score"]
	case "probability":
		if err := fields(obj, "type noul", ""); err != nil {
			return a, nil, err
		}
		if _, err := number(obj["noul"], 0, 1); err != nil {
			return a, nil, errors.New("probability must be between zero and one")
		}
		a.Value = obj["noul"]
	}
	return a, confidence, nil
}

func distribution(raw json.RawMessage, support map[string]bool) (map[string]json.RawMessage, error) {
	probs, err := object(raw)
	if err != nil || len(probs) != len(support) {
		return nil, errors.New("a complete native probability distribution is required")
	}
	var sum, minimum, maximum float64
	for id, raw := range probs {
		if !support[id] {
			return nil, errors.New("probability distribution has invalid support")
		}
		p, err := number(raw, 0, 1)
		if err != nil {
			return nil, errors.New("invalid distribution probability")
		}
		sum += p
		low, high := roundingBounds(raw, p, 1)
		minimum += low
		maximum += high
	}
	if sum == 0 || minimum > 1+probabilityTolerance || maximum < 1-probabilityTolerance {
		return nil, errors.New("probability distribution cannot sum to one within rounding bounds")
	}
	return probs, nil
}

// Bound the expected level of a normalized distribution within the displayed
// probabilities' rounding intervals. Filling low levels first gives the minimum;
// filling high levels first gives the maximum. No adjusted values leave Weigh.
func expectationBounds(probs map[string]json.RawMessage) (float64, float64) {
	low, capacity := make([]float64, len(probs)), make([]float64, len(probs))
	var mass, baseline float64
	for i := range low {
		raw := probs[strconv.Itoa(i)]
		p, _ := number(raw, 0, 1)
		lo, hi := roundingBounds(raw, p, 1)
		low[i], capacity[i] = lo, hi-lo
		mass += lo
		baseline += float64(i) * lo
	}
	bound := func(reverse bool) float64 {
		remaining, expectation := math.Max(0, 1-mass), baseline
		for i := range low {
			if reverse {
				i = len(low) - 1 - i
			}
			add := math.Min(remaining, capacity[i])
			expectation += float64(i) * add
			remaining -= add
		}
		return expectation
	}
	return bound(false), bound(true)
}

func validateUsage(raw json.RawMessage) error {
	obj, err := object(raw)
	if err != nil {
		return errors.New("invalid usage metadata")
	}
	if err := fields(obj, "input_tokens output_tokens", "cost"); err != nil {
		return errors.New("invalid usage metadata")
	}
	for _, key := range []string{"input_tokens", "output_tokens"} {
		text := string(obj[key])
		if text == "" {
			return errors.New("invalid token usage")
		}
		for _, digit := range text {
			if digit < '0' || digit > '9' {
				return errors.New("token usage must be a nonnegative integer")
			}
		}
	}
	if raw, ok := obj["cost"]; ok {
		if _, err := number(raw, 0, math.MaxFloat64); err != nil {
			return errors.New("invalid cost metadata")
		}
	}
	return nil
}
