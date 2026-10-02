package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPriorResultSkipsCaseArtifactsBeforeReadLimits(t *testing.T) {
	input := packetTest(t, "eval")
	original, err := readTree(input)
	if err != nil {
		t.Fatal(err)
	}
	for name, file := range original {
		if strings.HasPrefix(name, "cases/") {
			writeTest(t, filepath.Join(input, "evaluation", name), string(file.Data), file.Mode)
		}
	}
	if err := os.RemoveAll(filepath.Join(input, "cases")); err != nil {
		t.Fatal(err)
	}
	writeTest(t, filepath.Join(input, "result.json"), `{"schema":1,"action":"create"}`, 0600)
	oldWork := filepath.Join(input, "cases", "old", "work")
	if err := os.MkdirAll(oldWork, 0700); err != nil {
		t.Fatal(err)
	}
	large, err := os.Create(filepath.Join(oldWork, "unused-output"))
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(maxTree + 1); err != nil {
		t.Fatal(err)
	}
	if err := large.Close(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(oldWork, "unused-pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/", filepath.Join(oldWork, "unused-link")); err != nil {
		t.Fatal(err)
	}
	packet, err := admitRequest(input, "eval", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Cases) != 2 {
		t.Fatalf("expected reusable cases, got %+v", packet.Cases)
	}
	for name := range packet.Files {
		if strings.HasPrefix(name, "cases/old/") {
			t.Fatalf("old output admitted: %s", name)
		}
	}
}

func TestReadTreeRejectsFileReplacementBeforeOpen(t *testing.T) {
	for _, change := range []string{"symlink", "fifo", "directory", "regular"} {
		t.Run(change, func(t *testing.T) {
			directory := t.TempDir()
			file := filepath.Join(directory, "selected")
			writeTest(t, file, "expected", 0644)
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			before, err := root.Lstat("selected")
			if err != nil {
				t.Fatal(err)
			}
			// Retain the old inode so a newly created regular file cannot
			// accidentally reuse its identity.
			if err := os.Rename(file, filepath.Join(directory, "original")); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "symlink":
				err = os.Symlink("original", file)
			case "fifo":
				err = syscall.Mkfifo(file, 0600)
			case "directory":
				err = os.Mkdir(file, 0700)
			case "regular":
				writeTest(t, file, "expected", 0644)
				err = os.Chtimes(file, before.ModTime(), before.ModTime())
			}
			if err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			if _, err := readTreeFile(root, "selected", before); err == nil {
				t.Fatal("changed file was accepted")
			}
			if time.Since(started) > time.Second {
				t.Fatal("nonregular file inspection blocked")
			}
		})
	}
}

func TestReadTreeRejectsTraversedSymlinkReplacement(t *testing.T) {
	for _, outside := range []bool{false, true} {
		t.Run(map[bool]string{false: "within-root", true: "outside-root"}[outside], func(t *testing.T) {
			directory := t.TempDir()
			writeTest(t, filepath.Join(directory, "branch", "selected"), "expected", 0644)
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			before, err := root.Lstat("branch/selected")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(directory, "branch"), filepath.Join(directory, "original")); err != nil {
				t.Fatal(err)
			}
			target := "original"
			if outside {
				target = t.TempDir()
				writeTest(t, filepath.Join(target, "selected"), "outside", 0644)
			}
			if err := os.Symlink(target, filepath.Join(directory, "branch")); err != nil {
				t.Fatal(err)
			}
			if _, err := readTreeFile(root, "branch/selected", before); err == nil {
				t.Fatal("traversed directory replacement was accepted")
			}
			if _, err := readTree(directory); err == nil {
				t.Fatal("tree traversal accepted a directory symlink")
			}
		})
	}
}

func TestTreeStatDetectsChangedFileWithRestoredMtime(t *testing.T) {
	file := filepath.Join(t.TempDir(), "selected")
	writeTest(t, file, "original", 0644)
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if treeChangeTime(before) == nil {
		t.Fatal("supported Unix host has no selected ctime field")
	}
	writeTest(t, file, "modified", 0644)
	if err := os.Chtimes(file, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if sameTreeStat(before, after) {
		t.Fatal("restored mtime hid the content change")
	}
}

func TestReadTreePreservesSelectedBytesAndModes(t *testing.T) {
	directory := t.TempDir()
	writeTest(t, filepath.Join(directory, "nested", "program"), "#!/bin/sh\nexit 0\n", 0755)
	writeTest(t, filepath.Join(directory, "value"), "selected bytes\x00", 0600)
	files, err := readTree(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files["nested/program"].Mode != 0755 || files["value"].Mode != 0600 || string(files["value"].Data) != "selected bytes\x00" {
		t.Fatalf("incorrect snapshot: %+v", files)
	}
}
