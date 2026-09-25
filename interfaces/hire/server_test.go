package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func testApp(t *testing.T) *app {
	t.Helper()
	m, data := newTestJobs(t)
	source := t.TempDir()
	command := filepath.Join(t.TempDir(), "fake public tool")
	writeFixture(t, command, "#!/bin/sh\nprintf '%s\\n' \"$@\"\nprintf 'progress only\\n' >&2\nif [ \"$1\" = verify ]; then\n  test -f \"$2/README.md\" || exit 1\nfi\n", 0700)
	c := catalog{Revision: strings.Repeat("a", 40), Source: source,
		Workers: []entry{{ID: "writer", Description: "Write clearly", Status: "active", Kind: "worker"}, {ID: "trial", Description: "Try a specialty", Status: "experimental", Kind: "worker"}},
		Teams:   []entry{{ID: "studio", Description: "A writing team", Status: "active", Kind: "team", Members: map[string]member{"writer": {Worker: "writer"}}}}}
	a, err := newApp(config{Source: source, Data: data, Hire: command, Python: command}, c, m, "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func formFor(a *app) url.Values {
	return url.Values{"csrf": {a.csrf}, "nonce": {randomID()}, "title": {"Support writer"}, "goal": {"Write a useful reply"}, "mode": {"blank"}}
}

func serveTest(a *app, method, path string, form url.Values) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8787"+path, strings.NewReader(form.Encode()))
	if method == "POST" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	return w
}

func admittedJob(t *testing.T, a *app, w *httptest.ResponseRecorder) Job {
	t.Helper()
	if w.Code != 303 {
		t.Fatalf("submit: status %d: %s", w.Code, w.Body.String())
	}
	id := strings.TrimPrefix(w.Header().Get("Location"), "/jobs/")
	return awaitJob(t, a.jobs, id)
}

func TestHireLiteralScaffoldAndDuplicateSubmission(t *testing.T) {
	a := testApp(t)
	f := formFor(a)
	goal := "$(touch should-not-exist); 'quoted' & useful"
	f.Set("goal", goal)
	w := serveTest(a, "POST", "/hire", f)
	j := admittedJob(t, a, w)
	want := []string{a.cfg.Hire, "new", filepath.Join(j.Dir, "expert"), goal}
	if !reflect.DeepEqual(j.Args, want) {
		t.Fatalf("argv: %#v", j.Args)
	}
	if got := a.jobs.Log(j.ID, "stdout"); got != strings.Join(want[1:], "\n")+"\n" {
		t.Fatalf("literal argv lost: %q", got)
	}
	if a.jobs.Log(j.ID, "stderr") != "progress only\n" {
		t.Fatal("stderr was not retained separately")
	}
	if jobLabel(j) != "Draft created" || !strings.Contains(jobNote(j), "failing check") {
		t.Fatalf("draft misrepresented: %s: %s", jobLabel(j), jobNote(j))
	}
	again := serveTest(a, "POST", "/hire", f)
	if again.Code != 303 || again.Header().Get("Location") != w.Header().Get("Location") || len(a.jobs.List()) != 1 {
		t.Fatal("duplicate form started a second command")
	}
	verify := admittedJob(t, a, serveTest(a, "POST", "/jobs/"+j.ID+"/verify", formFor(a)))
	if !reflect.DeepEqual(verify.Args, []string{a.cfg.Hire, "verify", filepath.Join(j.Dir, "expert")}) {
		t.Fatalf("verify argv: %#v", verify.Args)
	}
	if verify.ExitCode == nil || *verify.ExitCode != 1 || jobLabel(verify) == "Structure verified" {
		t.Fatal("incomplete scaffold reported structurally ready")
	}
}

func TestBuildRequiresServerEnablementAndConsent(t *testing.T) {
	a := testApp(t)
	f := formFor(a)
	f.Set("mode", "build")
	f.Set("model-consent", "yes")
	if w := serveTest(a, "POST", "/hire", f); w.Code != 403 {
		t.Fatalf("disabled model build: %d", w.Code)
	}
	a.cfg.AllowBuild = true
	f.Del("model-consent")
	if w := serveTest(a, "POST", "/hire", f); w.Code != 422 {
		t.Fatalf("unconfirmed model build: %d", w.Code)
	}
	if len(a.jobs.List()) != 0 {
		t.Fatal("rejected model request launched a command")
	}
	f.Set("model-consent", "yes")
	f.Set("model", "openai-codex/gpt-6-sol")
	f.Set("reads", "question.txt")
	f.Set("writes", "reply.md")
	f.Set("good", "Cites supplied facts")
	f.Set("bad", "Invents a policy")
	j := admittedJob(t, a, serveTest(a, "POST", "/hire", f))
	base := filepath.Dir(j.Dir)
	brief := filepath.Join(base, "brief.txt")
	want := []string{a.cfg.Hire, "build", "-C", j.Dir, "-evidence", filepath.Join(base, "evidence"), "-goal-file", brief, "-m", "openai-codex/gpt-6-sol", "-turns", "8", "-timeout", "10m"}
	if !reflect.DeepEqual(j.Args, want) {
		t.Fatalf("build argv: %#v", j.Args)
	}
	if within(j.Dir, brief) || within(j.Dir, filepath.Join(base, "evidence")) || within(a.cfg.Source, j.Dir) {
		t.Fatal("work, evidence and source boundaries overlap")
	}
	b, err := os.ReadFile(brief)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Write a useful reply", "Inputs:\nquestion.txt", "Outputs:\nreply.md", "Acceptable result:\nCites supplied facts", "Result to reject:\nInvents a policy"} {
		if !strings.Contains(string(b), text) {
			t.Fatalf("brief omitted %q", text)
		}
	}
	for _, path := range []string{j.Dir, filepath.Join(base, "evidence"), brief} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("not private: %s", path)
		}
	}
}

func TestInvalidBriefPreservesInput(t *testing.T) {
	a := testApp(t)
	f := formFor(a)
	f.Set("title", "")
	f.Set("goal", "A valuable brief <script>must stay text</script>")
	f.Set("reads", "policy.txt")
	w := serveTest(a, "POST", "/hire", f)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "A valuable brief &lt;script&gt;") || !strings.Contains(w.Body.String(), `value="policy.txt"`) || !strings.Contains(w.Body.String(), "Your brief is kept below") {
		t.Fatalf("brief lost or unescaped on validation: %d %s", w.Code, w.Body.String())
	}
}

func TestValidUnicodeAndEscapedBriefSurvivesSubmissionAndRestart(t *testing.T) {
	for _, character := range []string{"文", "<", "😀"} {
		t.Run(character, func(t *testing.T) {
			a := testApp(t)
			form := formFor(a)
			goal := strings.Repeat(character, 24000)
			form.Set("goal", goal)
			job := admittedJob(t, a, serveTest(a, "POST", "/hire", form))
			if job.ExitCode == nil || *job.ExitCode != 0 || job.Args[3] != goal {
				t.Fatal("valid encoded brief rejected or changed")
			}
			if err := a.jobs.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := newJobManager(a.cfg.Data)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			retained, ok := reopened.Get(job.ID)
			if !ok || retained.Args[3] != goal {
				t.Fatal("valid brief's JSON record was lost on restart")
			}
		})
	}
	a := testApp(t)
	f := formFor(a)
	f.Set("good", strings.Repeat("文", 1501))
	if w := serveTest(a, "POST", "/hire", f); w.Code != 422 || !strings.Contains(w.Body.String(), "Your brief is kept below") {
		t.Fatal("decoded example limit not enforced with preserved form")
	}
}

func BenchmarkCatalogPage(b *testing.B) {
	m, err := newJobManager(filepath.Join(b.TempDir(), "private"))
	if err != nil {
		b.Fatal(err)
	}
	defer m.Close()
	c := catalog{Revision: strings.Repeat("a", 40)}
	for range 22 {
		c.Workers = append(c.Workers, entry{ID: "presentation-designer", Description: "Create a coherent presentation with editable charts, tables and text.", Kind: "worker", Status: "experimental"})
	}
	a, err := newApp(config{}, c, m, "127.0.0.1:8787")
	if err != nil {
		b.Fatal(err)
	}
	h := a.handler()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:8787/", nil))
		if w.Code != 200 {
			b.Fatal(w.Code)
		}
	}
}

func TestPinnedExportsAndExperimentalConsent(t *testing.T) {
	for _, test := range []struct{ kind, id, status, command string }{{"worker", "writer", "active", "export"}, {"team", "studio", "active", "export-team"}, {"worker", "trial", "experimental", "export"}} {
		t.Run(test.id, func(t *testing.T) {
			a := testApp(t)
			f := formFor(a)
			f.Set("kind", test.kind)
			f.Set("id", test.id)
			f.Set("revision", a.cat.Revision)
			if test.status == "experimental" {
				if w := serveTest(a, "POST", "/exports", f); w.Code != 422 {
					t.Fatalf("experimental export without consent: %d", w.Code)
				}
				f.Set("experimental", "yes")
			}
			f.Set("revision", strings.Repeat("b", 40))
			if w := serveTest(a, "POST", "/exports", f); w.Code != 409 {
				t.Fatalf("stale commit accepted: %d", w.Code)
			}
			f.Set("revision", a.cat.Revision)
			j := admittedJob(t, a, serveTest(a, "POST", "/exports", f))
			want := []string{a.cfg.Python, filepath.Join(a.cfg.Source, "scripts", "workers"), test.command, test.id, filepath.Join(j.Dir, "export"), "--ref", a.cat.Revision}
			if test.status == "experimental" {
				want = append(want, "--allow-experimental")
			}
			if !reflect.DeepEqual(j.Args, want) {
				t.Fatalf("export argv: %#v", j.Args)
			}
		})
	}
}

func TestHTTPBoundaryRejectsUntrustedMutations(t *testing.T) {
	a := testApp(t)
	for _, test := range []struct{ name, host, origin, fetch, token string }{{"host", "attacker.example", "", "", a.csrf}, {"origin", "127.0.0.1:8787", "https://attacker.example", "", a.csrf}, {"fetch", "127.0.0.1:8787", "", "cross-site", a.csrf}, {"csrf", "127.0.0.1:8787", "", "", "wrong"}} {
		t.Run(test.name, func(t *testing.T) {
			f := formFor(a)
			f.Set("csrf", test.token)
			r := httptest.NewRequest("POST", "http://"+test.host+"/hire", strings.NewReader(f.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", test.origin)
			r.Header.Set("Sec-Fetch-Site", test.fetch)
			w := httptest.NewRecorder()
			a.handler().ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("rejected mutation returned %d", w.Code)
			}
		})
	}
	f := formFor(a)
	f.Set("goal", strings.Repeat("x", 400<<10))
	if w := serveTest(a, "POST", "/hire", f); w.Code != 413 {
		t.Fatalf("unbounded body: %d", w.Code)
	}
	if len(a.jobs.List()) != 0 {
		t.Fatal("rejected request started work")
	}
	w := serveTest(a, "GET", "/", nil)
	if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing response protection")
	}
}

func TestCatalogAndReadmeAreEscaped(t *testing.T) {
	a := testApp(t)
	payload := "<script>alert('unsafe')</script>"
	a.cat.Workers[0].Description = payload
	writeFixture(t, filepath.Join(a.cfg.Source, "workers", "writer", "expert", "README.md"), payload, 0600)
	for _, path := range []string{"/", "/workers/writer"} {
		w := serveTest(a, "GET", path, nil)
		if w.Code != 200 {
			t.Fatalf("%s: %d: %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), payload) || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
			t.Fatalf("%s failed to escape source text", path)
		}
	}
	if w := serveTest(a, "GET", "/workers/missing", nil); w.Code != 404 {
		t.Fatalf("missing worker: %d", w.Code)
	}
	if w := serveTest(a, "GET", "/?q="+strings.Repeat("a", 501), nil); w.Code != 400 {
		t.Fatalf("unbounded search: %d", w.Code)
	}
}

func TestHTTPJobStatusDistinguishesUnfinishedAndUncertain(t *testing.T) {
	a := testApp(t)
	for _, test := range []struct{ code, label string }{{"2", "Unfinished"}, {"125", "Outcome uncertain"}, {"130", "Interrupted"}} {
		j, err := a.jobs.Start("build", "Fixture", a.cfg.Data, helperArgs("echo", test.code, "literal"), "")
		if err != nil {
			t.Fatal(err)
		}
		j = awaitJob(t, a.jobs, j.ID)
		w := serveTest(a, "GET", "/jobs/"+j.ID+"/status", nil)
		var status struct {
			Active                bool
			Label, Stdout, Stderr string
		}
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status.Active || status.Label != test.label || status.Stdout != "args=literal\ninput=" || status.Stderr != "progress only\n" {
			t.Fatalf("incorrect status: %+v", status)
		}
	}
	if w := serveTest(a, http.MethodGet, "/jobs/"+strings.Repeat("f", 32)+"/status", nil); w.Code != 404 {
		t.Fatalf("missing job: %d", w.Code)
	}
}

func TestBuildModelSelection(t *testing.T) {
	a := testApp(t)
	a.cfg.AllowBuild = true
	a.cfg.Model = "openai-codex/gpt-6-sol"
	if body := serveTest(a, "GET", "/hire", nil).Body.String(); !strings.Contains(body, `value="openai-codex/gpt-6-sol"`) {
		t.Fatal("configured model not shown")
	}
	f := formFor(a)
	f.Set("mode", "build")
	f.Set("model-consent", "yes")
	for _, model := range []string{"", "gpt-6-sol", "-m provider/model", "provider/model\nother", "provider/" + strings.Repeat("x", 200)} {
		f.Set("model", model)
		w := serveTest(a, "POST", "/hire", f)
		if w.Code != 422 || !strings.Contains(w.Body.String(), "Write a useful reply") || len(a.jobs.List()) != 0 {
			t.Fatalf("invalid model %q started work or lost brief: %d", model, w.Code)
		}
	}
	f.Set("model", "fixture/selected-model")
	j := admittedJob(t, a, serveTest(a, "POST", "/hire", f))
	found := false
	for i, arg := range j.Args {
		if arg == "-m" && i+1 < len(j.Args) && j.Args[i+1] == "fixture/selected-model" {
			found = true
		}
	}
	if !found {
		t.Fatalf("form selection did not override default: %v", j.Args)
	}
}

func TestMissingModelDiagnostic(t *testing.T) {
	for _, tc := range []struct{ diagnostic, label string }{
		{"ask: no model: pass -m provider/model or set ASK_MODEL", "Choose a model"},
		{"ask: openai-codex requires -header-fd; run with oauth with PROFILE -- ask -header-fd 3", "Connect your model account"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			a := testApp(t)
			writeFixture(t, a.cfg.Hire, "#!/bin/sh\nprintf '"+tc.diagnostic+"\\n' >&2\nexit 1\n", 0700)
			j, err := a.jobs.Start("build", "Retained failed build", a.cfg.Data, []string{a.cfg.Hire}, "")
			if err != nil {
				t.Fatal(err)
			}
			j = awaitJob(t, a.jobs, j.ID)
			for _, path := range []string{"/jobs/" + j.ID, "/jobs/" + j.ID + "/status"} {
				w := serveTest(a, "GET", path, nil)
				if w.Code != 200 || !strings.Contains(w.Body.String(), tc.label) || !strings.Contains(w.Body.String(), "has not been retried") {
					t.Fatalf("missing actionable diagnostic: %s", w.Body.String())
				}
			}
			if j.ExitCode == nil || *j.ExitCode != 1 || j.State != "failed" || len(a.jobs.List()) != 1 {
				t.Fatal("failure record changed")
			}
			j.State = "unknown"
			if label, _ := jobSummary(j, a.jobs.Log(j.ID, "stderr")); label != "Outcome unknown" {
				t.Fatal("diagnostic obscured unknown outcome")
			}

		})
	}
}
