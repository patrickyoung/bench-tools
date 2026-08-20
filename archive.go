package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxEventLine = 64 << 20

type event struct {
	Seq  int             `json:"seq"`
	Time time.Time       `json:"time"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
	Raw  json.RawMessage `json:"-"`
}

type session struct {
	Path   string
	Events []event
	Torn   bool
}

type header struct {
	ID      string `json:"id"`
	Version string `json:"ask"`
	Model   string `json:"model"`
	System  string `json:"system"`
	Parent  string `json:"parent,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	MediaType string          `json:"media_type,omitempty"`
	Name      string          `json:"name,omitempty"`
	Provider  string          `json:"provider,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
}

type usage struct {
	In         int     `json:"in"`
	Out        int     `json:"out"`
	Reasoning  int     `json:"reasoning,omitempty"`
	CacheRead  int     `json:"cache_read,omitempty"`
	CacheWrite int     `json:"cache_write,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
}

func (u *usage) add(v usage) {
	u.In += v.In
	u.Out += v.Out
	u.Reasoning += v.Reasoning
	u.CacheRead += v.CacheRead
	u.CacheWrite += v.CacheWrite
	u.Cost += v.Cost
}

type userData struct {
	Text   string  `json:"text,omitempty"`
	Blocks []block `json:"blocks,omitempty"`
	Source string  `json:"source,omitempty"`
}

type turn struct {
	Blocks  []block `json:"blocks"`
	Usage   usage   `json:"usage"`
	Model   string  `json:"model"`
	Stop    string  `json:"stop,omitempty"`
	Partial bool    `json:"partial,omitempty"`
	MS      int64   `json:"ms"`
}

type requestData struct {
	Model  string `json:"model"`
	Effort string `json:"effort,omitempty"`
}

type noteData struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

type retryData struct {
	Attempt int    `json:"attempt"`
	Status  int    `json:"status"`
	WaitMS  int64  `json:"wait_ms"`
	Error   string `json:"error,omitempty"`
}

type doneData struct {
	Reason string `json:"reason"`
	Error  string `json:"error,omitempty"`
}

type sessionRecord struct {
	Kind       string    `json:"kind"`
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Ask        string    `json:"ask,omitempty"`
	Model      string    `json:"model,omitempty"`
	Parent     string    `json:"parent,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	Started    time.Time `json:"started,omitempty"`
	Updated    time.Time `json:"updated,omitempty"`
	Events     int       `json:"events"`
	FirstSeq   int       `json:"first_seq,omitempty"`
	LastSeq    int       `json:"last_seq,omitempty"`
	Users      int       `json:"users"`
	Assistants int       `json:"assistants"`
	Requests   int       `json:"requests"`
	Notes      int       `json:"notes"`
	Retries    int       `json:"retries"`
	Outcome    string    `json:"outcome,omitempty"`
	Usage      usage     `json:"usage"`
	TornTail   bool      `json:"torn_tail,omitempty"`
}

type matchRecord struct {
	Kind    string    `json:"kind"`
	Session string    `json:"session"`
	Path    string    `json:"path"`
	Seq     int       `json:"seq"`
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Text    string    `json:"text"`
}

type eventRecord struct {
	Kind    string          `json:"kind"`
	Session string          `json:"session"`
	Path    string          `json:"path"`
	Event   json.RawMessage `json:"event"`
}

type errorRecord struct {
	Kind    string `json:"kind"`
	Session string `json:"session,omitempty"`
	Path    string `json:"path"`
	Error   string `json:"error"`
}

type warningRecord struct {
	Kind    string `json:"kind"`
	Session string `json:"session,omitempty"`
	Path    string `json:"path"`
	Warning string `json:"warning"`
}

type checkRecord struct {
	Kind    string `json:"kind"`
	Session string `json:"session"`
	Path    string `json:"path"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Error   string `json:"error,omitempty"`
}

type nodeRecord struct {
	Kind      string   `json:"kind"`
	ID        string   `json:"id"`
	Paths     []string `json:"paths,omitempty"`
	Model     string   `json:"model,omitempty"`
	Target    bool     `json:"target,omitempty"`
	Ambiguous bool     `json:"ambiguous,omitempty"`
}

type edgeRecord struct {
	Kind     string `json:"kind"`
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

func archiveFiles(dir string, explicit bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return nil, nil
		}
		return nil, fmt.Errorf("reading archive %s: %w", dir, err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 ||
			!strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", filepath.Join(dir, entry.Name()), err)
		}
		if info.Mode().IsRegular() {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// readSession uses one-line lookahead. Ask treats an invalid last record as
// a torn append and ignores it, while invalid JSON followed by another record
// is corruption. Delaying one parse until the next token tells those apart
// without seeking or rewriting the file.
func readSession(path string) (session, error) {
	return readSessionLimit(path, maxEventLine)
}

func readSessionLimit(path string, limit int) (session, error) {
	s := session{Path: path}
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, min(64<<10, limit)), limit)
	scan.Split(splitFullLines)
	var pending []byte
	for scan.Scan() {
		line := append([]byte(nil), scan.Bytes()...)
		if pending != nil {
			if err := appendEvent(&s, pending); err != nil {
				return s, fmt.Errorf("%s: corrupt event after seq %d: %w", path, s.lastSeq(), err)
			}
		}
		pending = line
	}
	if err := scan.Err(); err != nil {
		if pending != nil {
			if perr := appendEvent(&s, pending); perr != nil {
				return s, fmt.Errorf("%s: corrupt event after seq %d: %w", path, s.lastSeq(), perr)
			}
		}
		return s, fmt.Errorf("%s: event exceeds %d bytes: %w", path, limit, err)
	}
	if pending != nil {
		if err := appendEvent(&s, pending); err != nil {
			s.Torn = true
		}
	}
	return s, nil
}

func splitFullLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func appendEvent(s *session, line []byte) error {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil
	}
	var e event
	if err := json.Unmarshal(line, &e); err != nil {
		return err
	}
	e.Raw = append(json.RawMessage(nil), line...)
	s.Events = append(s.Events, e)
	return nil
}

func (s session) lastSeq() int {
	if len(s.Events) == 0 {
		return 0
	}
	return s.Events[len(s.Events)-1].Seq
}

func (s session) header() (header, error) {
	if len(s.Events) == 0 {
		return header{}, fmt.Errorf("%s is empty", s.Path)
	}
	e := s.Events[0]
	if e.Type != "session" {
		return header{}, fmt.Errorf("%s: first event is %q, want session", s.Path, e.Type)
	}
	var h header
	if err := json.Unmarshal(e.Data, &h); err != nil {
		return h, fmt.Errorf("%s: event %d (session): %w", s.Path, e.Seq, err)
	}
	if h.ID == "" {
		return h, fmt.Errorf("%s: session header has no id", s.Path)
	}
	return h, nil
}

func (s session) ID() string {
	h, err := s.header()
	if err == nil {
		return h.ID
	}
	return fileID(s.Path)
}

func fileID(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".jsonl")
}

func summarize(s session) (sessionRecord, error) {
	h, err := s.header()
	if err != nil {
		return sessionRecord{}, err
	}
	rec := sessionRecord{Kind: "session", ID: h.ID, Path: s.Path, Ask: h.Version,
		Model: h.Model, Parent: h.Parent, Summary: h.Summary, Events: len(s.Events), TornTail: s.Torn}
	if len(s.Events) > 0 {
		rec.Started = s.Events[0].Time
		rec.Updated = s.Events[len(s.Events)-1].Time
		rec.FirstSeq = s.Events[0].Seq
		rec.LastSeq = s.Events[len(s.Events)-1].Seq
	}
	for _, e := range s.Events {
		switch e.Type {
		case "user":
			rec.Users++
		case "assistant":
			var t turn
			if err := decodeEvent(e, &t); err != nil {
				return sessionRecord{}, err
			}
			rec.Assistants++
			rec.Usage.add(t.Usage)
		case "request":
			rec.Requests++
		case "note":
			rec.Notes++
		case "retry":
			rec.Retries++
		case "done":
			var d doneData
			if err := decodeEvent(e, &d); err != nil {
				return sessionRecord{}, err
			}
			rec.Outcome = d.Reason
		case "abort":
			rec.Outcome = "abort"
		}
	}
	return rec, nil
}

func decodeEvent(e event, dst any) error {
	if err := json.Unmarshal(e.Data, dst); err != nil {
		return fmt.Errorf("event %d (%s): %w", e.Seq, e.Type, err)
	}
	return nil
}

func errorFor(path string, s session, err error) errorRecord {
	return errorRecord{Kind: "error", Session: s.ID(), Path: path, Error: err.Error()}
}

func warningFor(s session, warning string) warningRecord {
	return warningRecord{Kind: "warning", Session: s.ID(), Path: s.Path, Warning: warning}
}

type lineageNode struct {
	ID      string
	Paths   []string
	Model   string
	Parent  string
	Summary string
}

func writeLineage(w io.Writer, targetID string, files []string) (int, error) {
	nodes := make(map[string]*lineageNode)
	records := newRecords(w)
	bad := false
	for _, path := range files {
		s, rerr := readSession(path)
		h, herr := s.header()
		if herr != nil {
			records.write(errorFor(path, s, errors.Join(rerr, herr)))
			bad = true
			continue
		}
		if rerr != nil {
			records.write(errorFor(path, s, rerr))
			bad = true
		} else if s.Torn {
			records.write(warningFor(s, "torn final line ignored"))
		}
		n := nodes[h.ID]
		if n == nil {
			n = &lineageNode{ID: h.ID, Model: h.Model, Parent: h.Parent, Summary: h.Summary}
			nodes[h.ID] = n
		} else if n.Parent != h.Parent || n.Summary != h.Summary {
			records.write(errorRecord{Kind: "error", Session: h.ID, Path: path,
				Error: "duplicate session id has conflicting lineage headers"})
			bad = true
		}
		n.Paths = append(n.Paths, path)
	}
	if nodes[targetID] == nil {
		records.write(errorRecord{Kind: "error", Session: targetID,
			Error: "target session is not present in the lineage archive"})
		if records.err != nil {
			return exitErr, records.err
		}
		return exitNo, nil
	}
	adj := make(map[string][]string)
	var edges []edgeRecord
	for _, n := range nodes {
		for relation, to := range map[string]string{"parent": n.Parent, "summary": n.Summary} {
			if to == "" {
				continue
			}
			edges = append(edges, edgeRecord{Kind: "edge", From: n.ID, To: to, Relation: relation})
			adj[n.ID] = append(adj[n.ID], to)
			adj[to] = append(adj[to], n.ID)
		}
	}
	reachable := map[string]bool{targetID: true}
	queue := []string{targetID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, next := range adj[id] {
			if !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}
	var ids []string
	for id := range reachable {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := nodes[id]
		if n == nil {
			records.write(nodeRecord{Kind: "node", ID: id, Target: id == targetID})
			continue
		}
		sort.Strings(n.Paths)
		records.write(nodeRecord{Kind: "node", ID: id, Paths: n.Paths, Model: n.Model,
			Target: id == targetID, Ambiguous: len(n.Paths) > 1})
	}
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Relation != b.Relation {
			return a.Relation < b.Relation
		}
		return a.To < b.To
	})
	for _, e := range edges {
		if reachable[e.From] || reachable[e.To] {
			records.write(e)
		}
	}
	if records.err != nil {
		return exitErr, records.err
	}
	return noIf(bad), nil
}
