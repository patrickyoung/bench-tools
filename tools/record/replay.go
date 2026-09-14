package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type replayData struct {
	summary
	files map[string]*os.File
	dir   string
}

func (v *replayData) close() {
	for _, f := range v.files {
		f.Close()
	}
	os.RemoveAll(v.dir)
}

func replayCLI(verb string, args []string) int {
	f := flags(verb)
	file := f.String("f", "", "recorded session")
	ask := f.String("ask", "ask", "Ask executable")
	stream := f.String("stream", "", "extract one stream")
	jsonOut := f.Bool("json", false, "verified receipt JSON")
	if err := parse(f, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 125
	}
	if *file == "" || f.NArg() != 0 || (*stream != "" && *jsonOut) || (verb == "check" && (*stream != "" || *jsonOut)) {
		return fail(errors.New("select -f SESSION and at most one of -stream or -json"))
	}
	v, err := verify(*ask, *file)
	if err != nil {
		return fail(err)
	}
	defer v.close()
	if verb == "check" {
		fmt.Fprintln(os.Stdout, "ok: complete process recording; Ask seals and all retained bytes verify")
		return 0
	}
	if *jsonOut {
		if err := json.NewEncoder(os.Stdout).Encode(v.summary); err != nil {
			return fail(err)
		}
		return 0
	}
	if *stream != "" {
		selected := v.files[*stream]
		if selected == nil {
			return fail(fmt.Errorf("unknown stream %q", *stream))
		}
		if _, err := io.Copy(os.Stdout, selected); err != nil {
			return fail(err)
		}
		return 0
	}
	// The two streams have independent order. Replaying stdout before stderr
	// does not assert an original cross-pipe scheduling or terminal ordering.
	if _, err := io.Copy(os.Stdout, v.files["stdout"]); err != nil {
		return fail(err)
	}
	if _, err := io.Copy(os.Stderr, v.files["stderr"]); err != nil {
		return fail(err)
	}
	return v.Terminal.Exit
}

func strict(raw []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}

func required(raw []byte, names ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for _, name := range names {
		if len(fields[name]) == 0 || bytes.Equal(fields[name], []byte("null")) {
			return fmt.Errorf("missing required receipt field %q", name)
		}
	}
	return nil
}

func verify(ask, file string) (v *replayData, err error) {
	dir, err := os.MkdirTemp("", "record-verify-")
	if err != nil {
		return nil, err
	}
	v = &replayData{files: map[string]*os.File{}, dir: dir}
	defer func() {
		if err != nil {
			v.close()
		}
	}()
	snapshot, err := os.CreateTemp(dir, "events-")
	if err != nil {
		return v, err
	}
	defer snapshot.Close()
	// Ask emits the very snapshot it checked. No check-then-reread of the
	// original file, and no implementation of Ask seals inside Record.
	if err = askCommand(ask, []string{"replay", "-check", "-json", file}, nil, snapshot); err != nil {
		return v, err
	}
	if _, err = snapshot.Seek(0, 0); err != nil {
		return v, err
	}
	d := json.NewDecoder(snapshot)
	digests := map[string]*streamDigest{}
	started, ended := false, false
	for {
		var event struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err = d.Decode(&event); err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return v, err
		}
		if event.Type == "session" || event.Type == "seal" {
			continue
		}
		if event.Type != "note" {
			return v, fmt.Errorf("unexpected event %q in a dedicated recording session", event.Type)
		}
		var note struct {
			Source string          `json:"source"`
			Kind   string          `json:"kind"`
			Body   json.RawMessage `json:"body"`
		}
		if err = json.Unmarshal(event.Data, &note); err != nil {
			return v, err
		}
		if note.Source != "record" || ended {
			return v, errors.New("foreign or trailing note in recording")
		}
		switch note.Kind {
		case "record.intent/v1":
			if started {
				return v, errors.New("duplicate recording intent")
			}
			if err = strict(note.Body, &v.Intent); err != nil {
				return v, err
			}
			if err = required(note.Body, "version", "argv", "cwd"); err != nil {
				return v, err
			}
			if v.Intent.Version != 1 || v.Intent.TimeoutNS < 0 || v.Intent.GraceNS < 0 || len(v.Intent.Argv) == 0 || len(v.Intent.Argv[0]) == 0 || !filepath.IsAbs(string(v.Intent.Cwd)) {
				return v, errors.New("invalid invocation identity")
			}
			names := []string{"stdin", "stdout", "stderr"}
			for i, a := range v.Intent.Artifacts {
				if a.Name != fmt.Sprintf("artifact:%d", i) || !filepath.IsAbs(string(a.Path)) || (a.Phase != "input" && a.Phase != "output" && a.Phase != "session") {
					return v, errors.New("invalid artifact declaration")
				}
				names = append(names, a.Name)
			}
			for _, name := range names {
				var f *os.File
				f, err = os.CreateTemp(dir, "stream-")
				if err != nil {
					return v, err
				}
				v.files[name], digests[name] = f, newDigest()
			}
			started = true
		case "record.chunk/v1":
			if !started {
				return v, errors.New("chunk precedes intent")
			}
			var c chunk
			if err = strict(note.Body, &c); err != nil {
				return v, err
			}
			if err = required(note.Body, "stream", "offset", "data"); err != nil {
				return v, err
			}
			s := digests[c.Stream]
			if s == nil || c.Offset != s.n || len(c.Data) == 0 || len(c.Data) > chunkSize {
				return v, errors.New("missing, reordered, empty, or undeclared stream chunk")
			}
			if err = writeAll(v.files[c.Stream], c.Data); err != nil {
				return v, err
			}
			s.add(c.Data)
		case "record.terminal/v1":
			if !started {
				return v, errors.New("terminal precedes intent")
			}
			if err = strict(note.Body, &v.Terminal); err != nil {
				return v, err
			}
			if err = required(note.Body, "streams", "started", "exit", "signal", "stdin_delivered", "stdin_eof", "complete", "interrupted"); err != nil {
				return v, err
			}
			t := v.Terminal
			if !t.Complete || t.Problem != "" {
				return v, errIncomplete
			}
			if t.Exit < 0 || t.Exit > 255 || t.Signal < 0 || t.Signal > 127 || (t.Signal > 0 && t.Exit != 128+t.Signal) || t.Started == (t.StartError != "") {
				return v, errors.New("invalid terminal outcome")
			}
			if !t.Started && (t.Exit != 126 && t.Exit != 127) {
				return v, errors.New("invalid startup failure")
			}
			if t.StdinDelivered < 0 || t.StdinDelivered > digests["stdin"].n {
				return v, errors.New("invalid delivered stdin length")
			}
			if len(t.Streams) != len(digests) {
				return v, errors.New("missing stream commitments")
			}
			for name, s := range digests {
				if t.Streams[name] != s.commit() {
					return v, fmt.Errorf("%s: retained bytes do not match terminal commitment", name)
				}
			}
			ended = true
		default:
			return v, fmt.Errorf("unsupported record kind %q", note.Kind)
		}
	}
	if !started || !ended {
		return v, errIncomplete
	}
	for name, f := range v.files {
		if _, err = f.Seek(0, 0); err != nil {
			return v, err
		}
		if strings.HasPrefix(name, "artifact:") {
			for _, a := range v.Intent.Artifacts {
				if a.Name == name && a.Phase == "session" {
					if err = askCommand(ask, []string{"replay", "-check", f.Name()}, nil, io.Discard); err != nil {
						return v, err
					}
				}
			}
		}
	}
	return v, nil
}
