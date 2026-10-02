package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const fixtureArgvEntry = `#!/bin/sh
set -eu
test "${BUNDLE_WORKSPACE+x}" != x
test "${BUNDLE_INPUT+x}" != x
test "${BUNDLE_OUTPUT+x}" != x
test "${BUNDLE_WORK+x}" != x
test "${BUNDLE_STATE+x}" != x
test "${BUNDLE_CONTROL+x}" != x
test "${BUNDLE_GOAL_FILE+x}" != x
helper >&2
printf '%s\000' "$(pwd -P)" "$BUNDLE_ROOT" "$BUNDLE_ID" "$#"
for arg do printf '%s\000' "$arg"; done
cat
exit "${FIXTURE_EXIT:-0}"
`

func TestArgvApplicationOwnsInterface(t *testing.T) {
	root, env := fixture(t)
	for _, key := range []string{"BUNDLE_ROOT", "BUNDLE_ID", "BUNDLE_WORK", "BUNDLE_WORKSPACE", "BUNDLE_INPUT", "BUNDLE_OUTPUT", "BUNDLE_STATE", "BUNDLE_CONTROL", "BUNDLE_GOAL_FILE"} {
		env = append(env, key+"=inherited-stale-value")
	}
	input := []byte("literal stdin\x00\xff\n")
	arguments := []string{"create", "input directory", "output directory", "--help", "-o", "", "$(touch BAD); `touch BAD2`", "line one\nline two", "--bundle-info"}
	for _, args := range [][]string{arguments, {"--help"}, nil} {
		code, out, diagnostics := runExecutable(t, testArgvApp, root, env, input, args...)
		if code != 0 || string(diagnostics) != "bundled helper\n" {
			t.Fatalf("argv launch: exit %d, diagnostics %s", code, diagnostics)
		}
		fields := bytes.SplitN(out, []byte{0}, 5+len(args))
		if len(fields) != 5+len(args) || string(fields[0]) != root || string(fields[3]) != strconv.Itoa(len(args)) {
			t.Fatalf("changed cwd, argument count or boundaries: %q", out)
		}
		definition, identity := string(fields[1]), string(fields[2])
		if !filepath.IsAbs(definition) || len(identity) != 64 || !strings.Contains(definition, identity) {
			t.Fatalf("missing selected package identity and definition: %q", fields[:3])
		}
		for i, arg := range args {
			if string(fields[4+i]) != arg {
				t.Fatalf("argument %d changed: %q, want %q", i, fields[4+i], arg)
			}
		}
		if !bytes.Equal(fields[len(fields)-1], input) {
			t.Fatalf("stdin changed: %q", fields[len(fields)-1])
		}
		if _, err := os.Stat(filepath.Join(root, "state", "bundle", identity, "runs")); !os.IsNotExist(err) {
			t.Fatal("argv launcher created goal-mode run state")
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "state" {
		t.Fatalf("argv launcher created workspaces or evaluated an argument: %v, %v", entries, err)
	}
	for _, expected := range []int{1, 2, 3, 75, 125, 130} {
		code, out, diagnostics := runExecutable(t, testArgvApp, root, append(env, fmt.Sprintf("FIXTURE_EXIT=%d", expected)), input, "evaluate")
		if code != expected || !bytes.HasSuffix(out, input) || string(diagnostics) != "bundled helper\n" {
			t.Fatalf("argv status/streams changed: %d (want %d), %q, %q", code, expected, out, diagnostics)
		}
	}
}

func TestArgvMetadataDoesNotRunEntry(t *testing.T) {
	root, env := fixture(t)
	code, out, diagnostics := runExecutable(t, testArgvApp, root, env, nil, "--bundle-info")
	var m manifest
	if code != 0 || len(diagnostics) != 0 || json.Unmarshal(out, &m) != nil || m.App.Interface != "argv" {
		t.Fatalf("metadata: exit %d, %s, %s", code, out, diagnostics)
	}
	code, out, diagnostics = runExecutable(t, testArgvApp, root, env, nil, "--bundle-version")
	if code != 0 || len(diagnostics) != 0 || !bytes.HasPrefix(out, []byte("argv-fixture bundle/")) {
		t.Fatalf("version: exit %d, %s, %s", code, out, diagnostics)
	}
	if _, err := os.Stat(filepath.Join(root, "state")); !os.IsNotExist(err) {
		t.Fatal("metadata extracted or executed the application")
	}
}

func TestApplicationInterfaceValidation(t *testing.T) {
	base := application{Schema: 1, Name: "interface-fixture", Entry: "bin/entry", Files: []string{"bin/entry"}}
	for _, name := range []string{"", "goal", "argv"} {
		app := base
		app.Interface = name
		if err := validateApp(app); err != nil {
			t.Fatalf("valid interface %q: %v", name, err)
		}
	}
	for _, mutation := range []string{"unknown", "argv-followup", "argv-resume"} {
		app := base
		app.Interface = "argv"
		switch mutation {
		case "unknown":
			app.Interface = "command"
		case "argv-followup":
			app.Followup = true
		case "argv-resume":
			app.Resume = true
		}
		if err := validateApp(app); err == nil {
			t.Fatalf("accepted invalid interface declaration %s", mutation)
		}
	}
}
