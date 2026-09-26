package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var version = "0.1.0-dev"

type config struct {
	Moniker                                        string
	TailscaleOrigin, TailscaleUser                 string
	Source, Data, Addr, Hire, Agent, Python, Model string
	WebAttach                                      string
	Assistant                                      string
	Ask, Record                                    string
	AllowBuild                                     bool
	AllowRun                                       bool
	Plonk, PlonkURL, PlonkTokenFile                string
}

func main() {
	if len(os.Args) == 3 && os.Args[1] == "delivery" {
		os.Exit(deliveryProcess(os.Args[2]))
	}
	if len(os.Args) == 3 && os.Args[1] == "task" {
		os.Exit(taskProcess(os.Args[2]))
	}
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println("hire-ui " + version)
		return
	}
	var cfg config
	ask := os.Getenv("AGENT_ASK")
	if ask == "" {
		ask = "ask"
	}
	flag.StringVar(&cfg.Source, "source", "", "selected Bench source checkout (required)")
	flag.StringVar(&cfg.Data, "data", "", "private runtime directory outside source (required)")
	flag.StringVar(&cfg.Addr, "addr", "127.0.0.1:8787", "loopback listen address")
	flag.StringVar(&cfg.Moniker, "moniker", "moniker", "public name generator for temporary teams")
	flag.StringVar(&cfg.Hire, "hire", "hire", "installed Hire executable")
	flag.StringVar(&cfg.Assistant, "assistant", "", "selected bench-hire definition (default SOURCE/workers/bench-hire/expert)")
	flag.StringVar(&cfg.Ask, "ask", ask, "public Ask executable or configured wrapper for issue analysis")
	flag.StringVar(&cfg.Record, "record", "record", "public Record executable for issue analysis evidence")
	flag.StringVar(&cfg.Agent, "agent", "agent", "installed Agent executable for workers and conversation")
	flag.StringVar(&cfg.WebAttach, "web-attach", "", "explicit loopback browser endpoint offered to worker runs (browser must already be running)")
	flag.BoolVar(&cfg.AllowRun, "allow-run", false, "enable explicit worker runs through Agent")
	flag.StringVar(&cfg.Python, "python", "python3", "Python executable for the selected catalog command")
	flag.StringVar(&cfg.Model, "model", os.Getenv("ASK_MODEL"), "default provider/model for authoring (defaults to ASK_MODEL)")
	flag.BoolVar(&cfg.AllowBuild, "allow-build", false, "enable conversation, analysis and model-backed authoring")
	flag.StringVar(&cfg.Plonk, "plonk", "", "selected Plonk executable for results and sharing")
	flag.StringVar(&cfg.PlonkURL, "plonk-url", "", "selected Plonk delivery service")
	flag.StringVar(&cfg.PlonkTokenFile, "plonk-token-file", "", "private connection file; never passed to workers")
	flag.StringVar(&cfg.TailscaleOrigin, "tailscale-origin", "", "exact HTTPS origin served by a local Tailscale Serve proxy")
	flag.StringVar(&cfg.TailscaleUser, "tailscale-user", "", "Tailscale login permitted to use the remote interface")
	flag.Parse()
	if flag.NArg() != 0 {
		slog.Error("unexpected positional arguments")
		os.Exit(2)
	}
	if err := run(cfg); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func prepare(cfg config) (config, error) {
	if _, err := tailscaleHost(cfg); err != nil {
		return cfg, err
	}
	if err := validateDeliveryConfig(cfg); err != nil {
		return cfg, err
	}
	if cfg.Plonk != "" {
		p, err := exec.LookPath(cfg.Plonk)
		if err != nil {
			return cfg, err
		}
		cfg.Plonk, err = filepath.Abs(p)
		if err != nil {
			return cfg, err
		}
		cfg.PlonkTokenFile, err = filepath.Abs(cfg.PlonkTokenFile)
		if err != nil {
			return cfg, err
		}
		cfg.PlonkTokenFile, err = filepath.EvalSymlinks(cfg.PlonkTokenFile)
		if err != nil {
			return cfg, err
		}
	}
	if cfg.WebAttach != "" {
		if _, err := browserAddress(cfg.WebAttach); err != nil {
			return cfg, err
		}
	}
	if cfg.Source == "" || cfg.Data == "" {
		return cfg, fmt.Errorf("select -source and -data explicitly")
	}
	var err error
	cfg.Source, err = filepath.Abs(cfg.Source)
	if err != nil {
		return cfg, err
	}
	cfg.Source, err = filepath.EvalSymlinks(cfg.Source)
	if err != nil {
		return cfg, err
	}
	// Normalize through Git as source may have been a subdirectory.
	root, err := capture("git", "-C", cfg.Source, "rev-parse", "--show-toplevel")
	if err != nil {
		return cfg, err
	}
	cfg.Source = strings.TrimSpace(string(root))
	cfg.Data, err = filepath.Abs(cfg.Data)
	if err != nil {
		return cfg, err
	}
	cfg.Data, err = resolveNewPath(cfg.Data)
	if err != nil {
		return cfg, err
	}
	if within(cfg.Source, cfg.Data) {
		return cfg, fmt.Errorf("-data must be outside the source checkout")
	}
	if err := os.MkdirAll(cfg.Data, 0700); err != nil {
		return cfg, err
	}
	cfg.Data, err = filepath.EvalSymlinks(cfg.Data)
	if err != nil {
		return cfg, err
	}
	if within(cfg.Source, cfg.Data) {
		return cfg, fmt.Errorf("-data resolves inside the source checkout")
	}
	info, err := os.Stat(cfg.Data)
	if err != nil {
		return cfg, err
	}
	if info.Mode().Perm()&0077 != 0 {
		return cfg, fmt.Errorf("-data must be private (permissions 0700)")
	}
	for _, p := range []*string{&cfg.Hire, &cfg.Python} {
		resolved, err := exec.LookPath(*p)
		if err != nil {
			return cfg, err
		}
		*p, err = filepath.Abs(resolved)
		if err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Resolve the existing ancestor before creating a new directory. A symlink
// must not cause even a rejected data path to write inside the source tree.
func resolveNewPath(path string) (string, error) {
	ancestor := path
	var missing []string
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", err
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	return resolved, nil
}

func run(cfg config) error {
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("-addr must use a loopback IP address")
	}
	cfg, err = prepare(cfg)
	if err != nil {
		return err
	}
	cat, err := loadCatalog(cfg.Source, cfg.Python)
	if err != nil {
		return err
	}
	jobs, err := newJobManager(cfg.Data)
	if err != nil {
		return err
	}
	defer jobs.Close()
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	a, err := newApp(cfg, cat, jobs, listener.Addr().String())
	if err != nil {
		return err
	}
	server := &http.Server{Handler: a.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	slog.Info("Hire interface ready", "url", "http://"+listener.Addr().String(), "source", cfg.Source, "data", cfg.Data, "model_builds", cfg.AllowBuild)
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}
	return nil
}
