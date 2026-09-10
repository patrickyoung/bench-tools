package provider

import (
	"context"
	"strings"
	"testing"
)

// Token categories overlap differently on provider wires. The neutral
// budget baseline must include cached input and reasoning exactly once.
func TestContextTokensNormalizeProviderCounters(t *testing.T) {
	for _, wc := range wireCases {
		if wc.name == "replay" {
			continue // an old replay fixture deliberately lacks the new field
		}
		t.Run(wc.name, func(t *testing.T) {
			wire, want := wc.wire, 35
			switch wc.name {
			case "anthropic":
				wire = strings.Replace(wire, `"input_tokens":10,"output_tokens":0`, `"input_tokens":10,"output_tokens":0,"cache_read_input_tokens":100,"cache_creation_input_tokens":20`, 1)
				want = 155
			case "gemini":
				want = 40 // candidates exclude the five reasoning tokens
			}
			srv := serve(t, 200, sseHeader(), wire)
			d := checkContract(t, wc.make(srv.URL).Stream(context.Background(), contractReq()))
			if d.usage.ContextTokens != want {
				t.Fatalf("context footprint=%d, want %d; usage=%+v", d.usage.ContextTokens, want, d.usage)
			}
		})
	}
}
