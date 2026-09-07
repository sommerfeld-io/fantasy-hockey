// Package store owns all database access for the application. It exports the
// shared entity types (Participant, LoginCode) and the queries every
// higher-level package needs; nothing above it in the dependency graph
// redefines its own version of a store-owned entity.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver used by Migrate

	"github.com/golang-migrate/migrate/v4"
)

// normalizeEmail trims and lowercases an email address so lookups and writes
// never diverge on casing alone - a Participant who types their email with
// different casing than the seeded value must still match.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ErrParticipantNotFound is returned by ParticipantByEmail when no Participant
// is registered under the given email. Callers must not translate this into a
// different response than a match to avoid leaking which emails are
// registered.
var ErrParticipantNotFound = errors.New("store: participant not found")

// Participant is one of the three fixed people allowed to use the app.
type Participant struct {
	ID        string
	Slot      int
	Name      string
	Email     string
	CreatedAt time.Time
}

// LoginCode records one issued one-time login code. The plaintext code is
// never stored - only CodeHash (sha256 of the code). UsedAt is nil until the
// code is redeemed.
type LoginCode struct {
	ID            string
	ParticipantID string
	CodeHash      string
	IssuedAt      time.Time
	UsedAt        *time.Time
}

// Store wraps a PostgreSQL connection pool and provides the application's
// persistence operations.
type Store struct {
	pool *pgxpool.Pool
	dsn  string
}

// NewStore opens a connection pool to the database identified by dsn and
// verifies it is reachable.
func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping database: %w", err)
	}

	return &Store{pool: pool, dsn: dsn}, nil
}

// Close releases the underlying connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// Migrate applies every pending embedded migration. It uses its own
// short-lived database/sql connection (golang-migrate's driver contract)
// separate from the pool used for regular queries. golang-migrate's Up() has
// no context-aware variant, so it runs on a goroutine bounded by ctx: an
// unresponsive database fails fast at startup instead of hanging forever.
func (s *Store) Migrate(ctx context.Context) error {
	db, err := sql.Open("pgx", s.dsn)
	if err != nil {
		return fmt.Errorf("store: open migration connection: %w", err)
	}
	defer db.Close()

	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("store: create migration driver: %w", err)
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("store: load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("store: init migrator: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			done <- fmt.Errorf("store: run migrations: %w", err)
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("store: run migrations: %w", ctx.Err())
	}
}

// ParticipantByEmail looks up the Participant registered under email. It
// returns ErrParticipantNotFound if no Participant matches - callers must
// treat that the same as any other lookup failure from an enumeration
// standpoint.
func (s *Store) ParticipantByEmail(ctx context.Context, email string) (Participant, error) {
	const query = `SELECT id, slot, name, email, created_at FROM participant WHERE email = $1`

	var p Participant
	row := s.pool.QueryRow(ctx, query, normalizeEmail(email))
	if err := row.Scan(&p.ID, &p.Slot, &p.Name, &p.Email, &p.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Participant{}, ErrParticipantNotFound
		}
		return Participant{}, fmt.Errorf("store: lookup participant by email: %w", err)
	}

	return p, nil
}

// InsertLoginCode persists a newly issued login code. Multiple simultaneously
// valid codes per Participant are expected: this never invalidates or
// overwrites earlier rows.
func (s *Store) InsertLoginCode(ctx context.Context, code LoginCode) error {
	const query = `
		INSERT INTO login_code (id, participant_id, code_hash, issued_at, used_at)
		VALUES ($1, $2, $3, $4, $5)`

	if _, err := s.pool.Exec(ctx, query, code.ID, code.ParticipantID, code.CodeHash, code.IssuedAt, code.UsedAt); err != nil {
		return fmt.Errorf("store: insert login code: %w", err)
	}

	return nil
}

// ParticipantSeed is one (slot, name, email) triple to upsert via
// UpsertParticipants.
type ParticipantSeed struct {
	Slot  int
	Name  string
	Email string
}

// UpsertParticipants creates or updates the Participants assigned to each
// given slot (1-3), in a single transaction so a failure partway through
// never leaves some slots updated and others not. It is idempotent so it can
// safely run on every startup: an email change in the env vars just needs a
// redeploy. The id generated here is only used on first insert - an update on
// conflict leaves the existing row's id untouched.
func (s *Store) UpsertParticipants(ctx context.Context, seeds []ParticipantSeed) error {
	const query = `
		INSERT INTO participant (id, slot, name, email)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (slot) DO UPDATE SET name = EXCLUDED.name, email = EXCLUDED.email`

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin upsert participants transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit is a documented no-op

	for _, seed := range seeds {
		email := normalizeEmail(seed.Email)
		if _, err := tx.Exec(ctx, query, uuid.NewString(), seed.Slot, seed.Name, email); err != nil {
			return fmt.Errorf("store: upsert participant slot %d: %w", seed.Slot, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit upsert participants transaction: %w", err)
	}

	return nil
}
