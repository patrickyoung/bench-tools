package serve

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/patrickyoung/a2a/internal/boundary"
)

func certificateFile(t *testing.T, cert *x509.Certificate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func clientCertificates(t *testing.T) (string, []tls.Certificate) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "A2A test client CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	var clients []tls.Certificate
	for n := int64(2); n < 4; n++ {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(n), Subject: pkix.Name{CommonName: "client"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
		leaf, err := x509.CreateCertificate(rand.Reader, template, parsed, pub, priv)
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, tls.Certificate{Certificate: [][]byte{leaf, der}, PrivateKey: key})
	}
	return certificateFile(t, parsed), clients
}

func TestMTLSAuthenticatesDistinctPrincipals(t *testing.T) {
	ca, certs := clientCertificates(t)
	auth, err := NewAuth(AuthConfig{Mode: "mtls", ClientCA: ca})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(auth.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := principal(r.Context())
		if err != nil {
			t.Error(err)
		}
		io.WriteString(w, id)
	})))
	server.TLS = auth.TLS(&tls.Config{MinVersion: tls.VersionTLS12})
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	var identities []string
	for _, cert := range certs {
		transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.HasPrefix(string(b), "mtls:") {
			t.Fatalf("status=%d body=%s", resp.StatusCode, b)
		}
		identities = append(identities, string(b))
	}
	if identities[0] == identities[1] {
		t.Fatal("distinct clients share an identity")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	if resp, err := (&http.Client{Transport: transport}).Get(server.URL); err == nil {
		resp.Body.Close()
		t.Fatal("client without certificate was accepted")
	}
}

func TestOAuthUsesIntrospectionAndChecksResourceAuthority(t *testing.T) {
	issuer := "https://issuer.example"
	introspection := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok || id != "resource" || secret != "private-secret" {
			t.Error("introspection lacked its own credential")
			http.Error(w, "unauthorized", 401)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		token := r.Form.Get("token")
		result := map[string]any{"active": true, "sub": "user-a", "client_id": "client-a", "iss": issuer, "aud": []string{"https://worker.example"}, "scope": "agent:run", "exp": time.Now().Add(time.Minute).Unix()}
		switch token {
		case "good":
		case "other-client":
			result["client_id"] = "client-b"
		case "wrong-audience":
			result["aud"] = "https://other.example"
		case "wrong-scope":
			result["scope"] = "files:read"
		case "expired":
			result["exp"] = time.Now().Add(-time.Minute).Unix()
		case "wrong-issuer":
			result["iss"] = "https://attacker.example"
		default:
			result["active"] = false
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	defer introspection.Close()
	credentials := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(credentials, []byte(`{"client_id":"resource","client_secret":"private-secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	auth, err := NewAuth(AuthConfig{Mode: "oauth", Issuer: issuer, IntrospectionURL: introspection.URL, CredentialsFile: credentials, Audience: "https://worker.example", Scopes: []string{"agent:run"}, CA: certificateFile(t, introspection.Certificate())})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(auth.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := principal(r.Context())
		json.NewEncoder(w).Encode(map[string]string{"principal": id})
	})))
	defer server.Close()
	ca := certificateFile(t, server.Certificate())
	u, _ := boundary.Endpoint(server.URL, false)
	identities := map[string]string{}
	for _, test := range []struct {
		token  string
		status int
	}{{"good", 200}, {"other-client", 200}, {"wrong-audience", 403}, {"wrong-scope", 403}, {"expired", 401}, {"wrong-issuer", 401}, {"invalid", 401}, {"", 401}} {
		headers := http.Header{}
		if test.token != "" {
			headers.Set("Authorization", "Bearer "+test.token)
		}
		client, transport, err := boundary.HTTPClient(u, boundary.Network{CA: ca, Headers: headers, MaxOutput: 4096})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		transport.Close()
		if resp.StatusCode != test.status {
			t.Fatalf("%s: status=%d body=%s", test.token, resp.StatusCode, b)
		}
		if test.status == 200 {
			identities[test.token] = string(b)
		}
	}
	if identities["good"] == identities["other-client"] {
		t.Fatal("distinct OAuth clients share a task identity")
	}
}
