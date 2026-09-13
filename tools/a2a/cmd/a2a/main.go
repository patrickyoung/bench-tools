package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/patrickyoung/a2a/internal/client"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(client.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
