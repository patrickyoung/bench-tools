package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type processResult struct {
	Name    string   `json:"name"`
	Program string   `json:"program"`
	Args    []string `json:"args"`
	Exit    int      `json:"exit"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
}

func resolvePrograms(action, kind string) (map[string]string, error) {
	names := []string{"hire", "agent", "brief", "ask", "record", "cage"}
	if action == "train" && kind == "recovery" {
		names = append(names, "hone")
	}
	programs := map[string]string{}
	for _, name := range names {
		choice := name
		if selector := map[string]string{"brief": "AGENT_BRIEF", "record": "AGENT_RECORD", "cage": "AGENT_CAGE"}[name]; selector != "" && os.Getenv(selector) != "" {
			choice = os.Getenv(selector)
		}
		if name == "brief" && os.Getenv("AGENT_BRIEF") == "" && os.Getenv("BRIEF") != "" {
			choice = os.Getenv("BRIEF")
		}
		if name == "agent" && os.Getenv("HIRE_AGENT") != "" {
			choice = os.Getenv("HIRE_AGENT")
		}
		if name == "ask" {
			choice = os.Getenv("AGENT_ASK")
			if choice == "" {
				choice = os.Getenv("ASK")
			}
			if choice == "" {
				choice = "ask"
			}
		}
		p, err := exec.LookPath(choice)
		if err != nil {
			return nil, err
		}
		p, err = filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		p, err = filepath.EvalSymlinks(p)
		if err != nil {
			return nil, err
		}
		programs[name] = p
	}
	return programs, nil
}

// Freeze relative command search paths against the caller's cwd before any
// child changes directory. Preserve approved wrappers as literal executables.
func stableEnvironment(base []string) ([]string, error) {
	values := map[string]string{}
	for _, item := range base {
		key, value, _ := strings.Cut(item, "=")
		if key == "PATH" {
			parts := filepath.SplitList(value)
			for i, part := range parts {
				if part == "" {
					part = "."
				}
				abs, err := filepath.Abs(part)
				if err != nil {
					return nil, err
				}
				parts[i] = abs
			}
			values[key] = strings.Join(parts, string(os.PathListSeparator))
		}
		if strings.HasPrefix(key, "AGENT_") && (key == "AGENT_ASK" || key == "AGENT_BRIEF" || key == "AGENT_PLY" || key == "AGENT_CAGE" || key == "AGENT_RECORD") && strings.ContainsRune(value, filepath.Separator) && !filepath.IsAbs(value) {
			abs, err := filepath.Abs(value)
			if err != nil {
				return nil, err
			}
			values[key] = abs
		}
	}
	return replaceEnv(base, values), nil
}

func replaceEnv(base []string, values map[string]string) []string {
	var env []string
	for _, item := range base {
		name, _, _ := strings.Cut(item, "=")
		if _, ok := values[name]; !ok {
			env = append(env, item)
		}
	}
	for name, value := range values {
		env = append(env, name+"="+value)
	}
	return env
}

func (r *run) invoke(label, program string, args []string, cwd string, input []byte, extra map[string]string) int {
	if r.ctx.Err() != nil {
		return 130
	}
	bin := r.programs[program]
	if bin == "" {
		return r.fail(fmt.Errorf("unselected program %s", program), 127)
	}
	dir := filepath.Join(r.root, "records", label)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return r.fail(err, 1)
	}
	outPath, errPath := filepath.Join(dir, "stdout"), filepath.Join(dir, "stderr")
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return r.fail(err, 1)
	}
	defer out.Close()
	diag, err := os.OpenFile(errPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return r.fail(err, 1)
	}
	defer diag.Close()
	cmd := exec.Command(bin, args...)
	cmd.Dir = cwd
	selected := map[string]string{"HIRE_AGENT": r.programs["agent"], "AGENT_ASK": r.programs["ask"]}
	for _, name := range []string{"brief", "record", "cage"} {
		if r.programs[name] != "" {
			selected["AGENT_"+strings.ToUpper(name)] = r.programs[name]
		}
	}
	env := replaceEnv(r.env, selected)
	if extra != nil {
		env = replaceEnv(env, extra)
	}
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = out
	cmd.Stderr = io.MultiWriter(diag, os.Stderr)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	fmt.Fprintf(os.Stderr, "trait: %s\n", label)
	code := 0
	if err = cmd.Start(); err != nil {
		code = 126
		fmt.Fprintln(diag, err)
	} else {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		timer := time.NewTimer(r.config.timeout)
		select {
		case err = <-done:
			code = processExit(err)
		case <-r.ctx.Done():
			code = stopProcess(cmd, done, 130)
		case <-timer.C:
			code = stopProcess(cmd, done, 124)
		}
		timer.Stop()
	}
	if err := errors.Join(out.Sync(), diag.Sync()); err != nil {
		return r.fail(err, 125)
	}
	r.result.Processes = append(r.result.Processes, processResult{Name: label, Program: bin, Args: args, Exit: code, Stdout: outPath, Stderr: errPath})
	return code
}

func stopProcess(cmd *exec.Cmd, done <-chan error, code int) int {
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
	}
	return code
}

func processExit(err error) int {
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
	return 1
}

func (r *run) stdout(label string) ([]byte, error) {
	f, err := os.Open(filepath.Join(r.root, "records", label, "stdout"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 4<<20+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 4<<20 {
		return nil, errors.New("review evidence exceeds 4 MiB")
	}
	return b, nil
}
