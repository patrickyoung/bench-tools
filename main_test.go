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
	"testing"
	"time"
	"unicode/utf8"
)

type testTTY struct{ bytes.Buffer }

func (t *testTTY) Close() error { return nil }

func runMay(t *testing.T, state, input, answer string, ttyAvailable bool,
	args ...string) (int, string, string) {
	t.Helper()
	if err := os.Chmod(state, 0o700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	a := &app{
		in:       strings.NewReader(input),
		out:      &stdout,
		errOut:   &stderr,
		stateDir: state,
		now: func() time.Time {
			return time.Date(2026, 8, 20, 15, 4, 5, 6, time.UTC)
		},
		openTTY: func() (readWriteCloser, error) {
			if !ttyAvailable {
				return nil, errors.New("no tty")
			}
			return &testTTY{Buffer: *bytes.NewBufferString(answer)}, nil
		},
	}
	code := a.run(args)
	return code, stdout.String(), stderr.String()
}

func TestTerminalApprovalUsesTTYAndAuditsExactStdin(t *testing.T) {
	state := t.TempDir()
	action := "publish release v1.4.2\n"
	code, stdout, stderr := runMay(t, state, action, "yes\n", true)
	if code != exitYes || stdout != "" || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	body, err := os.ReadFile(filepath.Join(state, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rec auditRecord
	if err := json.Unmarshal(bytes.TrimSpace(body), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Action != action || rec.Digest != actionDigest("", action) ||
		rec.Verdict != "approved" {
		t.Fatalf("audit = %#v", rec)
	}
}

func TestTerminalAnythingButYesRefuses(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", "sure\n", "Y\n"} {
		code, _, stderr := runMay(t, t.TempDir(), "act\n", answer, true)
		want := exitNo
		if answer == "Y\n" {
			want = exitYes
		}
		if code != want {
			t.Errorf("answer %q: exit=%d stderr=%q", answer, code, stderr)
		}
	}
}

func TestNoTerminalFailsClosedAndAudits(t *testing.T) {
	state := t.TempDir()
	code, _, stderr := runMay(t, state, "act\n", "", false)
	if code != exitNo || !strings.Contains(stderr, "no controlling terminal") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	body, err := os.ReadFile(filepath.Join(state, "audit.jsonl"))
	if err != nil || !bytes.Contains(body, []byte(`"verdict":"unavailable"`)) {
		t.Fatalf("audit=%q err=%v", body, err)
	}
}

func TestJobParksGrantsOnceThenParksAgain(t *testing.T) {
	state := t.TempDir()
	job, action := "release-142", "publish release v1.4.2\n"
	digest := actionDigest(job, action)

	code, _, stderr := runMay(t, state, action, "", false, job)
	if code != exitParked || !strings.Contains(stderr, digest) {
		t.Fatalf("request: exit=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := runMay(t, state, "", "", false, "pending")
	if code != exitYes || !strings.Contains(stdout, action[:len(action)-1]) {
		t.Fatalf("pending: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stderr = runMay(t, state, "", "yes\n", true, "decide", digest)
	if code != exitYes || !strings.Contains(stderr, "granted") {
		t.Fatalf("decide: exit=%d stderr=%q", code, stderr)
	}
	code, _, stderr = runMay(t, state, action, "", false, job)
	if code != exitYes || !strings.Contains(stderr, "spent") {
		t.Fatalf("consume: exit=%d stderr=%q", code, stderr)
	}
	code, _, _ = runMay(t, state, action, "", false, job)
	if code != exitParked {
		t.Fatalf("reuse exit=%d", code)
	}
	spent, err := filepath.Glob(filepath.Join(state, "spent", digest+".*"))
	if err != nil || len(spent) != 1 {
		t.Fatalf("spent=%v err=%v", spent, err)
	}
}

func TestMachineRequestReportsTheSameExactOneShotDecision(t *testing.T) {
	state := t.TempDir()
	job, action := "release-142", "publish release v1.4.2\n"
	digest := actionDigest(job, action)

	code, stdout, stderr := runMay(t, state, action, "", false, "request", job)
	assertRequestResult(t, code, stdout, stderr, requestResult{
		Version: requestResultVersion, Job: job, Digest: digest, Action: action, Verdict: "parked",
	}, exitParked)
	if code, _, _ := runMay(t, state, "", "yes\n", true, "decide", digest); code != exitYes {
		t.Fatal(code)
	}
	code, stdout, stderr = runMay(t, state, action, "", false, "request", job)
	assertRequestResult(t, code, stdout, stderr, requestResult{
		Version: requestResultVersion, Job: job, Digest: digest, Action: action, Verdict: "spent",
	}, exitYes)
	code, stdout, stderr = runMay(t, state, action, "", false, "request", job)
	assertRequestResult(t, code, stdout, stderr, requestResult{
		Version: requestResultVersion, Job: job, Digest: digest, Action: action, Verdict: "parked",
	}, exitParked)
}

func TestMachineRequestReportsDeclineWithoutAnotherApprovalPath(t *testing.T) {
	state := t.TempDir()
	job, action := "archive-9", "erase archive\n"
	digest := actionDigest(job, action)
	if code, _, _ := runMay(t, state, action, "", false, "request", job); code != exitParked {
		t.Fatal(code)
	}
	if code, _, _ := runMay(t, state, "", "no\n", true, "decide", digest); code != exitNo {
		t.Fatal(code)
	}
	code, stdout, stderr := runMay(t, state, action, "", false, "request", job)
	assertRequestResult(t, code, stdout, stderr, requestResult{
		Version: requestResultVersion, Job: job, Digest: digest, Action: action, Verdict: "declined",
	}, exitNo)
}

func assertRequestResult(t *testing.T, code int, stdout, stderr string, want requestResult, wantCode int) {
	t.Helper()
	if code != wantCode || stderr != "" || !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var got requestResult
	dec := json.NewDecoder(strings.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("result=%#v want %#v", got, want)
	}
}

func TestConcurrentCallersCannotBothSpendOneGrant(t *testing.T) {
	state := t.TempDir()
	job, action := "release", "publish\n"
	digest := actionDigest(job, action)
	if code, _, _ := runMay(t, state, action, "", false, job); code != exitParked {
		t.Fatal(code)
	}
	if code, _, _ := runMay(t, state, "", "yes\n", true, "decide", digest); code != exitYes {
		t.Fatal(code)
	}

	start := make(chan struct{})
	results := make(chan int, 2)
	invoke := func() {
		<-start
		var stdout, stderr bytes.Buffer
		a := &app{
			in:       strings.NewReader(action),
			out:      &stdout,
			errOut:   &stderr,
			stateDir: state,
			now:      time.Now,
			openTTY: func() (readWriteCloser, error) {
				return nil, errors.New("no tty")
			},
		}
		results <- a.run([]string{job})
	}
	go invoke()
	go invoke()
	close(start)
	first, second := <-results, <-results
	approved := 0
	for _, code := range []int{first, second} {
		if code == exitYes {
			approved++
		}
	}
	if approved != 1 {
		t.Fatalf("concurrent exits = %d, %d; approved=%d", first, second, approved)
	}
}

func TestMachineCallersCannotBothReportOneGrantSpent(t *testing.T) {
	state := t.TempDir()
	job, action := "release", "publish\n"
	digest := actionDigest(job, action)
	if code, _, _ := runMay(t, state, action, "", false, "request", job); code != exitParked {
		t.Fatal(code)
	}
	if code, _, _ := runMay(t, state, "", "yes\n", true, "decide", digest); code != exitYes {
		t.Fatal(code)
	}

	type outcome struct {
		code int
		body string
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	invoke := func() {
		<-start
		var stdout, stderr bytes.Buffer
		a := &app{in: strings.NewReader(action), out: &stdout, errOut: &stderr,
			stateDir: state, now: time.Now,
			openTTY: func() (readWriteCloser, error) { return nil, errors.New("no tty") }}
		results <- outcome{code: a.run([]string{"request", job}), body: stdout.String()}
	}
	go invoke()
	go invoke()
	close(start)
	spent := 0
	for range 2 {
		got := <-results
		if got.code != exitYes {
			continue
		}
		var result requestResult
		if err := json.Unmarshal([]byte(got.body), &result); err != nil || result.Verdict != "spent" || result.Digest != digest {
			t.Fatalf("invalid successful machine result: exit=%d body=%q err=%v", got.code, got.body, err)
		}
		spent++
	}
	if spent != 1 {
		t.Fatalf("machine spent results=%d, want 1", spent)
	}
}

func TestJobBoundKeepsParkedRequestsReadable(t *testing.T) {
	for _, command := range []bool{false, true} {
		t.Run(fmt.Sprint(command), func(t *testing.T) {
			state := t.TempDir()
			job := strings.Repeat("j", maxJob)
			args := []string{job}
			if command {
				args = []string{"request", job}
			}
			if code, _, stderr := runMay(t, state, "act\n", "", false, args...); code != exitParked {
				t.Fatalf("max job exit=%d stderr=%q", code, stderr)
			}
			if code, stdout, stderr := runMay(t, state, "", "", false, "pending"); code != exitYes || !strings.Contains(stdout, job) {
				t.Fatalf("pending exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			tooLong := strings.Repeat("j", maxJob+1)
			args = []string{tooLong}
			if command {
				args = []string{"request", tooLong}
			}
			if code, stdout, stderr := runMay(t, state, "act\n", "", false, args...); code != exitErr || stdout != "" || !strings.Contains(stderr, "exceeds") {
				t.Fatalf("oversize exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestDeclineIsAnAnswer(t *testing.T) {
	state := t.TempDir()
	job, action := "archive-9", "erase archive\n"
	digest := actionDigest(job, action)
	if code, _, _ := runMay(t, state, action, "", false, job); code != exitParked {
		t.Fatal(code)
	}
	if code, _, _ := runMay(t, state, "", "no\n", true, "decide", digest); code != exitNo {
		t.Fatal(code)
	}
	if code, _, stderr := runMay(t, state, action, "", false, job); code != exitNo || !strings.Contains(stderr, "declined") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
}

func TestDecisionNeedsTTYAndLeavesRequestPending(t *testing.T) {
	state := t.TempDir()
	job, action := "job", "act\n"
	digest := actionDigest(job, action)
	if code, _, _ := runMay(t, state, action, "", false, job); code != exitParked {
		t.Fatal(code)
	}
	code, _, stderr := runMay(t, state, "", "", false, "decide", digest)
	if code != exitNo || !strings.Contains(stderr, "remains pending") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(state, "pending", digest+".json")); err != nil {
		t.Fatal(err)
	}
}

func TestDoubleDashPermitsReservedOrOptionLikeJob(t *testing.T) {
	for _, job := range []string{"check", "-release"} {
		state := t.TempDir()
		action := "act\n"
		code, _, stderr := runMay(t, state, action, "", false, "--", job)
		if code != exitParked || !strings.Contains(stderr, actionDigest(job, action)) {
			t.Fatalf("job=%q exit=%d stderr=%q", job, code, stderr)
		}
	}
}

func TestTamperedGrantFailsClosed(t *testing.T) {
	state := t.TempDir()
	job, action := "job", "act\n"
	digest := actionDigest(job, action)
	if code, _, _ := runMay(t, state, action, "", false, job); code != exitParked {
		t.Fatal(code)
	}
	if code, _, _ := runMay(t, state, "", "yes\n", true, "decide", digest); code != exitYes {
		t.Fatal(code)
	}
	grant := filepath.Join(state, "granted", digest+".json")
	if err := os.WriteFile(grant, []byte(`{"version":1,"digest":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runMay(t, state, action, "", false, job)
	if code != exitErr || !strings.Contains(stderr, "invalid digest") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
}

func TestStateAndAuditMustBePrivateRegularEntries(t *testing.T) {
	state := t.TempDir()
	if err := os.Chmod(state, 0o755); err != nil {
		t.Fatal(err)
	}
	var directOut, directErr bytes.Buffer
	a := &app{in: strings.NewReader("act\n"), out: &directOut, errOut: &directErr,
		stateDir: state, now: time.Now,
		openTTY: func() (readWriteCloser, error) {
			return &testTTY{Buffer: *bytes.NewBufferString("yes\n")}, nil
		}}
	if code := a.run(nil); code != exitErr || !strings.Contains(directErr.String(), "group or other") {
		t.Fatalf("permissive state: exit=%d stderr=%q", code, directErr.String())
	}

	if err := os.Chmod(state, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "audit")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(state, "audit.jsonl")); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runMay(t, state, "act\n", "yes\n", true)
	if code != exitErr || !strings.Contains(stderr, "audit log is not") {
		t.Fatalf("audit symlink: exit=%d stderr=%q", code, stderr)
	}
}

func TestDigestBindsJobAndExactBytes(t *testing.T) {
	values := []string{
		actionDigest("a", "words"),
		actionDigest("b", "words"),
		actionDigest("a", "words\n"),
		actionDigest("a", "Words"),
	}
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] || !validDigest(value) {
			t.Fatalf("digest collision or invalid: %q", value)
		}
		seen[value] = true
	}
}

func TestActionValidation(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"space", " \n\t"},
		{"nul", "act\x00now"},
		{"utf8", invalidUTF8},
		{"large", strings.Repeat("x", maxAction+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if utf8.ValidString(invalidUTF8) {
				t.Fatal("invalid fixture")
			}
			code, _, _ := runMay(t, t.TempDir(), tc.input, "yes\n", true)
			if code != exitErr {
				t.Fatalf("exit=%d", code)
			}
		})
	}
	code, _, _ := runMay(t, t.TempDir(), strings.Repeat("x", maxAction), "yes\n", true)
	if code != exitYes {
		t.Fatalf("exact bound exit=%d", code)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("broken") }

func TestInputAndOutputFailures(t *testing.T) {
	var stdout, stderr bytes.Buffer
	a := &app{in: failingReader{}, out: &stdout, errOut: &stderr,
		stateDir: t.TempDir(), now: time.Now,
		openTTY: func() (readWriteCloser, error) { return nil, errors.New("no") }}
	if code := a.run(nil); code != exitErr || !strings.Contains(stderr.String(), "read action") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken") }

func TestPendingOutputFailure(t *testing.T) {
	state := t.TempDir()
	job, action := "job", "act\n"
	if code, _, _ := runMay(t, state, action, "", false, job); code != exitParked {
		t.Fatal(code)
	}
	var stderr bytes.Buffer
	a := &app{in: strings.NewReader(""), out: brokenWriter{}, errOut: &stderr,
		stateDir: state, now: time.Now, openTTY: openControllingTTY}
	if code := a.run([]string{"pending"}); code != exitErr ||
		!strings.Contains(stderr.String(), "write stdout") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
}

func TestMachineOutputFailureAfterSpendCannotBecomeApproval(t *testing.T) {
	state := t.TempDir()
	job, action := "job", "act\n"
	digest := actionDigest(job, action)
	if code, _, _ := runMay(t, state, action, "", false, "request", job); code != exitParked {
		t.Fatal(code)
	}
	if code, _, _ := runMay(t, state, "", "yes\n", true, "decide", digest); code != exitYes {
		t.Fatal(code)
	}
	var stderr bytes.Buffer
	a := &app{in: strings.NewReader(action), out: brokenWriter{}, errOut: &stderr,
		stateDir: state, now: time.Now, openTTY: openControllingTTY}
	if code := a.run([]string{"request", job}); code != exitErr ||
		!strings.Contains(stderr.String(), "write stdout") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	if code, _, _ := runMay(t, state, action, "", false, "request", job); code != exitParked {
		t.Fatalf("a spent grant survived an unrecordable machine result: exit=%d", code)
	}
}

func TestPendingRefusesSpecialOrMisnamedEntries(t *testing.T) {
	for _, special := range []bool{true, false} {
		state := t.TempDir()
		if err := os.Chmod(state, 0o700); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := runMay(t, state, "act\n", "", false, "job")
		if code != exitParked {
			t.Fatalf("fixture: exit=%d stderr=%q", code, stderr)
		}
		pendingDir := filepath.Join(state, "pending")
		if special {
			target := filepath.Join(t.TempDir(), "target")
			if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(pendingDir, "extra.json")); err != nil {
				t.Fatal(err)
			}
		} else {
			paths, err := filepath.Glob(filepath.Join(pendingDir, "*.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("paths=%v err=%v", paths, err)
			}
			if err := os.Rename(paths[0], filepath.Join(pendingDir, "wrong.json")); err != nil {
				t.Fatal(err)
			}
		}
		code, _, stderr = runMay(t, state, "", "", false, "pending")
		if code != exitErr || (!strings.Contains(stderr, "not a regular file") &&
			!strings.Contains(stderr, "not named by its digest")) {
			t.Fatalf("special=%v exit=%d stderr=%q", special, code, stderr)
		}
	}
}

func TestArgumentsMetadataAndHelpWidth(t *testing.T) {
	state := t.TempDir()
	for _, tc := range []struct {
		args []string
		code int
		text string
	}{
		{[]string{"help"}, exitYes, "may - ask"},
		{[]string{"--help"}, exitYes, "may - ask"},
		{[]string{"version"}, exitYes, "may " + version},
		{[]string{"--version"}, exitYes, "may " + version},
		{[]string{"-wat"}, exitErr, ""},
		{[]string{"a", "b"}, exitErr, ""},
		{[]string{"decide"}, exitErr, ""},
		{[]string{"pending", "x"}, exitErr, ""},
		{[]string{"--"}, exitErr, ""},
		{[]string{"decide", strings.Repeat("z", 64)}, exitErr, ""},
	} {
		code, stdout, stderr := runMay(t, state, "", "", false, tc.args...)
		if code != tc.code || !strings.Contains(stdout, tc.text) {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q", tc.args, code, stdout, stderr)
		}
	}
	for i, line := range strings.Split(usageText, "\n") {
		if len(line) > 80 {
			t.Errorf("help line %d has %d columns: %q", i+1, len(line), line)
		}
	}
}

func TestSelfCheck(t *testing.T) {
	var out bytes.Buffer
	if err := selfCheck(&out); err != nil || out.String() != "may check: ok\n" {
		t.Fatalf("out=%q err=%v", out.String(), err)
	}
}

func TestReadRequestRejectsTrailingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	req := request{Version: stateVersion, Digest: actionDigest("j", "a"),
		Job: "j", Action: "a", AskedAt: time.Now().Format(time.RFC3339Nano)}
	body, _ := json.Marshal(req)
	body = append(body, []byte("\n{}\n")...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRequest(path); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("err=%v", err)
	}
}

func TestReadRequestRejectsOversizedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, maxStateSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRequest(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err=%v", err)
	}
}

func TestAskHumanReportsTTYFailures(t *testing.T) {
	tty := &faultTTY{}
	if _, err := askHuman(tty, request{Digest: strings.Repeat("a", 64), Action: "x"}); err == nil || !strings.Contains(err.Error(), "write controlling terminal") {
		t.Fatalf("err=%v", err)
	}
}

type faultTTY struct{}

func (*faultTTY) Read([]byte) (int, error)  { return 0, io.EOF }
func (*faultTTY) Write([]byte) (int, error) { return 0, fmt.Errorf("broken") }
func (*faultTTY) Close() error              { return nil }
