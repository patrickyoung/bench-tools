package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIVerbosityWire(t *testing.T) {
	for _, codex := range []bool{false, true} {
		for _, verbosity := range []string{"", "low", "medium", "high"} {
			for _, schema := range []string{"", `{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"],"additionalProperties":false}`} {
				t.Run(fmt.Sprintf("codex=%v/verbosity=%s/schema=%v", codex, verbosity, schema != ""), func(t *testing.T) {
					var body map[string]any
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						w.Header().Set("Content-Type", "text/event-stream")
						io.WriteString(w, openaiWire)
					}))
					defer srv.Close()
					p := NewOpenAI("test", srv.URL, srv.Client())
					if codex {
						p = NewOpenAICodex(srv.URL, srv.Client())
					}
					req := contractReq()
					req.Verbosity = verbosity
					req.Schema = json.RawMessage(schema)
					checkContract(t, p.Stream(context.Background(), req))
					text, _ := body["text"].(map[string]any)
					got, present := text["verbosity"]
					if verbosity == "" && present || verbosity != "" && got != verbosity {
						t.Fatalf("text.verbosity=%v, want %q (empty omitted)", got, verbosity)
					}
					format, _ := text["format"].(map[string]any)
					if schema == "" {
						if format != nil {
							t.Fatalf("verbosity added a format: %v", format)
						}
					} else if format["type"] != "json_schema" || format["strict"] != true || format["schema"] == nil {
						t.Fatalf("verbosity lost the schema: %v", format)
					}
					if body["instructions"] != req.System {
						t.Fatal("verbosity changed the system prompt")
					}
				})
			}
		}
	}
}
