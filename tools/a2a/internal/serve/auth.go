package serve

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/patrickyoung/a2a/internal/boundary"
)

type AuthConfig struct {
	Mode             string   `json:"mode"`
	ClientCA         string   `json:"clientCA,omitempty"`
	Allow            []string `json:"allow,omitempty"`
	Issuer           string   `json:"issuer,omitempty"`
	IntrospectionURL string   `json:"introspectionURL,omitempty"`
	CredentialsFile  string   `json:"credentialsFile,omitempty"`
	Audience         string   `json:"audience,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	CA               string   `json:"ca,omitempty"`
}

type Auth struct {
	config           AuthConfig
	client           *http.Client
	clientID, secret string
	clientCAs        *x509.CertPool
	metadataURL      string
}

type principalKey struct{}

func principal(ctx context.Context) (string, error) {
	id, _ := ctx.Value(principalKey{}).(string)
	if id == "" {
		return "", a2a.ErrUnauthenticated
	}
	return id, nil
}
func ownedContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, principalKey{}, id)
}

func LoadAuth(path string) (*Auth, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := boundary.ReadObject(f, 1<<20)
	if err != nil {
		return nil, err
	}
	var cfg AuthConfig
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(&cfg); err != nil {
		return nil, errors.New("invalid authentication configuration")
	}
	for _, field := range []*string{&cfg.ClientCA, &cfg.CredentialsFile, &cfg.CA} {
		if *field != "" && !filepath.IsAbs(*field) {
			*field = filepath.Join(filepath.Dir(path), *field)
		}
	}
	return NewAuth(cfg)
}

func NewAuth(cfg AuthConfig) (*Auth, error) {
	a := &Auth{config: cfg}
	switch cfg.Mode {
	case "mtls":
		if cfg.ClientCA == "" || cfg.IntrospectionURL != "" || cfg.CredentialsFile != "" || cfg.Issuer != "" || cfg.Audience != "" || cfg.CA != "" || len(cfg.Scopes) > 0 {
			return nil, errors.New("mTLS authentication requires only a client CA and optional identity allowlist")
		}
		pem, err := os.ReadFile(cfg.ClientCA)
		if err != nil {
			return nil, err
		}
		a.clientCAs = x509.NewCertPool()
		if !a.clientCAs.AppendCertsFromPEM(pem) {
			return nil, errors.New("client CA file contains no certificates")
		}
	case "oauth":
		if cfg.ClientCA != "" || cfg.Issuer == "" || cfg.Audience == "" || cfg.CredentialsFile == "" {
			return nil, errors.New("OAuth validation requires issuer, audience, introspection URL, and private client credentials")
		}
		if _, err := boundary.Endpoint(cfg.Issuer, false); err != nil {
			return nil, errors.New("OAuth issuer requires HTTPS")
		}
		u, err := boundary.Endpoint(cfg.IntrospectionURL, false)
		if err != nil {
			return nil, err
		}
		secret, err := boundary.PrivateFile(cfg.CredentialsFile, 64<<10)
		if err != nil {
			return nil, err
		}
		var credentials struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		if boundary.CheckJSON(secret) != nil || json.Unmarshal(secret, &credentials) != nil || credentials.ClientID == "" || credentials.ClientSecret == "" {
			return nil, errors.New("invalid private introspection credentials")
		}
		a.clientID, a.secret = credentials.ClientID, credentials.ClientSecret
		a.client, _, err = boundary.HTTPClient(u, boundary.Network{CA: cfg.CA, MaxOutput: 1 << 20})
		if err != nil {
			return nil, err
		}
		a.client.Timeout = 5 * time.Second
	default:
		return nil, errors.New("authentication mode must be mtls or oauth")
	}
	return a, nil
}

func (a *Auth) TLS(base *tls.Config) *tls.Config {
	cfg := base.Clone()
	if a.config.Mode == "mtls" {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cfg.ClientCAs = a.clientCAs
	}
	return cfg
}

func (a *Auth) identify(r *http.Request) (string, error) {
	var id string
	if a.config.Mode == "mtls" {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
			return "", a2a.ErrUnauthenticated
		}
		digest := sha256.Sum256(r.TLS.PeerCertificates[0].RawSubjectPublicKeyInfo)
		id = "mtls:" + hex.EncodeToString(digest[:])
	} else {
		values := r.Header.Values("Authorization")
		if len(values) != 1 {
			return "", a2a.ErrUnauthenticated
		}
		scheme, token, ok := strings.Cut(values[0], " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || len(token) > 32<<10 || strings.ContainsAny(token, " \t\r\n") {
			return "", a2a.ErrUnauthenticated
		}
		form := url.Values{"token": {token}, "token_type_hint": {"access_token"}}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, a.config.IntrospectionURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "", a2a.ErrUnauthenticated
		}
		req.SetBasicAuth(url.QueryEscape(a.clientID), url.QueryEscape(a.secret))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := a.client.Do(req)
		if err != nil {
			return "", a2a.ErrUnauthenticated
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", a2a.ErrUnauthenticated
		}
		var status struct {
			Active    bool            `json:"active"`
			Sub       string          `json:"sub"`
			ClientID  string          `json:"client_id"`
			Issuer    string          `json:"iss"`
			Audience  json.RawMessage `json:"aud"`
			Scope     string          `json:"scope"`
			Expires   *int64          `json:"exp"`
			NotBefore *int64          `json:"nbf"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&status) != nil || !status.Active || (status.Sub == "" && status.ClientID == "") || (status.Issuer != "" && status.Issuer != a.config.Issuer) {
			return "", a2a.ErrUnauthenticated
		}
		now := time.Now().Unix()
		if status.Expires != nil && *status.Expires <= now || status.NotBefore != nil && *status.NotBefore > now {
			return "", a2a.ErrUnauthenticated
		}
		var audiences []string
		var single string
		if json.Unmarshal(status.Audience, &single) == nil {
			audiences = []string{single}
		} else {
			_ = json.Unmarshal(status.Audience, &audiences)
		}
		if !slices.Contains(audiences, a.config.Audience) {
			return "", a2a.ErrUnauthorized
		}
		scopes := strings.Fields(status.Scope)
		for _, required := range a.config.Scopes {
			if !slices.Contains(scopes, required) {
				return "", a2a.ErrUnauthorized
			}
		}
		// Hash the tuple, not an ambiguous concatenation of identity strings.
		tuple, _ := json.Marshal([]string{a.config.Issuer, status.Sub, status.ClientID})
		digest := sha256.Sum256(tuple)
		id = "oauth:" + hex.EncodeToString(digest[:])
	}
	if len(a.config.Allow) > 0 && !slices.Contains(a.config.Allow, id) {
		return "", a2a.ErrUnauthorized
	}
	return id, nil
}

func (a *Auth) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := a.identify(r)
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, a2a.ErrUnauthorized) {
				status = http.StatusForbidden
			} else if a.config.Mode == "oauth" {
				challenge := `Bearer realm="a2a"`
				if a.metadataURL != "" {
					challenge += ", resource_metadata=" + strconv.Quote(a.metadataURL)
				}
				w.Header().Set("WWW-Authenticate", challenge)
			}
			http.Error(w, http.StatusText(status), status)
			return
		}
		next.ServeHTTP(w, r.WithContext(ownedContext(r.Context(), id)))
	})
}
