package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestTaskPresentationMatchesFrozenCompanionWithRawProcessBytes(t *testing.T) {
	check, err := filepath.Abs("../../workers/bench-hire/expert/task/bin/check")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(check); err != nil {
		t.Fatal(err)
	}
	for _, exit := range []int{0, 2} {
		t.Run(strconv.Itoa(exit), func(t *testing.T) {
			a := taskFixture(t)
			raw, err := os.ReadFile(a.cfg.Agent)
			if err != nil {
				t.Fatal(err)
			}
			old := " Path('response.json').write_text(json.dumps(reply));sys.exit(0)"
			if !strings.Contains(string(raw), old) {
				t.Fatal("fixture response seam changed")
			}
			// A deterministic reply is checked against the real frozen companion.
			// Neither the fixture nor the checker may repair the controller request.
			replacement := ` Path('response.json').write_text(json.dumps(reply))
 if r['stage']=='present':
  import subprocess
  status=subprocess.run(['/usr/bin/python3', ` + strconv.Quote(check) + `]).returncode
  assert p.read_bytes()==raw, 'checker changed request'
  sys.exit(status)
 sys.exit(0)`
			script := strings.Replace(string(raw), old, replacement, 1)
			script += "\nos.write(1,b'output\\x00\\x1b[31m\\r\\x7f\\xff\\n\\tend\\n')\n"
			script += "os.write(2,b'error\\x00\\x1b[0m\\b\\x7f\\xfe\\n\\tend\\n')\n"
			script += "sys.exit(" + strconv.Itoa(exit) + ")\n"
			writeFixture(t, a.cfg.Agent, script, 0700)
			turn := taskSubmit(t, a, "", "Create a checked document")
			if turn.Job.ExitCode == nil || *turn.Job.ExitCode != exit || turn.Result.Code != exit {
				t.Fatalf("process outcome changed: %+v %+v", turn.Job, turn.Result)
			}
			want := "Here is your work."
			if exit != 0 {
				want = "I need more information to finish."
			}
			if turn.Result.Message != want {
				t.Fatalf("presentation failed frozen check: %q\n%s", turn.Result.Message, a.jobs.Log(turn.Job.ID, "stderr"))
			}
			for stream, original := range map[string]string{
				"stdout": "output\x00\x1b[31m\r\x7f\xff\n\tend\n",
				"stderr": "error\x00\x1b[0m\b\x7f\xfe\n\tend\n",
			} {
				if !strings.Contains(a.jobs.Log(turn.Job.ID, stream), original) {
					t.Fatalf("raw %s log changed", stream)
				}
			}
			request, err := os.ReadFile(filepath.Join(filepath.Dir(turn.Job.Dir), "present", "request.json"))
			if err != nil {
				t.Fatal(err)
			}
			var present map[string]json.RawMessage
			if err := json.Unmarshal(request, &present); err != nil {
				t.Fatal(err)
			}
			var execution map[string]json.RawMessage
			if err := json.Unmarshal(present["execution"], &execution); err != nil {
				t.Fatal(err)
			}
			if len(execution) != 4 || bytes.Equal(present["worker_update"], []byte("null")) || len(present["worker_update"]) == 0 {
				t.Fatalf("presentation shape changed: %s", request)
			}
			for _, stream := range []string{"stdout", "stderr"} {
				var text string
				if err := json.Unmarshal(execution[stream], &text); err != nil {
					t.Fatal(err)
				}
				if !utf8.ValidString(text) || !strings.Contains(text, "\n\tend\n") {
					t.Fatalf("lost valid multiline text: %q", text)
				}
				for _, r := range text {
					if unicode.IsControl(r) && r != '\n' && r != '\t' {
						t.Fatalf("control character retained in %s: %q", stream, text)
					}
				}
			}
		})
	}
}
