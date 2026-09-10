package protocol

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/patrickyoung/oauth/internal/state"
)

func TestDiscoverProtectedResourceAndAuthorizationServer(t *testing.T) {
	var server *httptest.Server
	mux := http.NewServeMux()
	server = httptest.NewServer(mux)
	defer server.Close()
	resource := server.URL + "/mcp"
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ResourceMetadata{Resource: resource, AuthorizationServers: []string{server.URL}, ScopesSupported: []string{"read", "write"}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ServerMetadata{
			Issuer: server.URL, AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token",
			CodeChallengeMethodsSupported: []string{"S256"}, TokenEndpointAuthMethodsSupported: []string{"none", "client_secret_basic"},
		})
	})
	c := NewClient(time.Second, true)
	d, err := c.Discover(context.Background(), resource, "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Resource.Resource != resource || d.Server.Issuer != server.URL || d.Server.TokenEndpoint != server.URL+"/token" {
		t.Fatalf("discovery = %#v", d)
	}
}

func TestDiscoverPrefersChallengeMetadataAndScopes(t *testing.T) {
	var server *httptest.Server
	mux := http.NewServeMux()
	server = httptest.NewServer(mux)
	defer server.Close()
	resource := server.URL + "/mcp"
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+server.URL+`/metadata", scope="files:read files:write"`)
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("/metadata", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ResourceMetadata{Resource: resource, AuthorizationServers: []string{server.URL}, ScopesSupported: []string{"ignored"}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ServerMetadata{Issuer: server.URL, TokenEndpoint: server.URL + "/token"})
	})
	d, err := NewClient(time.Second, true).Discover(context.Background(), resource, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(d.Resource.ScopesSupported, " ") != "files:read files:write" {
		t.Fatalf("scopes = %v", d.Resource.ScopesSupported)
	}
}

func TestDiscoverIgnoresEmptyChallenge(t *testing.T) {
	var server *httptest.Server
	mux := http.NewServeMux()
	server = httptest.NewServer(mux)
	defer server.Close()
	resource := server.URL + "/mcp"
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("WWW-Authenticate", "")
		w.WriteHeader(http.StatusUnauthorized)
	})
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ResourceMetadata{Resource: resource, AuthorizationServers: []string{server.URL}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ServerMetadata{Issuer: server.URL, TokenEndpoint: server.URL + "/token"})
	})
	if _, err := NewClient(time.Second, true).Discover(context.Background(), resource, ""); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverRejectsUnsafeChallengeMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="http://attacker.example/metadata"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	if _, err := NewClient(time.Second, true).Discover(context.Background(), server.URL, ""); err == nil || !strings.Contains(err.Error(), "resource_metadata") {
		t.Fatalf("unsafe challenge err = %v", err)
	}
}

func TestDiscoverRejectsResourceAndIssuerSubstitution(t *testing.T) {
	var server *httptest.Server
	mux := http.NewServeMux()
	server = httptest.NewServer(mux)
	defer server.Close()
	resource := server.URL + "/mcp"
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(ResourceMetadata{Resource: server.URL + "/other", AuthorizationServers: []string{server.URL}})
	})
	if _, err := NewClient(time.Second, true).Discover(context.Background(), resource, ""); err == nil || !strings.Contains(err.Error(), "exact resource") {
		t.Fatalf("resource substitution err = %v", err)
	}
}

func TestClientCredentialsBasicAndResourceBinding(t *testing.T) {
	var got url.Values
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = r.ParseForm()
		got = r.PostForm
		fmt.Fprint(w, `{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","expires_in":3600,"scope":"read"}`)
	}))
	defer server.Close()
	p := state.Profile{Resource: server.URL + "/mcp", TokenEndpoint: server.URL, ClientID: "client id", ClientAuth: "client_secret_basic", Scopes: []string{"read"}}
	tok, err := NewClient(time.Second, true).ClientCredentials(context.Background(), p, "s:ecret")
	if err != nil {
		t.Fatal(err)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("client+id:s%3Aecret"))
	if auth != wantAuth || got.Get("grant_type") != "client_credentials" || got.Get("resource") != p.Resource || got.Get("scope") != "read" {
		t.Fatalf("auth=%q form=%v", auth, got)
	}
	if tok.AccessToken != "access" || tok.RefreshToken != "refresh" || tok.ExpiresIn != time.Hour {
		t.Fatalf("token = %#v", tok)
	}
}

func TestAuthorizationURLUsesPKCEStateAndResource(t *testing.T) {
	p := state.Profile{AuthorizationEndpoint: "https://issuer.example/authorize?audience=cli", ClientID: "client", Resource: "https://resource.example/mcp", Scopes: []string{"read", "write"}}
	raw, err := AuthorizationURL(p, "http://127.0.0.1:1234/callback", "challenge", "state")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	for key, want := range map[string]string{
		"audience": "cli", "client_id": "client", "resource": p.Resource,
		"code_challenge": "challenge", "code_challenge_method": "S256", "state": "state", "scope": "read write",
	} {
		if q.Get(key) != want {
			t.Errorf("%s = %q, want %q", key, q.Get(key), want)
		}
	}
}

func TestTokenPostDoesNotFollowRedirect(t *testing.T) {
	var stolen atomic.Int64
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server.URL+"/stolen", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/stolen", func(w http.ResponseWriter, _ *http.Request) {
		stolen.Add(1)
		fmt.Fprint(w, `{"access_token":"stolen","token_type":"Bearer"}`)
	})
	p := state.Profile{Resource: server.URL + "/mcp", TokenEndpoint: server.URL + "/token", ClientID: "client", ClientAuth: "client_secret_post"}
	if _, err := NewClient(time.Second, true).Refresh(context.Background(), p, "secret", "refresh"); err == nil {
		t.Fatal("followed token redirect")
	}
	if stolen.Load() != 0 {
		t.Fatal("redirect target received the secret-bearing request")
	}
}

func TestTokenErrorRedactsCredentialMaterial(t *testing.T) {
	const secret = "client-secret-value"
	const refresh = "refresh-secret-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"invalid_grant","error_description":"secret=%s refresh=%s"}`, secret, refresh)
	}))
	defer server.Close()
	p := state.Profile{Resource: server.URL + "/mcp", TokenEndpoint: server.URL, ClientID: "client", ClientAuth: "client_secret_post"}
	_, err := NewClient(time.Second, true).Refresh(context.Background(), p, secret, refresh)
	if err == nil {
		t.Fatal("refresh unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), refresh) {
		t.Fatalf("token error leaked credential material: %v", err)
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("token error did not preserve a useful redacted diagnostic: %v", err)
	}
}

func TestDeviceFlowPendingSlowDownAndSuccess(t *testing.T) {
	var polls atomic.Int64
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/device", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"device_code":"device","user_code":"ABCD","verification_uri":%q,"expires_in":60,"interval":5}`, server.URL+"/verify")
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		switch polls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"authorization_pending"}`)
		case 2:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"slow_down"}`)
		default:
			fmt.Fprint(w, `{"access_token":"ready","token_type":"Bearer"}`)
		}
	})
	p := state.Profile{Resource: server.URL + "/mcp", DeviceEndpoint: server.URL + "/device", TokenEndpoint: server.URL + "/token", ClientID: "client", ClientAuth: "none"}
	c := NewClient(time.Second, true)
	c.Sleep = func(context.Context, time.Duration) error { return nil }
	device, err := c.StartDevice(context.Background(), p, "")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := c.PollDevice(context.Background(), p, "", device, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "ready" || polls.Load() != 3 {
		t.Fatalf("token=%#v polls=%d", tok, polls.Load())
	}
}

func TestRefusals(t *testing.T) {
	for _, raw := range []string{"http://example.com/token", "https://user@example.com/token", "https://example.com/token#fragment"} {
		if _, err := ValidateEndpoint(raw); err == nil {
			t.Errorf("accepted unsafe endpoint %q", raw)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"access_token":"a","token_type":"DPoP"}`)
	}))
	defer server.Close()
	p := state.Profile{Resource: server.URL + "/mcp", TokenEndpoint: server.URL, ClientID: "client", ClientAuth: "none"}
	if _, err := NewClient(time.Second, true).ClientCredentials(context.Background(), p, ""); err == nil || !strings.Contains(err.Error(), "token_type") {
		t.Fatalf("non-Bearer token err = %v", err)
	}
}
