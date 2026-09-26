package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"path/filepath"
)

// Local entries are a view over retained outcomes, not a second registry.
// A newer build or verification supersedes earlier evidence for that folder.
func (a *app) catalog() catalog {
	c := a.cat
	c.Workers = append([]entry(nil), a.cat.Workers...)
	c.Teams = append(a.localTeams(), a.cat.Teams...)
	jobs := a.jobs.List()
	authored := make(map[string]bool)
	for _, j := range jobs {
		if j.Kind == "build" || j.Kind == "new" || j.Kind == "revise" {
			authored[filepath.Join(j.Dir, "expert")] = true
		}
	}
	seen := make(map[string]bool)
	var local []entry
	for _, j := range jobs {
		if j.Kind != "build" && j.Kind != "verify" {
			continue
		}
		target := resultPath(j)
		if !authored[target] || seen[target] {
			continue
		}
		seen[target] = true
		if j.State != "completed" || j.ExitCode == nil || *j.ExitCode != 0 {
			continue
		}
		relative, err := filepath.Rel(a.cfg.Data, filepath.Join(target, "README.md"))
		if err != nil || !within(filepath.Join(a.cfg.Data, "workspaces"), target) {
			continue
		}
		readme, err := readText(a.cfg.Data, relative, 64<<10)
		if err != nil || readme == "" {
			continue
		}
		digest := sha256.Sum256([]byte(target))
		description := "Built on your bench. Evaluate on a real case before relying on it."
		if j.Kind == "verify" {
			description = "Structure verified. Evaluate on a real case before relying on it."
		}
		local = append(local, entry{ID: fmt.Sprintf("%x", digest[:16]), Title: j.Title, Description: description, Owner: "Your local library", Status: "experimental", Kind: "worker", Local: true, JobID: j.ID, Path: target})
	}
	// Conversationally prepared definitions are reusable private library entries.
	// Deduplicate unchanged copies retained by refinement turns.
	definitions := map[string]bool{}
	for _, j := range jobs {
		if j.Kind != "task" || j.Active() || j.State == "unknown" {
			continue
		}
		r := a.taskResult(j)
		if !r.Prepared || r.Expert == "" || r.Flash != nil {
			continue
		}
		_, hash, err := definitionSnapshot(a.cfg.Data, r.Expert)
		if err != nil || definitions[hash] {
			continue
		}
		rec, err := a.taskRecord(j)
		if err != nil {
			continue
		}
		definitions[hash] = true
		e := entry{ID: j.ID, Title: r.Title, Description: "Prepared for your work. Open the conversation to review or refine it.", Owner: "Your local library", Status: "experimental", Kind: r.Kind, Local: true, JobID: j.ID, Path: r.Expert, WorkThread: rec.Thread}
		if r.Kind == "team" {
			c.Teams = append(c.Teams, e)
		} else {
			local = append(local, e)
		}
	}
	c.Workers = append(local, c.Workers...)
	return c
}

func (a *app) localDetail(w http.ResponseWriter, r *http.Request) {
	c := a.catalog()
	for _, e := range c.Workers {
		if !e.Local || e.ID != r.PathValue("id") {
			continue
		}
		relative, err := filepath.Rel(a.cfg.Data, filepath.Join(e.Path, "README.md"))
		if err != nil {
			a.fail(w, 404, "This worker’s guide is unavailable.")
			return
		}
		readme, err := readText(a.cfg.Data, relative, 64<<10)
		if err != nil {
			a.fail(w, 404, "This worker’s guide is unavailable.")
			return
		}
		j, ok := a.jobs.Get(e.JobID)
		if !ok {
			a.fail(w, 404, "This worker’s activity is unavailable.")
			return
		}
		a.render(w, 200, page{Title: e.Name(), View: "local-worker", Nav: "workers", Catalog: c, Entry: e, Readme: readme, Job: j, Result: e.Path})
		return
	}
	a.fail(w, 404, "That local worker is not available. Inspect its latest activity.")
}
