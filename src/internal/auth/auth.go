// Package auth handles login-code issuance: matching a submitted email
// against a Player, generating and persisting a one-time code, and emailing
// it. It never reveals whether an email matched - the caller sees the same
// outcome either way.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// codeUpperBound excludes itself, giving a uniformly distributed 6-digit
// code in [000000, 999999].
const codeUpperBound = 1_000_000

const (
	loginCodeSubject      = "Your Fantasy Hockey login code"
	loginCodeBodyTemplate = "Your login code is %s. It expires in 10 minutes."
)

// RequestLoginCode looks up email among st's players. When it matches, a
// new code is generated, hashed, persisted as a LoginCode row, and emailed
// via send. When it doesn't match, RequestLoginCode is a no-op: no row is
// written and send is never called. Either way it returns nil quickly, so
// the caller's response is identical regardless of a match: send runs in
// its own goroutine so a match never waits on the SMTP round-trip, and an
// internal hiccup generating the code is logged rather than returned. Only
// a failure to persist the new row is returned as an error.
func RequestLoginCode(st *store.Store, send mailer.Sender, email string) error {
	player, ok := st.FindPlayerByEmail(email)
	if !ok {
		return nil
	}

	code, err := generateCode()
	if err != nil {
		slog.Error("generate login code", "error", err)
		return nil
	}

	issuedAt := clock.NowTime().UTC().Format(time.RFC3339)
	if err := st.CreateLoginCode(player.ID, hashCode(code), issuedAt); err != nil {
		return fmt.Errorf("auth: persist login code: %w", err)
	}

	body := fmt.Sprintf(loginCodeBodyTemplate, code)
	go func() {
		if err := send(player.Email, loginCodeSubject, body); err != nil {
			slog.Error("send login code", "error", err)
			return
		}
		slog.Info("send login code", "player_id", player.ID)
	}()

	return nil
}

// generateCode returns a cryptographically random 6-digit numeric code,
// zero-padded.
func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeUpperBound))
	if err != nil {
		return "", fmt.Errorf("read random code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashCode returns the sha256 hex digest of code. The plaintext code itself
// is never persisted or logged.
func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
