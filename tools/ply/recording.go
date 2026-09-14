package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var ErrRecording = errors.New("recording incomplete; effects may exist")

func recordDefault() string {
	if value := os.Getenv("PLY_RECORD"); value != "" {
		return value
	}
	return "record"
}

// One invocation index is an Ask session, written only through Ask. Record
// owns process receipts and file snapshots. Neither format is parsed here.
type recording struct {
	Bin, Ask, Root, Dir string
	Index               Model
	Session             string
	Sessions            []string
	Outputs             []string
	Failed              bool
}

func openRecording(o *opts, ask, goal string, input []byte) (*recording, error) {
	if *o.recordDir == "" {
		return nil, nil
	}
	bin, err := resolveShellFlag("-record", *o.recordBin)
	if err != nil {
		return nil, err
	}
	root, err := canonicalWorkDir(*o.recordDir)
	if err != nil {
		return nil, err
	}
	work, err := canonicalWorkDir(*o.dir)
	if err != nil {
		return nil, err
	}
	if underTree(root, work) || underTree(work, root) {
		return nil, errors.New("recording directory must be outside the work tree")
	}
	dir, err := os.MkdirTemp(root, "run.")
	if err != nil {
		return nil, err
	}
	r := &recording{Bin: bin, Ask: ask, Root: root, Dir: dir,
		Index: Model{Bin: ask, Session: filepath.Join(dir, "index.jsonl")}}
	// Retain the exact finite input even if Ply later spools it for the model.
	for name, data := range map[string][]byte{"goal.txt": []byte(goal), "stdin.bin": input} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			return nil, err
		}
		defer os.Remove(path)
		o.recordInputs = append(o.recordInputs, path)
	}
	// Record safely probes Ask's model-free capabilities before init. This
	// also retains explicit input bytes before any verifier or model runs.
	args := []string{"run", "-ask", ask, "-f", filepath.Join(dir, "inputs.jsonl")}
	for _, path := range o.recordInputs {
		path, err = recordingPath(work, path)
		if err != nil {
			return nil, err
		}
		args = append(args, "-input", path)
	}
	for _, path := range o.recordOutputs {
		path, err = recordingPath(work, path)
		if err != nil {
			return nil, err
		}
		r.Outputs = append(r.Outputs, path)
	}
	args = append(args, "--", "/usr/bin/true")
	if err := recordingCommand(bin, args, nil); err != nil {
		return nil, err
	}
	inputHash, err := recordingDigest(filepath.Join(dir, "inputs.jsonl"))
	if err != nil {
		return nil, err
	}
	if err := recordingCommand(ask, []string{"init", "-f", r.Index.Session}, nil); err != nil {
		return nil, err
	}
	if err := r.note("start", map[string]any{"parent": os.Getenv("PLY_RECORD_PARENT"),
		"directory": work, "inputs": filepath.Join(dir, "inputs.jsonl"), "inputs_sha256": inputHash}); err != nil {
		return nil, err
	}
	return r, nil
}

func recordingPath(work, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(work, path)
	}
	return filepath.Abs(path)
}

func recordingCommand(bin string, args []string, input io.Reader) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = input
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	if err := runCommand(ctx, cmd); err != nil {
		return fmt.Errorf("%w: %s: %v: %s", ErrRecording, bin, err, diagnostic.String())
	}
	return nil
}

func (r *recording) note(phase string, body map[string]any) error {
	body["phase"] = phase
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.Index.Record(ctx, verdictSource, "ply.recording/v1", body); err != nil {
		r.Failed = true
		return fmt.Errorf("%w: %v", ErrRecording, err)
	}
	return nil
}

func (r *recording) session(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if r.Session == path {
		return nil
	}
	if err := r.note("session", map[string]any{"path": path, "previous": r.Session}); err != nil {
		return err
	}
	r.Session = path
	r.Sessions = append(r.Sessions, path)
	return nil
}

func (r *recording) begin(role string, argv []string) (string, []string, error) {
	dir, err := os.MkdirTemp(r.Dir, role+".")
	if err != nil {
		r.Failed = true
		return "", nil, err
	}
	file := filepath.Join(dir, "session.jsonl")
	// Publish the path before execution, so crashes cannot hide an attempt.
	if err := r.note("process", map[string]any{"role": role, "path": file, "session": r.Session}); err != nil {
		return "", nil, err
	}
	args := []string{r.Bin, "run", "-ask", r.Ask, "-f", file,
		"-grace", max(0, processGrace()-100*time.Millisecond).String(),
		"-label", "role=" + role, "-label", "index=" + r.Index.Session, "--"}
	return file, append(args, argv...), nil
}

func (r *recording) complete(file string) error {
	if err := recordingCommand(r.Bin, []string{"check", "-ask", r.Ask, "-f", file}, nil); err != nil {
		r.Failed = true
		return err
	}
	digest, err := recordingDigest(file)
	if err != nil {
		r.Failed = true
		return err
	}
	return r.note("process-complete", map[string]any{"path": file, "sha256": digest})
}

func recordingDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrRecording, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: expected regular receipt", ErrRecording)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

func (r *recording) finish(code int) int {
	// Snapshots preserve this invocation's session bytes even after a
	// checkpoint resumes and appends to its live conversation later.
	args := []string{"run", "-ask", r.Ask, "-f", filepath.Join(r.Dir, "outputs.jsonl")}
	for _, path := range r.Outputs {
		args = append(args, "-output", path)
	}
	seen := map[string]bool{}
	var missing []string
	for _, path := range r.Sessions {
		if seen[path] {
			continue
		}
		seen[path] = true
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, path)
			if code == 0 || code == 2 {
				r.Failed = true
			}
			continue
		}
		args = append(args, "-session", path)
	}
	args = append(args, "--", "/usr/bin/true")
	if err := recordingCommand(r.Bin, args, nil); err != nil {
		fmt.Fprintln(os.Stderr, "ply:", err)
		r.Failed = true
	}
	digest, err := recordingDigest(filepath.Join(r.Dir, "outputs.jsonl"))
	if err != nil {
		r.Failed = true
	}
	if r.Failed {
		code = 125
	}
	if err := r.note("terminal", map[string]any{"exit": code, "complete": !r.Failed,
		"outputs": filepath.Join(r.Dir, "outputs.jsonl"), "outputs_sha256": digest, "missing_sessions": missing}); err != nil {
		fmt.Fprintln(os.Stderr, "ply:", err)
		return 125
	}
	return code
}
