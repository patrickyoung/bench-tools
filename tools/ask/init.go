package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/patrickyoung/ask/internal/event"
)

// cmdInit creates a record before any model or external tool is called. The
// empty model/system describe what happened: neither has been selected yet.
// Ask still owns the log; an ordinary caller owns the work it later records.
func cmdInit(args []string) (code int) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	file := fs.String("f", "", "new session file (required; must not exist)")
	usage(fs, "ask init -f file")
	if err := fs.Parse(args); err != nil {
		return usageCode(fs, err)
	}
	if *file == "" || fs.NArg() != 0 {
		return fail(errors.New("init requires -f FILE and no message"))
	}
	if strings.ContainsAny(*file, "\r\n\x00") {
		return fail(errors.New("init session path must fit on one line"))
	}
	path, err := filepath.Abs(*file)
	if err != nil {
		return fail(err)
	}
	log, err := event.CreateFile(path)
	if err != nil {
		return fail(err)
	}
	defer closeLog(log, &code)
	if _, err := log.AppendSealed(event.Session, event.Header{
		ID: log.ID(), Version: version, Go: goVersion(), SDKs: sdkVersions(),
	}); err != nil {
		return fail(fmt.Errorf("initialize session: %w", err))
	}
	closeLog(log, &code)
	if code != 0 {
		return code
	}
	return printOutput(path + "\n")
}
