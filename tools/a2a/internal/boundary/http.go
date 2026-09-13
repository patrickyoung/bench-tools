// Package boundary supplies bounded, explicit network and file boundaries.
// It contains no protocol implementation, credential acquisition, or retries.
package boundary

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
)

type Network struct {
	CA, Cert, Key string
	LoopbackHTTP  bool
	Headers       http.Header
	MaxOutput     int64
}

func Endpoint(raw string, loopbackHTTP bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("endpoint requires an absolute URL without userinfo, query, or fragment")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && loopbackHTTP && IsLoopback(u.Hostname())) {
		return nil, errors.New("HTTPS is required; plain HTTP needs -http-loopback and a literal loopback address")
	}
	return u, nil
}

func IsLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func PrivateFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	i, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 {
		return nil, errors.New("credential file must be regular and mode 0600 or stricter")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = errors.New("credential file exceeds limit")
	}
	return b, err
}

func TLSConfig(ca, cert, key string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if ca != "" {
		pem, err := os.ReadFile(ca)
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = x509.NewCertPool()
		if !cfg.RootCAs.AppendCertsFromPEM(pem) {
			return nil, errors.New("CA file contains no certificates")
		}
	}
	if (cert == "") != (key == "") {
		return nil, errors.New("certificate and private key must be supplied together")
	}
	if cert != "" {
		pem, err := os.ReadFile(cert)
		if err != nil {
			return nil, err
		}
		secret, err := PrivateFile(key, 1<<20)
		if err != nil {
			return nil, err
		}
		pair, err := tls.X509KeyPair(pem, secret)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}

func ReadHeaders(r io.Reader) (http.Header, error) {
	b, err := io.ReadAll(io.LimitReader(r, (64<<10)+1))
	if err != nil || len(b) > 64<<10 {
		return nil, errors.New("cannot read bounded credential headers")
	}
	h := http.Header{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || name == "" {
			return nil, errors.New("invalid credential header")
		}
		for _, ch := range name {
			if !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", ch) {
				return nil, errors.New("invalid header name")
			}
		}
		value = strings.TrimSpace(value)
		for _, ch := range value {
			if ch < 32 || ch == 127 {
				return nil, errors.New("invalid header value")
			}
		}
		lower := strings.ToLower(name)
		switch lower {
		case "host", "connection", "content-type", "content-length", "content-encoding", "transfer-encoding", "accept", "accept-encoding", "trailer", "te", "upgrade", "proxy-authorization", "proxy-connection":
			return nil, errors.New("credential descriptor cannot set routing or framing headers")
		}
		if strings.HasPrefix(lower, "a2a-") || len(h.Values(name)) != 0 {
			return nil, errors.New("reserved or repeated credential header")
		}
		h.Set(name, value)
	}
	return h, nil
}

type Transport struct {
	next     *http.Transport
	endpoint *url.URL
	headers  http.Header
	limit    int64
	Sent     atomic.Bool
	Status   atomic.Int32
	Invalid  atomic.Bool
}

func HTTPClient(endpoint *url.URL, n Network) (*http.Client, *Transport, error) {
	tlsConfig, err := TLSConfig(n.CA, n.Cert, n.Key)
	if err != nil {
		return nil, nil, err
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = nil
	base.TLSClientConfig = tlsConfig
	base.DisableKeepAlives = true // no connection reuse or replay on a stale connection
	base.DisableCompression = true
	base.MaxResponseHeaderBytes = 64 << 10
	r := &Transport{next: base, endpoint: endpoint, headers: n.Headers.Clone(), limit: n.MaxOutput}
	if r.limit <= 0 {
		r.limit = 64 << 20
	}
	return &http.Client{Transport: r, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, r, nil
}

func (r *Transport) Close() { r.next.CloseIdleConnections() }

func (r *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != r.endpoint.Scheme || !strings.EqualFold(req.URL.Host, r.endpoint.Host) || req.URL.User != nil {
		return nil, errors.New("request left the selected endpoint origin")
	}
	trace := &httptrace.ClientTrace{WroteHeaders: func() { r.Sent.Store(true) }}
	clone := req.Clone(httptrace.WithClientTrace(req.Context(), trace))
	clone.Header = req.Header.Clone()
	clone.GetBody = nil // never replay a transmitted body
	for name, values := range r.headers {
		clone.Header[name] = append([]string(nil), values...)
	}
	var requestID json.RawMessage
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
		var envelope struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
		}
		if json.Unmarshal(b, &envelope) == nil && envelope.JSONRPC == "2.0" {
			requestID = envelope.ID
		}
		clone.Body = io.NopCloser(bytes.NewReader(b))
	}
	resp, err := r.next.RoundTrip(clone)
	if err != nil {
		return nil, err
	}
	r.Status.Store(int32(resp.StatusCode))
	limited := &limitedBody{ReadCloser: resp.Body, left: r.limit, invalid: &r.Invalid}
	typ, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if typ == "text/event-stream" && resp.StatusCode == http.StatusOK {
		resp.Body = &eventBody{source: limited, scanner: bufio.NewReader(limited), id: requestID, invalid: &r.Invalid, limit: r.limit}
		return resp, nil
	}
	b, readErr := io.ReadAll(limited)
	_ = limited.Close()
	if readErr != nil {
		r.Invalid.Store(true)
		return nil, errors.New("response exceeds bounds or was interrupted")
	}
	if resp.StatusCode == http.StatusOK {
		if err := validResponse(b, requestID); err != nil {
			r.Invalid.Store(true)
			return nil, err
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return resp, nil
}

func validResponse(b []byte, id json.RawMessage) error {
	if err := CheckJSON(b); err != nil {
		return err
	}
	if len(id) == 0 {
		return nil
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal(b, &envelope) != nil || envelope.JSONRPC != "2.0" || !bytes.Equal(bytes.TrimSpace(id), bytes.TrimSpace(envelope.ID)) || (len(envelope.Result) == 0) == (len(envelope.Error) == 0) {
		return errors.New("response lacks a unique matching JSON-RPC result")
	}
	return nil
}

type limitedBody struct {
	io.ReadCloser
	left    int64
	invalid *atomic.Bool
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if int64(len(p)) > b.left+1 {
		p = p[:b.left+1]
	}
	n, err := b.ReadCloser.Read(p)
	b.left -= int64(n)
	if b.left < 0 {
		b.invalid.Store(true)
		return 0, errors.New("response byte limit exceeded")
	}
	return n, err
}

// Validate each SSE data envelope before handing the original frame to the
// SDK. The SDK continues to own event parsing and A2A deserialization.
type eventBody struct {
	source  io.ReadCloser
	scanner *bufio.Reader
	id      json.RawMessage
	invalid *atomic.Bool
	limit   int64
	pending []byte
}

func (b *eventBody) Close() error { return b.source.Close() }
func (b *eventBody) Read(p []byte) (int, error) {
	for len(b.pending) == 0 {
		var frame bytes.Buffer
		var data []string
		for {
			line, err := b.scanner.ReadString('\n')
			frame.WriteString(line)
			if int64(frame.Len()) > b.limit {
				b.invalid.Store(true)
				return 0, errors.New("event exceeds byte limit")
			}
			trimmed := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			if v, ok := strings.CutPrefix(trimmed, "data:"); ok {
				data = append(data, strings.TrimPrefix(v, " "))
			}
			if err != nil {
				if err == io.EOF && frame.Len() == 0 {
					return 0, io.EOF
				}
				b.invalid.Store(true)
				return 0, errors.New("event stream ended inside a frame")
			}
			if trimmed == "" {
				break
			}
		}
		if len(data) > 0 {
			if err := validResponse([]byte(strings.Join(data, "\n")), b.id); err != nil {
				b.invalid.Store(true)
				return 0, fmt.Errorf("invalid event: %w", err)
			}
		}
		b.pending = frame.Bytes()
	}
	n := copy(p, b.pending)
	b.pending = b.pending[n:]
	return n, nil
}
