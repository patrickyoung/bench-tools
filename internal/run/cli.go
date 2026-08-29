package run

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/patrickyoung/oauth/internal/protocol"
	"github.com/patrickyoung/oauth/internal/state"
)

const Version = "0.1.0"

func CLI(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(argv) == 0 {
		usage(stderr)
		return 2
	}
	var err error
	switch argv[0] {
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "discover":
		err = discover(ctx, argv[1:], stdout, stderr)
	case "login":
		err = login(ctx, argv[1:], stdin, stdout, stderr)
	case "refresh":
		err = refresh(ctx, argv[1:], stdout, stderr)
	case "status", "list":
		err = status(argv[1:], stdout)
	case "logout":
		err = logout(argv[1:], stdout)
	case "header":
		err = header(ctx, argv[1:], stdout, stderr)
	case "with":
		code, runErr := with(ctx, argv[1:], stdin, stdout, stderr)
		if runErr != nil {
			fmt.Fprintf(stderr, "oauth: %v\n", runErr)
		}
		return code
	default:
		fmt.Fprintf(stderr, "oauth: unknown command %q\n", argv[0])
		usage(stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "oauth: %v\n", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage:
  oauth discover [flags] RESOURCE
  oauth login NAME [flags] RESOURCE
  oauth refresh [flags] NAME
  oauth status [NAME]
  oauth logout NAME
  oauth header [flags] NAME
  oauth with [flags] NAME -- COMMAND [ARG ...]

OAuth authorization for ordinary Unix programs. login supports authorization
code with PKCE, device authorization, and client credentials. with refreshes
before execution, writes one Authorization header to the child's descriptor 3,
and never retries the command. Tokens never appear in argv or the environment.
`)
}

type common struct {
	issuer       string
	timeout      time.Duration
	allowPrivate bool
}

func commonFlags(fs *flag.FlagSet, c *common) {
	fs.StringVar(&c.issuer, "issuer", "", "exact authorization-server issuer")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Minute, "network or login timeout")
	fs.BoolVar(&c.allowPrivate, "allow-private", false, "allow operator-approved private network endpoints")
}

func discover(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	commonFlags(fs, &c)
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: oauth discover [flags] RESOURCE")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	d, err := protocol.NewClient(c.timeout, c.allowPrivate).Discover(ctx, fs.Arg(0), c.issuer)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

type stringsFlag []string

func (s *stringsFlag) String() string { return strings.Join(*s, " ") }
func (s *stringsFlag) Set(v string) error {
	for _, scope := range strings.Fields(v) {
		if !slices.Contains(*s, scope) {
			*s = append(*s, scope)
		}
	}
	return nil
}

func login(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(argv) == 0 || strings.HasPrefix(argv[0], "-") {
		return errors.New("usage: oauth login NAME [flags] RESOURCE")
	}
	name := argv[0]
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c common
	commonFlags(fs, &c)
	flow := fs.String("flow", "auto", "login flow: auto, code, device, or client-credentials")
	clientID := fs.String("client-id", "", "OAuth client identifier")
	clientAuth := fs.String("client-auth", "auto", "none, client_secret_basic, or client_secret_post")
	authorizationEndpoint := fs.String("authorization-endpoint", "", "explicit authorization endpoint (requires -issuer and -token-endpoint)")
	tokenEndpoint := fs.String("token-endpoint", "", "explicit token endpoint (bypasses metadata discovery)")
	deviceEndpoint := fs.String("device-endpoint", "", "explicit device authorization endpoint")
	secretStdin := fs.Bool("client-secret-stdin", false, "read and store client secret from stdin")
	secretFile := fs.String("client-secret-file", "", "read and store client secret from a protected file")
	noBrowser := fs.Bool("no-browser", false, "print authorization URL without opening a browser")
	replace := fs.Bool("replace", false, "replace an existing profile")
	var scopes stringsFlag
	fs.Var(&scopes, "scope", "requested scope; repeatable or space-separated")
	if err := fs.Parse(argv[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 || *clientID == "" {
		return errors.New("login requires RESOURCE and -client-id")
	}
	if *secretStdin && *secretFile != "" {
		return errors.New("choose only one client secret source")
	}
	secret, err := readSecret(stdin, *secretStdin, *secretFile)
	if err != nil {
		return err
	}
	method := *clientAuth
	if method == "auto" {
		if secret == "" {
			method = "none"
		} else {
			method = "client_secret_basic"
		}
	}
	if !slices.Contains([]string{"none", "client_secret_basic", "client_secret_post"}, method) {
		return fmt.Errorf("unsupported client authentication method %q", method)
	}
	if method != "none" && secret == "" {
		return fmt.Errorf("%s requires -client-secret-stdin or -client-secret-file", method)
	}
	if !*replace {
		if _, err := state.LoadProfile(name); err == nil {
			return fmt.Errorf("profile %q already exists (use -replace)", name)
		} else if !errors.Is(err, os.ErrNotExist) {
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || !errors.Is(pathErr.Err, os.ErrNotExist) {
				return err
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	client := protocol.NewClient(c.timeout, c.allowPrivate)
	var d protocol.Discovery
	if *tokenEndpoint == "" {
		d, err = client.Discover(ctx, fs.Arg(0), c.issuer)
		if err != nil {
			return err
		}
	} else {
		if c.issuer == "" {
			return errors.New("explicit endpoints require -issuer")
		}
		for label, endpoint := range map[string]string{
			"resource": fs.Arg(0), "issuer": c.issuer, "authorization": *authorizationEndpoint,
			"token": *tokenEndpoint, "device": *deviceEndpoint,
		} {
			if endpoint == "" {
				continue
			}
			if _, validateErr := protocol.ValidateEndpoint(endpoint); validateErr != nil {
				return fmt.Errorf("%s endpoint: %w", label, validateErr)
			}
		}
		d = protocol.Discovery{
			Resource: protocol.ResourceMetadata{Resource: fs.Arg(0), ScopesSupported: scopes},
			Server: protocol.ServerMetadata{
				Issuer: c.issuer, AuthorizationEndpoint: *authorizationEndpoint, TokenEndpoint: *tokenEndpoint,
				DeviceAuthorizationEndpoint: *deviceEndpoint, CodeChallengeMethodsSupported: []string{"S256"},
			},
		}
	}
	if len(scopes) == 0 {
		scopes = append(scopes, d.Resource.ScopesSupported...)
	}
	p := state.Profile{
		Name:                        name,
		Resource:                    d.Resource.Resource,
		Issuer:                      d.Server.Issuer,
		AuthorizationEndpoint:       d.Server.AuthorizationEndpoint,
		TokenEndpoint:               d.Server.TokenEndpoint,
		DeviceEndpoint:              d.Server.DeviceAuthorizationEndpoint,
		RegistrationEndpoint:        d.Server.RegistrationEndpoint,
		ClientID:                    *clientID,
		ClientAuth:                  method,
		Scopes:                      scopes,
		AuthorizationResponseIssuer: d.Server.AuthorizationResponseIssuerParamSupported,
	}
	if len(d.Server.TokenEndpointAuthMethodsSupported) > 0 && !slices.Contains(d.Server.TokenEndpointAuthMethodsSupported, method) {
		return fmt.Errorf("authorization server does not advertise client authentication method %q", method)
	}
	selected := *flow
	if selected == "auto" {
		switch {
		case p.AuthorizationEndpoint != "" && slices.Contains(d.Server.CodeChallengeMethodsSupported, "S256"):
			selected = "code"
		case p.DeviceEndpoint != "":
			selected = "device"
		case method != "none" && (len(d.Server.GrantTypesSupported) == 0 || slices.Contains(d.Server.GrantTypesSupported, "client_credentials")):
			selected = "client-credentials"
		default:
			return errors.New("authorization server advertises no supported login flow")
		}
	}
	var tok protocol.Token
	switch selected {
	case "code":
		if p.AuthorizationEndpoint == "" || !slices.Contains(d.Server.CodeChallengeMethodsSupported, "S256") {
			return errors.New("authorization-code login requires an authorization endpoint advertising PKCE S256")
		}
		tok, err = codeLogin(ctx, client, p, secret, *noBrowser, stderr)
	case "device":
		tok, err = deviceLogin(ctx, client, p, secret, *noBrowser, stderr)
	case "client-credentials":
		if method == "none" {
			return errors.New("client credentials requires confidential client authentication")
		}
		tok, err = client.ClientCredentials(ctx, p, secret)
	default:
		return fmt.Errorf("unknown login flow %q", selected)
	}
	if err != nil {
		return err
	}
	cred := protocol.CredentialFromToken(tok, secret, time.Now(), "", p.Scopes)
	release, err := state.Lock(name)
	if err != nil {
		return err
	}
	defer release()
	if !*replace {
		if _, loadErr := state.LoadProfile(name); loadErr == nil {
			return fmt.Errorf("profile %q was created while login was in progress", name)
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			var pathErr *os.PathError
			if !errors.As(loadErr, &pathErr) || !errors.Is(pathErr.Err, os.ErrNotExist) {
				return loadErr
			}
		}
	}
	if err := state.Save(name, p, cred); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "logged in %s\n", name)
	return nil
}

func readSecret(stdin io.Reader, fromStdin bool, path string) (string, error) {
	if !fromStdin && path == "" {
		return "", nil
	}
	var r io.Reader = stdin
	if path != "" {
		fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if err != nil {
			return "", err
		}
		f := os.NewFile(uintptr(fd), path)
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return "", err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || !ok || stat.Nlink != 1 {
			return "", errors.New("client secret file must be a singly-linked regular file with mode 0600 or stricter")
		}
		r = f
	}
	b, err := io.ReadAll(io.LimitReader(r, 64<<10+1))
	if err != nil {
		return "", err
	}
	if len(b) > 64<<10 {
		return "", errors.New("client secret exceeds 64 KiB")
	}
	secret := strings.TrimSpace(string(b))
	if secret == "" {
		return "", errors.New("client secret is empty")
	}
	return secret, nil
}

type callbackResult struct {
	code string
	iss  string
	err  error
}

func codeLogin(ctx context.Context, client *protocol.Client, p state.Profile, secret string, noBrowser bool, stderr io.Writer) (protocol.Token, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return protocol.Token{}, err
	}
	defer listener.Close()
	redirect := "http://" + listener.Addr().String() + "/callback"
	verifier, challenge, err := protocol.PKCE()
	if err != nil {
		return protocol.Token{}, err
	}
	stateValue, err := protocol.RandomURLToken(32)
	if err != nil {
		return protocol.Token{}, err
	}
	authorize, err := protocol.AuthorizationURL(p, redirect, challenge, stateValue)
	if err != nil {
		return protocol.Token{}, err
	}
	result := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !protocol.SecureEqual(q.Get("state"), stateValue) {
			result <- callbackResult{err: errors.New("authorization response state did not match")}
			http.Error(w, "Authorization state did not match. Return to the terminal.", http.StatusBadRequest)
			return
		}
		if code := q.Get("error"); code != "" {
			result <- callbackResult{err: fmt.Errorf("authorization %s: %s", code, q.Get("error_description"))}
			http.Error(w, "Authorization was not granted. Return to the terminal.", http.StatusBadRequest)
			return
		}
		result <- callbackResult{code: q.Get("code"), iss: q.Get("iss")}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<!doctype html><title>OAuth complete</title><p>Authorization complete. You may close this window.</p>")
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Shutdown(context.Background())
	fmt.Fprintf(stderr, "oauth: open this URL to authorize %s:\n%s\n", p.Name, authorize)
	if !noBrowser {
		if err := openBrowser(authorize); err != nil {
			fmt.Fprintf(stderr, "oauth: could not open browser: %v\n", err)
		}
	}
	select {
	case <-ctx.Done():
		return protocol.Token{}, ctx.Err()
	case got := <-result:
		if got.err != nil {
			return protocol.Token{}, got.err
		}
		if got.code == "" {
			return protocol.Token{}, errors.New("authorization response contained no code")
		}
		if p.AuthorizationResponseIssuer && got.iss == "" {
			return protocol.Token{}, errors.New("authorization response omitted required issuer")
		}
		if got.iss != "" && got.iss != p.Issuer {
			return protocol.Token{}, fmt.Errorf("authorization response issuer %q does not match %q", got.iss, p.Issuer)
		}
		return client.ExchangeCode(ctx, p, secret, got.code, verifier, redirect)
	}
}

func deviceLogin(ctx context.Context, client *protocol.Client, p state.Profile, secret string, noBrowser bool, stderr io.Writer) (protocol.Token, error) {
	device, err := client.StartDevice(ctx, p, secret)
	if err != nil {
		return protocol.Token{}, err
	}
	open := device.VerificationURIComplete
	if open == "" {
		open = device.VerificationURI
	}
	fmt.Fprintf(stderr, "oauth: open %s and enter code %s\n", device.VerificationURI, device.UserCode)
	if !noBrowser {
		if err := openBrowser(open); err != nil {
			fmt.Fprintf(stderr, "oauth: could not open browser: %v\n", err)
		}
	}
	return client.PollDevice(ctx, p, secret, device, nil)
}

func openBrowser(target string) error {
	var argv []string
	switch runtime.GOOS {
	case "darwin":
		argv = []string{"open", target}
	case "windows":
		argv = []string{"rundll32", "url.dll,FileProtocolHandler", target}
	default:
		argv = []string{"xdg-open", target}
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func refresh(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.Duration("timeout", 30*time.Second, "refresh timeout")
	allowPrivate := fs.Bool("allow-private", false, "allow operator-approved private network endpoints")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: oauth refresh [flags] NAME")
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	if _, err := refreshLocked(ctx, fs.Arg(0), true, *timeout, *allowPrivate); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "refreshed %s\n", fs.Arg(0))
	return nil
}

func refreshLocked(ctx context.Context, name string, force bool, timeout time.Duration, allowPrivate bool) (state.Credential, error) {
	release, err := state.Lock(name)
	if err != nil {
		return state.Credential{}, err
	}
	defer release()
	p, c, err := state.Load(name)
	if err != nil {
		return c, err
	}
	if !force && protocol.Usable(c, time.Now()) {
		return c, nil
	}
	client := protocol.NewClient(timeout, allowPrivate)
	tok, err := client.Refresh(ctx, p, c.ClientSecret, c.RefreshToken)
	if err != nil {
		return c, err
	}
	next := protocol.CredentialFromToken(tok, c.ClientSecret, time.Now(), c.RefreshToken, c.GrantedScopes)
	if err := state.SaveCredential(name, next); err != nil {
		return c, err
	}
	return next, nil
}

func credential(ctx context.Context, name string, timeout time.Duration, allowPrivate bool) (state.Profile, state.Credential, error) {
	p, c, err := state.Load(name)
	if err != nil {
		return p, c, err
	}
	if protocol.Usable(c, time.Now()) {
		return p, c, nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c, err = refreshLocked(ctx, name, false, timeout, allowPrivate)
	return p, c, err
}

func status(argv []string, stdout io.Writer) error {
	if len(argv) > 1 {
		return errors.New("usage: oauth status [NAME]")
	}
	names := argv
	if len(names) == 0 {
		var err error
		names, err = state.List()
		if err != nil {
			return err
		}
	}
	for _, name := range names {
		p, c, err := state.Load(name)
		if err != nil {
			return err
		}
		condition := "expired"
		if protocol.Usable(c, time.Now()) {
			condition = "ready"
		} else if c.RefreshToken != "" {
			condition = "refreshable"
		}
		expires := "none"
		if !c.Expiry.IsZero() {
			expires = c.Expiry.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", name, condition, expires, p.Resource, strings.Join(p.Scopes, " "))
	}
	return nil
}

func logout(argv []string, stdout io.Writer) error {
	if len(argv) != 1 {
		return errors.New("usage: oauth logout NAME")
	}
	if err := state.Delete(argv[0]); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "logged out %s\n", argv[0])
	return nil
}

func authFlags(name string, argv []string, stderr io.Writer) (string, time.Duration, bool, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.Duration("timeout", 30*time.Second, "refresh timeout")
	allowPrivate := fs.Bool("allow-private", false, "allow operator-approved private network endpoints")
	if err := fs.Parse(argv); err != nil {
		return "", 0, false, nil, err
	}
	if fs.NArg() == 0 {
		return "", 0, false, nil, fmt.Errorf("%s requires a profile name", name)
	}
	return fs.Arg(0), *timeout, *allowPrivate, fs.Args()[1:], nil
}

func header(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
	name, timeout, allowPrivate, rest, err := authFlags("header", argv, stderr)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return errors.New("usage: oauth header [flags] NAME")
	}
	_, c, err := credential(ctx, name, timeout, allowPrivate)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Authorization: Bearer %s\n", c.AccessToken)
	return err
}

func with(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	sep := slices.Index(argv, "--")
	if sep < 0 || sep == len(argv)-1 {
		return 2, errors.New("usage: oauth with [flags] NAME -- COMMAND [ARG ...]")
	}
	name, timeout, allowPrivate, rest, err := authFlags("with", argv[:sep], stderr)
	if err != nil {
		return 2, err
	}
	if len(rest) != 0 {
		return 2, errors.New("unexpected arguments before --")
	}
	_, c, err := credential(ctx, name, timeout, allowPrivate)
	if err != nil {
		return 1, err
	}
	return Command(ctx, argv[sep+1:], "Authorization: Bearer "+c.AccessToken+"\n", stdin, stdout, stderr)
}
