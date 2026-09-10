package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	internalrun "github.com/patrickyoung/oauth/internal/run"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(internalrun.CLI(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
