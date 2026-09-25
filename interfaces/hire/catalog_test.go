package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureRepository(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	root := t.TempDir()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "--quiet", root}, {"-C", root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "--no-gpg-sign", "--quiet", "-m", "fixture"}} {
		cmd := exec.Command(git, args...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null"}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git: %v: %s", err, out)
		}
	}
	return root
}

func TestCatalogUsesPublicCommandForWorkersAndTeams(t *testing.T) {
	source := fixtureRepository(t)
	bin := filepath.Join(t.TempDir(), "catalog command")
	writeFixture(t, bin, "#!/bin/sh\n[ \"$2\" = list ] && [ \"$3\" = --all ] || exit 9\ncase \"$4\" in\n  --teams) printf '%s\\n' '{\"id\":\"studio\",\"description\":\"Team\",\"status\":\"active\",\"members\":{\"writer\":{\"worker\":\"writer\"}}}' ;;\n  '') printf '%s\\n' '{\"id\":\"writer\",\"description\":\"Writes\",\"status\":\"active\"}' ;;\n  *) exit 8 ;;\nesac\n", 0700)
	c, err := loadCatalog(source, bin)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Workers) != 1 || len(c.Teams) != 1 || !commitID.MatchString(c.Revision) || c.Workers[0].Kind != "worker" || c.Teams[0].Members["writer"].Worker != "writer" {
		t.Fatalf("catalog: %+v", c)
	}
	if e, ok := c.find("team", "studio"); !ok || e.URL() != "/teams/studio" {
		t.Fatal("team lookup failed")
	}
	writeFixture(t, bin, "#!/bin/sh\nprintf '%s\\n' '{\"id\":\"../escape\"}'\n", 0700)
	if _, err := loadCatalog(source, bin); err == nil {
		t.Fatal("accepted unsafe source ID")
	}
	writeFixture(t, bin, "#!/bin/sh\nprintf 'unavailable\\n' >&2\nexit 7\n", 0700)
	if _, err := loadCatalog(source, bin); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("lost command failure: %v", err)
	}
}

func TestReadTextBoundsAndRootConfinement(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "README.md"), "hello", 0600)
	if got, err := readText(root, "README.md", 5); err != nil || got != "hello" {
		t.Fatalf("read: %q %v", got, err)
	}
	if _, err := readText(root, "README.md", 4); err == nil {
		t.Fatal("accepted oversized text")
	}
	if _, err := readText(root, ".", 100); err == nil {
		t.Fatal("accepted directory")
	}
	outside := filepath.Join(t.TempDir(), "secret")
	writeFixture(t, outside, "private", 0600)
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := readText(root, "link", 100); err == nil {
		t.Fatal("followed symlink outside selected root")
	}
	if _, err := readText(root, "../secret", 100); err == nil {
		t.Fatal("accepted traversal")
	}
}

func TestReadTextRejectsGeneratedSymlinkRoot(t *testing.T) {
	outside := t.TempDir()
	writeFixture(t, filepath.Join(outside, "README.md"), "outside private guide", 0600)
	generated := filepath.Join(t.TempDir(), "expert")
	if err := os.Symlink(outside, generated); err != nil {
		t.Fatal(err)
	}
	if _, err := readText(generated, "README.md", 100); err == nil {
		t.Fatal("followed generated expert root outside its workspace")
	}
}

func TestPrepareRequiresPrivateExternalState(t *testing.T) {
	source := fixtureRepository(t)
	base := config{Source: source, Data: filepath.Join(t.TempDir(), "private"), Hire: os.Args[0], Python: os.Args[0]}
	prepared, err := prepare(base)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(prepared.Source) || !filepath.IsAbs(prepared.Data) {
		t.Fatal("paths not normalized")
	}
	for _, test := range []struct {
		name   string
		change func(*config)
	}{
		{"missing source", func(c *config) { c.Source = "" }},
		{"missing data", func(c *config) { c.Data = "" }},
		{"source itself", func(c *config) { c.Data = source }},
		{"source child", func(c *config) { c.Data = filepath.Join(source, "state") }},
		{"missing executable", func(c *config) { c.Hire = filepath.Join(source, "missing") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := base
			test.change(&c)
			if _, err := prepare(c); err == nil {
				t.Fatal("accepted unsafe configuration")
			}
		})
	}
	shared := t.TempDir()
	if err := os.Chmod(shared, 0755); err != nil {
		t.Fatal(err)
	}
	c := base
	c.Data = shared
	if _, err := prepare(c); err == nil {
		t.Fatal("accepted shared state directory")
	}
	link := filepath.Join(t.TempDir(), "source-link")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	c = base
	c.Data = link
	if _, err := prepare(c); err == nil {
		t.Fatal("accepted data symlink into source")
	}
	c.Data = filepath.Join(link, "must-not-create")
	if _, err := prepare(c); err == nil {
		t.Fatal("accepted data beneath a symlink into source")
	}
	if _, err := os.Stat(filepath.Join(source, "must-not-create")); !os.IsNotExist(err) {
		t.Fatal("rejected data path still created a directory in source")
	}
	if err := os.Mkdir(filepath.Join(source, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	c = base
	c.Source = filepath.Join(source, "nested")
	c.Data = filepath.Join(source, "elsewhere")
	if _, err := prepare(c); err == nil {
		t.Fatal("source subdirectory hid data in repository")
	}
}

func TestServerRejectsNonLoopbackBeforeSetup(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8787", "192.0.2.1:8787", "localhost:8787", ":8787"} {
		if err := run(config{Addr: addr}); err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Fatalf("%q: %v", addr, err)
		}
	}
}
