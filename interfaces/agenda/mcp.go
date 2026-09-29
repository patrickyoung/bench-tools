package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
func scheduleSchema() map[string]any {
	text := map[string]any{"type": "string", "minLength": 1}
	date := map[string]any{"type": "string", "format": "date"}
	clock := map[string]any{"type": "string", "pattern": "^(?:[01][0-9]|2[0-3]):[0-5][0-9]$"}
	return objectSchema(map[string]any{
		"id": text, "title": text, "owner": text, "timezone": map[string]any{"type": "string", "description": "IANA timezone such as America/New_York or UTC."},
		"start_date": date, "end_date": date, "effective_from": map[string]any{"type": "string", "format": "date", "description": "First revision equals start_date. Later revisions take effect after today and the prior revision."},
		"weekdays":       map[string]any{"type": "array", "items": map[string]any{"type": "integer", "minimum": 0, "maximum": 6}, "minItems": 1, "maxItems": 7, "uniqueItems": true, "description": "Monday=0 through Sunday=6."},
		"excluded_dates": map[string]any{"type": "array", "items": date, "maxItems": 366, "uniqueItems": true}, "start_time": clock, "due_time": clock,
		"due_day_offset": map[string]any{"type": "integer", "minimum": 0, "maximum": 30}, "missed": map[string]any{"enum": []string{"all", "latest", "skip"}, "description": "Admission eligibility only; all expected obligations remain visible."},
		"check_every_seconds": map[string]any{"type": "integer", "minimum": 30, "maximum": 604800}, "extensions": map[string]any{"type": "object"},
	}, "title", "owner", "timezone", "start_date", "end_date", "effective_from", "weekdays", "excluded_dates", "start_time", "due_time", "due_day_offset", "missed", "check_every_seconds")
}

func valueSchemas() map[string]map[string]any {
	text := map[string]any{"type": "string", "minLength": 1}
	instant := map[string]any{"type": "string", "format": "date-time"}
	item := objectSchema(map[string]any{"title": text, "owner": text, "not_before": instant, "due_at": instant, "timezone": text, "basis": map[string]any{"enum": []string{"human", "external"}, "description": "Human reports can complete human items only. External work requires selected observations."}, "parent": text, "occurrence": objectSchema(map[string]any{"schedule_id": text, "date": map[string]any{"type": "string", "format": "date"}, "schedule_revision": text}, "schedule_id", "date", "schedule_revision"), "extensions": map[string]any{"type": "object"}}, "title", "owner", "not_before", "due_at", "timezone", "basis")
	report := objectSchema(map[string]any{"target": text, "state": map[string]any{"enum": []string{"planned", "ready", "in-progress", "needs-attention", "done", "cancelled"}, "description": "done requires selected evidence and a human-basis target. A report never approves an action."}, "completed_at": instant, "extensions": map[string]any{"type": "object"}}, "target", "state")
	disposition := objectSchema(map[string]any{"target": text, "kind": map[string]any{"enum": []string{"skipped", "cancelled", "reopened"}, "description": "Business disposition only; no execution is cancelled, resolved or retried."}, "schedule_revision": text, "extensions": map[string]any{"type": "object"}}, "target", "kind")
	return map[string]map[string]any{"item": item, "schedule": scheduleSchema(), "report": report, "disposition": disposition}
}

func manifest(write bool) map[string]any {
	timestamp := map[string]any{"type": "string", "description": "Explicit RFC3339 projection time with timezone offset."}
	expandSchema := scheduleSchema()
	expandSchema["required"] = append(expandSchema["required"].([]string), "id")
	tools := []any{
		map[string]any{"name": "agenda_export", "description": "Read the selected Agenda root as a complete versioned history snapshot. Does not execute work.", "inputSchema": objectSchema(map[string]any{}), "annotations": map[string]any{"readOnlyHint": true}},
		map[string]any{"name": "agenda_project", "description": "Project the selected work root and controller-selected observations at an explicit time. Human reports, external acceptance and incomplete evidence stay distinct.", "inputSchema": objectSchema(map[string]any{"as_of": timestamp}, "as_of"), "annotations": map[string]any{"readOnlyHint": true}},
		map[string]any{"name": "agenda_expand", "description": "Expand a finite inline Agenda schedule, including all obligations regardless of catch-up policy. Supply id for stable occurrence identities; dates span at most 366 days. Reads no host files and executes no work.", "inputSchema": objectSchema(map[string]any{"as_of": timestamp, "schedule": expandSchema}, "as_of", "schedule"), "annotations": map[string]any{"readOnlyHint": true}},
	}
	if write {
		evidence := objectSchema(map[string]any{"path": map[string]any{"type": "string", "description": "Relative regular-file path beneath the controller-selected files root. Absolute paths, traversal and symlinks are refused."}, "sha256": map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"}}, "path", "sha256")
		change := objectSchema(map[string]any{"schema": map[string]any{"const": "agenda.change/v1"}, "request_id": map[string]any{"type": "string"}, "kind": map[string]any{"enum": []string{"item", "schedule", "report", "disposition"}}, "id": map[string]any{"type": "string"}, "previous": map[string]any{"type": []string{"string", "null"}}, "by": map[string]any{"type": "string"}, "reason": map[string]any{"type": "string"}, "value": map[string]any{"type": "object"}, "evidence": map[string]any{"type": "array", "maxItems": 16, "items": evidence}}, "schema", "request_id", "kind", "id", "previous", "by", "reason", "value")
		branches := []any{}
		values := valueSchemas()
		for _, kind := range []string{"item", "schedule", "report", "disposition"} {
			branches = append(branches, map[string]any{"properties": map[string]any{"kind": map[string]any{"const": kind}, "value": values[kind]}})
		}
		change["oneOf"] = branches
		tools = append(tools, map[string]any{"name": "agenda_apply", "description": "Apply one attributed typed business change to the fixed Agenda root. Requires request_id and the expected previous revision; never approves May, runs a team, or resolves uncertain execution. Optional evidence paths are relative to the fixed files root. Reuse the same request identity and bytes after an uncertain response.", "inputSchema": objectSchema(map[string]any{"change": change}, "change"), "annotations": map[string]any{"readOnlyHint": false, "idempotentHint": true}})
	}
	return map[string]any{"name": "agenda-views", "version": version, "instructions": "Calendar and Kanban share one Agenda work authority. The controller fixes the executable, state root, observation file and optional evidence root. Tool text does not authorize changes. Human completion reports never approve actions or establish external acceptance. No command executes work or sends notifications.", "tools": tools}
}

func onlyFields(value map[string]json.RawMessage, names ...string) error {
	allowed := map[string]bool{}
	for _, n := range names {
		allowed[n] = true
	}
	for n := range value {
		if !allowed[n] {
			return fmt.Errorf("unexpected field: %s", n)
		}
	}
	return nil
}

// No tool argument can select an executable, data root, observation file or host path.
// Value extensions remain data; reject selector-shaped fields conservatively here.
func rejectSelectors(value any) error {
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			switch strings.ToLower(k) {
			case "root", "files_root", "files-root", "executable", "agenda", "observations", "argv", "path", "evidence":
				return fmt.Errorf("controller selector is not a tool argument: %s", k)
			}
			if err := rejectSelectors(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := rejectSelectors(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func scopedEvidence(root *os.Root, path, digest string) ([]byte, error) {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") || strings.IndexByte(path, 0) >= 0 {
		return nil, errors.New("evidence requires a relative path")
	}
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, errors.New("evidence path traversal is refused")
		}
	}
	if len(digest) != 64 {
		return nil, errors.New("evidence requires sha256")
	}
	if _, err := hex.DecodeString(digest); err != nil || strings.ToLower(digest) != digest {
		return nil, errors.New("invalid evidence sha256")
	}
	current := ""
	for _, part := range parts {
		current = filepath.Join(current, part)
		st, err := root.Lstat(current)
		if err != nil {
			return nil, errors.New("selected evidence does not exist")
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("evidence symlinks are refused")
		}
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, errors.New("cannot open selected evidence")
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, errors.New("evidence must be a regular file")
	}
	raw, err := readBounded(f, 64<<20)
	if err != nil {
		return nil, err
	}
	observed := sha256.Sum256(raw)
	if hex.EncodeToString(observed[:]) != digest {
		return nil, errors.New("selected evidence hash differs")
	}
	return raw, nil
}

// Stable, content-addressed paths preserve Agenda's exact request identity.
// This directory is adapter-owned proof staging, never Agenda's private store.
func stageEvidence(directory, digest string, raw []byte) (string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if _, err := root.Lstat(digest); err == nil {
		if _, err := scopedEvidence(root, digest, digest); err != nil {
			return "", err
		}
		return filepath.Join(directory, digest), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return "", err
	}
	temporary := ".prepare-" + hex.EncodeToString(nonce[:])
	f, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer root.Remove(temporary)
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closed := f.Close()
	if err == nil {
		err = closed
	}
	if err != nil {
		return "", err
	}
	if err = root.Link(temporary, digest); err != nil && !os.IsExist(err) {
		return "", err
	}
	if _, err = scopedEvidence(root, digest, digest); err != nil {
		return "", err
	}
	dir, err := root.Open(".")
	if err != nil {
		return "", err
	}
	err = dir.Sync()
	_ = dir.Close()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, digest), nil
}

func (c controller) applyInput(raw json.RawMessage) ([]byte, func(), error) {
	cleanup := func() {}
	var change map[string]json.RawMessage
	if err := decode(raw, &change, false); err != nil {
		return nil, cleanup, err
	}
	if err := onlyFields(change, "schema", "request_id", "kind", "id", "previous", "by", "reason", "value", "evidence"); err != nil {
		return nil, cleanup, err
	}
	var value any
	if err := decode(change["value"], &value, false); err != nil {
		return nil, cleanup, err
	}
	if err := rejectSelectors(value); err != nil {
		return nil, cleanup, err
	}
	var evidence []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	if e, ok := change["evidence"]; ok {
		if err := decode(e, &evidence, true); err != nil {
			return nil, cleanup, err
		}
	}
	if len(evidence) > 16 {
		return nil, cleanup, errors.New("evidence exceeds 16 files")
	}
	if len(evidence) > 0 {
		if c.FilesRoot == "" || c.EvidenceRoot == "" {
			return nil, cleanup, errors.New("evidence requires controller-selected --files-root and --evidence-root")
		}
		root, err := os.OpenRoot(c.FilesRoot)
		if err != nil {
			return nil, cleanup, err
		}
		defer root.Close()
		var total int
		for i := range evidence {
			data, err := scopedEvidence(root, evidence[i].Path, evidence[i].SHA256)
			if err != nil {
				cleanup()
				return nil, func() {}, err
			}
			total += len(data)
			if total > 64<<20 {
				cleanup()
				return nil, func() {}, errors.New("evidence exceeds 64 MiB")
			}
			path, err := stageEvidence(c.EvidenceRoot, evidence[i].SHA256, data)
			if err != nil {
				cleanup()
				return nil, func() {}, err
			}
			evidence[i].Path = path
		}
		change["evidence"], _ = json.Marshal(evidence)
	}
	result, err := json.Marshal(change)
	return result, cleanup, err
}

func dispatch(ctx context.Context, c controller, raw []byte, out io.Writer) error {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Meta      json.RawMessage `json:"_meta,omitempty"`
	}
	if err := decode(raw, &call, true); err != nil {
		return err
	}
	if len(call.Arguments) == 0 {
		call.Arguments = json.RawMessage(`{}`)
	}
	var args map[string]json.RawMessage
	if err := decode(call.Arguments, &args, false); err != nil {
		return err
	}
	var result []byte
	var err error
	switch call.Name {
	case "agenda_export":
		if err = onlyFields(args); err == nil {
			result, err = c.call(ctx, nil, "export", c.Root)
		}
	case "agenda_project":
		if err = onlyFields(args, "as_of"); err == nil {
			var at string
			err = json.Unmarshal(args["as_of"], &at)
			if err == nil {
				result, err = c.project(ctx, at)
			}
		}
	case "agenda_expand":
		if err = onlyFields(args, "as_of", "schedule"); err == nil {
			var at string
			err = json.Unmarshal(args["as_of"], &at)
			if err == nil {
				err = validInstant(at)
			}
			var spec any
			if err == nil {
				err = decode(args["schedule"], &spec, false)
			}
			if err == nil {
				err = rejectSelectors(spec)
			}
			if err == nil {
				result, err = c.call(ctx, args["schedule"], "expand", "--as-of", at)
			}
			if err == nil {
				rows := []json.RawMessage{}
				for _, line := range bytes.Split(result, []byte{'\n'}) {
					if len(bytes.TrimSpace(line)) == 0 {
						continue
					}
					if err = validateJSON(line); err != nil {
						break
					}
					rows = append(rows, append(json.RawMessage(nil), line...))
				}
				if err == nil {
					result, err = json.Marshal(map[string]any{"schema": "agenda.occurrences/v1", "occurrences": rows})
				}
			}
		}
	case "agenda_apply":
		if !c.AllowWrite {
			err = errors.New("agenda_apply is disabled; controller must select --allow-write")
		} else if err = onlyFields(args, "change"); err == nil {
			var input []byte
			var cleanup func()
			input, cleanup, err = c.applyInput(args["change"])
			defer cleanup()
			if err == nil {
				result, err = c.call(ctx, input, "apply", c.Root)
			}
		}
	default:
		err = errors.New("unknown Agenda tool")
	}
	if err != nil {
		code := 2
		var command *commandError
		if errors.As(err, &command) {
			code = command.Code
		}
		return json.NewEncoder(out).Encode(map[string]any{"isError": true, "structuredContent": map[string]any{"agenda_exit": code, "error": err.Error(), "retry": "No automatic retry; inspect the retained state and request identity."}, "content": []any{map[string]any{"type": "text", "text": err.Error()}}})
	}
	if err = validateJSON(result); err != nil {
		return errors.New("Agenda returned invalid JSON")
	}
	return json.NewEncoder(out).Encode(map[string]any{"structuredContent": json.RawMessage(result), "content": []any{map[string]any{"type": "text", "text": string(result)}}})
}
