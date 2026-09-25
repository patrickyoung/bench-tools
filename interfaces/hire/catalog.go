package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var sourceID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var hexID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var commitID = regexp.MustCompile(`^[a-f0-9]{40}$`)

type member struct {
	Worker  string `json:"worker"`
	Adapter string `json:"adapter"`
}

type entry struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Owner       string            `json:"owner"`
	Status      string            `json:"status"`
	Members     map[string]member `json:"members"`
	Kind        string
	Title       string `json:"-"`
	Local       bool   `json:"-"`
	JobID       string `json:"-"`
	Path        string `json:"-"`
}

func (e entry) Name() string {
	if e.Local {
		return e.Title
	}
	return displayName(e.ID)
}
func (e entry) URL() string {
	if e.Local {
		return "/local-workers/" + e.ID
	}
	return "/" + e.Kind + "s/" + e.ID
}
func (e entry) Initials() string {
	var out string
	count := 0
	for _, word := range strings.Fields(strings.ReplaceAll(e.Name(), "-", " ")) {
		if count < 2 && word != "" {
			out += strings.ToUpper(string([]rune(word)[0]))
			count++
		}
	}
	return out
}
func (e entry) Exportable() bool {
	return !e.Local && (e.Status == "active" || e.Status == "experimental")
}

func displayName(id string) string {
	s := strings.ReplaceAll(id, "-", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

type catalog struct {
	Workers  []entry
	Teams    []entry
	Revision string
	Source   string
}

// The catalog command validates the library. The UI does not import its code.
func loadCatalog(source, python string) (catalog, error) {
	c := catalog{Source: source}
	for _, kind := range []string{"worker", "team"} {
		args := []string{filepath.Join(source, "scripts", "workers"), "list", "--all"}
		if kind == "team" {
			args = append(args, "--teams")
		}
		out, err := capture(python, args...)
		if err != nil {
			return c, fmt.Errorf("load %s catalog: %w", kind, err)
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		for {
			var e entry
			if err := dec.Decode(&e); err == io.EOF {
				break
			} else if err != nil {
				return c, err
			}
			if !sourceID.MatchString(e.ID) {
				return c, fmt.Errorf("invalid catalog ID")
			}
			e.Kind = kind
			if kind == "worker" {
				c.Workers = append(c.Workers, e)
			} else {
				c.Teams = append(c.Teams, e)
			}
		}
	}
	out, err := capture("git", "-C", source, "rev-parse", "HEAD")
	if err != nil {
		return c, err
	}
	c.Revision = strings.TrimSpace(string(out))
	if !commitID.MatchString(c.Revision) {
		return c, fmt.Errorf("source needs a full Git commit")
	}
	return c, nil
}

func (c catalog) find(kind, id string) (entry, bool) {
	entries := c.Workers
	if kind == "team" {
		entries = c.Teams
	}
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return entry{}, false
}

// Bounded public-command output prevents a malformed local command exhausting memory.
type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("command output exceeds %d bytes", b.limit)
	}
	return b.Buffer.Write(p)
}
func capture(bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = time.Second
	out, diagnostic := &limitedBuffer{limit: 2 << 20}, &limitedBuffer{limit: 16 << 10}
	cmd.Stdout, cmd.Stderr = out, diagnostic
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, diagnostic.String())
	}
	return out.Bytes(), nil
}

// os.Root prevents a source or generated symlink from reaching outside the selected root.
func readText(root, name string, max int64) (string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("text root must be a real directory")
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer r.Close()
	info, err = r.Lstat(name)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("text must be a regular file")
	}
	// Nonblocking prevents a replaced FIFO from hanging a request before Stat.
	f, err := r.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > max {
		return "", fmt.Errorf("file is not regular text within the size limit")
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return "", err
	}
	if int64(len(b)) > max {
		return "", fmt.Errorf("file exceeds size limit")
	}
	return string(b), nil
}
