package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskConversationSteersWhileWorkingAndKeepsLateNotes(t *testing.T) {
	a := taskFixture(t)
	raw, _ := os.ReadFile(a.cfg.Agent)
	fixture := strings.Replace(string(raw), "assert Path(os.environ['DOCUMENT_INPUT']).parent != Path.cwd()", `import time
steer=Path(sys.argv[sys.argv.index('-steer')+1])
assert not steer.is_relative_to(Path.cwd())
Path('ready').write_text('yes')
for _ in range(150):
 if steer.read_text().strip(): break
 time.sleep(.02)
else: sys.exit(1)
notes=[json.loads(s)['message'] for s in steer.read_text().splitlines()]
Path('guidance.txt').write_text('\n'.join(notes))
assert Path(os.environ['DOCUMENT_INPUT']).parent != Path.cwd()`, 1)
	writeFixture(t, a.cfg.Agent, fixture, 0700)
	f := formFor(a)
	f.Set("message", "Make a flyer")
	w := serveTest(a, "POST", "/work", f)
	if w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	thread := strings.TrimPrefix(w.Header().Get("Location"), "/work/")
	turn := a.taskTurns(thread)[0]
	ready := filepath.Join(turn.Job.Dir, "execution", "ready")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatal("worker did not start", a.jobs.Log(turn.Job.ID, "stderr"))
	}
	page := serveTest(a, "GET", "/work/"+thread, nil).Body.String()
	if !strings.Contains(page, "Send update ↑") || !strings.Contains(page, "/work/jobs/"+turn.Job.ID+"/message") {
		t.Fatal("live composer unavailable")
	}
	f = formFor(a)
	f.Set("message", "Use blue, and keep it short.")
	path := "/work/jobs/" + turn.Job.ID + "/message"
	if w := serveTest(a, "POST", path, f); w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	if w := serveTest(a, "POST", path, f); w.Code != 303 {
		t.Fatal("duplicate note failed")
	}
	j := awaitJob(t, a.jobs, turn.Job.ID)
	if j.State != "completed" {
		t.Fatal(a.jobs.Log(j.ID, "stderr"))
	}
	notes, err := readTaskMessages(filepath.Dir(j.Dir))
	if err != nil || len(notes) != 1 {
		t.Fatal("duplicate delivery", notes, err)
	}
	got, _ := os.ReadFile(filepath.Join(j.Dir, "execution", "guidance.txt"))
	if string(got) != "Use blue, and keep it short." {
		t.Fatalf("worker missed guidance: %q", got)
	}
	// Sending from an older open page after completion saves a note; it never
	// automatically launches work. A later explicit message retains the note.
	f = formFor(a)
	f.Set("message", "Also make a printable version.")
	if w := serveTest(a, "POST", path, f); w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	if len(a.jobs.List()) != 1 {
		t.Fatal("message silently launched another run")
	}
	writeFixture(t, a.cfg.Agent, string(raw), 0700)
	next := taskSubmit(t, a, thread, "Apply those changes")
	history, _ := json.Marshal(next.Record.History)
	if !strings.Contains(string(history), "Use blue") || !strings.Contains(string(history), "printable version") {
		t.Fatal("follow-up lost conversation notes")
	}
	f = formFor(a)
	f.Set("message", "Stale update")
	if w := serveTest(a, "POST", path, f); w.Code != 409 {
		t.Fatal("stale attempt accepted note")
	}
}

func TestTaskMessageFileLimitsAndLiteralArguments(t *testing.T) {
	root := t.TempDir()
	if err := initTaskMessages(root); err != nil {
		t.Fatal(err)
	}
	item := taskMessage{ID: randomID(), Message: "a\nb\n$(touch NO)", At: "now"}
	if err := appendTaskMessage(root, item); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "messages.jsonl"))
	if strings.Count(string(raw), "\n") != 1 {
		t.Fatal("note was not a complete single line")
	}
	args := taskSteerArgs(taskRecord{Steering: true}, root, []string{"agent", "run", "worker", "--", "goal"})
	if len(args) != 7 || args[2] != "-steer" || args[3] != filepath.Join(root, "messages.jsonl") || args[4] != "worker" {
		t.Fatal(args)
	}
	item.ID = randomID()
	item.Message = strings.Repeat("x", 9000)
	if err := appendTaskMessage(root, item); err == nil {
		t.Fatal("oversized guidance accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "messages.jsonl"), []byte(`{"id":`), 0600); err != nil {
		t.Fatal(err)
	}
	item.Message = "Do not repeat a partial write"
	if err := appendTaskMessage(root, item); err == nil {
		t.Fatal("uncertain save replayed")
	}
}
