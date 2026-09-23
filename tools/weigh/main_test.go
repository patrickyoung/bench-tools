package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

const sampleRequest = `{"version":1,"state":{"private":"selected evidence","large":900719925474099312345678901,"nested":[true,null,1e999]},"questions":{"route":{"type":"choice","question":"Which?","options":{"a":"Option A","b":"Option B"}},"quality":{"type":"score","question":"How much?","levels":["low","high"]},"true":{"type":"probability","question":"Is it true?"}}}`
const sampleResponse = `{"model":"typesafe/jev-1.13","id":"request-1","provider":"TypeSafe","answers":{"route":{"type":"choice","choice":"a","probabilities":{"a":0.9,"b":0.1},"confidence":0.8},"quality":{"type":"score","score":0.25,"probabilities":{"0":0.75,"1":0.25},"legend":{"0":"low","1":"high"},"confidence":0.5},"true":{"type":"noul","noul":0.2}},"usage":{"input_tokens":123,"output_tokens":0,"cost":0.00001}}`

func invoke(t *testing.T, endpoint, input string, extra ...string) (int, string, string) {
	t.Helper()
	args := []string{"-m", "openrouter/~typesafe/jev-latest", "-endpoint", endpoint}
	args = append(args, extra...)
	var out, diag bytes.Buffer
	code := run(context.Background(), args, strings.NewReader(input), &out, &diag,
		func(name string) string {
			if name != "OPENROUTER_API_KEY" {
				t.Errorf("unexpected environment lookup %q", name)
			}
			return "fixture-secret"
		}, func(int) (io.ReadCloser, error) { return nil, errors.New("no descriptor") })
	return code, out.String(), diag.String()
}

func TestNativeProtocolAndPreservation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/api/alpha/decisions" {
			t.Errorf("wrong native operation: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer fixture-secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing native headers")
		}
		body, _ := io.ReadAll(r.Body)
		if !bytes.Contains(body, []byte("900719925474099312345678901")) || !bytes.Contains(body, []byte("1e999")) {
			t.Error("state numeric literals lost precision")
		}
		var wire struct {
			Model     string `json:"model"`
			Questions map[string]struct {
				Type         string          `json:"type"`
				Instructions string          `json:"instructions"`
				Criteria     json.RawMessage `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.Unmarshal(body, &wire); err != nil {
			t.Fatal(err)
		}
		if wire.Model != "~typesafe/jev-latest" || wire.Questions["true"].Type != "noul" || wire.Questions["route"].Instructions != "Which?" || string(wire.Questions["quality"].Criteria) != `["low","high"]` || wire.Questions["true"].Criteria != nil {
			t.Errorf("wrong native mapping: %s", body)
		}
		io.WriteString(w, sampleResponse)
	}))
	defer server.Close()
	code, out, diag := invoke(t, server.URL+"/api/alpha/decisions", sampleRequest)
	if code != 0 || diag != "" || calls.Load() != 1 {
		t.Fatalf("code=%d calls=%d diag=%s", code, calls.Load(), diag)
	}
	var got result
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.Model.Requested != "openrouter/~typesafe/jev-latest" || got.Model.Reported != "typesafe/jev-1.13" || got.Metadata.RequestID != "request-1" {
		t.Fatalf("missing result identity: %s", out)
	}
	if string(got.Answers["quality"].Value) != "0.25" || len(got.Answers["route"].Probabilities) != 2 || got.Answers["true"].Probabilities != nil || string(got.Answers["true"].Value) != "0.2" || len(got.Metadata.Confidence) != 2 {
		t.Fatalf("lost native answer data: %s", out)
	}
	if strings.Contains(out+diag, "fixture-secret") || !strings.HasSuffix(out, "\n") {
		t.Fatal("secret leaked or final newline absent")
	}
}

func TestEncodedRequestBoundBeforeCredentials(t *testing.T) {
	// Rendered page text can contain many HTML-sensitive characters. Go's
	// default JSON encoding expands each '<' to six bytes. The public input
	// fits, but its native representation must not bypass the request limit.
	input := strings.Replace(sampleRequest, "selected evidence", strings.Repeat("<", int(maxBytes/6)), 1)
	if int64(len(input)) >= maxBytes {
		t.Fatal("test input should fit the public input bound")
	}
	var out, diag bytes.Buffer
	for _, header := range []bool{false, true} {
		args := []string{"-m", "openrouter/test"}
		if header {
			args = append(args, "-header-fd", "3")
		}
		code := run(context.Background(), args, strings.NewReader(input), &out, &diag,
			func(string) string { t.Fatal("oversized native request read credentials"); return "" },
			func(int) (io.ReadCloser, error) { t.Fatal("oversized native request opened header"); return nil, nil })
		if code != 2 || out.Len() != 0 || !strings.Contains(diag.String(), "encoded provider request exceeds") {
			t.Fatalf("native request bound: code=%d out=%d diag=%s", code, out.Len(), diag.String())
		}
	}
}

func TestInvalidRequestsNeverCallProvider(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	cases := []string{
		``, `null`, `{}`, sampleRequest + `{}`, strings.Replace(sampleRequest, `"version":1`, `"version":2`, 1),
		strings.Replace(sampleRequest, `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(sampleRequest, `"version":1`, `"version":1,"\u0076ersion":1`, 1),
		strings.Replace(sampleRequest, `"private":"selected evidence"`, `"private":"a","private":"b"`, 1),
		strings.Replace(sampleRequest, `"version":1`, `"version":1,"extra":"secret"`, 1),
		`{"version":1,"state":12,"questions":{}}`,
		`{"version":1,"state":{},"questions":{}}`,
		`{"version":1,"state":null,"questions":{"p":{"type":"probability","question":"Q"}}}`,
		`{"version":1,"state":true,"questions":{"p":{"type":"probability","question":"Q"}}}`,
		`{"version":1,"state":{},"questions":{"":{"type":"probability","question":"Q"}}}`,
		`{"version":1,"state":{},"questions":{"p":{"type":"probability","question":" "}}}`,
		`{"version":1,"state":{},"questions":{"p":{"type":"probability","question":"Q","options":{}}}}`,
		`{"version":1,"state":{},"questions":{"p":{"type":"choice","question":"Q","options":{"a":"A"}}}}`,
		`{"version":1,"state":{},"questions":{"p":{"type":"score","question":"Q","levels":["A"]}}}`,
		`{"version":1,"state":{},"questions":{"p":{"type":"score","question":"Q","levels":["A",null]}}}`,
		`{"version":1,"state":{},"questions":{"p":{"type":"text","question":"Q"}}}`,
		`{"version":1,"state":"\ud800","questions":{"p":{"type":"probability","question":"Q"}}}`,
		`{"version":1,"state":"\udc00","questions":{"p":{"type":"probability","question":"Q"}}}`,
		strings.Replace(sampleRequest, "selected evidence", string([]byte{0xff}), 1),
		`{"version":1,"state":` + strings.Repeat("[", 65) + `0` + strings.Repeat("]", 65) + `,"questions":{}}`,
	}
	for i, input := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			code, out, diag := invoke(t, server.URL, input)
			if code != 2 || out != "" || diag == "" || strings.Contains(diag, "selected evidence") || strings.Contains(diag, "secret") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out, diag)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid input made %d calls", calls.Load())
	}
}

func alteredResponse(t *testing.T, alter func(map[string]any)) string {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal([]byte(sampleResponse), &obj); err != nil {
		t.Fatal(err)
	}
	alter(obj)
	b, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInvalidResponsesAreNotResults(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"missing answer":    func(o map[string]any) { delete(o["answers"].(map[string]any), "route") },
		"extra answer":      func(o map[string]any) { o["answers"].(map[string]any)["extra"] = map[string]any{} },
		"missing model":     func(o map[string]any) { delete(o, "model") },
		"unknown top field": func(o map[string]any) { o["unexpected"] = "private-secret" },
		"missing distribution": func(o map[string]any) {
			delete(o["answers"].(map[string]any)["route"].(map[string]any), "probabilities")
		},
		"missing score distribution": func(o map[string]any) {
			delete(o["answers"].(map[string]any)["quality"].(map[string]any), "probabilities")
		},
		"extra choice":       func(o map[string]any) { o["answers"].(map[string]any)["route"].(map[string]any)["choice"] = "outside" },
		"null choice":        func(o map[string]any) { o["answers"].(map[string]any)["route"].(map[string]any)["choice"] = nil },
		"wrong type":         func(o map[string]any) { o["answers"].(map[string]any)["true"].(map[string]any)["type"] = "probability" },
		"noul range":         func(o map[string]any) { o["answers"].(map[string]any)["true"].(map[string]any)["noul"] = 1.01 },
		"noul string":        func(o map[string]any) { o["answers"].(map[string]any)["true"].(map[string]any)["noul"] = "0.2" },
		"score range":        func(o map[string]any) { o["answers"].(map[string]any)["quality"].(map[string]any)["score"] = 1.01 },
		"score inconsistent": func(o map[string]any) { o["answers"].(map[string]any)["quality"].(map[string]any)["score"] = 0.9 },
		"legend inconsistent": func(o map[string]any) {
			o["answers"].(map[string]any)["quality"].(map[string]any)["legend"] = map[string]any{"0": "high", "1": "low"}
		},
		"negative probability": func(o map[string]any) {
			o["answers"].(map[string]any)["route"].(map[string]any)["probabilities"] = map[string]any{"a": 1.1, "b": -0.1}
		},
		"incomplete support": func(o map[string]any) {
			o["answers"].(map[string]any)["route"].(map[string]any)["probabilities"] = map[string]any{"a": 1}
		},
		"wrong support": func(o map[string]any) {
			o["answers"].(map[string]any)["route"].(map[string]any)["probabilities"] = map[string]any{"a": 1, "c": 0}
		},
		"unnormalized": func(o map[string]any) {
			o["answers"].(map[string]any)["route"].(map[string]any)["probabilities"] = map[string]any{"a": 0.8, "b": 0.1}
		},
		"confidence range": func(o map[string]any) { o["answers"].(map[string]any)["route"].(map[string]any)["confidence"] = 2 },
		"negative usage":   func(o map[string]any) { o["usage"].(map[string]any)["input_tokens"] = -1 },
		"fractional usage": func(o map[string]any) { o["usage"].(map[string]any)["output_tokens"] = 0.5 },
		"negative cost":    func(o map[string]any) { o["usage"].(map[string]any)["cost"] = -0.1 },
		"null metadata":    func(o map[string]any) { o["usage"] = nil },
	}
	responses := map[string]string{"truncated": sampleResponse[:len(sampleResponse)-3], "trailing": sampleResponse + `{}`, "duplicate": strings.Replace(sampleResponse, `"model":`, `"model":"private-secret","model":`, 1), "nonfinite": strings.Replace(sampleResponse, `"noul":0.2`, `"noul":1e999`, 1)}
	for name, alter := range mutations {
		responses[name] = alteredResponse(t, alter)
	}
	for name, response := range responses {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, response) }))
			defer server.Close()
			code, out, diag := invoke(t, server.URL, sampleRequest)
			if code != 1 || out != "" || calls.Load() != 1 || strings.Contains(diag, "private-secret") {
				t.Fatalf("code=%d calls=%d stdout=%q stderr=%q", code, calls.Load(), out, diag)
			}
		})
	}
}

func TestNoMetadataInventedAndUncertainIsSuccess(t *testing.T) {
	response := alteredResponse(t, func(o map[string]any) {
		delete(o, "usage")
		delete(o, "id")
		delete(o, "provider")
		for _, a := range o["answers"].(map[string]any) {
			delete(a.(map[string]any), "confidence")
		}
		route := o["answers"].(map[string]any)["route"].(map[string]any)
		route["probabilities"] = map[string]any{"a": 0.5, "b": 0.5}
		o["answers"].(map[string]any)["true"].(map[string]any)["noul"] = 0
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, response) }))
	defer server.Close()
	code, out, diag := invoke(t, server.URL, sampleRequest)
	if code != 0 || strings.Contains(out, `"metadata"`) || diag != "" {
		t.Fatalf("%d %s %s", code, out, diag)
	}
}

func TestNoRetriesOrErrorBodyLeaks(t *testing.T) {
	for _, status := range []int{401, 429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				io.WriteString(w, `{"error":"fixture-secret selected evidence"}`)
			}))
			defer server.Close()
			code, out, diag := invoke(t, server.URL, sampleRequest)
			if code != 1 || out != "" || calls.Load() != 1 || strings.Contains(diag, "fixture-secret") || strings.Contains(diag, "selected evidence") {
				t.Fatalf("%d %d %q %q", code, calls.Load(), out, diag)
			}
		})
	}
}

func TestRedirectNeverReleasesCredentials(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	for _, status := range []int{301, 302, 303, 307, 308} {
		source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, status) }))
		code, out, _ := invoke(t, source.URL, sampleRequest)
		source.Close()
		if code != 1 || out != "" || targetCalls.Load() != 0 {
			t.Fatal("redirect followed or result emitted")
		}
	}
}

func TestTimeoutAndCancellation(t *testing.T) {
	arrived := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		arrived <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	start := time.Now()
	code, out, _ := invoke(t, server.URL, sampleRequest, "-timeout", "40ms")
	if code != 1 || out != "" || time.Since(start) > time.Second {
		t.Fatal("timeout did not stop request")
	}
	<-arrived
	ctx, cancel := context.WithCancel(context.Background())
	var stdout, diag bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"-m", "openrouter/test", "-endpoint", server.URL}, strings.NewReader(sampleRequest), &stdout, &diag, func(string) string { return "fixture" }, nil)
	}()
	<-arrived
	cancel()
	select {
	case status := <-done:
		if status != 1 || stdout.Len() != 0 {
			t.Fatal("cancellation became success")
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation stuck")
	}
}

type countReader struct {
	remaining int
	read      int
}

func (r *countReader) Read(b []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(b), r.remaining)
	for i := range b[:n] {
		b[i] = ' '
	}
	r.remaining -= n
	r.read += n
	return n, nil
}

func TestBoundedReadsAndPrivateDiagnostics(t *testing.T) {
	r := &countReader{remaining: maxBytes + 1024}
	var out, diag bytes.Buffer
	code := run(context.Background(), []string{"-m", "openrouter/test"}, r, &out, &diag, func(string) string { t.Fatal("oversized input read credentials"); return "" }, nil)
	if code != 2 || out.Len() != 0 || r.read != maxBytes+1 {
		t.Fatalf("input bound: code=%d read=%d", code, r.read)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(bytes.Repeat([]byte("x"), maxBytes+1024)) }))
	defer server.Close()
	code, text, _ := invoke(t, server.URL, sampleRequest)
	if code != 1 || text != "" {
		t.Fatal("oversized response accepted")
	}
}

type failWriter struct{ short bool }

func (w failWriter) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, errors.New("private-secret")
}

func TestOutputFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, sampleResponse) }))
	defer server.Close()
	for _, writer := range []io.Writer{failWriter{}, failWriter{short: true}} {
		var diag bytes.Buffer
		code := run(context.Background(), []string{"-m", "openrouter/test", "-endpoint", server.URL}, strings.NewReader(sampleRequest), writer, &diag, func(string) string { return "fixture" }, nil)
		if code != 1 || strings.Contains(diag.String(), "private-secret") {
			t.Fatal("output failure hidden or leaked")
		}
	}
}

func TestInvocationAndHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"-m", "other/model"}, {"-m", "openrouter/"}, {"-m", "openrouter/test", "-timeout", "0"}, {"-m", "openrouter/test", "-timeout", "-1s"}, {"-m", "openrouter/test", "-header-fd", "2"}, {"-m", "openrouter/test", "-header-fd", "-1"}, {"-m", "openrouter/test", "-endpoint", "http://example.com/decisions"}, {"-m", "openrouter/test", "-endpoint", "https://secret@example.com/"}, {"-m", "openrouter/test", "-endpoint", "https://example.com/?private=secret"}, {"version", "extra"}, {"-timeout", "private-secret"}} {
		var out, diag bytes.Buffer
		code := run(context.Background(), args, strings.NewReader(sampleRequest), &out, &diag, func(string) string { t.Fatal("invalid invocation read credentials"); return "" }, nil)
		if code != 2 || out.Len() != 0 || strings.Contains(diag.String(), "private-secret") {
			t.Fatalf("args %v code=%d out=%q diag=%q", args, code, out.String(), diag.String())
		}
	}
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}, {"version"}} {
		var out, diag bytes.Buffer
		code := run(context.Background(), args, nil, &out, &diag, func(string) string { t.Fatal("help read environment"); return "" }, nil)
		if code != 0 || out.Len() == 0 || diag.Len() != 0 {
			t.Fatalf("help %v: %d", args, code)
		}
	}
}

// The subprocess uses the actual inherited descriptor and signal-aware main.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("WEIGH_TEST_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"weigh"}, os.Args[i+1:]...)
			main()
			return
		}
	}
	os.Exit(99)
}

func helper(args ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestHelperProcess$", "--"}, args...)...)
	cmd.Env = []string{"WEIGH_TEST_HELPER=1", "OPENROUTER_API_KEY=wrong-ambient-key"}
	cmd.Stdin = strings.NewReader(sampleRequest)
	return cmd
}

func TestInheritedAuthorizationDescriptor(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer descriptor-secret" {
			t.Error("descriptor did not override environment")
		}
		io.WriteString(w, sampleResponse)
	}))
	defer server.Close()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	io.WriteString(w, "Authorization: Bearer descriptor-secret\n")
	w.Close()
	cmd := helper("-m", "openrouter/~typesafe/jev-latest", "-endpoint", server.URL, "-header-fd", "3")
	cmd.ExtraFiles = []*os.File{r}
	data, err := cmd.CombinedOutput()
	if err != nil || calls.Load() != 1 || bytes.Contains(data, []byte("descriptor-secret")) || bytes.Contains(data, []byte("wrong-ambient-key")) {
		t.Fatalf("descriptor process: %v %s", err, data)
	}
}

func TestSignalCancelsProvider(t *testing.T) {
	arrived := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		arrived <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	cmd := helper("-m", "openrouter/test", "-endpoint", server.URL)
	var out, diag bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diag
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-arrived:
	case <-time.After(3 * time.Second):
		cmd.Process.Kill()
		t.Fatal("provider request not started")
	}
	cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil || out.Len() != 0 {
			t.Fatal("signal became success")
		}
	case <-time.After(3 * time.Second):
		cmd.Process.Kill()
		t.Fatal("signal did not cancel")
	}
}

func TestInvalidAuthorization(t *testing.T) {
	for _, header := range []string{"Bearer secret", "Authorization:\n", "Authorization: Bearer secret\nX-Secret: secret", "Authorization: Bearer secret\rsecret", "Authorization: Bearer secret\n\n", "Authorization: Bearer secret\r", "Authorization:\tBearer secret", strings.Repeat("x", maxHeader+1)} {
		var out, diag bytes.Buffer
		code := run(context.Background(), []string{"-m", "openrouter/test", "-header-fd", "3"}, strings.NewReader(sampleRequest), &out, &diag, func(string) string { t.Fatal("descriptor selection fell back to key"); return "" }, func(int) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(header)), nil })
		if code != 1 || out.Len() != 0 || strings.Contains(diag.String(), "secret") {
			t.Fatalf("header accepted or leaked: %d %s", code, diag.String())
		}
	}
}
