package main

import (
	"os"
	"path/filepath"
)

func checkReadOnlyPaths(report *checkReport, exe, work string, env []string) {
	parent := filepath.Join(work, "protected-parent")
	inputs := filepath.Join(parent, "inputs")
	file := filepath.Join(inputs, "original")
	if err := os.MkdirAll(inputs, 0700); err != nil {
		report.record("read-only input setup", false, err.Error())
		return
	}
	if err := os.WriteFile(file, []byte("unchanged"), 0600); err != nil {
		report.record("read-only input setup", false, err.Error())
		return
	}
	base := []string{"-w", work, "-r", inputs, "--", "/bin/sh", "-c"}
	r := runSelf(exe, work, env, "", append(base,
		`cat protected-parent/inputs/original; printf writable > protected-parent/sibling`)...)
	body, _ := os.ReadFile(filepath.Join(parent, "sibling"))
	report.record("protected inputs readable and siblings writable", r.outcome.code == 0 && r.stdout == "unchanged" && string(body) == "writable", externalDetail(r))
	for _, probe := range []struct{ name, script string }{
		{"protected input overwrite denied", `printf changed > protected-parent/inputs/original`},
		{"protected input deletion denied", `rm protected-parent/inputs/original`},
		{"protected directory rename denied", `mv protected-parent/inputs protected-parent/moved`},
		{"protected ancestor rename denied", `mv protected-parent moved-parent`},
		{"protected input hardlink denied", `ln protected-parent/inputs/original hardlink`},
		{"protected input mode change denied", `chmod 000 protected-parent/inputs/original`},
		{"protected directory creation denied", `touch protected-parent/inputs/new`},
	} {
		r = runSelf(exe, work, env, "", append(base, probe.script)...)
		body, err := os.ReadFile(file)
		report.record(probe.name, childRan(r) && r.outcome.code != 0 && err == nil && string(body) == "unchanged", externalDetail(r))
	}
	// File overlays need the same deletion protection as directory overlays.
	r = runSelf(exe, work, env, "", "-w", work, "-r", file, "--", "/bin/sh", "-c", `rm protected-parent/inputs/original`)
	body, err := os.ReadFile(file)
	report.record("protected single file deletion denied", childRan(r) && r.outcome.code != 0 && err == nil && string(body) == "unchanged", externalDetail(r))
	r = runSelf(exe, work, env, "", "-w", work, "-r", filepath.Join(work, "missing"), "--", "/bin/sh", "-c", "touch must-not-start")
	_, err = os.Stat(filepath.Join(work, "must-not-start"))
	report.record("invalid protected input refuses child", r.outcome.code == exitUsage && os.IsNotExist(err), externalDetail(r))
}
