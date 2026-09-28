package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

func addProtectedRoots(p *policy, paths []string) error {
	if len(paths) > 128 {
		return fmt.Errorf("at most 128 read-only paths are supported")
	}
	for _, name := range paths {
		abs, err := filepath.Abs(name)
		if err != nil {
			return err
		}
		info, err := os.Lstat(abs)
		if err != nil || (!info.Mode().IsRegular() && !info.IsDir()) {
			return fmt.Errorf("read-only path %q must be an existing regular file or directory, not a link", name)
		}
		for parent := filepath.Dir(abs); parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			physicalParent, err := filepath.EvalSymlinks(filepath.Dir(parent))
			if err != nil {
				return err
			}
			alias := filepath.Join(physicalParent, filepath.Base(parent))
			for _, writable := range p.writes {
				if pathWithin(writable, alias) {
					return fmt.Errorf("read-only path %q traverses a mutable symlink", name)
				}
			}
		}
		real, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return fmt.Errorf("read-only path %q: %w", name, err)
		}
		if pathWithin(real, p.temp) {
			return fmt.Errorf("read-only path %q contains the writable temporary directory", name)
		}
		for _, writable := range p.writes {
			if pathWithin(real, writable) {
				return fmt.Errorf("writable root %q is inside read-only path %q", writable, name)
			}
		}
		// Existing hard links could expose the same inode under an allowed name.
		// Reject links/special files; backend policy prevents new child aliases.
		count := 0
		err = filepath.WalkDir(real, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			count++
			if count > 100000 {
				return fmt.Errorf("read-only tree exceeds 100000 entries")
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !info.Mode().IsRegular() || !singleLink(info) {
				return fmt.Errorf("read-only tree contains a link or special file: %s", path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("read-only path %q: %w", name, err)
		}
		p.reads = append(p.reads, filepath.Clean(real))
	}
	p.reads = minimalRoots(p.reads)
	sort.Slice(p.reads, func(i, j int) bool { return len(p.reads[i]) < len(p.reads[j]) })
	return nil
}

// Protect the names of writable ancestors, without denying ordinary writes to
// their other children. Outside writable roots the existing policy does this.
func protectedAncestors(p policy) []string {
	var paths []string
	for _, protected := range p.reads {
		for parent := filepath.Dir(protected); parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
			for _, writable := range p.writes {
				if pathWithin(writable, parent) {
					paths = append(paths, parent)
					break
				}
			}
		}
	}
	paths = minimalRoots(paths)
	sort.Slice(paths, func(i, j int) bool { return len(paths[i]) < len(paths[j]) })
	return paths
}
