package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// slotSeq guarantees each test gets its own slot number, avoiding the unique
// constraint even when tests run against a persisted (not recreated) database.
var slotSeq = time.Now().Unix() % 20000

func nextSlot() int {
	return int(atomic.AddInt64(&slotSeq, 1))
}

func uniqueEmail(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("store-test-%d@example.invalid", time.Now().UnixNano())
}

// requireTestDSN skips the calling test when POSTGRES_TEST_DSN is unset, so
// task go:test and task go:build never require a live PostgreSQL instance.
func requireTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set; skipping test that requires a real PostgreSQL instance")
	}
	return dsn
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()
	dsn := requireTestDSN(t)

	st, err := store.NewStore(ctx, dsn)
	if err != nil {
		t.Fatalf("NewStore(%q) returned error: %v", dsn, err)
	}
	t.Cleanup(st.Close)

	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	return st
}

// countLoginCodes queries the login_code table directly (bypassing Store's
// own API, which intentionally exposes no read-back method) to verify what
// InsertLoginCode actually persisted.
func countLoginCodes(t *testing.T, dsn, participantID string) int {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM login_code WHERE participant_id = $1`, participantID).Scan(&count); err != nil {
		t.Fatalf("count login codes: %v", err)
	}
	return count
}

func TestNewStoreShouldReturnErrorWhenTheDatabaseIsUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := store.NewStore(ctx, "postgres://user:pass@127.0.0.1:1/db")

	if err == nil {
		t.Fatal("expected NewStore to return an error for an unreachable database")
	}
}

func TestNewStoreShouldNotReturnErrorForAReachableDatabase(t *testing.T) {
	newTestStore(t) // fails the test itself if connecting errors
}

func TestMigrateShouldBeIdempotent(t *testing.T) {
	st := newTestStore(t)

	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("expected re-running Migrate to be a no-op, got error: %v", err)
	}
}

func TestParticipantByEmailShouldReturnTheUpsertedParticipant(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	slot := nextSlot()
	email := uniqueEmail(t)

	if err := st.UpsertParticipants(ctx, []store.ParticipantSeed{{Slot: slot, Name: "Test Participant", Email: email}}); err != nil {
		t.Fatalf("UpsertParticipant returned error: %v", err)
	}

	got, err := st.ParticipantByEmail(ctx, email)
	if err != nil {
		t.Fatalf("ParticipantByEmail returned error: %v", err)
	}

	if got.Email != email || got.Name != "Test Participant" || got.Slot != slot {
		t.Errorf("got %+v, want email=%q name=%q slot=%d", got, email, "Test Participant", slot)
	}
}

func TestParticipantByEmailShouldReturnErrParticipantNotFoundForAnUnknownEmail(t *testing.T) {
	st := newTestStore(t)

	_, err := st.ParticipantByEmail(context.Background(), uniqueEmail(t))

	if !errors.Is(err, store.ErrParticipantNotFound) {
		t.Errorf("expected ErrParticipantNotFound, got %v", err)
	}
}

func TestUpsertParticipantShouldUpdateNameAndEmailOnConflictingSlot(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	slot := nextSlot()
	firstEmail := uniqueEmail(t)
	secondEmail := "updated-" + uniqueEmail(t)

	if err := st.UpsertParticipants(ctx, []store.ParticipantSeed{{Slot: slot, Name: "Original Name", Email: firstEmail}}); err != nil {
		t.Fatalf("first UpsertParticipant returned error: %v", err)
	}
	if err := st.UpsertParticipants(ctx, []store.ParticipantSeed{{Slot: slot, Name: "Updated Name", Email: secondEmail}}); err != nil {
		t.Fatalf("second UpsertParticipant returned error: %v", err)
	}

	got, err := st.ParticipantByEmail(ctx, secondEmail)
	if err != nil {
		t.Fatalf("ParticipantByEmail returned error: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("expected name to be updated to %q, got %q", "Updated Name", got.Name)
	}

	if _, err := st.ParticipantByEmail(ctx, firstEmail); !errors.Is(err, store.ErrParticipantNotFound) {
		t.Errorf("expected the superseded email to no longer resolve, got err=%v", err)
	}
}

func TestUpsertParticipantsShouldPersistEverySeedInOneCall(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	slotA, slotB := nextSlot(), nextSlot()
	emailA, emailB := uniqueEmail(t), uniqueEmail(t)

	err := st.UpsertParticipants(ctx, []store.ParticipantSeed{
		{Slot: slotA, Name: "Participant A", Email: emailA},
		{Slot: slotB, Name: "Participant B", Email: emailB},
	})
	if err != nil {
		t.Fatalf("UpsertParticipants returned error: %v", err)
	}

	gotA, err := st.ParticipantByEmail(ctx, emailA)
	if err != nil {
		t.Fatalf("ParticipantByEmail(%q) returned error: %v", emailA, err)
	}
	if gotA.Name != "Participant A" {
		t.Errorf("got name %q, want %q", gotA.Name, "Participant A")
	}

	gotB, err := st.ParticipantByEmail(ctx, emailB)
	if err != nil {
		t.Fatalf("ParticipantByEmail(%q) returned error: %v", emailB, err)
	}
	if gotB.Name != "Participant B" {
		t.Errorf("got name %q, want %q", gotB.Name, "Participant B")
	}
}

func TestParticipantByEmailShouldMatchRegardlessOfCase(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	slot := nextSlot()
	email := uniqueEmail(t)

	if err := st.UpsertParticipants(ctx, []store.ParticipantSeed{{Slot: slot, Name: "Case Participant", Email: email}}); err != nil {
		t.Fatalf("UpsertParticipants returned error: %v", err)
	}

	got, err := st.ParticipantByEmail(ctx, strings.ToUpper(email))
	if err != nil {
		t.Fatalf("ParticipantByEmail with differently-cased email returned error: %v", err)
	}
	if got.Email != email {
		t.Errorf("got email %q, want %q", got.Email, email)
	}
}

func TestInsertLoginCodeShouldPersistARowForTheParticipant(t *testing.T) {
	dsn := requireTestDSN(t)
	st := newTestStore(t)
	ctx := context.Background()
	slot := nextSlot()
	email := uniqueEmail(t)

	if err := st.UpsertParticipants(ctx, []store.ParticipantSeed{{Slot: slot, Name: "Login Code Participant", Email: email}}); err != nil {
		t.Fatalf("UpsertParticipant returned error: %v", err)
	}
	participant, err := st.ParticipantByEmail(ctx, email)
	if err != nil {
		t.Fatalf("ParticipantByEmail returned error: %v", err)
	}

	code := store.LoginCode{
		ID:            uuid.NewString(),
		ParticipantID: participant.ID,
		CodeHash:      "deadbeef",
		IssuedAt:      time.Now().UTC(),
	}

	if err := st.InsertLoginCode(ctx, code); err != nil {
		t.Fatalf("InsertLoginCode returned error: %v", err)
	}

	if got := countLoginCodes(t, dsn, participant.ID); got != 1 {
		t.Errorf("expected 1 login code for participant, got %d", got)
	}
}

func TestInsertLoginCodeShouldAllowMultipleValidCodesForTheSameParticipant(t *testing.T) {
	dsn := requireTestDSN(t)
	st := newTestStore(t)
	ctx := context.Background()
	slot := nextSlot()
	email := uniqueEmail(t)

	if err := st.UpsertParticipants(ctx, []store.ParticipantSeed{{Slot: slot, Name: "Repeat Request Participant", Email: email}}); err != nil {
		t.Fatalf("UpsertParticipant returned error: %v", err)
	}
	participant, err := st.ParticipantByEmail(ctx, email)
	if err != nil {
		t.Fatalf("ParticipantByEmail returned error: %v", err)
	}

	for i := 0; i < 2; i++ {
		code := store.LoginCode{
			ID:            uuid.NewString(),
			ParticipantID: participant.ID,
			CodeHash:      fmt.Sprintf("hash-%d", i),
			IssuedAt:      time.Now().UTC(),
		}
		if err := st.InsertLoginCode(ctx, code); err != nil {
			t.Fatalf("InsertLoginCode #%d returned error: %v", i, err)
		}
	}

	if got := countLoginCodes(t, dsn, participant.ID); got != 2 {
		t.Errorf("expected 2 login codes for participant after a repeat request, got %d", got)
	}
}
