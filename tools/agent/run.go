package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func run(kind string, args []string) int {
	o, rest, err := parse(args)
	if err != nil {
		return problem(err, 2)
	}
	home := "."
	if len(rest) > 0 {
		if info, e := os.Stat(rest[0]); e == nil && info.IsDir() {
			home, rest = rest[0], rest[1:]
		} else if o.work != "" {
			return problem(fmt.Errorf("definition directory not found: %s", rest[0]), 2)
		}
	}
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	if o.goalFile != "" && len(rest) > 0 {
		return problem(fmt.Errorf("use -goal-file or goal arguments, not both"), 2)
	}
	nested, err := inheritedBoundary()
	if err != nil {
		return problem(err, 125)
	}
	d, err := openDefinition(home, o)
	if err != nil {
		return problem(err, 1)
	}
	if err := d.validateProcedures(); err != nil {
		return problem(err, 1)
	}
	if o.steer != "" {
		o.steer, err = steeringPath(o.steer, d)
		if err != nil {
			return problem(err, 2)
		}
	}
	if !o.quiet {
		fmt.Fprintf(os.Stderr, "agent: %s is a valid agent definition\n", d.Home)
	}

	task := strings.Join(rest, " ")
	if o.goalFile != "" {
		b, e := readRegular(o.goalFile, contextLimit)
		if e != nil {
			return problem(e, 2)
		}
		task = string(b)
		if !meaningful(b) {
			return problem(fmt.Errorf("goal file is empty"), 2)
		}
	}
	var wake []byte
	if kind == "tick" {
		if !meaningful(d.Files["HEARTBEAT.md"]) {
			if !o.quiet {
				fmt.Fprintln(os.Stderr, "agent: heartbeat is empty; quiet")
			}
			return 0
		}
		var status int
		wake, status, err = boundedOutput(filepath.Join(d.Home, "bin/wake"), nil, os.Environ(), d.Work, fileLimit)
		if err != nil {
			return problem(fmt.Errorf("bin/wake emitted too much output; the limit is %d bytes: %w", fileLimit, err), 2)
		}
		switch status {
		case 0:
			if !o.quiet {
				fmt.Fprintln(os.Stderr, "agent: wake check is quiet; no model call")
			}
			return 0
		case 1:
		default:
			return problem(fmt.Errorf("bin/wake is broken: exit %d", status), 2)
		}
		task = string(d.Files["HEARTBEAT.md"]) + focus(task)
	} else if !d.Portable {
		task = string(d.Files["GOAL.md"]) + focus(task)
	} else if task == "" && meaningful(d.Files["GOAL.md"]) {
		task = string(d.Files["GOAL.md"])
	}
	input, err := readInput()
	if err != nil {
		return problem(err, 2)
	}
	if strings.TrimSpace(task) == "" {
		if !d.Portable {
			return problem(fmt.Errorf("no goal"), 2)
		}
		task = string(input)
		input = nil
	}
	if len(task) > contextLimit {
		return problem(fmt.Errorf("run task exceeds %d bytes; pipe large evidence instead", contextLimit), 2)
	}
	if !meaningful([]byte(task)) {
		return problem(fmt.Errorf("no goal: supply goal text, -goal-file, GOAL.md or stdin"), 2)
	}
	body := d.context()
	if len(body) > contextLimit {
		return problem(fmt.Errorf("compiled definition exceeds %d bytes", contextLimit), 2)
	}
	ply, err := tool("AGENT_PLY", "ply")
	if err != nil {
		return problem(err, 2)
	}
	brief, err := tool("AGENT_BRIEF", "brief")
	if err != nil {
		return problem(err, 2)
	}
	ask, err := tool("AGENT_ASK", "ask")
	if err != nil {
		return problem(err, 2)
	}
	record, err := tool("AGENT_RECORD", "record")
	if err != nil {
		return problem(err, 125)
	}
	if inside(d.Work, record) || inside(d.State, record) || inside(d.Home, record) {
		return problem(fmt.Errorf("Record executable must be outside definition and mutable roots"), 125)
	}
	if err := checkPly(ply, !o.noCage); err != nil {
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
	cage := ""
	if !o.noCage {
		cage, err = tool("AGENT_CAGE", "cage")
		if err != nil {
			return problem(err, 2)
		}
		if inside(d.Home, cage) || inside(d.Work, cage) || inside(d.State, cage) {
			return problem(fmt.Errorf("Cage executable must be outside definition and mutable roots"), 2)
		}
	}
	tmpParent, err := realDir(os.TempDir())
	if err != nil {
		return problem(err, 2)
	}
	if inside(d.Home, tmpParent) || inside(d.Work, tmpParent) || inside(d.State, tmpParent) {
		return problem(fmt.Errorf("temporary directory must be outside agent home, work and state"), 2)
	}
	checkpoint := ""
	if o.checkpoint != "" {
		checkpoint = filepath.Join(d.Control, "checkpoints", o.checkpoint+".current")
		if err := checkCheckpoint(checkpoint, filepath.Join(d.Control, "runs")); err != nil {
			return problem(err, 2)
		}
	}
	tmp, err := os.MkdirTemp(tmpParent, "agent-run.")
	if err != nil {
		return problem(err, 2)
	}
	defer os.RemoveAll(tmp)
	actionTmp := filepath.Join(tmp, "action-tmp")
	if err := os.Mkdir(actionTmp, 0700); err != nil {
		return problem(err, 2)
	}
	skill := filepath.Join(tmp, "agent-context")
	if err := os.Mkdir(skill, 0700); err != nil {
		return problem(err, 2)
	}
	compiled := "---\nname: agent-context\ndescription: Private compiled context for this exact agent home run.\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(compiled), 0600); err != nil {
		return problem(err, 2)
	}
	goal := filepath.Join(tmp, "task.md")
	if err := os.WriteFile(goal, []byte(task), 0600); err != nil {
		return problem(err, 2)
	}
	var evidence bytes.Buffer
	if kind == "tick" && len(wake) > 0 {
		fmt.Fprintf(&evidence, "# Wake evidence\n\n%s\n", wake)
	}
	if kind != "tick" && meaningful(d.Files["PLAN.md"]) {
		fmt.Fprintf(&evidence, "# Standing plan\n\n%s\n", d.Files["PLAN.md"])
	}
	if len(input) > 0 {
		evidence.WriteString("\n# Piped input\n\n")
		evidence.Write(input)
		evidence.WriteByte('\n')
	}
	if evidence.Len() > inputLimit {
		return problem(fmt.Errorf("compiled piped evidence exceeds %d bytes", inputLimit), 2)
	}
	// All input and path validation precedes changes to caller-owned roots.
	for _, path := range d.runtimeDirectories() {
		if err := makeDir(path); err != nil {
			return problem(err, 2)
		}
	}
	model := o.model
	if model == "" {
		model = os.Getenv("ASK_MODEL")
	}
	if o.effort == "" && nested > 0 {
		o.effort = os.Getenv("PLY_EFFORT")
	}
	set := map[string]string{
		"AGENT_BIN": self, "AGENT_HOME": d.Home, "AGENT_WORK": d.Work, "AGENT_STATE": d.State, "AGENT_ACTION_TMP": actionTmp,
		"ASK": ask, "BRIEF": brief, "BRIEF_PATH": filepath.Join(d.Home, "skills"),
		"PLY_DIR": filepath.Join(d.Control, "runs"), "BRIEF_DIR": filepath.Join(d.Control, "selections"),
		"BRIEF_MODEL": model, "BRIEF_EFFORT": o.effort,
	}
	if cage != "" {
		set["AGENT_CAGE"] = cage
		set["AGENT_NET"] = "0"
		if o.network {
			set["AGENT_NET"] = "1"
		}
	}
	// Scrub controller-only external-effect capabilities and private transport
	// variables. Preserve Ply's inherited gate and depth: recursion cannot
	// turn an approved action boundary back into an unrestricted root.
	env := withEnv(os.Environ(), set, "RUN_KIND", "RUN_WAKE_OUTPUT", "RUN_INPUT", "RUN_STDIN_FILE", "AGENT_RUNTIME",
		"AGENT_MAY", "BENCH_MAY", "AGENT_ACTION", "AGENT_ACTION_PATH", "AGENT_ACTION_POLICY", "ACTION_PATH", "ACTION_POLICY", "ACTION_MAY", "ACTION_ASK",
		"PLY_TOOLS", "ASK_MODEL", "ASK_SYSTEM", "ASK_DIR")
	argv := []string{"-sh", "-shell", "/bin/sh", "-no-delegate", "-C", d.Work, "-check", shellQuote(filepath.Join(d.Home, "bin/check")), "-goal-file", goal}
	argv = append(argv, "-record", record, "-record-dir", filepath.Join(d.Control, "recordings"),
		"-record-input", goal, "-record-input", filepath.Join(skill, "SKILL.md"),
		"-record-input", filepath.Join(d.Home, "bin/check"))
	if d.Tools != "" {
		argv = append(argv, "-t", d.Tools)
	}
	if d.Skills != "" {
		argv = append(argv, "-s", "-")
	}
	argv = append(argv, "-s", skill)
	if model != "" {
		argv = append(argv, "-m", model)
	}
	argv = append(argv, "-effort", o.effort)
	if checkpoint != "" {
		argv = append(argv, "-checkpoint", checkpoint)
	}
	if o.steer != "" {
		argv = append(argv, "-steer", o.steer)
	}
	if o.quiet {
		argv = append(argv, "-q")
	}
	if !o.noCage {
		argv = append(argv, "-action-shell", self, "-action-boundary-exit", "125")
	} else {
		action := "/bin/sh"
		if nested > 0 && os.Getenv("PLY_ACTION_SHELL") != "" {
			action = os.Getenv("PLY_ACTION_SHELL")
		}
		argv = append(argv, "-action-shell", action)
	}
	argv = append(argv, o.forward...)
	if !o.quiet {
		fmt.Fprintf(os.Stderr, "agent: work=%s · state=%s · evidence=%s\n", d.Work, d.State, d.Control)
		if o.noCage {
			fmt.Fprintln(os.Stderr, "agent: Cage is disabled; ordinary host write and network reach")
		} else {
			fmt.Fprintf(os.Stderr, "agent: authority: Cage writes work+state; network=%t; host reads unrestricted\n", o.network)
		}
	}
	return execute(ply, argv, env, &evidence, os.Stdout, os.Stderr, "")
}

func focus(s string) string {
	if s == "" {
		return ""
	}
	return "\n\nInvocation focus:\n" + s
}

func readInput() ([]byte, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(os.Stdin, inputLimit+1))
	if err != nil {
		return nil, err
	}
	if len(b) > inputLimit {
		return nil, fmt.Errorf("piped input exceeds its bound; the limit is %d bytes", inputLimit)
	}
	return b, nil
}

func makeDir(path string) error {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return writable(path)
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return writable(path)
}

func checkPly(path string, confined bool) error {
	b, code, err := boundedOutput(path, []string{"capabilities"}, os.Environ(), "", fileLimit)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("cannot inspect Ply capabilities: exit %d", code)
	}
	var c struct {
		Schema   string `json:"schema"`
		Features struct {
			GoalFile       bool   `json:"goal_file"`
			NoDelegate     bool   `json:"no_delegate"`
			ActionBoundary string `json:"action_boundary_receipt"`
			Recording      string `json:"process_recording"`
		} `json:"features"`
	}
	if err := json.Unmarshal(b, &c); err != nil || c.Schema != "ply.capabilities/v1" {
		return fmt.Errorf("Ply is incompatible: needs ply.capabilities/v1")
	}
	if !c.Features.GoalFile || !c.Features.NoDelegate {
		return fmt.Errorf("Ply is incompatible: needs goal_file and no_delegate")
	}
	if c.Features.Recording != "ply.recording/v1" {
		return fmt.Errorf("Ply is incompatible: needs ply.recording/v1 process recording")
	}
	if confined && c.Features.ActionBoundary != "ply.action-boundary/v1" {
		return fmt.Errorf("Ply is incompatible: needs ply.action-boundary/v1")
	}
	return nil
}

func checkCheckpoint(path, runs string) error {
	b, err := readRegular(path, 16384)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(b) == 0 || b[len(b)-1] != '\n' || bytes.Count(b, []byte{'\n'}) != 1 || bytes.ContainsAny(b, "\r\x00") {
		return fmt.Errorf("checkpoint must contain one newline-terminated session path")
	}
	session := string(b[:len(b)-1])
	if !filepath.IsAbs(session) {
		return fmt.Errorf("checkpoint session must be absolute")
	}
	parent, err := realDir(filepath.Dir(session))
	if err != nil {
		return err
	}
	session = filepath.Join(parent, filepath.Base(session))
	if !inside(runs, session) {
		return fmt.Errorf("checkpoint session is outside this home's run evidence")
	}
	if info, err := os.Lstat(session); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("checkpoint session is not a regular file")
		}
	}
	return nil
}

// A parent with a custom action interpreter owns a boundary whose parameters
// Agent cannot safely rebind to a new workspace. Refuse that composition;
// ordinary shell children keep Ply's existing depth and approval gate.
func inheritedBoundary() (int, error) {
	depth := os.Getenv("PLY_DEPTH")
	if depth == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(depth)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid inherited PLY_DEPTH")
	}
	if n > 0 && os.Getenv("PLY_ACTION_SHELL") != "" {
		parent, err := filepath.EvalSymlinks(os.Getenv("PLY_ACTION_SHELL"))
		shell, shellErr := filepath.EvalSymlinks("/bin/sh")
		if err != nil || shellErr != nil || parent != shell {
			return 0, fmt.Errorf("nested Agent needs an ordinary shell boundary; the inherited action interpreter cannot be rebound")
		}
	}
	return n, nil
}
