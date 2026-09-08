// Command fantasy-hockey starts the application: it resolves the configured
// port and serves the web UI.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sommerfeld-io/fantasy-hockey/internal/server"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port, err := server.ResolvePort(os.Args[1:])
	if err != nil {
		return err
	}

	return server.Run(ctx, port, web.NewServer())
}

func main() {
	if err := run(); err != nil {
		slog.Error("fantasy-hockey exited", "error", err)
		os.Exit(1)
	}
}
