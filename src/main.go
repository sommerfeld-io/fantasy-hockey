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
	"syscall"

	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/server"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// defaultDataFile is used when neither DATA_FILE nor --data-file is given.
const defaultDataFile = "fantasy-hockey.yml"

// config holds every value main.go resolves before wiring dependencies
// together.
type config struct {
	port     int
	dataFile string
}

// resolveConfig parses --port/-p and --data-file from args in a single pass
// - net/http's flag package errors on any flag it doesn't recognize, so
// every CLI flag the binary accepts has to be declared together here rather
// than split across independent flag.FlagSets. --data-file wins over
// DATA_FILE when both are set (AD-25); the port default and flag names
// match internal/server.ResolvePort exactly, so a caller sees identical
// port behavior either way.
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
		resolvedDataFile = defaultDataFile
	}

	return config{port: *port, dataFile: resolvedDataFile}, nil
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := resolveConfig(os.Args[1:])
	if err != nil {
		return err
	}

	st, err := store.New(cfg.dataFile)
	if err != nil {
		return fmt.Errorf("open data file %s: %w", cfg.dataFile, err)
	}

	send := mailer.NewSMTPSender(
		os.Getenv("SMTP_HOST"),
		os.Getenv("SMTP_PORT"),
		os.Getenv("SMTP_USERNAME"),
		os.Getenv("SMTP_APP_PASSWORD"),
	)

	return server.Run(ctx, cfg.port, web.NewServer(st, send))
}

func main() {
	if err := run(); err != nil {
		slog.Error("fantasy-hockey exited", "error", err)
		os.Exit(1)
	}
}
