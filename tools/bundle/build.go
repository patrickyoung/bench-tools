package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const maxFile = 256 << 20
const maxPackage = 1 << 30

type archiveBuilder struct {
	files   map[string][]byte
	records map[string]fileRecord
	size    int64
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func (a *archiveBuilder) add(name string, data []byte, mode uint32, link string) error {
	if !relative(name) {
		return fmt.Errorf("invalid archive path: %q", name)
	}
	if _, found := a.records[name]; found {
		return fmt.Errorf("duplicate archive file: %s", name)
	}
	if len(a.records) >= 50000 || len(data) > maxFile || a.size+int64(len(data)) > maxPackage {
		return errors.New("package exceeds size/file limit")
	}
	if mode & ^uint32(0777) != 0 {
		return fmt.Errorf("unsupported file mode: %s", name)
	}
	r := fileRecord{Path: name, SHA256: digest(data), Mode: mode, Link: link}
	a.files[name], a.records[name] = data, r
	a.size += int64(len(data))
	return nil
}

func readRegular(name string) ([]byte, uint32, error) {
	if err := noLinks(name); err != nil {
		return nil, 0, err
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	i, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	if !i.Mode().IsRegular() || i.Size() > maxFile {
		return nil, 0, fmt.Errorf("not a bounded regular file: %s", name)
	}
	if i.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return nil, 0, fmt.Errorf("unsupported special mode: %s", name)
	}
	b, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if len(b) > maxFile {
		return nil, 0, fmt.Errorf("oversized file: %s", name)
	}
	return b, uint32(i.Mode().Perm()), err
}

func (a *archiveBuilder) file(name, source string) error {
	b, mode, err := readRegular(source)
	if err != nil {
		return err
	}
	return a.add(name, b, mode, "")
}

func validateApp(app application) error {
	if app.Schema != 1 || !identifier.MatchString(app.Name) || !relative(app.Entry) {
		return errors.New("app.json needs schema 1, a lowercase name and relative entry")
	}
	if len(app.Files) == 0 {
		return errors.New("app.json needs an explicit files list")
	}
	if app.Interface != "" && app.Interface != "goal" && app.Interface != "argv" {
		return errors.New("app.json interface must be goal or argv")
	}
	if app.Interface == "argv" && (app.Followup || app.Resume) {
		return errors.New("argv applications own their command interface; followup and resume must be false")
	}
	for _, name := range app.Requires {
		if !identifier.MatchString(name) {
			return fmt.Errorf("requirement must be a command name: %q", name)
		}
	}
	return nil
}

func build(args []string) error {
	f := flag.NewFlagSet("bundle build", flag.ContinueOnError)
	f.SetOutput(os.Stderr)
	output := f.String("o", "", "new executable path")
	prefix := f.String("runtime", "", "installed Bench prefix")
	var selected []string
	f.Func("tool", "include one independent package; repeatable", func(v string) error { selected = append(selected, v); return nil })
	if err := f.Parse(args); err != nil {
		return err
	}
	if *output == "" || *prefix == "" || f.NArg() != 1 {
		return errors.New("usage: bundle build -o APP -runtime PREFIX [-tool NAME ...] APP_DIR")
	}
	root, err := physicalDir(f.Arg(0))
	if err != nil {
		return err
	}
	runtimeRoot, err := physicalDir(*prefix)
	if err != nil {
		return err
	}
	data, _, err := readRegular(filepath.Join(root, "app.json"))
	if err != nil {
		return err
	}
	var app application
	if err := decode(data, &app); err != nil {
		return err
	}
	if err := validateApp(app); err != nil {
		return err
	}
	a := &archiveBuilder{files: map[string][]byte{}, records: map[string]fileRecord{}}
	for _, name := range app.Files {
		if !relative(name) {
			return fmt.Errorf("invalid source path %q", name)
		}
		if err := a.file("app/"+name, filepath.Join(root, filepath.FromSlash(name))); err != nil {
			return err
		}
	}
	if r, found := a.records["app/"+app.Entry]; !found || r.Mode&0111 == 0 {
		return errors.New("entry must be an executable in files")
	}
	if err := nativeExecutable(a.files["app/"+app.Entry]); err != nil {
		return fmt.Errorf("application entry %s: %w", app.Entry, err)
	}
	if err := a.runtime(runtimeRoot, selected); err != nil {
		return err
	}
	launcher, err := launcherSource()
	if err != nil {
		return err
	}
	m := manifest{Schema: 1, Builder: version, LauncherSHA256: digest(launcher), Platform: runtime.GOOS + "/" + runtime.GOARCH, App: app}
	for _, r := range a.records {
		m.Files = append(m.Files, r)
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	encoded, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	z := zip.NewWriter(&buffer)
	for _, r := range append([]fileRecord{{Path: "manifest.json"}}, m.Files...) {
		body := a.files[r.Path]
		if r.Path == "manifest.json" {
			body = encoded
		}
		h := &zip.FileHeader{Name: r.Path, Method: zip.Deflate}
		w, err := z.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := w.Write(body); err != nil {
			return err
		}
	}
	if err := z.Close(); err != nil {
		return err
	}
	return compile(*output, buffer.Bytes())
}

// Read the installer's public package receipts. Never crawl an operator home
// or follow the prefix's bin links into unrelated executables.
func (a *archiveBuilder) runtime(prefix string, names []string) error {
	root := filepath.Join(prefix, "lib", "bench-tools")
	if err := noLinks(root); err != nil {
		return err
	}
	if len(names) == 0 {
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
	}
	if len(names) == 0 {
		return errors.New("no runtime packages selected")
	}
	sort.Strings(names)
	for _, name := range names {
		if !identifier.MatchString(name) {
			return fmt.Errorf("invalid tool name: %q", name)
		}
		dir := filepath.Join(root, name)
		body, mode, err := readRegular(filepath.Join(dir, "package.json"))
		if err != nil {
			return err
		}
		var receipt struct {
			Schema       int             `json:"schema"`
			Name         string          `json:"name"`
			Commands     []string        `json:"commands"`
			Files        []fileRecord    `json:"files"`
			Source       json.RawMessage `json:"source"`
			MutablePaths []string        `json:"mutable_paths"`
		}
		if err := decode(body, &receipt); err != nil {
			return fmt.Errorf("%s receipt: %w", name, err)
		}
		if receipt.Schema != 1 || receipt.Name != name || len(receipt.Commands) == 0 {
			return fmt.Errorf("invalid receipt for %s", name)
		}
		base := "runtime/lib/bench-tools/" + name + "/"
		if err := a.add(base+"package.json", body, mode, ""); err != nil {
			return err
		}
		for _, r := range receipt.Files {
			if !relative(r.Path) || r.Link != "" {
				return fmt.Errorf("invalid receipt file %q", r.Path)
			}
			data, mode, err := readRegular(filepath.Join(dir, filepath.FromSlash(r.Path)))
			if err != nil {
				return err
			}
			if digest(data) != r.SHA256 || mode != r.Mode {
				return fmt.Errorf("package file changed: %s/%s", name, r.Path)
			}
			if err := a.add(base+r.Path, data, mode, ""); err != nil {
				return err
			}
		}
		for _, command := range receipt.Commands {
			if !identifier.MatchString(command) {
				return fmt.Errorf("invalid package command %q", command)
			}
			r, exists := a.records[base+"bin/"+command]
			if !exists || r.Mode&0111 == 0 {
				return fmt.Errorf("missing executable %s", command)
			}
			if err := nativeExecutable(a.files[r.Path]); err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			link := "../lib/bench-tools/" + name + "/bin/" + command
			if err := a.add("runtime/bin/"+command, []byte(link), 0777, link); err != nil {
				return err
			}
		}
		// Match the receipt's full inventory, including private assets. Extra
		// installed bytes must not silently change what "this package" means.
		expected := map[string]bool{"package.json": true}
		for _, r := range receipt.Files {
			expected[r.Path] = true
		}
		if err := filepath.WalkDir(dir, func(file string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dir, file)
			if err != nil {
				return err
			}
			if !expected[filepath.ToSlash(rel)] {
				return fmt.Errorf("unreceipted package file: %s", file)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// Recognize native executable headers without running supplied programs.
// Scripts keep their declared system interpreter as an external dependency.
func nativeExecutable(data []byte) error {
	if bytes.HasPrefix(data, []byte("#!")) {
		return nil
	}
	if runtime.GOOS == "darwin" {
		f, err := macho.NewFile(bytes.NewReader(data))
		if err != nil {
			return errors.New("expected a native Mach-O executable or script")
		}
		want := map[string]macho.Cpu{"amd64": macho.CpuAmd64, "arm64": macho.CpuArm64}[runtime.GOARCH]
		if f.Cpu != want || f.Type != macho.TypeExec {
			return errors.New("runtime executable platform mismatch")
		}
		return nil
	}
	if runtime.GOOS == "linux" {
		f, err := elf.NewFile(bytes.NewReader(data))
		if err != nil {
			return errors.New("expected a native ELF executable or script")
		}
		want := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[runtime.GOARCH]
		if f.Machine != want || (f.Type != elf.ET_EXEC && f.Type != elf.ET_DYN) {
			return errors.New("runtime executable platform mismatch")
		}
		return nil
	}
	return errors.New("bundle supports macOS and Linux")
}

func compile(output string, archive []byte) error {
	abs, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(abs); !os.IsNotExist(err) {
		return fmt.Errorf("output must not exist: %s", abs)
	}
	parent, err := physicalDir(filepath.Dir(abs))
	if err != nil {
		return err
	}
	abs = filepath.Join(parent, filepath.Base(abs))
	tmp, err := os.MkdirTemp(parent, ".bundle-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	entries, err := source.ReadDir(".")
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if !launcherFile(name) {
			continue
		}
		data, err := source.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0600); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "payload.zip"), archive, 0600); err != nil {
		return err
	}
	bind := "package main\nimport _ \"embed\"\n//go:embed payload.zip\nvar packed []byte\nfunc init() { payload = packed }\n"
	if err := os.WriteFile(filepath.Join(tmp, "payload_data.go"), []byte(bind), 0600); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-o", filepath.Join(tmp, "app"), ".")
	cmd.Dir, cmd.Stdout, cmd.Stderr = tmp, os.Stderr, os.Stderr
	cmd.Env = environment(map[string]string{"GOWORK": "off", "GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOFLAGS": "", "GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH, "CGO_ENABLED": "0"})
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compile launcher: %w", err)
	}
	// Exclusive publication on the same filesystem, including a racing symlink.
	if err := os.Link(filepath.Join(tmp, "app"), abs); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "bundle: built", abs)
	return nil
}

func launcherFile(name string) bool {
	return name == "go.mod" || (strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") && name != "payload_data.go")
}

// Development builds with changed launcher code must not silently share run
// identity just because their human-readable version is still -dev.
func launcherSource() ([]byte, error) {
	entries, err := source.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	for _, entry := range entries {
		if !launcherFile(entry.Name()) {
			continue
		}
		data, err := source.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		b.WriteString(entry.Name())
		b.WriteByte(0)
		b.Write(data)
		b.WriteByte(0)
	}
	return b.Bytes(), nil
}

func environment(set map[string]string, unset ...string) []string {
	removed := make(map[string]bool, len(unset))
	for _, key := range unset {
		removed[key] = true
	}
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, replace := set[key]; !replace && !removed[key] {
			env = append(env, item)
		}
	}
	for key, value := range set {
		env = append(env, key+"="+value)
	}
	return env
}
