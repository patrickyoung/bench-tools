package serve

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/patrickyoung/a2a/internal/client"
)

// The fixture is an ordinary executable supervised by the separately built Tend.
func TestWorkerFixture(t *testing.T) {
	if os.Getenv("FIXTURE_WORKER") != "yes" {
		return
	}
	b, _ := io.ReadAll(os.Stdin)
	action := os.Args[len(os.Args)-1]
	switch action {
	case "echo":
		os.Stdout.Write(b)
	case "fail":
		fmt.Print("rejected\n")
		os.Exit(1)
	case "unknown":
		os.Exit(125)
	case "wait":
		os.WriteFile("pid", []byte(fmt.Sprint(os.Getpid())), 0600)
		time.Sleep(time.Minute)
	case "continue":
		if _, err := os.Stat("first"); err != nil {
			os.WriteFile("first", b, 0600)
			fmt.Print("more input needed")
			os.Exit(75)
		}
		first, _ := os.ReadFile("first")
		os.Stdout.Write(append(first, b...))
	case "json":
		var in workerInput
		json.Unmarshal(b, &in)
		if in.TaskID == "" || in.ContextID == "" || in.Message == nil {
			os.Exit(1)
		}
		os.WriteFile("result.bin", []byte{0, 1, 255, 10}, 0600)
		json.NewEncoder(os.Stdout).Encode(map[string]any{"status": map[string]string{"state": "TASK_STATE_COMPLETED"}, "artifacts": []any{map[string]any{"parts": in.Message.Parts}}})
	case "auth":
		fmt.Print(`{"status":{"state":"TASK_STATE_AUTH_REQUIRED"}}`)
		os.Exit(75)
	case "fifo":
		syscall.Mkfifo("result.bin", 0600)
	case "symlink":
		os.Symlink("/etc/passwd", "result.bin")
	case "env":
		json.NewEncoder(os.Stdout).Encode(map[string]string{"task": os.Getenv("A2A_TASK_ID"), "context": os.Getenv("A2A_CONTEXT_ID"), "secret": os.Getenv("FIXTURE_SECRET"), "home": os.Getenv("HOME")})
	}
	os.Exit(0)
}

func fixtureConfig(t *testing.T, action string) Config {
	t.Helper()
	tend := os.Getenv("A2A_TEST_TEND")
	if tend == "" {
		t.Skip("executable integration requires A2A_TEST_TEND; root verification supplies it")
	}
	t.Setenv("FIXTURE_WORKER", "yes")
	return Config{State: filepath.Join(t.TempDir(), "state"), PublicURL: "http://127.0.0.1", Development: true, MaxActive: 2,
		Executor: ExecutorConfig{Command: []string{os.Args[0], "-test.run=^TestWorkerFixture$", "--", action}, Tend: tend, Input: "text", Output: "text", PassEnv: []string{"FIXTURE_WORKER"}, Timeout: 10 * time.Second, MaxInput: 1 << 20, MaxOutput: 1 << 20}}
}

func startFixture(t *testing.T, cfg Config, auth *Auth) (*Service, *httptest.Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	card := &a2a.AgentCard{Name: "fixture", Description: "offline executable", Version: "1"}
	service, err := NewService(ctx, cfg, card, auth)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(service.Handler)
	if auth != nil {
		server.TLS = auth.TLS(&tls.Config{MinVersion: tls.VersionTLS12})
		server.StartTLS()
	} else {
		server.Start()
	}
	t.Cleanup(func() {
		cancel()
		server.Close()
		deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := service.Executor.Drain(deadline); err != nil {
			t.Error(err)
		}
		service.Store.Close()
	})
	return service, server
}

func callFilter(t *testing.T, server *httptest.Server, binding, method string, input any, want int) *a2a.Task {
	t.Helper()
	raw, _ := json.Marshal(input)
	var out, diag bytes.Buffer
	endpoint := "/rpc"
	if binding == "rest" {
		endpoint = "/rest"
	}
	code := client.Run(context.Background(), []string{"request", "-http-loopback", "-timeout", "15s", "-transport", binding, method, server.URL + endpoint}, bytes.NewReader(raw), &out, &diag)
	if code != want {
		t.Fatalf("%s/%s: exit %d, want %d: %s %s", binding, method, code, want, out.String(), diag.String())
	}
	var task a2a.Task
	if json.Unmarshal(out.Bytes(), &task) != nil {
		t.Fatalf("invalid output: %s", out.String())
	}
	return &task
}
func message(text string) *a2a.SendMessageRequest {
	return &a2a.SendMessageRequest{Message: a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(text))}
}

func TestTendExecutableBoundary(t *testing.T) {
	for _, binding := range []string{"jsonrpc", "rest"} {
		t.Run(binding, func(t *testing.T) {
			for _, test := range []struct {
				action string
				code   int
			}{{"echo", 0}, {"fail", 1}, {"unknown", 125}, {"continue", 75}} {
				t.Run(test.action, func(t *testing.T) {
					cfg := fixtureConfig(t, test.action)
					_, server := startFixture(t, cfg, nil)
					task := callFilter(t, server, binding, "send", message("one\ntwo\n"), test.code)
					if task.ID == "" || task.ContextID == "" {
						t.Fatal("missing task handles")
					}
					if test.action == "echo" && task.Artifacts[0].Parts[0].Text() != "one\ntwo\n" {
						t.Fatal("text changed")
					}
					if test.action == "continue" {
						next := message("three\n")
						next.Message.TaskID = task.ID
						next.Message.ContextID = task.ContextID
						done := callFilter(t, server, binding, "send", next, 0)
						if done.ID != task.ID || done.Artifacts[len(done.Artifacts)-1].Parts[0].Text() != "one\ntwo\nthree\n" {
							t.Fatalf("continuation lost workspace: %+v", done)
						}
						other := callFilter(t, server, binding, "send", message("fresh"), 75)
						if other.ID == task.ID || other.ContextID == task.ContextID {
							t.Fatal("new tasks inherited context")
						}
					}
				})
			}
		})
	}
}

func TestStructuredFilesAndCredentialIsolation(t *testing.T) {
	cfg := fixtureConfig(t, "json")
	cfg.Executor.Input = "json"
	cfg.Executor.Output = "json"
	cfg.Executor.Artifacts = []string{"result.bin"}
	_, server := startFixture(t, cfg, nil)
	req := message("")
	req.Message.Parts = a2a.ContentParts{a2a.NewDataPart(map[string]any{"n": 42}), a2a.NewRawPart([]byte{0, 255})}
	task := callFilter(t, server, "jsonrpc", "send", req, 0)
	if len(task.Artifacts) != 2 || task.Artifacts[1].Parts[0].Filename != "result.bin" {
		t.Fatalf("file artifact missing: %+v", task)
	}
	raw := task.Artifacts[1].Parts[0].Content.(a2a.Raw)
	if !bytes.Equal(raw, []byte{0, 1, 255, 10}) {
		t.Fatal("binary artifact changed")
	}
	t.Run("fifo cannot block export", func(t *testing.T) {
		cfg := fixtureConfig(t, "fifo")
		cfg.Executor.Artifacts = []string{"result.bin"}
		_, s := startFixture(t, cfg, nil)
		callFilter(t, s, "jsonrpc", "send", message(""), 1)
	})
	t.Run("unsafe file", func(t *testing.T) {
		cfg := fixtureConfig(t, "symlink")
		cfg.Executor.Artifacts = []string{"result.bin"}
		_, s := startFixture(t, cfg, nil)
		callFilter(t, s, "rest", "send", message(""), 1)
	})
	t.Run("credentials", func(t *testing.T) {
		t.Setenv("FIXTURE_SECRET", "must-not-reach-worker")
		cfg := fixtureConfig(t, "env")
		_, s := startFixture(t, cfg, nil)
		task := callFilter(t, s, "rest", "send", message(""), 0)
		body := task.Artifacts[0].Parts[0].Text()
		if strings.Contains(body, "must-not-reach-worker") || !strings.Contains(body, string(task.ID)) {
			t.Fatal(body)
		}
	})
	t.Run("auth handoff releases slots", func(t *testing.T) {
		cfg := fixtureConfig(t, "auth")
		cfg.Executor.Output = "json"
		cfg.MaxActive = 1
		_, s := startFixture(t, cfg, nil)
		for n := 0; n < 3; n++ {
			task := callFilter(t, s, "rest", "send", message(""), 75)
			if task.Status.State != a2a.TaskStateInputRequired || task.Metadata["bench/authRequired"] != true {
				t.Fatal(task)
			}
		}
	})
}

func TestMTLSTaskOwnershipAcrossProtocolOperations(t *testing.T) {
	cfg := fixtureConfig(t, "continue")
	ca, certs := clientCertificates(t)
	auth, err := NewAuth(AuthConfig{Mode: "mtls", ClientCA: ca})
	if err != nil {
		t.Fatal(err)
	}
	_, server := startFixture(t, cfg, auth)
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	peers := []*a2aclient.Client{}
	for n, cert := range certs {
		transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{cert}}}
		t.Cleanup(transport.CloseIdleConnections)
		httpClient := &http.Client{Transport: transport, Timeout: 10 * time.Second}
		binding := a2a.TransportProtocolJSONRPC
		endpoint := "/rpc"
		option := a2aclient.WithJSONRPCTransport(httpClient)
		if n == 1 {
			binding = a2a.TransportProtocolHTTPJSON
			endpoint = "/rest"
			option = a2aclient.WithRESTTransport(httpClient)
		}
		c, err := a2aclient.NewFromEndpoints(context.Background(), []*a2a.AgentInterface{{URL: server.URL + endpoint, ProtocolBinding: binding, ProtocolVersion: a2a.Version}}, a2aclient.WithDefaultsDisabled(), option)
		if err != nil {
			t.Fatal(err)
		}
		peers = append(peers, c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := peers[0].SendMessage(ctx, message("private"))
	if err != nil {
		t.Fatal(err)
	}
	task := result.(*a2a.Task)
	for _, op := range []func() error{
		func() error { _, e := peers[1].GetTask(ctx, &a2a.GetTaskRequest{ID: task.ID}); return e },
		func() error { _, e := peers[1].CancelTask(ctx, &a2a.CancelTaskRequest{ID: task.ID}); return e },
		func() error {
			r := message("steal")
			r.Message.TaskID = task.ID
			_, e := peers[1].SendMessage(ctx, r)
			return e
		},
		func() error {
			r := message("steal-context")
			r.Message.ContextID = task.ContextID
			_, e := peers[1].SendMessage(ctx, r)
			return e
		},
		func() error {
			for _, e := range peers[1].SubscribeToTask(ctx, &a2a.SubscribeToTaskRequest{ID: task.ID}) {
				return e
			}
			return nil
		},
	} {
		if err := op(); err == nil {
			t.Fatal("other principal accessed task/context")
		}
	}
	list, err := peers[1].ListTasks(ctx, &a2a.ListTasksRequest{})
	if err != nil || len(list.Tasks) != 0 {
		t.Fatalf("foreign list: %+v %v", list, err)
	}
	own, err := peers[0].GetTask(ctx, &a2a.GetTaskRequest{ID: task.ID})
	if err != nil || own.Status.State != a2a.TaskStateInputRequired {
		t.Fatalf("foreign cancellation changed own task: %+v %v", own, err)
	}
	wrong := message("wrong context")
	wrong.Message.TaskID = task.ID
	wrong.Message.ContextID = "unrelated"
	if _, err := peers[0].SendMessage(ctx, wrong); err == nil {
		t.Fatal("task accepted wrong context")
	}
}

func TestOAuthExecutableCredentialHandoff(t *testing.T) {
	oauth, filter := os.Getenv("A2A_TEST_OAUTH"), os.Getenv("A2A_TEST_CLIENT")
	if oauth == "" || filter == "" {
		t.Skip("requires separately built OAuth and A2A commands")
	}
	cfg := fixtureConfig(t, "continue")
	server := httptest.NewUnstartedServer(nil)
	resource := "https://" + server.Listener.Addr().String()
	var issuer string
	tokens := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		id, secret, ok := r.BasicAuth()
		if r.URL.Path == "/token" {
			if !ok || id != "caller" || secret != "caller-secret" || r.Form.Get("resource") != resource || r.Form.Get("grant_type") != "client_credentials" {
				http.Error(w, "bad grant", 401)
				return
			}
			fmt.Fprint(w, `{"access_token":"protected-access-token","token_type":"Bearer","expires_in":3600,"scope":"agent:run"}`)
		} else if r.URL.Path == "/introspect" {
			if !ok || id != "resource" || secret != "resource-secret" {
				http.Error(w, "bad resource credential", 401)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"active": r.Form.Get("token") == "protected-access-token", "client_id": "caller", "iss": issuer, "aud": resource, "scope": "agent:run", "exp": time.Now().Add(time.Minute).Unix()})
		} else if r.URL.Path == "/.well-known/oauth-authorization-server" {
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "token_endpoint": issuer + "/token", "grant_types_supported": []string{"client_credentials"}, "token_endpoint_auth_methods_supported": []string{"client_secret_basic"}})
		} else {
			http.NotFound(w, r)
		}
	}))
	defer tokens.Close()
	// The existing OAuth CLI uses the OS trust store. Its token fixture uses
	// explicitly admitted loopback HTTP; A2A and introspection still use TLS.
	tokenHTTP := httptest.NewServer(tokens.Config.Handler)
	defer tokenHTTP.Close()
	issuer = tokens.URL
	credentials := filepath.Join(t.TempDir(), "resource.json")
	os.WriteFile(credentials, []byte(`{"client_id":"resource","client_secret":"resource-secret"}`), 0600)
	issuerCA := certificateFile(t, tokens.Certificate())
	auth, err := NewAuth(AuthConfig{Mode: "oauth", Issuer: issuer, IntrospectionURL: issuer + "/introspect", CredentialsFile: credentials, Audience: resource, Scopes: []string{"agent:run"}, CA: issuerCA})
	if err != nil {
		t.Fatal(err)
	}
	cfg.PublicURL = resource
	cfg.Development = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service, err := NewService(ctx, cfg, &a2a.AgentCard{Name: "fixture", Version: "1"}, auth)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Store.Close()
	server.Config.Handler = service.Handler
	server.StartTLS()
	defer server.Close()
	workerCA := certificateFile(t, server.Certificate())
	env := append(os.Environ(), "OAUTH_HOME="+t.TempDir())
	run := func(program string, args []string, input []byte, want int) []byte {
		t.Helper()
		deadline, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		cmd := exec.CommandContext(deadline, program, args...)
		cmd.Env = env
		cmd.Stdin = bytes.NewReader(input)
		var out, diag bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &diag
		err := cmd.Run()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		if code != want {
			t.Fatalf("%s: %d want %d: %s %s", filepath.Base(program), code, want, out.String(), diag.String())
		}
		if strings.Contains(out.String()+diag.String(), "protected-access-token") || strings.Contains(out.String()+diag.String(), "resource-secret") {
			t.Fatal("credential escaped")
		}
		return out.Bytes()
	}
	// Discovery and token acquisition remain OAuth's job; stdin is untouched by with.
	response, err := server.Client().Get(resource + "/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatal(err)
	}
	metadata, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !bytes.Contains(metadata, []byte(issuer)) {
		t.Fatalf("resource metadata: %s", metadata)
	}
	run(oauth, []string{"login", "worker", "-allow-private", "-flow", "client-credentials", "-client-id", "caller", "-client-auth", "client_secret_basic", "-client-secret-stdin", "-issuer", issuer, "-token-endpoint", tokenHTTP.URL + "/token", "-scope", "agent:run", resource}, []byte("caller-secret\n"), 0)
	raw, _ := json.Marshal(message("private request\n"))
	out := run(oauth, []string{"with", "worker", "--", filter, "request", "-header-fd", "3", "-ca", workerCA, "-timeout", "10s", "send", resource + "/rpc"}, raw, 75)
	var task a2a.Task
	if json.Unmarshal(out, &task) != nil || task.Status.State != a2a.TaskStateInputRequired {
		t.Fatal(string(out))
	}
	follow := message("resumed\n")
	follow.Message.TaskID = task.ID
	raw, _ = json.Marshal(follow)
	out = run(oauth, []string{"with", "worker", "--", filter, "request", "-header-fd", "3", "-ca", workerCA, "send", resource + "/rpc"}, raw, 0)
	if !bytes.Contains(out, []byte(`private request\nresumed\n`)) {
		t.Fatal(string(out))
	}
}
