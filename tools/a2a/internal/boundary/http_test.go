package boundary

import (
	"strings"
	"testing"
)

func TestEndpointAuthority(t *testing.T) {
	for _, url := range []string{"http://example.com", "http://localhost", "https://user:secret@example.com", "https://example.com?token=secret", "https://example.com#fragment"} {
		if _, err := Endpoint(url, true); err == nil {
			t.Fatalf("accepted %s", url)
		}
	}
	for _, url := range []string{"http://127.0.0.1:8080", "http://[::1]:8080", "https://example.com/agent"} {
		if _, err := Endpoint(url, true); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}
}

func TestCredentialHeadersCannotChangeProtocolRouting(t *testing.T) {
	for _, raw := range []string{"Host: attacker\n", "Content-Length: 10\n", "A2A-Version: 0.3\n", "Authorization: one\nAuthorization: two\n", "Authorization: bad\x00value\n", "Bad Header: value\n"} {
		if _, err := ReadHeaders(strings.NewReader(raw)); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	h, err := ReadHeaders(strings.NewReader("Authorization: Bearer credential\r\n"))
	if err != nil || h.Get("Authorization") != "Bearer credential" {
		t.Fatalf("headers=%v err=%v", h, err)
	}
}

func TestJSONRejectsAmbiguousRecords(t *testing.T) {
	for _, raw := range []string{"{\"x\":\"\xff\"}", `{"a":1,"a":2}`, `{"x":{"a":1,"a":2}}`, `{} {}`, `{"a":`, strings.Repeat("[", 130) + strings.Repeat("]", 130)} {
		if err := CheckJSON([]byte(raw)); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if _, err := ReadObject(strings.NewReader(`{"x":[true,null,1,"text"]}`), 1024); err != nil {
		t.Fatal(err)
	}
}
