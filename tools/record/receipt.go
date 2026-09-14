package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os/exec"
	"sync"
)

const chunkSize = 256 << 10

// Bytes in argv and filesystem names are base64 too: Unix does not require UTF-8.
type artifact struct {
	Name  string `json:"name"`
	Path  []byte `json:"path"`
	Phase string `json:"phase"`
}
type intent struct {
	Version          int        `json:"version"`
	TimeoutNS        int64      `json:"timeout_ns"`
	GraceNS          int64      `json:"grace_ns,omitempty"`
	Argv             [][]byte   `json:"argv"`
	Cwd              []byte     `json:"cwd"`
	Executable       []byte     `json:"executable"`
	ExecutableSHA256 string     `json:"executable_sha256"`
	Artifacts        []artifact `json:"artifacts"`
	PrivateFDs       []int      `json:"private_fds"`
	Labels           []string   `json:"labels"`
}
type chunk struct {
	Stream string `json:"stream"`
	Offset int64  `json:"offset"`
	Data   []byte `json:"data"`
}
type commitment struct {
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type terminal struct {
	Streams        map[string]commitment `json:"streams"`
	Started        bool                  `json:"started"`
	Exit           int                   `json:"exit"`
	Signal         int                   `json:"signal"`
	StdinDelivered int64                 `json:"stdin_delivered"`
	StdinEOF       bool                  `json:"stdin_eof"`
	Complete       bool                  `json:"complete"`
	Problem        string                `json:"problem,omitempty"`
	StartError     string                `json:"start_error,omitempty"`
	Interrupted    bool                  `json:"interrupted"`
}
type summary struct {
	Intent   intent   `json:"intent"`
	Terminal terminal `json:"terminal"`
}

type streamDigest struct {
	h hash.Hash
	n int64
}

func newDigest() *streamDigest             { return &streamDigest{h: sha256.New()} }
func (s *streamDigest) add(b []byte)       { s.h.Write(b); s.n += int64(len(b)) }
func (s *streamDigest) commit() commitment { return commitment{s.n, hex.EncodeToString(s.h.Sum(nil))} }

type recorder struct {
	mu         sync.Mutex
	ask, file  string
	streams    map[string]*streamDigest
	err        error
	kill       func()
	writer     io.WriteCloser
	acks       *json.Decoder
	process    *exec.Cmd
	diagnostic bytes.Buffer
	lastAck    int
}

func askCommand(ask string, args []string, in io.Reader, out io.Writer) error {
	cmd := exec.Command(ask, args...)
	cmd.Stdin, cmd.Stdout = in, out
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Ask %s: %w: %s", args[0], err, diagnostic.String())
	}
	return nil
}

func (r *recorder) note(kind string, value any) error {
	if r.writer != nil {
		return r.streamNote(kind, value)
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return askCommand(r.ask, []string{"note", "-q", "-f", r.file, "-s", "record", "-k", "record." + kind + "/v1", "-json", "-", "-seal"}, bytes.NewReader(b), io.Discard)
}

func (r *recorder) capture(name string, b []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	s := r.streams[name]
	if s == nil {
		return fmt.Errorf("undeclared stream %q", name)
	}
	for len(b) > 0 {
		n := min(len(b), chunkSize)
		if err := r.note("chunk", chunk{name, s.n, b[:n]}); err != nil {
			r.err = err
			if r.kill != nil {
				r.kill()
			}
			return err
		}
		s.add(b[:n])
		b = b[n:]
	}
	return nil
}

type captureWriter struct {
	r    *recorder
	name string
	out  io.Writer
}

func (w captureWriter) Write(b []byte) (int, error) {
	if err := w.r.capture(w.name, b); err != nil {
		return 0, err
	}
	n, err := w.out.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.r.mu.Lock()
		if w.r.err == nil {
			w.r.err = fmt.Errorf("publish %s: %w", w.name, err)
		}
		if w.r.kill != nil {
			w.r.kill()
		}
		w.r.mu.Unlock()
	}
	return n, err
}
