package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode"
)

const taskHandoffFileLimit = 25 << 20
const taskHandoffTotalLimit = 100 << 20

type taskHandoffFile struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
}

// A team's report is evidence, not authority to replay a process. The caller
// supplies the observed exit status and retains its own process receipt.
type taskHandoff struct {
	Version          int               `json:"version"`
	Invocation       string            `json:"invocation"`
	GoalSHA256       string            `json:"goal_sha256"`
	DefinitionSHA256 string            `json:"definition_sha256"`
	Status           string            `json:"status"`
	Stage            string            `json:"stage"`
	Message          string            `json:"message"`
	Reason           string            `json:"reason"`
	Next             string            `json:"next"`
	Question         string            `json:"question"`
	Files            []taskHandoffFile `json:"files"`
}

func taskHandoffPath(name string) bool {
	return name != "." && path.Clean(name) == name && filepath.IsLocal(name) &&
		!strings.Contains(name, "\\") && strings.IndexFunc(name, unicode.IsControl) < 0
}

// Open every directory relative to a confined root and reject links at each
// component. Check the opened inode too, so replacement cannot substitute an
// unchecked directory or special file between inspection and opening.
func taskHandoffRead(root *os.Root, name string, limit int64) ([]byte, error) {
	if !taskHandoffPath(name) {
		return nil, fmt.Errorf("invalid handoff path %q", name)
	}
	parts := strings.Split(name, "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		before, err := current.Lstat(part)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("handoff path has a missing or linked directory: %s", name)
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			return nil, err
		}
		defer next.Close()
		after, err := next.Stat(".")
		link, linkErr := current.Lstat(part)
		if err != nil || linkErr != nil || link.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, after) {
			return nil, fmt.Errorf("handoff directory changed: %s", name)
		}
		current = next
	}
	base := parts[len(parts)-1]
	before, err := current.Lstat(base)
	if err != nil || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, fmt.Errorf("handoff file is not a bounded regular file: %s", name)
	}
	f, err := current.OpenFile(base, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || after.Size() > limit || !os.SameFile(before, after) {
		return nil, fmt.Errorf("handoff file changed: %s", name)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("handoff file exceeds size limit: %s", name)
	}
	return b, err
}

func taskHandoffHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func readTaskHandoff(work, snapshot, invocation, goalHash, definitionHash string, code int) (taskHandoff, string, error) {
	var report taskHandoff
	fail := func(message string) (taskHandoff, string, error) {
		return taskHandoff{}, "", fmt.Errorf("team handoff: %s", message)
	}
	workInfo, err := os.Lstat(work)
	if err != nil || !workInfo.IsDir() || workInfo.Mode()&os.ModeSymlink != 0 {
		return fail("work root must be an existing unlinked directory")
	}
	root, err := os.OpenRoot(work)
	if err != nil {
		return fail(err.Error())
	}
	defer root.Close()
	openedWork, err := root.Stat(".")
	currentWork, currentErr := os.Lstat(work)
	if err != nil || currentErr != nil || currentWork.Mode()&os.ModeSymlink != 0 || !os.SameFile(workInfo, openedWork) {
		return fail("work root changed")
	}
	raw, err := taskHandoffRead(root, "task-result.json", 64<<10)
	if err != nil {
		return fail(err.Error())
	}
	var fields map[string]json.RawMessage
	if strictJSON(raw, &report) != nil || json.Unmarshal(raw, &fields) != nil || len(fields) != 11 || report.Files == nil || len(report.Files) > 64 {
		return fail("invalid result schema")
	}
	for _, value := range fields {
		if strings.TrimSpace(string(value)) == "null" {
			return fail("result fields cannot be null")
		}
	}
	if invocation == "" || report.Version != 1 || report.Invocation != invocation || !taskHandoffHash(goalHash) || !taskHandoffHash(definitionHash) || report.GoalSHA256 != goalHash || report.DefinitionSHA256 != definitionHash {
		return fail("result does not match this invocation, goal and definition")
	}
	if !boundedText(report.Stage, 120, true) || strings.IndexFunc(report.Stage, unicode.IsControl) >= 0 || !boundedText(report.Message, 2000, true) || !boundedText(report.Reason, 2000, false) || !boundedText(report.Next, 2000, false) || !boundedText(report.Question, 1000, false) {
		return fail("invalid status explanation")
	}
	valid := false
	switch report.Status {
	case "complete":
		valid = code == 0 && report.Reason == "" && report.Next == "" && report.Question == ""
	case "continue":
		valid = (code == 0 || code == 2) && strings.TrimSpace(report.Next) != "" && report.Reason == "" && report.Question == ""
	case "blocked":
		valid = code >= 0 && strings.TrimSpace(report.Reason) != "" && strings.TrimSpace(report.Next) != "" && report.Question == ""
	case "needs_input":
		valid = (code == 0 || code == 2 || code == 75) && strings.TrimSpace(report.Question) != "" && report.Next == "" && report.Reason == ""
	}
	if !valid {
		return fail("reported state contradicts the observed process outcome or lacks its next step")
	}
	var rawFiles []map[string]json.RawMessage
	if json.Unmarshal(fields["files"], &rawFiles) != nil {
		return fail("invalid file manifest")
	}
	contents := make(map[string][]byte, len(report.Files))
	progress := []string{}
	total := 0
	for i, file := range report.Files {
		if len(rawFiles[i]) != 3 || !taskHandoffPath(file.Path) || file.Path == "task-result.json" || !taskHandoffHash(file.SHA256) || (file.Kind != "work" && file.Kind != "evidence" && file.Kind != "deliverable") {
			return fail("invalid manifest entry")
		}
		if _, exists := contents[file.Path]; exists {
			return fail("duplicate manifest path")
		}
		data, e := taskHandoffRead(root, file.Path, taskHandoffFileLimit)
		if e != nil {
			return fail(e.Error())
		}
		total += len(data)
		if total > taskHandoffTotalLimit || digestText(data) != file.SHA256 {
			return fail("manifest content digest or total size does not match")
		}
		contents[file.Path] = data
		if file.Kind != "evidence" {
			progress = append(progress, file.Path+"\x00"+file.SHA256)
		}
	}
	// A distinct controller-selected snapshot is mandatory. Never overwrite an
	// earlier observation, even if it happens to contain the same result.
	if err = os.MkdirAll(filepath.Dir(snapshot), 0700); err != nil {
		return fail(err.Error())
	}
	if err = os.Mkdir(snapshot, 0700); err != nil {
		return fail("snapshot already exists or cannot be created")
	}
	retained := false
	defer func() {
		if !retained {
			_ = os.RemoveAll(snapshot)
		}
	}()
	for name, data := range contents {
		dest := filepath.Join(snapshot, "files", filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err == nil {
			err = os.WriteFile(dest, data, 0600)
		}
		if err != nil {
			return fail("could not retain selected files: " + err.Error())
		}
	}
	if err = os.WriteFile(filepath.Join(snapshot, "task-result.json"), raw, 0600); err != nil {
		return fail("could not retain result: " + err.Error())
	}
	retained = true
	sort.Strings(progress)
	return report, digestText([]byte(strings.Join(progress, "\n"))), nil
}
