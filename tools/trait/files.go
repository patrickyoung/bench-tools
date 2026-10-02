package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"syscall"
)

const maxFile = 16 << 20
const maxTree = 64 << 20

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type content struct {
	Data []byte
	Mode os.FileMode
}
type training struct {
	Kind    string `json:"kind"`
	Scope   string `json:"scope"`
	Session string `json:"session,omitempty"`
	Skill   string `json:"skill,omitempty"`
}
type testCase struct {
	ID           string   `json:"-"`
	Purpose      string   `json:"purpose"`
	Outputs      []string `json:"outputs"`
	ExpectedExit int      `json:"expected_exit,omitempty"`
}
type packet struct {
	Files        map[string]content
	Cases        []testCase
	Training     training
	RefreshCases bool
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func overlap(a, b string) bool {
	contains := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	return contains(a, b) || contains(b, a)
}
func relative(s string) bool {
	return s != "" && s != "." && path.Clean(s) == s && !path.IsAbs(s) && s != ".." && !strings.HasPrefix(s, "../") && !strings.ContainsAny(s, "\\\x00\r\n")
}

func physicalDir(name string) (string, error) {
	i, err := os.Lstat(name)
	if err != nil {
		return "", err
	}
	if !i.IsDir() {
		return "", fmt.Errorf("expected real directory: %s", name)
	}
	a, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(a)
}

// Resolve existing ancestors without creating the missing tail. Admission must
// not modify an input packet even when the caller selects overlapping storage.
func futureDir(name string) (string, error) {
	name = filepath.Clean(name)
	if _, err := os.Lstat(name); err == nil {
		return physicalDir(name)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	parent, err := futureDir(filepath.Dir(name))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(name)), nil
}

func readTree(root string) (map[string]content, error) {
	return readSelectedTree(root, false)
}

// An os.Root anchors every operation even if a descendant is renamed while
// being read. Explicit lstat/identity checks reject links and changed names;
// Root alone intentionally permits symlinks that stay inside its anchor.
func readSelectedTree(name string, priorOutput bool) (map[string]content, error) {
	initial, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !initial.IsDir() {
		return nil, fmt.Errorf("expected real directory: %s", name)
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	opened, err := root.Lstat(".")
	if err != nil || !sameTreeStat(initial, opened) {
		return nil, fmt.Errorf("directory changed while opening: %s", name)
	}
	skipResults := false
	if priorOutput {
		info, err := root.Lstat("result.json")
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		skipResults = err == nil && info.Mode().IsRegular()
	}
	files := map[string]content{}
	observed := map[string]os.FileInfo{".": opened}
	total, entries := 0, 0
	var visit func(string, os.FileInfo) error
	visit = func(dir string, expected os.FileInfo) error {
		if err := treeParents(root, dir); err != nil {
			return err
		}
		f, err := root.OpenFile(dir, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.IsDir() || !sameTreeStat(expected, info) {
			return fmt.Errorf("directory changed while opening: %s", dir)
		}
		for {
			batch, readErr := f.ReadDir(128)
			if readErr != nil && readErr != io.EOF {
				return readErr
			}
			for _, entry := range batch {
				// Old result work is not an input: do not even stat or descend
				// into it, and do not charge it against admission size limits.
				if skipResults && dir == "." && entry.Name() == "cases" {
					continue
				}
				entries++
				if entries > 8192 {
					return errors.New("directory exceeds 8192 selected entries")
				}
				rel := path.Join(dir, entry.Name())
				info, err := root.Lstat(rel)
				if err != nil {
					return err
				}
				observed[rel] = info
				if info.IsDir() {
					if err := visit(rel, info); err != nil {
						return err
					}
					continue
				}
				file, err := readTreeFile(root, rel, info)
				if err != nil {
					return err
				}
				total += len(file.Data)
				if total > maxTree || len(files) >= 4096 {
					return errors.New("directory exceeds 4096 files, 16 MiB per file or 64 MiB total")
				}
				files[rel] = file
			}
			if readErr == io.EOF {
				break
			}
		}
		after, err := f.Stat()
		if err != nil || !sameTreeStat(expected, after) {
			return fmt.Errorf("directory changed while reading: %s", dir)
		}
		return nil
	}
	if err := visit(".", opened); err != nil {
		return nil, err
	}
	// Also catch an earlier file changing while later files were read.
	for rel, before := range observed {
		after, err := root.Lstat(rel)
		if err != nil || !sameTreeStat(before, after) {
			return nil, fmt.Errorf("selected path changed while reading: %s", rel)
		}
	}
	after, err := os.Lstat(name)
	if err != nil || !sameTreeStat(initial, after) {
		return nil, fmt.Errorf("directory changed while reading: %s", name)
	}
	return files, nil
}

func treeParents(root *os.Root, name string) error {
	for dir := path.Dir(name); ; dir = path.Dir(dir) {
		info, err := root.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("selected path traverses a non-directory or symlink: %s", dir)
		}
		if dir == "." {
			return nil
		}
	}
}

func readTreeFile(root *os.Root, name string, expected os.FileInfo) (content, error) {
	var result content
	if !expected.Mode().IsRegular() || expected.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || expected.Size() > maxFile {
		return result, fmt.Errorf("expected bounded regular file, no links: %s", name)
	}
	if err := treeParents(root, name); err != nil {
		return result, err
	}
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return result, err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() || !sameTreeStat(expected, before) {
		return result, fmt.Errorf("file changed while opening: %s", name)
	}
	b, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return result, err
	}
	after, err := f.Stat()
	if err != nil || !sameTreeStat(before, after) || int64(len(b)) != before.Size() || len(b) > maxFile {
		return result, fmt.Errorf("file changed or exceeded its limit while reading: %s", name)
	}
	selected, err := root.Lstat(name)
	if err != nil || !sameTreeStat(after, selected) {
		return result, fmt.Errorf("file path changed while reading: %s", name)
	}
	if err := treeParents(root, name); err != nil {
		return result, err
	}
	return content{Data: b, Mode: before.Mode().Perm()}, nil
}

func sameTreeStat(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) && reflect.DeepEqual(treeChangeTime(a), treeChangeTime(b))
}

// Unix ctime cannot be restored with Chtimes, unlike mtime. Ignore atime,
// which our own reads may update. Stat names differ on Darwin and Linux.
func treeChangeTime(info os.FileInfo) any {
	v := reflect.ValueOf(info.Sys())
	if v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		for _, name := range []string{"Ctim", "Ctimespec"} {
			if field := v.FieldByName(name); field.IsValid() && field.CanInterface() {
				return field.Interface()
			}
		}
	}
	return nil
}

func decode(b []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected one JSON value")
	}
	return nil
}

func admit(root, action string) (packet, error) { return admitRequest(root, action, "") }

func admitRequest(root, action, request string) (packet, error) {
	p := packet{}
	files, err := readSelectedTree(root, true)
	if err != nil {
		return p, err
	}
	// A previous Trait result can be selected directly. Its evaluation recipe
	// is reusable; its old case outputs and status are not fresh inputs.
	if _, prior := files["result.json"]; prior {
		p.RefreshCases = action == "train"
	}
	for name, file := range subtree(files, "evaluation/cases") {
		if _, exists := files["cases/"+name]; exists {
			return p, errors.New("select cases/ or evaluation/cases/, not both")
		}
		files["cases/"+name] = file
	}
	if request != "" {
		files["REQUEST.md"] = content{[]byte(request), 0644}
	}
	p.Files = files
	for name := range files {
		top, _, nested := strings.Cut(name, "/")
		switch top {
		case "REQUEST.md", "train.json", "worker.lock.json", "team.lock.json", "CAPABILITIES.md", "result.json", "EVALUATION.md", "REVIEW.md":
			if nested {
				return p, fmt.Errorf("%s must be a file", top)
			}
		case "expert", "sources", "cases", "evaluation":
			if !nested {
				return p, fmt.Errorf("%s must be a directory", top)
			}
		default:
			return p, fmt.Errorf("unexpected input %s; select REQUEST.md, expert/, sources/, cases/, train.json and source locks explicitly", name)
		}
	}
	if action != "eval" && len(bytes.TrimSpace(files["REQUEST.md"].Data)) == 0 {
		return p, errors.New("REQUEST.md is required")
	}
	if len(files["REQUEST.md"].Data) > 1<<20 {
		return p, errors.New("REQUEST.md exceeds 1 MiB")
	}
	expert := subtree(files, "expert")
	if action == "create" && len(expert) != 0 {
		return p, errors.New("create takes a new brief; use edit for an existing expert")
	}
	if action != "create" {
		if len(expert) == 0 {
			return p, errors.New("expert/ is required")
		}
		if err := cleanDefinition(expert); err != nil {
			return p, err
		}
	}
	p.Cases, err = parseCases(files)
	if err != nil {
		return p, err
	}
	if _, err := parseExecution(subtree(files, "evaluation")); err != nil {
		return p, err
	}
	if action == "train" {
		p.Training = training{Kind: "knowledge", Scope: "private"}
		if data, present := files["train.json"]; present {
			p.Training = training{}
			if err := decode(data.Data, &p.Training); err != nil {
				return p, fmt.Errorf("train.json: %w", err)
			}
		}
		t := p.Training
		if t.Scope != "general" && t.Scope != "private" {
			return p, errors.New("train scope must explicitly be general or private")
		}
		switch t.Kind {
		case "knowledge":
			if t.Session != "" || t.Skill != "" {
				return p, errors.New("knowledge training cannot claim a recovery session")
			}
		case "recovery":
			if !filepath.IsAbs(t.Session) || !validName.MatchString(t.Skill) {
				return p, errors.New("recovery training needs an absolute original session path and skill name")
			}
			i, err := os.Lstat(t.Session)
			if err != nil || !i.Mode().IsRegular() {
				return p, errors.New("recovery session must be a regular existing file; retain its original evidence dependencies")
			}
		default:
			return p, errors.New("train kind must be knowledge or recovery")
		}
		transfer, regression := false, false
		for _, c := range p.Cases {
			transfer = transfer || c.Purpose == "transfer"
			regression = regression || c.Purpose == "regression"
		}
		if len(p.Cases) > 0 && (!transfer || !regression) {
			return p, errors.New("training needs a fresh transfer case and a regression/nonapplicable case")
		}
	} else if _, present := files["train.json"]; present {
		if _, prior := files["result.json"]; !prior {
			return p, errors.New("train.json is only used by train")
		}
	}
	return p, nil
}

func cleanDefinition(files map[string]content) error {
	for name := range files {
		for _, part := range strings.Split(name, "/") {
			switch part {
			case "work", "state", ".agent", ".git", ".env", "node_modules", "__pycache__":
				return fmt.Errorf("runtime/private content is not a clean expert: %s", name)
			}
		}
	}
	if len(files["AGENTS.md"].Data) == 0 || files["bin/check"].Mode&0111 == 0 || len(files["README.md"].Data) == 0 {
		return errors.New("expert needs AGENTS.md, README.md and executable bin/check")
	}
	return nil
}

func subtree(files map[string]content, root string) map[string]content {
	selected := map[string]content{}
	for name, file := range files {
		if strings.HasPrefix(name, root+"/") {
			selected[strings.TrimPrefix(name, root+"/")] = file
		}
	}
	return selected
}

func sortedFiles(files map[string]content) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func treeDigest(files map[string]content) string {
	h := sha256.New()
	for _, name := range sortedFiles(files) {
		f := files[name]
		fmt.Fprintf(h, "%s\x00%o\x00%s\x00", name, f.Mode, digest(f.Data))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func materialize(root string, files map[string]content, readonly bool) error {
	if err := os.Mkdir(root, 0700); err != nil {
		return err
	}
	for _, name := range sortedFiles(files) {
		if !relative(name) {
			return fmt.Errorf("unsafe output file %s", name)
		}
		file := files[name]
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		mode := file.Mode
		if readonly {
			mode = 0400
			if file.Mode&0111 != 0 {
				mode = 0500
			}
		}
		if err := os.WriteFile(target, file.Data, 0600); err != nil {
			return err
		}
		if err := os.Chmod(target, mode); err != nil {
			return err
		}
	}
	return nil
}

func atomicFile(name string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".trait-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err = errors.Join(err, closeErr); err != nil {
		return err
	}
	return os.Rename(f.Name(), name)
}

func prepare(c config, p packet, programs map[string]string) (*run, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(base) {
		return nil, errors.New("XDG_STATE_HOME must be absolute")
	}
	base, err := futureDir(base)
	if err != nil {
		return nil, err
	}
	storage := filepath.Join(base, "trait", "runs")
	if overlap(storage, c.output) || overlap(storage, c.input) {
		return nil, errors.New("input/output cannot overlap Trait controller storage")
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return nil, err
	}
	base, err = physicalDir(base)
	if err != nil {
		return nil, err
	}
	for _, part := range []string{"trait", "runs"} {
		base = filepath.Join(base, part)
		if err := os.Mkdir(base, 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
		i, err := os.Lstat(base)
		if err != nil || !i.IsDir() || i.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("expected private directory %s", base)
		}
	}
	if overlap(base, c.output) || overlap(base, c.input) {
		return nil, errors.New("input/output cannot overlap Trait controller storage")
	}
	if err := os.Mkdir(c.output, 0700); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(base, c.action+"-")
	if err != nil {
		return nil, err
	}
	env, err := stableEnvironment(os.Environ())
	if err != nil {
		return nil, err
	}
	r := &run{config: c, packet: p, root: root, source: filepath.Join(root, "input"), author: filepath.Join(root, "author"), programs: programs, env: env}
	r.candidate = filepath.Join(r.author, "expert")
	if err := materialize(r.source, p.Files, true); err != nil {
		return nil, err
	}
	snapshot, err := readTree(r.source)
	if err != nil {
		return nil, err
	}
	r.sourceSHA256 = treeDigest(snapshot)
	for _, name := range []string{r.author, filepath.Join(root, "records"), filepath.Join(root, "cases")} {
		if err := os.Mkdir(name, 0700); err != nil {
			return nil, err
		}
	}
	if c.action != "create" && c.action != "eval" {
		if err := materialize(r.candidate, subtree(p.Files, "expert"), false); err != nil {
			return nil, err
		}
	}
	r.result = result{Schema: 1, Action: c.action, Status: "running", Exit: 125, Input: c.input, InputSHA256: treeDigest(p.Files), Output: c.output, Records: root, Cases: []caseResult{}, Processes: []processResult{}, Limitations: []string{
		"Structural validation does not execute generated checks or establish worker quality.",
		"Case acceptance means only that the named checks accepted these fresh outputs; expected files are visible regression evidence, not hidden tests.",
		"Agent's selected worker verifier retains its host execution contract; only actions and the independent case check use Cage. Review the supplied verifier before running it.",
		"Training changes worker instructions and skills, not model weights; no library publication or global memory update is performed.",
		"Fresh case acceptance does not establish improvement over the original worker or causal use of a changed skill; inspect the linked Agent evidence for loaded instructions.",
	}}
	b, _ := json.MarshalIndent(r.result, "", "  ")
	if err := atomicFile(filepath.Join(c.output, "result.json"), append(b, '\n')); err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr, "trait: records", root)
	return r, nil
}

func parseCases(files map[string]content) ([]testCase, error) {
	var cases []testCase
	ids := map[string]bool{}
	for name := range files {
		if strings.HasPrefix(name, "cases/") {
			parts := strings.Split(name, "/")
			if len(parts) < 3 || !validName.MatchString(parts[1]) {
				return nil, fmt.Errorf("invalid case path %s", name)
			}
			ids[parts[1]] = true
		}
	}
	if len(ids) > 32 {
		return nil, errors.New("at most 32 cases are supported")
	}
	for id := range ids {
		base := "cases/" + id + "/"
		var c testCase
		if err := decode(files[base+"case.json"].Data, &c); err != nil {
			return nil, fmt.Errorf("case %s: %w", id, err)
		}
		c.ID = id
		if c.Purpose != "general" && c.Purpose != "transfer" && c.Purpose != "regression" {
			return nil, fmt.Errorf("case %s needs purpose general, transfer or regression", id)
		}
		if len(bytes.TrimSpace(files[base+"goal.md"].Data)) == 0 || len(files[base+"goal.md"].Data) > 1<<20 {
			return nil, fmt.Errorf("case %s needs goal.md of 1 byte to 1 MiB", id)
		}
		if files[base+"check"].Mode&0111 == 0 {
			return nil, fmt.Errorf("case %s needs an executable independent check", id)
		}
		if c.ExpectedExit != 0 && c.ExpectedExit != 2 {
			return nil, fmt.Errorf("case %s expected_exit must be 0 or 2; uncertainty, interruption and infrastructure failures cannot be expected successes", id)
		}
		if (len(c.Outputs) == 0 && c.ExpectedExit == 0) || len(c.Outputs) > 64 {
			return nil, fmt.Errorf("case %s needs 1 to 64 explicit regular-file outputs; expected refusal (2) may declare none", id)
		}
		seen := map[string]bool{}
		for _, output := range c.Outputs {
			if !relative(output) || seen[output] {
				return nil, fmt.Errorf("invalid/duplicate output %q", output)
			}
			seen[output] = true
		}
		cases = append(cases, c)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	return cases, nil
}
