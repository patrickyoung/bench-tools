package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func requestBytes(t *testing.T, questions any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"version": 1, "state": "evidence", "questions": questions})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRequestSupportBounds(t *testing.T) {
	for _, count := range []int{1, 2, 255, 256} {
		options := map[string]string{}
		for i := 0; i < count; i++ {
			options[fmt.Sprint(i)] = "description"
		}
		_, err := parseRequest(requestBytes(t, map[string]any{"q": map[string]any{"type": "choice", "question": "Q", "options": options}}))
		if (err == nil) != (count >= 2 && count <= 255) {
			t.Fatalf("choice count %d: %v", count, err)
		}
	}
	for _, count := range []int{1, 2, 10, 11} {
		levels := make([]string, count)
		for i := range levels {
			levels[i] = "level"
		}
		_, err := parseRequest(requestBytes(t, map[string]any{"q": map[string]any{"type": "score", "question": "Q", "levels": levels}}))
		if (err == nil) != (count >= 2 && count <= 10) {
			t.Fatalf("level count %d: %v", count, err)
		}
	}
	for _, count := range []int{1, 1024, 1025} {
		questions := map[string]any{}
		for i := 0; i < count; i++ {
			questions[fmt.Sprint(i)] = map[string]any{"type": "probability", "question": "Q"}
		}
		_, err := parseRequest(requestBytes(t, questions))
		if (err == nil) != (count <= 1024) {
			t.Fatalf("question count %d: %v", count, err)
		}
	}
	for _, id := range []string{strings.Repeat("a", 256), strings.Repeat("a", 257), "bad\nname", "bad\u0080name"} {
		_, err := parseRequest(requestBytes(t, map[string]any{id: map[string]any{"type": "probability", "question": "Q"}}))
		if (err == nil) != (len(id) == 256) {
			t.Fatalf("ID validation: %v", err)
		}
	}
}

func TestStrictJSONUnicodeAndDepth(t *testing.T) {
	valid := []string{`"\ud83d\ude00"`, `{"a":"\\ud800"}`, `{"\u0061":1}`, `[900719925474099312345678901,1e999]`, strings.Repeat("[", 64) + `0` + strings.Repeat("]", 64)}
	for _, s := range valid {
		if err := strictJSON([]byte(s)); err != nil {
			t.Fatalf("valid JSON rejected %q: %v", s, err)
		}
	}
	invalid := []string{`"\ud800"`, `"\udc00"`, `"\ud800\u0041"`, `{"a":1,"\u0061":2}`, `[1,]`, strings.Repeat("[", 65) + `0` + strings.Repeat("]", 65)}
	for _, s := range invalid {
		if err := strictJSON([]byte(s)); err == nil {
			t.Fatalf("invalid JSON accepted %q", s)
		}
	}
}

func TestDistributionsAreValidatedNotRepaired(t *testing.T) {
	req, err := parseRequest([]byte(sampleRequest))
	if err != nil {
		t.Fatal(err)
	}
	within := strings.Replace(sampleResponse, `"a":0.9`, `"a":0.9000005`, 1)
	output, err := parseResponse([]byte(within), "openrouter/test", req)
	if err != nil || !bytes.Contains(output, []byte(`"a":0.9000005`)) {
		t.Fatalf("tolerance changed probability: %v %s", err, output)
	}
	outOfBounds := strings.Replace(sampleResponse, `"a":0.9,"b":0.1`, `"a":0.90001,"b":0.10001`, 1)
	if _, err := parseResponse([]byte(outOfBounds), "openrouter/test", req); err == nil {
		t.Fatal("incorrect distribution accepted")
	}
	within = strings.Replace(sampleResponse, `"score":0.25`, `"score":0.2500005`, 1)
	output, err = parseResponse([]byte(within), "openrouter/test", req)
	if err != nil || !bytes.Contains(output, []byte(`"value":0.2500005`)) {
		t.Fatalf("tolerance rounded score: %v %s", err, output)
	}
}

func TestObservedRoundedNativeResponse(t *testing.T) {
	// Synthetic claim/passage probe of the native endpoint, 2026-09-18. The
	// service returned score 1.87 while its displayed probabilities imply 1.88.
	const observed = `{"model":"typesafe/jev-1.13-20260917","answers":{"support":{"type":"choice","choice":"supported","probabilities":{"supported":0.98,"insufficient":0.02,"contradicted":0},"confidence":0.98},"quality":{"type":"score","score":1.87,"legend":{"0":"Contradicted or no support","1":"Partial or uncertain support","2":"Direct explicit support"},"probabilities":{"0":0,"1":0.12,"2":0.88},"confidence":0.81},"true":{"type":"noul","noul":0.88}},"usage":{"input_tokens":439,"output_tokens":74,"cost":0.000018438},"id":"gen-dec-1789773497-ORHpy0L0VGkEBoyO9SxG","provider":"TypeSafe"}`
	req, err := parseRequest(requestBytes(t, map[string]any{
		"support": map[string]any{"type": "choice", "question": "Does the passage support the claim?", "options": map[string]string{"supported": "Direct support", "insufficient": "Neither", "contradicted": "Direct contradiction"}},
		"quality": map[string]any{"type": "score", "question": "How directly does the passage establish the claim?", "levels": []string{"Contradicted or no support", "Partial or uncertain support", "Direct explicit support"}},
		"true":    map[string]any{"type": "probability", "question": "The passage establishes that customers can export CSV files."},
	}))
	if err != nil {
		t.Fatal(err)
	}
	out, err := parseResponse([]byte(observed), "openrouter/~typesafe/jev-latest", req)
	if err != nil || !bytes.Contains(out, []byte(`"value":1.87`)) || !bytes.Contains(out, []byte(`"probabilities":{"0":0,"1":0.12,"2":0.88}`)) {
		t.Fatalf("rounded native response not preserved: %v %s", err, out)
	}
	for _, value := range []string{"1.86", "1.91", "2.01", "-0.01"} {
		bad := strings.Replace(observed, `"score":1.87`, `"score":`+value, 1)
		if _, err := parseResponse([]byte(bad), "openrouter/test", req); err == nil {
			t.Fatalf("inconsistent score %s accepted", value)
		}
	}
}

func TestRoundingRequiresFeasibleNormalizedDistribution(t *testing.T) {
	q := question{Type: "choice", Options: map[string]json.RawMessage{"a": json.RawMessage(`"A"`), "b": json.RawMessage(`"B"`), "c": json.RawMessage(`"C"`)}}
	for _, test := range []struct {
		probs string
		valid bool
	}{
		{`{"a":0.33,"b":0.33,"c":0.33}`, true},
		{`{"a":0.34,"b":0.33,"c":0.34}`, true},
		{`{"a":0.34,"b":0.34,"c":0.34}`, false},
		{`{"a":0.333,"b":0.333,"c":0.333}`, false},
		{`{"a":0,"b":0,"c":0}`, false},
		{`{"a":-0.001,"b":0.5,"c":0.5}`, false},
		{`{"a":1.001,"b":0,"c":0}`, false},
	} {
		raw := `{"type":"choice","choice":"a","probabilities":` + test.probs + `}`
		a, _, err := parseAnswer([]byte(raw), q)
		if (err == nil) != test.valid {
			t.Fatalf("distribution %s: %v", test.probs, err)
		}
		if test.valid {
			got, _ := json.Marshal(a.Probabilities)
			if string(got) != test.probs {
				t.Fatalf("native distribution changed: %s", got)
			}
		}
	}
	zeros, support := map[string]json.RawMessage{}, map[string]bool{}
	for i := 0; i < 255; i++ {
		id := fmt.Sprint(i)
		zeros[id], support[id] = json.RawMessage(`0`), true
	}
	raw, _ := json.Marshal(zeros)
	if _, err := distribution(raw, support); err == nil {
		t.Fatal("all-zero wide distribution accepted")
	}
}

func TestPreciseScoreRetainsTightValidation(t *testing.T) {
	q := question{Type: "score", Levels: []json.RawMessage{json.RawMessage(`"low"`), json.RawMessage(`"high"`)}}
	for _, test := range []struct {
		score string
		valid bool
	}{
		{"0.876544", true}, {"0.8765445", true}, {"0.87655", false}, {"0.88", true}, {"0.89", false},
	} {
		raw := `{"type":"score","score":` + test.score + `,"probabilities":{"0":0.123456,"1":0.876544}}`
		a, _, err := parseAnswer([]byte(raw), q)
		if (err == nil) != test.valid {
			t.Fatalf("score %s: %v", test.score, err)
		}
		if test.valid && string(a.Value) != test.score {
			t.Fatalf("native score changed: %s", a.Value)
		}
	}
}

func TestRoundingUsesDecimalLiteralPrecision(t *testing.T) {
	for _, text := range []string{"0", "-0.00", "0.9", "0.900", "9e-1", "900e-3", "0.01", "1e-2", "1.20e1"} {
		if !hundredth(json.RawMessage(text)) {
			t.Fatalf("exact hundredth rejected: %s", text)
		}
	}
	for _, text := range []string{"0.90000000000000001", "0.001", "1e-3", "1e-999999999999999999999999", "1.23e-2"} {
		if hundredth(json.RawMessage(text)) {
			t.Fatalf("precise literal given rounding allowance: %s", text)
		}
	}
	q := question{Type: "score", Levels: []json.RawMessage{json.RawMessage(`"low"`), json.RawMessage(`"high"`)}}
	raw := `{"type":"score","score":0.90000000000000001,"probabilities":{"0":0.104,"1":0.896}}`
	if _, _, err := parseAnswer([]byte(raw), q); err == nil {
		t.Fatal("precise native score was treated as rounded 0.9")
	}
}

func TestUsageRetainsIntegerPrecision(t *testing.T) {
	req, _ := parseRequest([]byte(sampleRequest))
	raw := strings.Replace(sampleResponse, `"input_tokens":123`, `"input_tokens":900719925474099312345678901`, 1)
	out, err := parseResponse([]byte(raw), "openrouter/test", req)
	if err != nil || !bytes.Contains(out, []byte(`"input_tokens":900719925474099312345678901`)) {
		t.Fatalf("usage lost precision: %v %s", err, out)
	}
}

func TestEndpointSelection(t *testing.T) {
	for _, endpoint := range []string{defaultEndpoint, "http://127.0.0.1:1234/api/alpha/decisions", "http://[::1]:1234/api/alpha/decisions"} {
		if err := validateEndpoint(endpoint); err != nil {
			t.Fatalf("valid endpoint %s: %v", endpoint, err)
		}
	}
	for _, endpoint := range []string{"http://localhost/", "http://192.168.1.2/", "http://0.0.0.0/", "ftp://127.0.0.1/", "https:///api/alpha/decisions", "https://example.com/#fragment", "https://example.com/?key=value", "https://user:secret@example.com/"} {
		if err := validateEndpoint(endpoint); err == nil {
			t.Fatalf("invalid endpoint accepted %s", endpoint)
		}
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("private-input-error") }

func TestReadFailureAndInputCancellation(t *testing.T) {
	var out, diag bytes.Buffer
	code := run(context.Background(), []string{"-m", "openrouter/test"}, brokenReader{}, &out, &diag, func(string) string { t.Fatal("read failure reached credentials"); return "" }, nil)
	if code != 1 || out.Len() != 0 || strings.Contains(diag.String(), "private-input-error") {
		t.Fatal("read error status or privacy wrong")
	}
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	out.Reset()
	diag.Reset()
	start := time.Now()
	code = run(context.Background(), []string{"-m", "openrouter/test", "-timeout", "20ms"}, r, &out, &diag, func(string) string { t.Fatal("timeout reached credentials"); return "" }, nil)
	if code != 1 || out.Len() != 0 || time.Since(start) > time.Second {
		t.Fatal("stdin timeout did not stop")
	}
}

func TestHelpAndManualContract(t *testing.T) {
	for n, line := range strings.Split(help, "\n") {
		if len(line) > 80 {
			t.Errorf("help line %d exceeds 80 columns", n+1)
		}
	}
	b, err := os.ReadFile("weigh.1")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range b {
		if v > 127 {
			t.Fatal("manual is not ASCII")
		}
	}
	for _, needle := range []string{version, "header-fd", "endpoint", "timeout", "OPENROUTER_API_KEY", "EXIT STATUS", "SYNOPSIS"} {
		if !bytes.Contains(b, []byte(needle)) {
			t.Errorf("manual missing %s", needle)
		}
	}
}
