//go:build linux

package main

import (
	"slices"
	"testing"
)

func TestBubblewrapUsesOnlyDocumentedHardBoundaryOptions(t *testing.T) {
	b := bubblewrapBackend{path: "/usr/bin/bwrap"}
	cmd, _, err := b.command(policy{
		cwd: "/work", writes: []string{"/work", "/scratch"},
	}, []string{"/bin/true"})
	if err != nil {
		t.Fatal(err)
	}
	wants := []string{
		"--die-with-parent", "--new-session", "--unshare-user",
		"--disable-userns", "--cap-drop", "ALL", "--ro-bind", "--dev",
		"--proc", "--info-fd", "3", "--unshare-net", "--bind", "--chdir",
	}
	for _, want := range wants {
		if !slices.Contains(cmd.Args, want) {
			t.Errorf("Bubblewrap args do not contain %q: %v", want, cmd.Args)
		}
	}
	if slices.Contains(cmd.Args, "--not-a-security-boundary") ||
		slices.Contains(cmd.Args, "--dev-bind") ||
		slices.Contains(cmd.Args, "--keep-fd") ||
		slices.Contains(cmd.Args, "--sync-fd") {
		t.Errorf("Bubblewrap args weaken or misspell the boundary: %v", cmd.Args)
	}
}

func TestNetFlagOmitsOnlyTheNetworkNamespace(t *testing.T) {
	b := bubblewrapBackend{path: "/usr/bin/bwrap"}
	cmd, _, err := b.command(policy{cwd: "/", network: true}, []string{"/bin/true"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(cmd.Args, "--unshare-net") {
		t.Errorf("-net still unshared the network: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Args, "--ro-bind") {
		t.Errorf("-net weakened the filesystem boundary: %v", cmd.Args)
	}
}
