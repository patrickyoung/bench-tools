package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
			name:  "sh, bash and shell all mean shell",
			reply: "```sh\na\n```\ntext\n```bash\nb\n```\n```shell\nc\n```",
			want:  []string{"a", "b", "c"},
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
			name:  "several blocks run in the order they were written",
			reply: "```ply\nfirst\n```\nthen\n```ply\nsecond\n```",
			want:  []string{"first", "second"},
		},
		{
			// A reply cut off by an output cap looks exactly like this, and
			// `rm -rf /tmp/build` cut in half is a different command.
			name:  "an unterminated fence runs nothing and says so",
			reply: "```ply\nrm -rf /tmp/build\n",
			note:  true,
		},
		{
			name:  "blocks before an unterminated one still run",
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
			name:  "an empty block is not a command",
			reply: "```ply\n\n```",
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
}

// TestProseIsTheReplyWithoutItsCommands: what a command was is about to be
// shown under a real prompt with what it printed, so showing it twice is
// noise. Somebody else's code block is not a command and stays put.
func TestProseIsTheReplyWithoutItsCommands(t *testing.T) {
	_, prose, _ := commands("Looking now.\n\n```ply\nls -la\n```\n\nThen this:\n\n```python\nprint(1)\n```")
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
	return Runner{Dir: dir, Path: path, Timeout: 10 * time.Second, Cap: 4096}
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
