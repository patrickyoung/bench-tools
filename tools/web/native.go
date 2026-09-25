package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/go-rod/rod/lib/launcher"
)

// nativeArgs uses Rod's user-mode preset while Web retains process ownership.
// Launcher.Launch may attach to an already listening port; that would silently
// change identity and ownership, so only its argument builder is used here.
func nativeArgs(executable, dir, profile string, headed bool) []string {
	l := launcher.NewUserMode().Bin(executable).UserDataDir(dir).RemoteDebuggingPort(0).
		Set("remote-debugging-address", "127.0.0.1").Set("no-first-run").
		Set("no-default-browser-check").Delete("no-startup-window").StartURL("about:blank")
	if profile == "" {
		profile = "Default"
	}
	l.ProfileDir(profile)
	if !headed {
		l.Set("headless", "new")
	}
	return l.FormatArgs()
}

// Canonicalize even a new path through existing symlinked parents, without
// reading any browser profile contents.
func canonicalPath(path string) (string, error) {
	absolute, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	resolved, e := filepath.EvalSymlinks(absolute)
	if e == nil {
		return resolved, nil
	}
	if !os.IsNotExist(e) {
		return "", e
	}
	parent := filepath.Dir(absolute)
	if parent == absolute {
		return "", e
	}
	resolved, e = canonicalPath(parent)
	if e != nil {
		return "", e
	}
	return filepath.Join(resolved, filepath.Base(absolute)), nil
}
func defaultBrowserRoots() []string {
	home, e := os.UserHomeDir()
	if e != nil {
		return nil
	}
	if runtime.GOOS == "darwin" {
		return []string{filepath.Join(home, "Library/Application Support/Google/Chrome"), filepath.Join(home, "Library/Application Support/Google/Chrome Beta"), filepath.Join(home, "Library/Application Support/Google/Chrome Canary"), filepath.Join(home, "Library/Application Support/Chromium")}
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	return []string{filepath.Join(config, "google-chrome"), filepath.Join(config, "google-chrome-beta"), filepath.Join(config, "google-chrome-unstable"), filepath.Join(config, "chromium")}
}
func selectedNativePath(path string, defaults []string) (string, error) {
	resolved, e := canonicalPath(path)
	if e != nil {
		return "", e
	}
	for _, root := range defaults {
		root, e = canonicalPath(root)
		if e != nil {
			return "", e
		}
		relative, e := filepath.Rel(root, resolved)
		if e == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return "", bad("Chrome's default personal data directory is not supported for remote debugging; choose a non-default --user-data-dir or use --attach with a browser already exposing CDP")
		}
	}
	return resolved, nil
}
func lockNativeProfile(path string) (string, *os.File, error) {
	dir, e := selectedNativePath(path, defaultBrowserRoots())
	if e != nil {
		return "", nil, e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", nil, e
	}
	lock, e := os.OpenFile(filepath.Join(dir, ".web-profile.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return "", nil, e
	}
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		lock.Close()
		return "", nil, fmt.Errorf("native browser profile is already in use by Web; use --attach for a running browser")
	}
	if _, e = os.Lstat(filepath.Join(dir, "SingletonLock")); !os.IsNotExist(e) {
		lock.Close()
		if e != nil {
			return "", nil, e
		}
		return "", nil, fmt.Errorf("native browser profile has a Chrome lock; close its browser or use --attach (Web never removes Chrome locks)")
	}
	return dir, lock, nil
}
