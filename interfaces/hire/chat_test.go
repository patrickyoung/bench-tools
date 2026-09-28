package main

import (
	"strings"
	"testing"
)

func TestChatShellAndPerConversationSite(t *testing.T) {
	a := taskFixture(t)
	first := taskSubmit(t, a, "", "First chat")
	second := taskSubmit(t, a, "", "Second chat")
	follow := taskSubmit(t, a, first.Record.Thread, "Refine first chat")
	page := serveTest(a, "GET", "/work/"+first.Record.Thread+"?site=1", nil)
	if page.Code != 200 {
		t.Fatal(page.Body.String())
	}
	html := page.Body.String()
	for _, want := range []string{`class="chat-app"`, `aria-label="Chats"`, `/work/` + second.Record.Thread, `aria-current="page"`, `class="chat-transcript"`, `class="conversation-composer work-composer"`, `class="chat-workspace site-open"`, `Plonk back`, `/work/jobs/` + follow.Job.ID + `/files/`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	for _, removed := range []string{`class="work-heading"`, `class="work-starters"`, `class="work-rail"`, `class="work-chat-invite"`, `class="topline"`} {
		if strings.Contains(html, removed) {
			t.Fatalf("retained unnecessary UI %s", removed)
		}
	}
	panel := html[strings.Index(html, `<aside id="plonk-panel"`):]
	if strings.Contains(panel, `/work/jobs/`+second.Job.ID+`/files/`) || strings.Contains(panel, `/work/jobs/`+first.Job.ID+`/files/`) {
		t.Fatal("site mixed another chat or an earlier output version")
	}
	empty := serveTest(a, "GET", "/", nil).Body.String()
	if !strings.Contains(empty, `This chat’s work will appear here.`) || !strings.Contains(empty, `aria-expanded="false">Plonk</a>`) {
		t.Fatal("new chat lacks its site panel")
	}
}
