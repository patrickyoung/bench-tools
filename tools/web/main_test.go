package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFixtures(t *testing.T) {
	if e := selfCheck(io.Discard); e != nil {
		t.Fatal(e)
	}
}
func TestOptions(t *testing.T) {
	for _, args := range [][]string{{"snapshot", "https://example.test", "--records-selector"}, {"get", "x", "--attach", "9222", "--profile", "x"}, {"run", "--keep"}, {"run", "--tab", "x"}, {"run", "--attach", "9222", "--keep", "--tab", "x"}, {"get", "x", "--wait", "whatever"}, {"get", "x", "--timeout", "0"}, {"get", "x", "--made-up"}, {"get", "x", "y"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, e := parseOptions(args[0], args[1:])
			if e == nil {
				t.Fatal("accepted invalid options")
			}
		})
	}
	o, e := parseOptions("snapshot", []string{"--attach", "9222", "https://example.test", "--records-selector", "article"})
	if e != nil || o.attach != "http://127.0.0.1:9222" || o.selector != "article" {
		t.Fatalf("%+v %v", o, e)
	}
}
func TestBoundedInput(t *testing.T) {
	if _, e := boundedRead(strings.NewReader(strings.Repeat("x", maxInput+1))); e == nil {
		t.Fatal("accepted oversized input")
	}
	for _, s := range []string{"null", "{}", "[null]", "[3]", "", "[] trailing"} {
		if _, e := parsePlan([]byte(s)); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
func TestGateWords(t *testing.T) {
	for _, kind := range []string{"click", "submit"} {
		if !consequential(kind, map[string]any{"irreversible": false}) {
			t.Fatal("gate bypass")
		}
	}
	if !consequential("read", map[string]any{"irreversible": true}) {
		t.Fatal("missing added gate")
	}
	s := map[string]any{"click": "#send"}
	if got := gateWords("click", s, "example.test"); got != "click #send on example.test" {
		t.Fatal(got)
	}
	s["may"] = "  exact approval  "
	if got := gateWords("click", s, "different.test"); got != "exact approval" {
		t.Fatal(got)
	}
}
func TestMayGate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("CLERK_WORKER", "worker")
	t.Setenv("JOB_ID", "job")
	t.Setenv("CLERK_WROOT", dir)
	t.Setenv("WEB_STATE", filepath.Join(dir, "audit.jsonl"))
	for _, code := range []int{0, 3, 75, 1, 77} {
		fakeMay(t, dir, code)
		e := gate(context.Background(), "click $(no-shell)", "run", "test", "fresh", "", io.Discard)
		want := code
		if code == 1 {
			want = 77
		}
		if want == 0 {
			if e != nil {
				t.Fatal(e)
			}
		} else {
			var f *failure
			if !errors.As(e, &f) || f.code != want {
				t.Fatalf("code %d: %v", code, e)
			}
		}
	}
	b, e := os.ReadFile(filepath.Join(dir, "audit.jsonl"))
	if e != nil || bytes.Count(b, []byte("\n")) != 5 || !bytes.Contains(b, []byte(`"grant":`)) {
		t.Fatalf("audit: %s %v", b, e)
	}
}
func TestAttachDeadlineAndNoFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	t.Setenv("WEB_ATTACH_TIMEOUT", "100")
	t.Setenv("WEB_BROWSER", "/must-not-launch")
	start := time.Now()
	_, e := startSession(context.Background(), options{attach: server.URL}, false, io.Discard)
	if e == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("deadline: %v after %v", e, time.Since(start))
	}
}
func TestProfileAtomicPrivateFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "state")
	other := filepath.Join(dir, "other")
	os.WriteFile(other, []byte("untouched"), 0644)
	if e := os.Symlink(other, target); e != nil {
		t.Fatal(e)
	}
	state := &storageState{Cookies: []storedCookie{}, Origins: []storedOrigin{}}
	if _, e := saveState(target, state); e != nil {
		t.Fatal(e)
	}
	st, e := os.Stat(target)
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal(st, e)
	}
	b, _ := os.ReadFile(other)
	if string(b) != "untouched" {
		t.Fatal("followed destination symlink")
	}
	if _, e := loadState(target); e != nil {
		t.Fatal(e)
	}
}

func fakeMay(t *testing.T, dir string, code int) {
	t.Helper()
	verdict := map[int]string{0: "spent", 3: "declined", 75: "parked"}[code]
	script := fmt.Sprintf(`#!/bin/sh
test "$#" = 2 && test "$1" = request || exit 9
action=$(/bin/cat)
printf '{"version":1,"job":"%%s","action":"%%s","verdict":"%s","digest":"%s"}\n' "$2" "$action"
exit %d
`, verdict, strings.Repeat("a", 64), code)
	if e := os.WriteFile(filepath.Join(dir, "may"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
}

func TestMayRejectsMismatchedResult(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("WEB_STATE", filepath.Join(dir, "audit"))
	for _, reply := range []string{`{"version":1,"job":"job","action":"different","verdict":"spent","digest":"` + strings.Repeat("a", 64) + `"}`, `{"version":1,"job":"job","action":"do it","verdict":"parked","digest":"` + strings.Repeat("a", 64) + `"}`, `not-json`} {
		os.WriteFile(filepath.Join(dir, "may"), []byte("#!/bin/sh\n/bin/cat >/dev/null\nprintf '%s' '"+reply+"'\nexit 0\n"), 0700)
		err := gate(context.Background(), "do it", "run", "", "fresh", "job", io.Discard)
		var f *failure
		if !errors.As(err, &f) || f.code != 77 {
			t.Fatalf("accepted malformed/mismatched May result: %s", reply)
		}
	}
}
func TestPartitionedProfileRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	os.WriteFile(path, []byte(`{"cookies":[{"name":"session","value":"x","domain":"example.test","partitionKey":"https://other.test"}],"origins":[]}`), 0600)
	if _, e := loadState(path); e == nil {
		t.Fatal("partitioned cookie silently widened")
	}
}
