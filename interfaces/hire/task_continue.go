package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Controller-owned inputs for one explicitly requested continuation. No argv or
// permissions are accepted from the browser, model output or execution workspace.
type taskResume struct {
	Root, Stage string
	Plan        taskPlan
	Result      taskResult
}

func finishTask(root string, result taskResult, code int, message string) int {
	result.Code = code
	if message != "" {
		result.Message = message
	}
	if err := saveTaskJSON(filepath.Join(root, "result.json"), result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return code
}

func (a *app) taskResumeInfo(j Job, rec taskRecord) (*taskResume, error) {
	if j.Kind != "task" || j.Active() || j.ExitCode == nil || !((*j.ExitCode == 2 && j.State == "unfinished") || (*j.ExitCode == 130 && (j.State == "cancelled" || j.State == "failed"))) {
		return nil, fmt.Errorf("this attempt does not have a confirmed resumable outcome")
	}
	root := filepath.Dir(j.Dir)
	if rec.Resume != nil {
		root = rec.Resume.Root
	}
	if !filepath.IsAbs(root) || !within(filepath.Join(a.cfg.Data, "workspaces"), root) {
		return nil, fmt.Errorf("saved workspace is unavailable")
	}
	read := func(name string, limit int) (string, error) {
		rel, e := filepath.Rel(a.cfg.Data, filepath.Join(root, name))
		if e != nil {
			return "", e
		}
		return readText(a.cfg.Data, rel, int64(limit))
	}
	saved, e := read("task.json", 2<<20)
	if e != nil {
		return nil, e
	}
	var original taskRecord
	if json.Unmarshal([]byte(saved), &original) != nil || original.Thread != rec.Thread || original.Resume != nil {
		return nil, fmt.Errorf("saved task does not match this conversation")
	}
	request, e := read("plan/request.json", 2<<20)
	if e != nil {
		return nil, e
	}
	response, e := read("plan/response.json", 128<<10)
	if e != nil {
		return nil, e
	}
	plan, e := decodeTaskPlan([]byte(response), []byte(request), original.Catalog)
	if e != nil || plan.Question != "" {
		return nil, fmt.Errorf("saved plan is unavailable")
	}
	result := a.taskResult(j)
	resume := &taskResume{Root: root, Plan: plan, Result: result, Stage: "build"}
	if result.Prepared {
		if result.Kind != "worker" || result.Expert != filepath.Join(root, "authoring", "expert") {
			return nil, fmt.Errorf("this specialist uses a separate recovery contract")
		}
		if _, e = read("execution.txt", 256<<10); e != nil {
			return nil, e
		}
		if _, _, e = definitionSnapshot(a.cfg.Data, filepath.Join(root, "runner")); e != nil {
			return nil, e
		}
		resume.Stage = "execute"
	} else {
		if _, e = read("build.txt", 256<<10); e != nil {
			return nil, e
		}
		// A team adapter has its own runtime entry contract; do not replay it here.
		if result.Kind == "team" || plan.Target == "new:team" {
			return nil, fmt.Errorf("team preparation needs a fresh attempt")
		}
		for _, c := range original.Catalog {
			if c.Key == plan.Target && c.Kind == "team" {
				return nil, fmt.Errorf("team preparation needs a fresh attempt")
			}
		}
	}
	for _, in := range plan.Inputs {
		if _, e = read(filepath.Join("inputs", in.Name), 128<<10); e != nil {
			return nil, e
		}
	}
	return resume, nil
}

func (a *app) taskContinue(w http.ResponseWriter, r *http.Request) {
	previous, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || previous.Kind != "task" {
		http.NotFound(w, r)
		return
	}
	rec, err := a.taskRecord(previous)
	if err != nil {
		a.fail(w, 409, "That saved work is unavailable.")
		return
	}
	out := &assistantResponse{header: make(http.Header)}
	a.admit(out, r, func() (Job, error) {
		if !a.cfg.AllowBuild || !a.cfg.AllowRun {
			return Job{}, fmt.Errorf("Hire is not connected for work")
		}
		turns := a.taskTurns(rec.Thread)
		if len(turns) == 0 || turns[len(turns)-1].Job.ID != previous.ID {
			return Job{}, fmt.Errorf("open the latest attempt before continuing")
		}
		for _, job := range a.jobs.List() {
			if job.Active() {
				return Job{}, fmt.Errorf("let the current work finish first")
			}
		}
		resume, err := a.taskResumeInfo(previous, rec)
		if err != nil {
			return Job{}, err
		}
		dir, err := a.workspace()
		if err != nil {
			return Job{}, err
		}
		root := filepath.Dir(dir)
		files, _, err := definitionSnapshot(a.cfg.Data, filepath.Join(resume.Root, "companion"))
		if err != nil {
			return Job{}, err
		}
		if err = writeDefinition(filepath.Join(root, "companion"), files); err != nil {
			return Job{}, err
		}
		for _, name := range []string{"AGENT.md", "HIRE.md", "ASK.md", "PLY.md", "TRAIL.md", "BENCH.md", "knowledge.md"} {
			rel, _ := filepath.Rel(a.cfg.Data, filepath.Join(resume.Root, name))
			if text, e := readText(a.cfg.Data, rel, 256<<10); e == nil {
				if e = os.WriteFile(filepath.Join(root, name), []byte(text), 0600); e != nil {
					return Job{}, e
				}
			}
		}
		rec.History = append(rec.History, assistantMessage{Role: "user", Text: rec.Message}, assistantMessage{Role: "assistant", Text: resume.Result.Message})
		if len(rec.History) > 24 {
			rec.History = rec.History[len(rec.History)-24:]
		}
		if notes, err := readTaskMessages(filepath.Dir(previous.Dir)); err == nil {
			for _, note := range notes {
				rec.History = append(rec.History, assistantMessage{Role: "user", Text: note.Message})
			}
		}
		rec.Attachments, err = taskAttachments(a.cfg.Data, rec.Thread)
		if err != nil {
			return Job{}, err
		}
		rec.Steering = true
		if err := initTaskMessages(root); err != nil {
			return Job{}, err
		}
		// Retain the user's conversation refinements across a saved checkpoint.
		// They remain guidance, never permission to repeat uncertain effects.
		if notes, err := readTaskMessages(filepath.Dir(previous.Dir)); err == nil {
			for _, note := range notes {
				if err := appendTaskMessage(root, note); err != nil {
					return Job{}, err
				}
			}
		}
		rec.Message = "Continue working"
		rec.Now = time.Now().Format(time.RFC3339)
		rec.Config = a.cfg
		rec.Previous = filepath.Dir(previous.Dir)
		rec.Resume = resume
		path := filepath.Join(root, "task.json")
		if err = saveTaskJSON(path, rec); err != nil {
			return Job{}, err
		}
		self, err := os.Executable()
		if err != nil {
			return Job{}, err
		}
		return a.jobs.Start("task", "Continue · "+resume.Result.Title, dir, []string{self, "task", path}, "")
	})
	if out.status != 303 {
		r.SetPathValue("thread", rec.Thread)
		a.renderTask(w, r, out.status, "I couldn’t continue this attempt yet. Your saved work is still here. Open the latest attempt or try a new message.", "")
		return
	}
	http.Redirect(w, r, "/work/"+rec.Thread, 303)
}

func resumeTaskProcess(ctx context.Context, snapshot string, rec taskRecord) int {
	saved := rec.Resume
	result := saved.Result
	result.Message, result.Question = "", ""
	result.Artifacts = nil
	result.Update = taskUpdate{}
	if saved.Stage == "build" {
		taskPhase(snapshot, "Continuing preparation from where we left off…")
		author := filepath.Join(saved.Root, "authoring")
		code, _, _ := taskCommand(ctx, author, nil, taskBuildArgs(rec, snapshot, author, filepath.Join(saved.Root, "build.txt"))...)
		if code != 0 {
			return finishTask(snapshot, result, code, "The specialist needs more work. What’s saved is safe; you can continue again.")
		}
		result.Expert, result.Kind, result.Prepared = filepath.Join(author, "expert"), "worker", true
	} else if saved.Stage != "execute" {
		return finishTask(snapshot, result, 1, "The saved continuation is unavailable.")
	}
	return executeTask(ctx, saved.Root, snapshot, rec, saved.Plan, result, saved.Stage == "execute")
}
