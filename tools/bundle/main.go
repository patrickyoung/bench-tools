// Bundle packages independent programs; the selected entry owns execution.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const version = "0.2.0-dev"

// The builder carries only its own source. The generated program adds a
// go:embed file for the admitted archive; it never imports a companion tool.
//
//go:embed *.go go.mod
var source embed.FS

var payload []byte
var identifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type application struct {
	Schema      int      `json:"schema"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Interface   string   `json:"interface,omitempty"`
	Entry       string   `json:"entry"`
	Files       []string `json:"files"`
	Requires    []string `json:"requires,omitempty"`
	Followup    bool     `json:"followup,omitempty"`
	Resume      bool     `json:"resume,omitempty"`
}

type fileRecord struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
	Mode   uint32 `json:"mode"`
	Link   string `json:"link,omitempty"`
}

type manifest struct {
	Schema         int          `json:"schema"`
	Builder        string       `json:"builder"`
	LauncherSHA256 string       `json:"launcher_sha256"`
	Platform       string       `json:"platform"`
	App            application  `json:"app"`
	Files          []fileRecord `json:"files"`
}

func main() {
	code := command(os.Args[1:])
	os.Exit(code)
}

func command(args []string) int {
	if len(payload) != 0 {
		return launch(args)
	}
	if len(args) == 0 {
		fmt.Print(buildHelp)
		return 0
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
		fmt.Print(buildHelp)
		return 0
	case "version", "-version", "--version":
		fmt.Println("bundle " + version)
		return 0
	case "build":
		if err := build(args[1:]); err != nil {
			return problem("bundle", err, 1)
		}
		return 0
	default:
		return problem("bundle", fmt.Errorf("unknown command %q", args[0]), 2)
	}
}

func problem(name string, err error, code int) int {
	fmt.Fprintln(os.Stderr, name+":", err)
	return code
}

func decode(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected one JSON value")
	}
	return nil
}

func relative(name string) bool {
	return name != "" && name != "." && path.Clean(name) == name && !path.IsAbs(name) &&
		!strings.ContainsAny(name, "\\\x00\r\n") && name != ".." && !strings.HasPrefix(name, "../")
}

// Inspect each path component before accessing admitted source or controller
// files. Package links are created separately from regular-file extraction.
func noLinks(name string) error {
	abs, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	current := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(abs, current), current) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed: %s", current)
		}
	}
	return nil
}

// Canonicalize caller-selected directory ancestors (macOS /var and /tmp are
// system links), but refuse a symlink as the selected directory itself.
func physicalDir(name string) (string, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("expected a real directory: %s", name)
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

const buildHelp = `bundle - package a team and independent Bench programs

  bundle build -o APP -runtime PREFIX [-tool NAME ...] APP_DIR
  bundle help | version

APP_DIR/app.json declares the entry adapter, exact source files, requirements,
and goal or argv interface. PREFIX is an installed Bench prefix containing
lib/bench-tools/*/package.json; -tool selects packages (default: all).
The build verifies package receipts and compiles a native single executable.
It needs Go 1.26+, makes no model calls, and never replaces an existing APP.
Credentials and team-specific system dependencies remain external.
`
