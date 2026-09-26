package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The caller owns this append-only file outside all worker/action write roots.
// Ply reads it through its public -steer seam. Saving a note is not a receipt
// proving the model read it, and never starts or retries a command.
type taskMessage struct {
	Attachments string `json:"attachments,omitempty"`
	ID          string `json:"id"`
	Message     string `json:"message"`
	At          string `json:"at"`
}

func taskSteeringDeferred(j Job, rec taskRecord) bool {
	_, err := os.Lstat(filepath.Join(filepath.Dir(j.Dir), "steering-deferred"))
	return !rec.Steering || err == nil
}

func initTaskMessages(root string) error {
	f, err := os.OpenFile(filepath.Join(root, "messages.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return f.Close()
}

func readTaskMessages(root string) ([]taskMessage, error) {
	raw, err := readText(root, "messages.jsonl", 60<<10)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if raw != "" && !strings.HasSuffix(raw, "\n") {
		return nil, fmt.Errorf("message save was interrupted")
	}
	out := []taskMessage{}
	for _, line := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		if line == "" {
			continue
		}
		var item taskMessage
		if len(line) > 8<<10 || strictJSON([]byte(line), &item) != nil || !hexID.MatchString(item.ID) || !boundedText(item.Message, 6000, true) {
			return nil, fmt.Errorf("saved conversation note is invalid")
		}
		out = append(out, item)
	}
	return out, nil
}

func appendTaskMessage(root string, item taskMessage) error {
	notes, err := readTaskMessages(root)
	if err != nil {
		return err
	}
	for _, note := range notes {
		if note.ID == item.ID {
			return nil
		}
	}
	line, err := json.Marshal(item)
	if err != nil || len(line) > 8<<10 {
		return fmt.Errorf("keep this update under 6,000 characters")
	}
	name := filepath.Join(root, "messages.jsonl")
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size()+int64(len(line))+1 > 60<<10 {
		return fmt.Errorf("this attempt’s notes are full; continue the conversation after it stops")
	}
	f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	if err != nil {
		return err
	}
	return f.Sync()
}

func taskSteerArgs(rec taskRecord, root string, args []string) []string {
	if !rec.Steering {
		return args
	}
	out := append([]string{}, args[:2]...)
	out = append(out, "-steer", filepath.Join(root, "messages.jsonl"))
	return append(out, args[2:]...)
}

func taskBuildArgs(rec taskRecord, snapshot, author, goal string) []string {
	return taskSteerArgs(rec, snapshot, buildArgs(rec.Config.Hire, author, goal, rec.Model))
}

func (a *app) taskMessage(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || j.Kind != "task" {
		http.NotFound(w, r)
		return
	}
	rec, err := a.taskRecord(j)
	if err != nil {
		a.fail(w, 409, "That conversation is unavailable.")
		return
	}
	r.SetPathValue("thread", rec.Thread)
	message := strings.TrimSpace(r.PostForm.Get("message"))
	if message == "" && hasTaskUploads(r) {
		message = "Use these attached files for this work."
	}
	nonce := r.PostForm.Get("nonce")
	if !hexID.MatchString(nonce) || !boundedText(message, 6000, true) {
		a.renderTask(w, r, 422, "Keep your update under 6,000 characters.", message)
		return
	}
	err = func() error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !a.cfg.AllowBuild || !a.cfg.AllowRun {
			return fmt.Errorf("Hire is not connected for work")
		}
		turns := a.taskTurns(rec.Thread)
		if len(turns) == 0 || turns[len(turns)-1].Job.ID != j.ID {
			return fmt.Errorf("open the latest conversation before sending this update")
		}
		root := filepath.Dir(j.Dir)
		if !rec.Steering {
			return fmt.Errorf("this older attempt cannot receive live updates; your message is kept below for when it stops")
		}
		_, manifest, err := a.acceptTaskUploads(rec.Thread, r)
		if err != nil {
			return err
		}
		return appendTaskMessage(root, taskMessage{ID: nonce, Message: message, At: time.Now().UTC().Format(time.RFC3339Nano), Attachments: manifest})
	}()
	if err != nil {
		a.renderTask(w, r, 409, err.Error(), message)
		return
	}
	http.Redirect(w, r, "/work/"+rec.Thread, 303)
}
