package serve

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/limiter"
	"github.com/patrickyoung/a2a/internal/boundary"
)

const Version = "0.1.0-dev"
const usage = `usage:
  a2aserve [options] CARD.json -- PROGRAM [ARG ...]

Required:
  -state DIR                 private task records, workspaces, and evidence
  -auth FILE                 mTLS or OAuth introspection configuration
  -tls-cert FILE -tls-key FILE

Options:
  -listen ADDR               default 127.0.0.1:8080
  -public-url URL            advertised HTTPS origin (default listener address)
  -tend PROGRAM              existing Tend executable (default tend on PATH)
  -input text|json           worker stdin contract (default text)
  -output text|json          worker stdout contract (default text)
  -artifact PATH             required workspace file to export; repeatable
  -pass-env NAME             explicitly pass a worker variable; repeatable
  -timeout D                Tend job deadline (default 30m)
  -max-active N             concurrent executions (default 8)
  -max-input N              request/worker input byte bound (default 16777216)
  -max-output N             result/artifact export bound (default 67108864)
  -dev-loopback             explicit unauthenticated HTTP, literal loopback only

The configured program runs through Tend with literal argv, in a task-specific
workspace. Agent remains a separate command. A2A requests cannot choose local
programs, credentials, or workspace roots. This listener has no model or goal
loop. Protocol endpoints are /rpc and /rest; the authenticated card is at
/.well-known/agent-card.json. Read README.md for worker and security contracts.
`

type Config struct {
	Listen, PublicURL, State, AuthFile, Cert, Key string
	Development                                   bool
	MaxActive                                     int
	Executor                                      ExecutorConfig
}

type Service struct {
	Handler  http.Handler
	Card     *a2a.AgentCard
	Store    *Store
	Executor *Executor
}

func Run(parent context.Context, args []string, out, diagnostics io.Writer) int {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(out, usage)
		return 0
	}
	if args[0] == "version" {
		fmt.Fprintln(out, "a2aserve "+Version)
		return 0
	}
	var cfg Config
	f := flag.NewFlagSet("a2aserve", flag.ContinueOnError)
	f.SetOutput(diagnostics)
	f.StringVar(&cfg.Listen, "listen", "127.0.0.1:8080", "listen address")
	f.StringVar(&cfg.PublicURL, "public-url", "", "advertised origin")
	f.StringVar(&cfg.State, "state", "", "private state directory")
	f.StringVar(&cfg.AuthFile, "auth", "", "authentication policy")
	f.StringVar(&cfg.Cert, "tls-cert", "", "server certificate")
	f.StringVar(&cfg.Key, "tls-key", "", "server private key")
	f.BoolVar(&cfg.Development, "dev-loopback", false, "explicit unauthenticated loopback HTTP")
	f.IntVar(&cfg.MaxActive, "max-active", 8, "concurrent executions")
	f.StringVar(&cfg.Executor.Tend, "tend", "tend", "Tend executable")
	f.StringVar(&cfg.Executor.Input, "input", "text", "worker input mode")
	f.StringVar(&cfg.Executor.Output, "output", "text", "worker output mode")
	f.DurationVar(&cfg.Executor.Timeout, "timeout", 30*time.Minute, "worker deadline")
	f.Int64Var(&cfg.Executor.MaxInput, "max-input", 16<<20, "input byte bound")
	f.Int64Var(&cfg.Executor.MaxOutput, "max-output", 64<<20, "output byte bound")
	f.Func("artifact", "required artifact path", func(value string) error { cfg.Executor.Artifacts = append(cfg.Executor.Artifacts, value); return nil })
	f.Func("pass-env", "worker environment name", func(value string) error { cfg.Executor.PassEnv = append(cfg.Executor.PassEnv, value); return nil })
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	rest := f.Args()
	if len(rest) < 3 || rest[1] != "--" {
		fmt.Fprintln(diagnostics, "a2aserve: expected CARD -- PROGRAM [ARG ...]")
		return 2
	}
	cfg.Executor.Command = rest[2:]
	card, err := loadCard(rest[0])
	if err != nil {
		fmt.Fprintln(diagnostics, "a2aserve: invalid Agent Card")
		return 2
	}
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintln(diagnostics, "a2aserve:", err)
		return 2
	}
	var auth *Auth
	var tlsConfig *tls.Config
	if !cfg.Development {
		auth, err = LoadAuth(cfg.AuthFile)
		if err != nil {
			fmt.Fprintln(diagnostics, "a2aserve: invalid authentication configuration")
			return 2
		}
		tlsConfig, err = boundary.TLSConfig("", cfg.Cert, cfg.Key)
		if err != nil {
			fmt.Fprintln(diagnostics, "a2aserve: invalid TLS certificate or private key")
			return 2
		}
		tlsConfig = auth.TLS(tlsConfig)
	}
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		fmt.Fprintln(diagnostics, "a2aserve: cannot listen on selected address")
		return 2
	}
	defer listener.Close()
	if cfg.PublicURL == "" {
		scheme := "https"
		if cfg.Development {
			scheme = "http"
		}
		cfg.PublicURL = scheme + "://" + listener.Addr().String()
	}
	u, err := boundary.Endpoint(cfg.PublicURL, cfg.Development)
	if err != nil || (u.Path != "" && u.Path != "/") {
		fmt.Fprintln(diagnostics, "a2aserve: public URL must be an HTTPS origin, or an explicit development loopback origin")
		return 2
	}
	service, err := NewService(ctx, cfg, card, auth)
	if err != nil {
		fmt.Fprintln(diagnostics, "a2aserve:", err)
		return 2
	}
	defer func() {
		cancel()
		deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = service.Executor.Drain(deadline)
		_ = service.Store.Close()
	}()
	server := &http.Server{Handler: service.Handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 64 << 10, ErrorLog: log.New(io.Discard, "", 0)}
	if tlsConfig != nil {
		listener = tls.NewListener(listener, tlsConfig)
	}
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
		_ = server.Close()
	}()
	fmt.Fprintln(diagnostics, "a2aserve: listening", listener.Addr().String(), "card", strings.TrimRight(cfg.PublicURL, "/")+a2asrv.WellKnownAgentCardPath)
	err = server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(diagnostics, "a2aserve: listener stopped with an error")
		return 1
	}
	if ctx.Err() != nil {
		<-closed
	}
	return 0
}

func validateConfig(cfg Config) error {
	if cfg.State == "" || cfg.MaxActive <= 0 || cfg.Executor.MaxInput <= 0 || cfg.Executor.MaxInput > 64<<20 || cfg.Executor.MaxOutput <= 0 || cfg.Executor.MaxOutput > 1<<30 || cfg.Executor.Timeout <= 0 {
		return errors.New("explicit state and positive execution bounds are required; input cannot exceed Tend's 64 MiB boundary")
	}
	if cfg.Development {
		host, _, err := net.SplitHostPort(cfg.Listen)
		if err != nil || !boundary.IsLoopback(host) || cfg.AuthFile != "" || cfg.Cert != "" || cfg.Key != "" {
			return errors.New("development mode requires a literal loopback listener and no production authentication/TLS options")
		}
	} else if cfg.AuthFile == "" || cfg.Cert == "" || cfg.Key == "" {
		return errors.New("production serving requires authentication and a TLS certificate/key")
	}
	return nil
}

func loadCard(path string) (*a2a.AgentCard, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := boundary.ReadObject(f, 1<<20)
	if err != nil {
		return nil, err
	}
	var card a2a.AgentCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, err
	}
	if card.Name == "" || card.Version == "" || card.Description == "" || len(card.Skills) == 0 {
		return nil, errors.New("card requires name, version, description, and skills")
	}
	for _, skill := range card.Skills {
		if skill.ID == "" || skill.Name == "" || skill.Description == "" {
			return nil, errors.New("card skills require identity and description")
		}
	}
	return &card, nil
}

func NewService(ctx context.Context, cfg Config, card *a2a.AgentCard, auth *Auth) (*Service, error) {
	store, err := OpenStore(cfg.State, cfg.Executor.MaxOutput+cfg.Executor.MaxInput+1<<20)
	if err != nil {
		return nil, err
	}
	executor, err := NewExecutor(ctx, store, cfg.Executor)
	if err != nil {
		store.Close()
		return nil, err
	}
	u, err := url.Parse(cfg.PublicURL)
	if err != nil {
		store.Close()
		return nil, err
	}
	base := strings.TrimRight(cfg.PublicURL, "/")
	card.SupportedInterfaces = []*a2a.AgentInterface{{URL: base + "/rpc", ProtocolBinding: a2a.TransportProtocolJSONRPC, ProtocolVersion: a2a.Version}, {URL: base + "/rest", ProtocolBinding: a2a.TransportProtocolHTTPJSON, ProtocolVersion: a2a.Version}}
	card.Capabilities = a2a.AgentCapabilities{Streaming: true, ExtendedAgentCard: true}
	if len(card.DefaultInputModes) == 0 {
		card.DefaultInputModes = []string{"text/plain"}
		if cfg.Executor.Input == "json" {
			card.DefaultInputModes = []string{"application/json"}
		}
	}
	if len(card.DefaultOutputModes) == 0 {
		card.DefaultOutputModes = []string{"text/plain", "application/json", "application/octet-stream"}
	}
	card.Signatures = nil // runtime changes invalidate operator-supplied signatures
	for _, skill := range card.Skills {
		skill.SecurityRequirements = nil
	}
	card.SecuritySchemes = nil
	card.SecurityRequirements = nil
	if auth != nil {
		name := a2a.SecuritySchemeName("credential")
		var scheme a2a.SecurityScheme = a2a.HTTPAuthSecurityScheme{Scheme: "bearer"}
		if auth.config.Mode == "mtls" {
			scheme = a2a.MutualTLSSecurityScheme{}
		}
		card.SecuritySchemes = a2a.NamedSecuritySchemes{name: scheme}
		card.SecurityRequirements = a2a.SecurityRequirementsOptions{{name: {}}}
	}
	handler := a2asrv.NewHandler(executor, a2asrv.WithTaskStore(store), a2asrv.WithCallInterceptors(&admission{store: store}), a2asrv.WithExtendedAgentCard(card), a2asrv.WithCapabilityChecks(&card.Capabilities), a2asrv.WithConcurrencyConfig(limiter.ConcurrencyConfig{MaxExecutions: cfg.MaxActive}), a2asrv.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	mux := http.NewServeMux()
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(card))
	mux.Handle("/rpc", a2asrv.NewJSONRPCHandler(handler))
	mux.Handle("/rest/", http.StripPrefix("/rest", a2asrv.NewRESTHandler(handler)))
	outer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != u.Scheme+"://"+u.Host {
			http.Error(w, "Origin refused", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			typ, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if typ != "application/json" {
				http.Error(w, "JSON content type required", http.StatusUnsupportedMediaType)
				return
			}
			control := http.NewResponseController(w)
			_ = control.SetReadDeadline(time.Now().Add(10 * time.Second))
			body, err := boundary.ReadObject(r.Body, cfg.Executor.MaxInput)
			r.Body.Close()
			_ = control.SetReadDeadline(time.Time{})
			if err != nil {
				http.Error(w, "Invalid bounded JSON request", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		mux.ServeHTTP(w, r)
	})
	var protected http.Handler = outer
	if auth != nil {
		if auth.config.Mode == "oauth" {
			auth.metadataURL = base + "/.well-known/oauth-protected-resource"
		}
		protected = auth.Wrap(outer)
		if auth.config.Mode == "oauth" {
			authenticated := protected
			protected = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// RFC 9728 discovery is public; it grants no task/card access.
				if r.URL.Path == "/.well-known/oauth-protected-resource" && r.Method == http.MethodGet {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"resource": auth.config.Audience, "authorization_servers": []string{auth.config.Issuer}, "bearer_methods_supported": []string{"header"}, "scopes_supported": auth.config.Scopes})
					return
				}
				authenticated.ServeHTTP(w, r)
			})
		}
	} else {
		protected = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			outer.ServeHTTP(w, r.WithContext(ownedContext(r.Context(), "development")))
		})
	}
	return &Service{Handler: protected, Card: card, Store: store, Executor: executor}, nil
}
