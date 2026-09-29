package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const version = "0.1.0"
const maxInput = 128 << 20
const maxObservations = 16 << 20
const maxParams = 2 << 20

type commandError struct {
	Code    int
	Message string
}

func (e *commandError) Error() string { return e.Message }

func readBounded(r io.Reader, maximum int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maximum {
		return nil, fmt.Errorf("input exceeds %d bytes", maximum)
	}
	return b, nil
}

// Validate JSON before decoding structs: encoding/json otherwise accepts duplicate keys.
func validateJSON(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("JSON is not valid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 48 {
			return errors.New("JSON exceeds depth limit")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, err := d.Token()
					if err != nil {
						return err
					}
					key, ok := k.(string)
					if !ok || seen[key] {
						return errors.New("duplicate or invalid JSON key")
					}
					seen[key] = true
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			default:
				return errors.New("invalid JSON delimiter")
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}

func decode(raw []byte, target any, strict bool) error {
	if err := validateJSON(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		d.DisallowUnknownFields()
	}
	return d.Decode(target)
}

type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("command output exceeds bound")
	}
	return b.Buffer.Write(p)
}

type controller struct {
	Agenda, Root, Observations, FilesRoot, EvidenceRoot string
	AllowWrite                                          bool
}

func selectedPath(path string, directory, executable bool) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("controller paths must be absolute")
	}
	actual, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(actual)
	if err != nil {
		return "", err
	}
	if directory {
		if !st.IsDir() {
			return "", errors.New("selected root must be a directory")
		}
	} else if !st.Mode().IsRegular() {
		return "", errors.New("selected path must be a regular file")
	}
	if executable && st.Mode().Perm()&0111 == 0 {
		return "", errors.New("selected Agenda command is not executable")
	}
	return actual, nil
}

func (c *controller) validate() error {
	var err error
	if c.Agenda, err = selectedPath(c.Agenda, false, true); err != nil {
		return fmt.Errorf("Agenda: %w", err)
	}
	if c.Root, err = selectedPath(c.Root, true, false); err != nil {
		return fmt.Errorf("root: %w", err)
	}
	if c.Observations != "" {
		if c.Observations, err = selectedPath(c.Observations, false, false); err != nil {
			return fmt.Errorf("observations: %w", err)
		}
	}
	if c.FilesRoot != "" {
		if c.FilesRoot, err = selectedPath(c.FilesRoot, true, false); err != nil {
			return fmt.Errorf("files-root: %w", err)
		}
	}
	if c.EvidenceRoot != "" {
		if c.EvidenceRoot, err = selectedPath(c.EvidenceRoot, true, false); err != nil {
			return fmt.Errorf("evidence-root: %w", err)
		}
		if c.EvidenceRoot == c.Root || c.EvidenceRoot == c.FilesRoot {
			return errors.New("evidence-root must be separate from work and input roots")
		}
	}
	return nil
}

func (c controller) call(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Agenda, args...)
	cmd.Stdin = bytes.NewReader(input)
	stdout, stderr := &cappedBuffer{limit: maxInput}, &cappedBuffer{limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	if err != nil {
		code := 2
		if cmd.Process != nil {
			code = 125
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
			if code < 0 {
				code = 125
			}
		}
		if ctx.Err() != nil {
			code = 125
		}
		message := strings.TrimSpace(stderr.String())
		if len(message) > 4096 {
			message = message[:4096]
		}
		if message == "" {
			message = err.Error()
		}
		return nil, &commandError{code, "Agenda: " + message}
	}
	return stdout.Bytes(), nil
}

func (c controller) observationBytes() ([]byte, error) {
	if c.Observations == "" {
		return nil, nil
	}
	f, err := os.Open(c.Observations)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readBounded(f, maxObservations)
}

func validInstant(value string) error {
	_, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return errors.New("as_of must be RFC3339 with an explicit offset")
	}
	return nil
}

func (c controller) project(ctx context.Context, asOf string) ([]byte, error) {
	if err := validInstant(asOf); err != nil {
		return nil, err
	}
	snapshot, err := c.call(ctx, nil, "export", c.Root)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Schema string `json:"schema"`
	}
	if err = decode(snapshot, &envelope, false); err != nil || envelope.Schema != "agenda.snapshot/v1" {
		return nil, errors.New("Agenda returned an invalid snapshot")
	}
	dir, err := os.MkdirTemp("", "agenda-ui-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "snapshot.json")
	if err = os.WriteFile(path, snapshot, 0600); err != nil {
		return nil, err
	}
	observations, err := c.observationBytes()
	if err != nil {
		return nil, err
	}
	return c.call(ctx, observations, "project", path, "--as-of", asOf)
}

func controllerFlags(fs *flag.FlagSet, c *controller) {
	fs.StringVar(&c.Agenda, "agenda", "", "absolute public Agenda executable")
	fs.StringVar(&c.Root, "root", "", "fixed Agenda data root")
	fs.StringVar(&c.Observations, "observations", "", "controller-selected observation JSONL file")
}

func run(ctx context.Context, args []string, in io.Reader, out, diagnostics io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: agenda-ui render|serve|mcp-manifest|mcp-dispatch|version")
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(out, version)
		return nil
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	switch args[0] {
	case "render":
		view := fs.String("view", "calendar", "calendar, kanban or ics")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("render reads one projection from stdin")
		}
		raw, err := readBounded(in, maxInput)
		if err != nil {
			return err
		}
		p, err := readProjection(raw)
		if err != nil {
			return err
		}
		return render(out, p, *view, false, 5)
	case "serve":
		var c controller
		controllerFlags(fs, &c)
		port := fs.Int("port", 8791, "loopback listener port (0 selects a free port)")
		poll := fs.Int("poll-seconds", 5, "view refresh interval (1..60)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		if err := c.validate(); err != nil {
			return err
		}
		return serve(ctx, c, *port, *poll, diagnostics)
	case "mcp-manifest":
		write := fs.Bool("allow-write", false, "include agenda_apply in the manifest")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected arguments")
		}
		return json.NewEncoder(out).Encode(manifest(*write))
	case "mcp-dispatch":
		var c controller
		controllerFlags(fs, &c)
		fs.BoolVar(&c.AllowWrite, "allow-write", false, "enable typed business changes")
		fs.StringVar(&c.FilesRoot, "files-root", "", "optional fixed root for relative evidence files")
		fs.StringVar(&c.EvidenceRoot, "evidence-root", "", "optional separate private proof staging root; required with evidence")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 || fs.Arg(0) != "tools/call" {
			return errors.New("only tools/call is supported")
		}
		if err := c.validate(); err != nil {
			return err
		}
		raw, err := readBounded(in, maxParams)
		if err != nil {
			return err
		}
		return dispatch(ctx, c, raw, out)
	default:
		return errors.New("unknown command")
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "agenda-ui:", err)
		code := 2
		var e *commandError
		if errors.As(err, &e) {
			code = e.Code
		}
		os.Exit(code)
	}
}
