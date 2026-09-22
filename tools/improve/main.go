package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"syscall"
	"time"
)

const version = "0.1.0"
const help = `usage: improve [-n] [-o NEW_DIRECTORY] < experiment.json
       improve verify [-record EXECUTABLE] DIRECTORY
       improve version

Run one bounded experiment through caller-selected public commands.
-n validates and prints the maximum workload without executing commands.
-o selects a new evidence directory; its parent must exist.
stdout: one JSON result. stderr: progress and errors.
Exit 0: supported proposal (or successful plan/verification).
Exit 1: no supported proposal or incomplete execution. Exit 2: invalid/error.
No source writes, deployment, automatic retry, provider client or daemon.
See improve.1 and README.md for the versioned JSON process contracts.
`

func verify(ctx context.Context, root, record string) error {
	p, e := plainPath(root)
	if e != nil {
		return e
	}
	raw, e := readFile(filepath.Join(p, "inventory.json"), maxJSON)
	if e != nil {
		return e
	}
	var inv Inventory
	if e = decode(raw, &inv); e != nil {
		return e
	}
	if inv.Version != 1 || len(inv.Files) == 0 {
		return fmt.Errorf("invalid inventory")
	}
	actual, e := artifacts(p)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(inv.Files, actual) {
		return fmt.Errorf("retained artifact inventory changed")
	}
	resultRaw, e := readFile(filepath.Join(p, "result.json"), maxJSON)
	if e != nil {
		return e
	}
	var result Result
	if e = decode(resultRaw, &result); e != nil {
		return e
	}
	if !reflect.DeepEqual(result.Calls, inv.Calls) {
		return fmt.Errorf("result and call inventory differ")
	}
	for _, c := range inv.Calls {
		if !cleanPath(c.Directory) || !c.Complete {
			return fmt.Errorf("incomplete recorded call")
		}
		check, cancel := context.WithTimeout(ctx, 30*time.Second)
		e = replay(check, record, filepath.Join(p, c.Directory), c)
		cancel()
		if e != nil {
			return e
		}
	}
	return nil
}
func run(args []string, in io.Reader, out, errout io.Writer) int {
	fail := func(e error) int { fmt.Fprintln(errout, "improve:", e); return 2 }
	if len(args) > 0 && (args[0] == "version" || args[0] == "-version" || args[0] == "--version") {
		if len(args) != 1 {
			return fail(fmt.Errorf("unexpected version arguments"))
		}
		fmt.Fprintln(out, "improve "+version)
		return 0
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(out, help)
		return 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	if len(args) > 0 && args[0] == "verify" {
		f := flag.NewFlagSet("verify", flag.ContinueOnError)
		f.SetOutput(errout)
		record := f.String("record", "record", "Record executable")
		if e := f.Parse(args[1:]); e != nil {
			return fail(e)
		}
		if f.NArg() != 1 {
			return fail(fmt.Errorf("verify requires one directory"))
		}
		exe, e := executable(*record)
		if e != nil {
			return fail(e)
		}
		if e = verify(ctx, f.Arg(0), exe); e != nil {
			return fail(e)
		}
		fmt.Fprintln(out, `{"version":1,"verified":true,"inference_calls":0}`)
		return 0
	}
	f := flag.NewFlagSet("improve", flag.ContinueOnError)
	f.SetOutput(errout)
	plan := f.Bool("n", false, "validate without execution")
	dest := f.String("o", "", "new output directory")
	if e := f.Parse(args); e != nil {
		return fail(e)
	}
	if f.NArg() != 0 || (!*plan && *dest == "") {
		return fail(fmt.Errorf("select -n or -o NEW_DIRECTORY; specification is stdin"))
	}
	raw, e := io.ReadAll(io.LimitReader(in, maxJSON+1))
	if e != nil {
		return fail(e)
	}
	s, e := validateSpec(raw)
	if e != nil {
		return fail(e)
	}
	output := ""
	if *dest != "" {
		output, e = outputPath(*dest, selected(s))
		if e != nil {
			return fail(e)
		}
	}
	if *plan {
		_, e = out.Write(marshal(map[string]any{"version": 1, "mode": "plan", "max_worker_trials": workload(s), "max_proposals": 1, "max_judges": 2, "max_seconds": s.MaxSeconds, "command_seconds": s.CommandSeconds, "executes_commands": false}))
		if e != nil {
			return fail(e)
		}
		return 0
	}
	bounded, cancel := context.WithTimeout(ctx, time.Duration(s.MaxSeconds)*time.Second)
	defer cancel()
	x, e := prepare(bounded, s, output)
	if x == nil {
		return fail(e)
	}
	if e == nil {
		e = x.execute()
	}
	code := 1
	if e != nil {
		x.result.Decision = "invalid_evidence"
		if errors.Is(e, errIncomplete) || bounded.Err() != nil {
			x.result.Decision = "inconclusive"
		} else {
			code = 2
		}
		x.result.Reason = e.Error()
	} else if x.result.Decision == "supported" {
		code = 0
	}
	if e = x.finish(); e != nil {
		// An admitted run always emits its terminal result, even if retaining
		// the inventory failed. finish removes any unsupported export.
		fmt.Fprintln(errout, "improve:", e)
		code = 2
	}
	if _, e = out.Write(marshal(x.result)); e != nil {
		return fail(e)
	}
	return code
}

var errIncomplete = errors.New("incomplete command")

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
