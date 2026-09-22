package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"
)

const maxJSON = 2 << 20

type Command struct {
	Argv []string `json:"argv"`
}
type Commands struct {
	Propose Command `json:"propose"`
	Trial   Command `json:"trial"`
	Judge   Command `json:"judge"`
}
type Case struct {
	ID     string `json:"id"`
	Family string `json:"family"`
	File   string `json:"file"`
}
type Spec struct {
	Version        int             `json:"version"`
	Source         string          `json:"source"`
	Mutable        []string        `json:"mutable"`
	Development    []Case          `json:"development"`
	Holdout        []Case          `json:"holdout"`
	Commands       Commands        `json:"commands"`
	Dependencies   []string        `json:"dependencies"`
	Record         string          `json:"record"`
	Settings       json.RawMessage `json:"settings"`
	Repeats        int             `json:"repeats"`
	CommandSeconds int             `json:"command_seconds"`
	MaxSeconds     int             `json:"max_seconds"`
}

// Validate duplicate keys at every depth, including opaque scores and settings.
func jsonValue(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); ok {
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return fmt.Errorf("duplicate or invalid JSON key")
				}
				seen[s] = true
				if e = jsonValue(d); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e := jsonValue(d); e != nil {
					return e
				}
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		_, err = d.Token()
	}
	return err
}
func exactKeys(value any, t reflect.Type) error {
	if t == reflect.TypeOf(json.RawMessage{}) {
		return nil
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if value == nil {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name != "" && name != "-" {
				fields[name] = f.Type
			}
		}
		for k, v := range m {
			ft, ok := fields[k]
			if !ok {
				return fmt.Errorf("unknown field %q", k)
			}
			if err := exactKeys(v, ft); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if a, ok := value.([]any); ok {
			for _, v := range a {
				if err := exactKeys(v, t.Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func decode(raw []byte, out any) error {
	if len(raw) > maxJSON || !utf8.Valid(raw) {
		return fmt.Errorf("JSON exceeds size limit or is not UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := jsonValue(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	var shape any
	d = json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&shape); err != nil {
		return err
	}
	if shape == nil {
		return fmt.Errorf("expected JSON object")
	}
	if err := exactKeys(shape, reflect.TypeOf(out).Elem()); err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func cleanPath(s string) bool {
	return s != "" && s != "." && filepath.ToSlash(filepath.Clean(s)) == s && !filepath.IsAbs(s) && !strings.ContainsAny(s, "\\\x00") && !strings.HasPrefix(s, "../") && s != ".."
}
func absolute(s string) (string, error) {
	if s == "" || strings.ContainsRune(s, 0) {
		return "", fmt.Errorf("empty or invalid path")
	}
	return filepath.Abs(s)
}

// Reject symlinks in caller-selected data paths, including parent components.
func plainPath(s string) (string, error) {
	p, e := absolute(s)
	if e != nil {
		return "", e
	}
	for q := p; ; q = filepath.Dir(q) {
		i, e := os.Lstat(q)
		if e != nil {
			return "", e
		}
		if i.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink data path: %s", q)
		}
		if filepath.Dir(q) == q {
			break
		}
	}
	return p, nil
}
func executable(s string) (string, error) {
	p, e := exec.LookPath(s)
	if e != nil {
		return "", e
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		return "", e
	}
	p, e = filepath.Abs(p)
	if e != nil {
		return "", e
	}
	i, e := os.Stat(p)
	if e != nil {
		return "", e
	}
	if !i.Mode().IsRegular() || i.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("not executable: %s", p)
	}
	return p, nil
}
func validateSpec(raw []byte) (Spec, error) {
	var s Spec
	if e := decode(raw, &s); e != nil {
		return s, e
	}
	if s.Version != 1 {
		return s, fmt.Errorf("version must be 1")
	}
	if s.Repeats == 0 {
		s.Repeats = 3
	}
	if s.CommandSeconds == 0 {
		s.CommandSeconds = 180
	}
	if s.MaxSeconds == 0 {
		s.MaxSeconds = 2400
	}
	if s.Repeats < 2 || s.Repeats > 10 || s.CommandSeconds < 1 || s.CommandSeconds > 3600 || s.MaxSeconds < 1 || s.MaxSeconds > 86400 {
		return s, fmt.Errorf("invalid experiment limits")
	}
	var e error
	s.Source, e = plainPath(s.Source)
	if e != nil {
		return s, e
	}
	if _, e = tree(s.Source); e != nil {
		return s, e
	}
	if len(s.Mutable) < 1 || len(s.Mutable) > 64 {
		return s, fmt.Errorf("select 1-64 mutable existing text files")
	}
	seen := map[string]bool{}
	for _, p := range s.Mutable {
		if !cleanPath(p) || seen[p] {
			return s, fmt.Errorf("invalid or duplicate mutable path")
		}
		seen[p] = true
		b, e := readFile(filepath.Join(s.Source, p), maxJSON)
		if e != nil || !utf8.Valid(b) {
			return s, fmt.Errorf("mutable file must be bounded UTF-8: %s", p)
		}
	}
	families := map[string]bool{}
	ids := map[string]bool{}
	contents := map[string]bool{}
	for split, rows := range [][]Case{s.Development, s.Holdout} {
		if len(rows) < 1 || len(rows) > 20 {
			return s, fmt.Errorf("each split needs 1-20 cases")
		}
		for i := range rows {
			c := &rows[i]
			if c.ID == "" || c.Family == "" || len(c.ID) > 200 || len(c.Family) > 200 || ids[c.ID] {
				return s, fmt.Errorf("case identities must be unique and families nonempty")
			}
			ids[c.ID] = true
			c.File, e = plainPath(c.File)
			if e != nil {
				return s, e
			}
			b, e := readFile(c.File, maxJSON)
			if e != nil {
				return s, e
			}
			if split == 1 && (families[c.Family] || contents[hash(b)]) {
				return s, fmt.Errorf("development and holdout overlap")
			}
			if split == 0 {
				families[c.Family] = true
				contents[hash(b)] = true
			}
		}
	}
	for _, c := range []*Command{&s.Commands.Propose, &s.Commands.Trial, &s.Commands.Judge} {
		if len(c.Argv) == 0 || len(c.Argv) > 128 {
			return s, fmt.Errorf("commands need literal argv")
		}
		for _, a := range c.Argv {
			if strings.ContainsRune(a, 0) || len(a) > 8192 {
				return s, fmt.Errorf("invalid argv")
			}
		}
		c.Argv[0], e = executable(c.Argv[0])
		if e != nil {
			return s, e
		}
	}
	if s.Record == "" {
		s.Record = "record"
	}
	s.Record, e = executable(s.Record)
	if e != nil {
		return s, e
	}
	if len(s.Dependencies) > 128 {
		return s, fmt.Errorf("too many dependencies")
	}
	for i, p := range s.Dependencies {
		s.Dependencies[i], e = plainPath(p)
		if e != nil {
			return s, e
		}
		if _, e = filePin(s.Dependencies[i]); e != nil {
			return s, e
		}
	}
	if len(s.Settings) == 0 {
		s.Settings = json.RawMessage(`{}`)
	}
	if e = admitCopies(s); e != nil {
		return s, e
	}
	return s, nil
}
func workload(s Spec) int { return s.Repeats * (3*len(s.Development) + 2*len(s.Holdout)) }

// Reject workloads whose unavoidable retained source copies already exceed
// the study bound. Mutable files may shrink, so count their baseline copies
// only; every unchanged file must survive in all trials and the final export.
func admitCopies(s Spec) error {
	// Calls retain literal argv in the result and inventory. Reserve half of
	// the JSON allowance for their worst-case ledger before any execution.
	// The other half covers result metadata and diagnostics.
	ledger := 0
	for _, entry := range []struct {
		command Command
		count   int
	}{
		{s.Commands.Trial, workload(s)}, {s.Commands.Propose, 1}, {s.Commands.Judge, 2},
	} {
		largestCost := 1.7976931348623157e308
		call := Call{Directory: "development-9-19-candidate", Argv: entry.command.Argv, Exit: -2147483648, Cost: &largestCost}
		ledger += entry.count * (len(marshal(call)) + 1)
	}
	if ledger > maxJSON/2 {
		return fmt.Errorf("planned call ledger exceeds JSON allowance; shorten command arguments or reduce workload")
	}
	pins, e := tree(s.Source)
	if e != nil {
		return e
	}
	allCopies := int64(workload(s) + 3) // baseline, candidate, exported proposal
	baselineCopies := int64(1 + s.Repeats*(2*len(s.Development)+len(s.Holdout)))
	if int64(len(pins))*allCopies > artifactFiles {
		return fmt.Errorf("planned source copies exceed study file limit")
	}
	mutable := map[string]bool{}
	for _, p := range s.Mutable {
		mutable[p] = true
	}
	var bytes int64
	for p := range pins {
		i, err := os.Stat(filepath.Join(s.Source, p))
		if err != nil {
			return err
		}
		copies := allCopies
		if mutable[p] {
			copies = baselineCopies
		}
		bytes += i.Size() * copies
	}
	if bytes > artifactBytes {
		return fmt.Errorf("planned source copies exceed study byte limit")
	}
	return nil
}
