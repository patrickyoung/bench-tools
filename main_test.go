package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("PLY_TEST_PROGRAM") == "1" {
		os.Exit(run(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// fakeAsk stands in for the model. It is a program, like the real one, so
// the test exercises the same path production does: argv, stdin, stdout,
// and an exit status. Replies are handed out in order.
func fakeAsk(t *testing.T, replies ...string) (bin, dir string) {
	t.Helper()
	dir = t.TempDir()
	for i, r := range replies {
		write(t, filepath.Join(dir, "reply."+itoa(i+1)), r, 0o644)
	}
	bin = filepath.Join(dir, "ask")
	write(t, bin, `#!/bin/sh
d=`+dir+`
echo "$@" >> "$d/argv.log"
[ -z "${ASK_SYSTEM-}" ] || printf '%s' "$ASK_SYSTEM" > "$d/system"
cat >> "$d/stdin.log"
[ "${1-}" = note ] && exit ${FAKE_ASK_NOTE_EXIT:-0}
n=$(cat "$d/n" 2>/dev/null || echo 0); n=$((n+1)); echo "$n" > "$d/n"
[ -f "$d/reply.$n" ] && cat "$d/reply.$n"
exit ${FAKE_ASK_EXIT:-0}
`, 0o755)
	return bin, dir
}

func itoa(n int) string { return strconv.Itoa(n) }

// runPly calls the program in process, with the streams a filter's contract
// is written about pointed somewhere a test can read them.
func runPly(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	out, err1 := os.Create(filepath.Join(dir, "out"))
	errf, err2 := os.Create(filepath.Join(dir, "err"))
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	so, se := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errf
	code = run(args)
	os.Stdout, os.Stderr = so, se
	out.Close()
	errf.Close()
	return code, read(t, filepath.Join(dir, "out")), read(t, filepath.Join(dir, "err"))
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// sandbox points ply's sessions somewhere disposable and gives it a model.
func sandbox(t *testing.T, replies ...string) (workdir, plydir, askdir string) {
	t.Helper()
	bin, askdir := fakeAsk(t, replies...)
	plydir = t.TempDir()
	t.Setenv("ASK", bin)
	t.Setenv("PLY_DIR", plydir)
	return t.TempDir(), plydir, askdir
}

// TestTheLoopRunsWhatTheModelWrites is the whole program in one test: a
// reply with a block runs it, the output comes back, and a reply without
// one ends the run and is the answer.
func TestTheLoopRunsWhatTheModelWrites(t *testing.T) {
	work, _, askdir := sandbox(t,
		"I will look first.\n\n```ply\necho hello > made.txt\n```",
		"Wrote made.txt.",
	)
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "make a file")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "Wrote made.txt." {
		t.Errorf("stdout = %q, want the final reply alone", got)
	}
	if _, err := os.Stat(filepath.Join(work, "made.txt")); err != nil {
		t.Errorf("the command did not run in -C's directory: %v", err)
	}
	// The output goes back as the next message, as a typescript.
	if sent := read(t, filepath.Join(askdir, "stdin.log")); !strings.Contains(sent, "$ echo hello > made.txt") {
		t.Errorf("the model was not shown what ran:\n%s", sent)
	}
}

// TestActionTurnsAreObservedBeforeTheModelContinues is the protocol's central
// invariant. A model response is not a little batch program with a fictional
// final answer attached: Ply consumes one action, returns real terminal
// evidence, and defers the rest until the model has observed that evidence.
func TestActionTurnsAreObservedBeforeTheModelContinues(t *testing.T) {
	work, _, askdir := sandbox(t,
		"```ply\ntouch unobserved-one\n```\n```ply\ntouch unobserved-two\n```\nBoth files exist.",
		"I will do the work now.\n\n```ply\nprintf 'real evidence\\n'; touch observed\n```",
		"Created and observed the requested file.",
	)
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "make one observed change")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(work, "unobserved-one")); err != nil {
		t.Fatalf("the first action did not run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "unobserved-two")); !os.IsNotExist(err) {
		t.Errorf("a deferred action ran: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "observed")); err != nil {
		t.Fatalf("corrected action did not run: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "Created and observed the requested file." {
		t.Fatalf("stdout = %q", got)
	}
	sent := read(t, filepath.Join(askdir, "stdin.log"))
	for _, want := range []string{"ran the first command block successfully", "shell is available", "deferred and did not run", "$ touch unobserved-one", "$ printf 'real evidence", "real evidence"} {
		if !strings.Contains(sent, want) {
			t.Errorf("conversation lost %q:\n%s", want, sent)
		}
	}
}

func TestRequireActionCorrectsAProseOnlyReply(t *testing.T) {
	work, _, askdir := sandbox(t,
		"I cannot run the shell in this turn.",
		"```ply\nprintf evidence; touch made\n```",
		"Created made after observing real evidence.",
	)
	code, stdout, stderr := runPly(t, "-sh", "-require-action", "-C", work, "make a file")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(work, "made")); err != nil {
		t.Fatalf("corrected action did not run: %v", err)
	}
	if !strings.Contains(stdout, "Created made") {
		t.Fatalf("stdout=%q", stdout)
	}
	sent := read(t, filepath.Join(askdir, "stdin.log"))
	for _, want := range []string{"no command has run", "shell is available", "exactly one complete fenced ply block", "evidence"} {
		if !strings.Contains(strings.ToLower(sent), strings.ToLower(want)) {
			t.Errorf("correction lost %q:\n%s", want, sent)
		}
	}
}

func TestRequireActionStopsPersistentProseAtExitTwo(t *testing.T) {
	work, _, askdir := sandbox(t,
		"No shell.",
		"Still no shell.",
		"I remain unable to act.",
	)
	code, _, stderr := runPly(t, "-sh", "-require-action", "-C", work, "make a file")
	if code != 2 || !strings.Contains(stderr, "actionless replies") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if got := strings.TrimSpace(read(t, filepath.Join(askdir, "n"))); got != "3" {
		t.Fatalf("model calls=%q, want 3", got)
	}
}

func TestSelectedShellRunsCommandsAndChecks(t *testing.T) {
	work, _, askdir := sandbox(t,
		"```ply\ntouch built\n```",
		"Built it.",
	)
	dir := t.TempDir()
	log := filepath.Join(dir, "runs")
	shell := filepath.Join(dir, "chosen-shell")
	write(t, shell, "#!/bin/sh\nprintf 'run\\n' >> "+shellQuote(log)+"\nexec /bin/sh \"$@\"\n", 0o755)

	code, _, stderr := runPly(t, "-sh", "-shell", shell, "-C", work,
		"-check", "test -f built", "build it")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if got := strings.Count(read(t, log), "run\n"); got != 3 {
		t.Fatalf("selected shell ran %d times, want initial check, command, and final check", got)
	}
	if system := read(t, filepath.Join(askdir, "system")); !strings.Contains(system, shellQuote(shell)+" -c SCRIPT") {
		t.Fatalf("model was not told the selected shell:\n%s", system)
	}
}

func TestMissingShellFailsBeforeTheModelOrSession(t *testing.T) {
	work, plydir, askdir := sandbox(t, "Done.")
	missing := filepath.Join(t.TempDir(), "missing")
	code, _, stderr := runPly(t, "-sh", "-shell", missing, "-C", work, "goal")
	if code != 1 || !strings.Contains(stderr, "-shell") || !strings.Contains(stderr, "not executable") {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); !os.IsNotExist(err) {
		t.Fatalf("model was called despite invalid shell: %v", err)
	}
	entries, err := os.ReadDir(plydir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid shell left a session behind: %v, %v", entries, err)
	}
}

func TestPlyShellIsAnExplicitDefaultNotTheLoginShell(t *testing.T) {
	t.Setenv("SHELL", "/definitely/not/the/interpreter")
	t.Setenv("PLY_SHELL", "/bin/zsh")
	o := newOpts("ply")
	if got := *o.shellExec; got != "/bin/zsh" {
		t.Fatalf("-shell default = %q, want PLY_SHELL", got)
	}

	t.Setenv("PLY_SHELL", "")
	o = newOpts("ply")
	if got := *o.shellExec; got != defaultShell {
		t.Fatalf("-shell default = %q, want %q despite SHELL", got, defaultShell)
	}
}

func TestExplicitDelegationRunsConcurrentIsolatedChildrenAndReturnsFailures(t *testing.T) {
	if os.Getenv("PLY_TEST_PROGRAM") == "1" {
		return
	}
	work := t.TempDir()
	evidence := filepath.Join(t.TempDir(), "subagents")
	askdir := t.TempDir()
	ask := filepath.Join(askdir, "ask")
	script := `#!/bin/sh
set -eu
d=` + shellQuote(askdir) + `
session=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -f) session=$2; shift 2 ;;
    *) shift ;;
  esac
done
[ -n "$session" ] || exit 1
mkdir -p "$(dirname "$session")"
: > "$session"
printf '%s' "${ASK_MODEL-}" > "$session.model"
printf '%s' "$ASK_SYSTEM" > "$session.system"
pwd > "$session.cwd"
input="$d/input.$$"
trap 'rm -f "$input"' EXIT
cat > "$input"
case "$session" in
*/001.jsonl)
  : > "$d/alpha-start"
  n=0
  while [ ! -f "$d/beta-start" ]; do
    n=$((n+1)); [ "$n" -lt 100 ] || { : > "$d/alpha-peer-missing"; break; }
    sleep 0.01
  done
  printf 'alpha summary with file evidence\n'
  exit 0 ;;
*/002.jsonl)
  : > "$d/beta-start"
  n=0
  while [ ! -f "$d/alpha-start" ]; do
    n=$((n+1)); [ "$n" -lt 100 ] || { : > "$d/beta-peer-missing"; break; }
    sleep 0.01
  done
  printf 'beta context full\n' >&2
  exit 2 ;;
esac
if grep -q 'delegate-root' "$input"; then
  cat <<'REPLY'
Delegating: alpha code map; beta race audit.

@@FENCE@@ply
umask 077
base=${PLY_DIR:-${TMPDIR:-/tmp}}
mkdir -p "$base"
run=$(mktemp -d "$base/ply-team.XXXXXX")
(
  rc=0
  "$PLY" -sh -C . -f "$run/001.jsonl" -- alpha-task >"$run/001.out" 2>"$run/001.err" || rc=$?
  printf '%s\n' "$rc" >"$run/001.rc.tmp"
  mv "$run/001.rc.tmp" "$run/001.rc"
) &
(
  rc=0
  "$PLY" -sh -C . -f "$run/002.jsonl" -- beta-task >"$run/002.out" 2>"$run/002.err" || rc=$?
  printf '%s\n' "$rc" >"$run/002.rc.tmp"
  mv "$run/002.rc.tmp" "$run/002.rc"
) &
wait
for n in 001 002; do
  if [ ! -f "$run/$n.rc" ]; then
    printf '[%s rc=missing]\n' "$n"
    continue
  fi
  rc=$(cat "$run/$n.rc")
  printf '[%s rc=%s]\n' "$n" "$rc"
  if [ "$rc" -eq 0 ]; then
    cat "$run/$n.out"
  else
    tail -n 4 "$run/$n.err"
  fi
done
@@FENCE@@
REPLY
  exit 0
fi
if grep -q '\[001 rc=0\]' "$input"; then
  cp "$input" "$d/root-merge-input"
  printf 'Root synthesis: alpha evidence accepted; beta failed with exit 2.\n'
  exit 0
fi
printf 'unexpected turn\n' >&2
exit 1
`
	script = strings.ReplaceAll(script, "@@FENCE@@", "```")
	write(t, ask, script, 0o755)
	t.Setenv("ASK", ask)
	t.Setenv("PLY_DIR", evidence)
	t.Setenv("PLY_TEST_PROGRAM", "1")
	parent := filepath.Join(t.TempDir(), "parent.jsonl")
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-f", parent, "-m", "openai/parent-model", "delegate-root")
	if code != 0 || strings.TrimSpace(stdout) != "Root synthesis: alpha evidence accepted; beta failed with exit 2." {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "Delegating: alpha code map; beta race audit.") {
		t.Fatalf("delegation activity was not visible on stderr:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(askdir, "alpha-peer-missing")); !os.IsNotExist(err) {
		t.Fatalf("alpha did not overlap beta: %v", err)
	}
	if _, err := os.Stat(filepath.Join(askdir, "beta-peer-missing")); !os.IsNotExist(err) {
		t.Fatalf("beta did not overlap alpha: %v", err)
	}
	runs, err := filepath.Glob(filepath.Join(evidence, "ply-team.*"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs=%v err=%v", runs, err)
	}
	for _, index := range []string{"001", "002"} {
		sessionPath := filepath.Join(runs[0], index+".jsonl")
		for suffix, want := range map[string]string{".model": "openai/parent-model"} {
			if got := read(t, sessionPath+suffix); got != want {
				t.Errorf("%s%s=%q, want %q", index, suffix, got, want)
			}
		}
		gotCWD, gotErr := filepath.EvalSymlinks(strings.TrimSpace(read(t, sessionPath+".cwd")))
		wantCWD, wantErr := filepath.EvalSymlinks(work)
		if gotErr != nil || wantErr != nil || gotCWD != wantCWD {
			t.Errorf("%s cwd=%q err=%v, want %q err=%v", index, gotCWD, gotErr, wantCWD, wantErr)
		}
		if system := read(t, sessionPath+".system"); strings.Contains(system, "another ordinary ply process is a subagent") {
			t.Errorf("nested %s prompt advertised recursive delegation", index)
		}
	}
	if got := read(t, filepath.Join(runs[0], "002.rc")); strings.TrimSpace(got) != "2" {
		t.Fatalf("failed child status=%q", got)
	}
	merge := read(t, filepath.Join(askdir, "root-merge-input"))
	if !strings.Contains(merge, "[001 rc=0]\nalpha summary") || !strings.Contains(merge, "[002 rc=2]") {
		t.Fatalf("root did not receive ordered success and failure:\n%s", merge)
	}
}

func TestInterruptingParentLetsNestedPlyKillItsCommandGroup(t *testing.T) {
	work := t.TempDir()
	askdir := t.TempDir()
	ask := filepath.Join(askdir, "ask")
	childSession := filepath.Join(t.TempDir(), "child.jsonl")
	pidfile := filepath.Join(t.TempDir(), "grandchild.pid")
	script := `#!/bin/sh
set -eu
session=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -f) session=$2; shift 2 ;;
    *) shift ;;
  esac
done
mkdir -p "$(dirname "$session")"
: > "$session"
input=$(mktemp)
trap 'rm -f "$input"' EXIT
cat > "$input"
case "$session" in
*child.jsonl)
  cat <<'REPLY'
Starting a long child command.

@@FENCE@@ply
echo $$ > ` + shellQuote(pidfile) + `
sleep 30
@@FENCE@@
REPLY
  ;;
*)
  cat <<'REPLY'
Delegating: cancellable child.

@@FENCE@@ply
"$PLY" -sh -C . -f ` + shellQuote(childSession) + ` -- child-cancel
@@FENCE@@
REPLY
  ;;
esac
`
	script = strings.ReplaceAll(script, "@@FENCE@@", "```")
	write(t, ask, script, 0o755)
	parent := filepath.Join(t.TempDir(), "parent.jsonl")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-sh", "-C", work, "-f", parent, "root-cancel")
	cmd.Env = append(os.Environ(), "PLY_TEST_PROGRAM=1", "ASK="+ask)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(pidfile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Wait()
			t.Fatal("nested command never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		t.Fatal("parent did not stop after interrupt")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(read(t, pidfile)))
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Kill(pid, 0); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("nested command process %d survived parent interrupt", pid)
	}
}

// TestCheckDecidesDone: exit 0 is a program's opinion, so `ply ... && push`
// means something. The model claiming success does not end the run.
func TestCheckDecidesDone(t *testing.T) {
	work, _, askdir := sandbox(t,
		"All done!", // a lie, and the check catches it
		"```ply\ntouch built\n```",
		"Built it.",
	)
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-check", "test -f "+filepath.Join(work, "built"), "build it")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "Built it." {
		t.Errorf("stdout = %q", got)
	}
	sent := read(t, filepath.Join(askdir, "stdin.log"))
	if !strings.Contains(sent, "check and it did not pass") {
		t.Errorf("the model was never told the check failed:\n%s", sent)
	}
}

// TestInitialCheckFailureIsFirstTurnEvidence pins the difference between a
// pre-check and an optimisation. Its diagnostic is evidence: the model sees
// it without rediscovering the failure, Ask records it, and it does not spend
// one of the post-turn check cycles.
func TestInitialCheckFailureIsFirstTurnEvidence(t *testing.T) {
	work, _, askdir := sandbox(t,
		"```ply\ntouch built\n```",
		"Built it.",
	)
	check := "test -f built || { echo baseline failure >&2; exit 1; }"
	code, _, stderr := runPly(t, "-sh", "-C", work, "-cycles", "1",
		"-check", check, "build it")
	if code != 0 {
		t.Fatalf("exit = %d, want 0: the pre-check spent the only cycle\n%s", code, stderr)
	}
	sent := read(t, filepath.Join(askdir, "stdin.log"))
	for _, want := range []string{"build it", initialCheckStart, "$ " + check,
		"baseline failure", "exit 1", initialCheckEnd} {
		if !strings.Contains(sent, want) {
			t.Errorf("first-turn evidence lost %q:\n%s", want, sent)
		}
	}
	if n := strings.Count(sent, initialCheckStart); n != 1 {
		t.Errorf("initial check was recorded %d times, want once:\n%s", n, sent)
	}
	if !strings.Contains(stderr, "check failed") || !strings.Contains(stderr, "baseline failure") {
		t.Errorf("the live typescript hid the pre-check failure:\n%s", stderr)
	}
}

// A check can judge the report itself, which is the artifact for a question.
// The pre-check receives EOF; each later check receives the candidate stdout
// as a newline-terminated text stream.
func TestCheckReceivesCandidateReportOnStdin(t *testing.T) {
	work, _, askdir := sandbox(t,
		"The answer is probably 41.",
		"The answer is 42.",
	)
	check := `answer=$(cat)
[ "$answer" = "The answer is 42." ] || {
  printf 'expected the supported answer; got: %s\n' "$answer" >&2
  exit 1
}`
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-check", check,
		"answer the question")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if stdout != "The answer is 42.\n" {
		t.Errorf("stdout = %q", stdout)
	}
	sent := read(t, filepath.Join(askdir, "stdin.log"))
	if !strings.Contains(sent, "expected the supported answer; got: The answer is probably 41.") {
		t.Errorf("candidate rejection did not reach the model:\n%s", sent)
	}
}

// Exit 1 means the candidate failed. Every other nonzero status means the
// verifier failed, so feeding it back as ordinary work would make the model
// repair its judge or loop forever on infrastructure.
func TestBrokenCheckStopsInsteadOfBecomingWork(t *testing.T) {
	t.Run("initial", func(t *testing.T) {
		work, plydir, askdir := sandbox(t, "should never be asked")
		code, _, stderr := runPly(t, "-sh", "-C", work, "-check",
			"echo verifier setup failed >&2; exit 7", "goal")
		if code != 1 || !strings.Contains(stderr, "check broken") || !strings.Contains(stderr, "exit 7") {
			t.Fatalf("exit = %d, stderr = %q", code, stderr)
		}
		if _, err := os.Stat(filepath.Join(askdir, "n")); !os.IsNotExist(err) {
			t.Fatalf("model was called despite broken pre-check: %v", err)
		}
		if entries, err := os.ReadDir(plydir); err != nil || len(entries) != 0 {
			t.Fatalf("broken pre-check left a session: %v, %v", entries, err)
		}
	})

	t.Run("after report", func(t *testing.T) {
		work, _, askdir := sandbox(t, "A candidate report.", "should not be asked")
		check := `if [ -s /dev/stdin ]; then
  echo verifier crashed >&2
  exit 7
fi
exit 1`
		code, stdout, stderr := runPly(t, "-sh", "-C", work, "-check", check, "goal")
		if code != 1 || stdout != "A candidate report.\n" {
			t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "check broken") || !strings.Contains(stderr, "verifier crashed") {
			t.Errorf("broken verifier was not diagnosed:\n%s", stderr)
		}
		if got := strings.TrimSpace(read(t, filepath.Join(askdir, "n"))); got != "1" {
			t.Errorf("model calls = %s, want 1", got)
		}
		if argv := read(t, filepath.Join(askdir, "argv.log")); !strings.Contains(argv, "-k "+verifierReceiptKind) {
			t.Errorf("a broken verifier did not write a structured receipt:\n%s", argv)
		}
	})
}

func TestVerifierOutputBeyondEvidenceCapIsBroken(t *testing.T) {
	work, _, askdir := sandbox(t, "candidate")
	check := `if [ -s /dev/stdin ]; then yes x | head -c 2000; exit 0; fi
exit 1`
	code, _, stderr := runPly(t, "-sh", "-C", work, "-cap", "512", "-check", check, "goal")
	if code != 1 || !strings.Contains(stderr, "output exceeded the evidence cap") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if got := strings.TrimSpace(read(t, filepath.Join(askdir, "n"))); got != "1" {
		t.Fatalf("model calls=%s, want 1", got)
	}
	if receipt := read(t, filepath.Join(askdir, "stdin.log")); !strings.Contains(receipt, `"outcome":"broken"`) || !strings.Contains(receipt, `"elided_bytes":`) {
		t.Fatalf("broken receipt = %s", receipt)
	}
}

// TestTheCheckIsNotScopedByTheToolbox: the toolbox exists to aim the model.
// The check is the caller's own program, and scoping it to three symlinks
// would mean `go test ./...` needed a toolbox holding go, git and a linker.
func TestTheCheckIsNotScopedByTheToolbox(t *testing.T) {
	work, _, _ := sandbox(t, "done")
	box := t.TempDir()
	write(t, filepath.Join(box, "onlytool"), "#!/bin/sh\n# the model's one tool\n", 0o755)

	// `env` is on the machine and not in the toolbox: a real check needs the
	// machine, and a model command must not.
	code, _, stderr := runPly(t, "-t", box, "-C", work, "-check", "env >/dev/null", "goal")
	if code != 0 {
		t.Fatalf("exit = %d, want 0: the check could not reach its own PATH\n%s", code, stderr)
	}
	// The toolbox still comes first, so a program in it can be the check.
	if code, _, _ := runPly(t, "-t", box, "-C", work, "-check", "onlytool", "goal"); code != 0 {
		t.Errorf("a toolbox program could not be used as the check: exit %d", code)
	}
}

// TestUnverifiedIsExitTwo: a supervisor has to tell "did not work" from
// "broke", or it retries the wrong one.
func TestUnverifiedIsExitTwo(t *testing.T) {
	work, _, _ := sandbox(t, "done", "done", "done", "done", "done", "done", "done")
	code, stdout, _ := runPly(t, "-sh", "-C", work, "-cycles", "2", "-check", "false", "impossible")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if strings.TrimSpace(stdout) != "done" {
		t.Errorf("stdout = %q: a run that fell short still has something to say", stdout)
	}
}

// TestNothingToBeDone: make's oldest manners. A goal already met costs no
// model call and leaves no session behind, which is what makes ply safe in
// a hook, a loop, or a Makefile.
func TestNothingToBeDone(t *testing.T) {
	work, plydir, askdir := sandbox(t, "should never be asked")
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-check", "true", "already true")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing: no work was done", stdout)
	}
	if !strings.Contains(stderr, "nothing to do") {
		t.Errorf("stderr = %q, want it to say so", stderr)
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); err == nil {
		t.Error("the model was called for a goal that was already met")
	}
	if ents, _ := os.ReadDir(plydir); len(ents) != 0 {
		t.Errorf("left %d files behind: %v", len(ents), ents)
	}
	// -B is for when you meant it anyway.
	if code, _, _ := runPly(t, "-sh", "-C", work, "-B", "-check", "true", "again"); code != 0 {
		t.Fatalf("-B exit = %d", code)
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); err != nil {
		t.Error("-B did not work the goal")
	}
}

// TestSessionInTheWorkTreeIsSaidOutLoud: a session under the directory
// commands run in is one `grep -r` away from being fed back to the model
// that wrote it. The default never lands there; -f can, and does so
// quietly, which is the kind of surprise this family does not have.
func TestSessionInTheWorkTreeIsSaidOutLoud(t *testing.T) {
	work, _, _ := sandbox(t, "done")
	inside := filepath.Join(work, "run.jsonl")
	_, _, stderr := runPly(t, "-sh", "-C", work, "-f", inside, "a goal")
	if !strings.Contains(stderr, "inside the work tree") {
		t.Errorf("a session at %s went unmentioned; stderr = %q", inside, stderr)
	}

	// The default session directory is not in the tree, and saying so about
	// it would be noise on every single run.
	_, _, stderr = runPly(t, "-sh", "-C", work, "another goal")
	if strings.Contains(stderr, "inside the work tree") {
		t.Errorf("the default session was called a work-tree session; stderr = %q", stderr)
	}

	// Neither is a sibling directory whose name happens to start the same
	// way: string prefixes are not path containment.
	outside := filepath.Join(filepath.Dir(work), filepath.Base(work)+"-logs")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, stderr = runPly(t, "-sh", "-C", work, "-f", filepath.Join(outside, "run.jsonl"), "third goal")
	if strings.Contains(stderr, "inside the work tree") {
		t.Errorf("%s was called a work-tree session; stderr = %q", outside, stderr)
	}
}

// TestBadInvocationLeavesNoLitter: a filter must not create a session for a
// call that was never going to run.
func TestBadInvocationLeavesNoLitter(t *testing.T) {
	work, plydir, _ := sandbox(t, "x")
	for _, args := range [][]string{
		{"-C", work, "no toolbox and no -sh"},
		{"-sh", "-C", filepath.Join(work, "nope"), "bad -C"},
		{"-sh", "-t", filepath.Join(work, "nope"), "bad -t"},
		{"-sh", "-C", work, "-cap", "10", "silly cap"},
		{"-sh", "-C", work},
	} {
		code, stdout, _ := runPly(t, args...)
		if code != 1 {
			t.Errorf("%v: exit = %d, want 1", args, code)
		}
		if stdout != "" {
			t.Errorf("%v: wrote %q to stdout", args, stdout)
		}
	}
	if ents, _ := os.ReadDir(plydir); len(ents) != 0 {
		t.Errorf("a refused invocation left %v behind", ents)
	}
}

// TestContextFullIsExitTwo: ask's exit 2 exists so a retry loop can stop.
// ply is that loop.
func TestContextFullIsExitTwo(t *testing.T) {
	work, _, _ := sandbox(t, "unused")
	t.Setenv("FAKE_ASK_EXIT", "2")
	if code, _, _ := runPly(t, "-sh", "-C", work, "goal"); code != 2 {
		t.Errorf("exit = %d, want 2 for a full context window", code)
	}
}

// TestCompactCarriesOnThroughAFullWindow: a full context window is
// permanent, so without -compact it ends the run at exit 2. With it, ask
// writes a handoff note into a fresh session and the loop moves into it —
// and re-sends the message the full session could not take, so nothing is
// dropped on the way across.
func TestCompactCarriesOnThroughAFullWindow(t *testing.T) {
	work, _, askdir := sandbox(t, "unused")
	// A fake ask that is full until it has been compacted, and answers
	// afterwards. `compact` prints the new session's path, as the real one
	// does.
	fresh := filepath.Join(t.TempDir(), "fresh.jsonl")
	write(t, filepath.Join(askdir, "ask"), `#!/bin/sh
d=`+askdir+`
for a in "$@"; do
  if [ "$a" = compact ]; then echo compacted >> "$d/compacted"; echo `+fresh+`; exit 0; fi
done
echo "$@" >> "$d/argv.log"
cat >> "$d/stdin.log"
[ -f "$d/compacted" ] || { echo "ask: context window is full" >&2; exit 2; }
echo done
`, 0o755)

	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-compact", "the goal")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "done" {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(askdir, "compacted")); err != nil {
		t.Fatal("the loop never compacted; a full window ended the run")
	}
	if !strings.Contains(stderr, "compacted into") {
		t.Errorf("stderr does not report the compaction:\n%s", stderr)
	}
	// The turn that overflowed is sent again, into the new session.
	if sent := read(t, filepath.Join(askdir, "stdin.log")); strings.Count(sent, "the goal") != 2 {
		t.Errorf("the message was not re-sent after compacting:\n%s", sent)
	}
	if argv := read(t, filepath.Join(askdir, "argv.log")); !strings.Contains(argv, fresh) {
		t.Errorf("the loop did not move into the compacted session:\n%s", argv)
	}
}

// TestCompactionIsBounded: a goal that needs more than a few compactions
// has outgrown one run, and the loop should say so rather than summarise
// forever.
func TestCompactionIsBounded(t *testing.T) {
	work, _, askdir := sandbox(t, "unused")
	write(t, filepath.Join(askdir, "ask"), `#!/bin/sh
for a in "$@"; do
  if [ "$a" = compact ]; then echo `+filepath.Join(t.TempDir(), "n.jsonl")+`; exit 0; fi
done
echo "ask: context window is full" >&2; exit 2
`, 0o755)
	code, _, stderr := runPly(t, "-sh", "-C", work, "-compact", "-compactions", "2", "goal")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "compactions") {
		t.Errorf("stderr does not name the cap:\n%s", stderr)
	}
}

func TestSessionOutTracksTheCurrentSession(t *testing.T) {
	work, _, askdir := sandbox(t, "unused")
	fresh := filepath.Join(t.TempDir(), "fresh.jsonl")
	write(t, filepath.Join(askdir, "ask"), `#!/bin/sh
d=`+askdir+`
for a in "$@"; do
  if [ "$a" = compact ]; then touch "$d/compacted"; echo `+fresh+`; exit 0; fi
done
cat >/dev/null
[ -f "$d/compacted" ] || exit 2
echo done
`, 0o755)
	control := filepath.Join(t.TempDir(), "current")
	code, _, stderr := runPly(t, "-sh", "-C", work, "-compact",
		"-session-out", control, "goal")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	want, err := filepath.Abs(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(read(t, control)); got != want {
		t.Errorf("session-out = %q, want %q", got, want)
	}
	if fi, err := os.Stat(control); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("session-out mode: info=%v err=%v, want 0600", fi, err)
	}
}

func TestSessionOutExistsBeforeTheFirstTurn(t *testing.T) {
	work, _, askdir := sandbox(t, "unused")
	session := filepath.Join(t.TempDir(), "named.jsonl")
	control := filepath.Join(t.TempDir(), "current")
	write(t, filepath.Join(askdir, "ask"), `#!/bin/sh
want=`+session+`
control=`+control+`
[ "$(cat "$control")" = "$want" ] || { echo missing-session-control >&2; exit 1; }
cat >/dev/null
echo done
`, 0o755)
	code, _, stderr := runPly(t, "-sh", "-C", work, "-f", session,
		"-session-out", control, "goal")
	if code != 0 {
		t.Fatalf("exit = %d: the control file was not ready before Ask\n%s", code, stderr)
	}
}

func TestSessionOutFailureIsInfrastructureFailure(t *testing.T) {
	work, _, askdir := sandbox(t, "should not run")
	control := filepath.Join(t.TempDir(), "missing", "current")
	code, _, stderr := runPly(t, "-sh", "-C", work, "-session-out", control, "goal")
	if code != 1 || !strings.Contains(stderr, "-session-out") {
		t.Fatalf("exit = %d stderr = %q, want an ordinary infrastructure error", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); err == nil {
		t.Error("Ask ran after the requested control channel failed")
	}
}

func TestSessionOutCannotOverwriteTheSession(t *testing.T) {
	work, _, askdir := sandbox(t, "should not run")
	session := filepath.Join(t.TempDir(), "task.jsonl")
	code, _, stderr := runPly(t, "-sh", "-C", work, "-f", session,
		"-session-out", session, "goal")
	if code != 1 || !strings.Contains(stderr, "must not name the Ask session") {
		t.Fatalf("exit = %d stderr = %q", code, stderr)
	}
	if _, err := os.Stat(session); err == nil {
		t.Error("the control artifact overwrote the Ask session path")
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); err == nil {
		t.Error("Ask ran after the colliding paths were refused")
	}
}

func TestModelFailureIsExitOne(t *testing.T) {
	work, _, _ := sandbox(t, "unused")
	t.Setenv("FAKE_ASK_EXIT", "1")
	if code, _, _ := runPly(t, "-sh", "-C", work, "goal"); code != 1 {
		t.Errorf("exit = %d, want 1 for a broken provider", code)
	}
}

func TestMalformedCommandProtocolIsExitTwo(t *testing.T) {
	work, _, _ := sandbox(t,
		"```ply\ntouch first",
		"```ply\ntouch second",
		"```ply\ntouch third",
	)
	code, _, stderr := runPly(t, "-sh", "-C", work, "goal")
	if code != 2 || !strings.Contains(stderr, "command protocol stalled") {
		t.Fatalf("exit = %d stderr = %q, want a protocol stall", code, stderr)
	}
	for _, name := range []string{"first", "second", "third"} {
		if _, err := os.Stat(filepath.Join(work, name)); err == nil {
			t.Errorf("the truncated %s command ran", name)
		}
	}
}

func TestBoundsRejectNegativeValues(t *testing.T) {
	for _, args := range [][]string{
		{"-sh", "-turns", "-1", "goal"},
		{"-sh", "-cycles", "-1", "goal"},
		{"-sh", "-compactions", "-1", "goal"},
		{"-sh", "-timeout", "0", "goal"},
	} {
		code, _, stderr := runPly(t, args...)
		if code != 1 || !strings.Contains(stderr, "usage:") {
			t.Errorf("%v: exit = %d stderr = %q, want a usage error", args, code, stderr)
		}
	}
}

func TestTurnLimitHasFiniteDefault(t *testing.T) {
	o := newOpts("ply")
	if *o.turns != defaultTurns || *o.turns <= 0 {
		t.Fatalf("default turns = %d, want finite %d", *o.turns, defaultTurns)
	}
}

func TestDefaultTurnLimitStopsContinuousCommands(t *testing.T) {
	work, _, askdir := sandbox(t, "unused")
	write(t, filepath.Join(askdir, "ask"), `#!/bin/sh
d=`+askdir+`
cat >/dev/null
n=$(cat "$d/n" 2>/dev/null || echo 0)
n=$((n+1))
echo "$n" > "$d/n"
printf '%s\n' '`+"```ply"+`' ':' '`+"```"+`'
`, 0o755)
	code, _, stderr := runPly(t, "-sh", "-C", work, "never stop")
	if code != 2 || !strings.Contains(stderr, "turn limit reached") {
		t.Fatalf("exit = %d stderr = %q, want the default turn bound", code, stderr)
	}
	if got := strings.TrimSpace(read(t, filepath.Join(askdir, "n"))); got != itoa(defaultTurns) {
		t.Errorf("model calls = %q, want %d", got, defaultTurns)
	}
}

// TestSystemPromptTravelsInTheEnvironment: it is long, it can carry a
// private procedure brief just printed, and argv is world-readable in ps(1).
func TestSystemPromptTravelsInTheEnvironment(t *testing.T) {
	work, _, askdir := sandbox(t, "done")
	runPly(t, "-sh", "-C", work, "goal")
	if sys := read(t, filepath.Join(askdir, "system")); !strings.Contains(sys, "fenced shell block") {
		t.Errorf("ASK_SYSTEM did not carry the protocol:\n%s", sys)
	}
	if argv := read(t, filepath.Join(askdir, "argv.log")); strings.Contains(argv, "-S") {
		t.Errorf("the system prompt went through argv: %s", argv)
	}
}

func TestReasoningEffortPassesLiterallyToAsk(t *testing.T) {
	work, _, askdir := sandbox(t, "done")
	code, _, stderr := runPly(t, "-sh", "-C", work, "-effort", "xhigh", "goal")
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	argv := read(t, filepath.Join(askdir, "argv.log"))
	if !strings.Contains(argv, "-effort xhigh") {
		t.Fatalf("ask argv lost effort: %q", argv)
	}
}

// TestStdinIsTheGoalOrRidesWithIt, per ask and mu both.
func TestGoalComesFromArgvOrStdin(t *testing.T) {
	work, _, askdir := sandbox(t, "done")
	runPly(t, "-sh", "-C", work, "the goal in argv")
	if sent := read(t, filepath.Join(askdir, "stdin.log")); !strings.Contains(sent, "the goal in argv") {
		t.Errorf("the goal never reached the model:\n%s", sent)
	}
}

func TestDepthIsBounded(t *testing.T) {
	work, _, _ := sandbox(t, "done")
	t.Setenv("PLY_DEPTH", "8")
	code, _, stderr := runPly(t, "-sh", "-C", work, "goal")
	if code != 1 || !strings.Contains(stderr, "deep inside itself") {
		t.Errorf("exit = %d, stderr = %q; a ply that starts ply must have a floor", code, stderr)
	}
}

// ---- the documentation is part of the program ----

func flags(t *testing.T) []string {
	t.Helper()
	var names []string
	newOpts("ply").fs.VisitAll(func(f *flag.Flag) { names = append(names, f.Name) })
	return names
}

func verbs() []string { return []string{"tools", "system", "version", "help"} }

func TestHelpFitsEightyColumns(t *testing.T) {
	for i, line := range strings.Split(help, "\n") {
		if len(line) > 80 {
			t.Errorf("help line %d is %d columns:\n%s", i+1, len(line), line)
		}
	}
}

func TestEveryFlagAndVerbIsDocumented(t *testing.T) {
	man := read(t, "ply.1")
	readme := read(t, "README.md")
	for _, f := range flags(t) {
		if !strings.Contains(help, "-"+f) {
			t.Errorf("flag -%s is not in ply help", f)
		}
		if !strings.Contains(man, "-"+f) {
			t.Errorf("flag -%s is not in ply.1", f)
		}
	}
	// Guard the whole as well as the parts: a flag-level check stays green
	// while an entire verb goes undocumented.
	for _, v := range verbs() {
		for name, doc := range map[string]string{"ply help": help, "ply.1": man, "README.md": readme} {
			if !strings.Contains(doc, "ply "+v) {
				t.Errorf("verb %q is not in %s", v, name)
			}
		}
	}
	for _, env := range []string{"PLY_TOOLS", "PLY_SHELL", "PLY_EFFORT", "PLY_DIR", "PLY_DEPTH", "ASK", "BRIEF", "NO_COLOR"} {
		if !strings.Contains(help, env) {
			t.Errorf("%s is not in ply help", env)
		}
		if !strings.Contains(man, env) {
			t.Errorf("%s is not in ply.1", env)
		}
	}
}

func TestManPageIsPlainAsciiAndCurrent(t *testing.T) {
	man := read(t, "ply.1")
	if !strings.Contains(man, version) {
		t.Errorf("ply.1 does not carry version %s", version)
	}
	for i, r := range man {
		if r > 127 {
			t.Fatalf("ply.1 has a non-ASCII rune %q at byte %d", r, i)
		}
	}
}

// TestHelpGoesToStdoutMisuseToStderr: a program that prints its help to
// stderr cannot be piped into less by the person who typed -h.
func TestHelpGoesToStdoutMisuseToStderr(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		code, stdout, stderr := runPly(t, arg)
		if code != 0 || !strings.HasPrefix(stdout, "ply") || stderr != "" {
			t.Errorf("%s: exit %d, stdout %d bytes, stderr %q", arg, code, len(stdout), stderr)
		}
	}
	code, stdout, stderr := runPly(t, "-nosuchflag", "goal")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "usage:") {
		t.Errorf("misuse: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestVersionIsOneLine(t *testing.T) {
	for _, arg := range []string{"version", "-V", "--version"} {
		code, stdout, _ := runPly(t, arg)
		if code != 0 || strings.TrimSpace(stdout) != "ply "+version {
			t.Errorf("%s: exit %d, stdout %q", arg, code, stdout)
		}
	}
}

// TestSystemVerbPrintsWhatWouldBeSent: it is a value, not a secret, so
// extending it is ordinary shell rather than a config format.
func TestSystemAndToolsVerbsShowTheRealThing(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "deploy"), "#!/bin/sh\n# push it\n", 0o755)

	code, stdout, _ := runPly(t, "system", "-t", dir, "-check", "make test")
	if code != 0 || !strings.Contains(stdout, "deploy") || !strings.Contains(stdout, "make test") {
		t.Errorf("ply system: exit %d, missing the toolbox or the check:\n%s", code, stdout)
	}
	code, stdout, _ = runPly(t, "tools", "-t", dir)
	if code != 0 || !strings.Contains(stdout, "push it") || !strings.Contains(stdout, "PATH="+dir) {
		t.Errorf("ply tools: exit %d:\n%s", code, stdout)
	}
	if code, _, stderr := runPly(t, "tools"); code != 1 || !strings.Contains(stderr, "no tools") {
		t.Errorf("ply tools with no toolbox: exit %d, %q", code, stderr)
	}
}

// TestTheVerdictLandsInTheLog is the promise ply has always made in its own
// AGENTS.md — "a command, its output, its exit status, the check's verdict"
// goes in the conversation — and did not keep. Without it a session holds
// every command that ran and nothing about whether the work was done, so a
// run that passed and a run that gave up are the same shape on disk and
// nothing reading the log afterwards can tell them apart.
func TestTheVerdictLandsInTheLog(t *testing.T) {
	verdict := func(t *testing.T, askdir string) (argv, text string) {
		t.Helper()
		return read(t, filepath.Join(askdir, "argv.log")), read(t, filepath.Join(askdir, "stdin.log"))
	}

	t.Run("passing", func(t *testing.T) {
		work, _, askdir := sandbox(t, "```ply\ntouch built\n```", "Built it.")
		code, _, stderr := runPly(t, "-sh", "-C", work, "-check", "test -f "+filepath.Join(work, "built"), "build it")
		if code != 0 {
			t.Fatalf("exit = %d\n%s", code, stderr)
		}
		argv, text := verdict(t, askdir)
		if !strings.Contains(argv, "note") || !strings.Contains(argv, "-s ply") {
			t.Errorf("ply never wrote a note:\n%s", argv)
		}
		for _, want := range []string{`"outcome":"accepted"`, `"phase":"candidate"`, `"candidate_sha256":`, `"verifier_sha256":`} {
			if !strings.Contains(text, want) {
				t.Errorf("the receipt is missing %q:\n%s", want, text)
			}
		}
	})

	t.Run("failing", func(t *testing.T) {
		work, _, askdir := sandbox(t, "done", "done", "done")
		if code, _, _ := runPly(t, "-sh", "-C", work, "-cycles", "1", "-check", "false", "impossible"); code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		argv, text := verdict(t, askdir)
		if !strings.Contains(argv, "note") {
			t.Errorf("a run that gave up recorded nothing:\n%s", argv)
		}
		if !strings.Contains(text, `"outcome":"rejected"`) {
			t.Errorf("the rejected receipt is not in what was written:\n%s", text)
		}
	})

	// No check, no verdict. "Done is a program's opinion" — with no program
	// there is no opinion to record, and the absence is the signal: nothing
	// reading the log later should mistake the model's word for a check.
	t.Run("no check", func(t *testing.T) {
		work, _, askdir := sandbox(t, "I think I am done.")
		if code, _, _ := runPly(t, "-sh", "-C", work, "no check at all"); code != 0 {
			t.Fatal("exit")
		}
		if argv, _ := verdict(t, askdir); strings.Contains(argv, "note") {
			t.Errorf("a run with no check claimed a verdict:\n%s", argv)
		}
	})

	// Each verifier execution gets one receipt, including an intermediate
	// rejection. The rejection is also folded as feedback, but the receipt is
	// what binds it to the exact candidate and verifier bytes.
	t.Run("recovered runs record once", func(t *testing.T) {
		work, _, askdir := sandbox(t,
			"All done!", // the check catches the lie
			"```ply\ntouch built\n```",
			"Built it.",
		)
		if code, _, _ := runPly(t, "-sh", "-C", work, "-check", "test -f "+filepath.Join(work, "built"), "-sh", "build it"); code != 0 {
			t.Fatal("exit")
		}
		_, text := verdict(t, askdir)
		if n := strings.Count(text, `"outcome":"rejected"`); n != 1 {
			t.Errorf("intermediate rejection receipts = %d, want 1", n)
		}
		if n := strings.Count(text, `"outcome":"accepted"`); n != 1 {
			t.Errorf("accepted receipts = %d, want 1", n)
		}
	})
}

func TestAcceptedOutcomeRequiresDurableReceipt(t *testing.T) {
	work, _, askdir := sandbox(t, "A supported answer.")
	t.Setenv("FAKE_ASK_NOTE_EXIT", "1")
	code, _, stderr := runPly(t, "-sh", "-B", "-C", work, "-check", "true", "answer")
	if code != 1 || !strings.Contains(stderr, "record verifier receipt") {
		t.Fatalf("exit=%d stderr=%q, want evidence persistence failure", code, stderr)
	}
	if got := strings.TrimSpace(read(t, filepath.Join(askdir, "n"))); got != "1" {
		t.Fatalf("model calls=%s, want 1", got)
	}
}

func TestPassingPrecheckRecordsReceiptInExistingContractSession(t *testing.T) {
	work, _, askdir := sandbox(t)
	session := filepath.Join(t.TempDir(), "contract.jsonl")
	write(t, session, "existing contract session\n", 0o600)
	code, _, stderr := runPly(t, "-sh", "-C", work, "-f", session,
		"-contract-id", "sha256:contract", "-check", "true", "already done")
	if code != 0 || !strings.Contains(stderr, "nothing to do") {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); !os.IsNotExist(err) {
		t.Fatalf("precheck called model: %v", err)
	}
	text := read(t, filepath.Join(askdir, "stdin.log"))
	for _, want := range []string{`"phase":"baseline"`, `"outcome":"accepted"`, `"contract_id":"sha256:contract"`} {
		if !strings.Contains(text, want) {
			t.Errorf("baseline receipt missing %q: %s", want, text)
		}
	}
}

// TestWhatWasLoadedLandsInTheLog closes the joint between brief and hone.
// `brief cat` prints a body without its frontmatter, so a skill reaches the
// system prompt as anonymous prose: what shaped a run is provable byte for
// byte, but nothing says which of those bytes were a skill called
// web-perf. Its name lived only on stderr, where it died with the terminal.
func TestWhatWasLoadedLandsInTheLog(t *testing.T) {
	work, _, askdir := sandbox(t, "```ply\ntouch built\n```", "Built it.")
	skills := t.TempDir()
	if err := os.MkdirAll(filepath.Join(skills, "house"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(skills, "house", "SKILL.md"),
		"---\nname: house\ndescription: House rules. Use when working here.\n---\n\n# house\n\nUse tabs.\n", 0o644)
	t.Setenv("BRIEF_PATH", skills)
	briefBin := filepath.Join(t.TempDir(), "brief")
	write(t, briefBin, "#!/bin/sh\n[ \"$1\" = cat ] && cat "+filepath.Join(skills, "$2", "SKILL.md")+"\n", 0o755)
	t.Setenv("BRIEF", briefBin)

	code, _, stderr := runPly(t, "-sh", "-C", work, "-s", "house",
		"-check", "test -f "+filepath.Join(work, "built"), "build it")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "Brief procedure house loaded (named)") {
		t.Fatalf("typescript hid the loaded procedure:\n%s", stderr)
	}
	argv := read(t, filepath.Join(askdir, "argv.log"))
	if !strings.Contains(argv, "note") {
		t.Fatalf("no note was written:\n%s", argv)
	}
	sent := read(t, filepath.Join(askdir, "stdin.log"))
	if !strings.Contains(sent, "loaded skill house (named)") {
		t.Errorf("the log does not say which skill was loaded:\n%s", sent)
	}
	system := read(t, filepath.Join(askdir, "system"))
	procedure := strings.Index(system, "Use tabs.")
	reminder := strings.Index(system, "PLY ACTION PROTOCOL REMINDER")
	if procedure < 0 || reminder < procedure || !strings.Contains(system[reminder:], "exactly one complete fenced ply block") {
		t.Fatalf("skill hid the trailing action protocol:\n%s", system)
	}

	// A run with no -s claims nothing. The absence is the signal, the same
	// way it is for a run with no check.
	work2, _, askdir2 := sandbox(t, "Done.")
	if code, _, _ := runPly(t, "-sh", "-C", work2, "no skill"); code != 0 {
		t.Fatal("exit")
	}
	if s := read(t, filepath.Join(askdir2, "stdin.log")); strings.Contains(s, "loaded skill") {
		t.Errorf("a run with no -s claimed a skill:\n%s", s)
	}
}

// TestResumingIsRunningItAgain pins the contract the field guide states:
// the state of the work is the work tree, not the conversation. A killed run
// is resumed by running it again, -f continues the conversation rather than
// starting over, and the pre-check makes re-entry free once the work is
// actually done. None of that is a feature -- it is three existing
// properties that together mean there is no task record to keep. Without a
// test they are folklore, and folklore is not a contract.
func TestResumingIsRunningItAgain(t *testing.T) {
	work, _, askdir := sandbox(t,
		"```ply\ntouch "+filepath.Join(t.TempDir(), "unrelated")+"\n```",
		"Still working on it.",
		"should never be asked",
	)
	sess := filepath.Join(t.TempDir(), "run.jsonl")
	done := filepath.Join(work, "done")
	check := "test -f " + done

	// A run that does not finish: the check never passes, the cap trips,
	// exit 2 says not-done rather than broken.
	code, _, stderr := runPly(t, "-sh", "-C", work, "-f", sess,
		"-check", check, "-cycles", "1", "finish the job")
	if code != 2 {
		t.Fatalf("unfinished run exit = %d, want 2\n%s", code, stderr)
	}
	calls := read(t, filepath.Join(askdir, "n"))

	// The work gets done out of band -- by a later turn, another process, or
	// a human. The point is that the tree changed and the transcript did not.
	write(t, done, "", 0o644)

	// Running it again is the resume. The pre-check sees the finished work,
	// so it costs nothing: no model call, and the same session untouched.
	code, stdout, stderr := runPly(t, "-sh", "-C", work, "-f", sess,
		"-check", check, "-cycles", "1", "finish the job")
	if code != 0 {
		t.Fatalf("resumed run exit = %d, want 0\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "nothing to do") {
		t.Errorf("stderr = %q, want the pre-check to have short-circuited it", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing: there was no work to do", stdout)
	}
	if now := read(t, filepath.Join(askdir, "n")); now != calls {
		t.Errorf("the model was called %s times, was %s: re-entry was not free", now, calls)
	}
}

// TestResumingContinuesTheConversation: -f on an existing session appends to
// it rather than starting a new one. That is what makes a resumed run
// cheaper than a fresh one, and it is why ply must never pass ask -n.
func TestResumingContinuesTheConversation(t *testing.T) {
	work, _, askdir := sandbox(t, "First.", "Second.")
	sess := filepath.Join(t.TempDir(), "run.jsonl")

	if code, _, e := runPly(t, "-sh", "-C", work, "-f", sess, "a goal"); code != 0 {
		t.Fatalf("exit = %d\n%s", code, e)
	}
	if code, _, e := runPly(t, "-sh", "-C", work, "-f", sess, "a goal"); code != 0 {
		t.Fatalf("exit = %d\n%s", code, e)
	}

	argv := read(t, filepath.Join(askdir, "argv.log"))
	if strings.Contains(argv, "-n ") || strings.HasSuffix(strings.TrimSpace(argv), "-n") {
		t.Errorf("ply passed ask -n, which would start over instead of resuming:\n%s", argv)
	}
	if n := strings.Count(argv, "-f "+sess); n != 2 {
		t.Errorf("named the session %d times, want 2:\n%s", n, argv)
	}
}

// TestTheCapabilityExampleIsDiscoverable: contrib/capability/fix-tests is
// the field guide's worked example of a capability, and the claim it makes
// is that nothing had to be built -- a directory of scripts is a toolbox and
// line 2 is the catalogue entry. So the example has to actually be one.
func TestTheCapabilityExampleIsDiscoverable(t *testing.T) {
	const dir = "contrib/capability"
	code, stdout, stderr := runPly(t, "tools", "-t", dir)
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "fix-tests") {
		t.Errorf("the example is not in its own catalogue:\n%s", stdout)
	}
	if !strings.Contains(stdout, "make the Go tests in this tree pass") {
		t.Errorf("the example has no synopsis in the catalogue:\n%s", stdout)
	}
	// The name is the filename, so a synopsis that repeats it prints twice.
	// contrib/edit says so in its own header; the example must obey it.
	if strings.Contains(stdout, "fix-tests  fix-tests") {
		t.Errorf("the synopsis repeats the name:\n%s", stdout)
	}
	fi, err := os.Stat(filepath.Join(dir, "fix-tests"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Error("the example is not executable, so it is not a tool")
	}
}
