package run

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/patrickyoung/oauth/internal/state"
)

func oauthServer(t *testing.T, token http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()
	var server *httptest.Server
	mux := http.NewServeMux()
	server = httptest.NewServer(mux)
	resource := server.URL + "/mcp"
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"resource": resource, "authorization_servers": []string{server.URL}, "scopes_supported": []string{"read"},
		})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token",
			"code_challenge_methods_supported":               []string{"S256"},
			"grant_types_supported":                          []string{"authorization_code", "refresh_token", "client_credentials"},
			"token_endpoint_auth_methods_supported":          []string{"none", "client_secret_basic", "client_secret_post"},
			"authorization_response_iss_parameter_supported": true,
		})
	})
	mux.HandleFunc("/token", token)
	return server, resource
}

func TestClientCredentialsLoginStatusHeaderAndLogout(t *testing.T) {
	t.Setenv("OAUTH_HOME", t.TempDir())
	var resource string
	server, resource := oauthServer(t, func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok || id != "client" || secret != "secret" {
			http.Error(w, "bad client", http.StatusUnauthorized)
			return
		}
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "client_credentials" || r.PostForm.Get("resource") != resource {
			http.Error(w, "bad grant", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"access_token":"top-secret-access","refresh_token":"top-secret-refresh","token_type":"Bearer","expires_in":3600,"scope":"read"}`)
	})
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := CLI(context.Background(), []string{"login", "corp", "-allow-private", "-flow", "client-credentials", "-client-id", "client", "-client-secret-stdin", resource}, strings.NewReader("secret\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("login exit=%d stderr=%s", code, stderr.String())
	}
	if stdout.String() != "logged in corp\n" || strings.Contains(stdout.String()+stderr.String(), "top-secret") {
		t.Fatalf("login output leaked or changed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	if code := CLI(context.Background(), []string{"status", "corp"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("status exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "corp\tready") || strings.Contains(stdout.String(), "top-secret") || strings.Contains(stdout.String(), "secret") {
		t.Fatalf("unsafe status = %q", stdout.String())
	}

	stdout.Reset()
	if code := CLI(context.Background(), []string{"header", "-allow-private", "corp"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("header exit=%d stderr=%s", code, stderr.String())
	}
	if stdout.String() != "Authorization: Bearer top-secret-access\n" {
		t.Fatalf("header = %q", stdout.String())
	}

	stdout.Reset()
	if code := CLI(context.Background(), []string{"logout", "corp"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("logout exit=%d stderr=%s", code, stderr.String())
	}
	if _, err := state.LoadProfile("corp"); !os.IsNotExist(err) {
		t.Fatalf("logout left profile: %v", err)
	}
}

func TestClientSecretFileSafety(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("credential\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readSecret(strings.NewReader(""), false, secret)
	if err != nil || got != "credential" {
		t.Fatalf("protected secret = %q, %v", got, err)
	}
	if err := os.Chmod(secret, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecret(strings.NewReader(""), false, secret); err == nil {
		t.Fatal("accepted group/world-readable secret file")
	}
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.Chmod(secret, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "secret-link")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecret(strings.NewReader(""), false, link); err == nil {
		t.Fatal("accepted symlinked secret file")
	}
	hard := filepath.Join(dir, "secret-hard")
	if err := os.Link(secret, hard); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecret(strings.NewReader(""), false, secret); err == nil {
		t.Fatal("accepted multiply-linked secret file")
	}
}

func TestClientCredentialsWithExplicitEndpoints(t *testing.T) {
	t.Setenv("OAUTH_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok || id != "machine" || secret != "credential" {
			http.Error(w, "bad client", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"access_token":"explicit-access","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()
	resource := server.URL + "/resource"
	var stdout, stderr bytes.Buffer
	code := CLI(context.Background(), []string{
		"login", "explicit", "-allow-private", "-flow", "client-credentials",
		"-issuer", server.URL, "-token-endpoint", server.URL,
		"-client-id", "machine", "-client-secret-stdin", resource,
	}, strings.NewReader("credential\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("login exit=%d stderr=%s", code, stderr.String())
	}
	p, c, err := state.Load("explicit")
	if err != nil {
		t.Fatal(err)
	}
	if p.Resource != resource || p.Issuer != server.URL || c.AccessToken != "explicit-access" || c.ClientSecret != "credential" {
		t.Fatalf("profile=%#v credential=%#v", p, c)
	}
}

func TestAuthorizationCodeLoginPKCECallback(t *testing.T) {
	t.Setenv("OAUTH_HOME", t.TempDir())
	var challenge string
	var server *httptest.Server
	var resource string
	server, resource = oauthServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("code") != "code-1" || r.PostForm.Get("resource") != resource {
			http.Error(w, "bad exchange", http.StatusBadRequest)
			return
		}
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
			http.Error(w, "bad verifier", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"access_token":"code-access","refresh_token":"code-refresh","token_type":"Bearer","expires_in":3600}`)
	})
	defer server.Close()

	stderrRead, stderrWrite := io.Pipe()
	var stdout bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- CLI(context.Background(), []string{"login", "human", "-allow-private", "-flow", "code", "-client-id", "public-cli", "-no-browser", "-timeout", "5s", resource}, strings.NewReader(""), &stdout, stderrWrite)
		stderrWrite.Close()
	}()
	reader := bufio.NewReader(stderrRead)
	line, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(line, "open this URL") {
		t.Fatalf("login prompt = %q, %v", line, err)
	}
	authorize, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	authorize = strings.TrimSpace(authorize)
	u, err := url.Parse(authorize)
	if err != nil {
		t.Fatal(err)
	}
	challenge = u.Query().Get("code_challenge")
	callback, err := url.Parse(u.Query().Get("redirect_uri"))
	if err != nil {
		t.Fatal(err)
	}
	q := callback.Query()
	q.Set("code", "code-1")
	q.Set("state", u.Query().Get("state"))
	q.Set("iss", server.URL)
	callback.RawQuery = q.Encode()
	resp, err := http.Get(callback.String())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if code := <-done; code != 0 {
		rest, _ := io.ReadAll(reader)
		t.Fatalf("login exit=%d stderr=%s", code, rest)
	}
	if stdout.String() != "logged in human\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	c, err := state.LoadCredential("human")
	if err != nil || c.AccessToken != "code-access" || c.RefreshToken != "code-refresh" {
		t.Fatalf("credential=%#v err=%v", c, err)
	}
}

func TestConcurrentExpiredCredentialRefreshesOnce(t *testing.T) {
	t.Setenv("OAUTH_HOME", t.TempDir())
	var grants atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		grants.Add(1)
		time.Sleep(30 * time.Millisecond)
		_ = r.ParseForm()
		if r.PostForm.Get("refresh_token") != "r-0" {
			http.Error(w, "spent token twice", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"access_token":"a-1","refresh_token":"r-1","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()
	p := state.Profile{Name: "corp", Resource: server.URL + "/mcp", Issuer: server.URL, TokenEndpoint: server.URL, ClientID: "client", ClientAuth: "none"}
	c := state.Credential{AccessToken: "expired", RefreshToken: "r-0", TokenType: "Bearer", Expiry: time.Now().Add(-time.Hour)}
	if err := state.Save("corp", p, c); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, got, err := credential(context.Background(), "corp", time.Second, true)
			if err == nil && got.AccessToken != "a-1" {
				err = fmt.Errorf("access token = %q", got.AccessToken)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if grants.Load() != 1 {
		t.Fatalf("refresh grants = %d, want 1", grants.Load())
	}
	stored, _ := state.LoadCredential("corp")
	if stored.RefreshToken != "r-1" {
		t.Fatalf("stored rotating refresh token = %q", stored.RefreshToken)
	}
}

func TestCommandPassesOnlyDescriptorAndPreservesExit(t *testing.T) {
	if os.Getenv("OAUTH_HELPER") == "1" {
		f := os.NewFile(3, "oauth-header")
		b, _ := io.ReadAll(f)
		for _, arg := range os.Args {
			if strings.Contains(arg, "descriptor-secret") {
				fmt.Fprintln(os.Stderr, "secret appeared in argv")
				os.Exit(97)
			}
		}
		for _, item := range os.Environ() {
			if strings.Contains(item, "descriptor-secret") {
				fmt.Fprintln(os.Stderr, "secret appeared in environment")
				os.Exit(98)
			}
		}
		fmt.Fprint(os.Stdout, string(b))
		os.Exit(9)
	}
	t.Setenv("OAUTH_HELPER", "1")
	var stdout, stderr bytes.Buffer
	code, err := Command(context.Background(), []string{os.Args[0], "-test.run=TestCommandPassesOnlyDescriptorAndPreservesExit"}, "Authorization: Bearer descriptor-secret\n", strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != 9 || stdout.String() != "Authorization: Bearer descriptor-secret\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
