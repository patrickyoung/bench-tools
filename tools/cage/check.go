package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const checkTokenEnv = "CAGE_INTERNAL_CHECK_TOKEN"

type externalResult struct {
	stdout   string
	stderr   string
	outcome  outcome
	timedOut bool
}

type checkReport struct {
	out    io.Writer
	passed int
	total  int
}

func (r *checkReport) record(name string, ok bool, detail string) {
	r.total++
	word := "FAIL"
	if ok {
		word = "ok"
		r.passed++
	}
	if detail == "" {
		fmt.Fprintf(r.out, "%s: %s\n", name, word)
		return
	}
	fmt.Fprintf(r.out, "%s: %s - %s\n", name, word, detail)
}

func runCheck(s streams) outcome {
	status := nativeBackend().status()
	if !status.Available {
		fmt.Fprintf(s.err, "cage check: %s\n", status.Detail)
		if status.Alternative != "" {
			fmt.Fprintf(s.err, "cage check: alternative: %s\n", status.Alternative)
		}
		return outcome{code: exitSetup}
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(s.err, "cage check: locating cage:", err)
		return outcome{code: exitSetup}
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Fprintln(s.err, "cage check: resolving cage:", err)
		return outcome{code: exitSetup}
	}
	root, err := os.MkdirTemp("", "cage-check-")
	if err != nil {
		fmt.Fprintln(s.err, "cage check: temporary root:", err)
		return outcome{code: exitSetup}
	}
	defer os.RemoveAll(root)
	work := filepath.Join(root, "work")
	extra := filepath.Join(root, "extra")
	scratch := filepath.Join(root, "scratch")
	for _, dir := range []string{work, extra, scratch} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			fmt.Fprintln(s.err, "cage check: temporary directory:", err)
			return outcome{code: exitSetup}
		}
	}
	token := randomToken()
	env := checkEnvironment(scratch, token)
	report := &checkReport{out: s.out}
	fmt.Fprintf(s.out, "backend: %s (%s)\n", status.Backend, status.Detail)

	workspaceFile := filepath.Join(work, "inside")
	r := runSelf(exe, work, env, "allowed stdin", "--", "/bin/sh", "-c",
		`printf workspace > "$1"`, "cage-check", workspaceFile)
	body, _ := os.ReadFile(workspaceFile)
	report.record("workspace write", r.outcome.code == 0 && string(body) == "workspace",
		externalDetail(r))

	tempFile := filepath.Join(scratch, "temporary")
	r = runSelf(exe, work, env, "", "-ro", "--", "/bin/sh", "-c",
		`printf temporary > "$1"`, "cage-check", tempFile)
	body, _ = os.ReadFile(tempFile)
	report.record("temporary write", r.outcome.code == 0 && string(body) == "temporary",
		externalDetail(r))

	outsideFile := filepath.Join(root, "outside")
	r = runSelf(exe, work, env, "", "--", "/bin/sh", "-c",
		`printf escaped > "$1"`, "cage-check", outsideFile)
	_, outsideErr := os.Stat(outsideFile)
	report.record("outside write denied", childRan(r) && r.outcome.code != 0 &&
		errors.Is(outsideErr, os.ErrNotExist),
		externalDetail(r))

	roFile := filepath.Join(work, "read-only-write")
	r = runSelf(exe, work, env, "", "-ro", "--", "/bin/sh", "-c",
		`printf escaped > "$1"`, "cage-check", roFile)
	_, roErr := os.Stat(roFile)
	report.record("read-only workspace", childRan(r) && r.outcome.code != 0 &&
		errors.Is(roErr, os.ErrNotExist),
		externalDetail(r))

	extraFile := filepath.Join(extra, "explicit")
	r = runSelf(exe, work, env, "", "-w", extra, "--", "/bin/sh", "-c",
		`printf explicit > "$1"`, "cage-check", extraFile)
	body, _ = os.ReadFile(extraFile)
	report.record("explicit writable directory",
		r.outcome.code == 0 && string(body) == "explicit", externalDetail(r))
	replacedFile := filepath.Join(work, "implicit")
	r = runSelf(exe, work, env, "", "-w", extra, "--", "/bin/sh", "-c",
		`printf escaped > "$1"`, "cage-check", replacedFile)
	_, replacedErr := os.Stat(replacedFile)
	report.record("explicit write replaces workspace", childRan(r) &&
		r.outcome.code != 0 && errors.Is(replacedErr, os.ErrNotExist), externalDetail(r))

	stdin := "stdin survives byte for byte\n\x00after nul\n"
	r = runSelf(exe, work, env, stdin, "--", "/bin/cat")
	report.record("stdin preservation", r.outcome.code == 0 && r.stdout == stdin,
		externalDetail(r))

	r = runSelf(exe, work, env, "", "--", "/bin/sh", "-c",
		"printf exact-stdout; printf exact-stderr >&2")
	report.record("stdout and stderr preservation", r.outcome.code == 0 &&
		r.stdout == "exact-stdout" && r.stderr == "exact-stderr", externalDetail(r))

	r = runSelf(exe, work, env, "", "--", "/bin/sh", "-c", "exit 42")
	report.record("exit propagation", r.outcome.code == 42 && r.outcome.signal == nil,
		externalDetail(r))

	r = runSelf(exe, work, env, "", "--", "/bin/sh", "-c", "kill -TERM $$")
	report.record("signal propagation", isTerminated(r.outcome),
		externalDetail(r))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		report.record("default network denied", false, err.Error())
		report.record("deliberate -net", false, err.Error())
	} else {
		address := listener.Addr().String()
		r = runSelf(exe, work, env, "", "--", exe,
			"__cage_check_network", token, address)
		report.record("default network denied", childRan(r) && r.outcome.code != 0,
			externalDetail(r))

		r = runSelf(exe, work, env, "", "-net", "--", exe,
			"__cage_check_network", token, address)
		accepted := acceptToken(listener, token)
		listener.Close()
		report.record("deliberate -net", r.outcome.code == 0 && accepted,
			externalDetail(r))
	}

	marker := filepath.Join(root, "must-not-run")
	r = runSelf(exe, work, env, "", "__cage_check_unavailable", token, marker)
	_, markerErr := os.Stat(marker)
	report.record("unavailable backend fails closed",
		r.outcome.code == exitSetup && errors.Is(markerErr, os.ErrNotExist), externalDetail(r))

	fmt.Fprintf(s.out, "cage check: %d/%d passed\n", report.passed, report.total)
	if report.passed != report.total {
		return outcome{code: exitCheck}
	}
	return outcome{code: exitOK}
}

func runSelf(exe, dir string, env []string, stdin string, args ...string) externalResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return externalResult{
		stdout: stdout.String(), stderr: strings.TrimSpace(stderr.String()),
		outcome: outcomeFromWait(err), timedOut: ctx.Err() != nil,
	}
}

func externalDetail(r externalResult) string {
	if r.timedOut {
		return "timed out"
	}
	if r.stderr != "" {
		line, _, _ := strings.Cut(r.stderr, "\n")
		return line
	}
	if r.outcome.signal != nil {
		return "signal " + r.outcome.signal.String()
	}
	if r.outcome.code != 0 {
		return fmt.Sprintf("status %d", r.outcome.code)
	}
	return ""
}

func childRan(r externalResult) bool {
	return !r.timedOut && !strings.Contains(r.stderr, "confinement setup")
}

func acceptToken(listener net.Listener, token string) bool {
	if tcp, ok := listener.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(2 * time.Second))
	}
	conn, err := listener.Accept()
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	body, err := io.ReadAll(io.LimitReader(conn, int64(len(token)+1)))
	return err == nil && string(body) == token
}

func randomToken() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(raw[:])
}

func checkEnvironment(temp, token string) []string {
	var env []string
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "TMPDIR=") || strings.HasPrefix(item, checkTokenEnv+"=") {
			continue
		}
		env = append(env, item)
	}
	return append(env, "TMPDIR="+temp, checkTokenEnv+"="+token)
}

func internalCommand(args []string, s streams) (bool, outcome) {
	if len(args) < 2 || os.Getenv(checkTokenEnv) == "" ||
		args[1] != os.Getenv(checkTokenEnv) {
		return false, outcome{}
	}
	switch args[0] {
	case "__cage_check_network":
		if len(args) != 3 {
			return true, outcome{code: exitUsage}
		}
		host, _, err := net.SplitHostPort(args[2])
		ip := net.ParseIP(host)
		if err != nil || ip == nil || !ip.IsLoopback() {
			fmt.Fprintln(s.err, "cage check probe: refusing a non-loopback address")
			return true, outcome{code: exitUsage}
		}
		conn, err := net.DialTimeout("tcp", args[2], 2*time.Second)
		if err != nil {
			fmt.Fprintln(s.err, "cage check probe:", err)
			return true, outcome{code: exitCheck}
		}
		_, err = io.WriteString(conn, args[1])
		closeErr := conn.Close()
		if err != nil || closeErr != nil {
			return true, outcome{code: exitCheck}
		}
		return true, outcome{code: exitOK}
	case "__cage_check_unavailable":
		if len(args) != 3 {
			return true, outcome{code: exitUsage}
		}
		p := policy{cwd: ".", temp: "."}
		child := []string{"/bin/sh", "-c", `printf ran > "$1"`, "cage-check", args[2]}
		return true, runBackend(unavailableBackend{
			name: "unavailable", detail: "simulated unavailable backend",
		}, p, child, s)
	}
	return false, outcome{}
}

func signalName(sig os.Signal) string {
	if sig == nil {
		return ""
	}
	return sig.String()
}

func isTerminated(result outcome) bool {
	// sandbox-exec reports a SIGTERM from its child using the shell convention
	// 128+signal. Backends that preserve wait status reach us as os.Signal.
	return signalName(result.signal) == "terminated" ||
		(result.signal == nil && result.code == 143)
}
