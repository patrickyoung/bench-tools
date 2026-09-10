package protocol

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/patrickyoung/oauth/internal/state"
)

const maxResponse = 1 << 20

type Client struct {
	HTTP         *http.Client
	AllowPrivate bool
	Now          func() time.Time
	Sleep        func(context.Context, time.Duration) error
}

type ResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

type ServerMetadata struct {
	Issuer                                    string   `json:"issuer"`
	AuthorizationEndpoint                     string   `json:"authorization_endpoint"`
	TokenEndpoint                             string   `json:"token_endpoint"`
	DeviceAuthorizationEndpoint               string   `json:"device_authorization_endpoint"`
	RegistrationEndpoint                      string   `json:"registration_endpoint"`
	CodeChallengeMethodsSupported             []string `json:"code_challenge_methods_supported"`
	GrantTypesSupported                       []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported         []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                           []string `json:"scopes_supported"`
	AuthorizationResponseIssuerParamSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

type Discovery struct {
	Resource ResourceMetadata `json:"resource_metadata"`
	Server   ServerMetadata   `json:"authorization_server_metadata"`
}

type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    time.Duration
	Scopes       []string
}

type DeviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
	Status      int    `json:"-"`
}

func (e *OAuthError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("oauth %s: %s", e.Code, e.Description)
	}
	return "oauth " + e.Code
}

func NewClient(timeout time.Duration, allowPrivate bool) *Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if allowPrivate || strings.EqualFold(host, "localhost") {
				return dialer.DialContext(ctx, network, address)
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if privateIP(ip) {
					return nil, fmt.Errorf("refusing private or local address %s for %s (use -allow-private for an operator-approved internal endpoint)", ip, host)
				}
			}
			var last error
			for _, ip := range ips {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				last = dialErr
			}
			if last == nil {
				last = fmt.Errorf("host %s resolved to no addresses", host)
			}
			return nil, last
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &Client{
		HTTP: &http.Client{
			Transport: transport,
			Timeout:   timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return errors.New("redirects are not followed")
			},
		},
		AllowPrivate: allowPrivate,
		Now:          time.Now,
		Sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func privateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func ValidateEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid endpoint %q", raw)
	}
	if u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("endpoint %q must not contain userinfo or fragment", raw)
	}
	host := u.Hostname()
	loopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		loopback = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return nil, fmt.Errorf("endpoint %q must use https (http is allowed only on loopback)", raw)
	}
	return u, nil
}

func (c *Client) Discover(ctx context.Context, resource, selectedIssuer string) (Discovery, error) {
	resourceURL, err := ValidateEndpoint(resource)
	if err != nil {
		return Discovery{}, err
	}
	if resourceURL.RawQuery != "" {
		return Discovery{}, fmt.Errorf("resource %q must not contain a query", resource)
	}
	challengeURL, challengeScopes, err := c.resourceChallenge(ctx, resource)
	if err != nil {
		return Discovery{}, err
	}
	var rm ResourceMetadata
	var last error
	candidates := resourceMetadataURLs(resourceURL)
	if challengeURL != "" {
		candidates = append([]string{challengeURL}, candidates...)
	}
	for _, candidate := range candidates {
		if err := c.getJSON(ctx, candidate, &rm); err == nil {
			last = nil
			break
		} else {
			last = err
		}
	}
	if last != nil {
		return Discovery{}, fmt.Errorf("protected resource metadata: %w", last)
	}
	if rm.Resource != resource {
		return Discovery{}, fmt.Errorf("protected resource metadata says %q, want exact resource %q", rm.Resource, resource)
	}
	if len(challengeScopes) > 0 {
		rm.ScopesSupported = challengeScopes
	}
	issuer := selectedIssuer
	if issuer == "" {
		if len(rm.AuthorizationServers) == 0 {
			return Discovery{}, errors.New("protected resource metadata names no authorization server")
		}
		issuer = rm.AuthorizationServers[0]
	} else if len(rm.AuthorizationServers) > 0 && !slices.Contains(rm.AuthorizationServers, issuer) {
		return Discovery{}, fmt.Errorf("selected issuer %q is not advertised by the resource", issuer)
	}
	issuerURL, err := ValidateEndpoint(issuer)
	if err != nil {
		return Discovery{}, fmt.Errorf("issuer: %w", err)
	}
	if issuerURL.RawQuery != "" {
		return Discovery{}, fmt.Errorf("issuer %q must not contain a query", issuer)
	}
	var sm ServerMetadata
	last = nil
	for _, candidate := range serverMetadataURLs(issuerURL) {
		if err := c.getJSON(ctx, candidate, &sm); err == nil {
			last = nil
			break
		} else {
			last = err
		}
	}
	if last != nil {
		return Discovery{}, fmt.Errorf("authorization server metadata: %w", last)
	}
	if sm.Issuer != issuer {
		return Discovery{}, fmt.Errorf("authorization metadata issuer %q does not exactly match %q", sm.Issuer, issuer)
	}
	for label, endpoint := range map[string]string{
		"authorization": sm.AuthorizationEndpoint,
		"token":         sm.TokenEndpoint,
		"device":        sm.DeviceAuthorizationEndpoint,
		"registration":  sm.RegistrationEndpoint,
	} {
		if endpoint == "" {
			continue
		}
		if _, err := ValidateEndpoint(endpoint); err != nil {
			return Discovery{}, fmt.Errorf("%s endpoint: %w", label, err)
		}
	}
	if sm.TokenEndpoint == "" {
		return Discovery{}, errors.New("authorization metadata has no token endpoint")
	}
	return Discovery{Resource: rm, Server: sm}, nil
}

func (c *Client) resourceChallenge(ctx context.Context, resource string) (string, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resource, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	for _, challenge := range resp.Header.Values("WWW-Authenticate") {
		fields := strings.Fields(challenge)
		if len(fields) == 0 || !strings.EqualFold(fields[0], "Bearer") {
			continue
		}
		metadata := authParam(challenge, "resource_metadata")
		if metadata == "" {
			continue
		}
		if _, err := ValidateEndpoint(metadata); err != nil {
			return "", nil, fmt.Errorf("resource_metadata challenge: %w", err)
		}
		return metadata, strings.Fields(authParam(challenge, "scope")), nil
	}
	return "", nil, nil
}

func authParam(challenge, want string) string {
	space := strings.IndexByte(challenge, ' ')
	if space < 0 {
		return ""
	}
	s := challenge[space+1:]
	for len(s) > 0 {
		s = strings.TrimLeft(s, " \t,")
		eq := strings.IndexByte(s, '=')
		if eq <= 0 {
			return ""
		}
		name := strings.TrimSpace(s[:eq])
		s = strings.TrimSpace(s[eq+1:])
		var value string
		if strings.HasPrefix(s, "\"") {
			s = s[1:]
			var b strings.Builder
			escaped := false
			for i := 0; i < len(s); i++ {
				if escaped {
					b.WriteByte(s[i])
					escaped = false
					continue
				}
				if s[i] == '\\' {
					escaped = true
					continue
				}
				if s[i] == '"' {
					value = b.String()
					s = s[i+1:]
					break
				}
				b.WriteByte(s[i])
			}
		} else {
			end := strings.IndexByte(s, ',')
			if end < 0 {
				value, s = strings.TrimSpace(s), ""
			} else {
				value, s = strings.TrimSpace(s[:end]), s[end+1:]
			}
		}
		if strings.EqualFold(name, want) {
			return value
		}
	}
	return ""
}

func resourceMetadataURLs(resource *url.URL) []string {
	root := *resource
	root.Path = "/.well-known/oauth-protected-resource"
	root.RawPath = ""
	root.RawQuery = ""
	if resource.Path == "" || resource.Path == "/" {
		return []string{root.String()}
	}
	path := *resource
	path.Path = "/.well-known/oauth-protected-resource" + strings.TrimSuffix(resource.Path, "/")
	path.RawPath = ""
	path.RawQuery = ""
	return []string{path.String(), root.String()}
}

func serverMetadataURLs(issuer *url.URL) []string {
	base := *issuer
	base.RawPath = ""
	base.RawQuery = ""
	path := strings.TrimSuffix(issuer.Path, "/")
	if path == "" {
		path = ""
	}
	oauth := base
	oauth.Path = "/.well-known/oauth-authorization-server" + path
	oidcInserted := base
	oidcInserted.Path = "/.well-known/openid-configuration" + path
	urls := []string{oauth.String(), oidcInserted.String()}
	if path != "" {
		oidcAppended := base
		oidcAppended.Path = path + "/.well-known/openid-configuration"
		urls = append(urls, oidcAppended.String())
	}
	return urls
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return err
	}
	if len(body) > maxResponse {
		return errors.New("response exceeds 1 MiB")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %s", endpoint, resp.Status)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s returned invalid JSON: %w", endpoint, err)
	}
	return nil
}

func RandomURLToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func PKCE() (verifier, challenge string, err error) {
	verifier, err = RandomURLToken(32)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func AuthorizationURL(p state.Profile, redirectURI, challenge, stateValue string) (string, error) {
	u, err := url.Parse(p.AuthorizationEndpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", stateValue)
	q.Set("resource", p.Resource)
	if len(p.Scopes) > 0 {
		q.Set("scope", strings.Join(p.Scopes, " "))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func SecureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (c *Client) ExchangeCode(ctx context.Context, p state.Profile, secret, code, verifier, redirectURI string) (Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
		"resource":      {p.Resource},
	}
	return c.token(ctx, p, secret, form)
}

func (c *Client) ClientCredentials(ctx context.Context, p state.Profile, secret string) (Token, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "resource": {p.Resource}}
	if len(p.Scopes) > 0 {
		form.Set("scope", strings.Join(p.Scopes, " "))
	}
	return c.token(ctx, p, secret, form)
}

func (c *Client) Refresh(ctx context.Context, p state.Profile, secret, refreshToken string) (Token, error) {
	if refreshToken == "" {
		return Token{}, errors.New("credential has no refresh token")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"resource":      {p.Resource},
	}
	return c.token(ctx, p, secret, form)
}

func (c *Client) StartDevice(ctx context.Context, p state.Profile, secret string) (DeviceAuthorization, error) {
	if p.DeviceEndpoint == "" {
		return DeviceAuthorization{}, errors.New("authorization server advertises no device endpoint")
	}
	form := url.Values{"client_id": {p.ClientID}, "resource": {p.Resource}}
	if len(p.Scopes) > 0 {
		form.Set("scope", strings.Join(p.Scopes, " "))
	}
	req, err := c.formRequest(ctx, p.DeviceEndpoint, form, p, secret)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	body, status, err := c.do(req)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	if status != http.StatusOK {
		return DeviceAuthorization{}, decodeOAuthError(status, body, secret)
	}
	var device DeviceAuthorization
	if err := json.Unmarshal(body, &device); err != nil {
		return device, err
	}
	if device.DeviceCode == "" || device.UserCode == "" || device.VerificationURI == "" || device.ExpiresIn <= 0 {
		return device, errors.New("device authorization response is incomplete")
	}
	if _, err := ValidateEndpoint(device.VerificationURI); err != nil {
		return device, fmt.Errorf("verification URI: %w", err)
	}
	if device.VerificationURIComplete != "" {
		if _, err := ValidateEndpoint(strings.Split(device.VerificationURIComplete, "?")[0]); err != nil {
			return device, fmt.Errorf("complete verification URI: %w", err)
		}
	}
	if device.Interval < 5 {
		device.Interval = 5
	}
	return device, nil
}

func (c *Client) PollDevice(ctx context.Context, p state.Profile, secret string, device DeviceAuthorization, progress func(time.Duration)) (Token, error) {
	interval := time.Duration(device.Interval) * time.Second
	deadline := c.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	for {
		if !c.Now().Before(deadline) {
			return Token{}, errors.New("device authorization expired")
		}
		if progress != nil {
			progress(interval)
		}
		if err := c.Sleep(ctx, interval); err != nil {
			return Token{}, err
		}
		form := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {device.DeviceCode},
		}
		tok, err := c.token(ctx, p, secret, form)
		if err == nil {
			return tok, nil
		}
		var oe *OAuthError
		if !errors.As(err, &oe) {
			return Token{}, err
		}
		switch oe.Code {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "access_denied", "expired_token":
			return Token{}, err
		default:
			return Token{}, err
		}
	}
}

func (c *Client) token(ctx context.Context, p state.Profile, secret string, form url.Values) (Token, error) {
	if form.Get("resource") == "" {
		form.Set("resource", p.Resource)
	}
	req, err := c.formRequest(ctx, p.TokenEndpoint, form, p, secret)
	if err != nil {
		return Token{}, err
	}
	body, status, err := c.do(req)
	if err != nil {
		return Token{}, err
	}
	if status != http.StatusOK {
		return Token{}, decodeOAuthError(status, body, secret, form.Get("refresh_token"), form.Get("code"), form.Get("device_code"))
	}
	var wire struct {
		AccessToken  string          `json:"access_token"`
		RefreshToken string          `json:"refresh_token"`
		TokenType    string          `json:"token_type"`
		ExpiresIn    json.RawMessage `json:"expires_in"`
		Scope        string          `json:"scope"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return Token{}, fmt.Errorf("token endpoint returned invalid JSON: %w", err)
	}
	if wire.AccessToken == "" {
		return Token{}, errors.New("token endpoint returned no access_token")
	}
	if wire.TokenType == "" {
		return Token{}, errors.New("token endpoint returned no token_type")
	}
	if !strings.EqualFold(wire.TokenType, "Bearer") {
		return Token{}, fmt.Errorf("unsupported token_type %q (only Bearer is supported)", wire.TokenType)
	}
	var expires time.Duration
	if len(wire.ExpiresIn) > 0 && string(wire.ExpiresIn) != "null" {
		var number json.Number
		if err := json.Unmarshal(wire.ExpiresIn, &number); err != nil {
			var text string
			if err2 := json.Unmarshal(wire.ExpiresIn, &text); err2 != nil {
				return Token{}, errors.New("token endpoint returned invalid expires_in")
			}
			number = json.Number(text)
		}
		seconds, err := strconv.ParseInt(number.String(), 10, 64)
		if err != nil || seconds < 0 || seconds > math.MaxInt64/int64(time.Second) {
			return Token{}, errors.New("token endpoint returned invalid expires_in")
		}
		expires = time.Duration(seconds) * time.Second
	}
	return Token{
		AccessToken:  wire.AccessToken,
		RefreshToken: wire.RefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expires,
		Scopes:       strings.Fields(wire.Scope),
	}, nil
}

func (c *Client) formRequest(ctx context.Context, endpoint string, form url.Values, p state.Profile, secret string) (*http.Request, error) {
	if _, err := ValidateEndpoint(endpoint); err != nil {
		return nil, err
	}
	switch p.ClientAuth {
	case "none":
		form.Set("client_id", p.ClientID)
	case "client_secret_basic":
		if secret == "" {
			return nil, errors.New("client_secret_basic requires a client secret")
		}
	case "client_secret_post":
		if secret == "" {
			return nil, errors.New("client_secret_post requires a client secret")
		}
		form.Set("client_id", p.ClientID)
		form.Set("client_secret", secret)
	default:
		return nil, fmt.Errorf("unsupported client authentication method %q", p.ClientAuth)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if p.ClientAuth == "client_secret_basic" {
		encoded := url.QueryEscape(p.ClientID) + ":" + url.QueryEscape(secret)
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(encoded)))
	}
	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(body) > maxResponse {
		return nil, resp.StatusCode, errors.New("response exceeds 1 MiB")
	}
	return body, resp.StatusCode, nil
}

func decodeOAuthError(status int, body []byte, secrets ...string) error {
	for _, secret := range secrets {
		if secret != "" {
			body = bytes.ReplaceAll(body, []byte(secret), []byte("[redacted]"))
		}
	}
	var oe OAuthError
	if json.Unmarshal(body, &oe) == nil && oe.Code != "" {
		oe.Status = status
		return &oe
	}
	line := strings.TrimSpace(string(bytes.ReplaceAll(body, []byte{'\n'}, []byte{' '})))
	if len(line) > 200 {
		line = line[:200]
	}
	if line == "" {
		line = http.StatusText(status)
	}
	return fmt.Errorf("token endpoint returned %d: %s", status, line)
}

func CredentialFromToken(tok Token, secret string, now time.Time, oldRefresh string, oldScopes []string) state.Credential {
	refresh := tok.RefreshToken
	if refresh == "" {
		refresh = oldRefresh
	}
	scopes := tok.Scopes
	if len(scopes) == 0 {
		scopes = oldScopes
	}
	var expiry time.Time
	if tok.ExpiresIn > 0 {
		expiry = now.Add(tok.ExpiresIn).UTC()
	}
	return state.Credential{
		Version:       1,
		AccessToken:   tok.AccessToken,
		RefreshToken:  refresh,
		ClientSecret:  secret,
		TokenType:     "Bearer",
		Expiry:        expiry,
		GrantedScopes: scopes,
	}
}

func Usable(c state.Credential, now time.Time) bool {
	return c.AccessToken != "" && (c.Expiry.IsZero() || now.Before(c.Expiry.Add(-30*time.Second)))
}
