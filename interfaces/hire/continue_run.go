package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
)

func runCanRecover(j Job) bool {
	return j.Kind == "run" && j.ExitCode != nil &&
		((j.State == "unfinished" && *j.ExitCode == 2) || (j.State == "failed" && *j.ExitCode == 125))
}

// Resume only the UI's known Agent invocation. Neither the browser nor model
// output can inject a command, change network permission, or select a new worker.
func (a *app) runContinuation(j Job) ([]string, bool, error) {
	if !a.cfg.AllowRun || !runCanRecover(j) {
		return nil, false, fmt.Errorf("this run is not available for continuation")
	}
	for _, newer := range a.jobs.List() {
		if newer.Dir == j.Dir && newer.ID != j.ID && !newer.Started.Before(j.Started) {
			return nil, false, fmt.Errorf("this run has newer activity; open its latest record")
		}
	}
	if !filepath.IsAbs(j.Dir) || !within(filepath.Join(a.cfg.Data, "workspaces"), j.Dir) {
		return nil, false, fmt.Errorf("run workspace is outside this interface")
	}
	args := j.Args
	if len(args) < 19 || len(args) > 22 || len(args[9]) > 200 || !modelName.MatchString(args[9]) || !validRunFile(args[17]) {
		return nil, false, fmt.Errorf("run command cannot be continued safely")
	}
	turns, err := strconv.Atoi(args[11])
	if err != nil || turns < 1 || turns > 100 {
		return nil, false, fmt.Errorf("invalid original turn limit")
	}
	parent := filepath.Dir(j.Dir)
	goal := filepath.Join(parent, "task.txt")
	want := []string{args[0], "run", "-C", j.Dir, "-evidence", filepath.Join(parent, "evidence"), "-goal-file", goal, "-m", args[9], "-turns", args[11], "-timeout", "10m", "-checkpoint", "run", "-record-output", args[17]}
	i := 18
	if args[i] == "-record-input" {
		if i+2 >= len(args) || !validRunFile(args[i+1]) || args[i+1] == args[17] {
			return nil, false, fmt.Errorf("invalid recorded input")
		}
		want = append(want, "-record-input", args[i+1])
		i += 2
	}
	if args[i] == "-net" {
		want = append(want, "-net")
		i++
	}
	if i != len(args)-1 {
		return nil, false, fmt.Errorf("run command cannot be continued safely")
	}
	source := args[i]
	known := false
	for _, worker := range a.jobs.List() {
		if worker.State == "completed" && worker.ExitCode != nil && *worker.ExitCode == 0 &&
			(worker.Kind == "build" || worker.Kind == "verify" || (worker.Kind == "export" && len(worker.Args) > 2 && worker.Args[2] == "export")) && resultPath(worker) == source {
			known = true
			break
		}
	}
	if !known || !within(filepath.Join(a.cfg.Data, "workspaces"), source) {
		return nil, false, fmt.Errorf("the original worker is unavailable")
	}
	want = append(want, source)
	if !slices.Equal(args, want) {
		return nil, false, fmt.Errorf("run command cannot be continued safely")
	}
	for _, file := range []string{goal, filepath.Join(source, "README.md"), filepath.Join(source, "bin", "check")} {
		rel, err := filepath.Rel(a.cfg.Data, file)
		if err != nil {
			return nil, false, err
		}
		if _, err := readText(a.cfg.Data, rel, 256<<10); err != nil {
			return nil, false, fmt.Errorf("the saved task or original worker is unavailable")
		}
	}
	agent := a.cfg.Agent
	if agent == "" {
		agent = "agent"
	}
	agent, err = exec.LookPath(agent)
	if err != nil {
		return nil, false, fmt.Errorf("Agent is not installed or configured for this server")
	}
	want[0], err = filepath.Abs(agent)
	if err != nil {
		return nil, false, err
	}
	want[11] = "50"
	return want, *j.ExitCode == 125, nil
}
