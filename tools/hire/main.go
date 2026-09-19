// Hire authors filesystem experts; Agent remains the only runner.
package main

import (
	"crypto/sha256"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

//go:embed expert
var expert embed.FS

//go:embed builder/home.sh
var homeMaintenance []byte

const version = "0.3.0-dev"

func main() { os.Exit(command(os.Args[1:])) }

func command(args []string) int {
	if len(args) == 0 {
		fmt.Print(help)
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Print(help)
		return 0
	case "version", "-V", "--version":
		fmt.Println("hire " + version)
		return 0
	case "build":
		return build(args[1:])
	case "verify":
		return verify(args[1:])
	case "new":
		if len(args) > 1 && args[1] == "-home" {
			args = append([]string{"new"}, args[2:]...)
		} else {
			args = append([]string{"new", "-definition"}, args[1:]...)
		}
		return maintain(args)
	case "learn", "history", "actions", "act", "proposals", "amend":
		return maintain(args)
	default:
		return problem(fmt.Errorf("unknown builder command %q; use agent to run an expert", args[0]), 2)
	}
}

func problem(err error, code int) int { fmt.Fprintln(os.Stderr, "hire:", err); return code }

func agentCommand() (string, error) {
	path := os.Getenv("HIRE_AGENT")
	if path == "" {
		path = "agent"
	}
	path, err := exec.LookPath(path)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func build(args []string) int {
	var work, evidence string
	var forward []string
	flags := flag.NewFlagSet("hire build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&work, "C", "", "existing build workspace")
	flags.StringVar(&evidence, "evidence", "", "controller evidence directory")
	for _, name := range strings.Fields("state goal-file m effort checkpoint turns cycles timeout cap verbosity compact-at compactions") {
		flags.Func(name, "Agent option", func(value string) error {
			forward = append(forward, "-"+name, value)
			return nil
		})
	}
	for _, name := range strings.Fields("net no-cage q B compact stream require-action") {
		flags.BoolFunc(name, "Agent option", func(value string) error {
			forward = append(forward, "-"+name+"="+value)
			return nil
		})
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	weighMode := os.Getenv("BENCH_WEIGH")
	switch weighMode {
	case "", "0":
		weighMode = "0"
	case "1":
	default:
		return problem(fmt.Errorf("BENCH_WEIGH must be 0 (disabled) or 1 (enabled); unset defaults to 0"), 2)
	}
	if work == "" {
		return problem(fmt.Errorf("build needs -C WORKSPACE; output is WORKSPACE/expert"), 2)
	}
	work, err := filepath.Abs(work)
	if err != nil {
		return problem(err, 2)
	}
	info, err := os.Lstat(work)
	if err != nil || !info.IsDir() {
		return problem(fmt.Errorf("build workspace must be an existing real directory: %s", work), 2)
	}
	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return problem(err, 2)
	}
	agent, err := agentCommand()
	if err != nil {
		return problem(err, 2)
	}
	self, err := os.Executable()
	if err != nil {
		return problem(err, 2)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return problem(err, 2)
	}
	if evidence == "" {
		root := os.Getenv("HIRE_DIR")
		if root == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return problem(err, 2)
			}
			root = filepath.Join(home, ".hire")
		}
		key := sha256.Sum256([]byte(work))
		evidence = filepath.Join(root, fmt.Sprintf("%x", key[:16]))
	}
	tmp, err := os.MkdirTemp("", "hire-definition.")
	if err != nil {
		return problem(err, 2)
	}
	defer os.RemoveAll(tmp)
	if err := fs.WalkDir(expert, "expert", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(tmp, path)
		if entry.IsDir() {
			return os.Mkdir(target, 0700)
		}
		body, err := expert.ReadFile(path)
		if err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if path == "expert/bin/check" {
			mode = 0700
		}
		return os.WriteFile(target, body, mode)
	}); err != nil {
		return problem(err, 2)
	}
	// A new build request may revise an already valid definition. Start a work
	// session even when structural verification passes; callers can explicitly
	// choose -B=false to retain Agent's zero-model pre-check behavior.
	argv := append([]string{"run", "-C", work, "-evidence", evidence, "-B"}, forward...)
	argv = append(argv, filepath.Join(tmp, "expert"), "--")
	argv = append(argv, flags.Args()...)
	env := environment(map[string]string{"HIRE_AGENT": agent, "HIRE_BIN": self, "BENCH_WEIGH": weighMode})
	return execute(agent, argv, env)
}

func verify(args []string) int {
	if len(args) != 1 {
		return problem(fmt.Errorf("verify needs EXPERT"), 2)
	}
	definition, err := filepath.Abs(args[0])
	if err != nil {
		return problem(err, 1)
	}
	readme, err := os.Lstat(filepath.Join(definition, "README.md"))
	if err != nil || !readme.Mode().IsRegular() || readme.Size() == 0 || readme.Size() > 65536 {
		return problem(fmt.Errorf("expert needs a regular, nonempty README.md of at most 65536 bytes"), 1)
	}
	for _, name := range []string{"work", "state", ".agent"} {
		if _, err := os.Lstat(filepath.Join(definition, name)); !os.IsNotExist(err) {
			return problem(fmt.Errorf("reusable definition must not contain %s", name), 1)
		}
	}
	agent, err := agentCommand()
	if err != nil {
		return problem(err, 2)
	}
	tmp, err := os.MkdirTemp("", "hire-verify.")
	if err != nil {
		return problem(err, 2)
	}
	defer os.RemoveAll(tmp)
	work := filepath.Join(tmp, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		return problem(err, 2)
	}
	// Agent validates structure and Brief procedures; it does not execute the
	// generated bin/check. Generated code remains outside controller authority.
	return execute(agent, []string{"check", "-C", work, "-evidence", filepath.Join(tmp, "evidence"), definition}, os.Environ())
}

func maintain(args []string) int {
	tmp, err := os.MkdirTemp("", "hire-maintenance.")
	if err != nil {
		return problem(err, 2)
	}
	defer os.RemoveAll(tmp)
	path := filepath.Join(tmp, "home.sh")
	if err := os.WriteFile(path, homeMaintenance, 0600); err != nil {
		return problem(err, 2)
	}
	agent, err := agentCommand()
	if err != nil && args[0] != "new" {
		return problem(err, 2)
	}
	return execute("/bin/sh", append([]string{path}, args...), environment(map[string]string{"AGENT_RUNTIME": agent}))
}

func environment(set map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(set))
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if _, replace := set[name]; !replace {
			env = append(env, item)
		}
	}
	for name, value := range set {
		env = append(env, name+"="+value)
	}
	return env
}

// The builder forwards cancellation to Agent; Agent and Ply own execution
// below that process boundary. No retry or alternative execution path.
func execute(bin string, args, env []string) int {
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	cmd := exec.Command(bin, args...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = env, os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return problem(err, 1)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case s := <-signals:
			if s == syscall.SIGHUP {
				s = syscall.SIGTERM
			}
			_ = cmd.Process.Signal(s)
		case err := <-done:
			if err == nil {
				return 0
			}
			var exited *exec.ExitError
			if errors.As(err, &exited) {
				if status, ok := exited.Sys().(syscall.WaitStatus); ok && status.Signaled() {
					return 128 + int(status.Signal())
				}
				return exited.ExitCode()
			}
			return problem(err, 1)
		}
	}
}

const help = `hire - build filesystem experts with Agent

  hire new DIR [description ...]            scaffold a reusable definition
  hire new -home DIR [description ...]      scaffold a legacy recurring home
  hire build -C WORKSPACE [Agent flags] [-- job description ...]
  hire verify EXPERT                       inspect structure; run no generated code
  hire learn | history | actions | act | proposals | amend
  hire version | help

build runs Hire's builder expert through Agent. It writes WORKSPACE/expert;
the workspace must already exist. Give the job in arguments, -goal-file FILE,
or stdin. Piped evidence and the answer retain Agent's stream contract.
Exit status is Agent's outcome, including 2 unfinished and 130 interrupted.

-evidence DIR selects controller records outside the workspace; the default is
$HIRE_DIR/KEY or ~/.hire/KEY, keyed by the physical build workspace. -checkpoint
NAME resumes through Agent/Ply. Model, effort, limits, state and explicit Cage
flags pass to Agent. HIRE_AGENT selects the public Agent executable.
BENCH_WEIGH=1 permits the builder to add optional Weigh use. Unset, empty or
0 disables new selection; other values are configuration errors. An unrelated
revision preserves existing dependencies. This does not select a Weigh model,
authorize live evaluations or enable a paid fallback.

verify checks structural readiness through agent check. It does not prove the
generated worker's quality or execute its check. Inspect the expert and its
validation examples before choosing to run it with agent run -C WORKSPACE.

The extracted home maintenance commands retain their existing argument and
receipt formats. They use Agent for validation and Hone/Trail/Action/May for
their existing jobs. This command has no web server, worker registry or loop.
`
