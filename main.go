// Command action controls one exact effectful connector invocation.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	version       = "0.1.0"
	maxInput      = 8 << 10
	maxDescriptor = 64 << 10
	maxPolicy     = 64 << 10
	maxMay        = 128 << 10
	maxOutput     = 1 << 20
)

var portableName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type descriptor struct {
	Version     int             `json:"version"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// requestFile is the deliberately small, portable format an agent may prepare
// without gaining access to the connector itself.
type requestFile struct {
	Version   int             `json:"version"`
	Connector string          `json:"connector"`
	Input     json.RawMessage `json:"input"`
}

type proposal struct {
	Version          int             `json:"version"`
	Connector        string          `json:"connector"`
	ConnectorPath    string          `json:"connector_path"`
	ConnectorSHA256  string          `json:"connector_sha256"`
	DescriptorSHA256 string          `json:"descriptor_sha256"`
	Description      string          `json:"description"`
	Directory        string          `json:"directory"`
	Input            json.RawMessage `json:"input"`
	InputSHA256      string          `json:"input_sha256"`
	ActionSHA256     string          `json:"action_sha256"`
	PolicyPath       string          `json:"policy_path,omitempty"`
	PolicySHA256     string          `json:"policy_sha256,omitempty"`
	MayPath          string          `json:"may_path,omitempty"`
	MaySHA256        string          `json:"may_sha256,omitempty"`
	Job              string          `json:"job"`
}

type actionEnvelope struct {
	Version          int             `json:"version"`
	Job              string          `json:"job"`
	Connector        string          `json:"connector"`
	ConnectorPath    string          `json:"connector_path"`
	ConnectorSHA256  string          `json:"connector_sha256"`
	DescriptorSHA256 string          `json:"descriptor_sha256"`
	Description      string          `json:"description"`
	Directory        string          `json:"directory"`
	Input            json.RawMessage `json:"input"`
}

type policyResult struct {
	Version      int    `json:"version"`
	ActionSHA256 string `json:"action_sha256"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
}

type mayResult struct {
	Version int    `json:"version"`
	Job     string `json:"job"`
	Digest  string `json:"digest"`
	Action  string `json:"action"`
	Verdict string `json:"verdict"`
}

type decisionReceipt struct {
	Version         int           `json:"version"`
	ProposalSHA256  string        `json:"proposal_sha256"`
	ActionSHA256    string        `json:"action_sha256"`
	Decision        string        `json:"decision"`
	Reason          string        `json:"reason"`
	PolicyExitCode  int           `json:"policy_exit_code,omitempty"`
	PolicyResult    *policyResult `json:"policy_result,omitempty"`
	PolicyResultSHA string        `json:"policy_result_sha256,omitempty"`
	MayPath         string        `json:"may_path,omitempty"`
	MaySHA256       string        `json:"may_sha256,omitempty"`
	MayExitCode     int           `json:"may_exit_code,omitempty"`
	MayResult       *mayResult    `json:"may_result,omitempty"`
	MayResultSHA256 string        `json:"may_result_sha256,omitempty"`
	Authorized      bool          `json:"authorized"`
	RequestReleased bool          `json:"request_released"`
}

type attemptReceipt struct {
	Version         int    `json:"version"`
	DecisionSHA256  string `json:"decision_sha256"`
	ActionSHA256    string `json:"action_sha256"`
	ConnectorPath   string `json:"connector_path"`
	ConnectorSHA256 string `json:"connector_sha256"`
	PID             int    `json:"pid"`
	RequestReleased bool   `json:"request_released"`
}

type sentReceipt struct {
	Version       int    `json:"version"`
	AttemptSHA256 string `json:"attempt_sha256"`
	ActionSHA256  string `json:"action_sha256"`
	InputSHA256   string `json:"input_sha256"`
	InputBytes    int    `json:"input_bytes"`
	Sent          bool   `json:"sent"`
}

type resultReceipt struct {
	Version      int    `json:"version"`
	SentSHA256   string `json:"sent_sha256"`
	ActionSHA256 string `json:"action_sha256"`
	Sent         bool   `json:"sent"`
	ExitCode     int    `json:"exit_code"`
	Outcome      string `json:"outcome"`
	Stdout       []byte `json:"stdout,omitempty"`
	StdoutSHA256 string `json:"stdout_sha256"`
	StdoutBytes  int    `json:"stdout_bytes"`
	Stderr       []byte `json:"stderr,omitempty"`
	StderrSHA256 string `json:"stderr_sha256"`
	StderrBytes  int    `json:"stderr_bytes"`
}

type recorder struct {
	ask     string
	session string
}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "--help", "-h":
		usage(stdout)
		return 0
	case "version", "--version", "-V":
		fmt.Fprintln(stdout, "action "+version)
		return 0
	case "ls":
		return listConnectors(stdout, stderr)
	case "show":
		if len(args) != 2 {
			usage(stderr)
			return 2
		}
		return showConnector(args[1], stdout, stderr)
	case "inspect":
		if len(args) != 2 {
			usage(stderr)
			return 2
		}
		return inspectRequest(args[1], stdout, stderr)
	case "check":
		if len(args) != 2 {
			usage(stderr)
			return 2
		}
		return checkRequest(args[1], stderr)
	case "run":
		return runAction(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "action: unknown command %q\n", args[0])
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `usage:
  action ls
  action show NAME
  action inspect PROPOSAL.json
  action check PROPOSAL.json
  action run -job JOB [-policy PROGRAM] [-may PROGRAM] [-record SESSION] NAME
  action run -job JOB [-policy PROGRAM] [-may PROGRAM] [-record SESSION] -proposal PROPOSAL.json
  action help
  action version

Connectors are executables on ACTION_PATH and support describe and run.
run reads one JSON object on stdin. Without -policy every action goes to May.
Exit 3 is declined, 75 is parked, and 125 means effects may exist without a
trustworthy terminal receipt.`)
}

func runAction(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	job := fs.String("job", "", "stable May job name")
	policyName := fs.String("policy", os.Getenv("ACTION_POLICY"), "deterministic policy executable")
	mayName := fs.String("may", envDefault("ACTION_MAY", "may"), "May executable")
	record := fs.String("record", "", "Ask session receiving sealed action events")
	askName := fs.String("ask", envDefault("ACTION_ASK", "ask"), "Ask executable used with -record")
	timeout := fs.Duration("timeout", 0, "connector deadline (zero means none)")
	proposalFile := fs.String("proposal", "", "strict action proposal JSON file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !utf8.ValidString(*job) || strings.TrimSpace(*job) == "" || len(*job) > 1024 {
		fmt.Fprintln(stderr, "action: run needs -job JOB")
		return 2
	}
	name, input, err := selectRequest(fs.Args(), *proposalFile, stdin)
	if err != nil {
		return diagnose(stderr, err)
	}
	connector, err := resolveOnPath(name, os.Getenv("ACTION_PATH"))
	if err != nil {
		return diagnose(stderr, err)
	}
	connectorHash, err := fileDigest(connector)
	if err != nil {
		return diagnose(stderr, err)
	}
	descRaw, desc, err := readDescriptor(connector)
	if err != nil {
		return diagnose(stderr, err)
	}
	if desc.Name != name {
		return diagnose(stderr, fmt.Errorf("connector descriptor name %q does not match %q", desc.Name, name))
	}
	dir, err := filepath.EvalSymlinks(mustAbs("."))
	if err != nil {
		return diagnose(stderr, fmt.Errorf("physical directory: %w", err))
	}
	policyPath, policyHash := "", ""
	if *policyName != "" {
		policyPath, err = exec.LookPath(*policyName)
		if err != nil {
			return diagnose(stderr, fmt.Errorf("policy: %w", err))
		}
		policyPath = mustAbs(policyPath)
		policyHash, err = fileDigest(policyPath)
		if err != nil {
			return diagnose(stderr, err)
		}
	}
	envelope := actionEnvelope{1, *job, name, connector, connectorHash, digest(descRaw), desc.Description, dir, input}
	actionBytes, err := canonicalLine(envelope)
	if err != nil {
		return diagnose(stderr, err)
	}
	if len(actionBytes) > 16<<10 {
		return diagnose(stderr, errors.New("canonical action exceeds May's 16 KiB limit"))
	}
	prop := proposal{1, name, connector, connectorHash, digest(descRaw), desc.Description, dir, input,
		digest(input), digest(actionBytes), policyPath, policyHash, "", "", *job}
	rec := recorder{}
	if *record != "" {
		askPath, lookErr := exec.LookPath(*askName)
		if lookErr != nil {
			return diagnose(stderr, fmt.Errorf("Ask: %w", lookErr))
		}
		rec = recorder{mustAbs(askPath), mustAbs(*record)}
		if err := rec.note("action.proposal/v1", prop); err != nil {
			return diagnose(stderr, fmt.Errorf("record proposal: %w", err))
		}
	}
	propHash := bodyDigest(prop)
	decision, code, err := authorize(actionBytes, propHash, policyPath, policyHash, *mayName, *job)
	if err != nil {
		return diagnose(stderr, err)
	}
	if err := rec.note("action.decision/v1", decision); err != nil {
		return diagnose(stderr, fmt.Errorf("record decision: %w", err))
	}
	if code != 0 {
		return code
	}
	if err := revalidate(connector, connectorHash, descRaw); err != nil {
		return diagnose(stderr, err)
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	var cancel context.CancelFunc
	if *timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, connector, "run")
	cmd.Dir = dir
	inR, inW, err := os.Pipe()
	if err != nil {
		return diagnose(stderr, err)
	}
	cmd.Stdin = inR
	outCap, errCap := &boundedBuffer{limit: maxOutput}, &boundedBuffer{limit: maxOutput}
	cmd.Stdout, cmd.Stderr = outCap, errCap
	if err := cmd.Start(); err != nil {
		inR.Close()
		inW.Close()
		if errors.Is(ctx.Err(), context.Canceled) {
			fmt.Fprintln(stderr, "action: interrupted before request release")
			return 130
		}
		return diagnose(stderr, fmt.Errorf("start connector: %w", err))
	}
	inR.Close()
	attempt := attemptReceipt{1, bodyDigest(decision), prop.ActionSHA256, connector, connectorHash, cmd.Process.Pid, false}
	if err := rec.note("action.attempt/v1", attempt); err != nil {
		inW.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return diagnose(stderr, fmt.Errorf("record prepared attempt before request release: %w", err))
	}
	if ctx.Err() != nil {
		inW.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if errors.Is(ctx.Err(), context.Canceled) {
			fmt.Fprintln(stderr, "action: interrupted before request release")
			return 130
		}
		return diagnose(stderr, fmt.Errorf("connector deadline before request release: %w", ctx.Err()))
	}
	if _, err := inW.Write(input); err != nil {
		inW.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return uncertain(stderr, "release request", err)
	}
	if err := inW.Close(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return uncertain(stderr, "close request", err)
	}
	sent := sentReceipt{1, bodyDigest(attempt), prop.ActionSHA256, digest(input), len(input), true}
	if err := rec.note("action.sent/v1", sent); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return uncertain(stderr, "record sent boundary", err)
	}
	waitErr := cmd.Wait()
	if outCap.overflow || errCap.overflow {
		return uncertain(stderr, "connector output", errors.New("configured output bound exceeded"))
	}
	exitCode, terminal := processExit(waitErr)
	if !terminal || ctx.Err() != nil {
		return uncertain(stderr, "connector terminal result", firstError(ctx.Err(), waitErr))
	}
	if exitCode != 0 && exitCode != 1 && exitCode != 2 && exitCode != 75 && exitCode != 125 {
		return uncertain(stderr, "connector terminal status", fmt.Errorf("unsupported exit %d", exitCode))
	}
	if exitCode == 125 {
		fmt.Fprint(stderr, string(errCap.buf))
		return 125
	}
	if exitCode == 0 || exitCode == 1 || exitCode == 75 {
		if _, err := readCanonicalObject(bytes.NewReader(outCap.buf), maxOutput); err != nil {
			return uncertain(stderr, "connector result", err)
		}
	}
	result := resultReceipt{1, bodyDigest(sent), prop.ActionSHA256, true, exitCode, outcome(exitCode),
		append([]byte(nil), outCap.buf...), digest(outCap.buf), len(outCap.buf), append([]byte(nil), errCap.buf...), digest(errCap.buf), len(errCap.buf)}
	if err := rec.note("action.result/v1", result); err != nil {
		return uncertain(stderr, "record terminal result", err)
	}
	fmt.Fprint(stderr, string(errCap.buf))
	if len(outCap.buf) > 0 {
		_, _ = stdout.Write(outCap.buf)
	}
	return exitCode
}

func selectRequest(args []string, proposalPath string, stdin io.Reader) (string, []byte, error) {
	if proposalPath == "" {
		if len(args) != 1 || !portableName.MatchString(args[0]) {
			return "", nil, errors.New("run needs one portable connector NAME or -proposal FILE")
		}
		input, err := readCanonicalObject(stdin, maxInput)
		return args[0], input, err
	}
	if len(args) != 0 {
		return "", nil, errors.New("connector NAME and -proposal are mutually exclusive")
	}
	raw, err := os.ReadFile(proposalPath)
	if err != nil {
		return "", nil, fmt.Errorf("read proposal: %w", err)
	}
	if len(raw) > maxInput+1024 {
		return "", nil, errors.New("proposal exceeds size limit")
	}
	var request requestFile
	if err := decodeStrict(raw, &request); err != nil {
		return "", nil, fmt.Errorf("proposal: %w", err)
	}
	if request.Version != 1 || !portableName.MatchString(request.Connector) {
		return "", nil, errors.New("proposal needs version 1 and a portable connector name")
	}
	input, err := canonicalObject(request.Input)
	if err != nil {
		return "", nil, fmt.Errorf("proposal input: %w", err)
	}
	if len(input) > maxInput {
		return "", nil, errors.New("proposal input exceeds size limit")
	}
	return request.Connector, input, nil
}

func inspectRequest(path string, stdout, stderr io.Writer) int {
	name, input, err := selectRequest(nil, path, bytes.NewReader(nil))
	if err != nil {
		return diagnose(stderr, err)
	}
	raw, err := canonicalLine(requestFile{Version: 1, Connector: name, Input: input})
	if err != nil {
		return diagnose(stderr, err)
	}
	_, _ = stdout.Write(raw)
	return 0
}

func checkRequest(path string, stderr io.Writer) int {
	_, _, err := selectRequest(nil, path, bytes.NewReader(nil))
	if err != nil {
		return diagnose(stderr, err)
	}
	return 0
}

func authorize(action []byte, proposalHash, policyPath, policyHash, mayName, job string) (decisionReceipt, int, error) {
	r := decisionReceipt{Version: 1, ProposalSHA256: proposalHash, ActionSHA256: digest(action), RequestReleased: false}
	decision, reason := "review", "no policy selected"
	if policyPath != "" {
		out, errout, code, err := runBounded(policyPath, []string{}, action, maxPolicy)
		if err != nil {
			return r, 2, fmt.Errorf("run policy: %w", err)
		}
		if code != 0 && code != 3 && code != 75 {
			return r, 2, fmt.Errorf("policy exited %d: %s", code, firstLine(errout))
		}
		var pr policyResult
		if err := decodeStrictLine(out, &pr); err != nil {
			return r, 2, fmt.Errorf("policy result: %w", err)
		}
		want := map[int]string{0: "allow", 3: "deny", 75: "review"}[code]
		if pr.Version != 1 || pr.ActionSHA256 != digest(action) || pr.Decision != want || strings.TrimSpace(pr.Reason) == "" || len(pr.Reason) > 2048 || errout != "" {
			return r, 2, errors.New("policy result does not match action, exit status, or stream contract")
		}
		if canonical, _ := canonicalLine(pr); out != string(canonical) {
			return r, 2, errors.New("policy result was not canonical JSON")
		}
		if got, err := fileDigest(policyPath); err != nil || got != policyHash {
			return r, 2, errors.New("policy executable changed during decision")
		}
		r.PolicyExitCode, r.PolicyResult, r.PolicyResultSHA = code, &pr, digest([]byte(out))
		decision, reason = pr.Decision, pr.Reason
	}
	r.Decision, r.Reason = decision, reason
	if decision == "deny" {
		return r, 3, nil
	}
	if decision == "allow" {
		r.Authorized = true
		return r, 0, nil
	}
	mayPath, err := exec.LookPath(mayName)
	if err != nil {
		return r, 2, fmt.Errorf("May: %w", err)
	}
	mayPath = mustAbs(mayPath)
	mayHash, err := fileDigest(mayPath)
	if err != nil {
		return r, 2, err
	}
	r.MayPath, r.MaySHA256 = mayPath, mayHash
	out, errout, code, err := runBounded(mayPath, []string{"request", job}, action, maxMay)
	if err != nil {
		return r, 2, fmt.Errorf("run May: %w", err)
	}
	if code != 0 && code != 3 && code != 75 {
		return r, 2, fmt.Errorf("May exited %d: %s", code, firstLine(errout))
	}
	var mr mayResult
	if err := decodeStrictLine(out, &mr); err != nil {
		return r, 2, fmt.Errorf("May result: %w", err)
	}
	want := map[int]string{0: "spent", 3: "declined", 75: "parked"}[code]
	if mr.Version != 1 || mr.Job != job || mr.Action != string(action) || mr.Digest != mayDigest(job, string(action)) || mr.Verdict != want || errout != "" {
		return r, 2, errors.New("May result does not match exact action and exit status")
	}
	if canonical, _ := canonicalLine(mr); out != string(canonical) {
		return r, 2, errors.New("May result was not canonical JSON")
	}
	if got, err := fileDigest(mayPath); err != nil || got != mayHash {
		return r, 2, errors.New("May executable changed during decision")
	}
	r.MayExitCode, r.MayResult, r.MayResultSHA256 = code, &mr, digest([]byte(out))
	r.Decision, r.Reason = map[int]string{0: "allow", 3: "deny", 75: "review"}[code], "May "+want
	r.Authorized = code == 0
	return r, code, nil
}

func (r recorder) note(kind string, body any) error {
	if r.session == "" {
		return nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	cmd := exec.Command(r.ask, "note", "-q", "-s", "action", "-f", r.session, "-k", kind, "-json", "-", "-seal")
	cmd.Stdin = bytes.NewReader(raw)
	var errout bytes.Buffer
	cmd.Stdout, cmd.Stderr = io.Discard, &errout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ask note: %v: %s", err, firstLine(errout.String()))
	}
	return nil
}

func listConnectors(stdout, stderr io.Writer) int {
	seen := map[string]bool{}
	var names []string
	for _, dir := range filepath.SplitList(os.Getenv("ACTION_PATH")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && portableName.MatchString(entry.Name()) && !seen[entry.Name()] {
				seen[entry.Name()] = true
				names = append(names, entry.Name())
			}
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path, err := resolveOnPath(name, os.Getenv("ACTION_PATH"))
		if err != nil {
			continue
		}
		raw, _, err := readDescriptor(path)
		if err != nil {
			fmt.Fprintf(stderr, "action: %s: %v\n", name, err)
			return 2
		}
		fmt.Fprintln(stdout, string(raw))
	}
	return 0
}

func showConnector(name string, stdout, stderr io.Writer) int {
	path, err := resolveOnPath(name, os.Getenv("ACTION_PATH"))
	if err != nil {
		return diagnose(stderr, err)
	}
	raw, _, err := readDescriptor(path)
	if err != nil {
		return diagnose(stderr, err)
	}
	fmt.Fprintln(stdout, string(raw))
	return 0
}

func readDescriptor(path string) ([]byte, descriptor, error) {
	out, errout, code, err := runBounded(path, []string{"describe"}, nil, maxDescriptor)
	if err != nil || code != 0 || errout != "" {
		return nil, descriptor{}, fmt.Errorf("describe connector: exit %d: %v %s", code, err, firstLine(errout))
	}
	raw, err := canonicalObject([]byte(out))
	if err != nil {
		return nil, descriptor{}, fmt.Errorf("connector descriptor: %w", err)
	}
	var d descriptor
	if err := decodeStrict(raw, &d); err != nil {
		return nil, d, err
	}
	if d.Version != 1 || !portableName.MatchString(d.Name) || strings.TrimSpace(d.Description) == "" || len(d.Description) > 2048 || len(d.InputSchema) == 0 {
		return nil, d, errors.New("connector descriptor is incomplete")
	}
	if _, err := canonicalObject(d.InputSchema); err != nil {
		return nil, d, fmt.Errorf("connector input_schema: %w", err)
	}
	return raw, d, nil
}

func revalidate(path, wantHash string, wantDescriptor []byte) error {
	got, err := fileDigest(path)
	if err != nil || got != wantHash {
		return errors.New("connector executable changed before execution")
	}
	raw, _, err := readDescriptor(path)
	if err != nil || !bytes.Equal(raw, wantDescriptor) {
		return errors.New("connector descriptor changed before execution")
	}
	return nil
}

func readCanonicalObject(r io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("JSON input exceeds %d bytes", limit)
	}
	return canonicalObject(raw)
}

func canonicalObject(raw []byte) ([]byte, error) {
	if err := rejectDuplicateKeys(raw); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v map[string]any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("expected exactly one JSON object")
	}
	return json.Marshal(v)
}

func canonicalLine(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	return append(raw, '\n'), err
}

func decodeStrict(raw []byte, dst any) error {
	if err := rejectDuplicateKeys(raw); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func rejectDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var walk func() error
	walk = func() error {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				keyToken, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("object key is not a string")
				}
				if seen[key] {
					return fmt.Errorf("duplicate JSON field %q", key)
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON")
		}
		return err
	}
	return nil
}

func decodeStrictLine(raw string, dst any) error {
	if !strings.HasSuffix(raw, "\n") || strings.Count(strings.TrimSuffix(raw, "\n"), "\n") != 0 {
		return errors.New("expected one JSON line")
	}
	return decodeStrict([]byte(raw), dst)
}

func resolveOnPath(name, pathList string) (string, error) {
	if pathList == "" {
		return "", errors.New("ACTION_PATH is empty")
	}
	for _, dir := range filepath.SplitList(pathList) {
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			physical, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return "", err
			}
			return mustAbs(physical), nil
		}
	}
	return "", fmt.Errorf("connector %q not found on ACTION_PATH", name)
}

func fileDigest(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("digest %s: %w", path, err)
	}
	return digest(raw), nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func bodyDigest(v any) string {
	raw, _ := json.Marshal(v)
	return digest(raw)
}

func mayDigest(job, action string) string {
	h := sha256.New()
	_, _ = io.WriteString(h, "may-v1\x00"+job+"\x00"+action)
	return hex.EncodeToString(h.Sum(nil))
}

type boundedBuffer struct {
	buf      []byte
	limit    int
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	space := b.limit - len(b.buf)
	if space > 0 {
		if len(p) < space {
			space = len(p)
		}
		b.buf = append(b.buf, p[:space]...)
	}
	if space < len(p) {
		b.overflow = true
	}
	return len(p), nil
}

func runBounded(path string, args []string, input []byte, limit int) (string, string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = bytes.NewReader(input)
	out, errout := &boundedBuffer{limit: limit}, &boundedBuffer{limit: limit}
	cmd.Stdout, cmd.Stderr = out, errout
	err := cmd.Run()
	if ctx.Err() != nil {
		return string(out.buf), string(errout.buf), 2, ctx.Err()
	}
	if out.overflow || errout.overflow {
		return string(out.buf), string(errout.buf), 2, errors.New("output bound exceeded")
	}
	code, terminal := processExit(err)
	if !terminal {
		return string(out.buf), string(errout.buf), 2, err
	}
	return string(out.buf), string(errout.buf), code, nil
}

func processExit(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ProcessState.Exited() {
		return ee.ExitCode(), true
	}
	return 2, false
}

func outcome(code int) string {
	switch code {
	case 0:
		return "completed"
	case 1:
		return "failed"
	case 2:
		return "broken"
	case 3:
		return "declined"
	case 75:
		return "waiting"
	default:
		return "unknown"
	}
}

func diagnose(w io.Writer, err error) int {
	fmt.Fprintf(w, "action: %v\n", err)
	return 2
}

func uncertain(w io.Writer, stage string, err error) int {
	fmt.Fprintf(w, "action: %s: %v; effects may exist; do not retry automatically\n", stage, err)
	return 125
}

func envDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func firstError(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
