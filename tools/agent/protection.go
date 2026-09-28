package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// This private invocation file is outside every worker/check writable root.
// It is transport for explicit caller selections, never worker-authored policy.
type inputProtection struct {
	Version  int      `json:"version"`
	Work     string   `json:"work"`
	State    string   `json:"state"`
	Temp     string   `json:"temp"`
	Cage     string   `json:"cage"`
	Check    string   `json:"check"`
	Network  bool     `json:"network"`
	ReadOnly []string `json:"read_only"`
}

func protectionEnabled(o options) bool { return o.protectInputs || len(o.readOnly) > 0 }

func protectedInputPath(name string, d *definition) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("empty protected input path")
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	// Canonicalize stable host aliases, but never freeze a path through an alias
	// the worker can change (such as WORK/link/file).
	for parent := filepath.Dir(abs); ; parent = filepath.Dir(parent) {
		info, e := os.Lstat(parent)
		if e != nil {
			return "", e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			physicalParent, e := filepath.EvalSymlinks(filepath.Dir(parent))
			if e != nil {
				return "", e
			}
			physicalLink := filepath.Join(physicalParent, filepath.Base(parent))
			if inside(d.Work, physicalLink) || inside(d.State, physicalLink) {
				return "", fmt.Errorf("protected input traverses a mutable symlink: %s", name)
			}
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
	if err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("protected input must contain only real directories and regular files: %s", path)
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && info.Mode().IsRegular() && st.Nlink != 1 {
			return fmt.Errorf("protected input has multiple hard links: %s", path)
		}
		return nil
	}); err != nil {
		return "", err
	}
	return abs, nil
}

func selectedReadOnly(o options, d *definition) ([]string, error) {
	selected := append([]string{}, o.readOnly...)
	if o.protectInputs {
		for _, name := range []string{"inputs", "request.md"} {
			path := filepath.Join(d.Work, name)
			if _, err := os.Lstat(path); err == nil {
				selected = append(selected, path)
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
	}
	seen := map[string]bool{}
	paths := []string{}
	for _, name := range selected {
		path, err := protectedInputPath(name, d)
		if err != nil {
			return nil, fmt.Errorf("read-only input: %w", err)
		}
		if overlap(path, d.Control) {
			return nil, fmt.Errorf("protected input must not overlap controller evidence: %s", path)
		}
		for _, runtime := range d.runtimeDirectories() {
			if inside(path, runtime) {
				return nil, fmt.Errorf("protected input contains a runtime write directory: %s", path)
			}
		}
		if !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	}
	if len(paths) > 128 {
		return nil, fmt.Errorf("at most 128 protected input paths may be selected")
	}
	sort.Strings(paths)
	return paths, nil
}

func protectionArgs(p inputProtection, command []string, existingStateOnly bool) []string {
	args := []string{}
	if p.Network {
		args = append(args, "-net")
	}
	args = append(args, "-w", p.Work)
	if !existingStateOnly {
		args = append(args, "-w", p.State)
	} else if info, err := os.Stat(p.State); err == nil && info.IsDir() {
		args = append(args, "-w", p.State)
	}
	for _, path := range p.ReadOnly {
		args = append(args, "-r", path)
	}
	return append(append(args, "--"), command...)
}

func readInputProtection(path string) (inputProtection, error) {
	var p inputProtection
	if !filepath.IsAbs(path) {
		return p, fmt.Errorf("input protection configuration must be an absolute controller path")
	}
	raw, err := readRegular(path, 1<<20)
	if err != nil {
		return p, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&p); err != nil {
		return p, err
	}
	if decoder.Decode(new(any)) != io.EOF || p.Version != 1 || p.ReadOnly == nil || len(p.ReadOnly) > 128 {
		return p, fmt.Errorf("invalid input protection configuration")
	}
	physical, err := filepath.EvalSymlinks(path)
	if err != nil || physical != path {
		return p, fmt.Errorf("input protection configuration traverses a symlink")
	}
	for _, dir := range []string{p.Work, p.State, p.Temp} {
		physical, err := realDir(dir)
		if err != nil || physical != dir || inside(dir, path) {
			return p, fmt.Errorf("input protection configuration must be outside physical writable roots")
		}
	}
	if inside(p.Work, p.Temp) || inside(p.State, p.Temp) {
		return p, fmt.Errorf("protected action temporary directory must be outside work and state")
	}
	for _, executablePath := range []string{p.Cage, p.Check} {
		if !filepath.IsAbs(executablePath) || inside(p.Work, executablePath) || inside(p.State, executablePath) || inside(p.Temp, executablePath) {
			return p, fmt.Errorf("protected execution requires pinned controller executables")
		}
		if err := executable(executablePath); err != nil {
			return p, err
		}
	}
	d := &definition{Work: p.Work, State: p.State}
	for _, selected := range p.ReadOnly {
		resolved, err := protectedInputPath(selected, d)
		if err != nil || resolved != selected || overlap(selected, p.Temp) {
			return p, fmt.Errorf("protected input binding changed: %s", selected)
		}
	}
	for _, dir := range []string{p.Work, p.State} {
		if err := hardlinks(dir); err != nil {
			return p, err
		}
	}
	return p, nil
}

func execProtected(p inputProtection, command []string) int {
	args := append([]string{p.Cage}, protectionArgs(p, command, false)...)
	if err := syscall.Exec(p.Cage, args, withEnv(os.Environ(), map[string]string{"TMPDIR": p.Temp})); err != nil {
		return problem(err, 125)
	}
	return 125
}

func protectedCheck(args []string) int {
	if len(args) != 1 {
		return problem(fmt.Errorf("protected checker requires one controller configuration"), 125)
	}
	p, err := readInputProtection(args[0])
	if err != nil {
		return problem(err, 125)
	}
	return execProtected(p, []string{p.Check})
}

// Establish the promised kernel boundary before creating caller runtime state.
func preflightProtection(p inputProtection) error {
	for _, path := range p.ReadOnly {
		if overlap(path, p.Temp) {
			return fmt.Errorf("protected input overlaps action temporary directory: %s", path)
		}
	}
	code := execute(p.Cage, protectionArgs(p, []string{"/bin/sh", "-c", ":"}, true), withEnv(os.Environ(), map[string]string{"TMPDIR": p.Temp}), nil, os.Stdout, os.Stderr, p.Work)
	if code != 0 {
		return fmt.Errorf("cannot establish input protection: Cage exited %d", code)
	}
	return nil
}
func protectedWake(o options, d *definition, selected []string) ([]byte, int, error) {
	cage, err := tool("AGENT_CAGE", "cage")
	if err != nil {
		return nil, 125, err
	}
	if inside(d.Home, cage) || inside(d.Work, cage) || inside(d.State, cage) {
		return nil, 125, fmt.Errorf("Cage executable must be outside definition and mutable roots")
	}
	parent, err := realDir(os.TempDir())
	if err != nil {
		return nil, 125, err
	}
	if inside(d.Home, parent) || inside(d.Work, parent) || inside(d.State, parent) {
		return nil, 125, fmt.Errorf("temporary directory must be outside agent home, work and state")
	}
	for _, path := range selected {
		if inside(path, parent) {
			return nil, 125, fmt.Errorf("protected input contains temporary directory: %s", path)
		}
	}
	tmp, err := os.MkdirTemp(parent, "agent-wake.")
	if err != nil {
		return nil, 125, err
	}
	defer os.RemoveAll(tmp)
	p := inputProtection{Work: d.Work, State: d.State, Temp: tmp, Cage: cage, Network: o.network, ReadOnly: selected}
	if err := preflightProtection(p); err != nil {
		return nil, 125, err
	}
	return boundedOutput(cage, protectionArgs(p, []string{filepath.Join(d.Home, "bin/wake")}, true), withEnv(os.Environ(), map[string]string{"TMPDIR": tmp}), d.Work, fileLimit)
}
