package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func (d *definition) show() int {
	h := sha256.New()
	for _, name := range definitionNames {
		if b, ok := d.Files[name]; ok {
			fmt.Fprintf(h, "%s\x00%s\x00", name, b)
		}
	}
	body, goal := d.context(), string(d.Files["GOAL.md"])
	fmt.Printf("agent-home: %s\ndefinition-sha256: %x\ncompiled-sha256: %x\n", d.Home, h.Sum(nil), sha256.Sum256([]byte(body+"\n"+goal)))
	for _, name := range []string{"check", "wake"} {
		path := filepath.Join(d.Home, "bin", name)
		f, err := os.Open(path)
		if os.IsNotExist(err) && name == "wake" {
			continue
		}
		if err != nil {
			return problem(err, 1)
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return problem(err, 1)
		}
		fmt.Printf("%s-sha256: %x\n", name, h.Sum(nil))
	}
	for _, field := range [][2]string{
		{"workspace", d.Work}, {"state", d.State}, {"toolbox", filepath.Join(d.Home, "tools")},
		{"skills", filepath.Join(d.Home, "skills")}, {"check", filepath.Join(d.Home, "bin/check")},
		{"evidence", filepath.Join(d.Control, "runs")}, {"selection-evidence", filepath.Join(d.Control, "selections/find")},
		{"checkpoints", filepath.Join(d.Control, "checkpoints")}, {"learning-evidence", filepath.Join(d.Control, "learning")},
		{"learning-proposals", filepath.Join(d.Control, "learning/proposals")},
		{"amendment-evidence", filepath.Join(d.Control, "amendments")}, {"action-proposals", filepath.Join(d.Work, "actions")},
	} {
		fmt.Printf("%s: %s\n", field[0], field[1])
	}
	fmt.Print("default-authority: full host tool catalogue; Cage writes work+state; network denied\n\n# Definition files\n\n")
	for _, name := range definitionNames {
		if b, ok := d.Files[name]; ok {
			fmt.Printf("- `%s` (%d bytes)\n", filepath.Join(d.Home, name), len(b))
		}
	}
	fmt.Printf("\n%s\n# Goal input\n\n%s\n", body, goal)
	if meaningful(d.Files["PLAN.md"]) {
		fmt.Printf("\n# Standing plan (evidence)\n\n%s\n", d.Files["PLAN.md"])
	}
	if meaningful(d.Files["HEARTBEAT.md"]) {
		fmt.Printf("\n# Heartbeat input before wake evidence\n\n%s\n", d.Files["HEARTBEAT.md"])
	}
	if d.Skills != "" {
		brief, err := tool("AGENT_BRIEF", "brief")
		if err != nil {
			return problem(err, 1)
		}
		fmt.Print("\n# Local skill catalogue\n\n")
		return execute(brief, []string{"ls"}, withEnv(os.Environ(), map[string]string{"BRIEF_PATH": d.Skills}), nil, os.Stdout, os.Stderr, "")
	}
	return 0
}
