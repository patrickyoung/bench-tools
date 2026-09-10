package provider

import (
	"context"
	"iter"
	"net/http"

	openai "github.com/openai/openai-go/v3"
)

// Cerebras returns parsed reasoning separately from the answer. Historical
// reasoning stays in the native reasoning field when continuing this provider.
type Cerebras struct{ c openai.Client }

func NewCerebras(key, base string, hc *http.Client) *Cerebras {
	if base == "" {
		base = "https://api.cerebras.ai/v1"
	}
	return &Cerebras{c: completionClient(key, base, hc)}
}

func (p *Cerebras) Stream(ctx context.Context, req Request) iter.Seq2[Chunk, error] {
	return func(yield func(Chunk, error) bool) {
		messages, err := completionMessages(req, "cerebras", "reasoning")
		if err != nil {
			yield(Chunk{}, err)
			return
		}
		body := map[string]any{
			"model": req.Model, "messages": messages, "stream": true,
			"stream_options":        map[string]any{"include_usage": true},
			"max_completion_tokens": req.MaxTokens, "reasoning_format": "parsed",
		}
		if req.Effort != "" {
			effort := req.Effort
			switch effort {
			case "off":
				effort = "none"
			case "xhigh":
				effort = "high"
			}
			body["reasoning_effort"] = effort
		}
		if len(req.Schema) > 0 {
			body["response_format"] = map[string]any{
				"type":        "json_schema",
				"json_schema": map[string]any{"name": "answer", "strict": true, "schema": req.Schema},
			}
		}
		streamCompletion(ctx, &p.c, "cerebras", "reasoning", body)(yield)
	}
}
