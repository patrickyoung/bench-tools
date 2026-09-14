package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnixExampleDocumentPreservesExtensions(t *testing.T) {
	dir, err := filepath.Abs("examples/unix-evidence/connectors")
	if err != nil {
		t.Fatal(err)
	}
	query := "Can demo-customer export its data?\n"
	code, stdout, stderr := runContext(t, dir, query, "query", "policy")
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	rows := decodeRows(t, stdout)
	if len(rows) != 1 || rows[0]["type"] != "document" {
		t.Fatalf("records=%#v", rows)
	}
	row := rows[0]
	if row["origin"].(map[string]any)["revision"] != "1" ||
		row["coverage"].(map[string]any)["truncated"] != false ||
		row["retrieval"].(map[string]any)["query"] != query {
		t.Fatalf("lost evidence metadata: %#v", row)
	}
}

func TestUnixExampleMCPAdapterRejectsUnusableResults(t *testing.T) {
	dir, err := filepath.Abs("examples/unix-evidence/connectors")
	if err != nil {
		t.Fatal(err)
	}
	// Invoke the fixture provider's public filter contract to get a native result.
	cmd := exec.Command("python3", "examples/unix-evidence/server/dispatch", "tools/call")
	cmd.Stdin = strings.NewReader(`{"name":"lookup_entitlements","arguments":{"customer_id":"demo-customer"}}`)
	payload, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		edit   func(map[string]any)
		status int
		exit   int
	}{
		{name: "table"},
		{name: "empty", edit: func(r map[string]any) {
			r["structuredContent"].(map[string]any)["table"].(map[string]any)["rows"] = []any{}
		}, exit: 1},
		{name: "tool error", edit: func(r map[string]any) { r["isError"] = true }, exit: 2},
		{name: "unfinished", edit: func(r map[string]any) { r["resultType"] = "input_required" }, exit: 2},
		{name: "unstructured", edit: func(r map[string]any) { delete(r, "structuredContent") }, exit: 2},
		{name: "empty partial", edit: func(r map[string]any) {
			data := r["structuredContent"].(map[string]any)
			data["table"].(map[string]any)["rows"] = []any{}
			data["truncated"] = true
		}, exit: 2},
		{name: "uncertain transport", status: 125, exit: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result map[string]any
			if err := json.Unmarshal(payload, &result); err != nil {
				t.Fatal(err)
			}
			if tc.edit != nil {
				tc.edit(result)
			}
			raw, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			body := "cat <<'RESULT'\n" + string(raw) + "\nRESULT\n"
			if tc.status != 0 {
				body += "exit 125\n"
			}
			writeConnector(t, bin, "mcp", body)
			t.Setenv("PATH", bin+string(filepath.ListSeparator)+os.Getenv("PATH"))
			code, stdout, stderr := runContext(t, dir, "Can demo-customer export?\n", "query", "entitlements")
			if code != tc.exit {
				t.Fatalf("exit=%d want=%d stderr=%q", code, tc.exit, stderr)
			}
			if tc.exit != 0 {
				if stdout != "" {
					t.Fatalf("failed result leaked evidence: %q", stdout)
				}
				return
			}
			row := decodeRows(t, stdout)[0]
			table := row["content"].(map[string]any)
			cells := table["rows"].([]any)[0].([]any)
			if row["type"] != "table" || cells[0] != "demo-customer" || cells[1] != true ||
				row["execution"].(map[string]any)["parameters"].(map[string]any)["customer_id"] != "demo-customer" {
				t.Fatalf("lost provider structure: %#v", row)
			}
		})
	}
}
