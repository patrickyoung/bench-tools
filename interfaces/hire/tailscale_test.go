package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTailscaleBoundary(t *testing.T) {
	a := testApp(t)
	a.cfg.TailscaleOrigin = "https://work.example.ts.net:10443"
	a.cfg.TailscaleUser = "owner@example.com"
	for _, tc := range []struct {
		name, method, host, peer, identity, origin, fetch, token string
		want                                                     int
	}{
		{"owner page", "GET", "work.example.ts.net:10443", "127.0.0.1:5000", "owner@example.com", "", "", "", 200},
		{"IPv6 proxy", "GET", "work.example.ts.net:10443", "[::1]:5000", "owner@example.com", "", "", "", 200},
		{"no identity", "GET", "work.example.ts.net:10443", "127.0.0.1:5000", "", "", "", "", 403},
		{"other user", "GET", "work.example.ts.net:10443", "127.0.0.1:5000", "other@example.com", "", "", "", 403},
		{"remote forged identity", "GET", "work.example.ts.net:10443", "100.64.0.2:5000", "owner@example.com", "", "", "", 403},
		{"unselected address", "GET", "wrong.example.ts.net:10443", "127.0.0.1:5000", "owner@example.com", "", "", "", 403},
		{"cross origin", "POST", "work.example.ts.net:10443", "127.0.0.1:5000", "owner@example.com", "https://evil.example", "same-origin", a.csrf, 403},
		{"opaque origin", "POST", "work.example.ts.net:10443", "127.0.0.1:5000", "owner@example.com", "null", "same-origin", a.csrf, 403},
		{"wrong scheme", "POST", "work.example.ts.net:10443", "127.0.0.1:5000", "owner@example.com", "http://work.example.ts.net:10443", "", a.csrf, 403},
		{"cross site metadata", "POST", "work.example.ts.net:10443", "127.0.0.1:5000", "owner@example.com", a.cfg.TailscaleOrigin, "cross-site", a.csrf, 403},
		{"missing form token", "POST", "work.example.ts.net:10443", "127.0.0.1:5000", "owner@example.com", a.cfg.TailscaleOrigin, "same-origin", "", 403},
		// An absent route proves a valid form reaches the mux without starting work.
		{"owner form", "POST", "work.example.ts.net:10443", "127.0.0.1:5000", "owner@example.com", a.cfg.TailscaleOrigin, "same-origin", a.csrf, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/"
			if tc.method == "POST" {
				path = "/no-operation"
			}
			r := httptest.NewRequest(tc.method, "https://"+tc.host+path, strings.NewReader("csrf="+tc.token))
			r.RemoteAddr = tc.peer
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Tailscale-User-Login", tc.identity)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.fetch)
			w := httptest.NewRecorder()
			a.handler().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
	if w := serveTest(a, "GET", "/", nil); w.Code != 200 {
		t.Fatal("local access changed")
	}
	if len(a.jobs.List()) != 0 {
		t.Fatal("boundary checks started work")
	}
}

func TestTailscaleConfigRequiresExplicitOriginAndUser(t *testing.T) {
	for _, origin := range []string{"", "http://work.example.ts.net", "https://work.example.ts.net/", "https://work.example.ts.net/path", "https://user@work.example.ts.net", "https://work.example.ts.net?", "https://evil.example", "https://work.example.ts.net#x"} {
		if _, err := tailscaleHost(config{TailscaleOrigin: origin, TailscaleUser: "owner@example.com"}); err == nil {
			t.Fatalf("accepted %q", origin)
		}
	}
	if _, err := tailscaleHost(config{TailscaleOrigin: "https://work.example.ts.net"}); err == nil {
		t.Fatal("accepted missing identity")
	}
	if _, err := tailscaleHost(config{}); err != nil {
		t.Fatal(err)
	}
}
