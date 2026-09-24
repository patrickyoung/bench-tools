package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

type storedCookie struct {
	PartitionKey json.RawMessage `json:"partitionKey,omitempty"`
	Name         string          `json:"name"`
	Value        string          `json:"value"`
	Domain       string          `json:"domain"`
	Path         string          `json:"path"`
	Expires      float64         `json:"expires"`
	HTTPOnly     bool            `json:"httpOnly"`
	Secure       bool            `json:"secure"`
	SameSite     string          `json:"sameSite"`
}
type storageItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type storedOrigin struct {
	Origin       string        `json:"origin"`
	LocalStorage []storageItem `json:"localStorage"`
}
type storageState struct {
	Cookies []storedCookie `json:"cookies"`
	Origins []storedOrigin `json:"origins"`
}

func loadState(path string) (*storageState, error) {
	raw, e := readFile(path)
	if e != nil {
		return nil, e
	}
	var state storageState
	if e = json.Unmarshal(raw, &state); e != nil {
		return nil, bad("invalid storage state: %v", e)
	}
	if state.Cookies == nil || state.Origins == nil {
		return nil, bad("storage state needs cookies and origins arrays")
	}
	for _, c := range state.Cookies {
		if len(c.PartitionKey) > 0 && string(c.PartitionKey) != "null" {
			return nil, bad("partitioned cookies are not supported by this storage-state port; use --attach")
		}
	}
	return &state, nil
}
func applyState(s *session, state *storageState) error {
	ctx, cancel := context.WithTimeout(s.page.GetContext(), 10*time.Second)
	defer cancel()
	p := s.page.Context(ctx)
	cookies := []*proto.NetworkCookieParam{}
	for _, c := range state.Cookies {
		v := &proto.NetworkCookieParam{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, HTTPOnly: c.HTTPOnly, Secure: c.Secure, SameSite: proto.NetworkCookieSameSite(c.SameSite)}
		if c.Expires > 0 {
			v.Expires = proto.TimeSinceEpoch(c.Expires)
		}
		cookies = append(cookies, v)
	}
	if len(cookies) > 0 {
		if e := p.SetCookies(cookies); e != nil {
			return e
		}
	}
	if len(state.Origins) == 0 {
		return nil
	}
	// Restore each origin once using an intercepted empty document. No request
	// reaches a site, and later navigation does not overwrite session changes.
	if e := (proto.FetchEnable{Patterns: []*proto.FetchRequestPattern{{URLPattern: "*"}}}).Call(p); e != nil {
		return e
	}
	defer func() { _ = (proto.FetchDisable{}).Call(p) }()
	listener, stop := context.WithCancel(ctx)
	defer stop()
	wait := p.Context(listener).EachEvent(func(event *proto.FetchRequestPaused) {
		_ = (proto.FetchFulfillRequest{RequestID: event.RequestID, ResponseCode: 200,
			ResponseHeaders: []*proto.FetchHeaderEntry{{Name: "Content-Type", Value: "text/html"}}, Body: []byte("<!doctype html><html><body></body></html>")}).Call(p)
	})
	done := make(chan struct{})
	go func() { defer close(done); wait() }()
	defer func() { stop(); <-done }()
	for _, origin := range state.Origins {
		u, e := url.Parse(origin.Origin)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return bad("storage origin must be an HTTP(S) origin")
		}
		if e = navigate(p, origin.Origin, "domcontentloaded"); e != nil {
			return e
		}
		if _, e = p.Eval(`(items) => {for(const item of items) localStorage.setItem(item.name,item.value)}`, origin.LocalStorage); e != nil {
			return e
		}
	}
	return nil
}
func captureState(s *session) (*storageState, error) {
	ctx, cancel := context.WithTimeout(s.browser.GetContext(), 30*time.Second)
	defer cancel()
	b := s.browser.Context(ctx)
	cookies, e := b.GetCookies()
	if e != nil {
		return nil, e
	}
	state := &storageState{Cookies: []storedCookie{}, Origins: []storedOrigin{}}
	for _, c := range cookies {
		if c.PartitionKey != nil || c.PartitionKeyOpaque {
			return nil, fmt.Errorf("cannot export partitioned cookies without losing scope; use --attach")
		}
		expires := float64(c.Expires)
		if c.Session {
			expires = -1
		}
		same := string(c.SameSite)
		if same == "" {
			same = "Lax"
		}
		state.Cookies = append(state.Cookies, storedCookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: expires, HTTPOnly: c.HTTPOnly, Secure: c.Secure, SameSite: same})
	}
	// Only auth calls this, after its explicit export gate. No ordinary read
	// enumerates or evaluates another tab. Closed origins are not represented.
	own, e := (proto.TargetGetTargetInfo{TargetID: s.page.TargetID}).Call(b)
	if e != nil {
		return nil, e
	}
	targets, e := (proto.TargetGetTargets{}).Call(b)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	for _, target := range targets.TargetInfos {
		if target.Type != "page" || target.BrowserContextID != own.TargetInfo.BrowserContextID || (!strings.HasPrefix(target.URL, "http://") && !strings.HasPrefix(target.URL, "https://")) {
			continue
		}
		p, e := b.PageFromTarget(target.TargetID)
		if e != nil {
			return nil, e
		}
		raw, e := pageValue(p, `() => ({origin:location.origin,localStorage:Object.keys(localStorage).map(name=>({name,value:localStorage.getItem(name)}))})`)
		if e != nil {
			return nil, e
		}
		var origin storedOrigin
		if e = json.Unmarshal(raw, &origin); e != nil {
			return nil, e
		}
		if !seen[origin.Origin] {
			state.Origins = append(state.Origins, origin)
			seen[origin.Origin] = true
		}
	}
	return state, nil
}
func saveState(path string, state *storageState) (int, error) {
	data, e := json.MarshalIndent(state, "", "  ")
	if e != nil {
		return 0, e
	}
	data = append(data, '\n')
	if len(data) > maxInput {
		return 0, fmt.Errorf("storage state exceeds 8 MiB")
	}
	dir := filepath.Dir(path)
	if e = os.MkdirAll(dir, 0700); e != nil {
		return 0, e
	}
	f, e := os.CreateTemp(dir, ".web-profile-*")
	if e != nil {
		return 0, e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(data); e != nil {
		f.Close()
		return 0, e
	}
	if e = f.Close(); e != nil {
		return 0, e
	}
	if e = os.Rename(f.Name(), path); e != nil {
		return 0, e
	}
	return len(data), nil
}
func authenticate(ctx context.Context, o options, in io.Reader, stderr io.Writer) (err error) {
	start := time.Now()
	size := 0
	defer func() {
		outcome := "ok"
		if err != nil {
			outcome = "fail"
		}
		audit("auth", o.args[0], size, start, outcome, mode(o), "", "")
	}()
	if o.attach != "" || o.userDataDir != "" && len(o.args) == 2 {
		source := "the browser at " + o.attach
		if o.userDataDir != "" {
			profile := o.profileDirectory
			if profile == "" {
				profile = "Default"
			}
			source = "native browser profile " + profile + " in " + o.userDataDir
		}
		if e := gate(ctx, "copy the whole logged-in state of "+source+" into "+o.args[1], "auth", o.args[0], mode(o), o.job, stderr); e != nil {
			return e
		}
	}
	s, e := startSession(ctx, o, true, stderr)
	if e != nil {
		return e
	}
	defer s.close(false, stderr)
	if o.attach == "" {
		run, cancel := context.WithTimeout(ctx, 30*time.Second)
		e = navigate(s.page.Context(run), o.args[0], "domcontentloaded")
		cancel()
		if e != nil {
			fmt.Fprintln(stderr, "web: could not preload login page:", e)
		}
		if o.userDataDir != "" {
			fmt.Fprintln(stderr, "web: log in BY HAND, then press Enter here to finish. The selected native profile persists browser changes as they happen.")
		} else {
			fmt.Fprintln(stderr, "web: log in BY HAND in the browser, then press Enter here to save the session.")
		}
		done := make(chan error, 1)
		go func() { _, e := bufio.NewReader(in).ReadString('\n'); done <- e }()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e = <-done:
			if e != nil {
				return fmt.Errorf("login was not confirmed: %w", e)
			}
		}
	}
	if len(o.args) == 1 {
		fmt.Fprintln(stderr, "web: session retained in selected native browser profile", o.userDataDir)
		return nil
	}
	state, e := captureState(s)
	if e != nil {
		return e
	}
	size, e = saveState(o.args[1], state)
	if e != nil {
		return e
	}
	fmt.Fprintln(stderr, "web: saved session to", o.args[1], "(mode 600; treat as a secret)")
	return nil
}
