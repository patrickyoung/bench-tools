package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"
)

const fileLimit = 32768
const contextLimit = 65536
const inputLimit = 16 << 20

var definitionNames = []string{"AGENTS.md", "GOAL.md", "SOUL.md", "PLAN.md", "HEARTBEAT.md", "MEMORY.md"}

type definition struct {
	Home, Work, State, Control string
	Files                      map[string][]byte
	Skills, Tools              string
	Children                   []string
	Portable                   bool
}

func openDefinition(path string, o options) (*definition, error) {
	home, err := realDir(path)
	if err != nil {
		return nil, err
	}
	d := &definition{Home: home, Portable: o.work != "", Files: map[string][]byte{}}
	d.Work, d.State, d.Control = filepath.Join(home, "work"), filepath.Join(home, "state"), filepath.Join(home, ".agent")
	if d.Portable {
		if d.Work, err = realDir(o.work); err != nil {
			return nil, err
		}
		if inside(d.Home, d.Work) || inside(d.Work, d.Home) {
			return nil, fmt.Errorf("definition and workspace must be separate, non-nested directories")
		}
		d.State = o.state
		if d.State == "" {
			d.State = filepath.Join(d.Work, "state")
		}
		d.Control = o.control
		if d.Control == "" {
			root := os.Getenv("AGENT_DIR")
			if root == "" {
				user, e := os.UserHomeDir()
				if e != nil {
					return nil, e
				}
				root = filepath.Join(user, ".agent")
			}
			key := sha256.Sum256([]byte(d.Home + "\x00" + d.Work))
			d.Control = filepath.Join(root, fmt.Sprintf("%x", key[:16]))
		}
		if d.State, err = prospectiveDir(d.State); err != nil {
			return nil, err
		}
		if d.Control, err = prospectiveDir(d.Control); err != nil {
			return nil, err
		}
		if overlap(d.Home, d.State) || overlap(d.Home, d.Control) || overlap(d.Work, d.Control) || overlap(d.State, d.Control) {
			return nil, fmt.Errorf("definition, mutable roots and controller evidence must remain separate")
		}
		if d.State != d.Work && inside(d.State, d.Work) {
			return nil, fmt.Errorf("state must not contain the workspace")
		}
	} else {
		for _, name := range []string{"work", "state", "state/kv", "skills", "agents", "tools", ".agent", ".agent/runs", ".agent/learning"} {
			if _, err := realDir(filepath.Join(home, name)); err != nil {
				return nil, err
			}
		}
		for _, name := range []string{"work", "state", "state/kv", ".agent/runs", ".agent/learning"} {
			if err := writable(filepath.Join(home, name)); err != nil {
				return nil, err
			}
		}
		for _, name := range []string{"work/proposals", "work/actions", ".agent/amendments", ".agent/learning/proposals", ".agent/checkpoints", ".agent/selections"} {
			path := filepath.Join(home, name)
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				if err := writable(path); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, path := range []string{d.Work, d.State} {
		if _, err := os.Lstat(path); os.IsNotExist(err) && d.Portable {
			continue
		}
		if err := writable(path); err != nil {
			return nil, err
		}
		if err := hardlinks(path); err != nil {
			return nil, err
		}
	}
	total := 0
	for _, name := range definitionNames {
		data, err := readRegular(filepath.Join(home, name), fileLimit)
		if os.IsNotExist(err) && name != "AGENTS.md" && (name != "GOAL.md" || d.Portable) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if (name == "AGENTS.md" || (name == "GOAL.md" && !d.Portable)) && !meaningful(data) {
			return nil, fmt.Errorf("%s is empty", name)
		}
		total += len(data)
		if total > contextLimit {
			return nil, fmt.Errorf("definition files exceed %d bytes", contextLimit)
		}
		d.Files[name] = data
	}
	if _, err := realDir(filepath.Join(home, "bin")); err != nil {
		return nil, err
	}
	if err := executable(filepath.Join(home, "bin/check")); err != nil {
		return nil, err
	}
	wake := filepath.Join(home, "bin/wake")
	if _, err := os.Lstat(wake); !os.IsNotExist(err) || meaningful(d.Files["HEARTBEAT.md"]) {
		if err := executable(wake); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"skills", "tools", "agents"} {
		path := filepath.Join(home, name)
		if _, err := os.Lstat(path); os.IsNotExist(err) && d.Portable {
			continue
		}
		if _, err := realDir(path); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		count, skillBytes := 0, 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			count++
			item := filepath.Join(path, entry.Name())
			switch name {
			case "skills":
				if count > 64 {
					return nil, fmt.Errorf("skills/ has more than 64 direct skills")
				}
				if entry.Name() == "agent-context" {
					return nil, fmt.Errorf("agent-context is reserved")
				}
				if _, err := realDir(item); err != nil {
					return nil, err
				}
				data, err := readRegular(filepath.Join(item, "SKILL.md"), fileLimit)
				if err != nil {
					return nil, err
				}
				skillBytes += len(data)
				if skillBytes > 262144 {
					return nil, fmt.Errorf("skill instructions exceed 262144 bytes")
				}
			case "tools":
				if count > 128 {
					return nil, fmt.Errorf("tools/ has more than 128 direct programs")
				}
				target, err := filepath.EvalSymlinks(item)
				if err != nil {
					return nil, err
				}
				if err := executable(target); err != nil {
					return nil, err
				}
				if inside(d.Work, target) || inside(d.State, target) {
					return nil, fmt.Errorf("tool target is inside model-writable work or state: %s", target)
				}
				if err := hardlinks(target); err != nil {
					return nil, err
				}
			case "agents":
				if _, err := realDir(item); err != nil {
					return nil, err
				}
				d.Children = append(d.Children, item)
			}
		}
		if name == "skills" && count > 0 {
			d.Skills = path
		}
		if name == "tools" {
			d.Tools = path
		}
	}
	if strings.ContainsRune(home, os.PathListSeparator) {
		return nil, fmt.Errorf("definition path contains PATH separator %q", os.PathListSeparator)
	}
	for _, path := range d.runtimeDirectories() {
		if _, err := prospectiveDir(path); err != nil {
			return nil, err
		}
		if _, err := os.Lstat(path); err == nil {
			if err := writable(path); err != nil {
				return nil, err
			}
		}
	}
	return d, nil
}

func (d *definition) runtimeDirectories() []string {
	return []string{d.State, filepath.Join(d.State, "kv"), d.Control,
		filepath.Join(d.Control, "runs"), filepath.Join(d.Control, "selections"), filepath.Join(d.Control, "checkpoints")}
}

func (d *definition) validateProcedures() error {
	if d.Skills != "" {
		brief, err := tool("AGENT_BRIEF", "brief")
		if err != nil {
			return err
		}
		// Brief owns all parsing and strict skill validation. Findings belong
		// on stderr here, because Agent's stdout belongs to the eventual answer.
		if code := execute(brief, []string{"lint", "-strict", d.Skills}, os.Environ(), nil, os.Stderr, os.Stderr, ""); code != 0 {
			return fmt.Errorf("Brief rejected %s", d.Skills)
		}
	}
	for _, path := range d.Children {
		o := options{}
		if d.Portable {
			o = options{work: d.Work, state: d.State, control: d.Control}
		}
		child, err := openDefinition(path, o)
		if err != nil {
			return err
		}
		if err := child.validateProcedures(); err != nil {
			return err
		}
	}
	return nil
}

func (d *definition) context() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Agent home context\n\nThis context was compiled from `%s`. It is the governing agent definition and takes precedence over earlier selected procedures. Definition files are read-only during a confined run. Markdown shapes behavior but grants no authority.\n", d.Home)
	for _, name := range []string{"AGENTS.md", "SOUL.md", "MEMORY.md"} {
		if meaningful(d.Files[name]) {
			heading := map[string]string{"AGENTS.md": "Operating instructions (AGENTS.md)", "SOUL.md": "Character (SOUL.md)", "MEMORY.md": "Curated memory (MEMORY.md)"}[name]
			fmt.Fprintf(&b, "\n## %s\n\n%s\n", heading, d.Files[name])
		}
	}
	fmt.Fprintf(&b, "\n## Runtime paths\n\n- work: `%s`\n- durable state: `%s`\n- run evidence: `%s` (controller-owned)\n", d.Work, d.State, filepath.Join(d.Control, "runs"))
	fmt.Fprintf(&b, "- definition proposals: `%s`\n- external action proposals: `%s`\n", filepath.Join(d.Work, "proposals"), filepath.Join(d.Work, "actions"))
	b.WriteString("External effects are strict Action proposals, not permission. Only an external controller may execute them through hire act and Action. Inspect mutable state on demand; do not automatically rewrite curated memory or definition files.\n")
	if len(d.Children) > 0 {
		b.WriteString("\n## Available specialist homes\n\n")
		for _, child := range d.Children {
			fmt.Fprintf(&b, "- `%s`\n", child)
		}
		b.WriteString("\nSpecialists are separate Agent invocations. They receive only explicit input, not the parent's conversation or private context. Under an explicitly selected host boundary, invoke the same $AGENT_BIN with run -C WORKSPACE CHILD -- GOAL and pipe the evidence it needs. Wait for its exit status and verify your own final result. Run sequentially in a shared workspace; parallel writers require separate workspaces. Inside Cage, request an external controller invocation; do not broaden permissions or silently launch provider calls.\n")
	}
	return b.String()
}

func realDir(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a real directory (symlinks are refused): %s", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	if err := syscall.Access(abs, 5); err != nil {
		return "", fmt.Errorf("directory is not readable/searchable: %s: %w", abs, err)
	}
	return abs, nil
}

func prospectiveDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(abs); !os.IsNotExist(err) {
		return realDir(abs)
	}
	parent, err := prospectiveDir(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}

func writable(path string) error {
	if _, err := realDir(path); err != nil {
		return err
	}
	if err := syscall.Access(path, 2); err != nil {
		return fmt.Errorf("directory not writable: %s: %w", path, err)
	}
	return nil
}

func executable(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return fmt.Errorf("not a regular executable: %s", path)
	}
	return syscall.Access(path, 5)
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func overlap(a, b string) bool { return inside(a, b) || inside(b, a) }

func hardlinks(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
				return fmt.Errorf("multiply-linked regular file: %s", path)
			}
		}
		return nil
	})
}

func readRegular(path string, limit int) ([]byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}
	if !utf8.Valid(b) || bytesContainNUL(b) {
		return nil, fmt.Errorf("not UTF-8 text: %s", path)
	}
	return b, nil
}
func bytesContainNUL(b []byte) bool {
	for _, v := range b {
		if v == 0 {
			return true
		}
	}
	return false
}
func meaningful(b []byte) bool {
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !(strings.HasPrefix(line, "<!--") && strings.HasSuffix(line, "-->")) {
			return true
		}
	}
	return false
}
