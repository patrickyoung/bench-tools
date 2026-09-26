package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("MONIKER_TEST_COMMAND") == "1" {
		os.Exit(command(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func checkRecord(t *testing.T, dir string, value nameRecord) {
	t.Helper()
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(value.ID) || !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(value.Slug) || value.Name == "" || len(value.Name) > 120 || len(value.Slug) > 160 {
		t.Fatalf("invalid result: %+v", value)
	}
	raw, err := os.ReadFile(filepath.Join(dir, value.Slug+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var stored nameRecord
	if json.Unmarshal(raw, &stored) != nil || stored != value {
		t.Fatalf("reservation differs from result: %s", raw)
	}
	info, err := os.Stat(filepath.Join(dir, value.Slug+".json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("reservation is not private", err)
	}
}

func TestThemesAndPrivateRegistry(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "registry")
	for theme := range themes {
		value, err := generate(dir, theme)
		if err != nil || value.Theme != theme {
			t.Fatal(value, err)
		}
		checkRecord(t, dir, value)
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("registry is not private", err)
	}
}

func TestConcurrentProcessesAndRestartNeverReuseNames(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "registry")
	type result struct {
		body []byte
		err  error
	}
	results := make(chan result, 16)
	invoke := func() {
		cmd := exec.Command(self, "-dir", dir, "-json")
		cmd.Env = []string{"MONIKER_TEST_COMMAND=1", "HOME=" + root, "TMPDIR=" + root, "PATH=/usr/bin:/bin"}
		var diag bytes.Buffer
		cmd.Stderr = &diag
		body, err := cmd.Output()
		if err != nil {
			err = fmt.Errorf("%w: %s", err, &diag)
		}
		results <- result{body, err}
	}
	for i := 0; i < 16; i++ {
		go invoke()
	}
	seen, ids := map[string]bool{}, map[string]bool{}
	accept := func() {
		t.Helper()
		got := <-results
		if got.err != nil {
			t.Fatal(got.err)
		}
		var value nameRecord
		if err := json.Unmarshal(got.body, &value); err != nil {
			t.Fatal(err)
		}
		if seen[value.Name] || ids[value.ID] || value.Theme != "playful" {
			t.Fatalf("duplicate reservation or wrong default: %+v", value)
		}
		seen[value.Name], ids[value.ID] = true, true
		checkRecord(t, dir, value)
	}
	for i := 0; i < 16; i++ {
		accept()
	}
	// A fresh process has no memory of the other invocations.
	invoke()
	accept()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 17 {
		t.Fatal("missing committed reservation", len(entries), err)
	}
}

type zeroEntropy struct{}

func (zeroEntropy) Read(b []byte) (int, error) { clear(b); return len(b), nil }

func TestCollisionFallbackAndBoundedExhaustion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "registry")
	const base = "Cosmic Otter Crew"
	const slug = "cosmic-otter-crew"
	choose := func(string) (string, error) { return base, nil }
	first, err := reserve(dir, "space", rand.Reader, choose)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, slug+".json"))
	second, err := reserve(dir, "space", rand.Reader, choose)
	if err != nil || first.Name == second.Name || !strings.HasPrefix(second.Name, base+" ") {
		t.Fatal("collision did not use suffix fallback", second, err)
	}
	checkRecord(t, dir, second)
	after, _ := os.ReadFile(filepath.Join(dir, slug+".json"))
	if !bytes.Equal(before, after) {
		t.Fatal("existing reservation changed")
	}
	// Empty/incomplete reservations still occupy a spelling. A fixed entropy
	// stream makes every suffix collide, testing the finite retry budget.
	if err := os.WriteFile(filepath.Join(dir, slug+"-00000000.json"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err = reserve(dir, "space", zeroEntropy{}, func(string) (string, error) { calls++; return base, nil })
	if err == nil || calls != baseAttempts {
		t.Fatal("collisions did not stop at the budget", calls, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, slug+"-00000000.json")); err != nil || len(b) != 0 {
		t.Fatal("incomplete reservation was recycled", err)
	}
}

type failedOutput struct{}

func (failedOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestOutputFailureRetainsReservation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "registry")
	var diag bytes.Buffer
	if code := command([]string{"-dir", dir}, nil, failedOutput{}, &diag); code != 1 {
		t.Fatal("output failure did not remain a runtime failure", code)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("uncertain reservation removed")
	}
	var out bytes.Buffer
	if code := command([]string{"-dir", dir}, nil, &out, &diag); code != 0 {
		t.Fatal(code, &diag)
	}
	entries, _ = os.ReadDir(dir)
	if len(entries) != 2 || !strings.HasSuffix(out.String(), "\n") || strings.Count(out.String(), "\n") != 1 {
		t.Fatal("retry or plain output violated its contract")
	}
}

func TestCLIUsageAndRegistryRefusals(t *testing.T) {
	root := t.TempDir()
	public := filepath.Join(root, "public")
	if err := os.Mkdir(public, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(public, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		code int
	}{
		{nil, 2}, {[]string{"-dir", "relative"}, 2},
		{[]string{"-dir", root, "-theme", "unknown"}, 2},
		{[]string{"-dir", root, "extra"}, 2}, {[]string{"-unknown"}, 2},
		{[]string{"-dir", public}, 1}, {[]string{"-dir", link}, 1},
	} {
		var out, diag bytes.Buffer
		if code := command(tc.args, nil, &out, &diag); code != tc.code || out.Len() != 0 || diag.Len() == 0 {
			t.Fatalf("args=%q status=%d stdout=%q stderr=%q", tc.args, code, &out, &diag)
		}
	}
	var out, diag bytes.Buffer
	if code := command([]string{"version"}, nil, &out, &diag); code != 0 || out.String() != "moniker 0.1.0\n" || diag.Len() != 0 {
		t.Fatal("version contract changed")
	}
}

func TestMCPAdapterStrictInputsAndErrorContracts(t *testing.T) {
	invalid := []string{
		`{}`, `null`, `[]`, `{"name":"other"}`, `{"name":1}`,
		`{"name":"generate_team_name","extra":true}`,
		`{"name":"generate_team_name","_meta":null}`,
		`{"name":"generate_team_name","_meta":[]}`,
		`{"name":"generate_team_name","_meta":{"x":1,"x":2}}`,
		`{"name":"generate_team_name","arguments":null}`,
		`{"name":"generate_team_name","arguments":[]}`,
		`{"name":"generate_team_name","arguments":{"dir":"/not-authorized"}}`,
		`{"name":"generate_team_name","arguments":{"theme":false}}`,
		`{"name":"generate_team_name","arguments":{"theme":null}}`,
		`{"name":"generate_team_name","arguments":{"theme":{}}}`,
		`{"name":"generate_team_name","arguments":{"theme":"unknown"}}`,
		`{"name":"generate_team_name","name":"generate_team_name"}`,
		`{"name":"generate_team_name","arguments":{"theme":"space","theme":"space"}}`,
		`{"name":"generate_team_name","arguments":{"theme":"space","th\u0065me":"space"}}`,
		`{"name":"generate_team_name"} {}`, `{"name":"generate_team_name"} trailing`,
		"{\"name\":\"\xff\"}", strings.Repeat(" ", mcpInputLimit+1),
	}
	for _, raw := range invalid {
		dir := filepath.Join(t.TempDir(), "registry")
		var out, diag bytes.Buffer
		code := command([]string{"mcp", "-dir", dir, "tools/call"}, strings.NewReader(raw), &out, &diag)
		var result mcpResult
		if code != 0 || json.Unmarshal(out.Bytes(), &result) != nil || !result.IsError || result.StructuredContent != nil || len(result.Content) != 1 || result.Content[0].Text == "" {
			t.Fatalf("invalid input %q accepted or wrong envelope: %d %s", raw, code, &out)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("invalid tool input created registry")
		}
	}
	dir := filepath.Join(t.TempDir(), "registry")
	var out, diag bytes.Buffer
	if code := command([]string{"mcp", "-dir", dir, "unknown/method"}, strings.NewReader("{}"), &out, &diag); code != 1 || !strings.Contains(out.String(), `"code":-32601`) {
		t.Fatal("unknown method contract", code, &out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("unknown method created registry")
	}
}

func TestMCPResultsAndManifest(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "registry")
	for _, raw := range []string{`{"name":"generate_team_name"}`, `{"name":"generate_team_name","arguments":{"theme":"nature"},"_meta":{"protocolVersion":"2025-11-25","clientInfo":{"name":"fixture"},"clientCapabilities":{},"dir":"/not-authorized","theme":"space"}}`} {
		var out, diag bytes.Buffer
		code := command([]string{"mcp", "-dir", dir, "tools/call"}, strings.NewReader(raw), &out, &diag)
		var result mcpResult
		if code != 0 || json.Unmarshal(out.Bytes(), &result) != nil || result.IsError || result.StructuredContent == nil || len(result.Content) != 1 || result.Content[0].Text != result.StructuredContent.Name {
			t.Fatal("MCP result contract", code, &out, &diag)
		}
		checkRecord(t, dir, *result.StructuredContent)
		if strings.Contains(raw, "_meta") && result.StructuredContent.Theme != "nature" {
			t.Fatal("protocol metadata changed the requested theme")
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("MCP calls should make distinct reservations")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	var out, diag bytes.Buffer
	if code := command([]string{"mcp", "-dir", dir, "tools/call"}, strings.NewReader(`{"name":"generate_team_name"}`), &out, &diag); code != 0 || !strings.Contains(out.String(), `"isError":true`) {
		t.Fatal("runtime tool failure became a transport failure", code, &out)
	}
	manifest, err := os.ReadFile("mcp/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var descriptor struct {
		Tools []struct {
			Name        string
			Annotations map[string]any
		}
	}
	if json.Unmarshal(manifest, &descriptor) != nil || len(descriptor.Tools) != 1 || descriptor.Tools[0].Name != "generate_team_name" {
		t.Fatal("manifest lost the adapter tool")
	}
	for _, key := range []string{"readOnlyHint", "idempotentHint", "openWorldHint"} {
		if descriptor.Tools[0].Annotations[key] != false {
			t.Fatalf("misleading manifest annotation %s", key)
		}
	}
}
