package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type caseResult struct {
	ID            string `json:"id"`
	Purpose       string `json:"purpose"`
	Status        string `json:"status"`
	ExpectedExit  int    `json:"expected_exit"`
	AgentExit     int    `json:"agent_exit"`
	EntryExit     *int   `json:"entry_exit,omitempty"`
	ExecutionWork string `json:"execution_work"`
	CheckExit     *int   `json:"check_exit,omitempty"`
	Work          string `json:"work"`
	Evidence      string `json:"evidence"`
}

func (r *run) evaluate() int {
	definition, err := readTree(r.candidate)
	if err != nil {
		return r.fail(err, 125)
	}
	if r.result.EvaluatedSHA256 == "" {
		r.result.EvaluatedSHA256 = treeDigest(definition)
	}
	if err := r.unchangedEvaluation(); err != nil {
		return r.fail(err, 125)
	}
	outputRoot := filepath.Join(r.config.output, "cases")
	if err := os.Mkdir(outputRoot, 0700); err != nil {
		return r.fail(err, 1)
	}
	overall := 0
	for _, c := range r.packet.Cases {
		caseRoot := filepath.Join(r.root, "cases", c.ID)
		if err := os.Mkdir(caseRoot, 0700); err != nil {
			return r.fail(err, 1)
		}
		work := filepath.Join(caseRoot, "work")
		if err := materialize(work, subtree(r.evaluationFiles, "cases/"+c.ID+"/input"), false); err != nil {
			return r.fail(err, 1)
		}
		admitted := filepath.Join(r.evaluationRoot, "cases", c.ID)
		evidence := filepath.Join(caseRoot, "evidence")
		args := []string{"run", "-B", "-C", work, "-state", filepath.Join(caseRoot, "state"), "-evidence", evidence, "-goal-file", filepath.Join(admitted, "goal.md")}
		args = append(args, r.agentFlags()...)
		for _, name := range sortedFiles(subtree(r.evaluationFiles, "cases/"+c.ID+"/input")) {
			args = append(args, "-record-input", name)
		}
		for _, name := range c.Outputs {
			args = append(args, "-record-output", name)
		}
		args = append(args, r.candidate)
		var code int
		verdict := caseResult{ID: c.ID, Purpose: c.Purpose, Status: "incomplete", ExpectedExit: c.ExpectedExit, ExecutionWork: work, Evidence: evidence}
		if r.execution.Entry == "" {
			code = r.invoke("case-"+c.ID+"-agent", "agent", args, work, nil, nil)
			verdict.AgentExit = code
		} else {
			code = r.runEntry(c, caseRoot, work, admitted, evidence)
			actual := code
			verdict.EntryExit = &actual
		}
		actualExit := code
		executionStatus := filepath.Join(caseRoot, "execution.json")
		if err := atomicFile(executionStatus, []byte(fmt.Sprintf("{\"exit\":%d,\"expected_exit\":%d}\n", actualExit, c.ExpectedExit))); err != nil {
			return r.fail(err, 125)
		}

		if err := r.unchangedEvaluation(); err != nil {
			verdict.Status = "definition_or_case_changed"
			r.result.Cases = append(r.result.Cases, verdict)
			return r.fail(err, 125)
		}
		// Check and publish one bounded copy, rather than rereading live work.
		// Only a confined entry is excluded from the snapshot's write authority.
		files, err := readTree(work)
		if err != nil {
			return r.fail(fmt.Errorf("case %s artifacts: %w", c.ID, err), 125)
		}
		snapshot := filepath.Join(caseRoot, "snapshot")
		if err := materialize(snapshot, files, true); err != nil {
			return r.fail(err, 1)
		}
		verdict.Work = snapshot
		if actualExit == c.ExpectedExit {
			code = 0 // Only the separate case check can accept an expected refusal.
			// Cage deliberately grants TMPDIR writes. Never inherit a broad
			// temporary root that may also contain inputs or controller evidence.
			checkTemp := filepath.Join(caseRoot, "check-temp")
			if err := os.Mkdir(checkTemp, 0700); err != nil {
				return r.fail(err, 1)
			}
			// Caller-selected checks stay outside the mutable workspace and run
			// under a separate read-only, network-denied Cage through Record.
			checkArgs := []string{"run", "-ask", r.programs["ask"], "-f", filepath.Join(caseRoot, "check.jsonl"), "-input", executionStatus}
			for _, name := range sortedFiles(subtree(r.evaluationFiles, "cases/"+c.ID)) {
				checkArgs = append(checkArgs, "-input", filepath.Join(admitted, filepath.FromSlash(name)))
			}
			for _, name := range c.Outputs {
				checkArgs = append(checkArgs, "-input", filepath.Join(snapshot, filepath.FromSlash(name)))
			}
			checkArgs = append(checkArgs, "--", r.programs["cage"], "-ro", "--", filepath.Join(admitted, "check"))
			check := r.invoke("case-"+c.ID+"-check", "record", checkArgs, snapshot, nil, map[string]string{"TRAIT_CASE_DIR": admitted, "TMPDIR": checkTemp, "TRAIT_EXECUTION_EXIT": fmt.Sprint(actualExit), "TRAIT_EXECUTION_STATUS": executionStatus, "TRAIT_EXECUTION_STATE": filepath.Join(caseRoot, "state"), "TRAIT_EXECUTION_EVIDENCE": evidence})
			verdict.CheckExit = &check
			if err := r.unchangedEvaluation(); err != nil {
				verdict.Status = "definition_or_case_changed"
				r.result.Cases = append(r.result.Cases, verdict)
				return r.fail(err, 125)
			}
			switch check {
			case 0:
				verdict.Status = "accepted"
			case 1:
				verdict.Status = "rejected"
				overall = 2
			default:
				verdict.Status = "broken"
				code = check
			}
		} else if actualExit == 0 {
			verdict.Status = "unexpected_success"
			code = 2
		}
		r.result.Cases = append(r.result.Cases, verdict)
		destination := filepath.Join(outputRoot, c.ID)
		if err := os.Mkdir(destination, 0700); err != nil {
			return r.fail(err, 1)
		}
		if err := materialize(filepath.Join(destination, "work"), files, false); err != nil {
			return r.fail(err, 1)
		}
		if code != 0 {
			return code
		}
	}
	if overall != 0 {
		r.result.Status = "rejected_on_cases"
	}
	return overall
}

// Verifiers retain their selected host contract. Detect a changed definition or
// case instead of crediting results to a different published worker identity.
func (r *run) unchangedEvaluation() error {
	for _, item := range []struct{ path, hash string }{{r.candidate, r.result.EvaluatedSHA256}, {r.source, r.sourceSHA256}, {r.evaluationRoot, r.evaluationSHA256}} {
		if item.path == "" {
			continue
		}
		files, err := readTree(item.path)
		if err != nil {
			return err
		}
		if treeDigest(files) != item.hash {
			return fmt.Errorf("evaluation definition or admitted cases changed: %s", item.path)
		}
	}
	return nil
}

func (r *run) runEntry(c testCase, caseRoot, work, admitted, evidence string) int {
	state, tmp := filepath.Join(caseRoot, "state"), filepath.Join(caseRoot, "entry-temp")
	for _, dir := range []string{state, evidence, tmp} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return r.fail(err, 1)
		}
	}
	goal := filepath.Join(admitted, "goal.md")
	values := map[string]string{"{goal}": goal, "{work}": work, "{state}": state, "{evidence}": evidence, "{expert}": r.candidate}
	args := []string{"run", "-ask", r.programs["ask"], "-f", filepath.Join(caseRoot, "entry.jsonl"), "-input", goal, "-input", filepath.Join(r.evaluationRoot, "run.json")}
	definition, err := readTree(r.candidate)
	if err != nil {
		return r.fail(err, 125)
	}
	for _, name := range sortedFiles(definition) {
		args = append(args, "-input", filepath.Join(r.candidate, filepath.FromSlash(name)))
	}
	for _, name := range sortedFiles(subtree(r.evaluationFiles, "cases/"+c.ID+"/input")) {
		args = append(args, "-input", filepath.Join(work, filepath.FromSlash(name)))
	}
	for _, name := range c.Outputs {
		args = append(args, "-output", filepath.Join(work, filepath.FromSlash(name)))
	}
	args = append(args, "--")
	if r.config.entryBoundary == "cage" {
		args = append(args, r.programs["cage"], "-net", "-w", work, "-w", state, "-w", evidence, "--")
	}
	args = append(args, filepath.Join(r.candidate, filepath.FromSlash(r.execution.Entry)))
	for _, arg := range r.execution.Args {
		if value, ok := values[arg]; ok {
			args = append(args, value)
		} else {
			args = append(args, arg)
		}
	}
	env := map[string]string{"TMPDIR": tmp, "TRAIT_AGENT": r.programs["agent"], "TRAIT_EXPERT": r.candidate, "TRAIT_WORK": work, "TRAIT_STATE": state, "TRAIT_EVIDENCE": evidence, "TRAIT_GOAL": goal, "TRAIT_TURNS": fmt.Sprint(r.config.turns), "TRAIT_TIMEOUT": r.config.timeout.String()}
	if r.config.model != "" {
		env["ASK_MODEL"] = r.config.model
	}
	return r.invoke("case-"+c.ID+"-entry", "record", args, work, r.evaluationFiles["cases/"+c.ID+"/goal.md"].Data, env)
}
