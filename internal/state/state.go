package state

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Profile struct {
	Version                     int      `json:"version"`
	Binding                     string   `json:"binding"`
	Name                        string   `json:"name"`
	Resource                    string   `json:"resource"`
	Issuer                      string   `json:"issuer"`
	AuthorizationEndpoint       string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint               string   `json:"token_endpoint"`
	DeviceEndpoint              string   `json:"device_authorization_endpoint,omitempty"`
	RegistrationEndpoint        string   `json:"registration_endpoint,omitempty"`
	ClientID                    string   `json:"client_id"`
	ClientAuth                  string   `json:"client_auth"`
	Scopes                      []string `json:"scopes,omitempty"`
	AuthorizationResponseIssuer bool     `json:"authorization_response_issuer,omitempty"`
}

type Credential struct {
	Version       int       `json:"version"`
	Binding       string    `json:"binding"`
	AccessToken   string    `json:"access_token,omitempty"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	ClientSecret  string    `json:"client_secret,omitempty"`
	TokenType     string    `json:"token_type,omitempty"`
	Expiry        time.Time `json:"expiry,omitempty"`
	GrantedScopes []string  `json:"granted_scopes,omitempty"`
}

func Home() (string, error) {
	if p := os.Getenv("OAUTH_HOME"); p != "" {
		return filepath.Abs(p)
	}
	if p := os.Getenv("XDG_STATE_HOME"); p != "" {
		return filepath.Abs(filepath.Join(p, "oauth"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "oauth"), nil
}

func Dir(name string) (string, error) {
	if !validName.MatchString(name) {
		return "", fmt.Errorf("invalid profile name %q", name)
	}
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, name), nil
}

func List() ([]string, error) {
	home, err := Home()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(home)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && validName.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

func LoadProfile(name string) (Profile, error) {
	var p Profile
	dir, err := Dir(name)
	if err != nil {
		return p, err
	}
	if err := readSecure(filepath.Join(dir, "profile.json"), &p); err != nil {
		return p, err
	}
	if p.Version != 1 || p.Name != name || p.Binding == "" {
		return p, fmt.Errorf("profile %q has an unsupported or mismatched record", name)
	}
	return p, nil
}

func LoadCredential(name string) (Credential, error) {
	var c Credential
	dir, err := Dir(name)
	if err != nil {
		return c, err
	}
	if err := readSecure(filepath.Join(dir, "credential.json"), &c); err != nil {
		return c, err
	}
	if c.Version != 1 || c.Binding == "" {
		return c, fmt.Errorf("profile %q has an unsupported credential record", name)
	}
	return c, nil
}

// Load reads a matched profile and credential pair. Binding prevents a
// reader from accepting records from opposite sides of an interrupted or
// concurrent profile replacement.
func Load(name string) (Profile, Credential, error) {
	p, err := LoadProfile(name)
	if err != nil {
		return p, Credential{}, err
	}
	c, err := LoadCredential(name)
	if err != nil {
		return p, c, err
	}
	if p.Binding != c.Binding {
		return p, c, fmt.Errorf("profile %q has mismatched state records", name)
	}
	return p, c, nil
}

func Save(name string, p Profile, c Credential) error {
	dir, err := ensureDir(name)
	if err != nil {
		return err
	}
	p.Version, p.Name = 1, name
	c.Version = 1
	binding := make([]byte, 16)
	if _, err := rand.Read(binding); err != nil {
		return err
	}
	p.Binding = base64.RawURLEncoding.EncodeToString(binding)
	c.Binding = p.Binding
	// Publish credentials first and the profile last. Load also verifies the
	// shared binding, so a crash between renames fails closed.
	if err := writeAtomic(filepath.Join(dir, "credential.json"), c); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "profile.json"), p)
}

func SaveCredential(name string, c Credential) error {
	dir, err := ensureDir(name)
	if err != nil {
		return err
	}
	p, err := LoadProfile(name)
	if err != nil {
		return err
	}
	c.Version = 1
	c.Binding = p.Binding
	return writeAtomic(filepath.Join(dir, "credential.json"), c)
}

func Delete(name string) error {
	dir, err := Dir(name)
	if err != nil {
		return err
	}
	for _, base := range []string{"profile.json", "credential.json", "lock"} {
		path := filepath.Join(dir, base)
		info, statErr := os.Lstat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing unsafe profile entry %s", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.Remove(dir)
}

func Lock(name string) (func(), error) {
	dir, err := ensureDir(name)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "lock")
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Sys().(*syscall.Stat_t).Nlink != 1 {
		f.Close()
		return nil, fmt.Errorf("refusing unsafe lock file %s", path)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

func ensureDir(name string) (string, error) {
	dir, err := Dir(name)
	if err != nil {
		return "", err
	}
	home := filepath.Dir(dir)
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(home, 0o700); err != nil {
		return "", err
	}
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("refusing unsafe profile directory %s", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func readSecure(path string, out any) error {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing unsafe state file %s", path)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink != 1 {
		return fmt.Errorf("refusing multiply-linked state file %s", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("refusing state file %s with permissions %04o", path, info.Mode().Perm())
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return fmt.Errorf("state file %s exceeds 1 MiB", path)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func writeAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".oauth-write.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if old, err := os.Lstat(path); err == nil {
		if old.Mode()&os.ModeSymlink != 0 || !old.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace unsafe state file %s", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	ok = true
	return nil
}
