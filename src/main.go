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
	"github.com/sommerfeld-io/fantasy-hockey/internal/observe"
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
// hand-recorded result value (store.ResultProblems), followed by one info
// line summarizing the total problem count found (including zero, AC4). A
// bad value never stops startup: the store ignores it, so it simply scores
// nothing. A misspelled/unknown key or wrongly shaped entry confined to
// results:/award_finalists: is reported the same way (spec-7-4); a shape
// error anywhere else (players:, teams:, etc.) still fails to load and is
// returned as an error.
func openStore(path string, logger *slog.Logger) (*store.Store, error) {
	st, err := store.New(path)
	if err != nil {
		return nil, fmt.Errorf("open data file %s: %w", path, err)
	}
	problems := st.ResultProblems()
	for _, problem := range problems {
		logger.Warn("malformed result in data file", "file", path, "problem", problem)
	}
	logger.Info("result problems found", "count", len(problems))
	return st, nil
}

// newSender reads the mail settings once, builds the selected transport and
// logs one warning naming (never valuing) the settings it ignores. An invalid
// MAIL_TYPE is an error.
func newSender(getenv func(string) string, logger *slog.Logger) (mailer.Sender, error) {
	settings := mailer.SettingsFromEnv(getenv)
	send, err := mailer.New(settings)
	if err != nil {
		return nil, err
	}
	if ignored := mailer.IgnoredVars(settings); len(ignored) > 0 {
		logger.Warn("ignoring mail settings of the unselected transport", "variables", strings.Join(ignored, ", "))
	}
	return send, nil
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

	send, err := newSender(os.Getenv, slog.Default())
	if err != nil {
		return err
	}

	return server.Run(ctx, cfg.port, web.NewServer(st, send, cfg.secret, observe.New()))
}

func main() {
	if err := run(); err != nil {
		slog.Error("fantasy-hockey exited", "error", err)
		os.Exit(1)
	}
}
