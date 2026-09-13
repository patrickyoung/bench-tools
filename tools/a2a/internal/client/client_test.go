package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

type peerExecutor struct{}

func (peerExecutor) Execute(ctx context.Context, c *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		if !yield(a2a.NewSubmittedTask(c, c.Message), nil) {
			return
		}
		text := c.Message.Parts[0].Text()
		state := a2a.TaskStateCompleted
		switch text {
		case "fail":
			state = a2a.TaskStateFailed
		case "input":
			state = a2a.TaskStateInputRequired
		case "auth":
			state = a2a.TaskStateAuthRequired
		}
		if text == "wait" {
			if !yield(a2a.NewStatusUpdateEvent(c, a2a.TaskStateWorking, nil), nil) {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
		if !yield(a2a.NewArtifactEvent(c, a2a.NewTextPart(text)), nil) {
			return
		}
		yield(a2a.NewStatusUpdateEvent(c, state, nil), nil)
	}
}
func (peerExecutor) Cancel(_ context.Context, c *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(c, a2a.TaskStateCanceled, nil), nil)
	}
}

func request(text string, immediate bool) string {
	b, _ := json.Marshal(&a2a.SendMessageRequest{Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text)), Config: &a2a.SendMessageConfig{ReturnImmediately: immediate}})
	return string(b)
}

func TestUpstreamInteroperabilityAndOutcomes(t *testing.T) {
	for _, binding := range []string{"rest", "jsonrpc"} {
		t.Run(binding, func(t *testing.T) {
			handler := a2asrv.NewHandler(peerExecutor{})
			var wire http.Handler = a2asrv.NewRESTHandler(handler)
			if binding == "jsonrpc" {
				wire = a2asrv.NewJSONRPCHandler(handler)
			}
			server := httptest.NewServer(wire)
			defer server.Close()
			for _, verb := range []string{"request", "listen"} {
				for _, test := range []struct {
					text string
					want int
				}{{"line one\nline two\n", 0}, {"fail", 1}, {"input", 75}, {"auth", 75}} {
					t.Run(verb+"/"+test.text, func(t *testing.T) {
						var out, diag bytes.Buffer
						got := Run(context.Background(), []string{verb, "-http-loopback", "-transport", binding, "send", server.URL}, strings.NewReader(request(test.text, false)), &out, &diag)
						if got != test.want {
							t.Fatalf("exit=%d want=%d stdout=%s diagnostics=%s", got, test.want, out.String(), diag.String())
						}
						if verb == "request" {
							var task a2a.Task
							if err := json.Unmarshal(out.Bytes(), &task); err != nil {
								t.Fatal(err)
							}
							if task.Artifacts[0].Parts[0].Text() != test.text {
								t.Fatalf("artifact changed: %s", out.String())
							}
						} else {
							for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
								if !json.Valid([]byte(line)) {
									t.Fatalf("not JSONL: %q", line)
								}
							}
						}
					})
				}
			}
			var out, diag bytes.Buffer
			code := Run(context.Background(), []string{"request", "-http-loopback", "-transport", binding, "send", server.URL}, strings.NewReader(request("wait", true)), &out, &diag)
			if code != 75 {
				t.Fatalf("accepted work: %d %s %s", code, out.String(), diag.String())
			}
			var task a2a.Task
			if err := json.Unmarshal(out.Bytes(), &task); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			code = Run(context.Background(), []string{"request", "-http-loopback", "-transport", binding, "cancel", server.URL}, strings.NewReader(fmt.Sprintf(`{"id":%q}`, task.ID)), &out, &diag)
			if code != 0 {
				t.Fatalf("cancel: %d %s %s", code, out.String(), diag.String())
			}
		})
	}
}

func TestWireAmbiguityIsNeverRetried(t *testing.T) {
	for _, kind := range []string{"wrong-id", "trailing-json", "duplicate-result", "drop", "oversize", "redirect"} {
		t.Run(kind, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var req struct {
					ID string `json:"id"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if kind == "drop" {
					conn, _, _ := w.(http.Hijacker).Hijack()
					conn.Close()
					return
				}
				if kind == "redirect" {
					w.Header().Set("Location", "http://127.0.0.1:1/elsewhere")
					w.WriteHeader(307)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if kind == "wrong-id" {
					req.ID = "unrelated"
				}
				if kind == "oversize" {
					fmt.Fprint(w, strings.Repeat(" ", 2048))
					return
				}
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%q,"result":{"id":"task","contextId":"context","status":{"state":"TASK_STATE_COMPLETED"}}`, req.ID)
				if kind == "duplicate-result" {
					fmt.Fprint(w, `,"result":{}`)
				}
				fmt.Fprint(w, "}")
				if kind == "trailing-json" {
					fmt.Fprint(w, "{}")
				}
			}))
			defer server.Close()
			var out, diag bytes.Buffer
			code := Run(context.Background(), []string{"request", "-http-loopback", "-max-output", "1024", "send", server.URL}, strings.NewReader(request("hello", false)), &out, &diag)
			if code != 125 || calls.Load() != 1 || out.Len() != 0 {
				t.Fatalf("code=%d calls=%d out=%s diag=%s", code, calls.Load(), out.String(), diag.String())
			}
		})
	}
}

func TestInvalidInputNeverReachesPeer(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	for _, input := range []string{`{}`, `{} {}`, `{"message":{},"message":{}}`, `{"message":{"role":"ROLE_USER"}}`} {
		var out, diag bytes.Buffer
		code := Run(context.Background(), []string{"request", "-http-loopback", "send", server.URL}, strings.NewReader(input), &out, &diag)
		if code != 2 || calls.Load() != 0 {
			t.Fatalf("code=%d calls=%d", code, calls.Load())
		}
	}
}

func TestInterruptWhileReadingInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	if code := Run(ctx, []string{"request", "send", "https://example.invalid"}, r, io.Discard, io.Discard); code != 130 {
		t.Fatalf("code=%d", code)
	}
}

func TestPeerRejectionAndUntrustedTLS(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); http.Error(w, "private diagnostic", 401) }))
	defer server.Close()
	var out, diag bytes.Buffer
	code := Run(context.Background(), []string{"request", "send", server.URL}, strings.NewReader(request("hello", false)), &out, &diag)
	if code != 2 || calls.Load() != 0 {
		t.Fatalf("untrusted TLS: code=%d calls=%d", code, calls.Load())
	}
	plain := httptest.NewServer(server.Config.Handler)
	defer plain.Close()
	code = Run(context.Background(), []string{"request", "-http-loopback", "send", plain.URL}, strings.NewReader(request("hello", false)), &out, &diag)
	if code != 1 || strings.Contains(diag.String(), "private diagnostic") {
		t.Fatalf("rejection: %d %s", code, diag.String())
	}
}

func TestResponseTaskMustMatchRequestedHandle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%q,"result":{"id":"unrelated","contextId":"context","status":{"state":"TASK_STATE_COMPLETED"}}}`, req.ID)
	}))
	defer server.Close()
	var out, diag bytes.Buffer
	code := Run(context.Background(), []string{"request", "-http-loopback", "get", server.URL}, strings.NewReader(`{"id":"requested"}`), &out, &diag)
	if code != 125 || out.Len() != 0 {
		t.Fatalf("wrong task accepted: %d %s", code, out.String())
	}
}

func TestStreamCannotLoseItsContextOrInventCompletion(t *testing.T) {
	for _, mode := range []string{"truncated", "wrong-context", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID string `json:"id"`
				}
				json.NewDecoder(r.Body).Decode(&req)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":%q,\"result\":{\"task\":{\"id\":\"task\",\"contextId\":\"context\",\"status\":{\"state\":\"TASK_STATE_WORKING\"}}}}\n\n", req.ID)
				if mode == "truncated" {
					fmt.Fprint(w, "data: {")
					return
				}
				contextID := "context"
				metadata := "{}"
				state := "TASK_STATE_FAILED"
				if mode == "wrong-context" {
					contextID = "other"
					state = "TASK_STATE_COMPLETED"
				} else {
					metadata = `{"bench/outcome":"unknown"}`
				}
				fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":%q,\"result\":{\"statusUpdate\":{\"taskId\":\"task\",\"contextId\":%q,\"status\":{\"state\":%q},\"metadata\":%s}}}\n\n", req.ID, contextID, state, metadata)
			}))
			defer server.Close()
			var out, diag bytes.Buffer
			code := Run(context.Background(), []string{"listen", "-http-loopback", "send", server.URL}, strings.NewReader(request("work", false)), &out, &diag)
			if code != 125 {
				t.Fatalf("%d %s %s", code, out.String(), diag.String())
			}
			if mode == "unknown" && !strings.Contains(out.String(), `"bench/outcome":"unknown"`) {
				t.Fatalf("lost uncertain task handle: %s %s", out.String(), diag.String())
			}
		})
	}
}
