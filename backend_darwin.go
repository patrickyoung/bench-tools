//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const seatbeltPath = "/usr/bin/sandbox-exec"

type seatbeltBackend struct{}

func nativeBackend() backend {
	if info, err := os.Stat(seatbeltPath); err != nil || info.IsDir() {
		return unavailableBackend{
			name: "seatbelt", detail: "macOS sandbox-exec is unavailable",
			alternative: "run the command in a container with explicit read-only and writable mounts",
		}
	}
	return seatbeltBackend{}
}

func (seatbeltBackend) status() statusRecord {
	return statusRecord{
		Backend: "seatbelt", Available: true, Complete: true,
		Filesystem: "writes restricted; reads unrestricted",
		Network:    "host network denied by default; -net permits",
		Detail:     "macOS Seatbelt via /usr/bin/sandbox-exec",
	}
}

func (seatbeltBackend) readyByte() byte { return 'x' }

func (seatbeltBackend) command(p policy, child []string) (*exec.Cmd, func(), error) {
	profile, err := os.CreateTemp("", "cage-*.sb")
	if err != nil {
		return nil, func() {}, err
	}
	name := profile.Name()
	cleanup := func() { _ = os.Remove(name) }
	var text strings.Builder
	text.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n")
	for _, path := range p.writes {
		fmt.Fprintf(&text, "(allow file-write* (subpath \"%s\"))\n", seatbeltQuote(path))
	}
	text.WriteString("(allow file-write* (literal \"/dev/null\") (literal \"/dev/zero\")\n")
	text.WriteString("  (literal \"/dev/dtracehelper\") (regex #\"^/dev/tty\"))\n")
	text.WriteString("(allow file-write-data (regex #\"^/dev/(stdout|stderr|fd/)\"))\n")
	if !p.network {
		text.WriteString("(deny network*)\n")
	}
	if _, err := profile.WriteString(text.String()); err != nil {
		profile.Close()
		cleanup()
		return nil, func() {}, err
	}
	if err := profile.Close(); err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return shellCommand(seatbeltPath, []string{"-f", name}, child), cleanup, nil
}

func seatbeltQuote(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`,
	)
	return r.Replace(s)
}
