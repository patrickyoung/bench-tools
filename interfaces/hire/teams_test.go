package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func teamFixture(t *testing.T) (*app, url.Values) {
	t.Helper()
	a := testApp(t)
	a.cfg.AllowBuild = true
	worker := localFixture(t, a, "build", "0")
	writeFixture(t, filepath.Join(worker.Dir, "expert", "AGENTS.md"), "Local worker instructions", 0600)
	writeFixture(t, filepath.Join(worker.Dir, "expert", "bin/check"), "#!/bin/sh\nexit 77\n", 0700)
	local := localEntries(a)[0]
	a.cfg.Hire = filepath.Join(t.TempDir(), "hire ' literal")
	writeFixture(t, a.cfg.Hire, `#!/bin/sh
set -eu
case "$1" in
new)
 mkdir -p "$2/bin"
 printf 'Draft instructions\n' > "$2/AGENTS.md"
 printf '#!/bin/sh\nexit 1\n' > "$2/bin/check"
 chmod 700 "$2/bin/check"
 ;;
build)
 printf 'Team instructions\n' > expert/AGENTS.md
 printf '# Team guide\nUse bin/team with a fresh workspace.\n' > expert/README.md
 printf '#!/bin/sh\nexit 77\n' > expert/bin/check
 printf '#!/bin/sh\nexit 0\n' > expert/bin/team
 chmod 700 expert/bin/check expert/bin/team
 ;;
verify) test -x "$2/bin/team";;
*) exit 19;;
esac
`, 0700)
	a.cfg.Python = filepath.Join(t.TempDir(), "source exporter")
	writeFixture(t, a.cfg.Python, `#!/bin/sh
set -eu
printf '%s\n' "$@"
mkdir -p "$4/expert/bin"
printf 'Exported worker\n' > "$4/expert/README.md"
printf 'Instructions\n' > "$4/expert/AGENTS.md"
printf '#!/bin/sh\nexit 77\n' > "$4/expert/bin/check"
chmod 700 "$4/expert/bin/check"
printf '{"source":"pinned fixture"}\n' > "$4/worker.lock.json"
`, 0700)
	f := formFor(a)
	f.Set("title", "Research ' $(touch INJECTED) team")
	f.Set("goal", "Produce an evidence-backed response")
	f.Set("handoffs", "Researcher hands evidence.md to reviewer.")
	f.Set("acceptance", "Reject unsupported assertions; fresh independent case.")
	f.Set("revision", a.cat.Revision)
	f.Set("roster-consent", "yes")
	f.Set("role-0-name", "Research Lead")
	f.Set("role-0-worker", "source:writer")
	f.Set("role-0-responsibility", "Write evidence.md")
	f.Set("role-1-name", "review")
	f.Set("role-1-worker", "local:"+local.ID)
	f.Set("role-1-responsibility", "Check claims and return review.md")
	return a, f
}
func teamBuildFixture(t *testing.T, a *app, draft Job) Job {
	t.Helper()
	f := formFor(a)
	f.Set("model", "fixture/model")
	f.Set("model-consent", "yes")
	return admittedJob(t, a, serveTest(a, "POST", "/local-teams/"+draft.ID+"/build", f))
}
func TestTeamCreateBuildReviewAndEdit(t *testing.T) {
	a, f := teamFixture(t)
	for _, path := range []string{"/teams", "/teams/new", "/teams/new?base=studio"} {
		w := serveTest(a, "GET", path, nil)
		if w.Code != 200 {
			t.Fatalf("form %s: %d", path, w.Code)
		}
	}
	draft := admittedJob(t, a, serveTest(a, "POST", "/teams", f))
	spec, err := a.readTeam(draft)
	if err != nil || spec.Roles[0].Role != "research-lead" {
		t.Fatal("roster not saved", err)
	}
	if got := serveTest(a, "POST", "/teams", f); got.Header().Get("Location") != "/jobs/"+draft.ID {
		t.Fatal("duplicate draft")
	}
	if len(a.catalog().Teams) != 2 || len(localEntries(a)) != 1 {
		t.Fatal("team missing or became individual worker")
	}
	before, _, err := definitionSnapshot(a.cfg.Data, filepath.Join(filepath.Dir(draft.Dir), "inputs/review/expert"))
	if err != nil {
		t.Fatal(err)
	}
	// A later local-worker edit must not change this draft's selected copy.
	local := localEntries(a)[0]
	writeFixture(t, filepath.Join(local.Path, "README.md"), "later worker revision", 0600)
	built := teamBuildFixture(t, a, draft)
	if built.State != "completed" {
		t.Fatal(a.jobs.Log(built.ID, "stderr"))
	}
	if _, ok := a.runnable(built.ID); ok {
		t.Fatal("team admitted to individual runner")
	}
	if serveTest(a, "POST", "/jobs/"+built.ID+"/verify", formFor(a)).Code != 409 {
		t.Fatal("review bypass")
	}
	if got := a.localTeams(); len(got) != 1 || got[0].Status != "draft" {
		t.Fatal("unreviewed team published")
	}
	for name, want := range before {
		raw, err := os.ReadFile(filepath.Join(built.Dir, "expert/agents/review", name))
		if err != nil || string(raw) != want.Text {
			t.Fatal("local worker pin changed")
		}
	}
	if _, err := os.Stat(filepath.Join(built.Dir, "INJECTED")); !os.IsNotExist(err) {
		t.Fatal("user text executed")
	}
	script, _ := os.ReadFile(filepath.Join(filepath.Dir(built.Dir), "assemble.sh"))
	if strings.Contains(string(script), "touch INJECTED") || !strings.Contains(string(script), a.cat.Revision) {
		t.Fatal("script contains brief text or lost source pin")
	}
	body := serveTest(a, "GET", "/local-teams/"+draft.ID, nil).Body.String()
	if !strings.Contains(body, "Verify and save team") || !strings.Contains(body, "bin/team") {
		t.Fatal("missing review")
	}
	_, hash, err := a.teamProposal(built, spec)
	if err != nil {
		t.Fatal(err)
	}
	save := formFor(a)
	save.Set("review-consent", "yes")
	save.Set("proposal-hash", "stale")
	if serveTest(a, "POST", "/local-teams/"+built.ID+"/save", save).Code != 409 {
		t.Fatal("stale review admitted")
	}
	save = formFor(a)
	save.Set("review-consent", "yes")
	save.Set("proposal-hash", hash)
	verified := admittedJob(t, a, serveTest(a, "POST", "/local-teams/"+built.ID+"/save", save))
	if verified.State != "completed" || a.localTeams()[0].Status != "experimental" {
		t.Fatal("team not saved")
	}
	if w := serveTest(a, "GET", "/teams/new?from="+verified.ID, nil); w.Code != 200 || !strings.Contains(w.Body.String(), "research-lead") {
		t.Fatal("membership edit missing")
	}
	f.Set("nonce", randomID())
	f.Set("parent", verified.ID)
	f.Set("title", "Changed membership")
	f.Set("role-1-worker", "source:trial")
	next := admittedJob(t, a, serveTest(a, "POST", "/teams", f))
	if next.Dir == draft.Dir || len(a.localTeams()) != 2 {
		t.Fatal("edit overwrote original")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(next.Dir), "parent-expert", "bin/team")); err != nil {
		t.Fatal("parent wiring lost")
	}
	writeFixture(t, filepath.Join(verified.Dir, "expert", "README.md"), "Changed after saving", 0600)
	if w := serveTest(a, "GET", "/local-teams/"+verified.ID, nil); !strings.Contains(w.Body.String(), "Team files changed since review") {
		t.Fatal("saved fingerprint not enforced")
	}
	if err := a.jobs.Close(); err != nil {
		t.Fatal(err)
	}
	m, err := newJobManager(a.cfg.Data)
	if err != nil {
		t.Fatal(err)
	}
	a.jobs = m
	t.Cleanup(func() { m.Close() })
	if len(a.localTeams()) != 2 {
		t.Fatal("teams not retained after restart")
	}
}
func TestTeamValidationAndFailedAssembly(t *testing.T) {
	a, f := teamFixture(t)
	bad := cloneTeamForm(f)
	bad.Set("role-1-name", "research lead")
	if w := serveTest(a, "POST", "/teams", bad); w.Code != 422 || !strings.Contains(w.Body.String(), "evidence-backed") {
		t.Fatal("duplicate role accepted or brief lost")
	}
	bad = cloneTeamForm(f)
	bad.Set("role-1-worker", "local:missing")
	if serveTest(a, "POST", "/teams", bad).Code != 422 {
		t.Fatal("missing worker accepted")
	}
	bad = cloneTeamForm(f)
	bad.Set("role-0-name", "../escape")
	if serveTest(a, "POST", "/teams", bad).Code != 422 {
		t.Fatal("path role accepted")
	}
	add := cloneTeamForm(f)
	add.Set("action", "add")
	if w := serveTest(a, "POST", "/teams", add); w.Code != 200 || !strings.Contains(w.Body.String(), "role-2-name") || len(a.localTeams()) != 0 {
		t.Fatal("add row saved team or lost form")
	}
	remove := cloneTeamForm(f)
	remove.Set("remove-role", "0")
	if w := serveTest(a, "POST", "/teams", remove); w.Code != 200 || strings.Contains(w.Body.String(), "role-1-name") || !strings.Contains(w.Body.String(), "Check claims") {
		t.Fatal("remove role lost remaining roster")
	}
	draft := admittedJob(t, a, serveTest(a, "POST", "/teams", f))
	a.cfg.AllowBuild = false
	g := formFor(a)
	g.Set("model", "fixture/model")
	g.Set("model-consent", "yes")
	if serveTest(a, "POST", "/local-teams/"+draft.ID+"/build", g).Code != 422 {
		t.Fatal("disabled build admitted")
	}
	a.cfg.AllowBuild = true
	writeFixture(t, a.cfg.Python, "#!/bin/sh\nprintf 'export refused\\n' >&2\nexit 17\n", 0700)
	failed := teamBuildFixture(t, a, draft)
	if failed.ExitCode == nil || *failed.ExitCode != 17 || !strings.Contains(a.jobs.Log(failed.ID, "stderr"), "export refused") {
		t.Fatal("export exit not retained")
	}
	if serveTest(a, "POST", "/local-teams/"+failed.ID+"/save", formFor(a)).Code != 409 {
		t.Fatal("failed build saved")
	}
	if serveTest(a, "POST", "/local-teams/"+draft.ID+"/build", g).Code != 409 {
		t.Fatal("failed assembly silently retried")
	}
}
func TestTeamMemberIntegrityAndMissingEntrypoint(t *testing.T) {
	a, f := teamFixture(t)
	draft := admittedJob(t, a, serveTest(a, "POST", "/teams", f))
	built := teamBuildFixture(t, a, draft)
	spec, _ := a.readTeam(built)
	path := filepath.Join(built.Dir, "expert/agents/review/README.md")
	original, _ := os.ReadFile(path)
	writeFixture(t, path, "weakened worker", 0600)
	if _, _, err := a.teamProposal(built, spec); err == nil {
		t.Fatal("changed selected worker accepted")
	}
	writeFixture(t, path, string(original), 0600)
	if err := os.Remove(filepath.Join(built.Dir, "expert/bin/team")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.teamProposal(built, spec); err == nil {
		t.Fatal("missing entry accepted")
	}
}

func cloneTeamForm(f url.Values) url.Values { out, _ := url.ParseQuery(f.Encode()); return out }
