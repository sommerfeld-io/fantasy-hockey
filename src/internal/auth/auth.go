// Package auth implements the login-code request flow: given an email, it
// decides - without ever revealing the decision to the caller - whether a
// code should be generated, persisted, and emailed.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// codeDigits is the number of digits in a generated login code.
const codeDigits = 6

// codeValidityWindow is how long a generated login code remains valid after
// issuance, unless used first (epic 1 context / FR-2).
const codeValidityWindow = 10 * time.Minute

// sessionTimeout is the sliding idle timeout: a session is considered ended
// once this long has elapsed since its issued-at, with no request in
// between. This is the only place the 30-minute constant lives.
const sessionTimeout = 30 * time.Minute

// ErrInvalidCode is returned by ValidateLoginCode when the submitted code
// doesn't authenticate - whether it's wrong, expired, or already used.
// Callers must never differentiate between these causes in what they show
// the visitor, mirroring FR-1's no-enumeration discipline.
var ErrInvalidCode = errors.New("auth: invalid code")

// ErrSessionExpired is returned by DecodeSession when the token is otherwise
// well-formed and correctly signed, but its issued-at is older than the
// 30-minute sliding timeout.
var ErrSessionExpired = errors.New("auth: session expired")

// Session is the identity carried by a signed session cookie: which
// Participant is signed in, and when the session was last (re-)issued.
type Session struct {
	ParticipantID string
	IssuedAt      time.Time
}

// Store is the persistence dependency Service needs. It is defined here (the
// consumer), not in internal/store, so tests can supply an in-memory fake
// instead of a real database connection.
type Store interface {
	// ParticipantByEmail returns the Participant registered under email, or
	// store.ErrParticipantNotFound if none matches.
	ParticipantByEmail(ctx context.Context, email string) (store.Participant, error)
	// InsertLoginCode persists a newly issued login code.
	InsertLoginCode(ctx context.Context, code store.LoginCode) error
	// UnusedLoginCodesForParticipant returns every still-unused login code
	// issued for participantID after issuedAfter.
	UnusedLoginCodesForParticipant(ctx context.Context, participantID string, issuedAfter time.Time) ([]store.LoginCode, error)
	// MarkLoginCodeUsed marks the login code identified by id as used. It
	// returns store.ErrLoginCodeAlreadyUsed if the code was already claimed
	// (by this or a concurrent request) - a caller mid-redemption must not
	// authenticate a session when it loses this race.
	MarkLoginCodeUsed(ctx context.Context, id string) error
}

// Mailer is the email-delivery dependency Service needs, defined here for the
// same reason as Store.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Service implements the login-code request/validation flow and the
// stateless session mechanism built on top of it.
type Service struct {
	store         Store
	mailer        Mailer
	sessionSecret string
}

// NewService creates a Service backed by the given Store and Mailer, signing
// session tokens with sessionSecret (AD-12/AD-31).
func NewService(s Store, m Mailer, sessionSecret string) *Service {
	return &Service{store: s, mailer: m, sessionSecret: sessionSecret}
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

// ValidateLoginCode checks code against every still-unused login code issued
// for the Participant registered under email within the last
// codeValidityWindow, and marks the matching code used the moment it
// succeeds - a used code can never authenticate again, even before its
// window elapses. It returns ErrInvalidCode for a wrong, expired, or
// already-used code without distinguishing which, mirroring FR-1's
// no-enumeration discipline.
func (s *Service) ValidateLoginCode(ctx context.Context, email, code string) (Session, error) {
	participant, err := s.store.ParticipantByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrParticipantNotFound) {
			return Session{}, ErrInvalidCode
		}
		return Session{}, fmt.Errorf("auth: lookup participant: %w", err)
	}

	cutoff := clock.NowTime().Add(-codeValidityWindow)
	candidates, err := s.store.UnusedLoginCodesForParticipant(ctx, participant.ID, cutoff)
	if err != nil {
		return Session{}, fmt.Errorf("auth: list unused login codes: %w", err)
	}

	hash := hashCode(code)
	for _, candidate := range candidates {
		// Constant-time comparison: a submitted code must never authenticate
		// faster or slower depending on how many hex characters happen to
		// match, mirroring DecodeSession's hmac.Equal use for its signature.
		if subtle.ConstantTimeCompare([]byte(candidate.CodeHash), []byte(hash)) != 1 {
			continue
		}
		if err := s.store.MarkLoginCodeUsed(ctx, candidate.ID); err != nil {
			if errors.Is(err, store.ErrLoginCodeAlreadyUsed) {
				// Lost the race to redeem this code - a concurrent request
				// already claimed it. Never issue a Session in that case.
				return Session{}, ErrInvalidCode
			}
			return Session{}, fmt.Errorf("auth: mark login code used: %w", err)
		}
		return Session{ParticipantID: participant.ID, IssuedAt: clock.NowTime()}, nil
	}

	return Session{}, ErrInvalidCode
}

// EncodeSession signs sess into a session-cookie token. Callers - the
// login-success path, the sliding-timeout re-issue, and tests - control
// IssuedAt explicitly rather than Service stamping clock.NowTime()
// internally, since each needs control over the stamped time. Encoding is
// pure string/base64 formatting keyed by an in-memory secret, so unlike
// DecodeSession it can never fail.
func (s *Service) EncodeSession(sess Session) string {
	payload := encodeSessionPayload(sess)
	sig := s.signSessionPayload(payload)
	return encodeSegment(payload) + "." + encodeSegment(sig)
}

// DecodeSession verifies token's signature and, if valid, checks it against
// the 30-minute sliding timeout - the only place that check happens,
// returning ErrSessionExpired when it fails.
func (s *Service) DecodeSession(token string) (Session, error) {
	rawPayload, rawSig, ok := strings.Cut(token, ".")
	if !ok {
		return Session{}, fmt.Errorf("auth: malformed session token")
	}

	payload, err := decodeSegment(rawPayload)
	if err != nil {
		return Session{}, fmt.Errorf("auth: decode session payload: %w", err)
	}
	sig, err := decodeSegment(rawSig)
	if err != nil {
		return Session{}, fmt.Errorf("auth: decode session signature: %w", err)
	}

	if !hmac.Equal(sig, s.signSessionPayload(payload)) {
		return Session{}, fmt.Errorf("auth: invalid session signature")
	}

	sess, err := decodeSessionPayload(payload)
	if err != nil {
		return Session{}, err
	}

	if clock.NowTime().Sub(sess.IssuedAt) > sessionTimeout {
		return Session{}, ErrSessionExpired
	}

	return sess, nil
}

// signSessionPayload returns the HMAC-SHA256 of payload keyed by the
// Service's session secret.
func (s *Service) signSessionPayload(payload []byte) []byte {
	mac := hmac.New(sha256.New, []byte(s.sessionSecret))
	mac.Write(payload)
	return mac.Sum(nil)
}

// encodeSessionPayload serializes sess as "participantID|issuedAtUnix".
func encodeSessionPayload(sess Session) []byte {
	return []byte(fmt.Sprintf("%s|%d", sess.ParticipantID, sess.IssuedAt.Unix()))
}

// decodeSessionPayload parses the "participantID|issuedAtUnix" format
// produced by encodeSessionPayload.
func decodeSessionPayload(payload []byte) (Session, error) {
	participantID, rawUnix, ok := strings.Cut(string(payload), "|")
	if !ok {
		return Session{}, fmt.Errorf("auth: malformed session payload")
	}

	unixSeconds, err := strconv.ParseInt(rawUnix, 10, 64)
	if err != nil {
		return Session{}, fmt.Errorf("auth: parse session issued-at: %w", err)
	}

	return Session{ParticipantID: participantID, IssuedAt: time.Unix(unixSeconds, 0).UTC()}, nil
}

// encodeSegment/decodeSegment encode the two dot-separated segments of a
// session token as unpadded base64url, per the token format in the design
// notes.
func encodeSegment(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeSegment(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
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
