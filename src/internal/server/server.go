// Package server resolves the HTTP port to listen on and runs the HTTP
// server, including startup logging and graceful shutdown.
package server

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// DefaultPort is used when neither --port nor -p is given.
const DefaultPort = 8080

// shutdownTimeout bounds how long in-flight requests get to finish once ctx
// is done.
const shutdownTimeout = 10 * time.Second

// readHeaderTimeout, readTimeout, writeTimeout, and idleTimeout are
// conservative defaults that keep a slow or malicious client from holding a
// connection open indefinitely.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

// ResolvePort parses --port/-p from args, falling back to DefaultPort when
// neither is given.
func ResolvePort(args []string) (int, error) {
	fs := flag.NewFlagSet("fantasy-hockey", flag.ContinueOnError)
	port := fs.Int("port", DefaultPort, "port to listen on")
	fs.IntVar(port, "p", DefaultPort, "port to listen on (shorthand for --port)")
	if err := fs.Parse(args); err != nil {
		return 0, fmt.Errorf("server: parse flags: %w", err)
	}
	return *port, nil
}

// Addr formats port as a net/http listen address, e.g. ":8080".
func Addr(port int) string {
	return fmt.Sprintf(":%d", port)
}

// Run starts handler on port, logging the port it is listening on so it is
// clearly visible at startup, and blocks until ctx is done or the server
// fails. On ctx.Done it shuts down gracefully within shutdownTimeout.
func Run(ctx context.Context, port int, handler http.Handler) error {
	addr := Addr(port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("fantasy-hockey listening", "port", port, "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server: shut down: %w", err)
		}
		return nil
	}
}
