package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
)

// Limits are per invocation. Hire/Agent/Ply own the conversation checkpoint.
func buildArgs(hire, dir, goal, model string) []string {
	return []string{hire, "build", "-C", dir, "-evidence", filepath.Join(filepath.Dir(dir), "evidence"), "-goal-file", goal, "-m", model, "-turns", "50", "-timeout", "10m", "-checkpoint", "build"}
}

// Only known unfinished authoring can continue. Reconstruct an allowlisted
// command, never replay arbitrary stored argv or a team's staging script.
func (a *app) continuation(j Job) ([]string, bool, error) {
	if !a.cfg.AllowBuild || j.State != "unfinished" || j.ExitCode == nil || *j.ExitCode != 2 || (j.Kind != "build" && j.Kind != "revise" && j.Kind != "team-build") {
		return nil, false, fmt.Errorf("only an unfinished model build can continue")
	}
	for _, newer := range a.jobs.List() {
		if newer.Dir == j.Dir && newer.ID != j.ID && !newer.Started.Before(j.Started) {
			return nil, false, fmt.Errorf("this draft has newer activity; open its latest record")
		}
	}
	if !filepath.IsAbs(j.Dir) || !within(filepath.Join(a.cfg.Data, "workspaces"), j.Dir) {
		return nil, false, fmt.Errorf("build workspace is outside this interface")
	}
	goalName := "brief.txt"
	args := j.Args
	if j.Kind == "team-build" {
		goalName = "team-brief.txt"
		if len(args) == 2 && args[0] == "/bin/sh" && args[1] == filepath.Join(filepath.Dir(j.Dir), "assemble.sh") {
			// Published by the fixed controller script only after every export/copy
			// succeeded. An export's exit 2 must not be mistaken for unfinished Hire.
			rel, _ := filepath.Rel(a.cfg.Data, filepath.Join(filepath.Dir(j.Dir), "build-ready.json"))
			raw, err := readText(a.cfg.Data, rel, 16<<10)
			if err != nil || json.Unmarshal([]byte(raw), &args) != nil {
				return nil, false, fmt.Errorf("team staging did not finish; create a fresh draft")
			}
		}
	}
	goal := filepath.Join(filepath.Dir(j.Dir), goalName)
	if len(args) != 14 && len(args) != 16 {
		return nil, false, fmt.Errorf("build command cannot be continued safely")
	}
	if len(args[9]) > 200 || !modelName.MatchString(args[9]) {
		return nil, false, fmt.Errorf("build model is unavailable")
	}
	want := buildArgs(args[0], j.Dir, goal, args[9])
	checkpoint := len(args) == 16
	if !checkpoint {
		want = want[:14] // Older UI builds retain files but have no conversation pointer.
	}
	if args[11] == "8" {
		want[11] = "8"
	}
	if !slices.Equal(args, want) {
		return nil, false, fmt.Errorf("build command cannot be continued safely")
	}
	rel, _ := filepath.Rel(a.cfg.Data, goal)
	if _, err := readText(a.cfg.Data, rel, 256<<10); err != nil {
		return nil, false, fmt.Errorf("the original build brief is unavailable")
	}
	return buildArgs(a.cfg.Hire, j.Dir, goal, args[9]), checkpoint, nil
}

func (a *app) continueBuild(w http.ResponseWriter, r *http.Request) {
	a.admit(w, r, func() (Job, error) {
		j, ok := a.jobs.Get(r.PathValue("id"))
		if !ok {
			return Job{}, fmt.Errorf("that activity was not found")
		}
		if j.Kind == "run" {
			args, review, err := a.runContinuation(j)
			if err != nil {
				return Job{}, err
			}
			if review && r.PostForm.Get("recovery-consent") != "yes" {
				return Job{}, fmt.Errorf("review the run’s output and diagnostics, then confirm continuation; its recording is incomplete and effects may already exist")
			}
			return a.jobs.Start("run", j.Title, j.Dir, args, "")
		}
		args, _, err := a.continuation(j)
		if err != nil {
			return Job{}, err
		}
		return a.jobs.Start(j.Kind, j.Title, j.Dir, args, "")
	})
}
