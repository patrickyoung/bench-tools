package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

const maxGoal = 1 << 20

type runBinding struct {
	Schema     int    `json:"schema"`
	Package    string `json:"package"`
	Work       string `json:"work"`
	Goal       string `json:"goal"`
	GoalSHA256 string `json:"goal_sha256"`
}

func launch(args []string) int {
	a, err := openArchive(payload)
	if err != nil {
		return problem("bundle", err, 125)
	}
	app := a.manifest.App
	if app.Interface == "argv" {
		return launchArgv(a, args)
	}
	f := flag.NewFlagSet(app.Name, flag.ContinueOnError)
	f.SetOutput(os.Stderr)
	workspace := f.String("w", "", "workspace for a new run (empty or containing only input/)")
	legacyOutput := f.String("o", "", "compatibility alias for -w")
	follow := f.String("c", "", "follow up in an existing workspace")
	resume := f.String("resume", "", "resume the last request in an existing workspace")
	info := f.Bool("info", false, "show embedded package manifest")
	ver := f.Bool("version", false, "show version and package identity")
	help := f.Bool("help", false, "show usage")
	f.BoolVar(help, "h", false, "show usage")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Printf(appHelp, app.Name, app.Description, app.Name, app.Name, app.Name)
		return 0
	}
	if *info {
		b, _ := json.MarshalIndent(a.manifest, "", "  ")
		fmt.Println(string(b))
		return 0
	}
	if *ver {
		fmt.Printf("%s bundle/%s %s\n", app.Name, version, a.identity)
		return 0
	}
	if a.manifest.Platform != runtime.GOOS+"/"+runtime.GOARCH {
		return problem(app.Name, errors.New("package platform mismatch"), 125)
	}
	mode := "run"
	if *follow != "" {
		mode = "follow"
	}
	if *resume != "" {
		mode = "resume"
	}
	selectors := 0
	invalidSelector := false
	f.Visit(func(v *flag.Flag) {
		switch v.Name {
		case "w", "o", "c", "resume":
			selectors++
			invalidSelector = invalidSelector || v.Value.String() == ""
		}
	})
	if selectors > 1 || invalidSelector || f.NArg() > 1 || (*resume != "" && f.NArg() != 0) {
		return problem(app.Name, errors.New("choose one of -w (-o alias), -c or -resume with a nonempty directory; supply one quoted goal except with -resume"), 2)
	}
	if *legacyOutput != "" {
		*workspace = *legacyOutput
	}
	if (mode == "follow" && !app.Followup) || (mode == "resume" && !app.Resume) {
		return problem(app.Name, fmt.Errorf("this team does not support %s", mode), 2)
	}
	var goal []byte
	stdin := io.Reader(os.Stdin)
	if st, err := os.Stdin.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
		stdin = bytes.NewReader(nil)
	}
	if mode != "resume" {
		if f.NArg() == 1 {
			goal = []byte(f.Arg(0))
		} else {
			st, err := os.Stdin.Stat()
			if err != nil {
				return problem(app.Name, err, 2)
			}
			if st.Mode()&os.ModeCharDevice != 0 {
				return problem(app.Name, errors.New("supply a goal argument or pipe a goal on stdin"), 2)
			}
			goal, err = io.ReadAll(io.LimitReader(os.Stdin, maxGoal+1))
			if err != nil {
				return problem(app.Name, err, 2)
			}
			stdin = bytes.NewReader(nil)
		}
		if len(goal) > maxGoal || len(bytes.TrimSpace(goal)) == 0 {
			return problem(app.Name, errors.New("goal must be nonempty and at most 1 MiB"), 2)
		}
	}
	prepared, code := prepareApplication(a)
	if code != 0 {
		return code
	}
	root, packageRoot, searchPath := prepared.root, prepared.packageRoot, prepared.searchPath
	runs := filepath.Join(root, "runs")
	if err := secureDir(runs); err != nil {
		return problem(app.Name, err, 1)
	}
	var work string
	if mode == "run" {
		work = *workspace
		parent := "."
		if work != "" {
			parent = filepath.Dir(work)
		}
		physicalParent, parentErr := physicalDir(parent)
		if parentErr != nil {
			return problem(app.Name, parentErr, 1)
		}
		candidate := filepath.Join(physicalParent, app.Name+"-new")
		if work != "" {
			candidate = filepath.Join(physicalParent, filepath.Base(work))
		}
		if overlaps(candidate, root) {
			return problem(app.Name, errors.New("workspace and bundle controller directories must be disjoint"), 1)
		}
		if work == "" {
			work, err = os.MkdirTemp(".", app.Name+"-")
		} else {
			err = os.Mkdir(work, 0700)
			if os.IsExist(err) {
				// A caller may stage input before launching. Admission under the
				// run lock below distinguishes prepared roots from prior work.
				err = nil
			}
		}
		if err != nil {
			return problem(app.Name, fmt.Errorf("workspace directory: %w", err), 1)
		}
	} else if mode == "follow" {
		work = *follow
	} else {
		work = *resume
	}
	work, err = physicalDir(work)
	if err != nil {
		return problem(app.Name, err, 1)
	}
	// No model-writable root may contain the package or its controller state.
	if overlaps(work, root) {
		return problem(app.Name, errors.New("workspace and bundle controller directories must be disjoint"), 1)
	}
	run := filepath.Join(runs, digest([]byte(work)))
	if mode == "run" {
		err = secureDir(run)
	} else {
		err = noLinks(run)
	}
	if err != nil {
		return problem(app.Name, errors.New("no matching run for this workspace and exact package"), 1)
	}
	runLock, err := lock(filepath.Join(run, "lock"))
	if err != nil {
		return problem(app.Name, err, 75)
	}
	defer runLock.Close()
	activePath := filepath.Join(run, "active.json")
	if _, err := os.Lstat(activePath); !os.IsNotExist(err) {
		return problem(app.Name, fmt.Errorf("prior invocation has no confirmed end; inspect its processes and effects before removing %s", activePath), 125)
	}
	control, state := filepath.Join(run, "control"), filepath.Join(run, "state")
	for _, dir := range []string{control, state} {
		if err := secureDir(dir); err != nil {
			return problem(app.Name, err, 1)
		}
	}
	bindingPath := filepath.Join(run, "run.json")
	binding := runBinding{Schema: 2, Package: a.identity, Work: work}
	if mode != "run" {
		data, _, err := readRegular(bindingPath)
		if err != nil {
			return problem(app.Name, err, 1)
		}
		if err := decode(data, &binding); err != nil {
			return problem(app.Name, err, 1)
		}
		if binding.Schema != 2 || binding.Package != a.identity || binding.Work != work || !relative(binding.Goal) || filepath.Base(binding.Goal) != binding.Goal {
			return problem(app.Name, errors.New("run binding changed"), 125)
		}
		if b, _, err := readRegular(filepath.Join(control, binding.Goal)); err != nil || len(b) > maxGoal || digest(b) != binding.GoalSHA256 {
			return problem(app.Name, errors.New("retained goal changed"), 125)
		}
	} else if _, err := os.Lstat(bindingPath); !os.IsNotExist(err) {
		return problem(app.Name, errors.New("workspace already has a retained run; use -c for a supported follow-up or choose another directory"), 1)
	}
	if err := prepareWorkspace(work, mode == "run"); err != nil {
		return problem(app.Name, err, 1)
	}
	if mode != "resume" {
		file, err := os.CreateTemp(control, "goal-")
		if err != nil {
			return problem(app.Name, err, 1)
		}
		name := file.Name()
		_, writeErr := file.Write(goal)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return problem(app.Name, err, 1)
		}
		if err := syncDirectory(control); err != nil {
			return problem(app.Name, err, 1)
		}
		binding.Goal = filepath.Base(name)
		binding.GoalSHA256 = digest(goal)
		if err := writeJSON(bindingPath, binding); err != nil {
			return problem(app.Name, err, 1)
		}
	}
	goalPath := filepath.Join(control, binding.Goal)
	if b, _, err := readRegular(goalPath); err != nil || len(b) > maxGoal || digest(b) != binding.GoalSHA256 {
		return problem(app.Name, errors.New("retained goal is missing or invalid"), 125)
	}
	working := filepath.Join(work, "working")
	output := filepath.Join(work, "output")
	fmt.Fprintf(os.Stderr, "%s: workspace %s\n%s: output %s\n%s: records %s\n", app.Name, work, app.Name, output, app.Name, control)
	env := environment(map[string]string{
		"PATH": searchPath, "BUNDLE_ROOT": filepath.Join(packageRoot, "app"),
		"BUNDLE_WORKSPACE": work, "BUNDLE_INPUT": filepath.Join(work, "input"),
		"BUNDLE_WORK": working, "BUNDLE_OUTPUT": output,
		"BUNDLE_STATE": state, "BUNDLE_CONTROL": control, "BUNDLE_GOAL_FILE": goalPath,
	})
	cmd := exec.Command(filepath.Join(packageRoot, "app", filepath.FromSlash(app.Entry)), mode)
	cmd.Dir, cmd.Env = working, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, os.Stdout, os.Stderr
	// A killed controller cannot attest that its child stopped. Retain a fence
	// across that crash, even after the OS releases the ordinary concurrency lock.
	if err := writeJSON(activePath, map[string]any{"controller_pid": os.Getpid(), "mode": mode, "goal_sha256": binding.GoalSHA256}); err != nil {
		return problem(app.Name, err, 1)
	}
	code = execute(cmd, app.Name)
	if err := os.Remove(activePath); err != nil {
		return problem(app.Name, err, 125)
	}
	return code
}

// Called under the private run lock, before admitting a goal. Layout directories
// are never repaired on continuation: losing one requires inspection, not a new
// invocation silently manufacturing apparently complete work.
func prepareWorkspace(root string, fresh bool) error {
	if fresh {
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name() != "input" || !entry.IsDir() {
				return fmt.Errorf("workspace is not fresh: %s; a new run accepts only an empty directory or input/ (choose a new workspace for prior or incomplete work)", root)
			}
		}
	}
	// Check the complete existing layout before creating anything. Inputs stay
	// caller-owned; this check does not freeze them or enforce read-only access.
	missing := []string{}
	// Different package identities have different private locks. Claim working/
	// first, so only one fresh launcher can own setup of this physical root.
	// A loser must not remove input/ that the winner admitted as staged material.
	for _, name := range []string{"working", "input", "output"} {
		dir := filepath.Join(root, name)
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) && fresh {
			missing = append(missing, dir)
			continue
		}
		if err != nil {
			return fmt.Errorf("workspace %s: %w", name, err)
		}
		if fresh && name != "input" {
			return fmt.Errorf("workspace is not fresh: %s already exists; choose a new workspace for prior or incomplete work", dir)
		}
		if !info.IsDir() {
			return fmt.Errorf("workspace %s must be a real directory: %s", name, dir)
		}
	}
	created := []string{}
	for _, dir := range missing {
		if err := os.Mkdir(dir, 0700); err != nil {
			// Remove only our newly created, still-empty directories. Never
			// delete staged input or recursively erase an interrupted job.
			for i := len(created) - 1; i >= 0; i-- {
				_ = os.Remove(created[i])
			}
			return fmt.Errorf("workspace setup incomplete: %w; inspect %s before retrying", err, root)
		}
		created = append(created, dir)
	}
	return nil
}

type preparedApplication struct {
	root, packageRoot, searchPath string
}

// Both interfaces use exactly the same verified package and executable lookup.
// Run admission belongs to the goal interface or to an argv application's entry.
func prepareApplication(a *packageArchive) (preparedApplication, int) {
	var prepared preparedApplication
	name := a.manifest.App.Name
	root, err := privateRoot()
	if err != nil {
		return prepared, problem(name, err, 1)
	}
	root = filepath.Join(root, a.identity)
	if err := secureDir(root); err != nil {
		return prepared, problem(name, err, 1)
	}
	// Serialize only extraction/verification; different jobs run concurrently.
	packageLock, err := lock(filepath.Join(root, "package.lock"))
	if err != nil {
		return prepared, problem(name, err, 75)
	}
	packageRoot, err := a.extract(root)
	packageLock.Close()
	if err != nil {
		return prepared, problem(name, err, 125)
	}
	searchPath := executablePath(filepath.Join(packageRoot, "runtime", "bin"), os.Getenv("PATH"))
	if err := requirements(a.manifest.App.Requires, searchPath); err != nil {
		return prepared, problem(name, err, 127)
	}
	return preparedApplication{root: root, packageRoot: packageRoot, searchPath: searchPath}, 0
}

func launchArgv(a *packageArchive, args []string) int {
	app := a.manifest.App
	// Reserve only these exact, sole arguments. Application flags, including
	// help, and any occurrence alongside other arguments belong to the entry.
	if len(args) == 1 {
		switch args[0] {
		case "--bundle-info":
			b, _ := json.MarshalIndent(a.manifest, "", "  ")
			fmt.Println(string(b))
			return 0
		case "--bundle-version":
			fmt.Printf("%s bundle/%s %s\n", app.Name, version, a.identity)
			return 0
		}
	}
	if a.manifest.Platform != runtime.GOOS+"/"+runtime.GOARCH {
		return problem(app.Name, errors.New("package platform mismatch"), 125)
	}
	prepared, code := prepareApplication(a)
	if code != 0 {
		return code
	}
	cmd := exec.Command(filepath.Join(prepared.packageRoot, "app", filepath.FromSlash(app.Entry)), args...)
	cmd.Env = environment(map[string]string{
		"PATH": prepared.searchPath, "BUNDLE_ROOT": filepath.Join(prepared.packageRoot, "app"), "BUNDLE_ID": a.identity,
	}, "BUNDLE_WORKSPACE", "BUNDLE_INPUT", "BUNDLE_WORK", "BUNDLE_OUTPUT", "BUNDLE_STATE", "BUNDLE_CONTROL", "BUNDLE_GOAL_FILE")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return execute(cmd, app.Name)
}

// Never reinterpret a relative PATH entry after switching to worker scratch.
func executablePath(bundled, inherited string) string {
	paths := []string{bundled}
	for _, dir := range filepath.SplitList(inherited) {
		if filepath.IsAbs(dir) {
			paths = append(paths, dir)
		}
	}
	return strings.Join(paths, string(os.PathListSeparator))
}

func overlaps(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator)) || strings.HasPrefix(b, a+string(filepath.Separator))
}

func lock(name string) (*os.File, error) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("busy: %s", name)
	}
	return f, nil
}

func writeJSON(name string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".binding-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(append(b, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err = errors.Join(err, closeErr); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), name); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(name))
}

func syncDirectory(name string) error {
	dir, err := os.Open(name)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func requirements(names []string, search string) error {
	for _, name := range names {
		found := false
		for _, dir := range filepath.SplitList(search) {
			// Refuse cwd lookup, including empty PATH entries.
			if !filepath.IsAbs(dir) {
				continue
			}
			i, err := os.Stat(filepath.Join(dir, name))
			if err == nil && i.Mode().IsRegular() && i.Mode().Perm()&0111 != 0 {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing required command %q", name)
		}
	}
	return nil
}

func execute(cmd *exec.Cmd, name string) int {
	// A process group also reaches shell children. Agent/Ply retain their own
	// descendant cancellation semantics; there is no background supervisor.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	if err := cmd.Start(); err != nil {
		return problem(name, err, 126)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case sig := <-signals:
			if s, ok := sig.(syscall.Signal); ok {
				_ = syscall.Kill(-cmd.Process.Pid, s)
			}
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
			return problem(name, err, 1)
		}
	}
}

const appHelp = `%s - %s

  %s [-w WORKSPACE] [--] 'GOAL'
  %s -c WORKSPACE [--] 'FOLLOW-UP'
  %s -resume WORKSPACE

Without a goal argument, stdin supplies the goal (at most 1 MiB). With a goal,
stdin passes through as evidence. No terminal chat loop is started. Flags must
precede the single goal argument; -- permits goals beginning with a dash.
The workspace contains input/ (source material), working/ (drafts), and
output/ (deliverables). Start with a new directory or one containing only input/.
Omit -w to create a retained workspace in the current directory.
-o is a compatibility alias for -w and uses the same three-folder layout.
Follow-up and resume require the team's explicit support and the exact bundle.
Stdout, stderr and exit status retain the entry command's contract.

  -info       print the embedded manifest, without executing the team
  -version    print the builder version and exact package identity
  -help       print this usage
`
