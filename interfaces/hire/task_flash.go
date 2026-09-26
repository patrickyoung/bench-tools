package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// A flash team is durable within its conversation, but never a library entry.
// Names are reserved by the separately selected public command, not the model.
type taskFlashTeam struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Theme string `json:"theme"`
}

var flashSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func prepareFlashTeam(ctx context.Context, root string, rec taskRecord, plan taskPlan) (*taskFlashTeam, error) {
	if plan.Target != "new:team" {
		for _, c := range rec.Catalog {
			if c.Key == plan.Target {
				return c.Flash, nil
			}
		}
		return nil, nil
	}
	read := func(raw []byte) (*taskFlashTeam, error) {
		var name taskFlashTeam
		if strictJSON(raw, &name) != nil || !hexID.MatchString(name.ID) || !boundedText(name.Name, 120, true) || len(name.Slug) > 160 || !flashSlug.MatchString(name.Slug) || name.Theme != "playful" {
			return nil, fmt.Errorf("name generator returned an invalid reservation")
		}
		return &name, nil
	}
	path := filepath.Join(root, "flash-team.json")
	if raw, err := os.ReadFile(path); err == nil {
		return read(raw)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if rec.Config.Moniker == "" {
		return nil, fmt.Errorf("select the public Moniker command for flash teams")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	code, raw, _ := taskCommand(ctx, root, nil, rec.Config.Moniker, "-dir", filepath.Join(rec.Config.Data, "team-names"), "-json")
	if code != 0 {
		return nil, taskSelectionExit{code}
	}
	name, err := read([]byte(raw))
	if err != nil {
		return nil, err
	}
	if err := saveTaskJSON(path, name); err != nil {
		return nil, err
	}
	return name, nil
}

// Temporary teams remain discoverable only inside the conversation that made
// them, including after a follow-up briefly selected an individual worker.
func withConversationFlashTeams(choices []taskChoice, turns []taskTurn) []taskChoice {
	out := append([]taskChoice{}, choices...)
	latest := map[string]taskChoice{}
	for _, turn := range turns {
		r := turn.Result
		if r.Flash == nil || !r.Prepared || r.Expert == "" || turn.Job.Active() || turn.Job.State == "unknown" {
			continue
		}
		latest[r.Flash.ID] = taskChoice{Key: "flash:" + r.Flash.ID, Name: r.Flash.Name, Kind: "team", Description: "Temporary team already assembled for this conversation. Reuse its specialists and handoffs when they still fit.", Path: r.Expert, Local: true, Flash: r.Flash}
	}
	keys := []string{}
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		candidate := latest[key]
		found := false
		for _, c := range out {
			if c.Path == candidate.Path {
				found = true
				break
			}
		}
		if !found {
			out = append(out, candidate)
		}
	}
	return out
}
