// Command fantasy-hockey starts the application: it resolves the configured
// port and data file, wires up the store and mailer, and serves the web UI.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/server"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// config holds every value main.go resolves before wiring dependencies
// together.
type config struct {
	port     int
	dataFile string
	secret   string
}

// resolveConfig parses --port/-p and --data-file from args in a single pass
// - net/http's flag package errors on any flag it doesn't recognize, so
// every CLI flag the binary accepts has to be declared together here rather
// than split across independent flag.FlagSets. --data-file wins over
// DATA_FILE when both are set (AD-25); the port default and flag names
// match internal/server.ResolvePort exactly, so a caller sees identical
// port behavior either way. SESSION_SECRET has no flag or default: it signs
// session cookies (internal/auth.IssueSessionCookie), so an unset value
// fails startup rather than silently generating one in-process.
func resolveConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("fantasy-hockey", flag.ContinueOnError)
	port := fs.Int("port", server.DefaultPort, "port to listen on")
	fs.IntVar(port, "p", server.DefaultPort, "port to listen on (shorthand for --port)")
	dataFile := fs.String("data-file", "", "path to the data file (overrides DATA_FILE)")
	if err := fs.Parse(args); err != nil {
		return config{}, fmt.Errorf("parse flags: %w", err)
	}

	resolvedDataFile := os.Getenv("DATA_FILE")
	if *dataFile != "" {
		resolvedDataFile = *dataFile
	}
	if resolvedDataFile == "" {
		resolvedDataFile = store.DataFileName
	}

	secret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if secret == "" {
		return config{}, fmt.Errorf("SESSION_SECRET environment variable is required")
	}

	return config{port: *port, dataFile: resolvedDataFile, secret: secret}, nil
}

// openStore opens the data file and logs one warning per malformed
// hand-recorded result value (store.ResultProblems). A bad value never stops
// startup: the store ignores it, so it simply scores nothing. A results
// entry of the wrong YAML shape (e.g. a scalar where a list belongs) still
// fails to load and is returned as an error.
func openStore(path string, logger *slog.Logger) (*store.Store, error) {
	st, err := store.New(path)
	if err != nil {
		return nil, fmt.Errorf("open data file %s: %w", path, err)
	}
	for _, problem := range st.ResultProblems() {
		logger.Warn("malformed result in data file", "file", path, "problem", problem)
	}
	return st, nil
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := resolveConfig(os.Args[1:])
	if err != nil {
		return err
	}

	st, err := openStore(cfg.dataFile, slog.Default())
	if err != nil {
		return err
	}

	send := mailer.NewSMTPSender(
		os.Getenv("SMTP_HOST"),
		os.Getenv("SMTP_PORT"),
		os.Getenv("SMTP_USERNAME"),
		os.Getenv("SMTP_APP_PASSWORD"),
	)

	return server.Run(ctx, cfg.port, web.NewServer(st, send, cfg.secret))
}

func main() {
	if err := run(); err != nil {
		slog.Error("fantasy-hockey exited", "error", err)
		os.Exit(1)
	}
}
