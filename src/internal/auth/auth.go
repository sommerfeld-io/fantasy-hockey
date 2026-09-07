// Package auth implements the login-code request flow: given an email, it
// decides - without ever revealing the decision to the caller - whether a
// code should be generated, persisted, and emailed.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/google/uuid"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// codeDigits is the number of digits in a generated login code.
const codeDigits = 6

// Store is the persistence dependency Service needs. It is defined here (the
// consumer), not in internal/store, so tests can supply an in-memory fake
// instead of a real database connection.
type Store interface {
	// ParticipantByEmail returns the Participant registered under email, or
	// store.ErrParticipantNotFound if none matches.
	ParticipantByEmail(ctx context.Context, email string) (store.Participant, error)
	// InsertLoginCode persists a newly issued login code.
	InsertLoginCode(ctx context.Context, code store.LoginCode) error
}

// Mailer is the email-delivery dependency Service needs, defined here for the
// same reason as Store.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Service implements the login-code request flow.
type Service struct {
	store  Store
	mailer Mailer
}

// NewService creates a Service backed by the given Store and Mailer.
func NewService(s Store, m Mailer) *Service {
	return &Service{store: s, mailer: m}
}

// RequestLoginCode generates, persists, and emails a login code for email if
// it matches a registered Participant. It returns nil whether or not email
// matched - callers must never let the response differ based on this error,
// or the login flow would leak which emails are registered. A non-nil error
// only ever indicates an unexpected infrastructure failure (DB, SMTP).
func (s *Service) RequestLoginCode(ctx context.Context, email string) error {
	participant, err := s.store.ParticipantByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrParticipantNotFound) {
			return nil
		}
		return fmt.Errorf("auth: lookup participant: %w", err)
	}

	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("auth: generate code: %w", err)
	}

	loginCode := store.LoginCode{
		ID:            uuid.NewString(),
		ParticipantID: participant.ID,
		CodeHash:      hashCode(code),
		IssuedAt:      clock.NowTime(),
	}

	if err := s.store.InsertLoginCode(ctx, loginCode); err != nil {
		return fmt.Errorf("auth: persist login code: %w", err)
	}

	subject := "Your Fantasy Hockey login code"
	body := fmt.Sprintf("Your login code is %s. It is valid for 10 minutes.", code)
	if err := s.mailer.Send(ctx, participant.Email, subject, body); err != nil {
		return fmt.Errorf("auth: send login code email: %w", err)
	}

	return nil
}

// generateCode returns a cryptographically random codeDigits-digit numeric
// code as a zero-padded string (e.g. "042913").
func generateCode() (string, error) {
	max := big.NewInt(1)
	for i := 0; i < codeDigits; i++ {
		max.Mul(max, big.NewInt(10))
	}

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("auth: generate random code: %w", err)
	}

	return fmt.Sprintf("%0*d", codeDigits, n.Int64()), nil
}

// hashCode returns the sha256 hex digest of code. The plaintext code is never
// persisted - only this hash.
func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
