//go:build linux

package main

import (
	"os"
	"os/exec"
)

type bubblewrapBackend struct{ path string }

func nativeBackend() backend {
	for _, path := range []string{"/usr/bin/bwrap", "/bin/bwrap", "/usr/local/bin/bwrap"} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return bubblewrapBackend{path: path}
		}
	}
	return unavailableBackend{
		name: "bubblewrap", detail: "Bubblewrap is not installed at a trusted system path",
		alternative: "install bubblewrap, or use podman with --network=none and explicit mounts",
	}
}

func (b bubblewrapBackend) status() statusRecord {
	return statusRecord{
		Backend: "bubblewrap", Available: true, Complete: true,
		Filesystem: "writes restricted; reads unrestricted",
		Network:    "host network denied by default; -net permits",
		Detail:     "Linux mount and network namespaces via " + b.path,
	}
}

func (b bubblewrapBackend) readyByte() byte { return '{' }

func (b bubblewrapBackend) command(p policy, child []string) (*exec.Cmd, func(), error) {
	args := []string{
		"--die-with-parent", "--new-session", "--unshare-user", "--disable-userns",
		"--cap-drop", "ALL", "--ro-bind", "/", "/", "--dev", "/dev",
		"--proc", "/proc", "--info-fd", "3",
	}
	if !p.network {
		args = append(args, "--unshare-net")
	}
	for _, path := range p.writes {
		args = append(args, "--bind", path, path)
	}
	args = append(args, "--chdir", p.cwd)
	args = append(args, child...)
	return exec.Command(b.path, args...), func() {}, nil
}
