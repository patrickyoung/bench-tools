package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const structuredRequest = `{"version":1,"state":"selected evidence","questions":{
"route":{"type":"choice","question":{"ask":"Which?","record":900719925474099312345678901},"options":{"a":{"covers":["A"],"not_for":"B"},"b":["B",{"example":1e999}],"other":null}},
"quality":{"type":"score","question":["How much?",{"focus":"evidence"}],"levels":[{"meaning":"low","record":900719925474099312345678901},["high",{"example":1e999}]]},
"true":{"type":"probability","question":{"ask":"Is it true?"},"criteria":{"true":{"includes":["explicit evidence"]},"false":["No evidence",{"missing":true}]}}
}}`

const structuredResponse = `{"model":"typesafe/fixture","answers":{
"route":{"type":"choice","choice":"other","probabilities":{"a":0.1,"b":0.2,"other":0.7}},
"quality":{"type":"score","score":0.25,"probabilities":{"0":0.75,"1":0.25},"legend":{"0":{"record":900719925474099312345678901,"meaning":"\u006cow"},"1":["high",{"example":1e999}]}},
"true":{"type":"noul","noul":0.2}},"usage":{"input_tokens":12,"output_tokens":3}}`

func TestStructuredDescriptionsOverHTTP(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		var wire struct {
			Questions map[string]struct {
				Type         string          `json:"type"`
				Instructions json.RawMessage `json:"instructions"`
				Criteria     json.RawMessage `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Error(err)
			return
		}
		if len(wire.Questions) != 3 || wire.Questions["true"].Type != "noul" ||
			string(wire.Questions["route"].Instructions) != `{"ask":"Which?","record":900719925474099312345678901}` ||
			string(wire.Questions["route"].Criteria) != `{"a":{"covers":["A"],"not_for":"B"},"b":["B",{"example":1e999}],"other":null}` ||
			string(wire.Questions["quality"].Instructions) != `["How much?",{"focus":"evidence"}]` ||
			string(wire.Questions["quality"].Criteria) != `[{"meaning":"low","record":900719925474099312345678901},["high",{"example":1e999}]]` ||
			string(wire.Questions["true"].Criteria) != `{"false":["No evidence",{"missing":true}],"true":{"includes":["explicit evidence"]}}` {
			t.Errorf("structured descriptions changed on wire: %s", body)
		}
		io.WriteString(w, structuredResponse)
	}))
	defer server.Close()
	code, out, diag := invoke(t, server.URL, structuredRequest)
	if code != 0 || calls.Load() != 1 || diag != "" || !strings.Contains(out, `"value":"other"`) || !strings.Contains(out, `"value":0.25`) {
		t.Fatalf("code=%d calls=%d diag=%s out=%s", code, calls.Load(), diag, out)
	}
}

func TestInvalidDescriptionsFailBeforeCredentials(t *testing.T) {
	cases := []string{
		`{"type":"probability","question":null}`,
		`{"type":"probability","question":true}`,
		`{"type":"probability","question":42}`,
		`{"type":"probability","question":"Q","criteria":null}`,
		`{"type":"probability","question":"Q","criteria":[]}`,
		`{"type":"probability","question":"Q","criteria":{"true":"Y"}}`,
		`{"type":"probability","question":"Q","criteria":{"true":"Y","false":"N","extra":"secret"}}`,
		`{"type":"probability","question":"Q","criteria":{"true":null,"false":"N"}}`,
		`{"type":"probability","question":"Q","criteria":{"true":"Y","false":false}}`,
		`{"type":"probability","question":"Q","criteria":{"true":" ","false":"N"}}`,
		`{"type":"choice","question":"Q","options":{"a":true,"b":"B"}}`,
		`{"type":"choice","question":"Q","options":{"a":12,"b":"B"}}`,
		`{"type":"choice","question":"Q","options":{"a":null,"b":null},"criteria":{"true":"Y","false":"N"}}`,
		`{"type":"score","question":"Q","levels":[{},null]}`,
		`{"type":"score","question":"Q","levels":[{},false]}`,
		`{"type":"score","question":"Q","levels":[{},1]}`,
		`{"type":"score","question":"Q","levels":[{}," "]}`,
		`{"type":"probability","question":{"private-secret":1,"private-secret":2}}`,
		`{"type":"probability","question":{"nested":"\ud800"}}`,
	}
	for i, q := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var out, diag bytes.Buffer
			input := `{"version":1,"state":"evidence","questions":{"q":` + q + `}}`
			code := run(context.Background(), []string{"-m", "openrouter/test"}, strings.NewReader(input), &out, &diag,
				func(string) string { t.Fatal("invalid description reached credentials"); return "" }, nil)
			if code != 2 || out.Len() != 0 || diag.Len() == 0 || strings.Contains(diag.String(), "secret") {
				t.Fatalf("code=%d out=%s diag=%s", code, &out, &diag)
			}
		})
	}
}

func TestStructuredLegendRejectsChanges(t *testing.T) {
	req, err := parseRequest([]byte(structuredRequest))
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range [][2]string{
		{`900719925474099312345678901`, `900719925474099312345678900`},
		{`1e999`, `1e998`},
		{`"\u006cow"`, `"high"`},
		{`["high",{"example":1e999}]`, `[{"example":1e999},"high"]`},
		{`"1":["high",{"example":1e999}]`, `"2":["high",{"example":1e999}]`},
		{`{"record":900719925474099312345678901,"meaning":"\u006cow"}`, `null`},
	} {
		bad := strings.Replace(structuredResponse, change[0], change[1], 1)
		if out, err := parseResponse([]byte(bad), "openrouter/test", req); err == nil || len(out) != 0 {
			t.Fatalf("changed legend accepted: %s", bad)
		}
	}
}
