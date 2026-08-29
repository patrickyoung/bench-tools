package state

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSaveLoadPermissionsAndDelete(t *testing.T) {
	t.Setenv("OAUTH_HOME", t.TempDir())
	p := Profile{Name: "corp", Resource: "https://resource.example", Issuer: "https://issuer.example", TokenEndpoint: "https://issuer.example/token", ClientID: "cli", ClientAuth: "none"}
	c := Credential{AccessToken: "secret-access", RefreshToken: "secret-refresh", Expiry: time.Now().Add(time.Hour)}
	if err := Save("corp", p, c); err != nil {
		t.Fatal(err)
	}
	dir, _ := Dir("corp")
	for _, path := range []string{dir, filepath.Join(dir, "profile.json"), filepath.Join(dir, "credential.json")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0o600)
		if info.IsDir() {
			want = 0o700
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s mode = %04o, want %04o", path, info.Mode().Perm(), want)
		}
	}
	gotP, err := LoadProfile("corp")
	if err != nil || gotP.Resource != p.Resource {
		t.Fatalf("profile = %#v, %v", gotP, err)
	}
	gotC, err := LoadCredential("corp")
	if err != nil || gotC.AccessToken != c.AccessToken || gotC.RefreshToken != c.RefreshToken {
		t.Fatalf("credential = %#v, %v", gotC, err)
	}
	if gotP.Binding == "" || gotP.Binding != gotC.Binding {
		t.Fatalf("state binding profile=%q credential=%q", gotP.Binding, gotC.Binding)
	}
	if err := Delete("corp"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("logout left profile directory: %v", err)
	}
}

func TestLoadRefusesMismatchedRecords(t *testing.T) {
	t.Setenv("OAUTH_HOME", t.TempDir())
	p := Profile{Resource: "https://resource.example", Issuer: "https://issuer.example", TokenEndpoint: "https://issuer.example/token", ClientID: "cli", ClientAuth: "none"}
	if err := Save("first", p, Credential{AccessToken: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := Save("second", p, Credential{AccessToken: "second"}); err != nil {
		t.Fatal(err)
	}
	first, _ := Dir("first")
	second, _ := Dir("second")
	data, err := os.ReadFile(filepath.Join(second, "credential.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "credential.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load("first"); err == nil {
		t.Fatal("accepted mismatched profile and credential records")
	}
}

func TestLoadRefusesSymlinkAndHardLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("link semantics differ on Windows")
	}
	t.Setenv("OAUTH_HOME", t.TempDir())
	p := Profile{Name: "corp", Resource: "https://resource.example", Issuer: "https://issuer.example", TokenEndpoint: "https://issuer.example/token", ClientID: "cli", ClientAuth: "none"}
	if err := Save("corp", p, Credential{AccessToken: "a"}); err != nil {
		t.Fatal(err)
	}
	dir, _ := Dir("corp")
	credential := filepath.Join(dir, "credential.json")
	original := filepath.Join(dir, "original")
	if err := os.Rename(credential, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original, credential); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCredential("corp"); err == nil {
		t.Fatal("loaded symlinked credential")
	}
	if err := os.Remove(credential); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(original, credential); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCredential("corp"); err == nil {
		t.Fatal("loaded multiply-linked credential")
	}
}

func TestLockSerializes(t *testing.T) {
	t.Setenv("OAUTH_HOME", t.TempDir())
	release, err := Lock("corp")
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan func(), 1)
	go func() {
		next, lockErr := Lock("corp")
		if lockErr != nil {
			acquired <- nil
			return
		}
		acquired <- next
	}()
	select {
	case <-acquired:
		t.Fatal("second lock did not block")
	case <-time.After(50 * time.Millisecond):
	}
	release()
	select {
	case next := <-acquired:
		if next == nil {
			t.Fatal("second lock failed")
		}
		next()
	case <-time.After(time.Second):
		t.Fatal("second lock did not proceed")
	}
}
