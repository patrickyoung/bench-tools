package provider

import (
	"context"
	"errors"
	"iter"
	"net/http"

	openai "github.com/openai/openai-go/v3"
)

// DeepSeek uses Chat Completions with reasoning_content alongside content.
// Without tools, the API accepts but ignores historical reasoning. Keep it
// attributable in the log and replay it in its native field, never as text.
type DeepSeek struct{ c openai.Client }

func NewDeepSeek(key, base string, hc *http.Client) *DeepSeek {
	if base == "" {
		base = "https://api.deepseek.com"
	}
	return &DeepSeek{c: completionClient(key, base, hc)}
}

var errDeepSeekSchema = errors.New("deepseek does not support -schema: JSON mode is not native JSON Schema output")

func (p *DeepSeek) Stream(ctx context.Context, req Request) iter.Seq2[Chunk, error] {
	return func(yield func(Chunk, error) bool) {
		if len(req.Schema) > 0 {
			yield(Chunk{}, errDeepSeekSchema)
			return
		}
		messages, err := completionMessages(req, "deepseek", "reasoning_content")
		if err != nil {
			yield(Chunk{}, err)
			return
		}
		body := map[string]any{
			"model": req.Model, "messages": messages, "stream": true,
			"stream_options": map[string]any{"include_usage": true},
			"max_tokens":     req.MaxTokens,
		}
		switch req.Effort {
		case "": // leave the provider's default alone
		case "off":
			body["thinking"] = map[string]string{"type": "disabled"}
		default:
			body["thinking"] = map[string]string{"type": "enabled"}
			// DeepSeek maps medium and xhigh to high, not its separate max mode.
			effort := req.Effort
			if effort == "medium" || effort == "xhigh" {
				effort = "high"
			}
			body["reasoning_effort"] = effort
		}
		streamCompletion(ctx, &p.c, "deepseek", "reasoning_content", body)(yield)
	}
}
