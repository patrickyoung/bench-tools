package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/patrickyoung/ask/internal/event"
)

func connectorWire(name, answer, stop string) string {
	field := "reasoning"
	if name == "deepseek" {
		field = "reasoning_content"
	}
	chunk, _ := json.Marshal(map[string]any{
		"id": "c1", "object": "chat.completion.chunk",
		"choices": []any{map[string]any{
			"index": 0, "delta": map[string]any{"content": answer, field: "private reasoning"}, "finish_reason": stop,
		}},
		"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 25},
	})
	return "data: " + string(chunk) + "\n\ndata: [DONE]\n\n"
}

func TestNewConnectorsContinueAndVerify(t *testing.T) {
	for _, name := range []string{"deepseek", "cerebras"} {
		t.Run(name, func(t *testing.T) {
			dir, calls, bodies := fake(t, 200, connectorWire(name, "the answer", "stop"))
			t.Setenv("ASK_MODEL", name+"/test-model")
			t.Setenv(strings.ToUpper(name)+"_BASE_URL", os.Getenv("ANTHROPIC_BASE_URL"))
			t.Setenv(strings.ToUpper(name)+"_API_KEY", "k")
			for _, args := range [][]string{{"-q", "hi"}, {"-q", "-c", "continue"}} {
				code, out, stderr := exec(t, "", args...)
				if code != 0 || out != "the answer\n" || stderr != "" {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, stderr)
				}
			}
			if calls.Load() != 2 || len(sessions(t, dir)) != 1 || !strings.Contains((*bodies)[1], "private reasoning") {
				t.Fatal("continuation lost its session or reasoning")
			}
			path := filepath.Join(dir, sessions(t, dir)[0])
			if err := event.Check(events(t, path)); err != nil {
				t.Fatal(err)
			}
			if code, _, stderr := exec(t, "", "replay", "-check", "-json", path); code != 0 {
				t.Fatal(stderr)
			}
		})
	}
}

func TestNewConnectorOutcomes(t *testing.T) {
	for _, name := range []string{"deepseek", "cerebras"} {
		for _, tc := range []struct {
			name   string
			status int
			wire   string
			code   int
		}{
			{"length", 200, connectorWire(name, "cut off", "length"), 1},
			{"filter", 200, connectorWire(name, "refused", "content_filter"), 1},
			{"resource", 200, connectorWire(name, "partial", "insufficient_system_resource"), 1},
			{"EOF", 200, strings.ReplaceAll(connectorWire(name, "partial", "stop"), "data: [DONE]\n\n", ""), 1},
			{"no finish", 200, connectorWire(name, "partial", ""), 1},
			{"reasoning only", 200, connectorWire(name, "", "stop"), 1},
			{"overflow", 400, `{"error":{"message":"context window full","code":"context_length_exceeded"}}`, 2},
			{"output limit", 400, `{"error":{"message":"max_tokens exceeds the limit of 100","code":"invalid_parameter"}}`, 1},
			{"request size", 413, `{"error":{"message":"request body too large"}}`, 1},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				dir, calls, _ := fake(t, tc.status, tc.wire)
				t.Setenv("ASK_MODEL", name+"/test-model")
				t.Setenv(strings.ToUpper(name)+"_BASE_URL", os.Getenv("ANTHROPIC_BASE_URL"))
				t.Setenv(strings.ToUpper(name)+"_API_KEY", "k")
				code, out, stderr := exec(t, "", "-q", "hi")
				if code != tc.code || out != "" || stderr == "" || calls.Load() != 1 {
					t.Fatalf("exit=%d stdout=%q stderr=%q calls=%d", code, out, stderr, calls.Load())
				}
				if err := event.Check(events(t, filepath.Join(dir, sessions(t, dir)[0]))); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestNewConnectorSchemas(t *testing.T) {
	t.Run("deepseek rejects before creating a session", func(t *testing.T) {
		dir, calls, _ := fake(t, 200, connectorWire("deepseek", `{}`, "stop"))
		t.Setenv("ASK_MODEL", "deepseek/test-model")
		t.Setenv("DEEPSEEK_API_KEY", "") // capability check precedes authentication
		code, out, stderr := exec(t, "", "-schema", writeSchema(t, `{}`), "hi")
		if code != 1 || out != "" || !strings.Contains(stderr, "deepseek does not support -schema") || calls.Load() != 0 || len(sessions(t, dir)) != 0 {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, stderr)
		}
	})
	for _, answer := range []string{`{"n":7}`, `{"n":"wrong"}`} {
		t.Run("cerebras/"+answer, func(t *testing.T) {
			dir, _, bodies := fake(t, 200, connectorWire("cerebras", answer, "stop"))
			t.Setenv("ASK_MODEL", "cerebras/test-model")
			t.Setenv("CEREBRAS_BASE_URL", os.Getenv("ANTHROPIC_BASE_URL"))
			t.Setenv("CEREBRAS_API_KEY", "k")
			code, out, stderr := exec(t, "", "-q", "-schema", writeSchema(t, numberSchema), "hi")
			if answer == `{"n":7}` {
				if code != 0 || out != answer+"\n" {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, stderr)
				}
			} else if code != 1 || out != "" || !strings.Contains(stderr, "structured output does not match schema") {
				t.Fatalf("invalid JSON admitted: exit=%d stdout=%q stderr=%q", code, out, stderr)
			}
			if !strings.Contains((*bodies)[0], `"type":"json_schema"`) || !strings.Contains((*bodies)[0], `"strict":true`) {
				t.Fatal("schema was not sent natively")
			}
			if err := event.Check(events(t, filepath.Join(dir, sessions(t, dir)[0]))); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNewConnectorInvalidInvocationLeavesNoSession(t *testing.T) {
	for _, name := range []string{"deepseek", "cerebras"} {
		t.Run(name, func(t *testing.T) {
			dir, calls, _ := fake(t, 200, connectorWire(name, "unused", "stop"))
			t.Setenv("ASK_MODEL", name+"/test-model")
			t.Setenv(strings.ToUpper(name)+"_BASE_URL", os.Getenv("ANTHROPIC_BASE_URL"))
			t.Setenv(strings.ToUpper(name)+"_API_KEY", "")
			code, out, stderr := exec(t, "", "hi")
			if code != 1 || out != "" || !strings.Contains(stderr, strings.ToUpper(name)+"_API_KEY is not set") {
				t.Fatalf("missing key: exit=%d stdout=%q stderr=%q", code, out, stderr)
			}
			t.Setenv(strings.ToUpper(name)+"_API_KEY", "k")
			path := filepath.Join(t.TempDir(), "document.pdf")
			if err := os.WriteFile(path, []byte("%PDF-1.7\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			code, out, stderr = exec(t, "", "-a", path, "hi")
			if code != 1 || out != "" || !strings.Contains(stderr, "does not accept application/pdf") {
				t.Fatalf("unsupported media: exit=%d stdout=%q stderr=%q", code, out, stderr)
			}
			if calls.Load() != 0 || len(sessions(t, dir)) != 0 {
				t.Fatal("invalid invocation made a call or a session")
			}
		})
	}
}

func TestNewConnectorsDescriptorAndCompact(t *testing.T) {
	for _, name := range []string{"deepseek", "cerebras"} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer descriptor-secret" {
					t.Error("provider did not receive descriptor authorization")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, connectorWire(name, "the answer", "stop"))
			}))
			defer srv.Close()
			dir := t.TempDir()
			t.Setenv("ASK_DIR", dir)
			t.Setenv("ASK_MODEL", name+"/test-model")
			t.Setenv(strings.ToUpper(name)+"_BASE_URL", srv.URL)
			t.Setenv(strings.ToUpper(name)+"_API_KEY", "")
			t.Setenv("NO_COLOR", "1")
			for _, compact := range []bool{false, true} {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				io.WriteString(w, "Authorization: Bearer descriptor-secret\n")
				w.Close()
				// Hand off a raw descriptor, just as exec would. Retaining an
				// os.File owner lets its finalizer close an unrelated reused fd
				// after readAuthorizationFD consumes the original descriptor.
				fd, err := syscall.Dup(int(r.Fd()))
				r.Close()
				if err != nil {
					t.Fatal(err)
				}
				args := []string{"-q", "-header-fd", fmt.Sprint(fd), "hi"}
				if compact {
					args = []string{"compact", "-q", "-header-fd", fmt.Sprint(fd)}
				}
				code, out, stderr := exec(t, "", args...)
				if code != 0 || strings.Contains(out+stderr, "descriptor-secret") {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, stderr)
				}
			}
			if len(sessions(t, dir)) != 3 {
				t.Fatal("compaction did not create three provable files")
			}
			for _, file := range sessions(t, dir) {
				path := filepath.Join(dir, file)
				raw, err := os.ReadFile(path)
				if err != nil || bytes.Contains(raw, []byte("descriptor-secret")) {
					t.Fatal("credential entered log or log unreadable")
				}
				if err := event.Check(events(t, path)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
