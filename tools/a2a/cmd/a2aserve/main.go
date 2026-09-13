package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/patrickyoung/a2a/internal/serve"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(serve.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
