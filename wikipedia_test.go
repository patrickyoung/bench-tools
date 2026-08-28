package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func wikipediaConnectorDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join(".context", "connectors"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestWikipediaConnectorDescribe(t *testing.T) {
	code, stdout, stderr := runContext(t, wikipediaConnectorDir(t), "", "ls")
	if code != exitYes || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	rows := decodeRows(t, stdout)
	found := false
	for _, row := range rows {
		if row["name"] == "wikipedia" {
			found = true
			if !strings.Contains(row["description"].(string), "encyclopedic") {
				t.Fatalf("description=%q", row["description"])
			}
		}
	}
	if !found {
		t.Fatalf("wikipedia missing from rows: %#v", rows)
	}
}

func TestWikipediaConnectorSearchesAndNormalizesResults(t *testing.T) {
	var requestQuery url.Values
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestQuery = r.URL.Query()
		userAgent = r.UserAgent()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"query":{"search":[`+
			`{"pageid":123,"title":"Unix philosophy","snippet":"small &amp; <span class=\"searchmatch\">composable</span> programs","timestamp":"2026-08-01T12:00:00Z","wordcount":900},`+
			`{"pageid":456,"title":"Pipeline (Unix)","snippet":"standard streams","timestamp":"2026-07-01T12:00:00Z","wordcount":500}`+
			`]}}`)
	}))
	defer server.Close()
	t.Setenv("WIKIPEDIA_API", server.URL)
	t.Setenv("WIKIPEDIA_USER_AGENT", "context-test/1.0 (https://example.test/contact)")

	code, stdout, stderr := runContext(t, wikipediaConnectorDir(t),
		"unix filter design", "query", "wikipedia")
	if code != exitYes || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if requestQuery.Get("action") != "query" || requestQuery.Get("list") != "search" ||
		requestQuery.Get("srsearch") != "unix filter design" ||
		requestQuery.Get("srlimit") != "5" || requestQuery.Get("srnamespace") != "0" {
		t.Fatalf("query=%v", requestQuery)
	}
	if userAgent != "context-test/1.0 (https://example.test/contact)" {
		t.Fatalf("User-Agent=%q", userAgent)
	}
	rows := decodeRows(t, stdout)
	if len(rows) != 2 {
		t.Fatalf("rows=%#v", rows)
	}
	first := rows[0]
	if first["source"] != "wikipedia" || first["id"] != "123" ||
		first["ref"] != citationRef("wikipedia", "123") ||
		first["modified_at"] != "2026-08-01T12:00:00Z" {
		t.Fatalf("first=%#v", first)
	}
	content := first["content"].(map[string]any)
	if content["text"] != "small & composable programs" || content["search_rank"] != json.Number("1") {
		t.Fatalf("content=%#v", content)
	}
	citation := first["citation"].(map[string]any)
	if citation["locator"] != "pageid:123" || citation["url"] != "https://en.wikipedia.org/?curid=123" {
		t.Fatalf("citation=%#v", citation)
	}
	license := first["license"].(map[string]any)
	if license["name"] != "CC BY-SA 4.0" {
		t.Fatalf("license=%#v", license)
	}
	checkCode, checkOut, checkErr := runContext(t, wikipediaConnectorDir(t), stdout, "check")
	if checkCode != exitYes || checkOut != "" || checkErr != "" {
		t.Fatalf("check exit=%d stdout=%q stderr=%q", checkCode, checkOut, checkErr)
	}
}

func TestWikipediaConnectorNoResultsAndHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("srsearch") == "nothing" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"query":{"search":[]}}`)
			return
		}
		if r.URL.Query().Get("srsearch") == "malformed" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"query":{"search":[{}]}}`)
			return
		}
		w.Header().Set("Retry-After", "10")
		http.Error(w, "busy", http.StatusTooManyRequests)
	}))
	defer server.Close()
	t.Setenv("WIKIPEDIA_API", server.URL)

	code, stdout, _ := runContext(t, wikipediaConnectorDir(t), "nothing", "query", "wikipedia")
	if code != exitNo || stdout != "" {
		t.Fatalf("no result: exit=%d stdout=%q", code, stdout)
	}
	code, stdout, stderr := runContext(t, wikipediaConnectorDir(t), "busy", "query", "wikipedia")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "retry after 10") {
		t.Fatalf("failure: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = runContext(t, wikipediaConnectorDir(t), "malformed", "query", "wikipedia")
	if code != exitErr || stdout != "" || !strings.Contains(stderr, "invalid API response") {
		t.Fatalf("malformed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
