package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrickyoung/ask/internal/event"
	"github.com/patrickyoung/ask/internal/provider"
)

func TestVerbosityPrecedenceAndReplay(t *testing.T) {
	for _, tt := range []struct {
		name, env, want string
		flags           []string
	}{
		{name: "default"},
		{name: "environment", env: "low", want: "low"},
		{name: "flag", env: "low", flags: []string{"-verbosity", "high"}, want: "high"},
		{name: "medium", flags: []string{"-verbosity", "medium"}, want: "medium"},
		{name: "empty overrides environment", env: "low", flags: []string{"-verbosity", ""}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir, _, bodies := fake(t, 200, answerWire)
			t.Setenv("ASK_VERBOSITY", tt.env)
			args := append(append([]string{}, tt.flags...), "hello")
			if code, _, stderr := exec(t, "", args...); code != 0 {
				t.Fatalf("ask exit=%d: %s", code, stderr)
			}
			name := sessions(t, dir)[0]
			events, err := event.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := event.Check(events); err != nil {
				t.Fatal(err)
			}
			for _, e := range events {
				if e.Type != event.Request {
					continue
				}
				code, stdout, stderr := exec(t, "", "replay", "-step", fmt.Sprint(e.Seq), name)
				if code != 0 {
					t.Fatalf("replay exit=%d: %s", code, stderr)
				}
				var req provider.Request
				if err := json.Unmarshal([]byte(stdout), &req); err != nil {
					t.Fatal(err)
				}
				if req.Verbosity != tt.want || len(req.Messages) != 1 || req.Messages[0].Blocks[0].Text != "hello" {
					t.Fatalf("replayed request lost verbosity or changed input: %+v", req)
				}
				if tt.want == "" && strings.Contains(stdout, `"verbosity"`) {
					t.Fatal("default verbosity was not omitted")
				}
				// Unsupported adapters ignore the setting rather than putting
				// an instruction in the system prompt or an unknown wire field.
				if strings.Contains((*bodies)[0], `"verbosity"`) || req.System != "be terse" {
					t.Fatal("verbosity changed Anthropic's wire request or prompt")
				}
				return
			}
			t.Fatal("request event missing")
		})
	}
}

func TestInvalidVerbosityLeavesNoSession(t *testing.T) {
	for _, command := range []string{"", "compact"} {
		for _, value := range []string{"off", "xhigh", "LOW", " low"} {
			t.Run(command+"/"+value, func(t *testing.T) {
				dir, calls, _ := fake(t, 200, answerWire)
				t.Setenv("ASK_VERBOSITY", value)
				args := []string{"hello"}
				if command != "" {
					args = []string{command}
				}
				code, stdout, stderr := exec(t, "", args...)
				if code != 1 || stdout != "" || !strings.Contains(stderr, "-verbosity must be") {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				if calls.Load() != 0 || len(sessions(t, dir)) != 0 {
					t.Fatal("invalid verbosity called a model or created a session")
				}
			})
		}
	}
}

func TestCompactVerbosityAndSubsequentCalls(t *testing.T) {
	dir, _, _ := fake(t, 200, answerWire)
	t.Setenv("ASK_VERBOSITY", "low")
	if code, _, stderr := exec(t, "", "hello"); code != 0 {
		t.Fatalf("seed exit=%d: %s", code, stderr)
	}
	code, stdout, stderr := exec(t, "", "compact", "-verbosity", "high")
	if code != 0 {
		t.Fatalf("compact exit=%d: %s", code, stderr)
	}
	compacted := strings.TrimSpace(stdout)
	if code, _, stderr := exec(t, "", "-f", compacted, "continue"); code != 0 {
		t.Fatalf("continue exit=%d: %s", code, stderr)
	}
	// The summary's explicit high is local to that call. Continuing uses
	// the environment's low; clearing it restores the provider default.
	t.Setenv("ASK_VERBOSITY", "")
	if code, _, stderr := exec(t, "", "-f", compacted, "continue again"); code != 0 {
		t.Fatalf("continue exit=%d: %s", code, stderr)
	}
	var summaryRequests, continuedRequests int
	for _, name := range sessions(t, dir) {
		events, err := event.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := event.Check(events); err != nil {
			t.Fatal(err)
		}
		for _, e := range events {
			if e.Type != event.Request {
				continue
			}
			req, err := event.As[provider.Request](e)
			if err != nil {
				t.Fatal(err)
			}
			if req.System == summarySystem {
				summaryRequests++
				if req.Verbosity != "high" {
					t.Fatalf("summary verbosity=%q, want high", req.Verbosity)
				}
			} else if filepath.Join(dir, name) == compacted {
				want := "low"
				if continuedRequests > 0 {
					want = ""
				}
				continuedRequests++
				if req.Verbosity != want {
					t.Fatalf("continued verbosity=%q, want %q", req.Verbosity, want)
				}
			}
		}
	}
	if summaryRequests != 1 || continuedRequests != 2 {
		t.Fatalf("summary requests=%d, continued requests=%d", summaryRequests, continuedRequests)
	}
}
