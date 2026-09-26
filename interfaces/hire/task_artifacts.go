package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const taskFileLimit = 25 << 20

var taskTypes = map[string]string{".css": "text/css", ".js": "text/javascript", ".mjs": "text/javascript", ".woff2": "font/woff2", ".woff": "font/woff", ".ico": "image/x-icon", ".gif": "image/gif", ".htm": "text/html", ".tsv": "text/tab-separated-values", ".pdf": "application/pdf", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".svg": "image/svg+xml", ".html": "text/html", ".md": "text/plain", ".txt": "text/plain", ".csv": "text/csv", ".json": "application/json", ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation", ".zip": "application/zip"}

func taskFile(root, name string) ([]byte, error) {
	if !validRunFile(name) {
		return nil, fmt.Errorf("invalid artifact name")
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	info, e := r.Lstat(name)
	if e != nil || !info.Mode().IsRegular() || info.Size() > taskFileLimit {
		return nil, fmt.Errorf("artifact is not a bounded regular file")
	}
	f, e := r.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e = f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() > taskFileLimit {
		return nil, fmt.Errorf("artifact changed")
	}
	b, e := io.ReadAll(io.LimitReader(f, taskFileLimit+1))
	if len(b) > taskFileLimit {
		return nil, fmt.Errorf("artifact too large")
	}
	return b, e
}
func collectTaskArtifacts(work, dest string, inputs map[string]bool) ([]taskArtifact, error) {
	if e := os.MkdirAll(dest, 0700); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(work)
	if e != nil {
		return nil, e
	}
	out := []taskArtifact{}
	total := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "delivery.json" || name == "hire-status.json" {
			continue
		}
		kind := taskTypes[strings.ToLower(filepath.Ext(name))]
		if kind == "" || inputs[name] || !validRunFile(name) || entry.IsDir() {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("result must not be a symlink")
		}
		b, err := taskFile(work, name)
		if err != nil {
			return nil, err
		}
		if len(b) == 0 {
			continue
		}
		total += len(b)
		if total > 100<<20 || len(out) >= 32 {
			return nil, fmt.Errorf("result collection exceeds bounds")
		}
		if err = os.WriteFile(filepath.Join(dest, name), b, 0600); err != nil {
			return nil, err
		}
		out = append(out, taskArtifact{Name: name, Type: kind, Size: int64(len(b))})
	}
	if b, e := taskFile(work, "delivery.json"); e == nil {
		if err := os.WriteFile(filepath.Join(dest, "delivery.json"), b, 0600); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func copyTaskArtifacts(source, dest string) error {
	entries, e := os.ReadDir(source)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	total := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		b, err := taskFile(source, entry.Name())
		if err != nil {
			return err
		}
		total += len(b)
		if total > 100<<20 {
			return fmt.Errorf("earlier work exceeds bounds")
		}
		if err = os.WriteFile(filepath.Join(dest, entry.Name()), b, 0600); err != nil {
			return err
		}
	}
	return nil
}
func (a *app) taskDownload(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	n, e := strconv.Atoi(r.PathValue("file"))
	if !ok || j.Kind != "task" || j.Active() || e != nil {
		http.NotFound(w, r)
		return
	}
	result := a.taskResult(j)
	if n < 0 || n >= len(result.Artifacts) {
		http.NotFound(w, r)
		return
	}
	item := result.Artifacts[n]
	b, e := taskFile(filepath.Join(filepath.Dir(j.Dir), "deliverables"), item.Name)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	kind := taskTypes[strings.ToLower(filepath.Ext(item.Name))]
	if kind == "" {
		http.NotFound(w, r)
		return
	}
	disposition := "attachment"
	if r.URL.Query().Get("preview") == "1" && (strings.HasPrefix(kind, "image/") && kind != "image/svg+xml" || kind == "application/pdf") {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", kind)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": item.Name}))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(b)
}
