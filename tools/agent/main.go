// Agent composes a filesystem definition with the public Brief and Ply programs.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const version = "0.3.0-dev"

func main() { os.Exit(command(os.Args[1:])) }

func command(args []string) int {
	if len(args) == 0 {
		fmt.Print(runnerHelp)
		return 0
	}
	if len(args) > 0 {
		switch args[0] {
		case "run", "tick":
			return run(args[0], args[1:])
		case "check":
			return inspect(args[1:], false)
		case "show":
			return inspect(args[1:], true)
		case "specialist":
			return specialist(args[1:])
		case "-c":
			return actionShell(args)
		case "version", "-V", "--version":
			fmt.Println("agent " + version)
			return 0
		case "help", "-h", "--help":
			fmt.Print(runnerHelp)
			return 0
		}
	}
	return problem(fmt.Errorf("unknown runner command %q; authoring belongs to the separate builder", args[0]), 2)
}

func inspect(args []string, show bool) int {
	o, rest, err := parse(args)
	if err != nil || len(rest) > 1 {
		return problem(fmt.Errorf("check/show expects [flags] DEFINITION"), 2)
	}
	home := "."
	if len(rest) == 1 {
		home = rest[0]
	}
	d, err := openDefinition(home, o)
	if err == nil {
		err = d.validateProcedures()
	}
	if err != nil {
		return problem(err, 1)
	}
	fmt.Fprintf(os.Stderr, "agent: %s is a valid agent definition\n", d.Home)
	if show {
		return d.show()
	}
	return 0
}

func specialist(args []string) int {
	if len(args) < 2 || !portableName(args[1]) {
		return problem(fmt.Errorf("specialist needs PARENT and a portable NAME"), 2)
	}
	parent, err := openDefinition(args[0], options{})
	if err != nil {
		return problem(err, 1)
	}
	if err := parent.validateProcedures(); err != nil {
		return problem(err, 1)
	}
	child, err := realDir(filepath.Join(parent.Home, "agents", args[1]))
	if err != nil {
		return problem(err, 2)
	}
	_, rest, err := parse(args[2:])
	if err != nil {
		return problem(err, 2)
	}
	flags := append([]string(nil), args[2:len(args)-len(rest)]...)
	if len(flags) > 0 && flags[len(flags)-1] == "--" {
		flags = flags[:len(flags)-1]
	}
	call := append(flags, child, "--")
	return run("run", append(call, rest...))
}

type options struct {
	work, state, control, goalFile, model, effort, checkpoint, steer string
	noCage, network, quiet                                           bool
	forward                                                          []string
}

func parse(args []string) (options, []string, error) {
	var o options
	fs := flag.NewFlagSet("agent run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.work, "C", "", "separate workspace")
	fs.StringVar(&o.state, "state", "", "mutable state directory")
	fs.StringVar(&o.control, "evidence", "", "controller evidence directory")
	fs.StringVar(&o.goalFile, "goal-file", "", "private invocation goal file")
	fs.StringVar(&o.model, "m", "", "model, passed to Ply")
	fs.StringVar(&o.effort, "effort", "", "effort, passed to Ply")
	fs.StringVar(&o.checkpoint, "checkpoint", "", "checkpoint name")
	fs.StringVar(&o.steer, "steer", "", "controller steering file, passed to Ply")
	fs.BoolVar(&o.noCage, "no-cage", false, "ordinary host actions")
	fs.BoolVar(&o.network, "net", false, "allow network inside Cage")
	fs.BoolVar(&o.quiet, "q", false, "quiet progress")
	for _, name := range strings.Fields("turns cycles timeout cap verbosity compact-at compactions") {
		fs.Func(name, "Ply option", func(value string) error {
			o.forward = append(o.forward, "-"+name, value)
			return nil
		})
	}
	for _, name := range []string{"record-input", "record-output"} {
		fs.Func(name, "select a file for replay", func(value string) error {
			o.forward = append(o.forward, "-"+name, value)
			return nil
		})
	}
	for _, name := range strings.Fields("B compact stream require-action") {
		fs.BoolFunc(name, "Ply option", func(value string) error {
			o.forward = append(o.forward, "-"+name+"="+value)
			return nil
		})
	}
	if err := fs.Parse(args); err != nil {
		return o, nil, err
	}
	if o.noCage && o.network {
		return o, nil, errors.New("-net has no meaning with -no-cage")
	}
	if o.checkpoint != "" && !portableName(o.checkpoint) {
		return o, nil, errors.New("checkpoint needs a portable name")
	}
	if o.work == "" && (o.state != "" || o.control != "") {
		return o, nil, errors.New("-state and -evidence require -C")
	}
	return o, fs.Args(), nil
}

func problem(err error, code int) int { fmt.Fprintln(os.Stderr, "agent:", err); return code }

func tool(name, fallback string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		value = fallback
	}
	path, err := exec.LookPath(value)
	if err != nil {
		return "", fmt.Errorf("%s executable not found: %s: %w", name, value, err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func withEnv(env []string, set map[string]string, unset ...string) []string {
	drop := map[string]bool{}
	for _, k := range unset {
		drop[k] = true
	}
	for k := range set {
		drop[k] = true
	}
	result := make([]string, 0, len(env)+len(set))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if !drop[key] {
			result = append(result, entry)
		}
	}
	for k, v := range set {
		result = append(result, k+"="+v)
	}
	return result
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func portableName(value string) bool {
	if len(value) == 0 || len(value) > 64 || value[0] < 'a' || value[0] > 'z' || strings.HasSuffix(value, "-") {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return false
		}
	}
	return true
}

func boundedOutput(bin string, args, env []string, dir string, limit int) ([]byte, int, error) {
	out := &boundedBuffer{limit: limit}
	code := execute(bin, args, env, nil, out, os.Stderr, dir)
	if out.full {
		return nil, code, fmt.Errorf("%s output exceeds %d bytes", bin, limit)
	}
	return out.Bytes(), code, nil
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
	full   bool
}

func (b *boundedBuffer) Bytes() []byte { return b.buffer.Bytes() }
func (b *boundedBuffer) Len() int      { return b.buffer.Len() }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > b.limit-b.Len() {
		b.full = true
		p = p[:max(0, b.limit-b.Len())]
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

const runnerHelp = `agent - run a filesystem expert using the Bench tools

  agent run [flags] HOME [-- focus ...]
  agent run -C WORKSPACE [flags] DEFINITION [-- goal ...]
  agent tick [flags] HOME
  agent specialist PARENT NAME [run flags] [-- focus ...]
  agent check [path flags] DEFINITION | agent show [path flags] DEFINITION
  agent help | agent version

Home runs preserve GOAL.md and add invocation focus. With -C, explicit goal
text replaces the standing goal. -goal-file reads a private goal from a file.
Piped stdin is evidence; if a portable definition has no goal, stdin is it.

  -C DIR           use a separate existing workspace
  -state DIR       mutable state (with -C; default WORKSPACE/state)
  -evidence DIR    controller records (with -C; default ~/.agent/KEY)
  -m MODEL         pass the model literally to Ply/Ask
  -effort LEVEL    pass reasoning effort literally
  -checkpoint NAME resume a named Ply conversation pointer
  -steer FILE      pass a controller-owned guidance file to Ply
  -goal-file FILE  private goal file, instead of goal text arguments
  -net             allow network in Cage actions
  -no-cage         ordinary host action permissions
  -record-input FILE  retain an input file before work (repeatable)
  -record-output FILE retain an output file after work (repeatable)
  -q               suppress progress
  -turns N -cycles N -timeout D -cap N -verbosity LEVEL
  -B -compact -compact-at N -compactions N -stream -require-action
                   passed literally to Ply; see ply help for their meaning

Definition: AGENTS.md and executable bin/check; optional SOUL.md, PLAN.md,
MEMORY.md, GOAL.md, skills/, tools/ and agents/. A toolbox is not a sandbox.
The default Cage boundary writes work, state and private action temp only;
network is denied and host reads remain unrestricted. Checks stay outside.
An explicit host boundary is required for ordinary recursive model calls.

stdin is input; stdout is the answer; stderr is progress. Exit is Ply's:
0 checked, 1 broken, 2 unfinished, 3 declined, 75 parked, 125 boundary failure,
130 interrupted. Invocation/definition errors use 2/1 respectively.

AGENT_ASK, AGENT_BRIEF, AGENT_PLY, AGENT_CAGE and AGENT_RECORD select companions.
Record is required for runs. Full action/check streams and selected files live
under evidence/recordings; Ask owns their sealed history. Recording failures
stop with 125. File selections are relative to the workspace.
AGENT_DIR replaces ~/.agent for portable evidence. KEY derives from the
physical definition and workspace paths, without a registry. Explicit state
and evidence paths are recommended when the caller manages their lifecycle.
AGENT_BIN, AGENT_HOME, AGENT_WORK and AGENT_STATE describe the current run.
Use hire new or hire build to author an expert. Agent contains no builder.
check/show accept -C, -state and -evidence without creating runtime state.
`
