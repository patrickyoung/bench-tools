package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func configureAssistant(t *testing.T, a *app) {
	t.Helper()
	a.cfg.AllowBuild = true
	a.cfg.Agent = filepath.Join(t.TempDir(), "agent fixture")
	writeFixture(t, a.cfg.Agent, "#!/bin/sh\nprintf '%s\\n' \"$@\"\n", 0700)
	for name, text := range map[string]string{"AGENTS.md": "Advisory fixture", "README.md": "Fixture guide", "bin/check": "#!/bin/sh\nexit 1\n"} {
		writeFixture(t, filepath.Join(a.assistantPath(), name), text, 0700)
	}
}

func assistantForm(a *app) url.Values {
	f := formFor(a)
	f.Set("message", "Help with this worker <script>untrusted</script>")
	f.Set("model", "fixture/model")
	return f
}

// The fake public Agent only records argv. Supply a separately bound response
// after it exits to exercise controller behavior without a model or credentials.
func assistantFixture(t *testing.T, a *app, f url.Values, actions []assistantAction) (Job, assistantRequest, string) {
	t.Helper()
	w := serveTest(a, "POST", "/assist", f)
	if w.Code != 303 || !strings.HasPrefix(w.Header().Get("Location"), "/assist/") {
		t.Fatalf("conversation submit: %d %s", w.Code, w.Body.String())
	}
	thread := strings.TrimPrefix(w.Header().Get("Location"), "/assist/")
	turns := a.assistantTurns(thread)
	if len(turns) == 0 {
		t.Fatal("lost conversation")
	}
	j := awaitJob(t, a.jobs, turns[len(turns)-1].Job.ID)
	raw, err := os.ReadFile(filepath.Join(j.Dir, "request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var req assistantRequest
	if err = json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	reply := assistantReply{Version: 1, RequestHash: digestText(raw), Message: "Observed facts, then a proposed next step. <script>never execute</script>", Question: "", Actions: actions}
	b, _ := json.Marshal(reply)
	writeFixture(t, filepath.Join(j.Dir, "response.json"), string(b), 0600)
	if _, _, err = a.assistantReply(j); err != nil {
		t.Fatal(err)
	}
	return j, req, thread
}

func createWorkerAction() assistantAction {
	return assistantAction{Kind: "create_worker", Label: "Draft a rules indexer", Fields: map[string]string{"title": "Rules indexer", "goal": "Index supplied rules with citations", "reads": "rules.txt", "writes": "index.md", "good": "Cite an actual rule", "bad": "Invent a rule"}, Roles: []assistantRole{}}
}

func actionForm(t *testing.T, a *app, j Job) url.Values {
	t.Helper()
	_, hash, err := a.assistantReply(j)
	if err != nil {
		t.Fatal(err)
	}
	f := formFor(a)
	f.Set("reply-hash", hash)
	return f
}

func TestAssistantConversationAndExplicitAction(t *testing.T) {
	a := testApp(t)
	configureAssistant(t, a)
	f := assistantForm(a)
	j, req, thread := assistantFixture(t, a, f, []assistantAction{createWorkerAction()})
	if len(a.jobs.List()) != 1 || req.KnowledgeSource != a.cfg.Source || len(req.History) != 0 {
		t.Fatal("unexpected effects/context")
	}
	argv := strings.Join(j.Args, " ")
	for _, forbidden := range []string{"-network", "-no-cage"} {
		if strings.Contains(argv, forbidden) {
			t.Fatal("expanded authority")
		}
	}
	for _, needed := range []string{"-turns 6", "-record-input request.json", "-record-output response.json"} {
		if !strings.Contains(argv, needed) {
			t.Fatal("missing boundary", needed)
		}
	}
	if again := serveTest(a, "POST", "/assist", f); again.Header().Get("Location") != "/assist/"+thread || len(a.jobs.List()) != 1 {
		t.Fatal("duplicate message")
	}
	body := serveTest(a, "GET", "/assist/"+thread, nil).Body.String()
	if strings.Contains(body, "<script>never") || !strings.Contains(body, "&lt;script&gt;never") || !strings.Contains(body, "Build this worker") {
		t.Fatal("unescaped or missing proposal")
	}
	if serveTest(a, "POST", "/jobs/"+j.ID+"/verify", formFor(a)).Code != 409 {
		t.Fatal("assistant treated as a generated expert")
	}
	follow := assistantForm(a)
	follow.Set("thread", thread)
	_, next, _ := assistantFixture(t, a, follow, []assistantAction{})
	if len(next.History) != 2 || !strings.Contains(next.History[1].Text, "Observed facts") || !strings.Contains(next.History[1].Text, "Index supplied rules with citations") || !strings.Contains(next.History[1].Text, "not applied by this card") {
		t.Fatal("history lost")
	}
	f = actionForm(t, a, j)
	f.Set("field-goal", "Index literal $(touch SHOULD_NOT_EXIST), with source citations")
	path := "/assist/turns/" + j.ID + "/actions/0"
	build := admittedJob(t, a, serveTest(a, "POST", path, f))
	if build.Kind != "build" {
		t.Fatal("did not dispatch public Hire")
	}
	brief, _ := os.ReadFile(filepath.Join(filepath.Dir(build.Dir), "brief.txt"))
	if !strings.Contains(string(brief), f.Get("field-goal")) {
		t.Fatal("edited intent lost")
	}
	count := len(a.jobs.List())
	// Receipt survives a fresh app instance, independent of its in-memory nonces.
	restarted, err := newApp(a.cfg, a.cat, a.jobs, "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}
	f.Set("csrf", restarted.csrf)
	w := serveTest(restarted, "POST", path, f)
	if w.Header().Get("Location") != "/jobs/"+build.ID || len(a.jobs.List()) != count {
		t.Fatal("proposal applied twice")
	}
	follow = assistantForm(restarted)
	follow.Set("thread", thread)
	_, next, _ = assistantFixture(t, restarted, follow, []assistantAction{})
	if !strings.Contains(next.History[1].Text, "Index literal $(touch SHOULD_NOT_EXIST)") || !strings.Contains(next.History[1].Text, build.ID) {
		t.Fatal("applied edits/outcome missing from follow-up context")
	}

}

func TestAssistantRevisionAndEvidenceSelection(t *testing.T) {
	a, original, run := analysisFixture(t)
	configureAssistant(t, a)
	local := localEntries(a)[0]
	f := assistantForm(a)
	f.Set("focus", "job:"+run.ID)
	actions := []assistantAction{{Kind: "revise_worker", Label: "Teach capture readiness", Target: "worker:local:" + local.ID, Fields: map[string]string{"change": "Wait for actual content; preserve acceptance."}, Roles: []assistantRole{}}, {Kind: "analyze_run", Label: "Inspect the capture", Target: "job:" + run.ID, Fields: map[string]string{"question": "Was content available?"}, Roles: []assistantRole{}}}
	j, req, _ := assistantFixture(t, a, f, actions)
	if len(req.AllowedTargets["revise_worker"]) != 1 || req.Context["run"] == nil {
		t.Fatal("missing selected context")
	}
	path := "/assist/turns/" + j.ID + "/actions/"
	f = actionForm(t, a, j)
	count := len(a.jobs.List())
	w := serveTest(a, "POST", path+"1", f)
	if w.Code != 303 || !strings.HasPrefix(w.Header().Get("Location"), "/jobs/"+run.ID+"/investigate?question=") || len(a.jobs.List()) != count {
		t.Fatal("analysis silently executed")
	}
	before, _ := os.ReadFile(filepath.Join(original.Dir, "expert/README.md"))
	proposal := admittedJob(t, a, serveTest(a, "POST", path+"0", f))
	after, _ := os.ReadFile(filepath.Join(original.Dir, "expert/README.md"))
	if proposal.Kind != "revise" || string(before) != string(after) || len(localEntries(a)) != 1 {
		t.Fatal("revision bypassed review")
	}
	// A second independent proposal cannot modify a worker changed since context capture.
	f = assistantForm(a)
	f.Set("focus", "worker:local:"+local.ID)
	j, _, _ = assistantFixture(t, a, f, actions[:1])
	writeFixture(t, filepath.Join(original.Dir, "expert/README.md"), "changed since discussion", 0600)
	if w := serveTest(a, "POST", "/assist/turns/"+j.ID+"/actions/0", actionForm(t, a, j)); w.Code != 409 {
		t.Fatal("stale baseline accepted")
	}
}

func TestAssistantTeamDraftAndTamper(t *testing.T) {
	a, tf := teamFixture(t)
	configureAssistant(t, a)
	act := assistantAction{Kind: "create_team", Label: "A two-person team", Fields: map[string]string{"title": "Research team", "goal": "Cited report", "handoffs": "Writer supplies report; reviewer supplies findings", "acceptance": "Reject unsupported claims"}, Roles: []assistantRole{{"writer", "worker:source:writer", "Write the report"}, {"reviewer", "worker:" + tf.Get("role-1-worker"), "Review claims"}}}
	j, _, _ := assistantFixture(t, a, assistantForm(a), []assistantAction{act})
	f := actionForm(t, a, j)
	path := "/assist/turns/" + j.ID + "/actions/0"
	f.Set("reply-hash", "old")
	if serveTest(a, "POST", path, f).Code != 409 {
		t.Fatal("stale reply accepted")
	}
	draft := admittedJob(t, a, serveTest(a, "POST", path, actionForm(t, a, j)))
	if draft.Kind != "team-new" {
		t.Fatalf("wrong team operation: %s", draft.Kind)
	}
	for _, job := range a.jobs.List() {
		if job.Kind == "team-build" {
			t.Fatal("silently built team")
		}
	}
	writeFixture(t, filepath.Join(j.Dir, "response.json"), `{"message":"bad"}`, 0600)
	if serveTest(a, "POST", path, f).Code != 409 {
		t.Fatal("tampered response accepted")
	}
}

func TestAssistantReplyRejectsMalformedAndUnauthorizedData(t *testing.T) {
	req := assistantRequest{Version: 1, Catalog: []assistantItem{{Key: "worker:source:writer", Kind: "worker"}}, AllowedTargets: map[string][]string{"open": {"worker:source:writer"}, "create_team": {"worker:source:writer", "worker:source:invented"}}}
	rawReq, _ := json.Marshal(req)
	r := assistantReply{Version: 1, RequestHash: digestText(rawReq), Message: "Useful reply", Question: "", Actions: []assistantAction{createWorkerAction()}}
	valid, _ := json.Marshal(r)
	if _, err := decodeAssistantReply(valid, rawReq); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"null question":  []byte(strings.Replace(string(valid), `"question":""`, `"question":null`, 1)),
		"missing target": []byte(strings.Replace(string(valid), `"target":"",`, "", 1)),
		"null target":    []byte(strings.Replace(string(valid), `"target":""`, `"target":null`, 1)),
		"duplicate":      []byte(strings.Replace(string(valid), `"version":1`, `"version":1,"version":1`, 1)),
		"unknown":        []byte(strings.Replace(string(valid), `"create_worker"`, `"execute_shell"`, 1)),
		"nul":            []byte(strings.Replace(string(valid), `Useful reply`, `bad\u0000text`, 1)),
		"trailing":       append(append([]byte{}, valid...), []byte(` {}`)...),
		"oversized":      []byte(strings.Repeat(" ", 128<<10) + string(valid)),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeAssistantReply(raw, rawReq); err == nil {
				t.Fatal("accepted invalid reply")
			}
		})
	}
	r.RequestHash = "wrong"
	b, _ := json.Marshal(r)
	if _, err := decodeAssistantReply(b, rawReq); err == nil {
		t.Fatal("accepted stale binding")
	}
	for _, target := range []string{"https://evil.invalid", "worker:local:unknown", ""} {
		if validateAssistantAction(req, assistantAction{Kind: "open", Label: "Open", Target: target, Fields: map[string]string{}, Roles: []assistantRole{}}) == nil {
			t.Fatal("unlisted navigation")
		}
	}
	team := assistantAction{Kind: "create_team", Label: "Team", Fields: map[string]string{"title": "t", "goal": "g", "handoffs": "h", "acceptance": "a"}, Roles: []assistantRole{{"writer", "worker:source:writer", "Write"}, {"reviewer", "worker:source:invented", "Review"}}}
	if validateAssistantAction(req, team) == nil {
		t.Fatal("team member not in catalog")
	}
	team.Roles[1] = assistantRole{"state", "worker:source:writer", "Review"}
	if validateAssistantAction(req, team) == nil {
		t.Fatal("reserved role")
	}
}

func TestAssistantUnknownActionOutcomeAndInputGuards(t *testing.T) {
	a := testApp(t)
	configureAssistant(t, a)
	f := assistantForm(a)
	a.cfg.AllowBuild = false
	if serveTest(a, "POST", "/assist", f).Code != 422 || len(a.jobs.List()) != 0 {
		t.Fatal("disabled conversation started a command")
	}
	a.cfg.AllowBuild = true
	f.Set("focus", "worker:local:invented")
	if serveTest(a, "POST", "/assist", f).Code != 409 || len(a.jobs.List()) != 0 {
		t.Fatal("unknown focus admitted")
	}
	f.Del("focus")
	j, _, thread := assistantFixture(t, a, f, []assistantAction{createWorkerAction()})
	writeFixture(t, filepath.Join(filepath.Dir(j.Dir), "action-0.json"), `{"State":"pending"}`, 0600)
	count := len(a.jobs.List())
	if serveTest(a, "POST", "/assist/turns/"+j.ID+"/actions/0", actionForm(t, a, j)).Code != 409 || len(a.jobs.List()) != count {
		t.Fatal("unconfirmed action was repeated")
	}
	body := serveTest(a, "GET", "/assist/"+thread, nil).Body.String()
	if !strings.Contains(body, "outcome is unconfirmed") || strings.Contains(body, ">Build this worker →</button>") {
		t.Fatal("uncertain action still offered")
	}
	writeFixture(t, filepath.Join(filepath.Dir(j.Dir), "snapshot.json"), `{}`, 0600)
	if _, _, err := a.assistantReply(j); err == nil {
		t.Fatal("controller snapshot tamper accepted")
	}
}
