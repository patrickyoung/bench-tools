package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type taskTeamPreflightFailure struct {
	Code   int
	Detail string
}

func (e taskTeamPreflightFailure) Error() string { return e.Detail }

type taskTeamInputsChanged struct{}

func (taskTeamInputsChanged) Error() string {
	return "supplied inputs or goal changed after preflight; rechecking before admission"
}

type taskTeamPreflightProof struct {
	Schema         string `json:"schema"`
	Definition     string `json:"definition_sha256"`
	Inputs         string `json:"inputs_sha256"`
	Goal           string `json:"goal_sha256"`
	DiagnosticRoot string `json:"diagnostic_root"`
}

func taskTeamInputsDigest(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) > 256 {
		return "", fmt.Errorf("team originals exceed 256 files")
	}
	records := []map[string]any{}
	total := 0
	for _, e := range entries {
		raw, err := taskFile(dir, e.Name())
		if err != nil {
			return "", err
		}
		total += len(raw)
		if total > 100<<20 {
			return "", fmt.Errorf("team originals exceed 100 MiB")
		}
		records = append(records, map[string]any{"name": e.Name(), "size": len(raw), "sha256": digestText(raw)})
	}
	raw, _ := json.Marshal(records)
	return digestText(raw), nil
}
func preflightTaskTeam(ctx context.Context, root, snapshot string, rec taskRecord, plan taskPlan) error {
	fail := func(code int, err error) error { return taskTeamPreflightFailure{code, err.Error()} }
	id := filepath.Base(root) + "-" + randomID()[:16]
	stage := filepath.Join(snapshot, "team-preflights", id)
	if err := os.MkdirAll(stage, 0700); err != nil {
		return fail(125, err)
	}
	unlock, err := lockTaskTeamAdmission(snapshot)
	if err != nil {
		return fail(125, err)
	}
	inputs, goal, _, err := stageTaskTeamInputs(root, snapshot, stage, filepath.Join(root, "work", "execution"), rec, plan)
	unlock()
	if err != nil {
		return fail(125, err)
	}
	inputHash, err := taskTeamInputsDigest(inputs)
	if err != nil {
		return fail(2, err)
	}
	goalRaw, err := os.ReadFile(goal)
	if err != nil {
		return fail(125, err)
	}
	expert := filepath.Join(root, "authoring", "expert")
	_, definitionHash, err := definitionSnapshot(rec.Config.Data, expert)
	if err != nil {
		return fail(125, err)
	}
	run := filepath.Join(rec.Config.TeamQueue, ".preflights", id)
	args := []string{"preflight", "-C", run, "-definition", expert, "-inputs", inputs, "-goal-file", goal, "-id", id, "-m", rec.Model, "-attempts", "48", "-turns", "50", "-timeout", "60", "-deadline", "120", "-corrections", "3"}
	if rec.Research {
		args = append(args, "-net")
	}
	if err = saveTaskJSON(filepath.Join(stage, "request.json"), map[string]any{"command": rec.Config.TeamCoordinator, "argv": append([]string{"team"}, args...)}); err != nil {
		return fail(125, err)
	}
	taskPhase(snapshot, "Checking the team’s real input handoff before starting specialists…")
	raw, code, commandErr := taskTeamCommand(ctx, rec.Config, nil, args...)
	if err = os.WriteFile(filepath.Join(stage, "response.json"), raw, 0600); err != nil {
		return fail(125, err)
	}
	var response struct {
		Schema     string `json:"schema"`
		Status     string `json:"status"`
		Coverage   string `json:"coverage"`
		Run        string `json:"run"`
		RequestID  string `json:"request_id"`
		Message    string `json:"message"`
		Diagnostic string `json:"diagnostic"`
		Repairable bool   `json:"repairable"`
		Execution  *struct {
			State  string `json:"state"`
			Exit   *int   `json:"exit"`
			Signal int    `json:"signal"`
		} `json:"execution"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Schema != "bench.team.preflight/v1" || response.Run != run || response.RequestID != id || response.Coverage != "first_prepare_only" {
		if commandErr != nil {
			return fail(taskOutcomeFailure(code), fmt.Errorf("preflight did not return a bound receipt: %w; diagnostics: %s", commandErr, run))
		}
		return fail(125, fmt.Errorf("preflight returned an invalid receipt; diagnostics: %s", run))
	}
	if code == 125 || response.Status == "unknown" {
		return fail(125, fmt.Errorf("preflight outcome is unknown: %s; inspect %s before any further attempt", response.Message, run))
	}
	if code != 0 && code != 2 {
		return fail(code, fmt.Errorf("preflight stopped (%d): %s; diagnostics: %s", code, response.Message, run))
	}
	if code == 2 && response.Status == "failed" {
		known := response.Execution != nil && response.Execution.Exit != nil && response.Execution.Signal == 0
		repairable := false
		if known {
			execution := response.Execution
			repairable = (execution.State == "failed" && (*execution.Exit == 1 || *execution.Exit == 2)) || (execution.State == "done" && *execution.Exit == 0)
		}
		if !response.Repairable || !repairable {
			stop := 125
			if response.Execution != nil && response.Execution.Exit != nil && *response.Execution.Exit != 0 && *response.Execution.Exit != 2 {
				stop = *response.Execution.Exit
			}
			return fail(stop, fmt.Errorf("preflight cannot safely be repaired automatically: %s; %s; diagnostics: %s", response.Message, response.Diagnostic, run))
		}
		return fail(2, fmt.Errorf("real first-prepare handoff failed: %s; inspect the actual assignment, originals manifest and Tend streams in %s (receipt %s)", response.Message+"; "+response.Diagnostic, run, filepath.Join(stage, "response.json")))
	}
	if commandErr != nil || code != 0 || response.Status != "passed" || response.Repairable || response.Execution == nil || response.Execution.State != "done" || response.Execution.Exit == nil || *response.Execution.Exit != 0 || response.Execution.Signal != 0 {
		return fail(125, fmt.Errorf("preflight receipt and command outcome disagree; diagnostics: %s", run))
	}
	proof := taskTeamPreflightProof{Schema: "hire.team.preflight/v1", Definition: definitionHash, Inputs: inputHash, Goal: digestText(goalRaw), DiagnosticRoot: run}
	if err = saveTaskJSON(filepath.Join(snapshot, "team-preflight.json"), proof); err != nil {
		return fail(125, err)
	}
	return nil
}
func verifyTaskTeamPreflight(snapshot, definition, inputs, goal string) error {
	raw, err := readText(snapshot, "team-preflight.json", 8192)
	if err != nil {
		return err
	}
	var proof taskTeamPreflightProof
	if strictJSON([]byte(raw), &proof) != nil || proof.Schema != "hire.team.preflight/v1" {
		return fmt.Errorf("team preparation receipt is invalid")
	}
	inputHash, err := taskTeamInputsDigest(inputs)
	if err != nil {
		return err
	}
	goalRaw, err := os.ReadFile(goal)
	if err != nil {
		return err
	}
	if proof.Definition != definition {
		return fmt.Errorf("the team definition changed after preflight; recheck preparation before admission")
	}
	if proof.Inputs != inputHash || proof.Goal != digestText(goalRaw) {
		return taskTeamInputsChanged{}
	}
	return nil
}
