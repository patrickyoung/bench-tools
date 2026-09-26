package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
)

type closeClient struct {
	events chan *cdp.Event
	closed chan string
}

func (c *closeClient) Event() <-chan *cdp.Event { return c.events }
func (c *closeClient) Call(_ context.Context, _, method string, params interface{}) ([]byte, error) {
	switch method {
	case "Target.setDiscoverTargets":
		return []byte(`{}`), nil
	case "Target.closeTarget":
		data, _ := json.Marshal(params)
		var request struct {
			TargetID string `json:"targetId"`
		}
		if err := json.Unmarshal(data, &request); err != nil {
			return nil, err
		}
		c.closed <- request.TargetID
		return []byte(`{"success":true}`), nil
	}
	return nil, fmt.Errorf("unexpected command: %s", method)
}

func TestOwnedTabCloseWaitsForMatchingDestruction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := &closeClient{events: make(chan *cdp.Event, 2), closed: make(chan string, 1)}
	defer close(client.events)
	browser := rod.New().Context(ctx).Client(client)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- closeTarget(browser, "owned") }()
	if id := <-client.closed; id != "owned" {
		t.Fatal("closed another tab", id)
	}
	client.events <- &cdp.Event{Method: "Target.targetDestroyed", Params: json.RawMessage(`{"targetId":"another"}`)}
	select {
	case err := <-done:
		t.Fatal("acknowledgement or unrelated event completed cleanup", err)
	case <-time.After(30 * time.Millisecond):
	}
	client.events <- &cdp.Event{Method: "Target.targetDestroyed", Params: json.RawMessage(`{"targetId":"owned"}`)}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestOwnedTabCloseRemainsBounded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	client := &closeClient{events: make(chan *cdp.Event), closed: make(chan string, 1)}
	defer close(client.events)
	browser := rod.New().Context(ctx).Client(client)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := closeTarget(browser, "owned"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
