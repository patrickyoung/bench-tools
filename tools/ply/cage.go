package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
)

const cageBoundaryExit = 125

var ErrConfinement = errors.New("action confinement failed")

// cageLauncher is one fixed action boundary. It is deliberately not a
// policy language: Cage gets the real workspace as its one explicit write
// root, the private TMPDIR through the ordinary process environment, and no
// network flag. Ask, May, Brief, and the verifier never use this launcher.
type cageLauncher struct {
	Bin       string
	BinSHA256 string
	Workspace string
	TempDir   string
}

func openCageLauncher(bin, workspace, temp string) (*cageLauncher, error) {
	path, err := filepath.Abs(bin)
	if err != nil {
		return nil, fmt.Errorf("resolve Cage: %w", err)
	}
	digest, err := executableDigest("Cage", path)
	if err != nil {
		return nil, err
	}
	workspace, err = canonicalDir("Cage workspace", workspace)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(temp, 0o700); err != nil {
		return nil, fmt.Errorf("Cage private temporary directory: %w", err)
	}
	if info, err := os.Lstat(temp); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("Cage private temporary directory must be a real directory, not a symlink")
	}
	if err := os.Chmod(temp, 0o700); err != nil {
		return nil, fmt.Errorf("Cage private temporary directory: %w", err)
	}
	temp, err = canonicalDir("Cage private temporary directory", temp)
	if err != nil {
		return nil, err
	}
	if pathWithin(workspace, temp) || pathWithin(temp, workspace) {
		return nil, errors.New("Cage workspace and private temporary directory must not contain one another")
	}
	if err := rejectExternalHardlinks(workspace, temp); err != nil {
		return nil, err
	}
	return &cageLauncher{Bin: path, BinSHA256: digest, Workspace: workspace, TempDir: temp}, nil
}

func (c *cageLauncher) argv(shell, script string) []string {
	return []string{c.Bin, "-w", c.Workspace, "--", shell, "-c", script}
}

func (c *cageLauncher) checkDigest() error {
	digest, err := executableDigest("Cage", c.Bin)
	if err != nil {
		return err
	}
	if digest != c.BinSHA256 {
		return errors.New("Cage executable changed during the run")
	}
	return nil
}

func executableDigest(name, path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", name, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("resolve %s: %s is not an executable regular file", name, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, f)
	closeErr := f.Close()
	if copyErr != nil {
		return "", fmt.Errorf("read %s: %w", name, copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("read %s: %w", name, closeErr)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

func canonicalDir(label, dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s: %s is not a directory", label, real)
	}
	return filepath.Clean(real), nil
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) &&
		(rel == "." || !startsDotDot(rel))
}

func startsDotDot(rel string) bool {
	return len(rel) >= 3 && rel[:2] == ".." && os.IsPathSeparator(rel[2])
}

func canonicalProspective(path string) (string, error) {
	return canonicalProspectiveDepth(path, 0)
}

func canonicalProspectiveDepth(path string, depth int) (string, error) {
	if depth > 32 {
		return "", errors.New("too many symbolic links")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if info, statErr := os.Lstat(abs); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(abs)
			if readErr != nil {
				return "", readErr
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(abs), target)
			}
			return canonicalProspectiveDepth(target, depth+1)
		}
		return filepath.EvalSymlinks(abs)
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	return canonicalLexical(abs)
}

func pathTouchesRoot(root, path string) (bool, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	volume := filepath.VolumeName(abs)
	current := volume + string(filepath.Separator)
	rest := strings.TrimPrefix(abs[len(volume):], string(filepath.Separator))
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		candidate := filepath.Join(current, part)
		if pathWithin(root, candidate) {
			return true, nil
		}
		info, statErr := os.Lstat(candidate)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(candidate)
			if readErr != nil {
				return false, readErr
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(candidate), target)
			}
			current, err = canonicalProspective(target)
			if err != nil {
				return false, err
			}
		} else if statErr == nil || os.IsNotExist(statErr) {
			current = candidate
		} else {
			return false, statErr
		}
		if pathWithin(root, current) {
			return true, nil
		}
	}
	return false, nil
}

func canonicalLexical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	probe := filepath.Dir(abs)
	var suffix []string
	for {
		parent, evalErr := filepath.EvalSymlinks(probe)
		if evalErr == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				parent = filepath.Join(parent, suffix[i])
			}
			return filepath.Join(parent, filepath.Base(abs)), nil
		}
		next := filepath.Dir(probe)
		if next == probe {
			return "", evalErr
		}
		suffix = append(suffix, filepath.Base(probe))
		probe = next
	}
}

func canonicalExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func validateCageControlPaths(workspace, temp, session, sessionOut, steering, ask, may, cage, actionShell, checkShell, ply string) error {
	workspace, err := canonicalDir("Cage workspace", workspace)
	if err != nil {
		return err
	}
	var tempRoot string
	if temp != "" {
		tempRoot, err = canonicalProspective(temp)
		if err != nil {
			return fmt.Errorf("Cage private temporary directory: %w", err)
		}
		if pathWithin(workspace, tempRoot) {
			return fmt.Errorf("Cage private temporary directory %s is inside the writable workspace; put the Ask session outside it", tempRoot)
		}
		if pathWithin(tempRoot, workspace) {
			return errors.New("Cage private temporary directory must not contain the writable workspace")
		}
	}
	targets := []struct {
		label, path string
		exists      bool
	}{
		{"Ask session", session, false},
		{"piped input spool directory", spoolDirectory(session), false},
		{"session pointer", sessionOut, false},
		{"steering file", steering, true},
		{"Ask executable", ask, true},
		{"May executable", may, true},
		{"Cage executable", cage, true},
		{"action shell executable", actionShell, true},
		{"check shell executable", checkShell, true},
		{"Ply executable", ply, true},
		{"Ask credential file", askAuthPath(), false},
	}
	if current, userErr := user.Current(); userErr == nil && current.HomeDir != "" {
		targets = append(targets, struct {
			label, path string
			exists      bool
		}{"May state", filepath.Join(current.HomeDir, ".local", "state", "may"), false})
	}
	for _, target := range targets {
		if target.path == "" {
			continue
		}
		raw, absErr := filepath.Abs(target.path)
		if absErr != nil {
			return fmt.Errorf("Cage %s: %w", target.label, absErr)
		}
		raw = filepath.Clean(raw)
		lexical, absErr := canonicalLexical(target.path)
		if absErr != nil {
			return fmt.Errorf("Cage %s: %w", target.label, absErr)
		}
		var resolved string
		if target.exists {
			resolved, err = canonicalExisting(target.path)
		} else {
			resolved, err = canonicalProspective(target.path)
		}
		if err != nil {
			return fmt.Errorf("Cage %s: %w", target.label, err)
		}
		touchesWorkspace, touchErr := pathTouchesRoot(workspace, raw)
		if touchErr != nil {
			return fmt.Errorf("Cage %s: %w", target.label, touchErr)
		}
		if touchesWorkspace || pathWithin(workspace, raw) || pathWithin(workspace, lexical) || pathWithin(workspace, resolved) {
			offending := resolved
			if pathWithin(workspace, raw) {
				offending = raw
			} else if pathWithin(workspace, lexical) {
				offending = lexical
			}
			return fmt.Errorf("Cage %s %s is inside the writable workspace; put controller state and executables outside it", target.label, offending)
		}
		touchesTemp := false
		if tempRoot != "" {
			touchesTemp, touchErr = pathTouchesRoot(tempRoot, raw)
			if touchErr != nil {
				return fmt.Errorf("Cage %s: %w", target.label, touchErr)
			}
		}
		if tempRoot != "" && (touchesTemp || pathWithin(tempRoot, raw) || pathWithin(tempRoot, lexical) || pathWithin(tempRoot, resolved)) {
			offending := resolved
			if pathWithin(tempRoot, raw) {
				offending = raw
			} else if pathWithin(tempRoot, lexical) {
				offending = lexical
			}
			return fmt.Errorf("Cage %s %s is inside the writable private temporary directory; put controller state and executables outside it", target.label, offending)
		}
	}
	return nil
}

func askAuthPath() string {
	if path := os.Getenv("ASK_AUTH_FILE"); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ask", "auth.json")
}

type inodeKey struct {
	dev uint64
	ino uint64
}

type inodeLinks struct {
	seen uint64
	want uint64
	path string
}

func rejectExternalHardlinks(roots ...string) error {
	links := make(map[inodeKey]inodeLinks)
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("inspect hard links for %s", path)
			}
			key := inodeKey{dev: uint64(stat.Dev), ino: uint64(stat.Ino)}
			record := links[key]
			record.seen++
			record.want = uint64(stat.Nlink)
			if record.path == "" {
				record.path = path
			}
			links[key] = record
			return nil
		})
		if err != nil {
			return fmt.Errorf("inspect Cage writable root %s: %w", root, err)
		}
	}
	for _, record := range links {
		if record.want > record.seen {
			return fmt.Errorf("Cage writable file %s has a hard link outside the admitted writable roots", record.path)
		}
	}
	return nil
}
