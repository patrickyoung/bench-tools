package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests cross the real executable boundary, including Ask's argument
// parser, provider adapter, durable log and replay verifier. They use only a
// loopback HTTP fixture, never a paid provider. In a standalone Ply checkout
// the implicit sibling may be absent; CI can REQUIRE the contract suite with:
//
//	PLY_TEST_ASK_SOURCE=/absolute/path/to/ask go test -run '^TestAskPlyContract$' .
//
// An explicitly configured missing or unbuildable source is a failure, not a
// skip. There are no production environment variables or protocol shortcuts.
func TestAskPlyContract(t *testing.T) {
	bin := buildContractAsk(t)
	t.Run("capped_action_and_resume", func(t *testing.T) {
		work, control, server := contractEnvironment(t, bin)
		const observed = "observed-result-token"
		server.setResponder(func(w http.ResponseWriter, r contractRequest, n int) {
			if n == 1 {
				contractResponse(w, "```ply\nprintf x >> effect-count\nprintf '%s%s\\n' 'observed-result-' 'token'\n```", 10)
			} else {
				contractResponse(w, "Completed from the recorded observation.", 10)
			}
		})
		code, out, stderr := runPly(t, "-sh", "-C", work, "-checkpoint", control, "-turns", "1", "perform exactly one effect")
		if code != 2 || out != "" {
			t.Fatalf("cap: exit=%d out=%q stderr=%q", code, out, stderr)
		}
		path := strings.TrimSpace(read(t, control))
		events := verifiedAskEvents(t, bin, path)
		if got := contractUserCount(events, "ply", observed); got != 1 {
			t.Fatalf("durable observation count=%d, want one", got)
		}
		if got := read(t, filepath.Join(work, "effect-count")); got != "x" {
			t.Fatalf("effects=%q", got)
		}
		code, out, stderr = runPly(t, "-sh", "-C", work, "-checkpoint", control, "-turns", "1", "continue without repeating the effect")
		if code != 0 || out != "Completed from the recorded observation.\n" {
			t.Fatalf("resume: exit=%d out=%q stderr=%q", code, out, stderr)
		}
		reqs := server.requests()
		if len(reqs) != 2 || strings.Count(reqs[1].userText(), observed) != 1 {
			t.Fatalf("resumed provider did not see one observation in %d requests", len(reqs))
		}
		if got := contractUserCount(verifiedAskEvents(t, bin, path), "ply", observed); got != 1 {
			t.Fatalf("resume duplicated observation: %d", got)
		}
		if read(t, filepath.Join(work, "effect-count")) != "x" {
			t.Fatal("resume repeated an uncertain effect")
		}
	})

	t.Run("overflow_explicit_model_and_checkpoint", func(t *testing.T) {
		work, control, server := contractEnvironment(t, bin)
		t.Setenv("ASK_MODEL", "openai/wrong-environment-default")
		var original string
		ordinary, summaries := 0, 0
		server.setResponder(func(w http.ResponseWriter, r contractRequest, _ int) {
			if r.Model != "explicit-model" {
				t.Errorf("model=%q, want explicit-model", r.Model)
			}
			if r.Text.Verbosity != "low" {
				t.Errorf("ordinary or summary request lost verbosity: %q", r.Text.Verbosity)
			}
			if r.summary() {
				summaries++
				contractResponse(w, "A minimal handoff.", 10)
				return
			}
			ordinary++
			if ordinary == 1 {
				b, err := os.ReadFile(control)
				if err != nil {
					t.Error(err)
				}
				original = strings.TrimSpace(string(b))
				contractOverflow(w)
				return
			}
			contractResponse(w, "Recovered after one compaction.", 10)
		})
		code, out, stderr := runPly(t, "-sh", "-C", work, "-checkpoint", control,
			"-m", "openai/explicit-model", "-compact", "-compactions", "1", "preserve the full original goal")
		if code != 0 || out != "Recovered after one compaction.\n" {
			t.Fatalf("overflow recovery: exit=%d out=%q stderr=%q", code, out, stderr)
		}
		_ = server.requests() // join the completed callback before inspecting its counters
		current := strings.TrimSpace(read(t, control))
		if original == "" || current == original || summaries != 1 || ordinary != 2 {
			t.Fatalf("original=%q current=%q summaries=%d ordinary=%d", original, current, summaries, ordinary)
		}
		events := verifiedAskEvents(t, bin, current)
		h := contractHeader(t, events)
		if h.Parent != strings.TrimSuffix(filepath.Base(original), ".jsonl") || h.Summary == "" {
			t.Fatalf("bad handoff ancestry: %+v", h)
		}
		verifiedAskEvents(t, bin, original)
		verifiedAskEvents(t, bin, filepath.Join(filepath.Dir(current), h.Summary+".jsonl"))
		if got := contractUserCount(events, "ply", "preserve the full original goal"); got != 1 {
			t.Fatalf("full goal was not retained independently of summary: %d", got)
		}
		code, out, stderr = runPly(t, "-sh", "-C", work, "-checkpoint", control,
			"-m", "openai/explicit-model", "-compactions", "1", "resume the compacted task")
		if code != 0 || out != "Recovered after one compaction.\n" || strings.TrimSpace(read(t, control)) != current {
			t.Fatalf("checkpoint resume: exit=%d out=%q stderr=%q", code, out, stderr)
		}
		reqs := server.requests()
		if !strings.Contains(reqs[len(reqs)-1].userText(), "preserve the full original goal") {
			t.Fatal("resuming fresh session lost the original goal")
		}
	})

	t.Run("repeated_proactive_compaction_retains_supplied_input", func(t *testing.T) {
		work, control, server := contractEnvironment(t, bin)
		const goal = "Repair both modules; retain the public interface; verify both acceptance checks."
		const input = "supplied-input-unique: alpha must remain 17; beta must remain 29\n"
		ordinary, summaries := 0, 0
		server.setResponder(func(w http.ResponseWriter, r contractRequest, _ int) {
			if r.summary() {
				summaries++
				// Deliberately omit goal and supplied input: Ply must carry
				// its exact instructions independently of summary quality.
				contractResponse(w, "Recent work continues; inspect current files.", 10)
				return
			}
			ordinary++
			if !strings.Contains(r.userText(), goal) || !strings.Contains(r.userText(), strings.TrimSpace(input)) {
				t.Errorf("ordinary request %d lost goal or supplied input: %s", ordinary, r.userText())
			}
			if ordinary < 3 {
				contractResponse(w, fmt.Sprintf("```ply\nprintf '%d\\n' >> steps\n```", ordinary), 500)
			} else {
				contractResponse(w, "Both modules verified.", 500)
			}
		})
		code, out, stderr := contractRunPlyInput(t, input, "-sh", "-C", work, "-checkpoint", control,
			"-compact-at", "100", "-compactions", "2", "-turns", "4", goal)
		_ = server.requests()
		if code != 0 || out != "Both modules verified.\n" || summaries != 2 || ordinary != 3 {
			t.Fatalf("proactive: exit=%d out=%q summaries=%d ordinary=%d stderr=%q", code, out, summaries, ordinary, stderr)
		}
		if read(t, filepath.Join(work, "steps")) != "1\n2\n" {
			t.Fatal("proactive compaction repeated or skipped an action")
		}
		path := strings.TrimSpace(read(t, control))
		for i := 0; i < 2; i++ {
			events := verifiedAskEvents(t, bin, path)
			if contractUserCount(events, "ply", goal) != 1 || contractUserCount(events, "ply", strings.TrimSpace(input)) != 1 {
				t.Fatalf("handoff %d lacks exact original context", i)
			}
			h := contractHeader(t, events)
			if h.Parent == "" || h.Summary == "" {
				t.Fatalf("handoff %d has no provenance", i)
			}
			verifiedAskEvents(t, bin, filepath.Join(filepath.Dir(path), h.Summary+".jsonl"))
			path = filepath.Join(filepath.Dir(path), h.Parent+".jsonl")
		}
		verifiedAskEvents(t, bin, path)
	})

	for _, proposal := range []string{"action", "report"} {
		t.Run("steering_during_"+proposal, func(t *testing.T) {
			work, control, server := contractEnvironment(t, bin)
			steer := filepath.Join(t.TempDir(), "steer")
			write(t, steer, "", 0o600)
			server.setResponder(func(w http.ResponseWriter, r contractRequest, n int) {
				if n == 1 {
					text := "Old report must not be accepted."
					if proposal == "action" {
						text = "```ply\ntouch must-not-run\n```"
					}
					contractStart(w, text)
					if err := os.WriteFile(steer, []byte("new-steering-token: inspect status before acting\n"), 0o600); err != nil {
						t.Error(err)
					}
					contractFinish(w, text, 10)
					return
				}
				if strings.Count(r.userText(), "new-steering-token") != 1 {
					t.Errorf("new steering did not arrive exactly once: %s", r.userText())
				}
				contractResponse(w, "Updated report accounts for new steering.", 10)
			})
			code, out, stderr := runPly(t, "-sh", "-B", "-C", work, "-checkpoint", control,
				"-steer", steer, "-check", "cat >> accepted-candidates", "complete the task")
			if code != 0 || out != "Updated report accounts for new steering.\n" || len(server.requests()) != 2 {
				t.Fatalf("steering: exit=%d out=%q stderr=%q", code, out, stderr)
			}
			if _, err := os.Stat(filepath.Join(work, "must-not-run")); !os.IsNotExist(err) {
				t.Fatalf("deferred action executed: %v", err)
			}
			if got := read(t, filepath.Join(work, "accepted-candidates")); got != "Updated report accounts for new steering.\n" {
				t.Fatalf("verifier received an obsolete candidate: %q", got)
			}
			verifiedAskEvents(t, bin, strings.TrimSpace(read(t, control)))
		})
	}

	for _, finish := range []bool{true, false} {
		name := "streamed_complete_response"
		if !finish {
			name = "streamed_incomplete_response"
		}
		t.Run(name, func(t *testing.T) {
			work, control, server := contractEnvironment(t, bin)
			const marker = "STREAMING_PROGRESS_WITNESS"
			capture := &contractProgress{marker: marker, seen: make(chan struct{})}
			server.setResponder(func(w http.ResponseWriter, _ contractRequest, n int) {
				if n > 1 {
					contractResponse(w, "Finished after the complete response.", 10)
					return
				}
				text := marker + "\n\n```ply\n[ -f response-completed ] || touch executed-too-early\ntouch executed-after-completion\n```"
				contractStart(w, text)
				select {
				case <-capture.seen:
				case <-time.After(5 * time.Second):
					t.Error("model progress was not visible while the response remained open")
				}
				if _, err := os.Stat(filepath.Join(work, "executed-after-completion")); !os.IsNotExist(err) {
					t.Errorf("action executed from a partial response: %v", err)
				}
				if finish {
					if err := os.WriteFile(filepath.Join(work, "response-completed"), nil, 0o600); err != nil {
						t.Error(err)
					}
					contractFinish(w, text, 10)
				}
			})
			// The subprocess uses TestMain's production dispatch hook, so
			// stderr can be observed concurrently without swapping globals.
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, self, "-sh", "-stream", "-C", work, "-checkpoint", control, "perform the action")
			cmd.Env = append(os.Environ(), "PLY_TEST_PROGRAM=1")
			var stdout bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, capture
			err = cmd.Run()
			if finish {
				if err != nil || stdout.String() != "Finished after the complete response.\n" {
					t.Fatalf("complete stream: err=%v out=%q stderr=%q", err, stdout.String(), capture.text())
				}
				if _, err := os.Stat(filepath.Join(work, "executed-after-completion")); err != nil {
					t.Fatal("completed action did not execute:", err)
				}
			} else {
				if err == nil || stdout.Len() != 0 {
					t.Fatalf("incomplete response was accepted: err=%v out=%q", err, stdout.String())
				}
				if _, err := os.Stat(filepath.Join(work, "executed-after-completion")); !os.IsNotExist(err) {
					t.Fatal("incomplete response executed the command")
				}
			}
			if _, err := os.Stat(filepath.Join(work, "executed-too-early")); !os.IsNotExist(err) {
				t.Fatal("action observed that generation was still incomplete")
			}
			if !strings.Contains(capture.text(), marker) {
				t.Fatal("progress marker never reached stderr")
			}
			verifiedAskEvents(t, bin, strings.TrimSpace(read(t, control)))
		})
	}

	t.Run("invalid_compaction_result_cannot_advance_checkpoint", func(t *testing.T) {
		work, control, server := contractEnvironment(t, bin)
		server.setResponder(func(w http.ResponseWriter, _ contractRequest, n int) {
			if n == 1 {
				contractResponse(w, "Seeded task.", 10)
			} else {
				contractOverflow(w)
			}
		})
		if code, _, stderr := runPly(t, "-sh", "-C", work, "-checkpoint", control, "seed task"); code != 0 {
			t.Fatal(stderr)
		}
		before := read(t, control)
		corrupt := filepath.Join(t.TempDir(), "corrupt.jsonl")
		write(t, corrupt, "this is not an Ask event\n", 0o600)
		wrapper := filepath.Join(t.TempDir(), "ask-wrapper")
		write(t, wrapper, "#!/bin/sh\nif [ \"$1\" = compact ]; then printf '%s\\n' "+shellQuote(corrupt)+"; exit 0; fi\nexec "+shellQuote(bin)+" \"$@\"\n", 0o700)
		t.Setenv("ASK", wrapper)
		code, out, stderr := runPly(t, "-sh", "-C", work, "-checkpoint", control, "-compact", "continue task")
		if code != 1 || out != "" || !strings.Contains(stderr, "did not replay") {
			t.Fatalf("corrupt handoff: exit=%d out=%q stderr=%q", code, out, stderr)
		}
		if read(t, control) != before {
			t.Fatal("corrupt handoff advanced checkpoint")
		}
		verifiedAskEvents(t, bin, strings.TrimSpace(before))
	})
}

func buildContractAsk(t *testing.T) string {
	t.Helper()
	source, explicit := os.LookupEnv("PLY_TEST_ASK_SOURCE")
	if !explicit {
		source = filepath.Join("..", "ask")
	}
	if source == "" {
		t.Fatal("PLY_TEST_ASK_SOURCE must name the Ask source directory")
	}
	if _, err := os.Stat(filepath.Join(source, "go.mod")); err != nil {
		if !explicit && os.IsNotExist(err) {
			t.Skip("real Ask contract tests need ../ask; set PLY_TEST_ASK_SOURCE to require them")
		}
		t.Fatalf("Ask source %q: %v", source, err)
	}
	bin := filepath.Join(t.TempDir(), "ask")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = source
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build real Ask: %v\n%s", err, out)
	}
	return bin
}

type contractRequest struct {
	Model        string            `json:"model"`
	Instructions string            `json:"instructions"`
	Input        []json.RawMessage `json:"input"`
	Text         struct {
		Verbosity string `json:"verbosity"`
	} `json:"text"`
}

func (r contractRequest) summary() bool {
	return strings.Contains(r.Instructions, "writing a handoff note")
}
func (r contractRequest) userText() string {
	var texts []string
	for _, raw := range r.Input {
		var item struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(raw, &item) != nil || item.Role != "user" {
			continue
		}
		var text string
		if json.Unmarshal(item.Content, &text) == nil {
			texts = append(texts, text)
			continue
		}
		var blocks []struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(item.Content, &blocks) == nil {
			for _, b := range blocks {
				texts = append(texts, b.Text)
			}
		}
	}
	return strings.Join(texts, "\n")
}

type contractServer struct {
	mu      sync.Mutex
	reqs    []contractRequest
	respond func(http.ResponseWriter, contractRequest, int)
}

func (s *contractServer) setResponder(fn func(http.ResponseWriter, contractRequest, int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.respond = fn
}

func (s *contractServer) requests() []contractRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]contractRequest(nil), s.reqs...)
}

func contractEnvironment(t *testing.T, bin string) (work, checkpoint string, fixture *contractServer) {
	t.Helper()
	fixture = &contractServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer fixture-only" {
			t.Errorf("unexpected local request: %s authorization=%q", r.URL.Path, r.Header.Get("Authorization"))
			http.Error(w, "unexpected local fixture request", 400)
			return
		}
		var req contractRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			http.Error(w, "bad fixture request", 400)
			return
		}
		fixture.mu.Lock()
		fixture.reqs = append(fixture.reqs, req)
		n := len(fixture.reqs)
		defer fixture.mu.Unlock()
		fixture.respond(w, req, n)
	}))
	t.Cleanup(server.Close)
	t.Setenv("ASK", bin)
	t.Setenv("ASK_DIR", t.TempDir())
	t.Setenv("ASK_MODEL", "openai/fixture-model")
	t.Setenv("OPENAI_API_KEY", "fixture-only")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("PLY_DIR", t.TempDir())
	for _, name := range []string{"PLY_TOOLS", "PLY_ACTION_SHELL", "PLY_MAY_JOB", "PLY_CONTRACT_ID", "PLY_EFFORT", "PLY_DEPTH"} {
		t.Setenv(name, "")
	}
	t.Setenv("PLY_SHELL", "/bin/sh")
	t.Setenv("NO_COLOR", "1")
	return t.TempDir(), filepath.Join(t.TempDir(), "checkpoint"), fixture
}

func contractEvent(w http.ResponseWriter, name string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func contractStart(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	contractEvent(w, "response.created", map[string]any{
		"type": "response.created", "sequence_number": 0,
		"response": map[string]any{"id": "resp_fixture", "object": "response", "status": "in_progress", "model": "fixture", "output": []any{}},
	})
	contractEvent(w, "response.output_text.delta", map[string]any{
		"type": "response.output_text.delta", "sequence_number": 1, "item_id": "msg_fixture", "output_index": 0, "content_index": 0, "delta": text,
	})
}

func contractFinish(w http.ResponseWriter, text string, inputTokens int) {
	contractEvent(w, "response.completed", map[string]any{
		"type": "response.completed", "sequence_number": 2,
		"response": map[string]any{
			"id": "resp_fixture", "object": "response", "status": "completed", "model": "fixture",
			"output": []any{map[string]any{"type": "message", "id": "msg_fixture", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}}},
			"usage":  map[string]any{"input_tokens": inputTokens, "output_tokens": 10, "total_tokens": inputTokens + 10},
		},
	})
}

func contractResponse(w http.ResponseWriter, text string, inputTokens int) {
	contractStart(w, text)
	contractFinish(w, text, inputTokens)
}

func contractOverflow(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	io.WriteString(w, `{"error":{"message":"maximum context window exceeded","type":"invalid_request_error","code":"context_length_exceeded"}}`)
}

type contractLogEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func verifiedAskEvents(t *testing.T, bin, path string) []contractLogEvent {
	t.Helper()
	cmd := exec.Command(bin, "replay", "-check", "-json", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("real Ask replay %s: %v: %s", path, err, stderr.String())
	}
	var events []contractLogEvent
	s := bufio.NewScanner(bytes.NewReader(out))
	s.Buffer(make([]byte, 4096), 64<<20)
	for s.Scan() {
		var e contractLogEvent
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func contractUserCount(events []contractLogEvent, source, needle string) int {
	count := 0
	for _, e := range events {
		if e.Type != "user" {
			continue
		}
		var u struct{ Text, Source string }
		if json.Unmarshal(e.Data, &u) == nil && u.Source == source && strings.Contains(u.Text, needle) {
			count++
		}
	}
	return count
}

type contractSessionHeader struct{ ID, Parent, Summary string }

func contractHeader(t *testing.T, events []contractLogEvent) contractSessionHeader {
	t.Helper()
	var h contractSessionHeader
	if len(events) == 0 || events[0].Type != "session" || json.Unmarshal(events[0].Data, &h) != nil {
		t.Fatal("missing Ask session header")
	}
	return h
}

func contractRunPlyInput(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	write(t, path, input, 0o600)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = old; f.Close() }()
	return runPly(t, args...)
}

type contractProgress struct {
	mu     sync.Mutex
	body   bytes.Buffer
	marker string
	seen   chan struct{}
	once   sync.Once
}

func (p *contractProgress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n, err := p.body.Write(b)
	if strings.Contains(p.body.String(), p.marker) {
		p.once.Do(func() { close(p.seen) })
	}
	return n, err
}

func (p *contractProgress) text() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.body.String()
}
