// A tool is a program, and the toolbox is $PATH. There is no schema, no
// registry and no manifest: a directory of executables is the capability
// set, the documentation, and the place your own programs go. Adding a tool
// is `ln -s`.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Box is the model's reach. Dir, when set, is the toolbox; Shell says the
// machine's own PATH comes too.
type Box struct {
	Dir   string // absolute path to the toolbox, or ""
	Shell bool   // -sh: inherit PATH as well
	Tools []Tool
}

// Tool is one program: its name, and the one line it says about itself.
type Tool struct {
	Name     string
	Synopsis string
}

// synopsisBytes bounds the read that looks for a script's comment line. A
// program that has not said what it is in the first 4k is not going to.
const synopsisBytes = 4 << 10

// openBox resolves the toolbox. An empty dir with -sh is the whole machine;
// an empty dir without it is a usage error the caller reports, because a
// model with no way to act is a spent API call and a confusing log.
func openBox(dir string, shell bool) (*Box, error) {
	b := &Box{Shell: shell}
	if dir == "" {
		return b, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("toolbox %s: %w", dir, err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("toolbox: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("toolbox %s is not a directory", dir)
	}
	b.Dir = abs
	ents, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("toolbox: %w", err)
	}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Stat follows the symlink, which is the point: a toolbox is
		// mostly links to programs that live elsewhere.
		fi, err := os.Stat(filepath.Join(abs, name))
		if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			continue
		}
		b.Tools = append(b.Tools, Tool{Name: name, Synopsis: firstComment(filepath.Join(abs, name))})
	}
	sort.Slice(b.Tools, func(i, j int) bool { return b.Tools[i].Name < b.Tools[j].Name })
	if len(b.Tools) == 0 && !shell {
		return nil, fmt.Errorf("toolbox %s holds no executable programs", dir)
	}
	return b, nil
}

// Path is the PATH the model's commands run with. Without -sh it is the
// toolbox alone, which is what makes a toolbox a toolbox: the model can
// reach the programs you put there and cannot name one you did not.
func (b *Box) Path() string {
	switch {
	case b.Dir == "":
		return os.Getenv("PATH")
	case b.Shell:
		return b.Dir + string(os.PathListSeparator) + os.Getenv("PATH")
	default:
		return b.Dir
	}
}

// CheckPath is the PATH the check runs with, and it is deliberately not the
// same one. The toolbox exists to aim the model; the check is the caller's
// own program, chosen by the caller, and scoping it to three symlinks would
// mean `go test ./...` needed a toolbox holding go, git and a linker.
// The toolbox still comes first, so a program in it can be the check.
func (b *Box) CheckPath() string {
	if b.Dir == "" {
		return os.Getenv("PATH")
	}
	return b.Dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

// Catalogue is level 1: the names, and whatever each program says about
// itself in one line. A program's documented interface and execution are the
// next levels; Unix does not define a universal help flag.
func (b *Box) Catalogue() string {
	var s strings.Builder
	switch {
	case b.Dir == "":
		s.WriteString("Your tools are every program on this machine: PATH is unchanged.\n")
	case b.Shell:
		s.WriteString("Your tools are every program on this machine, and these come\nfirst on PATH:\n\n")
	default:
		s.WriteString("Your tools are the programs on PATH, and PATH holds these and\nnothing else:\n\n")
	}
	width := 0
	for _, t := range b.Tools {
		if t.Synopsis != "" && len(t.Name) > width {
			width = len(t.Name)
		}
	}
	for _, t := range b.Tools {
		if t.Synopsis == "" {
			fmt.Fprintf(&s, "  %s\n", t.Name)
			continue
		}
		fmt.Fprintf(&s, "  %-*s  %s\n", width, t.Name, t.Synopsis)
	}
	if len(b.Tools) > 0 {
		s.WriteString("\nUse a program's documented read-only help form when its synopsis is not\nenough; do not guess a help flag because it may be an operand. Shell builtins\nwork as usual.\n")
	}
	return s.String()
}

// synopsis reads the line a script uses to say what it is: the first
// comment after the shebang, which is where flows and shell scripts have
// always put it. A compiled binary says nothing here and does not need to —
// the model already knows common programs.
func firstComment(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	r := bufio.NewReader(io.LimitReader(f, synopsisBytes))
	first, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(first, "#!") {
		return ""
	}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return ""
		}
		line = strings.TrimRight(line, "\n")
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || trimmed == "#":
			continue // a blank line or a bare divider is not a synopsis
		case strings.HasPrefix(trimmed, "#"):
			return clean(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
		default:
			return "" // code started; this script does not introduce itself
		}
	}
}

// clean keeps a synopsis to one printable line. It is going into a prompt
// and into `ply tools`, and a program that emits control characters in its
// first comment should not be able to shape either.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	if len(s) > 72 {
		s = strings.TrimSpace(s[:72]) + "..."
	}
	return s
}
