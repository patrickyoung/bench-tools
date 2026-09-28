package main

import "os/exec"

// This is an operator-selected process boundary, never a plan-selected command.
// The runner must preserve selected absolute paths, Cage write/network semantics,
// streams, signals and exit statuses, and supply native public member tools.
func taskTeamRunner(cfg config) (string, error) {
	if cfg.TeamRunner != "" {
		return exec.LookPath(cfg.TeamRunner)
	}
	return exec.LookPath("cage")
}
