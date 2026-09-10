//go:build !darwin && !linux

package main

func nativeBackend() backend {
	return unavailableBackend{
		name:   "none",
		detail: "Cage has no complete kernel backend for this platform",
		alternative: "use Windows Sandbox or a container with networking " +
			"disabled and explicit read-only/writable mounts",
	}
}
