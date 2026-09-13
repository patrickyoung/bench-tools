package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// The native executable also supplies Ply's -c interpreter seam. Cage owns
// confinement. This replaces Agent's shell helper on native run paths.
func actionShell(args []string) int {
	fail := func(err error) int { return problem(err, 125) }
	if len(args) != 2 || args[0] != "-c" {
		return fail(fmt.Errorf("action interpreter expects -c SCRIPT"))
	}
	work, err := realDir(os.Getenv("AGENT_WORK"))
	if err != nil {
		return fail(err)
	}
	state, err := realDir(os.Getenv("AGENT_STATE"))
	if err != nil {
		return fail(err)
	}
	tmp, err := realDir(os.Getenv("AGENT_ACTION_TMP"))
	if err != nil {
		return fail(err)
	}
	if inside(work, tmp) || inside(state, tmp) {
		return fail(fmt.Errorf("private action temporary directory must be outside work and state"))
	}
	cage := os.Getenv("AGENT_CAGE")
	if !filepath.IsAbs(cage) {
		return fail(fmt.Errorf("AGENT_CAGE must be an absolute pinned executable"))
	}
	if err := executable(cage); err != nil {
		return fail(err)
	}
	for _, path := range []string{work, state} {
		if err := hardlinks(path); err != nil {
			return fail(err)
		}
	}
	argv := []string{cage}
	if os.Getenv("AGENT_NET") == "1" {
		argv = append(argv, "-net")
	}
	argv = append(argv, "-w", work, "-w", state, "--", "/bin/sh", "-c", args[1])
	env := withEnv(os.Environ(), map[string]string{"TMPDIR": tmp})
	// Exec preserves the action's PID, process group, descriptors and signals.
	if err := syscall.Exec(cage, argv, env); err != nil {
		return fail(err)
	}
	return 125
}
