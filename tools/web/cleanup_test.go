package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
)

type creatingClient struct {
	closeClient
	cancel       context.CancelFunc
	createCalled bool
}

func (c *creatingClient) Call(ctx context.Context, session, method string, params interface{}) ([]byte, error) {
	switch method {
	case "Target.createTarget":
		c.createCalled = true
		c.cancel()
		// Cancellation arrives after Chrome creates the tab, before its reply.
		// The creation request must retain its bounded response context.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return []byte(`{"targetId":"new-owned"}`), nil
	case "Target.attachToTarget":
		return nil, ctx.Err()
	}
	return c.closeClient.Call(ctx, session, method, params)
}

func TestCanceledPageSetupRetainsOwnedTargetForCleanup(t *testing.T) {
	operations, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	events, stopEvents := context.WithCancel(context.Background())
	defer stopEvents()
	client := &creatingClient{
		closeClient: closeClient{events: make(chan *cdp.Event, 1), closed: make(chan string, 1)},
		cancel:      cancel,
	}
	defer close(client.events)
	browser := rod.New().Context(events).Client(client).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	s := &session{browser: browser.Context(operations), attached: true}
	if err := s.createPage(operations); !errors.Is(err, context.Canceled) {
		t.Fatalf("setup outcome: %v", err)
	}
	if !client.createCalled || s.ownedTarget != "new-owned" || s.page != nil {
		t.Fatalf("lost ownership before page initialization: target=%q page=%v", s.ownedTarget, s.page)
	}
	var stderr bytes.Buffer
	done := make(chan struct{})
	go func() { s.close(false, &stderr); close(done) }()
	select {
	case id := <-client.closed:
		if id != "new-owned" {
			t.Fatal("closed unrelated target", id)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled setup did not close its target")
	}
	client.events <- &cdp.Event{Method: "Target.targetDestroyed", Params: json.RawMessage(`{"targetId":"new-owned"}`)}
	<-done
	if stderr.Len() != 0 {
		t.Fatal("cleanup failed after cancellation", stderr.String())
	}
}

func TestCanceledSetupDoesNotCreateTarget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// No browser is needed: cancellation must be checked before any command.
	if err := (&session{}).createPage(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

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
