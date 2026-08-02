// A session is an ask log: one JSONL file, one event per line, appended and
// never rewritten. hone reads the format rather than importing it, for the
// reason ply does — the seam between these programs is a file, and a file
// that four programs agree about is a format, not a dependency.
//
// Only what a lesson is evidence for is decoded. The reasoning blocks that
// make up most of a modern session are skipped: they are provider-opaque,
// addressed to a turn that is over, and no lesson is ever grounded in them.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type kind string

const (
	kSession   kind = "session"
	kUser      kind = "user"
	kAssistant kind = "assistant"
	kNote      kind = "note"
)

type event struct {
	Seq  int             `json:"seq"`
	Type kind            `json:"type"`
	Data json.RawMessage `json:"data"`
}

type header struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	System string `json:"system"`
}

type userData struct {
	Text   string  `json:"text,omitempty"`
	Blocks []block `json:"blocks,omitempty"`
	Source string  `json:"source,omitempty"`
}

type turn struct {
	Blocks  []block `json:"blocks"`
	Partial bool    `json:"partial,omitempty"`
}

type noteData struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

type block struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// session is a run, read back. It holds what a lesson can be grounded in
// and nothing else.
type session struct {
	Path   string
	ID     string
	Model  string
	Goal   string   // the first thing asked
	Check  string   // the command that decided, lifted out of the system prompt
	Script []string // every typescript the run produced, in order
	Notes  []noteData
}

// text pulls the readable part of a message. Attachments are named, not
// carried: a lesson about a photograph is not a photograph.
func text(bs []block, fallback string) string {
	if len(bs) == 0 {
		return fallback
	}
	var parts []string
	for _, b := range bs {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "reasoning", "opaque":
			// skipped on purpose; see the file comment
		default:
			parts = append(parts, "["+b.Type+" attachment]")
		}
	}
	return strings.Join(parts, "\n")
}

// readSession parses a session file. A torn final line is dropped, the way
// ask drops it: a crash mid-write costs the last event, not the log.
// Corruption anywhere else is an error, because a log that is not a record
// of what happened is not evidence of anything.
func readSession(path string) (*session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := &session{Path: path}
	r := bufio.NewReaderSize(f, 1<<16)
	first := true
	for {
		line, rerr := r.ReadBytes('\n')
		atEOF := rerr == io.EOF
		if rerr != nil && !atEOF {
			return nil, rerr
		}
		if len(bytes.TrimSpace(line)) > 0 {
			var e event
			if jerr := json.Unmarshal(line, &e); jerr != nil {
				if atEOF || !more(r) {
					break // torn tail
				}
				return nil, fmt.Errorf("%s: corrupt event: %w", path, jerr)
			}
			if err := s.add(e, &first); err != nil {
				return nil, err
			}
		}
		if atEOF {
			break
		}
	}
	if s.ID == "" {
		return nil, fmt.Errorf("%s: no session header; this is not an ask session", path)
	}
	return s, nil
}

func more(r *bufio.Reader) bool {
	_, err := r.Peek(1)
	return err == nil
}

func (s *session) add(e event, first *bool) error {
	switch e.Type {
	case kSession:
		var h header
		if err := json.Unmarshal(e.Data, &h); err != nil {
			return fmt.Errorf("%s: header: %w", s.Path, err)
		}
		s.ID, s.Model = h.ID, h.Model
		s.Check = checkOf(h.System)
	case kUser:
		var u userData
		if err := json.Unmarshal(e.Data, &u); err != nil {
			return fmt.Errorf("%s: user event %d: %w", s.Path, e.Seq, err)
		}
		body := text(u.Blocks, u.Text)
		if *first {
			// The first message is the goal. Everything after it is what
			// commands printed, which is the run.
			s.Goal, *first = body, false
			return nil
		}
		s.Script = append(s.Script, body)
	case kAssistant:
		var t turn
		if err := json.Unmarshal(e.Data, &t); err != nil {
			return fmt.Errorf("%s: assistant event %d: %w", s.Path, e.Seq, err)
		}
		// A partial turn was interrupted mid-stream and never happened as
		// far as the conversation is concerned. ask excludes it from the
		// fold; excluding it here is the same statement.
		_ = t
	case kNote:
		var n noteData
		if err := json.Unmarshal(e.Data, &n); err != nil {
			return fmt.Errorf("%s: note event %d: %w", s.Path, e.Seq, err)
		}
		s.Notes = append(s.Notes, n)
	}
	return nil
}

// checkOf lifts the check command out of a ply system prompt. ply tells the
// model, in these words, what will decide whether it is done:
//
//	When you stop, ply runs
//
//	    go test ./...
//
//	and the run ends only when that exits zero.
//
// The prompt is recorded verbatim in the session header, so the command
// that judged a run is in the log whether or not anything else is. It is
// read for the record only — hone never runs it. The world has moved on
// since, and a check re-run today judges today's tree, not the one the run
// produced.
func checkOf(system string) string {
	const marker = "When you stop, ply runs"
	i := strings.Index(system, marker)
	if i < 0 {
		return ""
	}
	rest := system[i+len(marker):]
	end := strings.Index(rest, "and the run ends only when")
	if end < 0 {
		return ""
	}
	var out []string
	for _, line := range strings.Split(rest[:end], "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, "\n")
}
