package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

type Pin struct {
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}
type Inventory struct {
	Version int            `json:"version"`
	Files   map[string]Pin `json:"files"`
	Calls   []Call         `json:"calls"`
}

func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func marshal(v any) []byte {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	return append(b, '\n')
}
func readFile(p string, limit int64) ([]byte, error) {
	i, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() || i.Size() > limit {
		return nil, fmt.Errorf("not a bounded regular file: %s", p)
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("file grew beyond limit: %s", p)
	}
	return b, e
}
func filePin(p string) (Pin, error) {
	if _, e := plainPath(p); e != nil {
		return Pin{}, e
	}
	b, e := readFile(p, 64<<20)
	if e != nil {
		return Pin{}, e
	}
	i, e := os.Stat(p)
	if e != nil {
		return Pin{}, e
	}
	return Pin{hash(b), uint32(i.Mode().Perm())}, nil
}
func tree(root string) (map[string]Pin, error) {
	return boundedTree(root, 4096, 128<<20, "")
}

// A study contains many source copies plus process evidence. Its inventory is
// deliberately separate from the bound on one source or command directory.
const artifactFiles = 8192
const artifactBytes int64 = 512 << 20

func artifacts(root string) (map[string]Pin, error) {
	return boundedTree(root, artifactFiles, artifactBytes, "inventory.json")
}

func boundedTree(root string, maxFiles int, maxBytes int64, exclude string) (map[string]Pin, error) {
	if _, e := plainPath(root); e != nil {
		return nil, e
	}
	info, e := os.Lstat(root)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("not a real source directory")
	}
	pins := map[string]Pin{}
	var total int64
	e = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == exclude {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || d.Name() == ".git" {
			return fmt.Errorf("symlink or Git metadata in tree: %s", rel)
		}
		if d.IsDir() {
			return nil
		}
		i, e := d.Info()
		if e != nil {
			return e
		}
		total += i.Size()
		if len(pins) >= maxFiles || total > maxBytes {
			return fmt.Errorf("tree exceeds %d files or %d bytes", maxFiles, maxBytes)
		}
		pin, e := filePin(p)
		if e != nil {
			return e
		}
		pins[filepath.ToSlash(rel)] = pin
		return nil
	})
	if e == nil && len(pins) == 0 {
		e = fmt.Errorf("empty source tree")
	}
	return pins, e
}
func treeHash(pins map[string]Pin) string { return hash(marshal(pins)) }
func writeNew(path string, b []byte, mode os.FileMode) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Chmod(path, mode)
}
func save(path string, v any) error {
	raw := marshal(v)
	if len(raw) > maxJSON {
		return fmt.Errorf("JSON artifact exceeds size limit: %s", path)
	}
	return writeNew(path, raw, 0600)
}
func copyTree(from, to string, expected map[string]Pin) error {
	if e := os.Mkdir(to, 0700); e != nil {
		return e
	}
	keys := make([]string, 0, len(expected))
	for p := range expected {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		b, e := readFile(filepath.Join(from, p), 64<<20)
		if e != nil {
			return e
		}
		if hash(b) != expected[p].SHA256 {
			return fmt.Errorf("source changed during copy")
		}
		dest := filepath.Join(to, p)
		if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
			return e
		}
		if e = writeNew(dest, b, os.FileMode(expected[p].Mode)); e != nil {
			return e
		}
	}
	got, e := tree(to)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(got, expected) {
		return fmt.Errorf("copy differs from source")
	}
	return nil
}
func inside(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}
func disjoint(a, b string) bool { return !inside(a, b) && !inside(b, a) }
func outputPath(name string, selected []string) (string, error) {
	p, e := absolute(name)
	if e != nil {
		return "", e
	}
	if _, e = os.Lstat(p); !os.IsNotExist(e) {
		return "", fmt.Errorf("output must be new")
	}
	parent, e := plainPath(filepath.Dir(p))
	if e != nil {
		return "", e
	}
	p = filepath.Join(parent, filepath.Base(p))
	for _, input := range selected {
		if !disjoint(p, input) {
			return "", fmt.Errorf("output overlaps input: %s", input)
		}
	}
	return p, nil
}
