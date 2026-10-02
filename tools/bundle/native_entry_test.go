package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildRejectsForeignApplicationEntryBeforePublishing(t *testing.T) {
	root := t.TempDir()
	app, prefix := fixtureSource(root)
	// A recognizable executable header for the other supported operating
	// system must not ride inside a nominally native launcher as app data.
	foreign := make([]byte, 64)
	if runtime.GOOS == "darwin" {
		copy(foreign, []byte{0x7f, 'E', 'L', 'F', 2, 1, 1})
		binary.LittleEndian.PutUint16(foreign[16:], 2)
		binary.LittleEndian.PutUint16(foreign[18:], 62)
		binary.LittleEndian.PutUint32(foreign[20:], 1)
		binary.LittleEndian.PutUint16(foreign[52:], 64)
	} else {
		binary.LittleEndian.PutUint32(foreign, 0xfeedfacf)
		binary.LittleEndian.PutUint32(foreign[4:], 0x01000007)
		binary.LittleEndian.PutUint32(foreign[12:], 2)
	}
	put(filepath.Join(app, "bin/entry"), foreign, 0755)
	output := filepath.Join(root, "foreign-app")
	err := build([]string{"-o", output, "-runtime", prefix, app})
	if err == nil || !strings.Contains(err.Error(), "application entry") {
		t.Fatalf("foreign entry admitted: %v", err)
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("failed build published output")
	}
}
