// Command weave prints ready tasks from a finite graph and observed outcomes.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	version  = "0.1.0"
	maxBytes = 16 << 20
	maxLine  = 1 << 20
	maxTasks = 10000
	maxEdges = 100000
)

const help = `weave - print ready tasks from a graph and observed outcomes

  weave [--] tasks.jsonl < observations.jsonl > ready.jsonl
  weave help
  weave version

Tasks require id, needs (an array of task ids), and input (any JSON value).
Observations require id, task_sha256, state, and terminal evidence references.
States: queued, running, accepted, rejected, unknown. Absence means unadmitted.
task_sha256 hashes the exact task line, excluding its line terminator.
Evidence references require kind and ref; weave validates shape, not truth.

All input is validated before output. Ready tasks have no observation and
only accepted prerequisites. Original task bytes and input order are retained.
Each input is limited to 16 MiB, each line to 1 MiB, with at most 10000 tasks
or observations per stream and 100000 dependency edges. JSON nesting is
limited to 64 levels. Invalid UTF-8 and unpaired Unicode surrogates fail.
Use -- to treat a task filename literally, including help, version, or -.

Stdout is JSONL; stderr is diagnostics. Exit 0: valid projection (possibly
empty); 1: invalid data; 2: usage or operating failure. Empty is not done.
No execution, state, model, retry, or evidence verification occurs here.
`

type task struct {
	ID     string          `json:"id"`
	Needs  []string        `json:"needs"`
	Input  json.RawMessage `json:"input"`
	raw    []byte
	digest string
}

type reference struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type observation struct {
	ID       string            `json:"id"`
	Digest   string            `json:"task_sha256"`
	State    string            `json:"state"`
	Evidence []json.RawMessage `json:"evidence"`
}

type dataError struct{ error }

func invalid(format string, args ...any) error {
	return dataError{fmt.Errorf(format, args...)}
}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errout io.Writer) int {
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		if _, err := io.WriteString(out, help); err != nil {
			fmt.Fprintln(errout, "weave:", err)
			return 2
		}
		return 0
	}
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		if _, err := fmt.Fprintln(out, version); err != nil {
			fmt.Fprintln(errout, "weave:", err)
			return 2
		}
		return 0
	}
	literal := len(args) == 2 && args[0] == "--"
	if literal {
		args = args[1:]
	}
	if len(args) != 1 || (!literal && strings.HasPrefix(args[0], "-")) {
		fmt.Fprintln(errout, "usage: weave tasks.jsonl < observations.jsonl")
		return 2
	}
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintln(errout, "weave:", err)
		return 2
	}
	defer f.Close()
	ready, err := project(f, in)
	if err != nil {
		fmt.Fprintln(errout, "weave:", err)
		var bad dataError
		if errors.As(err, &bad) {
			return 1
		}
		return 2
	}
	w := bufio.NewWriter(out)
	for _, row := range ready {
		if _, err = w.Write(row); err != nil {
			break
		}
		end := "\n"
		if row[len(row)-1] == '\r' {
			// A payload CR needs its own CRLF terminator; otherwise rereading
			// the output would remove payload bytes and change the task hash.
			end = "\r\n"
		}
		if _, err = w.WriteString(end); err != nil {
			break
		}
	}
	if err == nil {
		err = w.Flush()
	}
	if err != nil {
		fmt.Fprintln(errout, "weave: stdout:", err)
		return 2
	}
	return 0
}

func lines(r io.Reader, name string) ([][]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(b) > maxBytes {
		return nil, invalid("%s exceeds %d bytes", name, maxBytes)
	}
	if len(b) == 0 {
		return nil, nil
	}
	terminated := b[len(b)-1] == '\n'
	records := bytes.Count(b, []byte{'\n'})
	if !terminated {
		records++
	}
	// Bound the slice allocation too: 16 MiB of newlines is millions of rows.
	if records > maxTasks {
		return nil, invalid("%s exceeds %d records", name, maxTasks)
	}
	rows := bytes.Split(b, []byte{'\n'})
	if terminated {
		rows = rows[:len(rows)-1]
	}
	for i, row := range rows {
		if i < len(rows)-1 || terminated {
			row = bytes.TrimSuffix(row, []byte{'\r'})
		}
		if len(row) > maxLine {
			return nil, invalid("%s line %d exceeds %d bytes", name, i+1, maxLine)
		}
		if !utf8.Valid(row) {
			return nil, invalid("%s line %d: invalid UTF-8", name, i+1)
		}
		if err := uniqueJSON(row); err != nil {
			return nil, invalid("%s line %d: %v", name, i+1, err)
		}
		rows[i] = row
	}
	return rows, nil
}

func project(tasks, observations io.Reader) ([][]byte, error) {
	rows, err := lines(tasks, "tasks")
	if err != nil {
		return nil, err
	}
	graph := make(map[string]*task, len(rows))
	ordered := make([]*task, 0, len(rows))
	edges := 0
	for i, row := range rows {
		var t task
		if err := fields(row, map[string]any{"id": &t.ID, "needs": &t.Needs, "input": &t.Input}); err != nil {
			return nil, invalid("task line %d: %v", i+1, err)
		}
		if t.ID == "" || len(t.ID) > 256 || t.Needs == nil || len(t.Input) == 0 {
			return nil, invalid("task line %d requires id (1-256 bytes), needs array, and input", i+1)
		}
		if _, exists := graph[t.ID]; exists {
			return nil, invalid("duplicate task %q", t.ID)
		}
		seen := make(map[string]bool)
		for _, id := range t.Needs {
			if id == "" || seen[id] {
				return nil, invalid("task %q has empty or duplicate dependency %q", t.ID, id)
			}
			seen[id] = true
		}
		edges += len(t.Needs)
		if edges > maxEdges {
			return nil, invalid("tasks exceed %d dependency edges", maxEdges)
		}
		t.raw = row
		t.digest = fmt.Sprintf("sha256:%x", sha256.Sum256(row))
		graph[t.ID], ordered = &t, append(ordered, &t)
	}
	// Kahn's algorithm validates cycles without a stack proportional to the graph.
	degree := make(map[string]int, len(graph))
	children := make(map[string][]string)
	queue := make([]string, 0, len(graph))
	for _, t := range ordered {
		degree[t.ID] = len(t.Needs)
		if len(t.Needs) == 0 {
			queue = append(queue, t.ID)
		}
		for _, id := range t.Needs {
			if graph[id] == nil {
				return nil, invalid("task %q needs missing task %q", t.ID, id)
			}
			children[id] = append(children[id], t.ID)
		}
	}
	for i := 0; i < len(queue); i++ {
		for _, id := range children[queue[i]] {
			degree[id]--
			if degree[id] == 0 {
				queue = append(queue, id)
			}
		}
	}
	if len(queue) != len(graph) {
		return nil, invalid("task graph contains a dependency cycle")
	}
	obsRows, err := lines(observations, "observations")
	if err != nil {
		return nil, err
	}
	states := make(map[string]string, len(obsRows))
	for i, row := range obsRows {
		var o observation
		if err := fields(row, map[string]any{"id": &o.ID, "task_sha256": &o.Digest, "state": &o.State, "evidence": &o.Evidence}); err != nil {
			return nil, invalid("observation line %d: %v", i+1, err)
		}
		t := graph[o.ID]
		if t == nil {
			return nil, invalid("observation refers to unknown task %q", o.ID)
		}
		if _, exists := states[o.ID]; exists {
			return nil, invalid("duplicate observation for %q; supply one current snapshot", o.ID)
		}
		if o.Digest != t.digest {
			return nil, invalid("observation for %q does not match exact task bytes", o.ID)
		}
		switch o.State {
		case "queued", "running":
		case "accepted", "rejected", "unknown":
			if len(o.Evidence) == 0 {
				return nil, invalid("%s observation for %q requires evidence", o.State, o.ID)
			}
		default:
			return nil, invalid("observation for %q has invalid state %q", o.ID, o.State)
		}
		for _, raw := range o.Evidence {
			var ref reference
			if err := fields(raw, map[string]any{"kind": &ref.Kind, "ref": &ref.Ref}); err != nil {
				return nil, invalid("observation for %q has invalid evidence: %v", o.ID, err)
			}
			if strings.TrimSpace(ref.Kind) == "" || strings.TrimSpace(ref.Ref) == "" {
				return nil, invalid("observation for %q has invalid evidence reference", o.ID)
			}
		}
		states[o.ID] = o.State
	}
	var ready [][]byte
	for _, t := range ordered {
		accepted := true
		for _, id := range t.Needs {
			if states[id] != "accepted" {
				accepted = false
			}
		}
		if states[t.ID] != "" && !accepted {
			return nil, invalid("admitted task %q has an unaccepted prerequisite", t.ID)
		}
		if states[t.ID] == "" && accepted {
			ready = append(ready, t.raw)
		}
	}
	return ready, nil
}

// Known field names are case-sensitive. encoding/json's struct field matching
// would otherwise interpret an extension named State as the field state.
func fields(row []byte, targets map[string]any) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(row, &obj); err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("expected a JSON object")
	}
	for name, target := range targets {
		if raw, ok := obj[name]; ok {
			if err := json.Unmarshal(raw, target); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	return nil
}

// Reject duplicate object keys before unmarshalling. Last-key-wins decoding
// would let different consumers disagree about a task or acceptance state.
func uniqueJSON(b []byte) error {
	if err := unicodeEscapes(b); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := value(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}

func value(d *json.Decoder, depth int) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	switch t {
	case json.Delim('{'):
		if depth >= 64 {
			return fmt.Errorf("JSON nesting exceeds 64 levels")
		}
		seen := make(map[string]bool)
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			k, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key is not text")
			}
			if seen[k] {
				return fmt.Errorf("duplicate JSON key %q", k)
			}
			seen[k] = true
			if err := value(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	case json.Delim('['):
		if depth >= 64 {
			return fmt.Errorf("JSON nesting exceeds 64 levels")
		}
		for d.More() {
			if err := value(d, depth+1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	return nil
}

// encoding/json replaces unpaired UTF-16 surrogates with U+FFFD. Reject them
// in every string before decoding, so distinct wire identities cannot collapse.
// The decoder still owns JSON syntax validation; this only checks escaped pairs.
func unicodeEscapes(b []byte) error {
	quoted := false
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case '"':
			quoted = !quoted
		case '\\':
			if !quoted {
				continue
			}
			if i+1 >= len(b) || b[i+1] != 'u' {
				i++ // Includes escaped quotes and literal backslashes.
				continue
			}
			r, ok := unicodeEscape(b, i)
			if !ok {
				return fmt.Errorf("invalid Unicode escape")
			}
			i += 5
			if r >= 0xD800 && r <= 0xDBFF {
				next, ok := unicodeEscape(b, i+1)
				if !ok || next < 0xDC00 || next > 0xDFFF {
					return fmt.Errorf("unpaired Unicode surrogate")
				}
				i += 6
			} else if r >= 0xDC00 && r <= 0xDFFF {
				return fmt.Errorf("unpaired Unicode surrogate")
			}
		}
	}
	return nil
}

func unicodeEscape(b []byte, start int) (uint64, bool) {
	if start+6 > len(b) || b[start] != '\\' || b[start+1] != 'u' {
		return 0, false
	}
	r, err := strconv.ParseUint(string(b[start+2:start+6]), 16, 16)
	return r, err == nil
}
