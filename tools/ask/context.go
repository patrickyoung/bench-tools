package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/patrickyoung/ask/internal/event"
	"github.com/patrickyoung/ask/internal/provider"
)

// contextState describes a budget estimate, never a tokenizer or a model
// catalogue. New provider usage supplies a measured baseline. Pending text
// uses one token per serialized byte plus message framing as a deliberately
// cautious allowance; media and provider replay can still defeat a budget.
type contextState struct {
	Session            string         `json:"session"`
	Model              string         `json:"model"`
	Usage              provider.Usage `json:"usage"`
	UsageSeq           int            `json:"usage_seq,omitempty"`
	Basis              string         `json:"basis"`
	PendingBytes       int            `json:"pending_bytes"`
	PendingMessages    int            `json:"pending_messages"`
	SystemBytes        int            `json:"system_bytes"`
	PendingSystemBytes int            `json:"pending_system_bytes"`
	EstimatedTokens    int            `json:"estimated_tokens"`
	UnmeasuredMedia    bool           `json:"unmeasured_media"`
	Limit              int            `json:"limit,omitempty"`
	Headroom           *int           `json:"headroom,omitempty"`
}

func contextUsage(events []event.Event) (contextState, error) {
	c := contextState{Basis: "serialized_bytes"}
	start, baseline := 0, 0
	var system, measuredSystem string
	for i, e := range events {
		if e.Type == event.Session {
			h, err := event.As[event.Header](e)
			if err != nil {
				return c, err
			}
			c.Model = h.Model
			system = h.System
		}
		if e.Type == event.Request {
			r, err := event.As[provider.Request](e)
			if err != nil {
				return c, err
			}
			system = r.System
		}
		if e.Type != event.Assistant {
			continue
		}
		t, err := event.As[event.Turn](e)
		if err != nil {
			return c, err
		}
		if t.Partial {
			continue
		}
		u := t.Usage
		if u.ContextTokens > 0 {
			baseline, c.Basis = u.ContextTokens, "provider_usage"
		} else if u.In > 0 {
			// Older logs do not normalize the overlapping provider counters.
			// Count all of them conservatively; label this as legacy rather
			// than pretend the sum is an exact input measurement.
			baseline = u.In + u.Out + u.CacheRead + u.CacheWrite + u.Reasoning
			c.Basis = "legacy_usage_allowance"
		} else {
			continue
		}
		c.Usage, c.UsageSeq, start = u, e.Seq, i+1
		measuredSystem = system
	}
	c.SystemBytes = len(system)
	if c.UsageSeq == 0 {
		baseline = len(system)
	} else if system != measuredSystem {
		// A failed request can record changed instructions after the last
		// measured turn. Allow the entire replacement's bytes; subtracting
		// the old string's byte count from measured tokens would undercount.
		c.PendingSystemBytes = len(system)
	}
	msgs, err := event.Fold(events[start:])
	if err != nil {
		return c, err
	}
	for _, m := range msgs {
		b, err := json.Marshal(m.Blocks)
		if err != nil {
			return c, err
		}
		c.PendingBytes += len(b)
		c.PendingMessages++
		for _, block := range m.Blocks {
			if block.Type == provider.Media || block.Type == provider.Opaque {
				c.UnmeasuredMedia = true
			}
		}
	}
	c.EstimatedTokens = baseline + c.PendingBytes + 32*c.PendingMessages + c.PendingSystemBytes
	return c, nil
}

func cmdContext(args []string) int {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	dir := fs.String("d", askDir(), "conversation directory")
	jsonOut := fs.Bool("json", false, "emit context usage and estimate as JSON")
	limit := fs.Int("limit", 0, "optional token budget for estimated headroom")
	usage(fs, "ask context [flags] [session]")
	if err := fs.Parse(args); err != nil {
		return usageCode(fs, err)
	}
	if fs.NArg() > 1 || *limit < 0 {
		return fail(errors.New("context takes at most one session and a nonnegative -limit"))
	}
	path, err := sessionPath(*dir, fs.Arg(0))
	if err != nil {
		return fail(err)
	}
	events, err := event.ReadFile(path)
	if err != nil {
		return fail(err)
	}
	if err := event.Check(events); err != nil {
		return fail(err)
	}
	c, err := contextUsage(events)
	if err != nil {
		return fail(err)
	}
	c.Session, err = filepath.Abs(path)
	if err != nil {
		return fail(err)
	}
	if *limit > 0 {
		c.Limit = *limit
		headroom := *limit - c.EstimatedTokens
		c.Headroom = &headroom
	}
	if *jsonOut {
		b, err := json.Marshal(c)
		if err != nil {
			return fail(err)
		}
		return printOutput(string(b) + "\n")
	}
	return printOutput(fmt.Sprintf("%d estimated tokens (%s; %d pending bytes, %d messages)\n", c.EstimatedTokens, c.Basis, c.PendingBytes, c.PendingMessages))
}
