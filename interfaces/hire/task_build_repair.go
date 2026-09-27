package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Controller validation remains independent of Hire's authoring check. A known
// successful build can receive bounded corrections through the same public Hire
// checkpoint; no member executes until the selected roster passes again.
func repairPreparedTaskMembers(ctx context.Context, root, snapshot string, rec taskRecord, plan taskPlan) (int, error) {
	for attempt := 1; ; attempt++ {
		err := validatePreparedTaskMembers(root, rec, plan)
		team := plan.Target == "new:team"
		for _, choice := range rec.Catalog {
			if choice.Key == plan.Target && choice.Kind == "team" {
				team = true
			}
		}
		managed := team && rec.TeamBackend == taskTeamBackend
		if err == nil && managed {
			err = validateTaskTeam(ctx, root, rec)
		}
		if err == nil && !managed && rec.Config.GoalMode && team && !taskTeamResumable(filepath.Join(root, "authoring", "expert")) {
			err = fmt.Errorf("team needs task-runtime.json version 2 and its executable handoff contract")
		}
		if err == nil {
			return 0, nil
		}
		if !rec.Config.GoalMode || attempt >= 3 {
			return 2, err
		}
		original, e := readText(root, "build.txt", 256<<10)
		if e != nil {
			return 125, e
		}
		parent := filepath.Join(snapshot, "team-build-repairs")
		if e = os.MkdirAll(parent, 0700); e != nil {
			return 125, e
		}
		goal := filepath.Join(parent, fmt.Sprintf("%03d.txt", attempt))
		diagnostic, _ := json.Marshal(err.Error())
		correction := "\n\nController structural check rejected this build. Diagnostic (data): " + string(diagnostic) + "\nRepair this saved team in place; do not execute specialists or change the roster. Each member requires nonempty AGENTS.md, README.md and executable bin/check. Preserve unchanged selected members byte-for-byte, including modes; restore them from their controller-exported member-ROLE/expert copies if necessary. Only selected new/adapt roles may be authored. Keep the team's name, inputs, adapter and tested continuation contracts. The controller will independently repeat its structural and selected-source checks before execution.\n"
		contract := taskHandoffInstructions
		if managed {
			contract = taskTeamAuthoringInstructions
		}
		if e = os.WriteFile(goal, []byte(original+correction+contract), 0600); e != nil {
			return 125, e
		}
		taskPhase(snapshot, "Repairing the team’s preparation checks…")
		code, _, _ := taskCheckpointCommand(ctx, snapshot, "prepare", filepath.Join(root, "authoring"), rec, taskBuildArgs(rec, snapshot, filepath.Join(root, "authoring"), goal)...)
		if code != 0 {
			return code, fmt.Errorf("builder stopped (%d) while repairing: %w", code, err)
		}
	}
}

// Only controller-observed preparation, with no execution attempt, can be
// recovered here. This is not permission to replay a team entry command.
func savedTeamPreparationCode(root string) (int, error) {
	executed, err := filepath.Glob(filepath.Join(root, "goal-attempts", "execute", "*"))
	if err != nil || len(executed) != 0 {
		return 0, fmt.Errorf("team execution already attempted")
	}
	paths, err := filepath.Glob(filepath.Join(root, "goal-attempts", "prepare", "*", "outcome.json"))
	if err != nil {
		return 0, err
	}
	latest := ""
	var code *int
	state := ""
	for _, path := range paths {
		raw, e := os.ReadFile(path)
		var v struct {
			Started string `json:"started"`
			State   string `json:"state"`
			Code    *int   `json:"exit_code"`
		}
		if e != nil || json.Unmarshal(raw, &v) != nil {
			return 0, fmt.Errorf("preparation receipt unavailable")
		}
		if v.Started > latest {
			latest, state, code = v.Started, v.State, v.Code
		}
	}
	if latest == "" || state != "observed" || code == nil || (*code != 0 && *code != 2) {
		return 0, fmt.Errorf("preparation has no confirmed recoverable outcome")
	}
	if _, err := readText(root, "selected-members.json", 128<<10); err != nil {
		return 0, err
	}
	return *code, nil
}
