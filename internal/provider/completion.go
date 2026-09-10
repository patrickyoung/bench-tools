package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

// completionClient shares transport, not provider policy. The adapters build
// their own requests and identify the native field carrying plain reasoning.
func completionClient(key, base string, hc *http.Client) openai.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(key), option.WithBaseURL(base),
		option.WithMaxRetries(0), // every retry belongs in Ask's log
	}
	if hc != nil {
		opts = append(opts, option.WithHTTPClient(hc))
	}
	return openai.NewClient(opts...)
}

func completionMessages(req Request, name, reasoningField string) ([]map[string]any, error) {
	var messages []map[string]any
	if req.System != "" {
		messages = append(messages, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range Merge(req.Messages) {
		var texts, reasons []string
		var parts []map[string]any
		for _, b := range m.Blocks {
			switch b.Type {
			case Text:
				texts = append(texts, b.Text)
				parts = append(parts, map[string]any{"type": "text", "text": b.Text})
			case Reasoning:
				if m.Role == Assistant && b.Provider == name {
					reasons = append(reasons, b.Text)
				}
			case Media:
				if err := Accepts(name+"/"+req.Model, b.MediaType); err != nil {
					return nil, err
				}
				parts = append(parts, map[string]any{
					"type": "image_url", "image_url": map[string]any{"url": dataURL(b.MediaType, b.Data)},
				})
			}
		}
		msg := map[string]any{"role": string(m.Role), "content": strings.Join(texts, "\n\n")}
		if hasMedia(m) {
			msg["content"] = parts
		}
		if len(reasons) > 0 {
			msg[reasoningField] = strings.Join(reasons, "\n\n")
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// completionDecoder retains the terminal marker the SDK's typed stream hides.
// Comments and empty keepalives carry no JSON. DONE ends the stream immediately;
// waiting for the server to close its connection afterwards can hang forever.
type completionDecoder struct {
	ssestream.Decoder
	done bool
}

func (d *completionDecoder) Next() bool {
	for !d.done && d.Decoder.Next() {
		data := bytes.TrimSpace(d.Event().Data)
		if len(data) == 0 {
			continue
		}
		if bytes.Equal(data, []byte("[DONE]")) {
			d.done = true
			return false
		}
		return true
	}
	return false
}

func streamCompletion(ctx context.Context, c *openai.Client, name, reasoningField string, body map[string]any) iter.Seq2[Chunk, error] {
	return func(yield func(Chunk, error) bool) {
		var response *http.Response
		if err := c.Post(ctx, "chat/completions", body, &response); err != nil {
			yield(Chunk{}, openaiErr(err))
			return
		}
		decoder := &completionDecoder{Decoder: ssestream.NewDecoder(response)}
		stream := ssestream.NewStream[openai.ChatCompletionChunk](decoder, nil)
		defer stream.Close()
		var text, reasoning strings.Builder
		var usage Usage
		var stop string
		for stream.Next() {
			chunk := stream.Current()
			// Usage is a snapshot, not an increment. Some endpoints put it on
			// the final choice; others send a separate chunk with no choices.
			if chunk.JSON.Usage.Valid() {
				u := chunk.Usage
				usage = Usage{
					In: int(u.PromptTokens), Out: int(u.CompletionTokens),
					Reasoning: int(u.CompletionTokensDetails.ReasoningTokens),
					CacheRead: int(u.PromptTokensDetails.CachedTokens),
				}
				usage.ContextTokens = usage.In + usage.Out
				if name == "deepseek" {
					usage.CacheRead = extraInt(u.JSON.ExtraFields, "prompt_cache_hit_tokens")
				}
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			if len(chunk.Choices) != 1 || chunk.Choices[0].Index != 0 || stop != "" {
				yield(Chunk{}, fmt.Errorf("%s: unexpected choice or choice after finish", name))
				return
			}
			choice := chunk.Choices[0]
			if f, ok := choice.Delta.JSON.ExtraFields[reasoningField]; ok {
				var s string
				if err := json.Unmarshal([]byte(f.Raw()), &s); err != nil {
					yield(Chunk{}, fmt.Errorf("%s: invalid %s: %w", name, reasoningField, err))
					return
				}
				if s != "" {
					reasoning.WriteString(s)
					if !yield(Chunk{Kind: KindReasoning, Text: s}, nil) {
						return
					}
				}
			}
			if s := choice.Delta.Content; s != "" {
				text.WriteString(s)
				if !yield(Chunk{Kind: KindText, Text: s}, nil) {
					return
				}
			}
			stop = choice.FinishReason
		}
		if err := stream.Err(); err != nil {
			yield(Chunk{}, fmt.Errorf("%s: %w", name, err))
			return
		}
		if !decoder.done || stop == "" {
			yield(Chunk{}, fmt.Errorf("%s: stream ended without a finish reason and [DONE]", name))
			return
		}
		if reasoning.Len() > 0 {
			if !yield(Chunk{Kind: KindBlock, Block: &Block{Type: Reasoning, Text: reasoning.String(), Provider: name}}, nil) {
				return
			}
		}
		if text.Len() > 0 {
			if !yield(Chunk{Kind: KindBlock, Block: &Block{Type: Text, Text: text.String()}}, nil) {
				return
			}
		}
		if !yield(Chunk{Kind: KindUsage, Usage: &usage}, nil) {
			return
		}
		switch stop {
		case "stop":
			stop = "end"
		case "length":
			stop = "max_tokens"
		}
		yield(Chunk{Kind: KindStop, Stop: stop}, nil)
	}
}
