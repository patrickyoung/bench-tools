package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A peer can reply after receiving the bytes while the sending goroutine has
// not yet returned from Write. Force that ordering without scheduler timing.
type earlyResponseConnection struct {
	mcp.Connection
	response *jsonrpc.Response
	onWrite  func() error
}

func (c *earlyResponseConnection) Write(_ context.Context, msg jsonrpc.Message) error {
	c.response = &jsonrpc.Response{
		ID:     msg.(*jsonrpc.Request).ID,
		Result: json.RawMessage(`{"content":[],"unknown":{"kept":true}}`),
	}
	return c.onWrite()
}

func (c *earlyResponseConnection) Read(context.Context) (jsonrpc.Message, error) {
	return c.response, nil
}

func TestRecorderCapturesResponseBeforeWriteReturns(t *testing.T) {
	for _, method := range []string{"server/discover", "tools/list", "tools/call"} {
		t.Run(method, func(t *testing.T) {
			rec := &recorder{methods: make(map[string]string), last: make(map[string]wireResponse)}
			effect := method == "tools/call"
			if method != "server/discover" {
				rec.Begin(effect)
			}
			inner := &earlyResponseConnection{}
			conn := &recordingConnection{Connection: inner, recorder: rec}
			inner.onWrite = func() error {
				if _, err := conn.Read(context.Background()); err != nil {
					return err
				}
				if rec.Sent() {
					t.Error("marked sent before Write succeeded")
				}
				return nil
			}
			id, err := jsonrpc.MakeID(float64(1))
			if err != nil {
				t.Fatal(err)
			}
			if err := conn.Write(context.Background(), &jsonrpc.Request{ID: id, Method: method}); err != nil {
				t.Fatal(err)
			}
			last, ok := rec.Last(method)
			if !ok || string(last.Result) != string(inner.response.Result) {
				t.Fatalf("lost early %s response: %#v, present=%v", method, last, ok)
			}
			if method != "server/discover" {
				out, err := finish(context.Background(), rec, nil)
				if err != nil || out.Code != 0 || string(out.Raw) != string(inner.response.Result) {
					t.Fatalf("early response outcome = %#v, %v", out, err)
				}
			}
			if rec.Sent() != effect {
				t.Fatalf("sent = %v, want %v", rec.Sent(), effect)
			}
		})
	}
}

func TestRecorderFailedWriteDoesNotMarkSent(t *testing.T) {
	rec := &recorder{methods: make(map[string]string), last: make(map[string]wireResponse)}
	rec.Begin(true)
	want := errors.New("write failed")
	inner := &earlyResponseConnection{onWrite: func() error { return want }}
	conn := &recordingConnection{Connection: inner, recorder: rec}
	id, err := jsonrpc.MakeID(float64(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(context.Background(), &jsonrpc.Request{ID: id, Method: "tools/call"}); !errors.Is(err, want) {
		t.Fatalf("write error = %v, want %v", err, want)
	}
	if rec.Sent() {
		t.Fatal("failed write marked sent")
	}
	if _, ready := rec.Target(); ready {
		t.Fatal("failed write manufactured a response")
	}
	if _, err := finish(context.Background(), rec, want); !errors.Is(err, want) {
		t.Fatalf("failed write outcome = %v, want original error", err)
	}
}
