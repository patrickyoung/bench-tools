package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	approvalReceiptKind    = "ply.approval/v1"
	approvalReceiptKindV2  = "ply.approval/v2"
	maxApprovalResult      = 128 << 10 // enough for May's maximally escaped 16 KiB action
	maxMayJob              = 1024
	maxMayAction           = 16 << 10
	approvalRequestTimeout = 10 * time.Second
	approvalRecordTimeout  = 10 * time.Second
)

var (
	ErrApprovalParked   = errors.New("approval required")
	ErrApprovalDeclined = errors.New("approval declined")
	ErrApprovalBoundary = errors.New("approval boundary failed")
)

// approvalAction is the exact, human-reviewable envelope May binds. The
// script is the same string Runner would otherwise pass to shell -c.
type approvalAction struct {
	Version     int                  `json:"version"`
	ContractID  string               `json:"contract_id,omitempty"`
	Directory   string               `json:"directory"`
	Shell       string               `json:"shell"`
	Path        string               `json:"path"`
	TimeoutNS   int64                `json:"timeout_ns"`
	Script      string               `json:"script"`
	Confinement *approvalConfinement `json:"confinement,omitempty"`
}

type approvalConfinement struct {
	Kind       string   `json:"kind"`
	CagePath   string   `json:"cage_path"`
	CageSHA256 string   `json:"cage_sha256"`
	Argv       []string `json:"argv"`
	Workspace  string   `json:"workspace"`
	TempDir    string   `json:"temp_dir"`
	Network    bool     `json:"network"`
}

type mayResult struct {
	Version int    `json:"version"`
	Job     string `json:"job"`
	Digest  string `json:"digest"`
	Action  string `json:"action"`
	Verdict string `json:"verdict"`
}

type approvalReceipt struct {
	Version         int            `json:"version"`
	ContractID      string         `json:"contract_id,omitempty"`
	Job             string         `json:"job"`
	Digest          string         `json:"digest"`
	Action          approvalAction `json:"action"`
	ActionSHA256    string         `json:"action_sha256"`
	Verdict         string         `json:"verdict"`
	MayPath         string         `json:"may_path"`
	MaySHA256       string         `json:"may_sha256"`
	MayArgv         []string       `json:"may_argv"`
	MayInputSHA256  string         `json:"may_input_sha256"`
	MayStdoutSHA256 string         `json:"may_stdout_sha256"`
	MayStdoutBytes  int64          `json:"may_stdout_bytes"`
	MayExitCode     int            `json:"may_exit_code"`
}

type mayGate struct {
	Bin       string
	BinSHA256 string
	Job       string
}

func openMayGate(bin, job string) (*mayGate, error) {
	if err := validateMayJob(job); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(bin)
	if err != nil {
		return nil, fmt.Errorf("resolve May: %w", err)
	}
	digest, err := mayExecutableDigest(path)
	if err != nil {
		return nil, err
	}
	return &mayGate{Bin: path, BinSHA256: digest, Job: job}, nil
}

func mayExecutableDigest(path string) (string, error) {
	return executableDigest("May", path)
}

func validateMayJob(job string) error {
	switch {
	case strings.TrimSpace(job) == "":
		return errors.New("-may-job: empty job")
	case !utf8.ValidString(job) || strings.IndexByte(job, 0) >= 0:
		return errors.New("-may-job: job must be valid UTF-8 without NUL bytes")
	case len(job) > maxMayJob:
		return fmt.Errorf("-may-job: job exceeds %d bytes", maxMayJob)
	}
	return nil
}

func (g mayGate) Request(ctx context.Context, contractID, script string, runner Runner) (approvalReceipt, error) {
	for _, field := range []struct {
		name, value string
		allowEmpty  bool
	}{
		{name: "contract ID", value: contractID, allowEmpty: true},
		{name: "working directory", value: runner.Dir},
		{name: "shell", value: runner.Shell},
		{name: "PATH", value: runner.Path, allowEmpty: true},
		{name: "script", value: script},
	} {
		if err := validateApprovalText(field.name, field.value, field.allowEmpty); err != nil {
			return approvalReceipt{}, err
		}
	}
	if runner.Cage != nil {
		for _, field := range []struct{ name, value string }{
			{"Cage path", runner.Cage.Bin},
			{"Cage digest", runner.Cage.BinSHA256},
			{"Cage workspace", runner.Cage.Workspace},
			{"Cage temporary directory", runner.Cage.TempDir},
		} {
			if err := validateApprovalText(field.name, field.value, false); err != nil {
				return approvalReceipt{}, err
			}
		}
		for i, arg := range runner.Cage.argv(runner.Shell, script) {
			if err := validateApprovalText(fmt.Sprintf("Cage argv[%d]", i), arg, false); err != nil {
				return approvalReceipt{}, err
			}
		}
		if err := runner.Cage.checkDigest(); err != nil {
			return approvalReceipt{}, err
		}
	}
	before, err := mayExecutableDigest(g.Bin)
	if err != nil {
		return approvalReceipt{}, err
	}
	if before != g.BinSHA256 {
		return approvalReceipt{}, errors.New("May executable changed after the approval gate was opened")
	}
	action := approvalActionFor(contractID, script, runner)
	body, err := json.Marshal(action)
	if err != nil {
		return approvalReceipt{}, fmt.Errorf("encode approval action: %w", err)
	}
	body = append(body, '\n')
	if len(body) > maxMayAction {
		return approvalReceipt{}, fmt.Errorf("May approval action exceeds %d bytes", maxMayAction)
	}

	stdout := &capBuf{cap: maxApprovalResult / 2}
	stderr := &capBuf{cap: maxApprovalResult / 2}
	argv := []string{g.Bin, "request", g.Job}
	requestCtx, cancel := context.WithTimeout(ctx, approvalRequestTimeout)
	defer cancel()
	cmd := exec.CommandContext(requestCtx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(body)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err = cmd.Run()
	if requestCtx.Err() != nil {
		return approvalReceipt{}, requestCtx.Err()
	}
	after, hashErr := mayExecutableDigest(g.Bin)
	if hashErr != nil {
		return approvalReceipt{}, hashErr
	}
	if after != before {
		return approvalReceipt{}, errors.New("May executable changed during the approval request")
	}
	code, codeErr := processCode(err)
	if codeErr != nil {
		return approvalReceipt{}, fmt.Errorf("run May approval gate: %w", codeErr)
	}
	out, outElided, outBytes := stdout.String()
	errout, errElided, _ := stderr.String()
	if outElided > 0 || errElided > 0 || outBytes > maxApprovalResult {
		return approvalReceipt{}, fmt.Errorf("May approval result exceeds %d bytes", maxApprovalResult)
	}
	if code != 0 && code != 3 && code != 75 {
		return approvalReceipt{}, fmt.Errorf("May approval gate exited %d: %s", code, firstLine(errout))
	}
	var result mayResult
	dec := json.NewDecoder(strings.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return approvalReceipt{}, fmt.Errorf("decode May approval result: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return approvalReceipt{}, errors.New("decode May approval result: trailing JSON")
		}
		return approvalReceipt{}, fmt.Errorf("decode May approval result: %w", err)
	}
	wantVerdict := map[int]string{0: "spent", 3: "declined", 75: "parked"}[code]
	if result.Version != 1 || result.Job != g.Job || result.Action != string(body) ||
		result.Digest != mayDigest(g.Job, string(body)) || result.Verdict != wantVerdict {
		return approvalReceipt{}, errors.New("May approval result does not match the exact action and exit status")
	}
	canonical, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return approvalReceipt{}, fmt.Errorf("encode May approval result: %w", marshalErr)
	}
	canonical = append(canonical, '\n')
	if out != string(canonical) || errout != "" {
		return approvalReceipt{}, errors.New("May approval result was not one canonical JSON line on stdout with empty stderr")
	}
	return approvalReceipt{
		Version: action.Version, ContractID: contractID, Job: g.Job, Digest: result.Digest,
		Action: action, ActionSHA256: digestText(string(body)), Verdict: result.Verdict,
		MayPath: g.Bin, MaySHA256: g.BinSHA256, MayArgv: argv,
		MayInputSHA256: digestText(string(body)), MayStdoutSHA256: digestText(out),
		MayStdoutBytes: outBytes, MayExitCode: code,
	}, nil
}

func validateApprovalText(name, value string, allowEmpty bool) error {
	switch {
	case value == "" && !allowEmpty:
		return fmt.Errorf("approval %s is empty", name)
	case !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0:
		return fmt.Errorf("approval %s must be valid UTF-8 without NUL bytes", name)
	}
	return nil
}

func approvalActionFor(contractID, script string, runner Runner) approvalAction {
	action := approvalAction{
		Version: 1, ContractID: contractID, Directory: runner.Dir, Shell: runner.Shell,
		Path: runner.Path, TimeoutNS: int64(runner.Timeout), Script: script,
	}
	if runner.Cage != nil {
		action.Version = 2
		action.Confinement = &approvalConfinement{
			Kind: "cage", CagePath: runner.Cage.Bin, CageSHA256: runner.Cage.BinSHA256,
			Argv: runner.Cage.argv(runner.Shell, script), Workspace: runner.Cage.Workspace,
			TempDir: runner.Cage.TempDir, Network: false,
		}
	}
	return action
}

func processCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if code := exit.ExitCode(); code >= 0 {
			return code, nil
		}
		return 0, fmt.Errorf("May approval gate was killed")
	}
	return 0, err
}

func mayDigest(job, action string) string {
	sum := sha256.Sum256([]byte("may-v1\x00" + job + "\x00" + action))
	return fmt.Sprintf("%x", sum)
}

func (l *Loop) recordApproval(ctx context.Context, receipt approvalReceipt) error {
	if l.Model.Session == "" {
		return errors.New("record approval receipt: no Ask session")
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), approvalRecordTimeout)
	defer cancel()
	kind := approvalReceiptKind
	if receipt.Action.Version == 2 {
		kind = approvalReceiptKindV2
	}
	if err := l.Model.Record(recordCtx, verdictSource, kind, receipt); err != nil {
		return fmt.Errorf("record approval receipt: %w", err)
	}
	return nil
}
