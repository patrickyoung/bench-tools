package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests below drive run() exactly as a shell does -- argv in, streams
// and an exit code out -- against a fake ask. It is a program, like the
// real one, so what they pin is the program and not a rehearsal of it.

// fakeAsk stands in for the model. It answers `replay -check` with success
// and everything else with the canned reply, and records what it was sent.
func fakeAsk(t *testing.T, reply string) (bin, dir string) {
	t.Helper()
	dir = t.TempDir()
	bin = filepath.Join(dir, "ask")
	write(t, bin, `#!/bin/sh
d=`+dir+`
echo "$@" >> "$d/argv.log"
next=
for arg do
  if [ "$next" = file ]; then printf '%s\n' '{"type":"fake-wording-session"}' > "$arg"; next=; continue; fi
  if [ "$arg" = -f ]; then next=file; fi
done
case "$1" in
  replay) exit ${FAKE_REPLAY_EXIT:-0} ;;
  note)   exit 0 ;;
esac
printf '%s' "$ASK_SYSTEM" >> "$d/system.log"
cat >> "$d/stdin.log"
n=$(cat "$d/n" 2>/dev/null || echo 0); n=$((n+1)); echo "$n" > "$d/n"
if [ "$n" = 1 ]; then cat "$d/reply"; else echo "A description of the skill, and when to use it."; fi
`, 0o755)
	write(t, filepath.Join(dir, "reply"), reply, 0o644)
	return bin, dir
}

func write(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// runHone calls the program in process, with the streams a filter's
// contract is written about pointed somewhere a test can read them.
func runHone(t *testing.T, args ...string) (code int, stdout, stderr string) {
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

// writeSession writes a session file the way ask does: one JSON event per
// line. It takes the pieces the gate is about.
func writeSession(t *testing.T, path, check string, script []string, verdict string) {
	t.Helper()
	var b strings.Builder
	add := func(typ string, data any) {
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		line, err := json.Marshal(map[string]any{"seq": 1, "type": typ, "data": json.RawMessage(raw)})
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	system := "You are working through a Unix shell to reach a goal.\n"
	if check != "" {
		system += "\nWhen you stop, ply runs\n\n    " + check + "\n\nand the run ends only when that exits zero.\n"
	}
	add("session", map[string]any{"id": strings.TrimSuffix(filepath.Base(path), ".jsonl"), "model": "m", "system": system})
	add("user", map[string]any{"text": "make the tests pass"})
	for _, s := range script {
		add("assistant", map[string]any{"blocks": []map[string]any{{"type": "text", "text": "working"}}})
		add("user", map[string]any{"text": s})
	}
	if verdict != "" {
		add("note", map[string]any{"source": "ply", "text": verdict + ":\n\n$ " + check + "\nok\n"})
	}
	write(t, path, b.String(), 0o644)
}

// sandbox points hone at a fake ask and a disposable skill directory.
func sandbox(t *testing.T, reply string) (sessions, skills, askdir string) {
	t.Helper()
	bin, askdir := fakeAsk(t, reply)
	sessions, skills = t.TempDir(), t.TempDir()
	t.Setenv("ASK", bin)
	t.Setenv("BRIEF", filepath.Join(t.TempDir(), "no-brief-here"))
	t.Setenv("ASK_DIR", sessions)
	t.Setenv("BRIEF_PATH", skills)
	t.Setenv("HONE_DIR", t.TempDir())
	return sessions, skills, askdir
}

// TestTheWholeProgram: a run that failed and then passed yields a lesson on
// stdout, and the exit status says something was learned.
func TestTheWholeProgram(t *testing.T) {
	sessions, _, _ := sandbox(t, "- Test files here declare package main.")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{
		"$ go test ./...\nfound packages x and main\nexit 1\n",
		"$ sed -i '' s/x/main/ add.go\n$ go test ./...\nok\n",
	}, passedMark)

	code, stdout, stderr := runHone(t, s)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "Test files here declare package main.") {
		t.Errorf("stdout does not carry the lesson: %q", stdout)
	}
	if !strings.Contains(stdout, "<!-- hone run ") {
		t.Errorf("stdout does not carry the provenance: %q", stdout)
	}
	if strings.Contains(stderr, "package main") {
		t.Errorf("the lesson leaked onto stderr: %q", stderr)
	}
}

// The exit contract is brief's, not ask's: no is an ordinary answer here,
// because most runs teach nothing and a loop over an archive branches on it
// rather than stopping.
func TestNothingToLearnIsExitOneWithEmptyStdout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		script  []string
		verdict string
		says    string
	}{
		{"no verdict", []string{"$ go test ./...\nexit 1\n", "$ go test ./...\nok\n"}, "", "no check ran"},
		{"check failed", []string{"$ go test ./...\nexit 1\n", "$ go test ./...\nok\n"}, failedMark, "never passed"},
		{"nothing failed", []string{"$ go test ./...\nok\n"}, passedMark, "nothing ever failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sessions, _, askdir := sandbox(t, "- a lesson that must never be asked for")
			s := filepath.Join(sessions, "run.jsonl")
			writeSession(t, s, "go test ./...", tc.script, tc.verdict)

			code, stdout, stderr := runHone(t, s)
			if code != 1 {
				t.Fatalf("exit = %d, want 1\n%s", code, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty: a no must not be piped on as a lesson", stdout)
			}
			if !strings.Contains(stderr, tc.says) {
				t.Errorf("stderr does not say why: %q", stderr)
			}
			// And the gate is arithmetic, so it costs nothing.
			if _, err := os.Stat(filepath.Join(askdir, "n")); err == nil {
				t.Error("a model was asked about a run that could not teach anything")
			}
		})
	}
}

// "none" is a real answer from the model too, and it is expected far more
// often than not.
func TestNoneFromTheModelIsExitOne(t *testing.T) {
	sessions, _, _ := sandbox(t, "none")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{"$ x\nexit 1\n", "$ y\nok\n"}, passedMark)

	code, stdout, stderr := runHone(t, s)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "nothing worth keeping") {
		t.Errorf("stderr = %q", stderr)
	}
}

// A log that does not replay is not a record of what happened, and learning
// from one is the failure this program is arranged to avoid.
func TestASessionThatDoesNotReplayIsRefused(t *testing.T) {
	sessions, _, _ := sandbox(t, "- a lesson")
	t.Setenv("FAKE_REPLAY_EXIT", "1")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{"$ x\nexit 1\n", "$ y\nok\n"}, passedMark)

	code, stdout, stderr := runHone(t, s)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "not a record of what happened") {
		t.Errorf("stderr does not say why: %q", stderr)
	}
}

func TestIntoWritesASkillAndSaysWhere(t *testing.T) {
	sessions, skills, _ := sandbox(t, "- Test files here declare package main.")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{"$ x\nexit 1\n", "$ y\nok\n"}, passedMark)

	code, stdout, stderr := runHone(t, "-into", "house", s)
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	path := filepath.Join(skills, "house", "SKILL.md")
	if !strings.Contains(stdout, path) {
		t.Errorf("stdout does not name the file it wrote: %q", stdout)
	}
	doc := read(t, path)
	if !strings.Contains(doc, "Test files here declare package main.") {
		t.Errorf("the lesson is not in the skill:\n%s", doc)
	}
	if !strings.Contains(doc, "name: house") {
		t.Errorf("the skill has no frontmatter:\n%s", doc)
	}
	// The description was written by a model, so it is findable.
	if strings.Contains(doc, "will not be found") {
		t.Errorf("no description was written:\n%s", doc)
	}

	// A run teaches once, and the second pass costs nothing.
	code, _, stderr = runHone(t, "-into", "house", s)
	if code != 1 {
		t.Errorf("honing the same run twice: exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "already learned from") {
		t.Errorf("stderr = %q", stderr)
	}
}

// -N is for reading before believing: it says what would be learned and
// writes nothing.
func TestDryRunWritesNothing(t *testing.T) {
	sessions, skills, _ := sandbox(t, "- a lesson")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{"$ x\nexit 1\n", "$ y\nok\n"}, passedMark)

	code, stdout, _ := runHone(t, "-N", "-into", "house", s)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "a lesson") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(skills, "house")); err == nil {
		t.Error("-N wrote a skill")
	}
}

func TestPreparedProposalAdmitsExactBytesWithoutAnotherModelCall(t *testing.T) {
	sessions, skills, askdir := sandbox(t, "- Test files here declare package main.")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{"$ x\nexit 1\n", "$ y\nok\n"}, passedMark)
	artifact := filepath.Join(t.TempDir(), "review.json")

	code, stdout, stderr := runHone(t, "-into", "house", "-prepare", artifact, s)
	if code != 0 {
		t.Fatalf("prepare exit=%d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, proposalVersion) || !strings.Contains(stdout, "skill-bytes:") || !strings.Contains(stdout, "Test files here") {
		t.Fatalf("proposal review output is incomplete:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(skills, "house", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("prepare changed the skill: %v", err)
	}
	p, err := readProposal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if p.BeforeSHA256 != absentHash || p.SourceID != "run" || p.AfterSHA256 != sha256Bytes([]byte(p.Document)) {
		t.Fatalf("proposal is not exactly bound: %#v", p)
	}
	modelCalls := strings.TrimSpace(read(t, filepath.Join(askdir, "n")))
	if modelCalls != "2" { // one lesson call and one new-skill description
		t.Fatalf("prepare model calls=%q, want 2", modelCalls)
	}

	code, shown, stderr := runHone(t, "show", artifact)
	if code != 0 || !strings.Contains(shown, p.Document) {
		t.Fatalf("show exit=%d stderr=%q output=%q", code, stderr, shown)
	}
	code, stdout, stderr = runHone(t, "admit", artifact)
	if code != 0 {
		t.Fatalf("admit exit=%d\n%s", code, stderr)
	}
	if got := read(t, filepath.Join(skills, "house", "SKILL.md")); got != p.Document {
		t.Fatalf("admitted bytes differ from review\ngot:\n%s\nwant:\n%s", got, p.Document)
	}
	if !strings.Contains(stdout, "reviewed lesson") || !strings.Contains(stderr, "no model called") {
		t.Fatalf("admit streams stdout=%q stderr=%q", stdout, stderr)
	}
	if got := strings.TrimSpace(read(t, filepath.Join(askdir, "n"))); got != modelCalls {
		t.Fatalf("admit called a model: before=%q after=%q", modelCalls, got)
	}
	argv := read(t, filepath.Join(askdir, "argv.log"))
	if !strings.Contains(argv, "replay -check "+p.Source) || !strings.Contains(argv, "replay -check "+p.Wording) {
		t.Fatalf("admit did not replay both provenance sessions:\n%s", argv)
	}
	if code, _, stderr := runHone(t, "admit", artifact); code != 1 || !strings.Contains(stderr, "already learned from") {
		t.Fatalf("second admit exit=%d stderr=%q", code, stderr)
	}
}

func TestProposalAdmissionRefusesStaleOrAlteredBytes(t *testing.T) {
	sessions, skills, askdir := sandbox(t, "- Keep exact reviewed bytes.")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "check", []string{"$ check\nexit 1\n", "$ check\nok\n"}, passedMark)
	dir := filepath.Join(skills, "house")
	original := scaffold("house", "Existing procedure.")
	write(t, filepath.Join(dir, "SKILL.md"), original, 0o644)
	artifact := filepath.Join(t.TempDir(), "review.json")
	if code, _, stderr := runHone(t, "-into", "house", "-prepare", artifact, s); code != 0 {
		t.Fatalf("prepare exit=%d\n%s", code, stderr)
	}
	modelCalls := strings.TrimSpace(read(t, filepath.Join(askdir, "n")))
	write(t, filepath.Join(dir, "SKILL.md"), scaffold("house", "Changed after review."), 0o644)
	code, _, stderr := runHone(t, "admit", artifact)
	if code != 2 || !strings.Contains(stderr, "changed after proposal review") {
		t.Fatalf("stale admit exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(read(t, filepath.Join(dir, "SKILL.md")), "Changed after review") {
		t.Error("stale admission overwrote the changed skill")
	}
	if got := strings.TrimSpace(read(t, filepath.Join(askdir, "n"))); got != modelCalls {
		t.Fatalf("stale admission called a model: before=%q after=%q", modelCalls, got)
	}

	p, err := readProposal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	p.Document += "tampered\n"
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	altered := filepath.Join(t.TempDir(), "altered.json")
	write(t, altered, string(body), 0o600)
	if code, _, stderr := runHone(t, "show", altered); code != 2 || !strings.Contains(stderr, "does not match after sha256") {
		t.Fatalf("altered proposal show exit=%d stderr=%q", code, stderr)
	}

	write(t, filepath.Join(dir, "SKILL.md"), original, 0o644)
	p.Document = strings.TrimRight(p.Document, "\n") + "\n\n## Unreviewed rewrite\nextra authority\n"
	p.AfterSHA256 = sha256Bytes([]byte(p.Document))
	body, err = json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	rehashed := filepath.Join(t.TempDir(), "rehashed.json")
	write(t, rehashed, string(body), 0o600)
	if code, _, stderr := runHone(t, "admit", rehashed); code != 2 || !strings.Contains(stderr, "not the exact Hone append") {
		t.Fatalf("rehashed rewrite admit exit=%d stderr=%q", code, stderr)
	}
	if got := read(t, filepath.Join(dir, "SKILL.md")); got != original {
		t.Error("rehashed rewrite changed the skill")
	}
}

func TestPrepareRefusesOccupiedArtifactBeforeCallingModel(t *testing.T) {
	sessions, _, askdir := sandbox(t, "- a lesson")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "check", []string{"$ check\nexit 1\n", "$ check\nok\n"}, passedMark)
	artifact := filepath.Join(t.TempDir(), "exists.json")
	write(t, artifact, "keep me\n", 0o600)
	code, _, stderr := runHone(t, "-into", "house", "-prepare", artifact, s)
	if code != 2 || !strings.Contains(stderr, "already exists") || read(t, artifact) != "keep me\n" {
		t.Fatalf("occupied prepare exit=%d stderr=%q artifact=%q", code, stderr, read(t, artifact))
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); !os.IsNotExist(err) {
		t.Fatalf("occupied proposal called a model: %v", err)
	}
}

// -why prints the evidence and asks nothing. A lesson is a claim, and being
// able to read the evidence before paying for the claim is the difference
// between a tool you trust and one you audit afterwards.
func TestWhyShowsTheEvidenceAndCallsNoModel(t *testing.T) {
	sessions, _, askdir := sandbox(t, "- a lesson that must never be asked for")
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{
		"$ go test ./...\nfound packages x and main\nexit 1\n",
		"$ sed -i '' s/x/main/ add.go\n$ go test ./...\nok\n",
	}, passedMark)

	code, stdout, _ := runHone(t, "-why", s)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"GOAL", "CHECK", "STUMBLE 1", "found packages x and main"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("evidence is missing %q:\n%s", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(askdir, "n")); err == nil {
		t.Error("-why called a model")
	}
	if argv := read(t, filepath.Join(askdir, "argv.log")); !strings.Contains(argv, "replay -check "+s) {
		t.Fatalf("-why did not replay-verify its evidence:\n%s", argv)
	}
	t.Setenv("FAKE_REPLAY_EXIT", "1")
	if code, stdout, stderr := runHone(t, "-why", s); code != 1 || stdout != "" || !strings.Contains(stderr, "does not replay") {
		t.Fatalf("damaged -why exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// A directory is every session in it, which is what a batch over an archive
// wants -- and one that teaches nothing must not stop the batch.
func TestADirectoryIsABatch(t *testing.T) {
	sessions, skills, _ := sandbox(t, "- a lesson")
	writeSession(t, filepath.Join(sessions, "a.jsonl"), "go test ./...", []string{"$ x\nok\n"}, passedMark)
	writeSession(t, filepath.Join(sessions, "b.jsonl"), "go test ./...", []string{"$ x\nexit 1\n", "$ y\nok\n"}, passedMark)

	code, _, stderr := runHone(t, "-into", "house", sessions)
	if code != 0 {
		t.Fatalf("exit = %d, want 0: one session taught something\n%s", code, stderr)
	}
	if !strings.Contains(read(t, filepath.Join(skills, "house", "SKILL.md")), "a lesson") {
		t.Error("the session that taught something was skipped")
	}
}

func TestForgetCommand(t *testing.T) {
	// A distinctive lesson: the scaffold's own prose says "the comment
	// after a lesson names the session", so asserting on the words "a
	// lesson" would match the boilerplate and never the lesson.
	const lesson = "Tabs, not spaces, in this tree."
	sessions, skills, _ := sandbox(t, "- "+lesson)
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{"$ x\nexit 1\n", "$ y\nok\n"}, passedMark)
	if code, _, se := runHone(t, "-into", "house", s); code != 0 {
		t.Fatalf("setup: exit %d\n%s", code, se)
	}
	path := filepath.Join(skills, "house", "SKILL.md")

	code, _, stderr := runHone(t, "forget", "run", filepath.Join(skills, "house"))
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if strings.Contains(read(t, path), lesson) {
		t.Errorf("the lesson survived:\n%s", read(t, path))
	}
	// Forgetting what was never learned is a no, not a failure.
	if code, _, _ := runHone(t, "forget", "run", filepath.Join(skills, "house")); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

// Errors are 2, and they are never mistaken for "nothing to learn": a
// missing ask must not look like a quiet run.
func TestBrokenIsExitTwo(t *testing.T) {
	sessions, _, _ := sandbox(t, "- a lesson")
	if code, _, _ := runHone(t, filepath.Join(sessions, "nope.jsonl")); code != 2 {
		t.Error("a missing session was not exit 2")
	}
	if code, _, _ := runHone(t, "-n", "0"); code != 2 {
		t.Error("a nonsense -n was not exit 2")
	}
	// A flag after a session is a filename, and saying so beats failing to
	// stat "-into" three frames down.
	s := filepath.Join(sessions, "run.jsonl")
	writeSession(t, s, "go test ./...", []string{"$ x\nok\n"}, passedMark)
	code, _, stderr := runHone(t, s, "-into", "house")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "put flags first") {
		t.Errorf("stderr does not explain: %q", stderr)
	}
}

func TestHelpGoesToStdoutAndMisuseToStderr(t *testing.T) {
	code, stdout, stderr := runHone(t, "help")
	if code != 0 || !strings.Contains(stdout, "hone") {
		t.Fatalf("help: exit %d, stdout %q", code, stdout)
	}
	if stderr != "" {
		t.Errorf("help wrote to stderr: %q", stderr)
	}
	for _, line := range strings.Split(usageText, "\n") {
		if len(line) > 80 {
			t.Errorf("help line is %d columns: %q", len(line), line)
		}
	}
	// A misuse leaves stdout empty for whatever was parsing it.
	_, stdout, _ = runHone(t, "forget")
	if stdout != "" {
		t.Errorf("a misuse wrote to stdout: %q", stdout)
	}
}

func TestPromptIsAValue(t *testing.T) {
	code, stdout, _ := runHone(t, "prompt")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "none") || !strings.Contains(stdout, "prevented") {
		t.Errorf("prompt does not print the prompt:\n%s", stdout)
	}
	if strings.Contains(stdout, "%d") {
		t.Errorf("the prompt was printed with its format verb unfilled:\n%s", stdout)
	}
}

func TestATypoIsNotASession(t *testing.T) {
	code, _, stderr := runHone(t, "forgett", "x", "y")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "did you mean") {
		t.Errorf("stderr = %q", stderr)
	}
}

// Every verb and flag is documented, and the whole verb as well as its
// parts: a flag-level check stays green while an entire verb goes
// undocumented.
func TestDocsCoverEveryCommandAndFlag(t *testing.T) {
	man, err := os.ReadFile("hone.1")
	if err != nil {
		t.Skip("hone.1 not written yet")
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range verbs {
		if !strings.Contains(usageText, "hone "+v) {
			t.Errorf("verb %q is missing from hone help", v)
		}
		if !strings.Contains(string(man), "hone "+v) {
			t.Errorf("verb %q is missing from hone.1", v)
		}
		if !strings.Contains(string(readme), "hone "+v) {
			t.Errorf("verb %q is missing from README.md", v)
		}
	}
	for _, f := range []string{"-into", "-n", "-m", "-N", "-why", "-prepare", "-d", "-no-verify", "-q"} {
		if !strings.Contains(usageText, f) {
			t.Errorf("flag %q is missing from hone help", f)
		}
		if !strings.Contains(string(man), f) {
			t.Errorf("flag %q is missing from hone.1", f)
		}
	}
	for _, e := range []string{"ASK", "BRIEF", "ASK_DIR", "BRIEF_PATH", "HONE_DIR"} {
		if !strings.Contains(usageText, e) {
			t.Errorf("environment variable %q is missing from hone help", e)
		}
		if !strings.Contains(string(man), e) {
			t.Errorf("environment variable %q is missing from hone.1", e)
		}
	}
}

// The manual is checked the way the others are: pure ASCII so it renders
// the same everywhere, carrying the version the binary reports, and with
// every macro one groff knows.
func TestManPageIsCleanAndCurrent(t *testing.T) {
	b, err := os.ReadFile("hone.1")
	if err != nil {
		t.Fatal(err)
	}
	man := string(b)
	for i := 0; i < len(man); i++ {
		if man[i] > 127 {
			line := 1 + strings.Count(man[:i], "\n")
			t.Fatalf("hone.1:%d: byte %#x is not ASCII; use a roff escape (\\(em, \\(aq)", line, man[i])
		}
	}
	if !strings.Contains(man, "hone "+version) {
		t.Errorf("hone.1 does not carry version %q", version)
	}
	if !strings.HasPrefix(man, ".TH HONE 1") {
		t.Error("hone.1 does not start with a .TH")
	}
	// A section that is declared and empty renders as a heading with
	// nothing under it, which reads as a missing page rather than a
	// deliberate omission.
	for _, sh := range []string{"NAME", "SYNOPSIS", "DESCRIPTION", "COMMANDS", "FLAGS", "ENVIRONMENT", "EXIT STATUS", "SEE ALSO"} {
		if !strings.Contains(man, ".SH "+sh+"\n") {
			t.Errorf("hone.1 has no %s section", sh)
		}
	}
}

// The README is where somebody decides whether to trust this, so the claim
// the whole design rests on has to be in it.
func TestReadmeStatesTheRule(t *testing.T) {
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	for _, want := range []string{"learns from recoveries", "never failed", "never passed", "exit 1"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md does not say %q", want)
		}
	}
}
