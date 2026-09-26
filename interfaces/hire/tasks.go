package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type taskChoice struct {
	Flash       *taskFlashTeam    `json:"flash,omitempty"`
	Members     map[string]member `json:",omitempty"`
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind"`
	Description string            `json:"description"`
	Path        string            `json:"path"`
	ID          string            `json:"id"`
	Local       bool              `json:"local"`
	NeedsBuild  bool              `json:"needs_build"`
}
type taskRecord struct {
	Research                                               bool             `json:",omitempty"`
	Attachments                                            []taskAttachment `json:",omitempty"`
	Steering                                               bool             `json:",omitempty"`
	Resume                                                 *taskResume      `json:",omitempty"`
	Thread, Message, Focus, Previous, Model, Now, Revision string
	History                                                []assistantMessage
	Catalog                                                []taskChoice
	Config                                                 config
}
type taskArtifact struct {
	Name, Type string
	Size       int64
	Preview    string `json:"-"`
}
type taskResult struct {
	Flash                                  *taskFlashTeam `json:"flash,omitempty"`
	Update                                 taskUpdate     `json:",omitempty"`
	Message, Question, Title, Expert, Kind string
	Prepared                               bool
	Code                                   int
	Artifacts                              []taskArtifact
}
type taskTurn struct {
	Specialist taskSpecialist
	Deferred   bool
	Messages   []taskMessage
	Update     taskUpdate
	Recovery   string
	Stopped    bool
	Job        Job
	Record     taskRecord
	Result     taskResult
	Progress   string
	Delivery   deliveryView
	Outputs    []taskOutput
}
type taskView struct {
	Research                    bool
	Attachments                 []taskAttachment
	Deferred                    bool
	Thread, Draft, Focus, Error string
	ActiveJob                   string
	Turns, Recent               []taskTurn
	Active                      bool
}

func saveTaskJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func (a *app) taskRecord(j Job) (taskRecord, error) {
	var rec taskRecord
	rel, _ := filepath.Rel(a.cfg.Data, filepath.Join(filepath.Dir(j.Dir), "task.json"))
	b, e := readText(a.cfg.Data, rel, 2<<20)
	if e == nil {
		e = json.Unmarshal([]byte(b), &rec)
	}
	return rec, e
}
func (a *app) taskResult(j Job) taskResult {
	var out taskResult
	rel, _ := filepath.Rel(a.cfg.Data, filepath.Join(filepath.Dir(j.Dir), "result.json"))
	b, e := readText(a.cfg.Data, rel, 128<<10)
	if e == nil {
		_ = json.Unmarshal([]byte(b), &out)
	}
	if out.Update.Blocked != "" && out.Message == "Your work is ready to review. Tell me what you’d like to change." {
		out.Message = "Your available work is saved. Still needed: " + out.Update.Blocked
	}
	if out.Message == "" && !j.Active() {
		out.Message = "I stopped before I could finish. Your earlier work is saved. Tell me what you’d like to try next."
	}
	return out
}
func (a *app) taskTurns(thread string) []taskTurn {
	out := []taskTurn{}
	for _, j := range a.jobs.List() {
		if j.Kind != "task" {
			continue
		}
		rec, e := a.taskRecord(j)
		if e != nil || (thread != "" && rec.Thread != thread) {
			continue
		}
		turn := taskTurn{Job: j, Record: rec, Result: a.taskResult(j)}
		if !j.Active() && thread != "" {
			for i, f := range turn.Result.Artifacts {
				if f.Type == "text/plain" && f.Size <= 24000 {
					if b, err := taskFile(filepath.Join(filepath.Dir(j.Dir), "deliverables"), f.Name); err == nil {
						turn.Result.Artifacts[i].Preview = string(b)
					}
				}
			}
		}
		if j.Active() {
			rel, _ := filepath.Rel(a.cfg.Data, filepath.Join(filepath.Dir(j.Dir), "progress.txt"))
			turn.Progress, _ = readText(a.cfg.Data, rel, 2048)
			if turn.Progress == "" {
				turn.Progress = "Thinking about the best way to help…"
			}
		}
		turn.Messages, _ = readTaskMessages(filepath.Dir(j.Dir))
		turn.Deferred = taskSteeringDeferred(j, rec)
		turn.Update = a.taskUpdate(j, rec, turn.Result)
		if thread != "" {
			turn.Specialist = a.taskSpecialist(j, rec, turn.Result)
		}
		out = append(out, turn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Job.Started.Before(out[j].Job.Started) })
	seenMessages := map[string]bool{}
	for i := range out {
		visible := []taskMessage{}
		for _, note := range out[i].Messages {
			if !seenMessages[note.ID] {
				visible = append(visible, note)
				seenMessages[note.ID] = true
			}
		}
		out[i].Messages = visible
	}
	latest := -1
	for i := range out {
		if len(out[i].Result.Artifacts) > 0 {
			latest = i
		}
	}
	if thread != "" {
		for i := range out {
			out[i].Outputs = taskOutputs(filepath.Join(filepath.Dir(out[i].Job.Dir), "deliverables"), out[i].Result)
		}
		if latest >= 0 {
			out[latest].Delivery = a.deliveryView(thread, out[latest].Job.ID)
		}
		if len(out) > 0 {
			last := &out[len(out)-1]
			last.Stopped = !last.Job.Active() && last.Job.State != "completed"
			if last.Stopped && a.cfg.AllowBuild && a.cfg.AllowRun {
				last.Recovery = "retry"
				if _, err := a.taskResumeInfo(last.Job, last.Record); err == nil {
					last.Recovery = "resume"
				}
			}
		}
	}
	return out
}
func (a *app) taskPage(w http.ResponseWriter, r *http.Request) { a.renderTask(w, r, 200, "", "") }
func (a *app) renderTask(w http.ResponseWriter, r *http.Request, code int, problem, draft string) {
	thread := r.PathValue("thread")
	if thread != "" && !hexID.MatchString(thread) {
		a.fail(w, 404, "That conversation was not found.")
		return
	}
	v := taskView{Thread: thread, Focus: r.URL.Query().Get("focus"), Draft: draft, Error: problem}
	if thread != "" {
		v.Attachments, _ = taskAttachments(a.cfg.Data, thread)
		v.Turns = a.taskTurns(thread)
		if len(v.Turns) > 0 {
			v.Research = v.Turns[len(v.Turns)-1].Record.Research
		}
		if len(v.Turns) == 0 {
			a.fail(w, 404, "That conversation was not found.")
			return
		}
	}
	for _, turn := range v.Turns {
		if turn.Job.Active() {
			v.ActiveJob = turn.Job.ID
			v.Deferred = turn.Deferred
		}
	}
	seen := map[string]bool{}
	all := a.taskTurns("")
	for i := len(all) - 1; i >= 0; i-- {
		t := all[i]
		if !seen[t.Record.Thread] && len(v.Recent) < 12 {
			v.Recent = append(v.Recent, t)
			seen[t.Record.Thread] = true
		}
	}
	for _, j := range a.jobs.List() {
		if j.Active() {
			v.Active = true
			break
		}
	}
	a.render(w, code, page{Title: "Your work", View: "tasks", Nav: "work", Work: v})
}
func (a *app) taskChoices() []taskChoice {
	c := a.catalog()
	out := []taskChoice{}
	for _, e := range append(c.Workers, c.Teams...) {
		if !e.Local && !e.Exportable() {
			continue
		}
		scope, path := "source", filepath.Join(a.cfg.Source, e.Kind+"s", e.ID, "expert")
		if e.Local {
			scope, path = "local", e.Path
			if e.Kind == "team" && e.WorkThread == "" {
				j, _, err := a.latestTeam(e.ID)
				if err != nil {
					continue
				}
				path = filepath.Join(j.Dir, "expert")
			}
		}
		out = append(out, taskChoice{Key: e.Kind + ":" + scope + ":" + e.ID, Name: e.Name(), Kind: e.Kind, Description: e.Description, Path: path, ID: e.ID, Local: e.Local, Members: e.Members})
	}
	return out
}
func (a *app) taskSend(w http.ResponseWriter, r *http.Request) {
	message := strings.TrimSpace(r.PostForm.Get("message"))
	if message == "" && hasTaskUploads(r) {
		message = "Use these attached files for this work."
	}
	thread := r.PostForm.Get("thread")
	if thread == "" {
		thread = randomID()
	}
	fail := func(code int, s string) {
		if len(a.taskTurns(thread)) > 0 {
			r.SetPathValue("thread", thread)
		}
		a.renderTask(w, r, code, s, message)
	}
	if !a.cfg.AllowBuild || !a.cfg.AllowRun {
		fail(403, "Hire is not connected for work yet. Your message is saved here.")
		return
	}
	if !hexID.MatchString(thread) || !boundedText(message, 12000, true) {
		fail(422, "Tell me what you’d like done, in a message under 12,000 characters.")
		return
	}
	if !modelName.MatchString(a.cfg.Model) {
		fail(422, "A model connection needs to be configured for Hire.")
		return
	}
	out := &assistantResponse{header: make(http.Header)}
	uploadProblem := ""
	a.admit(out, r, func() (Job, error) {
		turns := a.taskTurns(thread)
		if previous := r.PostForm.Get("retry-job"); previous != "" {
			if len(turns) == 0 || turns[len(turns)-1].Job.ID != previous || !turns[len(turns)-1].Stopped {
				return Job{}, fmt.Errorf("open the latest work before trying again")
			}
		}
		rec := taskRecord{Research: r.PostForm.Get("research") == "yes", Steering: true, Thread: thread, Message: message, Focus: r.PostForm.Get("focus"), Model: a.cfg.Model, Now: time.Now().Format(time.RFC3339), Revision: a.cat.Revision, Config: a.cfg, Catalog: a.taskChoices(), History: []assistantMessage{}}
		for _, t := range turns {
			if t.Job.Active() {
				return Job{}, fmt.Errorf("this conversation is still working")
			}
			if t.Job.State == "unknown" || (t.Job.ExitCode != nil && *t.Job.ExitCode != 0 && *t.Job.ExitCode != 2 && *t.Job.ExitCode != 130) {
				rec.History = append(rec.History, assistantMessage{Role: "assistant", Text: "The previous process outcome was not observed. This new user message permits fresh local work only; never repeat an external action or claim the previous process completed."})
			}
			rec.History = append(rec.History, assistantMessage{Role: "user", Text: t.Record.Message}, assistantMessage{Role: "assistant", Text: t.Result.Message + "\n" + t.Result.Question})
			for _, note := range t.Messages {
				rec.History = append(rec.History, assistantMessage{Role: "user", Text: note.Message})
			}
			if len(t.Result.Artifacts) > 0 || (rec.Previous == "" && t.Result.Expert != "") {
				rec.Previous = filepath.Dir(t.Job.Dir)
			}
		}
		// Keep recent conversation bounded; durable earlier turns remain on screen.
		if len(rec.History) > 24 {
			rec.History = rec.History[len(rec.History)-24:]
		}
		dir, err := a.workspace()
		if err != nil {
			return Job{}, err
		}
		parent := filepath.Dir(dir)
		if rec.Previous != "" {
			var last taskTurn
			for _, t := range turns {
				if filepath.Dir(t.Job.Dir) == rec.Previous {
					last = t
				}
			}
			if last.Result.Expert != "" {
				rec.Catalog = append(rec.Catalog, taskChoice{Key: "previous", Name: "Your current specialist", Kind: last.Result.Kind, Path: last.Result.Expert, Flash: last.Result.Flash, Local: true, NeedsBuild: !last.Result.Prepared})
			}
		}
		if strings.HasPrefix(rec.Focus, "job:") {
			j, ok := a.jobs.Get(strings.TrimPrefix(rec.Focus, "job:"))
			if !ok || j.Active() {
				return Job{}, fmt.Errorf("that work is unavailable right now")
			}
			rec.History = append(rec.History, assistantMessage{Role: "assistant", Text: "Selected prior activity: " + j.Title + "\n" + a.jobs.Log(j.ID, "stderr")})
			if j.Kind == "run" && len(j.Args) > 0 {
				rec.Catalog = append(rec.Catalog, taskChoice{Key: "selected", Name: j.Title, Kind: "worker", Path: j.Args[len(j.Args)-1], Local: true})
			}
		}
		rec.Catalog = withConversationFlashTeams(rec.Catalog, turns)
		rec.Catalog = withTaskAdaptations(rec.Catalog)
		profile, _, err := assistantDefinition(filepath.Join(a.assistantPath(), "task"))
		if err != nil {
			return Job{}, fmt.Errorf("Hire’s work companion is unavailable: %w", err)
		}
		if err = writeDefinition(filepath.Join(parent, "companion"), profile); err != nil {
			return Job{}, err
		}
		// Copy the reusable reference notes the companion is taught to consult.
		for _, name := range []string{"AGENT.md", "HIRE.md", "ASK.md", "PLY.md", "TRAIL.md", "BENCH.md", "knowledge.md"} {
			if b, e := os.ReadFile(filepath.Join(a.assistantPath(), name)); e == nil {
				if e = os.WriteFile(filepath.Join(parent, name), b, 0600); e != nil {
					return Job{}, e
				}
			}
		}
		rec.Attachments, _, err = a.acceptTaskUploads(thread, r)
		if err != nil {
			uploadProblem = err.Error()
			return Job{}, err
		}
		if err = initTaskMessages(parent); err != nil {
			return Job{}, err
		}
		if err = saveTaskJSON(filepath.Join(parent, "task.json"), rec); err != nil {
			return Job{}, err
		}
		self, err := os.Executable()
		if err != nil {
			return Job{}, err
		}
		return a.jobs.Start("task", truncateMessage(message, 80), dir, []string{self, "task", filepath.Join(parent, "task.json")}, "")
	})
	if out.status != 303 {
		if uploadProblem != "" {
			fail(out.status, uploadProblem+" Choose your files again before sending.")
			return
		}
		fail(out.status, "I couldn’t start just yet. Another task may still be working; your message is kept here.")
		return
	}
	j, _ := a.jobs.Get(strings.TrimPrefix(out.header.Get("Location"), "/jobs/"))
	rec, _ := a.taskRecord(j)
	http.Redirect(w, r, "/work/"+rec.Thread, 303)
}
func (a *app) taskStatus(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || (j.Kind != "task" && j.Kind != "delivery") {
		http.NotFound(w, r)
		return
	}
	progress := ""
	if j.Active() {
		rel, _ := filepath.Rel(a.cfg.Data, filepath.Join(filepath.Dir(j.Dir), "progress.txt"))
		progress, _ = readText(a.cfg.Data, rel, 2048)
	}
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{"active": j.Active(), "message": progress}
	if j.Kind == "task" {
		if rec, err := a.taskRecord(j); err == nil {
			response["update"] = a.taskUpdate(j, rec, a.taskResult(j))
			response["specialist"] = a.taskSpecialist(j, rec, a.taskResult(j))
			response["deferred"] = taskSteeringDeferred(j, rec)
		}
	}
	if j.Kind == "delivery" && !j.Active() && j.ExitCode != nil && *j.ExitCode == 0 {
		var rec deliveryRecord
		if b, e := os.ReadFile(filepath.Join(filepath.Dir(j.Dir), "delivery-request.json")); e == nil && json.Unmarshal(b, &rec) == nil && rec.Action == "prepare" {
			response["destination"] = "/work/jobs/" + rec.TaskID + "/delivery/"
		}
	}
	_ = json.NewEncoder(w).Encode(response)
}

func taskModelCatalog(choices []taskChoice) []map[string]string {
	out := []map[string]string{}
	for _, c := range choices {
		out = append(out, map[string]string{"key": c.Key, "name": c.Name, "kind": c.Kind, "description": c.Description, "path": c.Path})
	}
	return out
}

func (a *app) taskStop(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || j.Kind != "task" {
		http.NotFound(w, r)
		return
	}
	rec, err := a.taskRecord(j)
	if err != nil {
		a.fail(w, 409, "That conversation could not be found.")
		return
	}
	if err = a.jobs.Cancel(j.ID); err != nil {
		a.fail(w, 409, err.Error())
		return
	}
	http.Redirect(w, r, "/work/"+rec.Thread, 303)
}
