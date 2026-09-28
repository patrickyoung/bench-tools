package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func handoffFixture(t *testing.T) (string, taskHandoff) {
	t.Helper()
	work := t.TempDir()
	writeFixture(t, filepath.Join(work, ".team/member/output.txt"), "retained candidate", 0600)
	writeFixture(t, filepath.Join(work, ".team/member/log.txt"), "first log", 0600)
	return work, taskHandoff{
		Version: 1, Invocation: "controller-invocation", GoalSHA256: digestText([]byte("goal")), DefinitionSHA256: digestText([]byte("definition")),
		Status: "continue", Stage: "Review", Message: "The candidate is ready for review.", Next: "Review the retained candidate.",
		Files: []taskHandoffFile{{Path: ".team/member/output.txt", Kind: "work", SHA256: digestText([]byte("retained candidate"))}, {Path: ".team/member/log.txt", Kind: "evidence", SHA256: digestText([]byte("first log"))}},
	}
}

func writeHandoff(t *testing.T, work string, report taskHandoff) {
	t.Helper()
	if err := saveTaskJSON(filepath.Join(work, "task-result.json"), report); err != nil {
		t.Fatal(err)
	}
}

func readHandoffFixture(work, snapshot string, code int) (taskHandoff, string, error) {
	return readTaskHandoff(work, snapshot, "controller-invocation", digestText([]byte("goal")), digestText([]byte("definition")), code)
}

func TestTaskHandoffRetainsNestedEvidenceAndImmutableSnapshot(t *testing.T) {
	work, report := handoffFixture(t)
	writeHandoff(t, work, report)
	snapshot := filepath.Join(t.TempDir(), "observation")
	got, digest, err := readHandoffFixture(work, snapshot, 2)
	if err != nil || got.Status != "continue" || !taskHandoffHash(digest) {
		t.Fatalf("handoff: %+v %q %v", got, digest, err)
	}
	writeFixture(t, filepath.Join(work, report.Files[0].Path), "later mutation", 0600)
	saved, err := os.ReadFile(filepath.Join(snapshot, "files", report.Files[0].Path))
	if err != nil || string(saved) != "retained candidate" {
		t.Fatal("snapshot did not retain exact observed bytes", err)
	}
	if _, err := os.Stat(filepath.Join(snapshot, "task-result.json")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(work, report.Files[0].Path), "retained candidate", 0600)
	if _, _, err := readHandoffFixture(work, snapshot, 2); err == nil {
		t.Fatal("replaced an existing snapshot")
	}
}

func TestTaskHandoffProgressIgnoresProseEvidenceAndOrder(t *testing.T) {
	work, report := handoffFixture(t)
	writeHandoff(t, work, report)
	_, first, err := readHandoffFixture(work, filepath.Join(t.TempDir(), "first"), 2)
	if err != nil {
		t.Fatal(err)
	}
	report.Message, report.Next, report.Stage = "Different words", "Another phrasing", "Another label"
	writeFixture(t, filepath.Join(work, report.Files[1].Path), "new log bytes", 0600)
	report.Files[1].SHA256 = digestText([]byte("new log bytes"))
	report.Files[0], report.Files[1] = report.Files[1], report.Files[0]
	writeHandoff(t, work, report)
	_, second, err := readHandoffFixture(work, filepath.Join(t.TempDir(), "second"), 2)
	if err != nil || first != second {
		t.Fatalf("non-work changes counted as progress: %v", err)
	}
	writeFixture(t, filepath.Join(work, report.Files[1].Path), "corrected candidate", 0600)
	report.Files[1].SHA256 = digestText([]byte("corrected candidate"))
	writeHandoff(t, work, report)
	_, third, err := readHandoffFixture(work, filepath.Join(t.TempDir(), "third"), 2)
	if err != nil || third == second {
		t.Fatalf("nested work change did not count: %v", err)
	}
}

func TestTaskHandoffRejectsMalformedOrStaleReports(t *testing.T) {
	cases := map[string]func(*taskHandoff){
		"invocation":       func(r *taskHandoff) { r.Invocation = "old invocation" },
		"goal":             func(r *taskHandoff) { r.GoalSHA256 = digestText([]byte("old goal")) },
		"definition":       func(r *taskHandoff) { r.DefinitionSHA256 = digestText([]byte("old definition")) },
		"version":          func(r *taskHandoff) { r.Version = 2 },
		"status":           func(r *taskHandoff) { r.Status = "running" },
		"empty next":       func(r *taskHandoff) { r.Next = "  " },
		"long message":     func(r *taskHandoff) { r.Message = strings.Repeat("a", 2001) },
		"stage newline":    func(r *taskHandoff) { r.Stage = "worker\nforged" },
		"files null":       func(r *taskHandoff) { r.Files = nil },
		"duplicate path":   func(r *taskHandoff) { r.Files = append(r.Files, r.Files[0]) },
		"bad hash":         func(r *taskHandoff) { r.Files[0].SHA256 = digestText([]byte("wrong")) },
		"bad kind":         func(r *taskHandoff) { r.Files[0].Kind = "checkpoint" },
		"parent traversal": func(r *taskHandoff) { r.Files[0].Path = "../output.txt" },
		"absolute path":    func(r *taskHandoff) { r.Files[0].Path = "/tmp/output.txt" },
		"unclean path":     func(r *taskHandoff) { r.Files[0].Path = ".team//member/output.txt" },
		"backslash":        func(r *taskHandoff) { r.Files[0].Path = ".team\\member\\output.txt" },
		"control path":     func(r *taskHandoff) { r.Files[0].Path = "output\n.txt" },
		"result itself":    func(r *taskHandoff) { r.Files[0].Path = "task-result.json" },
		"missing file":     func(r *taskHandoff) { r.Files[0].Path = "missing.txt" },
		"file count": func(r *taskHandoff) {
			for len(r.Files) <= 64 {
				r.Files = append(r.Files, r.Files[0])
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			work, report := handoffFixture(t)
			change(&report)
			writeHandoff(t, work, report)
			snapshot := filepath.Join(t.TempDir(), "observation")
			if _, _, err := readHandoffFixture(work, snapshot, 2); err == nil {
				t.Fatal("accepted invalid report")
			}
			if _, err := os.Stat(snapshot); !os.IsNotExist(err) {
				t.Fatal("invalid report produced a snapshot")
			}
		})
	}
	for _, mutation := range []string{"missing", "unknown", "duplicate", "trailing", "missing field", "missing file field", "null field"} {
		t.Run(mutation, func(t *testing.T) {
			work, report := handoffFixture(t)
			raw, _ := json.Marshal(report)
			switch mutation {
			case "missing":
				raw = nil
			case "unknown":
				raw = append([]byte(`{"unknown":true,`), raw[1:]...)
			case "duplicate":
				raw = append([]byte(`{"version":1,`), raw[1:]...)
			case "trailing":
				raw = append(raw, []byte(` {}`)...)
			case "missing field":
				raw = []byte(strings.Replace(string(raw), `"question":"",`, "", 1))
			case "missing file field":
				raw = []byte(strings.Replace(string(raw), `"kind":"work",`, "", 1))
			case "null field":
				raw = []byte(strings.Replace(string(raw), `"reason":""`, `"reason":null`, 1))
			}
			if raw != nil {
				writeFixture(t, filepath.Join(work, "task-result.json"), string(raw), 0600)
			}
			if _, _, err := readHandoffFixture(work, filepath.Join(t.TempDir(), "observation"), 2); err == nil {
				t.Fatal("accepted invalid JSON")
			}
		})
	}
}

func TestTaskHandoffTotalLimit(t *testing.T) {
	work, report := handoffFixture(t)
	report.Files = []taskHandoffFile{}
	data := []byte(strings.Repeat("x", 21<<20))
	for i := 0; i < 5; i++ {
		name := "candidate-" + strconv.Itoa(i) + ".bin"
		if err := os.WriteFile(filepath.Join(work, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		report.Files = append(report.Files, taskHandoffFile{Path: name, Kind: "work", SHA256: digestText(data)})
	}
	writeHandoff(t, work, report)
	snapshot := filepath.Join(t.TempDir(), "observation")
	if _, _, err := readHandoffFixture(work, snapshot, 2); err == nil {
		t.Fatal("accepted oversized combined handoff")
	}
	if _, err := os.Stat(snapshot); !os.IsNotExist(err) {
		t.Fatal("oversize handoff produced snapshot")
	}
}

func TestTaskHandoffOutcomeMatrix(t *testing.T) {
	for _, status := range []string{"complete", "continue", "blocked", "needs_input"} {
		for _, code := range []int{-1, 0, 1, 2, 3, 75, 125, 130, 137} {
			work, report := handoffFixture(t)
			report.Status, report.Next = status, ""
			want := false
			switch status {
			case "complete":
				want = code == 0
			case "continue":
				report.Next = "Continue this checkpoint"
				want = code == 0 || code == 2
			case "blocked":
				report.Reason, report.Next = "Required tool unavailable", "Repair the tool setup"
				want = code >= 0
			case "needs_input":
				report.Question = "Which supplied location?"
				want = code == 0 || code == 2 || code == 75
			}
			writeHandoff(t, work, report)
			_, _, err := readHandoffFixture(work, filepath.Join(t.TempDir(), "observation"), code)
			if (err == nil) != want {
				t.Fatalf("%s code %d: %v", status, code, err)
			}
		}
	}
}

func TestTaskHandoffRejectsLinksSpecialFilesAndOversize(t *testing.T) {
	for _, kind := range []string{"file link", "directory link", "report link", "fifo", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			work, report := handoffFixture(t)
			writeHandoff(t, work, report)
			target := filepath.Join(work, report.Files[0].Path)
			switch kind {
			case "directory link":
				old := filepath.Join(work, ".team/member")
				moved := filepath.Join(work, "member")
				if err := os.Rename(old, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, old); err != nil {
					t.Fatal(err)
				}
			case "report link":
				old := filepath.Join(work, "task-result.json")
				moved := filepath.Join(work, "report.json")
				if err := os.Rename(old, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, old); err != nil {
					t.Fatal(err)
				}
			case "file link":
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(work, report.Files[1].Path), target); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := syscall.Mkfifo(target, 0600); err != nil {
					t.Fatal(err)
				}
			case "oversize":
				if err := os.Truncate(target, taskHandoffFileLimit+1); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := readHandoffFixture(work, filepath.Join(t.TempDir(), "observation"), 2); err == nil {
				t.Fatal("accepted unsafe file")
			}
		})
	}
}
