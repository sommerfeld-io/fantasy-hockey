package auth_test

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// sha256Hex returns the sha256 hex digest of s, mirroring how internal/auth
// hashes a submitted code before comparing it against a persisted row.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// seedStoreWithCode bootstraps a store and persists one LoginCode row for
// code, issued at issuedAt (RFC3339), for playerID.
func seedStoreWithCode(t *testing.T, playerID, code string, issuedAt time.Time) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.New(filepath.Join(dir, "fantasy-hockey.yml"))
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	sum := sha256Hex(code)
	if err := st.CreateLoginCode(playerID, sum, issuedAt.Format(time.RFC3339)); err != nil {
		t.Fatalf("CreateLoginCode: %v", err)
	}
	return st
}

func TestValidateLoginCodeShouldMatchAnUnusedUnexpiredCode(t *testing.T) {
	st := seedStoreWithCode(t, "basti", "123456", time.Now().UTC())

	playerID, ok, err := auth.ValidateLoginCode(st, "123456")
	if err != nil {
		t.Fatalf("ValidateLoginCode returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected a match, got none")
	}
	if playerID != "basti" {
		t.Errorf("expected player id %q, got %q", "basti", playerID)
	}
}

func TestValidateLoginCodeShouldRejectAWrongCode(t *testing.T) {
	st := seedStoreWithCode(t, "basti", "123456", time.Now().UTC())

	_, ok, err := auth.ValidateLoginCode(st, "000000")
	if err != nil {
		t.Fatalf("ValidateLoginCode returned error: %v", err)
	}
	if ok {
		t.Fatal("expected a wrong code to be rejected")
	}
}

func TestValidateLoginCodeShouldLogAMatch(t *testing.T) {
	st := seedStoreWithCode(t, "basti", "123456", time.Now().UTC())
	logs := captureLogs(t)

	if _, ok, err := auth.ValidateLoginCode(st, "123456"); err != nil || !ok {
		t.Fatalf("expected a match, got ok=%v, err=%v", ok, err)
	}

	if !strings.Contains(logs.String(), "validate login code") {
		t.Errorf("expected a log line for the match, got %q", logs.String())
	}
	if !strings.Contains(logs.String(), "player_id=basti") {
		t.Errorf("expected the log line to include player_id=basti, got %q", logs.String())
	}
}

func TestValidateLoginCodeShouldNotLogAWrongCode(t *testing.T) {
	st := seedStoreWithCode(t, "basti", "123456", time.Now().UTC())
	logs := captureLogs(t)

	if _, ok, err := auth.ValidateLoginCode(st, "000000"); err != nil || ok {
		t.Fatalf("expected a rejection, got ok=%v, err=%v", ok, err)
	}

	if logs.Len() != 0 {
		t.Errorf("expected no log output for a wrong code, got %q", logs.String())
	}
}

func TestValidateLoginCodeShouldRejectAnExpiredCode(t *testing.T) {
	st := seedStoreWithCode(t, "basti", "123456", time.Now().UTC().Add(-11*time.Minute))

	_, ok, err := auth.ValidateLoginCode(st, "123456")
	if err != nil {
		t.Fatalf("ValidateLoginCode returned error: %v", err)
	}
	if ok {
		t.Fatal("expected an expired code to be rejected")
	}
}

func TestValidateLoginCodeShouldRejectAnAlreadyUsedCode(t *testing.T) {
	st := seedStoreWithCode(t, "basti", "123456", time.Now().UTC())

	if _, ok, err := auth.ValidateLoginCode(st, "123456"); err != nil || !ok {
		t.Fatalf("expected the first attempt to succeed, got ok=%v, err=%v", ok, err)
	}

	_, ok, err := auth.ValidateLoginCode(st, "123456")
	if err != nil {
		t.Fatalf("ValidateLoginCode returned error: %v", err)
	}
	if ok {
		t.Fatal("expected a reused code to be rejected")
	}
}

func TestValidateLoginCodeShouldReturnIdenticalOutcomesForWrongExpiredAndUsedCodes(t *testing.T) {
	usedStore := seedStoreWithCode(t, "basti", "123456", time.Now().UTC())
	if _, ok, err := auth.ValidateLoginCode(usedStore, "123456"); err != nil || !ok {
		t.Fatalf("expected priming attempt to succeed, got ok=%v, err=%v", ok, err)
	}

	tests := []struct {
		name string
		st   *store.Store
		code string
	}{
		{"wrong", seedStoreWithCode(t, "basti", "123456", time.Now().UTC()), "000000"},
		{"expired", seedStoreWithCode(t, "basti", "123456", time.Now().UTC().Add(-11*time.Minute)), "123456"},
		{"used", usedStore, "123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			playerID, ok, err := auth.ValidateLoginCode(tt.st, tt.code)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if ok {
				t.Fatal("expected the code to be rejected")
			}
			if playerID != "" {
				t.Fatalf("expected an empty player id, got %q", playerID)
			}
		})
	}
}
