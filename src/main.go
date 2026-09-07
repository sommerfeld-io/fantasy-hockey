// Command fantasy-hockey starts the application: it resolves the database
// connection, runs migrations, bootstraps the fixed Participants from the
// environment, and serves the web UI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// defaultHTTPAddr is used when HTTP_ADDR is unset.
const defaultHTTPAddr = ":8080"

// startupTimeout bounds how long connecting to the database and running
// migrations may take before the process gives up and fails fast, rather than
// hanging forever against an unreachable host.
const startupTimeout = 30 * time.Second

// shutdownTimeout bounds how long in-flight requests get to finish once a
// termination signal arrives.
const shutdownTimeout = 10 * time.Second

// httpReadHeaderTimeout, httpReadTimeout, httpWriteTimeout, and
// httpIdleTimeout are conservative defaults that keep a slow or malicious
// client from holding a connection open indefinitely.
const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 10 * time.Second
	httpWriteTimeout      = 10 * time.Second
	httpIdleTimeout       = 60 * time.Second
)

// participantSlots is the fixed number of Participants the app supports.
const participantSlots = 3

// participantEnv holds one PARTICIPANT_<slot>_NAME/PARTICIPANT_<slot>_EMAIL
// pair read from the environment.
type participantEnv struct {
	slot  int
	name  string
	email string
}

// resolveDatabaseURL implements the shared connection-string resolution: the
// --database-url flag wins if set, otherwise DATABASE_URL, otherwise the
// process must fail fast rather than start against an empty connection. args
// and getenv are injected so this logic is unit-testable without touching
// real process state.
func resolveDatabaseURL(args []string, getenv func(string) string) (string, error) {
	fs := flag.NewFlagSet("fantasy-hockey", flag.ContinueOnError)
	flagDSN := fs.String("database-url", "", "PostgreSQL connection string (overrides DATABASE_URL)")
	if err := fs.Parse(args); err != nil {
		return "", fmt.Errorf("main: parse flags: %w", err)
	}

	if *flagDSN != "" {
		return *flagDSN, nil
	}
	if envDSN := getenv("DATABASE_URL"); envDSN != "" {
		return envDSN, nil
	}

	return "", fmt.Errorf("main: DATABASE_URL environment variable or --database-url flag must be set")
}

// requireEnv reads key via getenv and fails fast if it is unset or empty.
func requireEnv(getenv func(string) string, key string) (string, error) {
	v := getenv(key)
	if v == "" {
		return "", fmt.Errorf("main: required environment variable %s is not set", key)
	}
	return v, nil
}

// readParticipants reads and validates all 6 PARTICIPANT_*_NAME/_EMAIL vars
// for slots 1-3. Missing any of the 6, or two slots sharing the same email
// (case-insensitively), fails the whole read with a clear error rather than
// letting a duplicate surface later as an opaque database constraint failure.
func readParticipants(getenv func(string) string) ([]participantEnv, error) {
	participants := make([]participantEnv, 0, participantSlots)
	seenEmails := make(map[string]int, participantSlots)

	for slot := 1; slot <= participantSlots; slot++ {
		name, err := requireEnv(getenv, fmt.Sprintf("PARTICIPANT_%d_NAME", slot))
		if err != nil {
			return nil, err
		}
		email, err := requireEnv(getenv, fmt.Sprintf("PARTICIPANT_%d_EMAIL", slot))
		if err != nil {
			return nil, err
		}

		normalized := strings.ToLower(strings.TrimSpace(email))
		if other, ok := seenEmails[normalized]; ok {
			return nil, fmt.Errorf("main: PARTICIPANT_%d_EMAIL and PARTICIPANT_%d_EMAIL must not match", other, slot)
		}
		seenEmails[normalized] = slot

		participants = append(participants, participantEnv{slot: slot, name: name, email: email})
	}

	return participants, nil
}

// config holds everything run needs, resolved and validated up front so the
// process fails fast, before any I/O (DB connection, migrations, listening),
// if anything required is missing.
type config struct {
	databaseURL   string
	participants  []participantEnv
	smtpUsername  string
	smtpPassword  string
	sessionSecret string
}

// loadConfig resolves the connection string and validates every required
// env var. It fails on the first missing value rather than starting with a
// partial configuration.
func loadConfig() (config, error) {
	dsn, err := resolveDatabaseURL(os.Args[1:], os.Getenv)
	if err != nil {
		return config{}, err
	}
	sessionSecret, err := requireEnv(os.Getenv, "SESSION_SECRET")
	if err != nil {
		return config{}, err
	}
	participants, err := readParticipants(os.Getenv)
	if err != nil {
		return config{}, err
	}
	smtpUsername, err := requireEnv(os.Getenv, "SMTP_USERNAME")
	if err != nil {
		return config{}, err
	}
	smtpPassword, err := requireEnv(os.Getenv, "SMTP_APP_PASSWORD")
	if err != nil {
		return config{}, err
	}

	return config{
		databaseURL:   dsn,
		participants:  participants,
		smtpUsername:  smtpUsername,
		smtpPassword:  smtpPassword,
		sessionSecret: sessionSecret,
	}, nil
}

// openStore connects to the database, runs migrations, and upserts the fixed
// Participants - everything that must succeed before the app can serve a
// single request. The whole sequence is bounded by startupTimeout so an
// unreachable database fails fast instead of hanging the process forever.
func openStore(ctx context.Context, cfg config) (*store.Store, error) {
	ctx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	st, err := store.NewStore(ctx, cfg.databaseURL)
	if err != nil {
		return nil, fmt.Errorf("main: connect to database: %w", err)
	}

	if err := st.Migrate(ctx); err != nil {
		st.Close()
		return nil, fmt.Errorf("main: run migrations: %w", err)
	}

	seeds := make([]store.ParticipantSeed, 0, len(cfg.participants))
	for _, p := range cfg.participants {
		seeds = append(seeds, store.ParticipantSeed{Slot: p.slot, Name: p.name, Email: p.email})
	}
	if err := st.UpsertParticipants(ctx, seeds); err != nil {
		st.Close()
		return nil, fmt.Errorf("main: upsert participants: %w", err)
	}

	return st, nil
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	authSvc := auth.NewService(st, mailer.New(cfg.smtpUsername, cfg.smtpPassword), cfg.sessionSecret)
	handler := web.NewServer(authSvc)

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = defaultHTTPAddr
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("fantasy-hockey listening", "addr", addr)
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
			return fmt.Errorf("main: shut down server: %w", err)
		}
		return nil
	}
}

func main() {
	if err := run(); err != nil {
		slog.Error("fantasy-hockey exited", "error", err)
		os.Exit(1)
	}
}
