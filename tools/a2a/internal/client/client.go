package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	a2alog "github.com/a2aproject/a2a-go/v2/log"
	"github.com/patrickyoung/a2a/internal/boundary"
)

const Version = "0.1.0-dev"
const help = `usage:
  a2a discover [options] CARD_URL
  a2a request [options] METHOD URL < request.json
  a2a listen [options] METHOD URL < request.json

request methods: send, get, list, cancel, card,
                 push-get, push-list, push-set, push-delete
listen methods:  send, subscribe

Options precede METHOD and URL:
  -transport jsonrpc|rest  explicit A2A 1.0 binding (default jsonrpc)
  -header-fd N            credential headers from descriptor N (N >= 3)
  -ca FILE                trust this CA bundle instead of system roots
  -cert FILE -key FILE    client certificate and private key for mTLS
  -timeout D             whole invocation deadline (default no deadline)
  -max-input N           input byte bound (default 16777216)
  -max-output N          response/output byte bound (default 67108864)
  -http-loopback         explicitly permit HTTP on a literal loopback address

Each invocation selects one endpoint. No redirects, retries, automatic polling,
protocol fallback, credential acquisition, or hidden context. A bare origin for
discover selects /.well-known/agent-card.json; otherwise supply the card URL.
Stdout is JSON (JSONL for listen); stderr is diagnostics. Exit status:
0 complete positive, 1 known negative, 2 local/pre-transmission failure,
75 unfinished, 125 uncertain after transmission, 130 interrupted before send.
`

type options struct {
	network   boundary.Network
	transport string
	fd        int
	timeout   time.Duration
	maxInput  int64
}

func Run(parent context.Context, args []string, in io.Reader, out, diagnostics io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(out, help)
		return 0
	}
	if args[0] == "version" {
		fmt.Fprintln(out, "a2a "+Version)
		return 0
	}
	verb := args[0]
	if verb != "discover" && verb != "request" && verb != "listen" {
		fmt.Fprintln(diagnostics, "a2a: unknown command")
		return 2
	}
	var o options
	f := flag.NewFlagSet(verb, flag.ContinueOnError)
	f.SetOutput(diagnostics)
	f.StringVar(&o.transport, "transport", "jsonrpc", "explicit transport: jsonrpc or rest")
	f.IntVar(&o.fd, "header-fd", -1, "credential header descriptor")
	f.StringVar(&o.network.CA, "ca", "", "CA bundle")
	f.StringVar(&o.network.Cert, "cert", "", "client certificate")
	f.StringVar(&o.network.Key, "key", "", "private key")
	f.BoolVar(&o.network.LoopbackHTTP, "http-loopback", false, "permit literal loopback HTTP")
	f.DurationVar(&o.timeout, "timeout", 0, "whole invocation deadline")
	f.Int64Var(&o.maxInput, "max-input", 16<<20, "input byte limit")
	f.Int64Var(&o.network.MaxOutput, "max-output", 64<<20, "output byte limit")
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	pos := f.Args()
	if (verb == "discover" && len(pos) != 1) || (verb != "discover" && len(pos) != 2) || o.maxInput <= 0 || o.maxInput > 1<<30 || o.network.MaxOutput <= 0 || o.network.MaxOutput > 1<<30 || o.timeout < 0 || (o.fd != -1 && o.fd < 3) || (o.transport != "jsonrpc" && o.transport != "rest") {
		fmt.Fprintln(diagnostics, "a2a: invalid arguments; see a2a help")
		return 2
	}
	ctx := a2alog.AttachLogger(parent, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if o.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}
	if o.fd >= 3 {
		file := os.NewFile(uintptr(o.fd), "a2a-credential-headers")
		if file == nil {
			fmt.Fprintln(diagnostics, "a2a: invalid credential descriptor")
			return 2
		}
		defer file.Close()
		stopRead := context.AfterFunc(ctx, func() { _ = file.Close() })
		defer stopRead()
		var err error
		o.network.Headers, err = boundary.ReadHeaders(file)
		if err != nil {
			if parent.Err() != nil {
				return 130
			}
			fmt.Fprintln(diagnostics, "a2a: cannot read valid credential headers")
			return 2
		}
	}
	u, err := boundary.Endpoint(pos[len(pos)-1], o.network.LoopbackHTTP)
	if err != nil {
		fmt.Fprintln(diagnostics, "a2a:", err)
		return 2
	}
	if verb == "discover" && (u.Path == "" || u.Path == "/") {
		u.Path = "/.well-known/agent-card.json"
	}
	httpClient, record, err := boundary.HTTPClient(u, o.network)
	if err != nil {
		fmt.Fprintln(diagnostics, "a2a: invalid TLS configuration")
		return 2
	}
	defer record.Close()
	writer := &output{writer: out, left: o.network.MaxOutput}
	if verb == "discover" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return 2
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return failure(ctx, record, err, writer, diagnostics)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return failure(ctx, record, errors.New("card request rejected"), writer, diagnostics)
		}
		b, err := boundary.ReadObject(resp.Body, o.network.MaxOutput)
		var card a2a.AgentCard
		if err == nil {
			err = json.Unmarshal(b, &card)
		}
		if err != nil || card.Name == "" || len(card.SupportedInterfaces) == 0 {
			fmt.Fprintln(diagnostics, "a2a: invalid Agent Card")
			return 125
		}
		if err = writer.raw(b); err != nil {
			return 125
		}
		return 0
	}
	raw, err := readInput(ctx, in, o.maxInput)
	if err != nil {
		if parent.Err() != nil {
			return 130
		}
		fmt.Fprintln(diagnostics, "a2a: invalid or incomplete bounded JSON input")
		return 2
	}
	method := pos[0]
	operation, err := prepare(method, verb, raw)
	if err != nil {
		fmt.Fprintln(diagnostics, "a2a:", err)
		return 2
	}
	protocol := a2a.TransportProtocolJSONRPC
	transport := a2aclient.WithJSONRPCTransport(httpClient)
	if o.transport == "rest" {
		protocol = a2a.TransportProtocolHTTPJSON
		transport = a2aclient.WithRESTTransport(httpClient)
	}
	remote, err := a2aclient.NewFromEndpoints(ctx, []*a2a.AgentInterface{{URL: u.String(), ProtocolBinding: protocol, ProtocolVersion: a2a.Version}}, a2aclient.WithDefaultsDisabled(), transport)
	if err != nil {
		fmt.Fprintln(diagnostics, "a2a: cannot construct selected protocol transport")
		return 2
	}
	if verb == "listen" {
		return listen(ctx, record, operation.stream(ctx, remote), operation.taskID, operation.contextID, writer, diagnostics)
	}
	result, err := operation.call(ctx, remote)
	if err != nil {
		return failure(ctx, record, err, writer, diagnostics)
	}
	if task, ok := result.(*a2a.Task); ok && (task == nil || operation.taskID != "" && task.ID != operation.taskID || operation.contextID != "" && task.ContextID != operation.contextID) {
		return 125
	}
	code := Outcome(result)
	if method == "cancel" {
		if task, ok := result.(*a2a.Task); ok && code != 125 && task.Status.State == a2a.TaskStateCanceled {
			code = 0
		}
	}
	if err := writer.value(result); err != nil {
		fmt.Fprintln(diagnostics, "a2a: cannot write bounded result")
		return 125
	}
	return code
}

func readInput(ctx context.Context, r io.Reader, limit int64) ([]byte, error) {
	type result struct {
		b   []byte
		err error
	}
	ready := make(chan result, 1)
	go func() { b, err := boundary.ReadObject(r, limit); ready <- result{b, err} }()
	select {
	case got := <-ready:
		return got.b, got.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type operation struct {
	taskID    a2a.TaskID
	contextID string
	call      func(context.Context, *a2aclient.Client) (any, error)
	stream    func(context.Context, *a2aclient.Client) iter.Seq2[a2a.Event, error]
}

func decode[T any](raw []byte) (*T, error) {
	var value T
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return nil, errors.New("request does not match the selected A2A method")
	}
	return &value, nil
}

func idOK(id string) bool {
	return id != "" && !strings.ContainsAny(id, "/\\?#\r\n\x00") && id != "." && id != ".."
}

func prepare(method, verb string, raw []byte) (operation, error) {
	var op operation
	invalid := errors.New("invalid parameters for selected A2A method")
	if verb == "listen" && method != "send" && method != "subscribe" {
		return op, errors.New("listen requires send or subscribe")
	}
	switch method {
	case "send":
		p, err := decode[a2a.SendMessageRequest](raw)
		if err != nil || p.Message == nil || !idOK(string(p.Message.ID)) || len(p.Message.Parts) == 0 || p.Message.Role != a2a.MessageRoleUser {
			return op, invalid
		}
		if p.Message.TaskID != "" && !idOK(string(p.Message.TaskID)) {
			return op, invalid
		}
		for _, part := range p.Message.Parts {
			if part == nil || part.Content == nil {
				return op, invalid
			}
		}
		op.taskID, op.contextID = p.Message.TaskID, p.Message.ContextID
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) { return c.SendMessage(ctx, p) }
		op.stream = func(ctx context.Context, c *a2aclient.Client) iter.Seq2[a2a.Event, error] {
			return c.SendStreamingMessage(ctx, p)
		}
	case "get":
		p, err := decode[a2a.GetTaskRequest](raw)
		if err != nil || !idOK(string(p.ID)) {
			return op, invalid
		}
		op.taskID = p.ID
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) { return c.GetTask(ctx, p) }
	case "list":
		p, err := decode[a2a.ListTasksRequest](raw)
		if err != nil || p.PageSize < 0 || p.PageSize > 100 {
			return op, invalid
		}
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) { return c.ListTasks(ctx, p) }
	case "cancel":
		p, err := decode[a2a.CancelTaskRequest](raw)
		if err != nil || !idOK(string(p.ID)) {
			return op, invalid
		}
		op.taskID = p.ID
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) { return c.CancelTask(ctx, p) }
	case "subscribe":
		if verb != "listen" {
			return op, errors.New("subscribe requires listen")
		}
		p, err := decode[a2a.SubscribeToTaskRequest](raw)
		if err != nil || !idOK(string(p.ID)) {
			return op, invalid
		}
		op.taskID = p.ID
		op.stream = func(ctx context.Context, c *a2aclient.Client) iter.Seq2[a2a.Event, error] {
			return c.SubscribeToTask(ctx, p)
		}
	case "card":
		p, err := decode[a2a.GetExtendedAgentCardRequest](raw)
		if err != nil {
			return op, invalid
		}
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) { return c.GetExtendedAgentCard(ctx, p) }
	case "push-get":
		p, err := decode[a2a.GetTaskPushConfigRequest](raw)
		if err != nil || !idOK(string(p.TaskID)) || !idOK(p.ID) {
			return op, invalid
		}
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) { return c.GetTaskPushConfig(ctx, p) }
	case "push-list":
		p, err := decode[a2a.ListTaskPushConfigRequest](raw)
		if err != nil || !idOK(string(p.TaskID)) {
			return op, invalid
		}
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) { return c.ListTaskPushConfigs(ctx, p) }
	case "push-set":
		p, err := decode[a2a.PushConfig](raw)
		if err != nil || !idOK(string(p.TaskID)) {
			return op, invalid
		}
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) { return c.CreateTaskPushConfig(ctx, p) }
	case "push-delete":
		p, err := decode[a2a.DeleteTaskPushConfigRequest](raw)
		if err != nil || !idOK(string(p.TaskID)) || !idOK(p.ID) {
			return op, invalid
		}
		op.call = func(ctx context.Context, c *a2aclient.Client) (any, error) {
			err := c.DeleteTaskPushConfig(ctx, p)
			return map[string]any{}, err
		}
	default:
		return op, errors.New("unknown A2A method")
	}
	return op, nil
}

// Outcome classifies execution results, not merely a successful HTTP exchange.
func Outcome(value any) int {
	switch result := value.(type) {
	case *a2a.Task:
		if result == nil || result.ID == "" || result.ContextID == "" {
			return 125
		}
		if result.Metadata["bench/outcome"] == "unknown" {
			return 125
		}
		return stateOutcome(result.Status.State)
	case *a2a.TaskStatusUpdateEvent:
		if result == nil || result.TaskID == "" || result.ContextID == "" {
			return 125
		}
		if result.Metadata["bench/outcome"] == "unknown" {
			return 125
		}
		return stateOutcome(result.Status.State)
	case *a2a.Message:
		if result == nil || result.ID == "" || result.Role != a2a.MessageRoleAgent || len(result.Parts) == 0 {
			return 125
		}
		for _, part := range result.Parts {
			if part == nil || part.Content == nil {
				return 125
			}
		}
		return 0
	case *a2a.TaskArtifactUpdateEvent:
		if result == nil || result.TaskID == "" || result.ContextID == "" || result.Artifact == nil || len(result.Artifact.Parts) == 0 {
			return 125
		}
		for _, part := range result.Artifact.Parts {
			if part == nil || part.Content == nil {
				return 125
			}
		}
		return 75
	default:
		return 0
	}
}

func stateOutcome(state a2a.TaskState) int {
	switch state {
	case a2a.TaskStateCompleted:
		return 0
	case a2a.TaskStateFailed, a2a.TaskStateCanceled, a2a.TaskStateRejected:
		return 1
	case a2a.TaskStateSubmitted, a2a.TaskStateWorking, a2a.TaskStateInputRequired, a2a.TaskStateAuthRequired:
		return 75
	default:
		return 125
	}
}

func listen(ctx context.Context, record *boundary.Transport, events iter.Seq2[a2a.Event, error], taskID a2a.TaskID, contextID string, out *output, diagnostics io.Writer) int {
	final := false
	code := 125
	for event, err := range events {
		if err != nil {
			return failure(ctx, record, err, out, diagnostics)
		}
		if final || event == nil {
			fmt.Fprintln(diagnostics, "a2a: event after final outcome or missing event")
			return 125
		}
		code = Outcome(event)
		if code == 125 {
			// Preserve a valid uncertain task handle for explicit inspection.
			switch e := event.(type) {
			case *a2a.Task:
				if e != nil && e.ID != "" && e.Metadata["bench/outcome"] == "unknown" {
					_ = out.value(a2a.StreamResponse{Event: event})
				}
			case *a2a.TaskStatusUpdateEvent:
				if e != nil && e.TaskID != "" && e.Metadata["bench/outcome"] == "unknown" {
					_ = out.value(a2a.StreamResponse{Event: event})
				}
			}
			return 125
		}
		if info, ok := event.(a2a.TaskInfoProvider); ok {
			facts := info.TaskInfo()
			id := facts.TaskID
			if facts.ContextID != "" {
				if contextID != "" && facts.ContextID != contextID {
					return 125
				}
				contextID = facts.ContextID
			}
			if id != "" {
				if taskID != "" && id != taskID {
					return 125
				}
				taskID = id
			}
		}
		switch e := event.(type) {
		case *a2a.Message:
			final = true
		case *a2a.Task:
			final = e.Status.State != a2a.TaskStateWorking && e.Status.State != a2a.TaskStateSubmitted
		case *a2a.TaskStatusUpdateEvent:
			final = e.Status.State != a2a.TaskStateWorking && e.Status.State != a2a.TaskStateSubmitted
		}
		if err := out.value(a2a.StreamResponse{Event: event}); err != nil {
			return 125
		}
		// Auth/input-required is an explicit handoff to the caller. Some peers
		// keep that subscription open for out-of-band authentication; this
		// filter returns the handle instead of becoming a hidden waiter.
		if final {
			return code
		}
	}
	if !final {
		fmt.Fprintln(diagnostics, "a2a: stream ended without a final outcome")
		return 125
	}
	return code
}

func failure(ctx context.Context, record *boundary.Transport, err error, out *output, diagnostics io.Writer) int {
	if !record.Sent.Load() {
		if errors.Is(ctx.Err(), context.Canceled) {
			return 130
		}
		fmt.Fprintln(diagnostics, "a2a: request failed before transmission")
		return 2
	}
	if !record.Invalid.Load() {
		status := record.Status.Load()
		if status >= 400 && status < 500 {
			if out.value(map[string]any{"error": map[string]any{"httpStatus": status}}) != nil {
				return 125
			}
			fmt.Fprintln(diagnostics, "a2a: peer rejected the request")
			return 1
		}
		var peer *a2a.Error
		if errors.As(err, &peer) {
			if out.value(map[string]any{"error": map[string]any{"reason": a2a.ErrorReason(peer), "message": peer.Message, "details": peer.Details}}) != nil {
				return 125
			}
			fmt.Fprintln(diagnostics, "a2a: peer returned an A2A error")
			if peer.Details["bench/outcome"] == "unknown" {
				return 125
			}
			return 1
		}
	}
	fmt.Fprintln(diagnostics, "a2a: outcome uncertain after transmission; inspect the task before any new submission")
	return 125
}

type output struct {
	writer io.Writer
	left   int64
}

func (o *output) raw(b []byte) error {
	if int64(len(b))+1 > o.left {
		return errors.New("output exceeds byte limit")
	}
	b = append(append([]byte(nil), b...), '\n')
	n, err := o.writer.Write(b)
	o.left -= int64(n)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return err
}
func (o *output) value(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return o.raw(b)
}
