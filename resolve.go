package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func projectRoot(start string) (string, error) {
	for dir := start; ; dir = filepath.Dir(dir) {
		marker := filepath.Join(dir, ".git")
		info, err := os.Lstat(marker)
		switch {
		case err == nil && (info.IsDir() || info.Mode().IsRegular()):
			return dir, nil
		case err == nil:
			// A symlink, device, or other special entry is not a root marker.
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return "", fmt.Errorf("inspect root marker %q: %w", marker, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", &refusal{fmt.Sprintf(
				"no project root: no regular .git file or .git directory above %q", start)}
		}
	}
}

func loadInstructions(root, target string) ([]instruction, int, error) {
	dirs, err := pathFromRoot(root, target)
	if err != nil {
		return nil, 0, err
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return nil, 0, fmt.Errorf("open project root %q: %w", root, err)
	}
	var files []instruction
	total := 0
	seen := make(map[string]string)
	for _, dir := range dirs {
		for _, name := range instructionNames {
			logical := filepath.Join(dir, name)
			file, found, err := loadInstruction(rootFS, root, logical, seen)
			if err != nil {
				_ = rootFS.Close()
				return nil, 0, err
			}
			if !found {
				continue
			}
			if file.aliasOf != "" {
				files = append(files, file)
				continue
			}
			if total+len(file.data) > maxRules {
				_ = rootFS.Close()
				return nil, 0, &refusal{fmt.Sprintf(
					"instruction set is %d bytes after %q; limit is %d bytes",
					total+len(file.data), logical, maxRules)}
			}
			total += len(file.data)
			seen[file.resolved] = file.logical
			files = append(files, file)
		}
	}
	if err := rootFS.Close(); err != nil {
		return nil, 0, fmt.Errorf("close project root %q: %w", root, err)
	}
	return files, total, nil
}

func pathFromRoot(root, target string) ([]string, error) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return nil, fmt.Errorf("locate %q under project root %q: %w", target, root, err)
	}
	if escapesRoot(rel) {
		return nil, &refusal{fmt.Sprintf("directory %q escapes project root %q", target, root)}
	}
	dirs := []string{root}
	if rel == "." {
		return dirs, nil
	}
	cur := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		dirs = append(dirs, cur)
	}
	return dirs, nil
}

func loadInstruction(rootFS *os.Root, root, logical string,
	seen map[string]string) (instruction, bool, error) {
	_, err := os.Lstat(logical)
	if errors.Is(err, os.ErrNotExist) {
		return instruction{}, false, nil
	}
	if err != nil {
		return instruction{}, false, fmt.Errorf("inspect %q: %w", logical, err)
	}

	resolved, err := filepath.EvalSymlinks(logical)
	if err != nil {
		return instruction{}, false, fmt.Errorf("resolve instruction %q: %w", logical, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return instruction{}, false, fmt.Errorf("make instruction %q absolute: %w", logical, err)
	}
	if !within(root, resolved) {
		return instruction{}, false, &refusal{fmt.Sprintf(
			"instruction %q resolves outside project root to %q", logical, resolved)}
	}
	if first, ok := seen[resolved]; ok {
		return instruction{logical: logical, resolved: resolved, aliasOf: first}, true, nil
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return instruction{}, false, fmt.Errorf("locate instruction %q: %w", logical, err)
	}
	f, err := rootFS.Open(rel)
	if err != nil {
		return instruction{}, false, fmt.Errorf("open instruction %q: %w", logical, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return instruction{}, false, fmt.Errorf("inspect instruction %q: %w", logical, err)
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return instruction{}, false, &refusal{fmt.Sprintf(
			"instruction %q is not a regular file", logical)}
	}
	if info.Size() > maxRules {
		_ = f.Close()
		return instruction{}, false, &refusal{fmt.Sprintf(
			"instruction %q is %d bytes; limit is %d bytes",
			logical, info.Size(), maxRules)}
	}

	data, err := readBounded(f, maxRules)
	if err != nil {
		return instruction{}, false, fmt.Errorf("read instruction %q: %w", logical, err)
	}
	if len(data) > maxRules {
		return instruction{}, false, &refusal{fmt.Sprintf(
			"instruction %q exceeds %d bytes", logical, maxRules)}
	}
	return instruction{logical: logical, resolved: resolved, data: data}, true, nil
}

func readBounded(f *os.File, limit int) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	closeErr := f.Close()
	return data, errors.Join(readErr, closeErr)
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && !escapesRoot(rel)
}

func escapesRoot(rel string) bool {
	return rel == ".." || filepath.IsAbs(rel) ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func textOutput(files []instruction) []byte {
	var out []byte
	for _, file := range files {
		if file.aliasOf != "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out, '\n')
		}
		out = append(out, file.data...)
	}
	return out
}

func listOutput(files []instruction) []byte {
	var out []byte
	for _, file := range files {
		out = append(out, file.logical...)
		out = append(out, '\n')
	}
	return out
}
