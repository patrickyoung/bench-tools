package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func mode(o options) string {
	if o.attach != "" {
		return "attach"
	}
	if o.userDataDir != "" {
		return "native"
	}
	if o.profile != "" {
		return "profile"
	}
	return "fresh"
}
func audit(cmd, url string, size int, start time.Time, outcome, mode, grant, what string) {
	path := os.Getenv("WEB_STATE")
	if path == "" {
		root := os.Getenv("XDG_STATE_HOME")
		if root == "" {
			home, e := os.UserHomeDir()
			if e != nil {
				return
			}
			root = filepath.Join(home, ".local", "state")
		}
		path = filepath.Join(root, "web", "web.jsonl")
	}
	row := map[string]any{"t": time.Now().UTC().Format(time.RFC3339Nano), "cmd": cmd, "url": url, "bytes": size, "ms": time.Since(start).Milliseconds(), "outcome": outcome, "mode": mode}
	if grant != "" {
		row["grant"] = grant
	}
	if what != "" {
		row["what"] = what
	}
	b, e := json.Marshal(row)
	if e != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	f, e := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
func gate(ctx context.Context, description, cmd, url, identity, job string, stderr io.Writer) error {
	start := time.Now()
	code := 77
	digest := ""
	worker, legacyJob, root := os.Getenv("CLERK_WORKER"), os.Getenv("JOB_ID"), os.Getenv("CLERK_WROOT")
	if job == "" && worker != "" && legacyJob != "" && root != "" {
		job = worker + "/" + legacyJob
	}
	if job != "" {
		proc := exec.CommandContext(ctx, "may", "request", job)
		proc.Stdin = strings.NewReader(description)
		var reply bytes.Buffer
		proc.Stdout = &reply
		proc.Stderr = stderr
		err := proc.Run()
		status := 0
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				status = exit.ExitCode()
			} else {
				status = 77
			}
		}
		var result struct {
			Version int    `json:"version"`
			Job     string `json:"job"`
			Action  string `json:"action"`
			Verdict string `json:"verdict"`
			Digest  string `json:"digest"`
		}
		if json.Unmarshal(reply.Bytes(), &result) == nil {
			want := map[int]string{0: "spent", 3: "declined", 75: "parked"}[status]
			if want != "" && result.Verdict == want && result.Version == 1 && result.Job == job && result.Action == description && len(result.Digest) == 64 {
				code = status
				digest = result.Digest
			}
		}

	} else if tty, e := os.OpenFile("/dev/tty", os.O_RDWR, 0); e == nil {
		defer tty.Close()
		fmt.Fprintf(tty, "\nweb: about to %s\nweb: allow it? [y/N] ", description)
		answer := make(chan string, 1)
		go func() { s, _ := bufio.NewReader(tty).ReadString('\n'); answer <- s }()
		select {
		case s := <-answer:
			code = 3
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "y" || s == "yes" {
				code = 0
			}
		case <-ctx.Done():
			code = 77
		}
	}
	outcome := map[int]string{0: "granted", 3: "declined", 75: "parked", 77: "no_approver"}[code]
	audit(cmd+":gate", url, 0, start, outcome, identity, digest, description)
	if code == 0 {
		return nil
	}
	if code == 77 {
		return &failure{77, "no approver: " + description + " (use --may-job with may or a controlling terminal)"}
	}
	return &failure{code, description}
}
