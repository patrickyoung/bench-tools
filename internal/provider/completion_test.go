package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var completionCases = []wireCase{
	{name: "deepseek", wire: deepseekWire, make: func(u string) Provider { return NewDeepSeek("k", u, nil) }},
	{name: "cerebras", wire: cerebrasWire, make: func(u string) Provider { return NewCerebras("k", u, nil) }},
}

// DeepSeek puts usage on the last choice; Cerebras may send it separately.
const deepseekWire = `: keep-alive

data: {"id":"d1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning_content":"ponder"},"finish_reason":null}]}

data: {"id":"d1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning_content":"ing","content":"Hello"},"finish_reason":null}]}

data: {"id":"d1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":25,"prompt_cache_hit_tokens":3,"prompt_cache_miss_tokens":7,"completion_tokens_details":{"reasoning_tokens":5}}}

data: [DONE]

`

const cerebrasWire = `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning":"ponder"},"finish_reason":null}]}

data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning":"ing","content":"Hello"},"finish_reason":null}]}

data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}]}

data: {"id":"c1","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":25,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens_details":{"reasoning_tokens":5}}}

data: [DONE]

`

func TestCompletionReasoningRoundTrip(t *testing.T) {
	for _, name := range []string{"deepseek", "cerebras"} {
		t.Run(name, func(t *testing.T) {
			field, wire := "reasoning_content", deepseekWire
			if name == "cerebras" {
				field, wire = "reasoning", cerebrasWire
			}
			bodies := make(chan map[string]any, 2)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				bodies <- body
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, wire)
			}))
			defer srv.Close()
			t.Setenv(strings.ToUpper(name)+"_BASE_URL", srv.URL)
			t.Setenv(strings.ToUpper(name)+"_API_KEY", "k")
			p, _, err := New(name+"/test-model", Options{})
			if err != nil {
				t.Fatal(err)
			}
			r := contractReq()
			d := checkContract(t, p.Stream(context.Background(), r))
			<-bodies
			if d.usage.CacheRead != 3 || d.usage.Reasoning != 5 || d.usage.CacheWrite != 0 {
				t.Fatalf("usage changed: %+v", d.usage)
			}
			// Replay after a real JSON round trip, including a foreign turn.
			raw, _ := json.Marshal(d.blocks)
			var blocks []Block
			if err := json.Unmarshal(raw, &blocks); err != nil {
				t.Fatal(err)
			}
			r.Messages = append(r.Messages,
				Message{Role: Assistant, Blocks: blocks},
				Message{Role: User, Blocks: []Block{{Type: Text, Text: "next"}}},
				Message{Role: Assistant, Blocks: []Block{{Type: Text, Text: "first"}, {Type: Reasoning, Text: "FOREIGN", Provider: "other"}, {Type: Text, Text: "second"}}},
				Message{Role: User, Blocks: []Block{{Type: Text, Text: "continue"}}})
			checkContract(t, p.Stream(context.Background(), r))
			body := <-bodies
			msgs := body["messages"].([]any)
			assistant := msgs[2].(map[string]any) // system, user, assistant
			if assistant[field] != "pondering" || assistant["content"] != "Hello world" {
				t.Fatalf("native reasoning changed: %v", assistant)
			}
			foreign := msgs[4].(map[string]any)
			if foreign[field] != nil || foreign["content"] != "first\n\nsecond" {
				t.Fatalf("foreign turn changed: %v", foreign)
			}
		})
	}
}

func TestCompletionRejectsBrokenStreams(t *testing.T) {
	for _, c := range completionCases {
		t.Run(c.name, func(t *testing.T) {
			for name, wire := range map[string]string{
				"no DONE":              strings.ReplaceAll(c.wire, "data: [DONE]\n\n", ""),
				"no finish":            strings.ReplaceAll(c.wire, `"finish_reason":"stop"`, `"finish_reason":null`),
				"empty":                "data: [DONE]\n\n",
				"bad JSON":             "data: {broken}\n\ndata: [DONE]\n\n",
				"bad reasoning":        strings.Replace(c.wire, `"ponder"`, `123`, 1),
				"extra choice":         strings.Replace(c.wire, `"index":0`, `"index":1`, 1),
				"content after finish": strings.Replace(c.wire, "data: [DONE]", `data: {"choices":[{"index":0,"delta":{"content":"late"}}]}`+"\n\ndata: [DONE]", 1),
			} {
				t.Run(name, func(t *testing.T) {
					srv := serve(t, 200, sseHeader(), wire)
					var got error
					for chunk, err := range c.make(srv.URL).Stream(context.Background(), contractReq()) {
						if got != nil || chunk.Kind == KindStop {
							t.Fatal("broken stream emitted success or continued after error")
						}
						got = err
					}
					if got == nil {
						t.Fatal("broken stream succeeded")
					}
				})
			}
		})
	}
}

func TestCompletionStopReasons(t *testing.T) {
	for _, c := range completionCases {
		for _, reason := range []string{"length", "content_filter", "insufficient_system_resource", "tool_calls"} {
			srv := serve(t, 200, sseHeader(), strings.ReplaceAll(c.wire, `"finish_reason":"stop"`, `"finish_reason":"`+reason+`"`))
			d := checkContract(t, c.make(srv.URL).Stream(context.Background(), contractReq()))
			want := reason
			if reason == "length" {
				want = "max_tokens"
			}
			if d.stop != want {
				t.Fatalf("%s stop %q became %q", c.name, reason, d.stop)
			}
		}
	}
}

func TestCompletionClosesStream(t *testing.T) {
	for _, c := range completionCases {
		for _, mode := range []string{"cancel", "break", "DONE"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				closed := make(chan struct{})
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					wire := c.wire
					if mode != "DONE" {
						wire = wire[:strings.Index(wire, "data: [DONE]")]
					}
					io.WriteString(w, wire)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					close(closed)
				}))
				defer srv.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var got error
				for _, err := range c.make(srv.URL).Stream(ctx, contractReq()) {
					if mode == "break" {
						break
					}
					if mode == "cancel" {
						cancel()
					}
					got = err
				}
				if mode == "cancel" && !errors.Is(got, context.Canceled) {
					t.Fatalf("cancellation lost: %v", got)
				}
				if mode == "DONE" && got != nil {
					t.Fatalf("waited for EOF after DONE: %v", got)
				}
				select {
				case <-closed:
				case <-time.After(5 * time.Second):
					t.Fatal("stream body was not closed")
				}
			})
		}
	}
}

func TestCompletionAuthAndErrors(t *testing.T) {
	for _, c := range completionCases {
		t.Run(c.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Header.Get("Authorization") != "Bearer inherited" {
					t.Error("descriptor authorization was not used")
				}
				w.Header().Set("Retry-After", "7")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(429)
				io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit_error","code":"rate_limit"}}`)
			}))
			defer srv.Close()
			t.Setenv(strings.ToUpper(c.name)+"_BASE_URL", srv.URL)
			t.Setenv(strings.ToUpper(c.name)+"_API_KEY", "")
			p, _, err := New(c.name+"/test-model", Options{Authorization: "Bearer inherited"})
			if err != nil {
				t.Fatal(err)
			}
			var got error
			for _, err := range p.Stream(context.Background(), contractReq()) {
				got = err
			}
			var pe *Error
			if !errors.As(got, &pe) || pe.RetryAfter != 7*time.Second || !pe.Retryable() || calls.Load() != 1 {
				t.Fatalf("error=%v calls=%d; SDK must leave retries to Ask", got, calls.Load())
			}
		})
	}
}

func TestCompletionParameters(t *testing.T) {
	for _, c := range completionCases {
		for _, effort := range []string{"", "off", "low", "medium", "high", "xhigh"} {
			t.Run(c.name+"/"+effort, func(t *testing.T) {
				bodies := make(chan map[string]any, 1)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
						t.Errorf("wrong endpoint or key: %s", r.URL.Path)
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					bodies <- body
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, c.wire)
				}))
				defer srv.Close()
				r := contractReq()
				r.Effort = effort
				checkContract(t, c.make(srv.URL).Stream(context.Background(), r))
				body := <-bodies
				tokenField, wantEffort := "max_completion_tokens", effort
				if c.name == "deepseek" {
					tokenField = "max_tokens"
					if effort == "off" {
						wantEffort = ""
						if body["thinking"].(map[string]any)["type"] != "disabled" {
							t.Fatal("off did not disable thinking")
						}
					} else if effort != "" && body["thinking"].(map[string]any)["type"] != "enabled" {
						t.Fatal("explicit effort did not enable thinking")
					}
					if effort == "medium" || effort == "xhigh" {
						wantEffort = "high"
					}
				} else {
					if body["reasoning_format"] != "parsed" {
						t.Fatal("reasoning could enter normal answer text")
					}
					if effort == "off" {
						wantEffort = "none"
					} else if effort == "xhigh" {
						wantEffort = "high"
					}
				}
				actual, _ := body["reasoning_effort"].(string)
				if actual != wantEffort || body[tokenField] != float64(r.MaxTokens) || body["stream_options"].(map[string]any)["include_usage"] != true {
					t.Fatalf("parameters changed: %v", body)
				}
				if effort == "" && (body["thinking"] != nil || body["reasoning_effort"] != nil) {
					t.Fatal("default invocation selected a reasoning effort")
				}
				for _, key := range []string{"tools", "tool_choice", "cache_control", "provider", "clear_thinking"} {
					if body[key] != nil {
						t.Errorf("unexpected provider policy %s", key)
					}
				}
			})
		}
	}
}

func TestCompletionDescriptorRefusesRedirect(t *testing.T) {
	for _, c := range completionCases {
		t.Run(c.name, func(t *testing.T) {
			var reached atomic.Bool
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached.Store(true)
			}))
			defer target.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			}))
			defer origin.Close()
			t.Setenv(strings.ToUpper(c.name)+"_BASE_URL", origin.URL)
			t.Setenv(strings.ToUpper(c.name)+"_API_KEY", "")
			p, _, err := New(c.name+"/m", Options{Authorization: "Bearer inherited-secret"})
			if err != nil {
				t.Fatal(err)
			}
			var got error
			for _, err := range p.Stream(context.Background(), contractReq()) {
				got = err
			}
			if got == nil || !strings.Contains(got.Error(), "redirected origin") || strings.Contains(got.Error(), "inherited-secret") || reached.Load() {
				t.Fatalf("redirect guard failed: err=%v reached=%v", got, reached.Load())
			}
		})
	}
}

func TestCompletionAttachments(t *testing.T) {
	r := contractReq()
	image := Block{Type: Media, MediaType: "image/png", Data: []byte("exact image bytes")}
	r.Messages[0].Blocks = append(r.Messages[0].Blocks, image)
	body := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		body <- string(b)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, deepseekWire)
	}))
	defer srv.Close()
	checkContract(t, NewDeepSeek("k", srv.URL, nil).Stream(context.Background(), r))
	if got := <-body; !strings.Contains(got, dataURL(image.MediaType, image.Data)) || !strings.Contains(got, `"type":"image_url"`) {
		t.Fatalf("DeepSeek image bytes were not carried inline: %s", got)
	}
	var got error
	for _, err := range NewCerebras("k", "http://unused.invalid", nil).Stream(context.Background(), r) {
		got = err
	}
	if got == nil || !strings.Contains(got.Error(), "text only") {
		t.Fatalf("Cerebras accepted unsupported media: %v", got)
	}
}
