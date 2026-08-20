package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	stateVersion = 1
	maxStateSize = maxAction*6 + 4096
)

var errStateRace = errors.New("approval state changed; retry")

type request struct {
	Version int    `json:"version"`
	Digest  string `json:"digest"`
	Job     string `json:"job"`
	Action  string `json:"action"`
	AskedAt string `json:"asked_at"`
}

type auditRecord struct {
	Time    string `json:"time"`
	Digest  string `json:"digest"`
	Job     string `json:"job,omitempty"`
	Action  string `json:"action"`
	Verdict string `json:"verdict"`
}

func (a *app) ensureState() error {
	if a.stateDir == "" {
		return errors.New("cannot locate home directory for state")
	}
	for _, dir := range []string{
		a.stateDir,
		filepath.Join(a.stateDir, "pending"),
		filepath.Join(a.stateDir, "granted"),
		filepath.Join(a.stateDir, "declined"),
		filepath.Join(a.stateDir, "spent"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create state directory %q: %w", dir, err)
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return fmt.Errorf("inspect state directory %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("state path %q is not a directory", dir)
		}
		if info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("state directory %q permits group or other access", dir)
		}
	}
	return nil
}

func (a *app) statePath(kind, digest string) string {
	return filepath.Join(a.stateDir, kind, digest+".json")
}

func (a *app) forJob(job, action string) int {
	if err := a.ensureState(); err != nil {
		return a.fail(exitErr, err)
	}
	digest := actionDigest(job, action)
	req := request{
		Version: stateVersion,
		Digest:  digest,
		Job:     job,
		Action:  action,
		AskedAt: a.now().UTC().Format(time.RFC3339Nano),
	}

	declined := a.statePath("declined", digest)
	if exists, err := a.validStateFile(declined, req); err != nil {
		return a.fail(exitErr, err)
	} else if exists {
		fmt.Fprintf(a.errOut, "may: declined %s\n", digest)
		return exitNo
	}

	granted := a.statePath("granted", digest)
	if exists, err := a.validStateFile(granted, req); err != nil {
		return a.fail(exitErr, err)
	} else if exists {
		if err := a.consumeGrant(granted, digest); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return a.fail(exitErr, errStateRace)
			}
			return a.fail(exitErr, fmt.Errorf("consume grant %s: %w", digest, err))
		}
		if err := a.audit("spent", digest, job, action); err != nil {
			return a.fail(exitErr, err)
		}
		fmt.Fprintf(a.errOut, "may: granted %s (spent)\n", digest)
		return exitYes
	}

	pending := a.statePath("pending", digest)
	if exists, err := a.validStateFile(pending, req); err != nil {
		return a.fail(exitErr, err)
	} else if exists {
		fmt.Fprintf(a.errOut, "may: parked %s\n", digest)
		return exitParked
	}

	body, err := json.Marshal(req)
	if err != nil {
		return a.fail(exitErr, fmt.Errorf("encode request: %w", err))
	}
	body = append(body, '\n')
	if err := a.audit("asked", digest, job, action); err != nil {
		return a.fail(exitErr, err)
	}
	if err := writeExclusive(pending, body); err != nil {
		if errors.Is(err, os.ErrExist) {
			fmt.Fprintf(a.errOut, "may: parked %s\n", digest)
			return exitParked
		}
		return a.fail(exitErr, fmt.Errorf("record request: %w", err))
	}
	fmt.Fprintf(a.errOut, "may: parked %s: %s\n", digest, strconv.Quote(action))
	return exitParked
}

func (a *app) consumeGrant(granted, digest string) error {
	root := filepath.Join(a.stateDir, "spent")
	stamp := a.now().UTC().UnixNano()
	for sequence := 0; sequence < 1000; sequence++ {
		dir := filepath.Join(root, fmt.Sprintf("%s.%d.%d", digest, stamp, sequence))
		if err := os.Mkdir(dir, 0o700); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return err
		}
		err := os.Rename(granted, filepath.Join(dir, "request.json"))
		if err != nil {
			_ = os.Remove(dir)
		}
		return err
	}
	return errors.New("cannot allocate unique spent-grant directory")
}

func (a *app) validStateFile(path string, want request) (bool, error) {
	got, err := readRequest(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if got.Version != stateVersion || got.Digest != want.Digest ||
		got.Job != want.Job || got.Action != want.Action ||
		actionDigest(got.Job, got.Action) != got.Digest {
		return false, fmt.Errorf("state file %q does not match the exact request", path)
	}
	return true, nil
}

func readRequest(path string) (request, error) {
	entry, err := os.Lstat(path)
	if err != nil {
		return request{}, err
	}
	if !entry.Mode().IsRegular() {
		return request{}, fmt.Errorf("request %q is not a regular file", path)
	}
	if entry.Mode().Perm()&0o077 != 0 {
		return request{}, fmt.Errorf("request %q permits group or other access", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return request{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return request{}, fmt.Errorf("inspect request %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return request{}, fmt.Errorf("request %q is not a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return request{}, fmt.Errorf("request %q permits group or other access", path)
	}
	if info.Size() > maxStateSize {
		return request{}, fmt.Errorf("request %q exceeds %d bytes", path, maxStateSize)
	}
	var req request
	dec := json.NewDecoder(io.LimitReader(f, maxStateSize+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return request{}, fmt.Errorf("decode request %q: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return request{}, fmt.Errorf("decode request %q: trailing JSON", path)
		}
		return request{}, fmt.Errorf("decode request %q: %w", path, err)
	}
	if !validDigest(req.Digest) || req.Version != stateVersion ||
		actionDigest(req.Job, req.Action) != req.Digest {
		return request{}, fmt.Errorf("request %q has an invalid digest", path)
	}
	return req, nil
}

func writeExclusive(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(body)
	if writeErr == nil && n != len(body) {
		writeErr = io.ErrShortWrite
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}

func (a *app) audit(verdict, digest, job, action string) error {
	rec := auditRecord{
		Time:    a.now().UTC().Format(time.RFC3339Nano),
		Digest:  digest,
		Job:     job,
		Action:  action,
		Verdict: verdict,
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode audit record: %w", err)
	}
	body = append(body, '\n')
	path := filepath.Join(a.stateDir, "audit.jsonl")
	if entry, err := os.Lstat(path); err == nil {
		if !entry.Mode().IsRegular() || entry.Mode().Perm()&0o077 != 0 {
			return errors.New("audit log is not a private regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect audit log: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	info, statErr := f.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		_ = f.Close()
		if statErr != nil {
			return fmt.Errorf("inspect audit log: %w", statErr)
		}
		return errors.New("audit log is not a private regular file")
	}
	n, writeErr := f.Write(body)
	if writeErr == nil && n != len(body) {
		writeErr = io.ErrShortWrite
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
}

func (a *app) pending() int {
	if err := a.ensureState(); err != nil {
		return a.fail(exitErr, err)
	}
	paths, err := sortedJSONFiles(filepath.Join(a.stateDir, "pending"))
	if err != nil {
		return a.fail(exitErr, fmt.Errorf("list pending requests: %w", err))
	}
	for _, path := range paths {
		req, err := readRequest(path)
		if err != nil {
			return a.fail(exitErr, err)
		}
		if filepath.Base(path) != req.Digest+".json" {
			return a.fail(exitErr, fmt.Errorf(
				"pending request %q is not named by its digest", path))
		}
		body, err := json.Marshal(req)
		if err != nil {
			return a.fail(exitErr, fmt.Errorf("encode pending request: %w", err))
		}
		body = append(body, '\n')
		if code := a.write(body); code != exitYes {
			return code
		}
	}
	return exitYes
}

func (a *app) decide(digest string) int {
	if !validDigest(digest) {
		return a.fail(exitErr, errors.New("DIGEST must be 64 lowercase hexadecimal characters"))
	}
	if err := a.ensureState(); err != nil {
		return a.fail(exitErr, err)
	}
	pending := a.statePath("pending", digest)
	req, err := readRequest(pending)
	if errors.Is(err, os.ErrNotExist) {
		return a.fail(exitErr, fmt.Errorf("no pending request %s", digest))
	}
	if err != nil {
		return a.fail(exitErr, err)
	}
	tty, err := a.openTTY()
	if err != nil {
		if auditErr := a.audit("decision-unavailable", digest, req.Job, req.Action); auditErr != nil {
			return a.fail(exitErr, auditErr)
		}
		return a.fail(exitNo, errors.New("no controlling terminal; request remains pending"))
	}
	approved, err := askHuman(tty, req)
	if err != nil {
		return a.fail(exitErr, err)
	}
	kind := "declined"
	verdict := "declined"
	code := exitNo
	if approved {
		kind = "granted"
		verdict = "granted"
		code = exitYes
	}
	destination := a.statePath(kind, digest)
	if _, err := os.Lstat(destination); err == nil {
		return a.fail(exitErr, fmt.Errorf("decision already exists for %s", digest))
	} else if !errors.Is(err, os.ErrNotExist) {
		return a.fail(exitErr, fmt.Errorf("inspect decision for %s: %w", digest, err))
	}
	if err := a.audit(verdict, digest, req.Job, req.Action); err != nil {
		return a.fail(exitErr, err)
	}
	if err := os.Rename(pending, destination); err != nil {
		return a.fail(exitErr, fmt.Errorf("record decision for %s: %w", digest, err))
	}
	fmt.Fprintf(a.errOut, "may: %s %s\n", verdict, digest)
	return code
}
