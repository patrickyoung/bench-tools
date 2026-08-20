package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type checkTTY struct{ bytes.Buffer }

func (t *checkTTY) Close() error { return nil }

func selfCheck(out io.Writer) error {
	state, err := os.MkdirTemp("", "may-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(state)
	if err := os.Chmod(state, 0o700); err != nil {
		return err
	}
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	run := func(input, answer string, ttyAvailable bool, args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		a := &app{
			in:       strings.NewReader(input),
			out:      &stdout,
			errOut:   &stderr,
			stateDir: state,
			now:      func() time.Time { return now },
			openTTY: func() (readWriteCloser, error) {
				if !ttyAvailable {
					return nil, errors.New("no tty")
				}
				return &checkTTY{Buffer: *bytes.NewBufferString(answer)}, nil
			},
		}
		return a.run(args), stdout.String(), stderr.String()
	}

	if code, _, _ := run("publish release v1.4.2\n", "yes\n", true); code != exitYes {
		return fmt.Errorf("terminal yes exited %d", code)
	}
	if code, _, _ := run("delete production\n", "no\n", true); code != exitNo {
		return fmt.Errorf("terminal no exited %d", code)
	}
	if code, _, _ := run("send report\n", "", false); code != exitNo {
		return fmt.Errorf("missing terminal exited %d", code)
	}

	action := "publish release v1.4.2\n"
	job := "release-142"
	digest := actionDigest(job, action)
	if code, _, _ := run(action, "", false, job); code != exitParked {
		return fmt.Errorf("new job request exited %d", code)
	}
	if code, stdout, _ := run("", "", false, "pending"); code != exitYes ||
		!strings.Contains(stdout, digest) || !strings.Contains(stdout, "publish release") {
		return fmt.Errorf("pending did not expose the exact request")
	}
	if code, _, _ := run("", "yes\n", true, "decide", digest); code != exitYes {
		return fmt.Errorf("grant decision exited %d", code)
	}
	if code, _, _ := run(action, "", false, job); code != exitYes {
		return fmt.Errorf("matching grant exited %d", code)
	}
	if code, _, _ := run(action, "", false, job); code != exitParked {
		return fmt.Errorf("spent grant was reusable: exit %d", code)
	}

	declinedAction := "erase archive\n"
	declinedJob := "archive-9"
	declinedDigest := actionDigest(declinedJob, declinedAction)
	if code, _, _ := run(declinedAction, "", false, declinedJob); code != exitParked {
		return fmt.Errorf("decline fixture did not park: exit %d", code)
	}
	if code, _, _ := run("", "n\n", true, "decide", declinedDigest); code != exitNo {
		return fmt.Errorf("decline decision exited %d", code)
	}
	if code, _, _ := run(declinedAction, "", false, declinedJob); code != exitNo {
		return fmt.Errorf("declined request exited %d", code)
	}

	tamperedAction := "transfer funds\n"
	tamperedJob := "transfer-1"
	tamperedDigest := actionDigest(tamperedJob, tamperedAction)
	if code, _, _ := run(tamperedAction, "", false, tamperedJob); code != exitParked {
		return fmt.Errorf("tamper fixture did not park: exit %d", code)
	}
	if code, _, _ := run("", "yes\n", true, "decide", tamperedDigest); code != exitYes {
		return fmt.Errorf("tamper fixture grant exited %d", code)
	}
	grant := filepath.Join(state, "granted", tamperedDigest+".json")
	if err := os.WriteFile(grant, []byte("{\"version\":1,\"digest\":\"bad\"}\n"), 0o600); err != nil {
		return fmt.Errorf("tamper grant: %w", err)
	}
	if code, _, _ := run(tamperedAction, "", false, tamperedJob); code != exitErr {
		return fmt.Errorf("tampered grant did not fail closed: exit %d", code)
	}

	if actionDigest("exact", "words") == actionDigest("exact", "words\n") {
		return errors.New("trailing newline did not change digest")
	}
	auditBody, err := os.ReadFile(filepath.Join(state, "audit.jsonl"))
	if err != nil {
		return fmt.Errorf("read audit: %w", err)
	}
	lines := bytes.Split(bytes.TrimSpace(auditBody), []byte{'\n'})
	if len(lines) < 11 {
		return fmt.Errorf("audit has %d records, want at least 11", len(lines))
	}
	seenWords := false
	for _, line := range lines {
		var rec auditRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("invalid audit JSON: %w", err)
		}
		if rec.Action == action && rec.Digest == digest {
			seenWords = true
		}
	}
	if !seenWords {
		return errors.New("audit omitted the exact words")
	}

	fmt.Fprintln(out, "may check: ok")
	return nil
}
