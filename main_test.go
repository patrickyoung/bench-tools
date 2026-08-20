package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeBackend struct {
	ready bool
	code  int
}

type hangingBackend struct{ fakeBackend }

func (hangingBackend) command(policy, []string) (*exec.Cmd, func(), error) {
	return exec.Command("/bin/sleep", "30"), func() {}, nil
}

func (b fakeBackend) status() statusRecord {
	return statusRecord{
		Backend: "fake", Available: true, Complete: true,
		Filesystem: "test", Network: "test",
	}
}

func (b fakeBackend) readyByte() byte { return 'x' }

func (b fakeBackend) command(_ policy, child []string) (*exec.Cmd, func(), error) {
	if !b.ready {
		return exec.Command("/bin/sh", "-c", "exit 7"), func() {}, nil
	}
	args := []string{"-c", launcher, "cage-launch"}
	args = append(args, child...)
	return exec.Command("/bin/sh", args...), func() {}, nil
}

func runForTest(args ...string) (outcome, string, string) {
	var stdout, stderr bytes.Buffer
	r := run(args, streams{in: strings.NewReader(""), out: &stdout, err: &stderr})
	return r, stdout.String(), stderr.String()
}

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want options
		bad  string
	}{
		{"legacy command", []string{"echo", "hello"}, options{command: []string{"echo", "hello"}}, ""},
		{"all flags", []string{"-net", "-w", "a", "-w", "b", "--", "cmd", "-x"},
			options{network: true, writes: []string{"a", "b"}, command: []string{"cmd", "-x"}}, ""},
		{"read only", []string{"-ro", "--", "cmd"},
			options{readOnly: true, command: []string{"cmd"}}, ""},
		{"conflict", []string{"-ro", "-w", ".", "cmd"}, options{}, "cannot"},
		{"missing write", []string{"-w"}, options{}, "needs"},
		{"unknown", []string{"-wat", "cmd"}, options{}, "unknown"},
		{"empty", nil, options{}, "no command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOptions(tt.args)
			if tt.bad != "" {
				if err == nil || !strings.Contains(err.Error(), tt.bad) {
					t.Fatalf("error = %v, want %q", err, tt.bad)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("options = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestPolicyDefaultsAndExplicitRoots(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	temp := filepath.Join(root, "temp")
	extra := filepath.Join(root, "extra")
	for _, dir := range []string{work, temp, extra} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(work)
	t.Setenv("TMPDIR", temp)
	work, _ = filepath.EvalSymlinks(work)
	temp, _ = filepath.EvalSymlinks(temp)
	extra, _ = filepath.EvalSymlinks(extra)

	p, err := makePolicy(options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.writes, []string{work, temp}) {
		t.Errorf("default writes = %v", p.writes)
	}
	p, err = makePolicy(options{readOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.writes, []string{temp}) {
		t.Errorf("read-only writes = %v", p.writes)
	}
	p, err = makePolicy(options{writes: []string{extra}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.writes, []string{extra, temp}) {
		t.Errorf("explicit writes = %v", p.writes)
	}
}

func TestPolicyRejectsAnythingButAnExistingDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, filepath.Join(t.TempDir(), "missing")} {
		if _, err := makePolicy(options{writes: []string{path}}); err == nil {
			t.Errorf("accepted writable path %q", path)
		}
	}
}

func TestInvalidTemporaryDirectoryIsSetupFailure(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	r, stdout, stderr := runForTest("--", "/bin/true")
	if r.code != exitSetup || stdout != "" ||
		!strings.Contains(stderr, "confinement setup") {
		t.Fatalf("outcome %#v, stdout %q, stderr %q", r, stdout, stderr)
	}
}

func TestRunnerPreservesStreamsAndChildStatus(t *testing.T) {
	stdin := "one\ntwo\n"
	var stdout, stderr bytes.Buffer
	child := []string{"/bin/sh", "-c", `cat; printf child-out; printf child-err >&2; exit 37`}
	r := runBackend(fakeBackend{ready: true}, policy{}, child, streams{
		in: strings.NewReader(stdin), out: &stdout, err: &stderr,
	})
	if r.code != 37 || r.signal != nil {
		t.Fatalf("outcome = %#v", r)
	}
	if stdout.String() != stdin+"child-out" || stderr.String() != "child-err" {
		t.Errorf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestRunnerDistinguishesSetupFailureFromChildFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := runBackend(fakeBackend{ready: false}, policy{}, []string{"/bin/sh", "-c", "exit 7"},
		streams{in: strings.NewReader(""), out: &stdout, err: &stderr})
	if r.code != exitSetup {
		t.Fatalf("outcome = %#v", r)
	}
	if !strings.Contains(stderr.String(), "did not establish the boundary") {
		t.Errorf("stderr = %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	r = runBackend(fakeBackend{ready: true}, policy{}, []string{"/bin/sh", "-c", "exit 7"},
		streams{in: strings.NewReader(""), out: &stdout, err: &stderr})
	if r.code != 7 {
		t.Fatalf("child outcome = %#v", r)
	}
}

func TestRunnerTimesOutOnlyBackendSetup(t *testing.T) {
	old := setupTimeout
	setupTimeout = 25 * time.Millisecond
	defer func() { setupTimeout = old }()
	var stderr bytes.Buffer
	start := time.Now()
	r := runBackend(hangingBackend{}, policy{}, []string{"must-not-run"}, streams{
		in: strings.NewReader(""), out: io.Discard, err: &stderr,
	})
	if r.code != exitSetup || time.Since(start) > time.Second {
		t.Fatalf("outcome %#v after %s", r, time.Since(start))
	}
	if !strings.Contains(stderr.String(), "no readiness proof") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestUnavailableBackendNeverConstructsAChild(t *testing.T) {
	var stderr bytes.Buffer
	r := runBackend(unavailableBackend{name: "none", detail: "not here"}, policy{},
		[]string{"this-must-not-run"}, streams{in: strings.NewReader(""), out: io.Discard, err: &stderr})
	if r.code != exitSetup || !strings.Contains(stderr.String(), "not here") {
		t.Fatalf("outcome %#v, stderr %q", r, stderr.String())
	}
}

func TestStatusIsMachineReadableAndHonest(t *testing.T) {
	r, stdout, stderr := runForTest("status")
	if r.code != 0 || stderr != "" {
		t.Fatalf("outcome %#v, stderr %q", r, stderr)
	}
	var got statusRecord
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "cage-status" || got.Platform != runtime.GOOS || got.Backend == "" {
		t.Errorf("status = %#v", got)
	}
	if got.Complete && !got.Available {
		t.Error("an unavailable backend claimed complete enforcement")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestStatusReportsOutputFailure(t *testing.T) {
	var stderr bytes.Buffer
	r := run([]string{"status"}, streams{in: strings.NewReader(""), out: brokenWriter{}, err: &stderr})
	if r.code != exitSetup || !strings.Contains(stderr.String(), "writing status") {
		t.Fatalf("outcome %#v, stderr %q", r, stderr.String())
	}
}

func TestHelpVersionAndUsage(t *testing.T) {
	r, stdout, stderr := runForTest("help")
	if r.code != 0 || stdout != usageText || stderr != "" {
		t.Errorf("help: %#v, %q, %q", r, stdout, stderr)
	}
	r, stdout, stderr = runForTest("version")
	if r.code != 0 || stdout != "cage "+version+"\n" || stderr != "" {
		t.Errorf("version: %#v, %q, %q", r, stdout, stderr)
	}
	r, stdout, stderr = runForTest()
	if r.code != exitUsage || stdout != "" || stderr != usageText {
		t.Errorf("usage: %#v, %q, %q", r, stdout, stderr)
	}
}
