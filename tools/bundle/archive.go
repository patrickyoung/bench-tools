package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type packageArchive struct {
	manifest manifest
	files    map[string]*zip.File
	identity string
}

func openArchive(data []byte) (*packageArchive, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	a := &packageArchive{files: map[string]*zip.File{}, identity: digest(data)}
	var total uint64
	for _, f := range z.File {
		if !relative(f.Name) || f.UncompressedSize64 > maxFile || a.files[f.Name] != nil {
			return nil, fmt.Errorf("invalid archive entry: %s", f.Name)
		}
		total += f.UncompressedSize64
		if total > maxPackage || len(a.files) >= 50001 {
			return nil, errors.New("archive exceeds limits")
		}
		a.files[f.Name] = f
	}
	b, err := a.read("manifest.json")
	if err != nil {
		return nil, err
	}
	if err := decode(b, &a.manifest); err != nil {
		return nil, err
	}
	if a.manifest.Schema != 1 {
		return nil, errors.New("unsupported bundle schema")
	}
	if err := validateApp(a.manifest.App); err != nil {
		return nil, err
	}
	if len(a.files) != len(a.manifest.Files)+1 {
		return nil, errors.New("archive inventory mismatch")
	}
	seen := map[string]fileRecord{}
	for _, r := range a.manifest.Files {
		if !relative(r.Path) || (!strings.HasPrefix(r.Path, "app/") && !strings.HasPrefix(r.Path, "runtime/")) || a.files[r.Path] == nil || r.Mode & ^uint32(0777) != 0 {
			return nil, fmt.Errorf("invalid manifest path/mode: %s", r.Path)
		}
		if _, found := seen[r.Path]; found {
			return nil, fmt.Errorf("duplicate manifest path: %s", r.Path)
		}
		seen[r.Path] = r
	}
	for _, r := range a.manifest.Files {
		for parent := path.Dir(r.Path); parent != "."; parent = path.Dir(parent) {
			if _, exists := seen[parent]; exists {
				return nil, fmt.Errorf("file used as a directory: %s", parent)
			}
		}
		if r.Link != "" {
			// Only generated command links, never user-supplied source links.
			if path.Dir(r.Path) != "runtime/bin" || path.IsAbs(r.Link) {
				return nil, errors.New("invalid runtime link")
			}
			target := path.Clean(path.Join(path.Dir(r.Path), r.Link))
			dest, exists := seen[target]
			if !strings.HasPrefix(target, "runtime/lib/bench-tools/") || !exists || dest.Link != "" || dest.Mode&0111 == 0 {
				return nil, errors.New("runtime link escapes package or is not executable")
			}
		}
	}
	entry, found := seen["app/"+a.manifest.App.Entry]
	if !found || entry.Link != "" || entry.Mode&0111 == 0 {
		return nil, errors.New("missing executable entry")
	}
	return a, nil
}

func (a *packageArchive) read(name string) ([]byte, error) {
	f := a.files[name]
	if f == nil {
		return nil, fmt.Errorf("missing archive file: %s", name)
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, maxFile+1))
	if len(data) > maxFile {
		return nil, errors.New("archive file too large")
	}
	return data, err
}

func privateRoot() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(base) {
		return "", errors.New("XDG_STATE_HOME must be absolute")
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", err
	}
	base, err := physicalDir(base)
	if err != nil {
		return "", err
	}
	root := filepath.Join(base, "bundle")
	if err := secureDir(root); err != nil {
		return "", err
	}
	return root, nil
}

func secureDir(name string) error {
	if err := os.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if err := noLinks(name); err != nil {
		return err
	}
	i, err := os.Stat(name)
	if err != nil {
		return err
	}
	if !i.IsDir() || i.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("expected private directory: %s", name)
	}
	return nil
}

func packageMode(r fileRecord) os.FileMode {
	return os.FileMode(r.Mode)
}

func (a *packageArchive) extract(root string) (string, error) {
	destination := filepath.Join(root, "package")
	if _, err := os.Lstat(destination); err == nil {
		return destination, a.verify(destination)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	tmp, err := os.MkdirTemp(root, ".extract-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	for _, r := range a.manifest.Files {
		body, err := a.read(r.Path)
		if err != nil {
			return "", err
		}
		if digest(body) != r.SHA256 {
			return "", fmt.Errorf("archive digest mismatch: %s", r.Path)
		}
		name := filepath.Join(tmp, filepath.FromSlash(r.Path))
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return "", err
		}
		if r.Link != "" {
			if string(body) != r.Link {
				return "", errors.New("link payload mismatch")
			}
			if err := os.Symlink(r.Link, name); err != nil {
				return "", err
			}
		} else {
			if err := os.WriteFile(name, body, 0600); err != nil {
				return "", err
			}
			if err := os.Chmod(name, packageMode(r)); err != nil {
				return "", err
			}
		}
	}
	if err := a.verify(tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, destination); err != nil {
		return "", err
	}
	return destination, nil
}

func (a *packageArchive) verify(root string) error {
	if err := noLinks(root); err != nil {
		return err
	}
	want := map[string]fileRecord{}
	dirs := map[string]bool{".": true}
	for _, r := range a.manifest.Files {
		want[r.Path] = r
		for d := path.Dir(r.Path); d != "."; d = path.Dir(d) {
			dirs[d] = true
		}
	}
	seen := 0
	err := filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if !dirs[rel] {
				return fmt.Errorf("unexpected package directory: %s", rel)
			}
			return nil
		}
		r, found := want[rel]
		if !found {
			return fmt.Errorf("unexpected package file: %s", rel)
		}
		seen++
		if r.Link != "" {
			target, err := os.Readlink(name)
			if err != nil || target != r.Link {
				return fmt.Errorf("package link changed: %s", rel)
			}
			return nil
		}
		data, mode, err := readRegular(name)
		if err != nil {
			return err
		}
		if digest(data) != r.SHA256 || os.FileMode(mode) != packageMode(r) {
			return fmt.Errorf("package file changed: %s", rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if seen != len(want) {
		return errors.New("package files missing")
	}
	return nil
}
