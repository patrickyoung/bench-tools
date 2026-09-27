package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const taskTeamBackend = "manage-team/v1"

type taskTeamReference struct {
	Version          string `json:"version"`
	Run              string `json:"run"`
	ID               string `json:"id"`
	DefinitionSHA256 string `json:"definition_sha256"`
}
type taskTeamExecution struct {
	State string `json:"state"`
	Exit  *int   `json:"exit"`
}
type taskTeamStatus struct {
	DriverActive  bool              `json:"driver_active"`
	LastExecution taskTeamExecution `json:"last_execution"`
	Schema        string            `json:"schema"`
	Run           string            `json:"run"`
	RequestID     string            `json:"request_id"`
	Status        string            `json:"status"`
	Phase         string            `json:"phase"`
	Worker        *string           `json:"worker"`
	Message       string            `json:"message"`
	Diagnostic    string            `json:"diagnostic"`
	Revision      int               `json:"revision"`
	Question      string            `json:"question"`
	QuestionID    *string           `json:"question_id"`
	Resumable     bool              `json:"resumable"`
	AttemptsUsed  int               `json:"attempts_used"`
	AttemptsLimit int               `json:"attempts_limit"`
	Execution     struct {
		State string `json:"state"`
		Exit  *int   `json:"exit"`
	} `json:"execution"`
	Messages []struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Revision int    `json:"revision"`
	} `json:"messages"`
	Files []struct {
		Path   string `json:"path"`
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
		Kind   string `json:"kind"`
	} `json:"files"`
}

func (s taskTeamStatus) CanSend() bool {
	return s.Status == "ready" || s.Status == "running" || ((s.Status == "blocked" || s.Status == "needs_input") && s.Resumable)
}
func prepareTeamCoordinator(cfg *config) error {
	if (cfg.TeamCoordinator == "") != (cfg.TeamQueue == "") {
		return fmt.Errorf("select -team-coordinator and -team-queue together")
	}
	if cfg.TeamCoordinator == "" {
		return nil
	}
	p, err := exec.LookPath(cfg.TeamCoordinator)
	if err != nil {
		return fmt.Errorf("team coordinator: %w", err)
	}
	cfg.TeamCoordinator, err = filepath.Abs(p)
	if err != nil {
		return err
	}
	p, err = filepath.Abs(cfg.TeamQueue)
	if err != nil {
		return err
	}
	p, err = resolveNewPath(p)
	if err != nil {
		return err
	}
	if !within(filepath.Join(cfg.Data, "workspaces"), p) || p == filepath.Join(cfg.Data, "workspaces") {
		return fmt.Errorf("-team-queue must be a dedicated directory under data/workspaces")
	}
	if err = os.MkdirAll(p, 0700); err != nil {
		return err
	}
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("team queue must be a private directory")
	}
	cfg.TeamQueue = p
	return nil
}

// Only bounded public client operations run in the UI. Neither work nor resume
// is admitted here; the separately selected host worker owns child lifetimes.
type teamClientOutput struct {
	buffer bytes.Buffer
	full   bool
}

func (b *teamClientOutput) Write(p []byte) (int, error) {
	n := len(p)
	if b.buffer.Len()+n > 1<<20 {
		b.full = true
		return n, nil
	}
	_, err := b.buffer.Write(p)
	return n, err
}
func taskTeamCommand(ctx context.Context, cfg config, input []byte, args ...string) ([]byte, int, error) {
	if cfg.TeamCoordinator == "" {
		return nil, 125, fmt.Errorf("team coordinator is not selected")
	}
	limit := 20 * time.Second
	if len(args) > 0 && args[0] == "preflight" {
		limit = 150 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, cfg.TeamCoordinator, append([]string{"team"}, args...)...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.WaitDelay = time.Second
	out, diag := &teamClientOutput{}, &teamClientOutput{}
	cmd.Stdout = out
	cmd.Stderr = diag
	err := cmd.Run()
	if len(args) > 0 && (args[0] == "admit" || args[0] == "validate" || args[0] == "preflight") {
		_, _ = os.Stdout.Write(out.buffer.Bytes())
		_, _ = os.Stderr.Write(diag.buffer.Bytes())
	}
	code := 0
	if err != nil {
		code = 125
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
			if s, ok := e.Sys().(syscall.WaitStatus); ok && s.Signaled() {
				code = 128 + int(s.Signal())
			}
		}
	}
	if ctx.Err() != nil {
		return nil, 125, fmt.Errorf("coordinator transport interrupted: %w", ctx.Err())
	}
	if out.full || diag.full {
		return nil, 125, fmt.Errorf("coordinator response exceeds bounds")
	}
	if err != nil {
		detail := strings.TrimSpace(diag.buffer.String())
		if detail == "" {
			detail = err.Error()
		}
		return out.buffer.Bytes(), code, fmt.Errorf("coordinator %s stopped (%d): %s", args[0], code, truncateMessage(detail, 1000))
	}
	return out.buffer.Bytes(), code, nil
}
func decodeTaskTeam(raw []byte, ref taskTeamReference, schema string) (taskTeamStatus, error) {
	var s taskTeamStatus
	if json.Unmarshal(raw, &s) != nil || s.Schema != schema || s.Run != ref.Run || s.RequestID != ref.ID || s.Revision < 0 || s.Revision > 1024 || !boundedText(s.Message, 6000, true) || !boundedText(s.Question, 2000, false) || len(s.Diagnostic) > 16000 {
		return s, fmt.Errorf("coordinator returned an unbound status")
	}
	switch s.Status {
	case "ready", "running", "needs_input", "blocked", "unknown", "cancelled", "complete", "cancelling":
	default:
		return s, fmt.Errorf("coordinator returned an unknown state")
	}
	if s.Status == "needs_input" && (s.QuestionID == nil || s.Question == "") {
		return s, fmt.Errorf("coordinator question lacks an identity")
	}
	return s, nil
}
func readTaskTeamReference(j Job, rec taskRecord) (taskTeamReference, error) {
	var ref taskTeamReference
	if rec.TeamBackend != taskTeamBackend {
		return ref, os.ErrNotExist
	}
	root := filepath.Dir(j.Dir)
	raw, e := readText(root, "team-run.json", 4096)
	if e != nil {
		return ref, e
	}
	if strictJSON([]byte(raw), &ref) != nil || ref.Version != taskTeamBackend || !hexID.MatchString(ref.ID) || ref.Run != filepath.Join(rec.Config.TeamQueue, ref.ID) || !within(filepath.Join(rec.Config.Data, "workspaces"), ref.Run) || !teamFingerprint.MatchString(ref.DefinitionSHA256) {
		return ref, fmt.Errorf("saved coordinator reference is invalid")
	}
	return ref, nil
}
func validateTaskTeam(ctx context.Context, root string, rec taskRecord) error {
	expert := filepath.Join(root, "authoring", "expert")
	// Public structural validation never executes the recipe or its checks.
	raw, _, err := taskTeamCommand(ctx, rec.Config, nil, "validate", "-definition", expert)
	if err != nil {
		return err
	}
	var valid struct {
		Schema string `json:"schema"`
		Valid  bool   `json:"valid"`
		Steps  []struct {
			ID     string `json:"id"`
			Worker string `json:"worker"`
		} `json:"steps"`
	}
	if json.Unmarshal(raw, &valid) != nil || valid.Schema != "bench.team.validation/v1" || !valid.Valid {
		return fmt.Errorf("team recipe did not pass structural validation")
	}
	var members []preparedTaskMember
	b, err := os.ReadFile(filepath.Join(root, "selected-members.json"))
	if err != nil {
		return err
	}
	if json.Unmarshal(b, &members) != nil || len(members) < 2 || len(members) != len(valid.Steps) {
		return fmt.Errorf("team recipe does not preserve the selected roster")
	}
	selected := map[string]bool{}
	for _, m := range members {
		selected[m.Role] = true
	}
	seen := map[string]bool{}
	for _, step := range valid.Steps {
		if !selected[step.ID] || seen[step.ID] || step.Worker != "agents/"+step.ID {
			return fmt.Errorf("team recipe replaced or omitted selected member %s", step.ID)
		}
		seen[step.ID] = true
	}
	return nil
}
func stageTaskTeamInputs(root, snapshot, stage, work string, rec taskRecord, plan taskPlan) (string, string, taskRecord, error) {
	var err error
	rec.Attachments, err = taskAttachments(rec.Config.Data, rec.Thread)
	if err != nil {
		return "", "", rec, err
	}
	inputs := filepath.Join(stage, "team-originals")
	if err = os.MkdirAll(inputs, 0700); err != nil {
		return "", "", rec, err
	}
	if err = copyTaskArtifacts(work, inputs); err != nil {
		return "", "", rec, err
	}
	attachments := []map[string]any{}
	for i, f := range rec.Attachments {
		b, e := taskFile(filepath.Dir(f.Path), filepath.Base(f.Path))
		if e != nil || int64(len(b)) != f.Size || digestText(b) != f.SHA256 {
			return "", "", rec, fmt.Errorf("selected attachment changed")
		}
		name := fmt.Sprintf("attachment-%02d%s", i+1, strings.ToLower(filepath.Ext(f.Name)))
		if _, e = os.Lstat(filepath.Join(inputs, name)); !os.IsNotExist(e) {
			return "", "", rec, fmt.Errorf("attachment input name conflicts")
		}
		if e = os.WriteFile(filepath.Join(inputs, name), b, 0600); e != nil {
			return "", "", rec, e
		}
		attachments = append(attachments, map[string]any{"file": name, "name": f.Name, "sha256": f.SHA256})
	}
	if _, e := os.Lstat(filepath.Join(inputs, "team-attachments.json")); !os.IsNotExist(e) {
		return "", "", rec, fmt.Errorf("team-attachments.json is reserved for the controller attachment map")
	}
	if err = saveTaskJSON(filepath.Join(inputs, "team-attachments.json"), map[string]any{"schema": "hire.team.attachments/v1", "files": attachments}); err != nil {
		return "", "", rec, err
	}
	attachmentJSON, _ := json.Marshal(attachments)
	goal := plan.Brief + "\n\nPreserve the supplied editable originals and unaffected design when editing. Complete the selected goal and its checks. Do not send messages, publish, purchase, install dependencies or change settings. Deliver only requested outputs; source references are not deliverables. Final deliverable names must be flat, unique, supported by the interface, at most 32 files / 25 MiB each / 100 MiB total.\nSupplied attachments in originals packet: " + string(attachmentJSON)
	goal += "\nProvide delivery.json when useful: title, summary, artifacts [{id,title,description,original,preview,thumbnail,pages}]. Each original/preview/thumbnail/pages entry names an included flat file. Keep editable native originals and requested PDF or image previews; group derivatives with the original. Supported files: .pdf .png .jpg .jpeg .webp .gif .svg .html .htm .css .js .mjs .woff .woff2 .ico .md .txt .csv .tsv .json .docx .pptx .xlsx .zip. Never claim unavailable conversions succeeded."
	if rec.Research {
		goal += "\nThe user enabled public web research. Use public sources when needed, preserve URLs/dates, and report what was verified. This grants no publication, messaging, purchase or private-account access."
	} else {
		goal += "\nWeb research is not enabled. Use supplied local evidence; report missing research plainly and never imply online verification occurred."
	}
	seenNotes := map[string]bool{}
	for _, notesRoot := range []string{root, snapshot} {
		if notes, e := readTaskMessages(notesRoot); e == nil {
			for _, note := range notes {
				if !seenNotes[note.ID] {
					goal += "\nUser update saved during preparation: " + note.Message
					seenNotes[note.ID] = true
				}
			}
		} else {
			return "", "", rec, e
		}
	}
	goalPath := filepath.Join(stage, "team-goal.txt")
	if err = os.WriteFile(goalPath, []byte(goal), 0600); err != nil {
		return "", "", rec, err
	}
	return inputs, goalPath, rec, nil
}

func admitTaskTeam(ctx context.Context, root, snapshot string, rec taskRecord, plan taskPlan, result taskResult, work string) int {
	fail := func(code int, err error) int {
		result.GoalStatus = "blocked"
		return finishTask(snapshot, result, code, "Team admission was not confirmed: "+err.Error()+". Its prepared workers and inputs are saved.")
	}
	if err := validateTaskTeam(ctx, root, rec); err != nil {
		return fail(2, err)
	}
	id := filepath.Base(root)
	if !hexID.MatchString(id) {
		return fail(125, fmt.Errorf("invalid saved task identity"))
	}
	ref := taskTeamReference{Version: taskTeamBackend, Run: filepath.Join(rec.Config.TeamQueue, id), ID: id}
	// No production admission occurs until the last checked bytes are captured
	// under the same short lock used by the preparation composer.
	capture := func() (string, string, error) {
		unlock, err := lockTaskTeamAdmission(snapshot)
		if err != nil {
			return "", "", err
		}
		defer unlock()
		if _, err = os.Lstat(filepath.Join(snapshot, "team-run.json")); !os.IsNotExist(err) {
			return "", "", fmt.Errorf("an admission reference already exists; inspect its status before any further action")
		}
		stage, err := os.MkdirTemp(snapshot, "team-admission-")
		if err != nil {
			return "", "", err
		}
		inputs, goal, current, err := stageTaskTeamInputs(root, snapshot, stage, work, rec, plan)
		if err != nil {
			return "", "", err
		}
		_, hash, err := definitionSnapshot(rec.Config.Data, result.Expert)
		if err != nil {
			return "", "", err
		}
		if err = verifyTaskTeamPreflight(snapshot, hash, inputs, goal); err != nil {
			return "", "", err
		}
		ref.DefinitionSHA256 = hash
		if err = saveTaskJSON(filepath.Join(snapshot, "team-run.json"), ref); err != nil {
			return "", "", err
		}
		rec = current
		if err = saveTaskJSON(filepath.Join(snapshot, "task.json"), rec); err != nil {
			return "", "", err
		}
		result.GoalStatus = "admitting"
		result.Message = "The selected team is prepared; confirming its admission."
		if err = saveTaskJSON(filepath.Join(snapshot, "result.json"), result); err != nil {
			return "", "", err
		}
		return inputs, goal, nil
	}
	var inputs, goalPath string
	var err error
	for recheck := 0; ; recheck++ {
		inputs, goalPath, err = capture()
		if err == nil {
			break
		}
		if _, changed := err.(taskTeamInputsChanged); !changed {
			return fail(125, err)
		}
		if recheck >= 2 {
			result.Prepared = false
			return fail(2, fmt.Errorf("inputs kept changing across three checked captures; the latest update is saved, but preparation needs a stable input set"))
		}
		taskPhase(snapshot, "Including your latest update and rechecking the input handoff…")
		if code, checkErr := repairPreparedTaskMembers(ctx, root, snapshot, rec, plan); checkErr != nil {
			result.Prepared = false
			return fail(code, checkErr)
		}
	}
	args := []string{"admit", "-C", ref.Run, "-definition", result.Expert, "-inputs", inputs, "-goal-file", goalPath, "-id", ref.ID, "-m", rec.Model, "-attempts", "48", "-turns", "50", "-timeout", "600", "-deadline", "2700", "-corrections", "3"}
	if rec.Research {
		args = append(args, "-net")
	}
	if err = saveTaskJSON(filepath.Join(snapshot, "team-admission-request.json"), map[string]any{"command": rec.Config.TeamCoordinator, "argv": append([]string{"team"}, args...), "reference": ref}); err != nil {
		return fail(125, err)
	}
	taskPhase(snapshot, "Submitting your team to its independent worker…")
	raw, code, err := taskTeamCommand(ctx, rec.Config, nil, args...)
	if err != nil {
		return fail(code, err)
	}
	status, err := decodeTaskTeam(raw, ref, "bench.team.status/v1")
	if err != nil {
		return fail(125, err)
	}
	result.GoalStatus = status.Status
	result.Message = status.Message
	result.Update = taskUpdate{Now: status.Message}
	// Exit 0 here acknowledges admission only. UI team projections read the
	// coordinator's goal state; they never infer completion from this process.
	return finishTask(snapshot, result, 0, status.Message)
}
func (a *app) projectTaskTeam(turn *taskTurn) {
	ref, err := readTaskTeamReference(turn.Job, turn.Record)
	if os.IsNotExist(err) {
		return
	}
	if err == nil {
		raw, _, e := taskTeamCommand(context.Background(), turn.Record.Config, nil, "status", ref.Run)
		err = e
		if err == nil {
			var s taskTeamStatus
			s, err = decodeTaskTeam(raw, ref, "bench.team.status/v1")
			if err == nil {
				turn.Coordinator = &s
			}
		}
	}
	if err != nil {
		turn.CoordinatorError = err.Error()
		turn.Result.GoalStatus = "unavailable"
		message := "The team’s status is unavailable: " + truncateMessage(err.Error(), 600) + ". Its work has not been restarted."
		if strings.HasPrefix(turn.Result.Message, "Team admission was not confirmed:") {
			message = turn.Result.Message + " The coordinator status is unavailable; this request has not been repeated."
		}
		turn.Result.Message = message
		turn.Result.Update = taskUpdate{Blocked: message}
		turn.Job.State = "running"
		turn.Progress = turn.Result.Message
		return
	}
	s := turn.Coordinator
	turn.Result.GoalStatus, turn.Result.Message, turn.Result.Question = s.Status, s.Message, s.Question
	turn.Result.Update = taskUpdate{Now: s.Phase, Next: fmt.Sprintf("%d of %d member attempts used.", s.AttemptsUsed, s.AttemptsLimit)}
	if s.Worker != nil {
		turn.Result.Update.Now = *s.Worker + " · " + s.Phase
	}
	turn.Progress = s.Message
	turn.Deferred = false
	turn.Result.Code = 2
	switch s.Status {
	case "ready", "running":
		turn.Job.State = "running"
		if s.Status == "ready" && !s.DriverActive {
			turn.Progress = "Waiting for the host worker. Next: " + turn.Result.Update.Now
			turn.Result.Update.Next = turn.Progress
		}
	case "needs_input":
		turn.Job.State = "unfinished"
		turn.Result.Code = 75
		turn.Result.Update.Blocked = s.Question
	case "cancelling":
		turn.Job.State = "cancelling"
	case "complete":
		turn.Job.State = "completed"
		turn.Result.Code = 0
		turn.Result.Update = taskUpdate{Done: s.Message}
		a.teamMu.Lock()
		err = a.importTaskTeam(turn.Job, turn.Record, ref, &turn.Result)
		a.teamMu.Unlock()
		if err != nil {
			turn.Result.GoalStatus = "delivery_pending"
			turn.Result.Code = 2
			turn.Result.Update.Blocked = "The accepted files could not be imported: " + truncateMessage(err.Error(), 400)
		}
	case "unknown":
		turn.Job.State = "unknown"
		turn.Result.Code = 125
		turn.Result.Update.Blocked = s.Message
	case "cancelled":
		turn.Job.State = "cancelled"
		turn.Result.Code = 130
		turn.Result.Update.Blocked = s.Message
	default:
		turn.Job.State = "unfinished"
		turn.Result.Update.Blocked = s.Message
	}
	observed := s.Execution.Exit
	if observed == nil {
		observed = s.LastExecution.Exit
	}
	if observed != nil && *observed != 0 && *observed != 2 && s.Status != "complete" {
		turn.Result.Code = *observed
	}
	code := turn.Result.Code
	turn.Job.ExitCode = &code
}
func (a *app) importTaskTeam(j Job, rec taskRecord, ref taskTeamReference, result *taskResult) error {
	root := filepath.Dir(j.Dir)
	// Import is idempotent, bound to the immutable accepted delivery. It never
	// executes returned files and never asks a model to re-decide acceptance.
	raw, _, err := taskTeamCommand(context.Background(), rec.Config, nil, "result", ref.Run)
	if err != nil {
		return err
	}
	s, err := decodeTaskTeam(raw, ref, "bench.team.result/v1")
	if err != nil {
		return err
	}
	if s.Status != "complete" || len(s.Files) == 0 || len(s.Files) > 32 {
		return fmt.Errorf("no accepted delivery")
	}
	tmp, err := os.MkdirTemp(root, ".team-delivery-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	artifacts := []taskArtifact{}
	seen := map[string]bool{}
	var total int64
	for _, f := range s.Files {
		kind := taskTypes[strings.ToLower(filepath.Ext(f.Name))]
		expected := filepath.Join(ref.Run, "delivery", "files", f.Name)
		if f.Kind != "deliverable" || !validRunFile(f.Name) || seen[f.Name] || kind == "" || f.Path != expected || f.Size < 0 || f.Size > taskFileLimit || !teamFingerprint.MatchString(f.SHA256) {
			return fmt.Errorf("invalid accepted file")
		}
		physical, err := filepath.EvalSymlinks(f.Path)
		if err != nil || physical != f.Path {
			return fmt.Errorf("accepted file path changed")
		}
		b, err := taskFile(filepath.Dir(f.Path), f.Name)
		if err != nil || int64(len(b)) != f.Size || digestText(b) != f.SHA256 {
			return fmt.Errorf("accepted file bytes changed")
		}
		total += f.Size
		if total > 100<<20 {
			return fmt.Errorf("delivery exceeds bounds")
		}
		seen[f.Name] = true
		if err = os.WriteFile(filepath.Join(tmp, f.Name), b, 0600); err != nil {
			return err
		}
		if f.Name != "delivery.json" {
			artifacts = append(artifacts, taskArtifact{Name: f.Name, Type: kind, Size: f.Size})
		}
	}
	dest := filepath.Join(root, "deliverables")
	if _, err = os.Stat(dest); os.IsNotExist(err) {
		if err = os.Rename(tmp, dest); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		entries, e := os.ReadDir(dest)
		if e != nil || len(entries) != len(s.Files) {
			return fmt.Errorf("previous delivery differs from accepted packet")
		}
		for _, f := range s.Files {
			b, e := taskFile(dest, f.Name)
			if e != nil || digestText(b) != f.SHA256 {
				return fmt.Errorf("previous delivery changed")
			}
		}
	}
	result.Artifacts = artifacts
	if rec.Config.Plonk != "" && a.deliveryState(rec.Thread).TaskID != j.ID {
		result.GoalStatus = "delivery_pending"
		result.Update.Blocked = "Files are finished and checked; private delivery is pending."
	}
	return saveTaskJSON(filepath.Join(root, "result.json"), result)
}
func (a *app) stopTaskTeam(w http.ResponseWriter, r *http.Request, j Job, rec taskRecord) bool {
	ref, err := readTaskTeamReference(j, rec)
	if os.IsNotExist(err) {
		return false
	}
	if err == nil {
		_, _, err = taskTeamCommand(r.Context(), rec.Config, nil, "cancel", ref.Run)
	}
	if err != nil {
		a.fail(w, 409, "I couldn’t confirm the team stop request: "+err.Error())
	} else {
		http.Redirect(w, r, "/work/"+rec.Thread, 303)
	}
	return true
}
func (a *app) messageTaskTeam(w http.ResponseWriter, r *http.Request, j Job, rec taskRecord) bool {
	ref, err := readTaskTeamReference(j, rec)
	if os.IsNotExist(err) {
		return false
	}
	fail := func(s string) {
		r.SetPathValue("thread", rec.Thread)
		a.renderTask(w, r, 409, s, r.PostForm.Get("message"))
	}
	if err != nil {
		fail(err.Error())
		return true
	}
	if !a.cfg.AllowBuild || !a.cfg.AllowRun {
		fail("Hire is not connected for work")
		return true
	}
	for _, other := range a.jobs.List() {
		if other.Kind == "task" && other.Started.After(j.Started) {
			r, e := a.taskRecord(other)
			if e == nil && r.Thread == rec.Thread {
				fail("Open the latest conversation before sending this update.")
				return true
			}
		}
	}
	message := strings.TrimSpace(r.PostForm.Get("message"))
	if message == "" && hasTaskUploads(r) {
		message = "Use these attached files for this work."
	}
	id := r.PostForm.Get("nonce")
	revision, e := strconv.Atoi(r.PostForm.Get("team-revision"))
	if e != nil || revision < 0 || !hexID.MatchString(id) || !boundedText(message, 6000, true) {
		fail("This update needs the current team revision. Refresh the conversation and send it again.")
		return true
	}
	var question *string
	if q := r.PostForm.Get("team-question"); q != "" {
		question = &q
	}
	a.mu.Lock()
	files, manifest, uploadErr := a.acceptTeamMessageUploads(rec.Thread, r)
	a.mu.Unlock()
	if uploadErr != nil {
		fail(uploadErr.Error())
		return true
	}
	payload := map[string]any{"id": id, "revision": revision, "question_id": question, "text": message}
	if manifest != "" {
		attachments := []map[string]string{}
		for _, f := range files {
			if filepath.Dir(f.Path) == filepath.Dir(manifest) {
				attachments = append(attachments, map[string]string{"path": f.Path, "name": f.Name})
			}
		}
		payload["attachments"] = attachments
	}
	raw, _ := json.Marshal(payload)
	_, _, err = taskTeamCommand(r.Context(), rec.Config, raw, "send", ref.Run)
	if err == nil {
		a.mu.Lock()
		err = appendTaskMessage(filepath.Dir(j.Dir), taskMessage{ID: id, Message: message, At: time.Now().UTC().Format(time.RFC3339Nano), Attachments: manifest})
		a.mu.Unlock()
	}
	if err != nil {
		fail("The team did not confirm this update: " + err.Error())
	} else {
		http.Redirect(w, r, "/work/"+rec.Thread, 303)
	}
	return true
}

// This observer only imports accepted bytes and completes already-authorized
// private delivery. It cannot dispatch/continue members or advance Manage.
func (a *app) observeTaskTeams(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	attempted := map[string]bool{}
	for {
		for _, j := range a.jobs.List() {
			if ctx.Err() != nil {
				return
			}
			if j.Kind != "task" || j.Active() {
				continue
			}
			rec, err := a.taskRecord(j)
			if err != nil || rec.TeamBackend != taskTeamBackend {
				continue
			}
			turn := taskTurn{Job: j, Record: rec, Result: a.taskResult(j)}
			a.projectTaskTeam(&turn)
			if turn.Coordinator == nil || turn.Coordinator.Status != "complete" || turn.Result.GoalStatus != "delivery_pending" || len(turn.Result.Artifacts) == 0 || attempted[j.ID] {
				continue
			}
			if rec.Config.Plonk == "" || rec.Config.PlonkURL == "" || rec.Config.PlonkTokenFile == "" {
				continue
			}
			started, err := a.queueTeamDelivery(j, rec, turn.Result)
			if started || err != nil {
				attempted[j.ID] = true
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *app) queueTeamDelivery(j Job, rec taskRecord, result taskResult) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, other := range a.jobs.List() {
		if other.Active() {
			return false, nil
		}
	}
	if a.deliveryState(rec.Thread).TaskID == j.ID {
		return true, nil
	}
	// A newer conversation result or an explicit delivery cancellation wins.
	for _, other := range a.jobs.List() {
		if other.Kind == "task" && other.Started.After(j.Started) {
			r, e := a.taskRecord(other)
			if e == nil && r.Thread == rec.Thread {
				return true, nil
			}
		}
		if other.Kind == "delivery" {
			var r deliveryRecord
			b, e := os.ReadFile(filepath.Join(filepath.Dir(other.Dir), "delivery-request.json"))
			if e == nil && json.Unmarshal(b, &r) == nil && r.Thread == rec.Thread {
				if r.TaskID == j.ID && other.State == "cancelled" {
					return true, nil
				}
				break
			}
		}
	}
	record, err := prepareGoalDeliveryRecord(filepath.Dir(j.Dir), rec, result, j.ID)
	if err != nil {
		return false, err
	}
	dir, err := a.workspace()
	if err != nil {
		return false, err
	}
	path := filepath.Join(filepath.Dir(dir), "delivery-request.json")
	if err = saveTaskJSON(path, record); err != nil {
		return false, err
	}
	self, err := os.Executable()
	if err != nil {
		return false, err
	}
	_, err = a.jobs.Start("delivery", "Open your results", dir, []string{self, "delivery", path}, "")
	return err == nil, err
}

// A repeated message ID selects the same uploaded bytes. The general upload
// store retains the original batch; this protocol additionally rejects a caller
// trying to replace it under the same idempotency key.
func (a *app) acceptTeamMessageUploads(thread string, r *http.Request) ([]taskAttachment, string, error) {
	manifest := filepath.Join(a.cfg.Data, "attachments", thread, r.PostForm.Get("nonce"), "files.json")
	if raw, e := readText(filepath.Dir(manifest), "files.json", 64<<10); e == nil {
		var saved []taskAttachment
		if json.Unmarshal([]byte(raw), &saved) != nil {
			return nil, "", fmt.Errorf("saved attachment batch is invalid")
		}
		if hasTaskUploads(r) {
			headers := r.MultipartForm.File["attachments"]
			if len(headers) != len(saved) {
				return nil, "", fmt.Errorf("this update ID already names different attachments")
			}
			for i, h := range headers {
				f, err := h.Open()
				if err != nil {
					return nil, "", err
				}
				b, err := io.ReadAll(io.LimitReader(f, taskUploadLimit+1))
				f.Close()
				if err != nil || h.Filename != saved[i].Name || int64(len(b)) != saved[i].Size || digestText(b) != saved[i].SHA256 {
					return nil, "", fmt.Errorf("this update ID already names different attachments")
				}
			}
		}
		return saved, manifest, nil
	} else if !os.IsNotExist(e) {
		return nil, "", e
	}
	return a.acceptTaskUploads(thread, r)
}

// A file lock coordinates the bounded preparation handoff with the separate UI
// process. It never spans a coordinator command or a worker invocation.
func lockTaskTeamAdmission(root string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(root, "team-admission.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		f.Close()
		return nil, fmt.Errorf("invalid team admission lock")
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	closed := false
	return func() {
		if !closed {
			closed = true
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
		}
	}, nil
}
