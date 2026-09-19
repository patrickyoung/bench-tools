package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultEndpoint = "https://openrouter.ai/api/alpha/decisions"

func validateEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Opaque != "" {
		return errors.New("-endpoint must be a full URL without credentials, query, or fragment")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" {
		if ip := net.ParseIP(u.Hostname()); ip != nil && ip.IsLoopback() {
			return nil
		}
	}
	return errors.New("-endpoint requires HTTPS; HTTP is permitted only for literal loopback IPs")
}

func headerValue(s string) bool {
	if s == "" {
		return false
	}
	for _, b := range []byte(s) {
		if b < 32 || b > 126 {
			return false
		}
	}
	return true
}

func parseAuthorization(raw []byte) (string, error) {
	line := string(raw)
	if strings.HasSuffix(line, "\n") {
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
	}
	if strings.ContainsAny(line, "\r\n") {
		return "", errors.New("invalid authorization header")
	}
	name, value, ok := strings.Cut(line, ":")
	value = strings.Trim(value, " ")
	if !ok || !strings.EqualFold(name, "Authorization") || !headerValue(value) {
		return "", errors.New("invalid authorization header")
	}
	return value, nil
}

func infer(ctx context.Context, endpoint, model, authorization string, input request) ([]byte, error) {
	body, err := input.native(strings.TrimPrefix(model, "openrouter/"))
	if err != nil {
		return nil, errors.New("could not encode provider request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("could not create provider request")
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "weigh/"+version)
	// One fresh HTTP/1 transport, one POST, no idempotency key or redirects.
	// Avoid a reused-connection retry and leave all retry policy with callers.
	transport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:    10 * time.Second,
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: 64 << 10,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("provider request interrupted or timed out")
		}
		return nil, errors.New("provider transport failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// Error bodies can echo private state or credentials. Never print them.
		return nil, fmt.Errorf("provider returned HTTP %d", response.StatusCode)
	}
	raw, err := readContext(ctx, response.Body, maxBytes)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("provider response interrupted or timed out")
		}
		return nil, errors.New("could not read response within the 8 MiB limit")
	}
	result, err := parseResponse(raw, model, input)
	if err != nil {
		return nil, fmt.Errorf("invalid provider response: %s", err)
	}
	return result, nil
}
