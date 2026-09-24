package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// TestBrowser exercises the public executable, not an in-process substitute.
// It is deliberately opt-in: no browser download, public site, or model call.
func TestBrowser(t *testing.T) {
	path := os.Getenv("WEB_TEST_BROWSER")
	if path == "" {
		t.Skip("set WEB_TEST_BROWSER to run actual Chromium contract checks")
	}
	t.Setenv("WEB_BROWSER", path)
	dir := t.TempDir()
	bin := filepath.Join(dir, "web")
	build := exec.Command("go", "build", "-o", bin, ".")
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %s %v", b, e)
	}
	var acts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fingerprint":
			fmt.Fprint(w, `<!doctype html><body><script>document.body.innerText=JSON.stringify({webdriver:navigator.webdriver,languages:navigator.languages});</script></body>`)
		case "/native-set":
			fmt.Fprint(w, `<!doctype html><body>native<script>document.cookie='native=yes; Max-Age=86400; path=/';localStorage.setItem('native','retained');</script></body>`)
		case "/native-get":
			fmt.Fprintf(w, `<!doctype html><body>%s<script>document.body.innerText += ':'+localStorage.getItem('native')</script></body>`, r.Header.Get("Cookie"))
		case "/redirect":
			http.Redirect(w, r, "/page", 302)
		case "/huge":
			fmt.Fprint(w, `<body><script>document.body.innerText="x".repeat(9*1024*1024)</script></body>`)
		case "/act":
			acts.Add(1)
			fmt.Fprint(w, "ok")
		case "/change":
			fmt.Fprint(w, `<script>localStorage.setItem("saved","changed")</script>`)
		case "/cookie":
			fmt.Fprintf(w, "<body>%s<script>document.body.innerText += ':'+localStorage.getItem('saved')</script></body>", r.Header.Get("Cookie"))
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><title>Fixture</title><base href="/base/"><nav><a href="nav">Navigation</a></nav><h1>Rendered</h1><div id="rows"><article class="row"><a href="one">First</a></article><article class="row"><a href="two">Second</a></article><article class="row" style="display:none"><a href="hidden">Hidden</a></article></div><input id="name" value="old"><input id="password" type="password"><select id="pick"><option value="a">Alpha</option><option value="b">Beta</option></select><button id="act" onclick="fetch('/act')">Act</button><p id="result"></p><script>document.querySelector('#result').innerText='JavaScript ran';document.cookie='fixture=yes; path=/';localStorage.setItem('saved','retained');</script>`)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, e := startSession(ctx, options{}, false, io.Discard)
	if e != nil {
		t.Fatal(e)
	}
	defer s.close(false, io.Discard)
	b, e := os.ReadFile(filepath.Join(s.dir, "DevToolsActivePort"))
	if e != nil {
		t.Fatal(e)
	}
	port := strings.Split(string(b), "\n")[0]
	if e = navigate(s.page.Timeout(5*time.Second), server.URL+"/page", "domcontentloaded"); e != nil {
		t.Fatal(e)
	}
	count := func() int {
		t.Helper()
		v, e := (proto.TargetGetTargets{}).Call(s.browser.Timeout(2 * time.Second))
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		for _, v := range v.TargetInfos {
			if v.Type == "page" {
				n++
			}
		}
		return n
	}
	baseline := count()
	mayDir := filepath.Join(dir, "commands")
	os.Mkdir(mayDir, 0700)
	invoke := func(t *testing.T, plan string, args ...string) (int, string, string) {
		t.Helper()
		run, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		c := exec.CommandContext(run, bin, args...)
		c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		c.Env = append(os.Environ(), "WEB_STATE="+filepath.Join(dir, "audit.jsonl"), "PATH="+mayDir, "CLERK_WORKER=test", "JOB_ID=test", "CLERK_WROOT="+dir)
		c.Stdin = strings.NewReader(plan)
		var out, stderr bytes.Buffer
		c.Stdout = &out
		c.Stderr = &stderr
		e := c.Run()
		code := 0
		if e != nil {
			if x, ok := e.(*exec.ExitError); ok {
				code = x.ExitCode()
			} else {
				t.Fatalf("execute: %v\n%s", e, stderr.String())
			}
		}
		return code, out.String(), stderr.String()
	}
	assert := func(t *testing.T, plan string, args ...string) string {
		t.Helper()
		code, out, stderr := invoke(t, plan, args...)
		if code != 0 {
			t.Fatalf("%v exit %d\nstdout: %s\nstderr: %s", args, code, out, stderr)
		}
		return out
	}
	t.Run("snapshot-and-render", func(t *testing.T) {
		out := assert(t, "", "snapshot", server.URL+"/redirect", "--attach", port, "--records-selector", ".row")
		var v struct {
			URL       string `json:"url"`
			Requested string `json:"requested_url"`
			Records   []struct {
				Text  string `json:"text"`
				Links []link `json:"links"`
			} `json:"records"`
			Links []link `json:"links"`
		}
		if e = json.Unmarshal([]byte(out), &v); e != nil {
			t.Fatal(e)
		}
		if v.URL != server.URL+"/page" || v.Requested != server.URL+"/redirect" || len(v.Records) != 2 || len(v.Links) != 3 || v.Records[0].Links[0].URL != server.URL+"/base/one" {
			t.Fatalf("snapshot: %s", out)
		}
		md := assert(t, "", "get", server.URL+"/page", "--attach", port)
		if !strings.Contains(md, "JavaScript ran") || strings.Contains(md, "Navigation") || !strings.Contains(md, "/base/one") {
			t.Fatal(md)
		}
		links := assert(t, "", "links", server.URL+"/page", "--attach", port)
		if strings.Contains(links, "Navigation") || !strings.Contains(links, "First\t") {
			t.Fatal(links)
		}
		assert(t, "", "text", server.URL, "--attach", port, "--wait", "load")
		assert(t, "", "html", server.URL, "--attach", port, "--wait", "networkidle")
		shot := filepath.Join(dir, "shot.png")
		if got := assert(t, "", "shot", server.URL, shot, "--attach", port); got != shot+"\n" {
			t.Fatal("shot did not return its filename", got)
		}
		data, _ := os.ReadFile(shot)
		if !bytes.HasPrefix(data, []byte("\x89PNG")) {
			t.Fatal("not PNG")
		}
		code, overflow, _ := invoke(t, "", "snapshot", server.URL+"/huge", "--attach", port)
		if code != 1 || overflow != "" {
			t.Fatal("oversized snapshot emitted partial data", code, len(overflow))
		}
		code, invalid, _ := invoke(t, "", "snapshot", server.URL, "--attach", port, "--records-selector", "[")
		if code != 1 || invalid != "" {
			t.Fatal("invalid selector emitted partial data", code, invalid)
		}
		if count() != baseline {
			t.Fatal("read leaked/closed tabs")
		}
	})
	t.Run("plans-gates-and-retention", func(t *testing.T) {
		plan := fmt.Sprintf(`[{"goto":%q},{"type":["#name","new"]},{"select":["#pick","b"]},{"wait":20},{"html":"#name"},{"read":"#result"}]`, server.URL)
		assert(t, plan, "run", "--attach", port)
		assert(t, fmt.Sprintf(`[{"goto":%q},{"goto":%q},{"read":"#result"}]`, server.URL, server.URL+"#fragment"), "run", "--attach", port)
		if count() != baseline {
			t.Fatal("plan tab leaked")
		}
		click := fmt.Sprintf(`[{"goto":%q},{"click":"#act","irreversible":false},{"wait":200}]`, server.URL)
		for _, code := range []int{75, 3, 77, 0} {
			fakeMay(t, mayDir, code)
			actual, out, stderr := invoke(t, click, "run", "--attach", port, "--keep")
			if actual != code {
				t.Fatalf("gate %d got %d: %s %s", code, actual, out, stderr)
			}
			if !json.Valid([]byte(out)) {
				t.Fatal("gate polluted JSON stdout")
			}
			if code != 0 {
				if count() != baseline || acts.Load() != 0 {
					t.Fatal("refused gate acted or leaked")
				}
			} else {
				if acts.Load() != 1 || count() != baseline+1 {
					t.Fatal("successful keep failed")
				}
				var id string
				for _, line := range strings.Split(stderr, "\n") {
					if strings.HasPrefix(line, "web: tab ") {
						id = strings.Fields(line)[2]
					}
				}
				if id == "" {
					t.Fatal("no stable tab ID")
				}
				assert(t, `[{"read":"#result"}]`, "run", "--attach", port, "--tab", id)
				actual, _, _ = invoke(t, `[{"wait":"#missing","timeout":100}]`, "run", "--attach", port, "--tab", id)
				if actual != 1 || count() != baseline+1 {
					t.Fatal("failed named tab was closed")
				}
				_, e = (proto.TargetCloseTarget{TargetID: proto.TargetTargetID(id)}).Call(s.browser)
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		code, out, _ := invoke(t, fmt.Sprintf(`[{"goto":%q},{"type":["#password","never"]}]`, server.URL), "run", "--attach", port)
		if code != 1 || !strings.Contains(out, "never types") {
			t.Fatal("password not refused", out)
		}
		code, _, _ = invoke(t, `[{"wait":true}]`, "run", "--attach", port, "--keep")
		if code != 1 || count() != baseline {
			t.Fatal("failed kept plan leaked")
		}
		// Stdin is plan data, never permission; no job and no controlling tty.
		c := exec.Command(bin, "run", "--attach", port)
		c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		c.Env = []string{"PATH=" + mayDir, "WEB_STATE=" + filepath.Join(dir, "audit.jsonl")}
		c.Stdin = strings.NewReader(click)
		terminalOut, e := c.Output()
		var exit *exec.ExitError
		if !errors.As(e, &exit) || exit.ExitCode() != 77 || !json.Valid(terminalOut) || acts.Load() != 1 {
			t.Fatalf("noninteractive act was not refused: %v %s", e, terminalOut)
		}
	})
	t.Run("stealth-is-opt-in-and-scoped", func(t *testing.T) {
		baseline := assert(t, "", "text", server.URL+"/fingerprint")
		if !strings.Contains(baseline, `"webdriver":true`) {
			t.Fatal("fresh control did not expose automation", baseline)
		}
		for _, args := range [][]string{{"text", server.URL + "/fingerprint", "--stealth"}, {"text", server.URL + "/fingerprint", "--attach", port, "--stealth"}} {
			got := assert(t, "", args...)
			if strings.Contains(got, `"webdriver":true`) {
				t.Fatal("stealth was not injected before the page script", got)
			}
		}
		got := assert(t, fmt.Sprintf(`[{"goto":%q},{"read":"body"}]`, server.URL+"/fingerprint"), "run", "--stealth", "--attach", port)
		if strings.Contains(got, `\"webdriver\":true`) {
			t.Fatal("plan did not receive stealth", got)
		}
		value, e := pageString(s.page, `()=>String(navigator.webdriver)`)
		if e != nil || value != "true" {
			t.Fatal("stealth changed an unrelated tab", value, e)
		}
		fakeMay(t, mayDir, 75)
		code, _, _ := invoke(t, fmt.Sprintf(`[{"goto":%q},{"click":"#act"}]`, server.URL), "run", "--attach", port, "--stealth")
		if code != 75 || acts.Load() != 1 {
			t.Fatal("stealth changed approval semantics", code, acts.Load())
		}
	})
	t.Run("native-profile-reuse-and-ownership", func(t *testing.T) {
		profile := filepath.Join(dir, "native profile")
		fakeMay(t, mayDir, 75)
		destination := filepath.Join(dir, "native-export.json")
		code, _, _ := invoke(t, "\n", "auth", server.URL, destination, "--user-data-dir", profile, "--stealth")
		if code != 75 {
			t.Fatal("native export was not gated", code)
		}
		if _, e := os.Stat(profile); !os.IsNotExist(e) {
			t.Fatal("refused export touched native profile", e)
		}
		if _, e := os.Stat(destination); !os.IsNotExist(e) {
			t.Fatal("refused export created credential file", e)
		}
		assert(t, "", "text", server.URL+"/native-set", "--user-data-dir", profile)
		if _, e := os.Stat(profile); e != nil {
			t.Fatal("profile was deleted", e)
		}
		got := assert(t, "", "text", server.URL+"/native-get", "--user-data-dir", profile, "--stealth")
		if !strings.Contains(got, "native=yes:retained") {
			t.Fatal("native session was not reused", got)
		}
		other := assert(t, "", "text", server.URL+"/native-get", "--user-data-dir", profile, "--profile-directory", "Profile 1")
		if strings.Contains(other, "native=yes") || strings.Contains(other, "retained") {
			t.Fatal("explicit native profile was ignored", other)
		}
		fresh := assert(t, "", "text", server.URL+"/native-get")
		if strings.Contains(fresh, "native=yes") || strings.Contains(fresh, "retained") {
			t.Fatal("native identity leaked into fresh mode", fresh)
		}
		held, e := startSession(ctx, options{userDataDir: profile}, false, io.Discard)
		if e != nil {
			t.Fatal(e)
		}
		code, _, stderr := invoke(t, "", "text", server.URL, "--user-data-dir", profile)
		if code != 1 || !strings.Contains(stderr, "already in use") {
			held.close(false, io.Discard)
			t.Fatal("simultaneous profile use was accepted", code, stderr)
		}
		// A stale native port file must never connect to and then close a
		// browser Web did not launch for this invocation.
		held.close(false, io.Discard)
		ownPort, _ := os.ReadFile(filepath.Join(s.dir, "DevToolsActivePort"))
		if e = os.WriteFile(filepath.Join(profile, "DevToolsActivePort"), ownPort, 0600); e != nil {
			t.Fatal(e)
		}
		assert(t, "", "text", server.URL+"/native-get", "--user-data-dir", profile)
		if _, e := (proto.BrowserGetVersion{}).Call(s.browser); e != nil {
			t.Fatal("stale native port selected/closed the other browser", e)
		}
		code, _, _ = invoke(t, fmt.Sprintf(`[{"goto":%q,"profile":"missing.json"}]`, server.URL), "run", "--user-data-dir", profile)
		if code != 2 {
			t.Fatal("native and plan storage state were mixed", code)
		}
	})
	t.Run("profile-export-and-replay", func(t *testing.T) {
		profile := filepath.Join(dir, "profile.json")
		fakeMay(t, mayDir, 75)
		code, _, _ := invoke(t, "", "auth", server.URL, profile, "--attach", port)
		if code != 75 {
			t.Fatal(code)
		}
		if _, e := os.Stat(profile); !os.IsNotExist(e) {
			t.Fatal("refused auth wrote credentials")
		}
		fakeMay(t, mayDir, 0)
		assert(t, "", "auth", server.URL, profile, "--attach", port)
		if count() != baseline {
			t.Fatal("auth changed tabs")
		}
		st, e := os.Stat(profile)
		if e != nil || st.Mode().Perm() != 0600 {
			t.Fatal(st, e)
		}
		got := assert(t, "", "text", server.URL+"/cookie", "--profile", profile)
		if !strings.Contains(got, "fixture=yes:retained") {
			t.Fatal("profile not replayed", got)
		}
		plan := fmt.Sprintf(`[{"goto":%q,"profile":%q},{"goto":%q},{"read":"body"}]`, server.URL+"/change", profile, server.URL+"/cookie")
		changed := assert(t, plan, "run")
		if !strings.Contains(changed, "fixture=yes:changed") {
			t.Fatal("profile overwrote changed session on navigation", changed)
		}
		got = assert(t, "", "text", server.URL+"/cookie")
		if strings.Contains(got, "fixture=yes") || strings.Contains(got, "retained") {
			t.Fatal("fresh read inherited login", got)
		}
	})
	t.Run("compose-with-weigh", func(t *testing.T) {
		weigh := os.Getenv("WEB_TEST_WEIGH")
		if weigh == "" {
			t.Skip("set WEB_TEST_WEIGH to the independent Weigh executable")
		}
		snapshot := assert(t, "", "snapshot", server.URL, "--attach", port, "--records-selector", ".row")
		var state any
		if e := json.Unmarshal([]byte(snapshot), &state); e != nil {
			t.Fatal(e)
		}
		calls := make(chan map[string]any, 1)
		provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
				t.Error(e)
			}
			calls <- body
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"model":"fixture/offline","answers":{"listed":{"type":"noul","noul":0.8}}}`)
		}))
		defer provider.Close()
		request := map[string]any{"version": 1, "state": state, "questions": map[string]any{"listed": map[string]any{"type": "probability", "question": "Does the observed page list a record?"}}}
		raw, _ := json.Marshal(request)
		command := exec.Command(weigh, "-m", "openrouter/fixture/offline", "-endpoint", provider.URL)
		command.Env = append(os.Environ(), "OPENROUTER_API_KEY=offline-test-placeholder")
		command.Stdin = bytes.NewReader(raw)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		reply, e := command.Output()
		if e != nil {
			t.Fatalf("Weigh: %v %s", e, stderr.String())
		}
		var result struct {
			Answers map[string]struct {
				Value float64 `json:"value"`
			} `json:"answers"`
		}
		if e = json.Unmarshal(reply, &result); e != nil || result.Answers["listed"].Value != 0.8 {
			t.Fatalf("Weigh output: %s %v", reply, e)
		}
		select {
		case received := <-calls:
			got, _ := json.Marshal(received["state"])
			want, _ := json.Marshal(state)
			if !bytes.Equal(got, want) {
				t.Fatal("observation changed across executable composition")
			}
		default:
			t.Fatal("no local inference request")
		}
	})
	t.Run("signal-cleans-owned-tab", func(t *testing.T) {
		c := exec.Command(bin, "run", "--attach", port)
		c.Env = append(os.Environ(), "WEB_STATE="+filepath.Join(dir, "audit.jsonl"))
		c.Stdin = strings.NewReader(`[{"wait":60000}]`)
		var output bytes.Buffer
		c.Stderr = &output
		if e = c.Start(); e != nil {
			t.Fatal(e)
		}
		until := time.Now().Add(5 * time.Second)
		for count() == baseline && time.Now().Before(until) {
			time.Sleep(20 * time.Millisecond)
		}
		if count() != baseline+1 {
			c.Process.Kill()
			c.Wait()
			t.Fatal("no plan tab", output.String())
		}
		c.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- c.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			c.Process.Kill()
			<-done
			t.Fatal("signal cleanup hung")
		}
		if count() != baseline {
			t.Fatal("signal leaked tab")
		}
	})
	got, e := pageString(s.page, `() => document.querySelector('#result').innerText`)
	if e != nil || got != "JavaScript ran" {
		t.Fatal("unrelated tab was changed", got, e)
	}
}
