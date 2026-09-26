package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Ply owns guidance consumption. Agent only pins the selected file outside its
// mutable action roots; no steering text becomes arguments or authority.
func steeringPath(path string, d *definition) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// Check each ancestor too: a path through a mutable symlink must not become
	// controller input merely because the symlink currently points outside it.
	for parent := filepath.Dir(abs); ; parent = filepath.Dir(parent) {
		physical, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return "", fmt.Errorf("steering directory: %w", err)
		}
		if inside(d.Work, physical) || inside(d.State, physical) {
			return "", fmt.Errorf("steering file must be outside mutable work and state")
		}
		if filepath.Dir(parent) == parent {
			break
		}
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	abs = filepath.Join(parent, filepath.Base(abs))
	fd, err := syscall.Open(abs, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", fmt.Errorf("open steering file: %w", err)
	}
	f := os.NewFile(uintptr(fd), abs)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("steering path is not a regular file")
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
		return "", fmt.Errorf("steering file must not have multiple hard links")
	}
	return abs, nil
}
