package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleProjection = `{"schema":"agenda.projection/v1","as_of":"2026-09-29T15:00:00Z","snapshot_sha256":"test","coverage":"complete","obligations":[{"id":"review","revision":"abc","title":"Review <script>alert(1)</script>","owner":"Morgan","not_before":"2026-09-29T09:00:00Z","due_at":"2026-09-29T16:00:00Z","timezone":"UTC","basis":"human","state":"completed","execution":"human-reported","acceptance":"reported-done","timeliness":"on-time","attention":[]},{"id":"delivery","revision":"def","title":"Deliver report","owner":"Riley","not_before":"2026-09-29T09:00:00Z","due_at":"2026-09-29T14:00:00Z","timezone":"UTC","basis":"external","state":"active","execution":"unknown","acceptance":"unconfirmed","timeliness":"overdue","attention":["execution-unknown"]}],"attention":[{"id":"delivery","reasons":["execution-unknown"]}],"errors":[],"notice":"Observations are caller-supplied."}`

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func fakeController(t *testing.T) controller {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "work root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "agenda;literal")
	script := "#!/bin/sh\nset -eu\nprintf '%s\\n' \"$@\" >> " + shellQuote(filepath.Join(dir, "argv")) + "\ncase $1 in\nexport) printf '%s\\n' '{\"schema\":\"agenda.snapshot/v1\",\"records\":[],\"head\":null}' ;;\nproject) [ -f \"$2\" ]; cat > " + shellQuote(filepath.Join(dir, "observed")) + "; printf '%s\\n' " + shellQuote(sampleProjection) + " ;;\napply) cat > " + shellQuote(filepath.Join(dir, "applied")) + "; printf '%s\\n' '{\"schema\":\"agenda.record/v1\",\"revision\":\"result\"}' ;;\nexpand) cat > " + shellQuote(filepath.Join(dir, "expanded")) + "; printf '%s\\n' '{\"id\":\"daily/2026-09-29\"}' ;;\n*) exit 9;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	c := controller{Agenda: path, Root: root}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSavedViewsNeedNoControllerAndEscapeContent(t *testing.T) {
	for _, view := range []string{"calendar", "kanban"} {
		t.Run(view, func(t *testing.T) {
			var out, errout bytes.Buffer
			err := run(context.Background(), []string{"render", "--view", view}, strings.NewReader(sampleProjection), &out, &errout)
			if err != nil {
				t.Fatal(err)
			}
			text := out.String()
			for _, want := range []string{"Morgan", "Riley", "Human-reported completion", "execution-unknown", "read-only snapshot", "&lt;script&gt;"} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Contains(text, "Review <script>") {
				t.Fatal("unescaped obligation text")
			}
			if strings.Contains(text, `data-live="true"`) {
				t.Fatal("offline view is live")
			}
		})
	}
}

func TestStrictJSONAndProjectionIdentity(t *testing.T) {
	for _, raw := range []string{`{"schema":"x","schema":"agenda.projection/v1"}`, sampleProjection + `{}`, strings.Replace(sampleProjection, `"id":"delivery"`, `"id":"review"`, 1)} {
		if _, err := readProjection([]byte(raw)); err == nil {
			t.Fatal("accepted ambiguous projection")
		}
	}
}

func TestProjectUsesLiteralPublicCommandsAndSelectedObservation(t *testing.T) {
	c := fakeController(t)
	dir := filepath.Dir(c.Agenda)
	observations := `{"id":"delivery","state":"unknown"}`
	c.Observations = filepath.Join(dir, "observation.jsonl")
	if err := os.WriteFile(c.Observations, []byte(observations), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := c.project(context.Background(), "2026-09-29T15:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = readProjection(raw); err != nil {
		t.Fatal(err)
	}
	args, _ := os.ReadFile(filepath.Join(dir, "argv"))
	if !strings.Contains(string(args), "export\n"+c.Root+"\nproject\n") || !strings.Contains(string(args), "--as-of\n2026-09-29T15:00:00Z\n") {
		t.Fatalf("wrong argv: %s", args)
	}
	observed, _ := os.ReadFile(filepath.Join(dir, "observed"))
	if string(observed) != observations {
		t.Fatal("selected observations were changed")
	}
}

func TestReadOnlyHTTPAndNoControllerPathArguments(t *testing.T) {
	c := fakeController(t)
	s := &viewServer{Controller: c, Poll: 5}
	handler := s.handler(8791)
	for _, path := range []string{"/calendar", "/kanban"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:8791"+path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Morgan") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private view must not cache")
		}
	}
	for _, test := range []struct {
		method, path, host, origin string
		code                       int
	}{{"POST", "/calendar", "127.0.0.1:8791", "", 405}, {"GET", "/calendar?root=/tmp", "127.0.0.1:8791", "", 400}, {"GET", "/calendar", "evil.example", "", 403}, {"GET", "/calendar", "127.0.0.1:8791", "https://evil.example", 403}, {"GET", "/missing", "127.0.0.1:8791", "", 404}} {
		r := httptest.NewRequest(test.method, "http://127.0.0.1:8791"+test.path, nil)
		r.Host = test.host
		r.Header.Set("Origin", test.origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.code {
			t.Fatalf("%+v: %d", test, w.Code)
		}
	}
}

func TestMCPReadOnlyAndScoping(t *testing.T) {
	c := fakeController(t)
	for _, raw := range []string{`{"name":"agenda_apply","arguments":{"change":{}}}`, `{"name":"agenda_project","arguments":{"as_of":"2026-09-29T15:00:00Z","root":"/tmp/other"}}`, `{"name":"agenda_export","arguments":{"executable":"/bin/sh"}}`, `{"name":"agenda_expand","arguments":{"as_of":"2026-09-29T15:00:00Z","schedule":{"path":"/etc/passwd"}}}`} {
		var out bytes.Buffer
		if err := dispatch(context.Background(), c, []byte(raw), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"isError":true`) {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(c.Agenda), "applied")); !os.IsNotExist(err) {
		t.Fatal("disabled write invoked Agenda")
	}
	var out bytes.Buffer
	if err := dispatch(context.Background(), c, []byte(`{"name":"agenda_project","arguments":{"as_of":"2026-09-29T15:00:00Z"}}`), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "structuredContent") {
		t.Fatal("missing structured MCP result")
	}
}

func TestScopedProofRejectsEscapesSymlinksAndChangedBytes(t *testing.T) {
	dir := t.TempDir()
	proof := []byte("selected proof")
	sum := sha256.Sum256(proof)
	digest := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(dir, "proof.txt"), proof, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if raw, err := scopedEvidence(root, "proof.txt", digest); err != nil || !bytes.Equal(raw, proof) {
		t.Fatalf("proof: %s %v", raw, err)
	}
	if err := os.Symlink(filepath.Join(dir, "proof.txt"), filepath.Join(dir, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc", filepath.Join(dir, "outside")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"/etc/passwd", "../proof.txt", "a/../proof.txt", "linked", "outside/passwd", "./proof.txt"} {
		if _, err := scopedEvidence(root, name, digest); err == nil {
			t.Errorf("accepted %s", name)
		}
	}
	if _, err := scopedEvidence(root, "proof.txt", strings.Repeat("0", 64)); err == nil {
		t.Fatal("accepted wrong proof hash")
	}
}

func TestManifestWriteGateAndCommandStatus(t *testing.T) {
	read, _ := json.Marshal(manifest(false))
	write, _ := json.Marshal(manifest(true))
	if strings.Contains(string(read), "agenda_apply") || !strings.Contains(string(write), "agenda_apply") {
		t.Fatal("manifest write gate differs")
	}
	c := fakeController(t)
	if err := os.WriteFile(c.Agenda, []byte("#!/bin/sh\nprintf 'unknown effect' >&2\nexit 125\n"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err := c.call(context.Background(), nil, "export", c.Root)
	var command *commandError
	if !errors.As(err, &command) || command.Code != 125 {
		t.Fatalf("lost public status: %v", err)
	}
}

func TestColumnsPreserveExplicitHumanAndQueueStates(t *testing.T) {
	for _, test := range []struct {
		row  obligation
		want string
	}{
		{obligation{Basis: "human", State: "active", ReportedState: "planned"}, "Planned"},
		{obligation{Basis: "human", State: "active", ReportedState: "ready"}, "Ready"},
		{obligation{Basis: "human", State: "active", ReportedState: "in-progress"}, "In progress"},
		{obligation{Basis: "external", State: "active", Execution: "queued"}, "Ready"},
		{obligation{Basis: "external", State: "active", Execution: "unobserved"}, "Planned"},
		{obligation{Basis: "external", State: "active", Execution: "waiting"}, "Needs attention"},
	} {
		if got := cardColumn(test.row); got != test.want {
			t.Errorf("%+v: %s not %s", test.row, got, test.want)
		}
	}
}

func TestICSAndSuppliedCoordination(t *testing.T) {
	p, err := readProjection([]byte(sampleProjection))
	if err != nil {
		t.Fatal(err)
	}
	p.Obligations[0].Title = "Review, 旅; " + strings.Repeat("旅", 50) + "\nnext"
	var observation struct {
		Extensions map[string]json.RawMessage `json:"extensions"`
	}
	observation.Extensions = map[string]json.RawMessage{"human_coordination": json.RawMessage(`{"state":"awaiting-input"}`), "bench_status": json.RawMessage(`{"milestones":[{"id":"review","state":"unconfirmed"}]}`)}
	p.Obligations[0].Observation = &observation
	for _, view := range []string{"calendar", "kanban"} {
		var out bytes.Buffer
		if err := render(&out, p, view, false, 5); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "awaiting-input") || !strings.Contains(out.String(), "milestones") {
			t.Fatalf("%s lost supplied status", view)
		}
	}
	var out bytes.Buffer
	if err := render(&out, p, "ics", false, 5); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out.String(), "\r\n") {
		if len(line) > 75 {
			t.Fatal("ICS exceeds octet folding limit")
		}
	}
	unfolded := strings.ReplaceAll(out.String(), "\r\n ", "")
	if strings.Count(unfolded, "BEGIN:VEVENT") != 2 || !strings.Contains(unfolded, `Review\, 旅\;`) || !strings.Contains(unfolded, `\nnext`) {
		t.Fatalf("bad ICS: %s", unfolded)
	}
}
