package serve

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/patrickyoung/a2a/internal/boundary"
)

type ExecutorConfig struct {
	Command             []string
	Tend                string
	Input, Output       string
	Artifacts, PassEnv  []string
	Timeout             time.Duration
	MaxInput, MaxOutput int64
}

type execution struct {
	cancel   context.CancelFunc
	done     chan struct{}
	canceled bool // protected by Executor.mu
}

type Executor struct {
	config   ExecutorConfig
	store    *Store
	lifetime context.Context
	mu       sync.Mutex
	active   map[a2a.TaskID]*execution
}

func NewExecutor(ctx context.Context, store *Store, cfg ExecutorConfig) (*Executor, error) {
	if len(cfg.Command) == 0 || cfg.Timeout <= 0 || cfg.MaxInput <= 0 || cfg.MaxOutput <= 0 {
		return nil, errors.New("worker command and positive execution limits are required")
	}
	if cfg.Input != "text" && cfg.Input != "json" || cfg.Output != "text" && cfg.Output != "json" {
		return nil, errors.New("worker input/output mode must be text or json")
	}
	for _, name := range cfg.Artifacts {
		if !filepath.IsLocal(name) || name == "." {
			return nil, errors.New("artifact paths must be relative paths within the workspace")
		}
	}
	for _, name := range cfg.PassEnv {
		if name == "" || strings.HasPrefix(name, "TEND_") || strings.HasPrefix(name, "_TEND_") || strings.HasPrefix(name, "A2A_") {
			return nil, errors.New("invalid or reserved worker environment name")
		}
		for i, ch := range name {
			if !(ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || i > 0 && ch >= '0' && ch <= '9') {
				return nil, errors.New("invalid worker environment name")
			}
		}
	}
	command, err := exec.LookPath(cfg.Command[0])
	if err != nil {
		return nil, err
	}
	command, err = filepath.Abs(command)
	if err != nil {
		return nil, err
	}
	cfg.Command = append([]string{command}, cfg.Command[1:]...)
	if cfg.Tend == "" {
		cfg.Tend = "tend"
	}
	tend, err := exec.LookPath(cfg.Tend)
	if err != nil {
		return nil, errors.New("a2aserve requires the existing tend executable")
	}
	cfg.Tend, err = filepath.Abs(tend)
	if err != nil {
		return nil, err
	}
	return &Executor{config: cfg, store: store, lifetime: ctx, active: map[a2a.TaskID]*execution{}}, nil
}

func (e *Executor) Execute(ctx context.Context, c *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		runCtx, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(e.lifetime, cancel)
		defer stop()
		defer cancel()
		run := &execution{cancel: cancel, done: make(chan struct{})}
		e.mu.Lock()
		e.active[c.TaskID] = run
		e.mu.Unlock()
		defer func() { e.mu.Lock(); delete(e.active, c.TaskID); close(run.done); e.mu.Unlock() }()
		if c.StoredTask == nil {
			if !yield(a2a.NewSubmittedTask(c, c.Message), nil) {
				return
			}
		}
		if !yield(a2a.NewStatusUpdateEvent(c, a2a.TaskStateWorking, nil), nil) {
			return
		}
		result, err := e.execute(runCtx, c)
		e.mu.Lock()
		canceled := run.canceled
		e.mu.Unlock()
		if canceled {
			return
		} // the cancellation producer supplies the final event
		if err != nil {
			event := a2a.NewStatusUpdateEvent(c, a2a.TaskStateFailed, a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(err.Error())))
			event.Metadata = map[string]any{"bench/outcome": "unknown"}
			yield(event, nil)
			return
		}
		for _, artifact := range result.Artifacts {
			event := a2a.NewArtifactEvent(c, artifact.Parts...)
			if artifact.ID == "" {
				artifact.ID = event.Artifact.ID
			}
			event.Artifact = artifact
			event.LastChunk = true
			if !yield(event, nil) {
				return
			}
		}
		event := a2a.NewStatusUpdateEvent(c, result.Status.State, result.Status.Message)
		event.Metadata = result.Metadata
		yield(event, nil)
	}
}

func (e *Executor) Cancel(ctx context.Context, c *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		e.mu.Lock()
		run := e.active[c.TaskID]
		if run != nil {
			run.canceled = true
			run.cancel()
		}
		e.mu.Unlock()
		if run != nil {
			select {
			case <-run.done:
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			}
		}
		yield(a2a.NewStatusUpdateEvent(c, a2a.TaskStateCanceled, a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("Local execution stopped. Completed external effects are not undone."))), nil)
	}
}

type workerInput struct {
	TaskID    a2a.TaskID     `json:"taskId"`
	ContextID string         `json:"contextId"`
	Message   *a2a.Message   `json:"message"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type workRecord struct {
	Job    string `json:"job"`
	Event  string `json:"event"`
	Status string `json:"status"`
	Exit   *int   `json:"exit"`
	Output string `json:"output"`
}

func key(value string) string { sum := sha256.Sum256([]byte(value)); return hex.EncodeToString(sum[:]) }

func (e *Executor) execute(ctx context.Context, c *a2asrv.ExecutorContext) (*a2a.Task, error) {
	var input []byte
	if e.config.Input == "text" {
		if c.Message == nil || len(c.Message.Parts) != 1 {
			return negative(c, "Text workers require exactly one text part."), nil
		}
		text, ok := c.Message.Parts[0].Content.(a2a.Text)
		if !ok {
			return negative(c, "This worker accepts text; structured messages require a JSON worker."), nil
		}
		input = []byte(text)
	} else {
		var err error
		input, err = json.Marshal(workerInput{c.TaskID, c.ContextID, c.Message, c.Metadata})
		if err != nil {
			return negative(c, "Invalid structured worker input."), nil
		}
	}
	if int64(len(input)) > e.config.MaxInput {
		return negative(c, "Worker input exceeds its byte limit."), nil
	}
	taskDir := filepath.Join(e.store.Root(), "workspaces", key(string(c.TaskID)))
	for _, dir := range []string{"work", "state", "control", "tmp"} {
		p := filepath.Join(taskDir, dir)
		if err := os.MkdirAll(p, 0700); err != nil {
			return nil, errors.New("cannot prepare task workspace")
		}
		i, err := os.Lstat(p)
		if err != nil || !i.IsDir() {
			return nil, errors.New("task workspace is not a real directory")
		}
	}
	work := filepath.Join(taskDir, "work")
	attempt := filepath.Join(e.store.Root(), "evidence", key(string(c.TaskID)), key(c.Message.ID))
	if err := os.MkdirAll(filepath.Dir(attempt), 0700); err != nil {
		return nil, errors.New("cannot prepare task evidence")
	}
	if err := os.Mkdir(attempt, 0700); err != nil {
		return nil, errors.New("attempt evidence already exists or cannot be created; no work was resubmitted")
	}
	queue := filepath.Join(attempt, "tend")
	job := "worker" // the private attempt queue contains exactly one job
	env := e.environment(queue, taskDir, c)
	args := append([]string{"submit", "-id", job, "-C", work, "--"}, e.config.Command...)
	accepted, err := e.tend(ctx, env, input, args...)
	if err != nil || strings.TrimSpace(string(accepted)) != job {
		return nil, errors.New("Tend submission did not return its expected job handle; inspect attempt evidence")
	}
	// One fresh private queue, one explicit durable transition. No worker loop,
	// retry, or invocation of another task can be hidden inside this request.
	receipt, workErr := e.tend(ctx, env, nil, "work")
	var record workRecord
	if boundary.CheckJSON(receipt) != nil || json.Unmarshal(receipt, &record) != nil || record.Job != job || record.Event != "attempt.finished" || record.Exit == nil {
		return nil, errors.New("Tend execution has no trustworthy terminal receipt; inspect attempt evidence")
	}
	if err := os.WriteFile(filepath.Join(attempt, "receipt.json"), receipt, 0600); err != nil {
		return nil, errors.New("cannot retain Tend receipt")
	}
	if _, err := e.tend(ctx, env, nil, "check"); err != nil {
		return nil, errors.New("Tend evidence verification failed; inspect the retained attempt")
	}
	result := &a2a.Task{ID: c.TaskID, ContextID: c.ContextID, Metadata: map[string]any{"bench/exitCode": *record.Exit, "bench/tendStatus": record.Status, "bench/attempt": key(c.Message.ID)}}
	result.Status.State = a2a.TaskStateFailed
	switch {
	case record.Status == "unknown" || workErr != nil || ctx.Err() != nil:
		markUnknown(result, "Execution outcome requires inspection of the retained Tend evidence.")
	case *record.Exit == 0 && record.Status == "done":
		result.Status.State = a2a.TaskStateCompleted
	case *record.Exit == 75:
		result.Status.State = a2a.TaskStateInputRequired
	case *record.Exit == 125 || *record.Exit == 130:
		markUnknown(result, "Worker reported an uncertain execution outcome.")
	}
	root, err := os.OpenRoot(queue)
	if err != nil {
		return nil, errors.New("cannot open execution evidence")
	}
	defer root.Close()
	if !filepath.IsLocal(record.Output) {
		return nil, errors.New("Tend output path escaped its evidence directory")
	}
	f, err := root.OpenFile(record.Output, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("cannot read sealed worker output")
	}
	defer f.Close()
	i, err := f.Stat()
	if err != nil || !i.Mode().IsRegular() {
		return nil, errors.New("worker output is not a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, e.config.MaxOutput+1))
	if err != nil || int64(len(b)) > e.config.MaxOutput {
		return negative(c, "Worker output exceeded the exported byte limit."), nil
	}
	if e.config.Output == "json" && len(bytes.TrimSpace(b)) > 0 {
		var structured a2a.Task
		if boundary.CheckJSON(b) != nil || json.Unmarshal(b, &structured) != nil || structured.ID != "" && structured.ID != c.TaskID || structured.ContextID != "" && structured.ContextID != c.ContextID {
			return nil, errors.New("worker returned an invalid structured task result")
		}
		if *record.Exit == 0 && result.Metadata["bench/outcome"] != "unknown" && structured.Status.State != "" {
			result.Status = structured.Status
		}
		if *record.Exit == 75 && structured.Status.State == a2a.TaskStateAuthRequired {
			result.Status = structured.Status
		}
		for name, value := range structured.Metadata {
			if !strings.HasPrefix(name, "bench/") {
				result.Metadata[name] = value
			}
		}
		result.Artifacts = structured.Artifacts
	} else if len(b) > 0 {
		if !utf8.Valid(b) {
			return negative(c, "Text output must be valid UTF-8; export binary data as a file artifact."), nil
		}
		result.Artifacts = []*a2a.Artifact{{Parts: a2a.ContentParts{a2a.NewTextPart(string(b))}}}
	}
	// A finished Unix process cannot remain waiting for out-of-band OAuth.
	// Resume is an explicit new message, represented by A2A input-required.
	// The SDK keeps auth-required executions alive; do not leak an execution
	// slot or introduce a credential waiter around a process that has exited.
	if result.Status.State == a2a.TaskStateAuthRequired {
		result.Status.State = a2a.TaskStateInputRequired
		result.Metadata["bench/authRequired"] = true
	}
	if !result.Status.State.Terminal() && result.Status.State != a2a.TaskStateInputRequired && result.Status.State != a2a.TaskStateAuthRequired {
		return nil, errors.New("worker returned an unsupported final state")
	}
	if result.Status.State == a2a.TaskStateCompleted {
		artifacts, err := e.collect(work)
		if err != nil {
			return negative(c, "A required file artifact is missing, unsafe, or exceeds its export limit."), nil
		}
		result.Artifacts = append(result.Artifacts, artifacts...)
	}
	for _, artifact := range result.Artifacts {
		if artifact == nil || len(artifact.Parts) == 0 {
			return nil, errors.New("worker returned an invalid artifact")
		}
		for _, part := range artifact.Parts {
			if part == nil || part.Content == nil {
				return nil, errors.New("worker returned an invalid artifact part")
			}
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil || int64(len(encoded)) > e.config.MaxOutput {
		return negative(c, "Combined result exceeds the exported byte limit."), nil
	}
	return result, nil
}

func negative(c *a2asrv.ExecutorContext, message string) *a2a.Task {
	return &a2a.Task{ID: c.TaskID, ContextID: c.ContextID, Status: a2a.TaskStatus{State: a2a.TaskStateFailed, Message: a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(message))}}
}

func (e *Executor) environment(queue, taskDir string, c *a2asrv.ExecutorContext) []string {
	names := append([]string{"PATH", "HOME", "LANG", "LC_ALL", "ASK", "PLY", "MAY", "CAGE"}, e.config.PassEnv...)
	env := []string{}
	seen := map[string]bool{}
	for _, name := range names {
		if !seen[name] {
			if value, ok := os.LookupEnv(name); ok {
				env = append(env, name+"="+value)
			}
			seen[name] = true
		}
	}
	extra := append(append([]string(nil), e.config.PassEnv...), "A2A_TASK_ID", "A2A_CONTEXT_ID", "A2A_WORKSPACE", "A2A_STATE", "A2A_EVIDENCE")
	// Tend owns lease renewal. A subsecond override can expire during process
	// startup on a busy host, before the submitted worker is allowed to run.
	// Retain Tend's default lease; the explicit job deadline and cancellation
	// still bound execution independently of that lease.
	return append(env, "TEND_ROOT="+queue, "TEND_JOB_MAX="+e.config.Timeout.String(), "TEND_PASS="+strings.Join(extra, " "), "TMPDIR="+filepath.Join(taskDir, "tmp"), "A2A_TASK_ID="+string(c.TaskID), "A2A_CONTEXT_ID="+c.ContextID, "A2A_WORKSPACE="+filepath.Join(taskDir, "work"), "A2A_STATE="+filepath.Join(taskDir, "state"), "A2A_EVIDENCE="+filepath.Join(taskDir, "control"))
}

func (e *Executor) tend(ctx context.Context, env []string, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, e.config.Tend, args...)
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(input)
	var out cappedBuffer
	out.limit = 1 << 20
	cmd.Stdout = &out
	// Tend's own worker stderr is already retained privately in its evidence.
	// Never turn supervisor diagnostics into a remote task error message.
	cmd.Stderr = io.Discard
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	return out.Bytes(), err
}

type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("supervisor result exceeds byte limit")
	}
	return b.Buffer.Write(p)
}

func (e *Executor) collect(work string) ([]*a2a.Artifact, error) {
	root, err := os.OpenRoot(work)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	left := e.config.MaxOutput / 2
	var result []*a2a.Artifact
	for _, name := range e.config.Artifacts {
		f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		i, err := f.Stat()
		if err != nil || !i.Mode().IsRegular() || i.Size() > left {
			f.Close()
			return nil, errors.New("artifact exceeds regular-file boundary")
		}
		b, err := io.ReadAll(io.LimitReader(f, left+1))
		f.Close()
		if err != nil || int64(len(b)) > left {
			return nil, errors.New("artifact exceeds byte limit")
		}
		left -= int64(len(b))
		part := a2a.NewRawPart(b)
		part.Filename = name
		part.MediaType = mime.TypeByExtension(filepath.Ext(name))
		if part.MediaType == "" {
			part.MediaType = "application/octet-stream"
		}
		result = append(result, &a2a.Artifact{Name: name, Parts: a2a.ContentParts{part}})
	}
	return result, nil
}

// Drain is called after stopping HTTP admission and canceling the lifetime.
func (e *Executor) Drain(ctx context.Context) error {
	e.mu.Lock()
	runs := make([]*execution, 0, len(e.active))
	for _, run := range e.active {
		runs = append(runs, run)
	}
	e.mu.Unlock()
	for _, run := range runs {
		select {
		case <-run.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
