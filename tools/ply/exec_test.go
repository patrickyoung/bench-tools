package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommandsExtractsShellBlocks(t *testing.T) {
	cases := []struct {
		name  string
		reply string
		want  []string
		note  bool
	}{
		{
			name:  "the canonical block",
			reply: "I will look.\n\n```ply\nls -la\n```\n",
			want:  []string{"ls -la"},
		},
		{
			name:  "only the first of several recognized shell blocks runs",
			reply: "```sh\na\n```\ntext\n```bash\nb\n```\n```shell\nc\n```",
			want:  []string{"a"},
			note:  true,
		},
		{
			name:  "another language is prose, not a command",
			reply: "```python\nprint(1)\n```\n```json\n{}\n```",
		},
		{
			// The only escape the format has, and the prompt names it.
			name:  "an indented fence is quoted, not run",
			reply: "run this yourself:\n\n    ```sh\n    rm -rf /\n    ```\n",
		},
		{
			name:  "the first of several blocks is the one action",
			reply: "```ply\nfirst\n```\nthen\n```ply\nsecond\n```",
			want:  []string{"first"},
			note:  true,
		},
		{
			// A reply cut off by an output cap looks exactly like this, and
			// `rm -rf /tmp/build` cut in half is a different command.
			name:  "an unterminated fence runs nothing and says so",
			reply: "```ply\nrm -rf /tmp/build\n",
			note:  true,
		},
		{
			name:  "a complete first block runs before an unfinished deferred one",
			reply: "```ply\nsafe\n```\n```ply\ntruncated",
			want:  []string{"safe"},
			note:  true,
		},
		{
			// Writing a README means a line of three backticks inside the
			// command, which is why markdown has longer fences.
			name:  "a longer fence carries three backticks inside it",
			reply: "````ply\ncat > R.md <<'EOF'\n```sh\nmake\n```\nEOF\n````",
			want:  []string{"cat > R.md <<'EOF'\n```sh\nmake\n```\nEOF"},
		},
		{
			name:  "an empty block is corrected rather than mistaken for a report",
			reply: "```ply\n\n```",
			note:  true,
		},
		{
			name:  "text after a command is deferred rather than trusted",
			reply: "```ply\ntouch made\n```\nMade it.",
			want:  []string{"touch made"},
			note:  true,
		},
		{
			name:  "leading commentary may introduce one final command",
			reply: "I will inspect it.\n\n```ply\nprintf '%s\\n' one; printf '%s\\n' two\n```",
			want:  []string{"printf '%s\\n' one; printf '%s\\n' two"},
		},
		{
			name:  "a fence must start the line",
			reply: "the text ```ply ls ``` inline",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, note := commands(c.reply)
			if len(got) != len(c.want) {
				t.Fatalf("got %d commands %q, want %d %q", len(got), got, len(c.want), c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("command %d = %q, want %q", i, got[i], c.want[i])
				}
			}
			if (note != "") != c.note {
				t.Errorf("note = %q, want note: %v", note, c.note)
			}
		})
	}
	for _, lang := range []string{"sh", "bash", "shell", "zsh"} {
		t.Run(lang+" is a command label", func(t *testing.T) {
			got, _, note := commands("```" + lang + "\nprintf ok\n```")
			if len(got) != 1 || got[0] != "printf ok" || note != "" {
				t.Fatalf("commands=%q note=%q", got, note)
			}
		})
	}
}

// TestProseIsTheReplyWithoutItsCommands: what a command was is about to be
// shown under a real prompt with what it printed, so showing it twice is
// noise. Somebody else's code block is not a command and stays put.
func TestProseIsTheReplyWithoutItsCommands(t *testing.T) {
	_, prose, _ := commands("Looking now.\n\n```python\nprint(1)\n```\n\nThen this:\n\n```ply\nls -la\n```")
	if strings.Contains(prose, "ls -la") {
		t.Errorf("the command is still in the prose:\n%s", prose)
	}
	for _, want := range []string{"Looking now.", "Then this:", "print(1)"} {
		if !strings.Contains(prose, want) {
			t.Errorf("prose lost %q:\n%s", want, prose)
		}
	}
}

// TestWeldedFencesAreNotSilentlyLost: two adjacent text blocks joined with
// nothing put a closing fence and the next opening one on one line. ask no
// longer does that, but if anything ever does again, the failure must be a
// note the model can act on rather than commands that quietly vanish.
func TestWeldedFencesAreNotSilentlyLost(t *testing.T) {
	cmds, _, note := commands("```ply\nls\n``````ply\npwd\n```")
	if len(cmds) == 0 && note == "" {
		t.Fatal("welded fences produced neither commands nor a note: the run would stall in silence")
	}
}

func newRunner(t *testing.T, dir, path string) Runner {
	t.Helper()
	return Runner{Dir: dir, Path: path, Shell: defaultShell, Timeout: 10 * time.Second, Cap: 4096}
}

func TestResolveShellFindsOneExecutableWithoutFollowingItsName(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "chosen-shell")
	write(t, target, "#!/bin/sh\nexit 0\n", 0o755)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	got, err := resolveShell(link)
	if err != nil {
		t.Fatal(err)
	}
	if got != link {
		t.Fatalf("resolved shell = %q, want invocation name %q", got, link)
	}

	t.Setenv("PATH", dir)
	got, err = resolveShell("chosen-shell")
	if err != nil || got != link {
		t.Fatalf("PATH lookup = %q, %v; want %q", got, err, link)
	}
}

func TestResolveShellRejectsMissingAndEmptyChoices(t *testing.T) {
	for _, name := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		if _, err := resolveShell(name); err == nil || !strings.Contains(err.Error(), "-shell") {
			t.Errorf("resolveShell(%q) error = %v", name, err)
		}
	}
}

func TestRunUsesTheSelectedShell(t *testing.T) {
	dir := t.TempDir()
	shell := filepath.Join(dir, "chosen-shell")
	write(t, shell, "#!/bin/sh\nprintf 'chosen shell\\n'\nexec /bin/sh \"$@\"\n", 0o755)
	r := newRunner(t, dir, os.Getenv("PATH"))
	r.Shell = shell
	res := r.Run(context.Background(), "printf 'command ran\\n'")
	for _, want := range []string{"chosen shell", "command ran"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("output %q does not contain %q", res.Output, want)
		}
	}
}

func TestRunReportsExitStatus(t *testing.T) {
	r := newRunner(t, t.TempDir(), os.Getenv("PATH"))
	res := r.Run(context.Background(), "echo hello; exit 3")
	if res.Code != 3 {
		t.Errorf("code = %d, want 3", res.Code)
	}
	if !strings.Contains(res.Output, "hello") {
		t.Errorf("output = %q, want it to hold hello", res.Output)
	}
	if !strings.Contains(res.Typescript(), "exit 3") {
		t.Errorf("typescript hides the exit status:\n%s", res.Typescript())
	}
}

func TestInterpreterStartDiagnosticHasConsistentReceiptBytes(t *testing.T) {
	r := newRunner(t, t.TempDir(), os.Getenv("PATH"))
	r.Shell = filepath.Join(t.TempDir(), "missing-shell")
	res := r.RunInput(context.Background(), "verify", "candidate")
	if !res.StartError || res.Code != 1 || res.Output == "" {
		t.Fatalf("result=%#v, want interpreter start failure and diagnostic", res)
	}
	if res.Total != int64(len(res.Output)) || res.Elided != 0 {
		t.Fatalf("diagnostic accounting disagrees with retained bytes: %#v", res)
	}
	receipt := receiptFor("", "candidate", "candidate", r, res)
	if receipt.Outcome != "broken" || receipt.OutputBytes != int64(len(receipt.Output)) ||
		receipt.OutputSHA256 != digestText(string(receipt.Output)) || receipt.ElidedBytes != 0 {
		t.Fatalf("start failure receipt is inconsistent: %#v", receipt)
	}
}

func TestRunPreservesObservedOutputWhenInheritedPipesDoNotClose(t *testing.T) {
	t.Setenv("PLY_DEPTH", "0")
	dir := t.TempDir()
	script, pidfile := stubbornDescendant(t, dir, false)
	script = strings.TrimSuffix(script, "\nwait") + "\nprintf 'observed before exit\\n'; exit 0"
	r := newRunner(t, dir, os.Getenv("PATH"))
	res := r.Run(context.Background(), script)
	if !res.OutputIncomplete || res.StartError || res.Interrupted || res.Killed || res.Code != 0 {
		t.Fatalf("result=%#v, want incomplete output from a started, successful interpreter", res)
	}
	if res.Output != "observed before exit\n" || res.Total != int64(len(res.Output)) || res.Elided != 0 {
		t.Fatalf("observed bytes or accounting lost: %#v", res)
	}
	if !strings.Contains(res.Typescript(), "observed output is incomplete") ||
		strings.Contains(res.Typescript(), "could not start") {
		t.Fatalf("typescript misrepresents the executed command: %s", res.Typescript())
	}
	if got := receiptFor("", "candidate", "answer", r, res); got.Outcome != "broken" {
		t.Fatalf("incomplete verifier output became completion evidence: %#v", got)
	}
	requireProcessStopped(t, waitForPID(t, pidfile))
}

func TestRunInterleavesStdoutAndStderr(t *testing.T) {
	r := newRunner(t, t.TempDir(), os.Getenv("PATH"))
	res := r.Run(context.Background(), "echo out; echo err >&2")
	for _, want := range []string{"out", "err"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("output %q lost %q; a terminal shows both", res.Output, want)
		}
	}
}

// TestRunHasNoStdin: nothing is typing. A command that reads stdin must see
// EOF rather than block, or one `cat` hangs the loop until the timeout,
// every turn, forever.
func TestRunHasNoStdin(t *testing.T) {
	r := newRunner(t, t.TempDir(), os.Getenv("PATH"))
	done := make(chan Result, 1)
	go func() { done <- r.Run(context.Background(), "cat; echo done") }()
	select {
	case res := <-done:
		if !strings.Contains(res.Output, "done") {
			t.Errorf("output = %q, want it to have finished", res.Output)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a command reading stdin blocked; stdin must be the null device")
	}
}

// TestTimeoutKillsTheProcessGroup: a script that leaves a child holding the
// pipe is the difference between a timeout and a hang, which is why the
// command gets its own process group and the group is what gets killed.
func TestTimeoutKillsTheProcessGroup(t *testing.T) {
	r := newRunner(t, t.TempDir(), os.Getenv("PATH"))
	r.Timeout = 300 * time.Millisecond
	start := time.Now()
	res := r.Run(context.Background(), "sleep 30 & echo started; sleep 30")
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("took %s; the background child kept the pipe open", elapsed)
	}
	if !res.Killed || res.Code != exitTimeout {
		t.Errorf("killed = %v code = %d, want true and %d", res.Killed, res.Code, exitTimeout)
	}
	if !strings.Contains(res.Typescript(), "killed after") {
		t.Errorf("typescript does not say it was killed:\n%s", res.Typescript())
	}
}

func TestTimeoutKillsIgnoringDescendantAfterLeaderAndPipesExit(t *testing.T) {
	t.Setenv("PLY_DEPTH", "0")
	dir := t.TempDir()
	script, pidfile := stubbornDescendant(t, dir, true)
	r := newRunner(t, dir, os.Getenv("PATH"))
	r.Timeout = 300 * time.Millisecond
	start := time.Now()
	res := r.Run(context.Background(), script)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("teardown took %s", elapsed)
	}
	if !res.Killed || res.Code != exitTimeout {
		t.Fatalf("result=%#v, want timeout", res)
	}
	requireProcessStopped(t, waitForPID(t, pidfile))
}

func TestSuccessfulRunPreservesDeliberatelyDetachedWork(t *testing.T) {
	dir := t.TempDir()
	script, pidfile := stubbornDescendant(t, dir, true)
	script = strings.TrimSuffix(script, "\nwait") + "\nexit 0"
	r := newRunner(t, dir, os.Getenv("PATH"))
	res := r.Run(context.Background(), script)
	if res.Code != 0 || res.Interrupted || res.Killed {
		t.Fatalf("successful launch=%#v", res)
	}
	if pid := waitForPID(t, pidfile); syscall.Kill(pid, 0) != nil {
		t.Fatal("normal successful execution canceled redirected background work")
	}
}

func TestCanceledRunLetsNestedPlyKillIgnoringDescendant(t *testing.T) {
	t.Setenv("PLY_DEPTH", "0")
	work := t.TempDir()
	script, pidfile := stubbornDescendant(t, work, true)
	ask, _ := fakeAsk(t, "```ply\n"+script+"\n```")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := newRunner(t, work, os.Getenv("PATH"))
	r.Env = []string{"PLY_TEST_PROGRAM=1", "ASK=" + ask, "PLY_DEPTH=1"}
	session := filepath.Join(t.TempDir(), "nested.jsonl")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan Result, 1)
	go func() {
		done <- r.Run(ctx, shellQuote(bin)+" -sh -C "+shellQuote(work)+
			" -f "+shellQuote(session)+" child")
	}()
	pid := waitForPID(t, pidfile)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("nested Ply did not stop after cancellation")
	}
	requireProcessStopped(t, pid)
}

func TestCanceledRunPreservesSuccessfulExitButMarksInterruption(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "ready.pid")
	r := newRunner(t, dir, os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan Result, 1)
	go func() {
		done <- r.Run(ctx, "trap 'exit 0' INT; echo $$ > "+shellQuote(pidfile)+"; sleep 30 & wait")
	}()
	waitForPID(t, pidfile)
	cancel()
	select {
	case res := <-done:
		if !res.Interrupted || res.Code != 0 || res.StartError || res.Killed {
			t.Fatalf("result=%#v, want interrupted exit 0", res)
		}
		if !strings.Contains(res.Typescript(), "interrupted; command effects may exist") {
			t.Fatalf("typescript hides interruption: %s", res.Typescript())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled action did not stop")
	}
}

// stubbornDescendant leaves an interrupt-ignoring child in the shell's group.
// With redirected output, both the group leader and its output copying finish
// immediately on SIGINT; neither is proof that the child is gone.
func stubbornDescendant(t *testing.T, dir string, redirect bool) (script, pidfile string) {
	t.Helper()
	pidfile = filepath.Join(dir, "stubborn.pid")
	groupfile := filepath.Join(dir, "group.pid")
	child := "trap '' INT TERM HUP; echo $$ > " + shellQuote(pidfile) + "; exec sleep 30"
	script = "trap 'exit 130' INT\necho $$ > " + shellQuote(groupfile) + "\n/bin/sh -c " + shellQuote(child)
	if redirect {
		script += " >/dev/null 2>&1"
	}
	script += " &\nwait"
	t.Cleanup(func() {
		if body, err := os.ReadFile(groupfile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(body))); err == nil && pid > 0 {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
		if body, err := os.ReadFile(pidfile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(body))); err == nil && pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	return script, pidfile
}

func waitForPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(path)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(body))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process never wrote %s", path)
	return 0
}

func requireProcessStopped(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		// A container's PID 1 may leave a killed orphan as a zombie. It
		// cannot perform more actions and is waiting only to be reaped.
		state, _ := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(pid)).Output()
		if strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned descendant %d is still running after teardown", pid)
}

// TestToolboxScopesPATH: the toolbox is the capability set. A program that
// is not in it cannot be named, which is what makes -t mean anything.
func TestToolboxScopesPATH(t *testing.T) {
	box := t.TempDir()
	write(t, filepath.Join(box, "mine"), "#!/bin/sh\necho mine ran\n", 0o755)
	b, err := openBox(box, false)
	if err != nil {
		t.Fatal(err)
	}
	r := newRunner(t, t.TempDir(), b.Path())

	if res := r.Run(context.Background(), "mine"); !strings.Contains(res.Output, "mine ran") {
		t.Errorf("a program in the toolbox did not run: %q", res.Output)
	}
	res := r.Run(context.Background(), "git --version")
	if res.Code == 0 {
		t.Errorf("git ran with PATH=%s; the toolbox must be the whole PATH", b.Path())
	}
}

// TestOutputIsCappedAndSaysSo: truncation the model cannot see is the one
// failure nothing downstream can detect, so the marker goes in the text it
// reads, and both ends are kept — a build fails at the end, a listing
// matters at the start.
func TestOutputIsCappedAndSaysSo(t *testing.T) {
	r := newRunner(t, t.TempDir(), os.Getenv("PATH"))
	r.Cap = 2048
	res := r.Run(context.Background(), "echo FIRST; i=0; while [ $i -lt 4000 ]; do echo pad-line-$i; i=$((i+1)); done; echo LAST")
	if res.Elided == 0 {
		t.Fatal("nothing was elided from a stream far past the cap")
	}
	if !strings.Contains(res.Output, "FIRST") {
		t.Error("the head was dropped; a listing matters at the start")
	}
	if !strings.Contains(res.Output, "LAST") {
		t.Error("the tail was dropped; a build fails at the end")
	}
	if !strings.Contains(res.Output, "elided") {
		t.Errorf("the elision is not announced in what the model reads:\n%s", res.Output)
	}
	if len(res.Output) > 3*r.Cap {
		t.Errorf("kept %d bytes for a cap of %d", len(res.Output), r.Cap)
	}
}

func TestCapBufCountsEverythingItDrops(t *testing.T) {
	b := &capBuf{cap: 4}
	for range 10 {
		b.Write([]byte("0123456789"))
	}
	out, elided, total := b.String()
	if total != 100 {
		t.Errorf("total = %d, want 100", total)
	}
	if elided != 92 {
		t.Errorf("elided = %d, want 92", elided)
	}
	if !strings.HasPrefix(out, "0123") || !strings.HasSuffix(out, "6789") {
		t.Errorf("kept %q, want the first four bytes and the last four", out)
	}
}

// TestTypescriptReadsLikeATerminal: the model has seen a million terminal
// sessions and no examples of whatever we might invent instead.
func TestTypescriptReadsLikeATerminal(t *testing.T) {
	got := Result{Cmd: "cat <<EOF\nhi\nEOF", Output: "hi\n"}.Typescript()
	want := "$ cat <<EOF\n> hi\n> EOF\nhi\n"
	if got != want {
		t.Errorf("typescript =\n%q\nwant\n%q", got, want)
	}
	if quiet := (Result{Cmd: "true"}).Typescript(); !strings.Contains(quiet, "no output") {
		t.Errorf("a silent success says nothing at all:\n%s", quiet)
	}
}

func write(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}
