package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var errNotFound = errors.New("not found")

type connector struct {
	name string
	path string
}

func (a *app) connectors() ([]connector, error) {
	dirs, err := a.connectorDirs()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []connector
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if seen[name] || !validName(name) {
				continue
			}
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err != nil {
				return nil, fmt.Errorf("stat %s: %w", path, err)
			}
			if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
				continue
			}
			seen[name] = true
			out = append(out, connector{name: name, path: path})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

func (a *app) connector(name string) (string, error) {
	if !validName(name) {
		return "", fmt.Errorf("invalid source name %q", name)
	}
	connectors, err := a.connectors()
	if err != nil {
		return "", err
	}
	for _, c := range connectors {
		if c.name == name {
			return c.path, nil
		}
	}
	return "", errNotFound
}

func (a *app) connectorDirs() ([]string, error) {
	if value := a.getenv("CONTEXT_PATH"); value != "" {
		var dirs []string
		for _, dir := range filepath.SplitList(value) {
			if dir != "" {
				dirs = append(dirs, dir)
			}
		}
		return dirs, nil
	}
	home, err := a.homeDir()
	if err != nil {
		return nil, fmt.Errorf("home directory: %w", err)
	}
	return []string{filepath.Join(".context", "connectors"),
		filepath.Join(home, ".context", "connectors")}, nil
}

func validName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return false
	}
	for i, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			i > 0 && (r == '-' || r == '_' || r == '.') {
			continue
		}
		return false
	}
	return true
}

func normalizeSource(raw []byte, wantName string) ([]byte, error) {
	lines, err := splitJSONL(raw)
	if err != nil {
		return nil, err
	}
	if len(lines) != 1 {
		return nil, fmt.Errorf("describe must emit exactly one source record")
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(lines[0], &row); err != nil {
		return nil, fmt.Errorf("invalid source JSON: %w", err)
	}
	kind, err := stringField(row, "kind")
	if err != nil || kind != "source" {
		return nil, fmt.Errorf("source kind must be %q", "source")
	}
	version, err := intField(row, "version")
	if err != nil || version != 1 {
		return nil, fmt.Errorf("source version must be 1")
	}
	name, err := stringField(row, "name")
	if err != nil || name != wantName {
		return nil, fmt.Errorf("source name must be %q", wantName)
	}
	description, err := stringField(row, "description")
	if err != nil || strings.TrimSpace(description) == "" {
		return nil, fmt.Errorf("source description must be a nonempty string")
	}
	return json.Marshal(row)
}

func readBounded(r io.Reader, limit int) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > limit {
		return nil, fmt.Errorf("input exceeds %d bytes", limit)
	}
	return b, nil
}
