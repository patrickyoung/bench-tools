package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in real public-process check; no model or user configuration is used.
// BENCH_UI_INTEGRATION_SOURCE=/absolute/bench-tools \
// BENCH_UI_INTEGRATION_BIN=/absolute/bench-tools/.build/bin \
// GOWORK=off go test -run '^TestPublicExecutableIntegration$' -v .
func TestPublicExecutableIntegration(t *testing.T) {
	source, bin := os.Getenv("BENCH_UI_INTEGRATION_SOURCE"), os.Getenv("BENCH_UI_INTEGRATION_BIN")
	if source == "" && bin == "" {
		t.Skip("set BENCH_UI_INTEGRATION_SOURCE and BENCH_UI_INTEGRATION_BIN for public-process integration")
	}
	if !filepath.IsAbs(source) || !filepath.IsAbs(bin) {
		t.Fatal("both integration paths must be explicit and absolute")
	}
	for _, tool := range []string{"hire", "agent", "brief", "ask", "ply", "cage", "record"} {
		info, err := os.Stat(filepath.Join(bin, tool))
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			t.Fatalf("build public %s in the selected binary directory first", tool)
		}
	}
	if os.Getenv("BENCH_UI_INTEGRATION_CHILD") != "1" {
		// Run with an allowlisted environment, never inheriting model credentials,
		// command overrides, user configuration or a user's HOME.
		temp := t.TempDir()
		home, tmp := filepath.Join(temp, "home"), filepath.Join(temp, "tmp")
		for _, dir := range []string{home, tmp} {
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
		}
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Fatal(err)
		}
		python, err = filepath.Abs(python)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestPublicExecutableIntegration$", "-test.v")
		cmd.Env = []string{
			"BENCH_UI_INTEGRATION_CHILD=1", "BENCH_UI_INTEGRATION_SOURCE=" + source, "BENCH_UI_INTEGRATION_BIN=" + bin, "BENCH_UI_INTEGRATION_PYTHON=" + python,
			"PATH=" + bin + ":/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + home, "TMPDIR=" + tmp, "XDG_CONFIG_HOME=" + filepath.Join(home, "config"), "XDG_DATA_HOME=" + filepath.Join(home, "data"),
			"HIRE_AGENT=" + filepath.Join(bin, "agent"), "AGENT_BRIEF=" + filepath.Join(bin, "brief"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LANG=C", "LC_ALL=C",
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated integration: %v\n%s", err, output)
		}
		t.Log(string(output))
		return
	}
	cfg, err := prepare(config{Source: source, Data: filepath.Join(t.TempDir(), "private"), Hire: filepath.Join(bin, "hire"), Python: os.Getenv("BENCH_UI_INTEGRATION_PYTHON")})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := loadCatalog(cfg.Source, cfg.Python)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Workers) == 0 || len(cat.Teams) == 0 {
		t.Fatal("real catalog needs workers and teams")
	}
	manager, err := newJobManager(cfg.Data)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	app, err := newApp(cfg, cat, manager, "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	submit := func(path string, form url.Values) Job {
		t.Helper()
		form.Set("csrf", app.csrf)
		form.Set("nonce", randomID())
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8787"+path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "http://127.0.0.1:8787")
		rec := httptest.NewRecorder()
		app.handler().ServeHTTP(rec, req)
		if rec.Code != 303 {
			t.Fatalf("%s: status %d: %s", path, rec.Code, rec.Body.String())
		}
		return awaitJob(t, manager, strings.TrimPrefix(rec.Header().Get("Location"), "/jobs/"))
	}
	requireExit := func(job Job, want int) {
		t.Helper()
		if job.ExitCode == nil || *job.ExitCode != want {
			t.Fatalf("want exit %d; job=%+v\nstdout=%s\nstderr=%s", want, job, manager.Log(job.ID, "stdout"), manager.Log(job.ID, "stderr"))
		}
	}
	for _, item := range []struct{ kind, id string }{{"worker", "frontend"}, {"team", "page-team"}} {
		entry, ok := cat.find(item.kind, item.id)
		if !ok || !entry.Exportable() {
			t.Fatalf("integration fixture %s %s unavailable", item.kind, item.id)
		}
		job := submit("/exports", url.Values{"kind": {item.kind}, "id": {item.id}, "revision": {cat.Revision}, "experimental": {"yes"}})
		requireExit(job, 0)
		data, err := os.ReadFile(filepath.Join(job.Dir, "export", item.kind+".lock.json"))
		if err != nil {
			t.Fatal(err)
		}
		var lock struct {
			Source struct {
				Commit string `json:"commit"`
			} `json:"source"`
			Members map[string]json.RawMessage `json:"members"`
		}
		if err := json.Unmarshal(data, &lock); err != nil {
			t.Fatal(err)
		}
		if lock.Source.Commit != cat.Revision {
			t.Fatal("export is not pinned to displayed source")
		}
		if item.kind == "team" {
			if len(lock.Members) != len(entry.Members) {
				t.Fatal("team export omitted declared roster")
			}
			for role := range lock.Members {
				if _, err := os.Stat(filepath.Join(job.Dir, "export", "expert", "agents", role, "AGENTS.md")); err != nil {
					t.Fatalf("member %s was not assembled: %v", role, err)
				}
			}
		}
		t.Logf("real %s export %s at %s succeeded", item.kind, item.id, cat.Revision)
	}
	draft := submit("/hire", url.Values{"title": {"Integration draft"}, "goal": {"Read a supplied question and write a factual answer."}, "mode": {"blank"}})
	requireExit(draft, 0)
	missing := submit("/jobs/"+draft.ID+"/verify", url.Values{})
	requireExit(missing, 1)
	if !strings.Contains(manager.Log(missing.ID, "stderr"), "README.md") {
		t.Fatal("blank draft failed for an unexpected reason")
	}
	expert := filepath.Join(draft.Dir, "expert")
	if err := os.WriteFile(filepath.Join(expert, "README.md"), []byte("# Integration fixture\nRead a supplied question; write an answer. Structural verification must not execute the checker.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(draft.Dir, "generated-check-was-executed")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	if err := os.WriteFile(filepath.Join(expert, "bin", "check"), []byte("#!/bin/sh\nprintf 'must not run' > "+quote(marker)+"\nexit 77\n"), 0700); err != nil {
		t.Fatal(err)
	}
	verified := submit("/jobs/"+draft.ID+"/verify", url.Values{})
	requireExit(verified, 0)
	locals := localEntries(app)
	if len(locals) != 1 || locals[0].JobID != verified.ID {
		t.Fatal("real verified draft did not enter local Workers list")
	}
	for _, path := range []string{"/workers?q=Integration", locals[0].URL()} {
		page := serveTest(app, "GET", path, nil)
		if page.Code != 200 || !strings.Contains(page.Body.String(), "Integration draft") {
			t.Fatalf("verified worker not visible at %s", path)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("structural verification executed generated checker code")
	}
	t.Log("real Hire scaffold and Agent structural verification succeeded; generated checker did not execute")
	// This controller-authored fixture accepts supplied input in the pre-check,
	// exercising Agent execution and recording without any model or service call.
	if err := os.WriteFile(filepath.Join(expert, "bin", "check"), []byte("#!/bin/sh\ncat input.txt > report.md\n"), 0700); err != nil {
		t.Fatal(err)
	}
	app.cfg.AllowRun, app.cfg.Agent = true, filepath.Join(bin, "agent")
	run := submit("/jobs/"+verified.ID+"/run", url.Values{"goal": {"Read the supplied fixture"}, "model": {"fixture/no-model-call"}, "turns": {"20"}, "input-name": {"input.txt"}, "input": {"offline fixture result"}, "output": {"report.md"}, "run-consent": {"yes"}})
	requireExit(run, 0)
	result := serveTest(app, "GET", "/jobs/"+run.ID+"/output", nil)
	if result.Code != 200 || result.Body.String() != "offline fixture result" {
		t.Fatal("real Agent result not downloadable")
	}
	t.Log("real Agent run and result download passed using a zero-model pre-check")
	// Retain an unfinished run with a declared fixture, then continue through
	// real Agent. Its real pre-check accepts the existing input without a model.
	stoppedAgent := filepath.Join(t.TempDir(), "offline unfinished Agent")
	writeFixture(t, stoppedAgent, "#!/bin/sh\nprintf 'partial work' > progress.txt\nexit 2\n", 0700)
	app.cfg.Agent = stoppedAgent
	unfinishedRun := submit("/jobs/"+verified.ID+"/run", url.Values{"goal": {"Finish from retained input"}, "model": {"fixture/no-model-call"}, "turns": {"20"}, "input-name": {"input.txt"}, "input": {"retained continuation input"}, "output": {"report.md"}, "run-consent": {"yes"}})
	requireExit(unfinishedRun, 2)
	app.cfg.Agent = filepath.Join(bin, "agent")
	continuedRun := submit("/jobs/"+unfinishedRun.ID+"/continue", url.Values{})
	requireExit(continuedRun, 0)
	if continuedRun.Dir != unfinishedRun.Dir || continuedRun.Args[11] != "50" {
		t.Fatal("continuation lost work or turn budget")
	}
	if b, err := os.ReadFile(filepath.Join(continuedRun.Dir, "progress.txt")); err != nil || string(b) != "partial work" {
		t.Fatal("continuation lost partial files")
	}
	if w := serveTest(app, "GET", "/jobs/"+continuedRun.ID+"/output", nil); w.Code != 200 || w.Body.String() != "retained continuation input" {
		t.Fatal("continued result unavailable")
	}
	t.Log("real Agent accepted 50-turn continuation and completed from retained input using its zero-model pre-check")

	// Exercise team assembly with real pinned source exports and Hire scaffold/
	// verification. Only model authoring is replaced by a declared offline fixture.
	publicHire := cfg.Hire
	author := filepath.Join(t.TempDir(), "offline team author")
	teamMarker := filepath.Join(t.TempDir(), "team-check-executed")
	fixture := "#!/bin/sh\nset -eu\nif [ \"$1\" != build ]; then exec " + quote(publicHire) + " \"$@\"; fi\n" +
		"printf 'Team fixture instructions\\n' > expert/AGENTS.md\n" +
		"printf '# Team fixture\\nUse bin/team after independent evaluation.\\n' > expert/README.md\n" +
		"printf '%s\\n' '#!/bin/sh' " + quote("touch "+quote(teamMarker)) + " 'exit 77' > expert/bin/check\n" +
		"printf '%s\\n' '#!/bin/sh' 'exit 77' > expert/bin/team\nchmod 700 expert/bin/check expert/bin/team\n"
	writeFixture(t, author, fixture, 0700)
	app.cfg.Hire, app.cfg.AllowBuild = author, true
	teamDraft := submit("/teams", url.Values{
		"title": {"Offline integration team"}, "goal": {"Assemble selected public workers"},
		"handoffs": {"Maker gives result to reviewer"}, "acceptance": {"Independent review rejects unsupported output"},
		"revision": {cat.Revision}, "base": {"page-team"}, "roster-consent": {"yes"},
		"role-0-name": {"maker"}, "role-0-worker": {"source:frontend"}, "role-0-responsibility": {"Create the page"},
		"role-1-name": {"review"}, "role-1-worker": {"local:" + locals[0].ID}, "role-1-responsibility": {"Review the result"},
	})
	requireExit(teamDraft, 0)
	teamBuild := submit("/local-teams/"+teamDraft.ID+"/build", url.Values{"model": {"fixture/no-model-call"}, "model-consent": {"yes"}})
	requireExit(teamBuild, 0)
	spec, err := app.readTeam(teamBuild)
	if err != nil {
		t.Fatal(err)
	}
	_, hash, err := app.teamProposal(teamBuild, spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"base/team.lock.json", "inputs/maker/worker.lock.json"} {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(teamBuild.Dir), name))
		if err != nil || !strings.Contains(string(b), cat.Revision) {
			t.Fatalf("source pin missing in %s: %v", name, err)
		}
	}
	teamSaved := submit("/local-teams/"+teamBuild.ID+"/save", url.Values{"proposal-hash": {hash}, "review-consent": {"yes"}})
	requireExit(teamSaved, 0)
	if len(app.localTeams()) != 1 || app.localTeams()[0].Status != "experimental" {
		t.Fatal("verified team not discoverable")
	}
	if _, err := os.Stat(teamMarker); !os.IsNotExist(err) {
		t.Fatal("team verification executed generated check")
	}
	t.Log("real source-team and worker exports, local member snapshots, Hire scaffold and reviewed team verification passed; model authoring was an offline fixture")

	// Real Hire parses and forwards the UI's new turn/checkpoint options. Only
	// Agent execution is a declared fixture; no account or paid model is used.
	checkpointAgent := filepath.Join(t.TempDir(), "offline checkpoint agent")
	writeFixture(t, checkpointAgent, "#!/bin/sh\nprintf '%s\\n' \"$@\"\nexit 2\n", 0700)
	previousAgent := os.Getenv("HIRE_AGENT")
	t.Setenv("HIRE_AGENT", checkpointAgent)
	app.cfg.Hire = publicHire
	longBuild := submit("/hire", url.Values{"title": {"Long authoring fixture"}, "goal": {"Continue a saved draft"}, "mode": {"build"}, "model": {"fixture/no-model-call"}, "model-consent": {"yes"}})
	requireExit(longBuild, 2)
	more := submit("/jobs/"+longBuild.ID+"/continue", url.Values{})
	requireExit(more, 2)
	for _, j := range []Job{longBuild, more} {
		out := manager.Log(j.ID, "stdout")
		if !strings.Contains(out, "-turns\n50\n") || !strings.Contains(out, "-checkpoint\nbuild\n") || !strings.Contains(out, "-C\n"+j.Dir+"\n") {
			t.Fatalf("Hire did not forward continuation contract: %s", out)
		}
	}
	if more.Dir != longBuild.Dir {
		t.Fatal("continuation changed workspace")
	}
	t.Setenv("HIRE_AGENT", previousAgent)
	t.Log("real Hire accepted 50-turn checkpointed builds and continuation, with declared offline Agent fixture")

	// The real advisory definition and Agent pre-check exercise the UI's exact
	// argv/workspace/response contract. Only response authorship is an offline
	// fixture; no model or personal account is used.
	replyFixture := filepath.Join(t.TempDir(), "offline reply.py")
	writeFixture(t, replyFixture, `import hashlib,json
from pathlib import Path
raw=Path("request.json").read_bytes()
reply={"version":1,"request_sha256":hashlib.sha256(raw).hexdigest(),
       "message":"Offline process fixture, not a model diagnosis.","question":"",
       "actions":[{"kind":"open","label":"Inspect Frontend","target":"worker:source:frontend","fields":{},"roles":[]}]}
Path("response.json").write_text(json.dumps(reply))
`, 0600)
	agentFixture := filepath.Join(t.TempDir(), "prechecked public Agent")
	writeFixture(t, agentFixture, "#!/bin/sh\nset -eu\n"+quote(cfg.Python)+" "+quote(replyFixture)+"\nexec "+quote(filepath.Join(bin, "agent"))+" \"$@\"\n", 0700)
	app.cfg.Agent = agentFixture
	f := assistantForm(app)
	f.Set("message", "Find a worker for a web page")
	w := serveTest(app, "POST", "/assist", f)
	if w.Code != 303 {
		t.Fatalf("advisory admission: %d %s", w.Code, w.Body.String())
	}
	thread := strings.TrimPrefix(w.Header().Get("Location"), "/assist/")
	turns := app.assistantTurns(thread)
	if len(turns) != 1 {
		t.Fatal("missing retained turn")
	}
	turn := awaitJob(t, manager, turns[0].Job.ID)
	requireExit(turn, 0)
	if _, _, err := app.assistantReply(turn); err != nil {
		t.Fatal("real definition/controller disagreement", err)
	}
	if !strings.Contains(serveTest(app, "GET", "/assist/"+thread, nil).Body.String(), "Inspect Frontend") {
		t.Fatal("validated action not visible")
	}
	t.Log("real bench-hire checker and Agent composition passed with a declared prewritten reply; no model-quality claim")

}
