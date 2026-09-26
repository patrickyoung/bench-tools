package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// A selected Serve origin is additional to loopback access, not a wildcard host
// or permission to trust forwarded headers from arbitrary network peers.
func tailscaleHost(cfg config) (string, error) {
	if cfg.TailscaleOrigin == "" && cfg.TailscaleUser == "" {
		return "", nil
	}
	u, err := url.Parse(cfg.TailscaleOrigin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || !strings.HasSuffix(u.Hostname(), ".ts.net") || cfg.TailscaleUser == "" || strings.ContainsAny(cfg.TailscaleUser, "\r\n\t ") {
		return "", fmt.Errorf("select an exact HTTPS Tailscale origin (without a path) and one Tailscale login")
	}
	return u.Host, nil
}

func (a *app) requestDenial(r *http.Request) string {
	host, _ := tailscaleHost(a.cfg)
	if host == "" || r.Host != host {
		if a.hosts[r.Host] {
			return ""
		}
		return "address"
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(peer)
	if err != nil || ip == nil || !ip.IsLoopback() {
		return "proxy"
	}
	// Tailscale Serve strips caller-supplied identity headers and injects its
	// authenticated identity. Only its loopback proxy is trusted here. Local
	// processes already have access through the ordinary loopback interface.
	identities := r.Header.Values("Tailscale-User-Login")
	if len(identities) == 0 || (len(identities) == 1 && identities[0] == "") {
		return "identity-missing"
	}
	if len(identities) != 1 || identities[0] != a.cfg.TailscaleUser {
		return "identity-mismatch"
	}
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
		if origin := r.Header.Get("Origin"); origin != "" && origin != a.cfg.TailscaleOrigin {
			return "origin"
		}
	}
	return ""
}
