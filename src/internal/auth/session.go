package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
)

// SessionCookieName is the cookie IssueSessionCookie sets and
// ParseSessionCookie reads back. Exported so other packages (e.g.
// internal/web's session middleware) read the cookie by the same name it's
// signed under, instead of duplicating the literal.
const SessionCookieName = "session"

// SessionIdleTimeout is how long a session cookie remains valid without a
// fresh request. ValidateSession rejects any cookie older than this,
// sliding the timeout forward on every request a caller re-issues the
// cookie for (PRD FR-3).
const SessionIdleTimeout = 30 * time.Minute

// IssueSessionCookie builds a signed session cookie for playerID. The
// cookie's value is base64url(player_id) + "|" + issued_at_RFC3339 + "." +
// hex(HMAC-SHA256(secret, payload)) - player_id is base64url-encoded on its
// own (rather than joined with issued_at before encoding) so a hand-typed
// player_id containing a literal "|" can never be misread as the segment
// separator. It carries no Max-Age/Expires (AD-11 - validity is enforced
// server-side, not by the browser) and no Secure flag (AD-14 - plain HTTP
// today). Nothing yet reads this cookie back on a route - that's Story
// 1.3's job.
func IssueSessionCookie(playerID, secret string) *http.Cookie {
	issuedAt := clock.NowTime().UTC().Format(time.RFC3339)
	payload := sessionPayload(playerID, issuedAt)

	c := baseSessionCookie()
	c.Value = encodeSessionValue(payload, secret)
	return c
}

// ClearSessionCookie builds a cookie that instructs the browser to delete
// the session cookie immediately. It carries the same Name/Path/HttpOnly/
// SameSite as IssueSessionCookie (so the browser recognizes it as the same
// cookie to overwrite) but an empty Value and a negative Max-Age - Go's
// net/http convention for "delete this cookie now". Per AD-11's stateless
// design, this is purely a client-side instruction; there's no server-side
// invalidation list to update.
func ClearSessionCookie() *http.Cookie {
	c := baseSessionCookie()
	c.MaxAge = -1
	return c
}

// baseSessionCookie returns the Name/Path/HttpOnly/SameSite shape shared by
// every session cookie this package produces, so IssueSessionCookie and
// ClearSessionCookie can't drift apart on those attributes - each caller
// only sets what actually differs (Value, MaxAge).
func baseSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// ParseSessionCookie verifies c's signature against secret and, if it
// matches, returns the player ID and issued-at time it carries. Any
// wrong-shape value, a tampered signature, or a signature made with a
// different secret all yield ok=false.
func ParseSessionCookie(c *http.Cookie, secret string) (playerID string, issuedAt time.Time, ok bool) {
	if c == nil {
		return "", time.Time{}, false
	}

	payload, sig, found := strings.Cut(c.Value, ".")
	if !found {
		return "", time.Time{}, false
	}

	if !hmac.Equal([]byte(sig), []byte(expectedSignature(payload, secret))) {
		return "", time.Time{}, false
	}

	encodedID, issuedAtStr, found := strings.Cut(payload, "|")
	if !found {
		return "", time.Time{}, false
	}

	idBytes, err := base64.RawURLEncoding.DecodeString(encodedID)
	if err != nil {
		return "", time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339, issuedAtStr)
	if err != nil {
		return "", time.Time{}, false
	}

	return string(idBytes), parsed, true
}

// ValidateSession parses and verifies c against secret via
// ParseSessionCookie, then confirms it carries a non-empty player id issued
// within SessionIdleTimeout and not in the future (guarding against clock
// skew). A missing/invalid/tampered cookie, an empty decoded player id, a
// future-dated issued_at, and an idle-expired issued_at all yield the
// identical ok=false - deliberately collapsed into one outcome so a caller
// (e.g. a redirect-to-/login middleware) can't distinguish which case
// occurred.
func ValidateSession(c *http.Cookie, secret string) (playerID string, ok bool) {
	id, issuedAt, ok := ParseSessionCookie(c, secret)
	if !ok || id == "" {
		return "", false
	}
	age := clock.NowTime().Sub(issuedAt)
	if age < 0 || age > SessionIdleTimeout {
		return "", false
	}
	return id, true
}

// playerIDContextKey is the unexported type ContextWithPlayerID and
// PlayerIDFromContext key their value under, per Go's context convention -
// an unexported type guarantees no other package can collide on the key by
// stashing its own value under a plain string.
type playerIDContextKey struct{}

// ContextWithPlayerID returns a copy of ctx carrying playerID, so a handler
// downstream of a session-validating middleware (e.g. internal/web's
// requireSession) can read the logged-in player's id straight from the
// request context instead of re-parsing and re-validating the session
// cookie itself.
func ContextWithPlayerID(ctx context.Context, playerID string) context.Context {
	return context.WithValue(ctx, playerIDContextKey{}, playerID)
}

// PlayerIDFromContext returns the player id ContextWithPlayerID stored on
// ctx, if any. ok is false when ctx carries no player id - e.g. a handler
// invoked outside requireSession's middleware.
func PlayerIDFromContext(ctx context.Context) (playerID string, ok bool) {
	playerID, ok = ctx.Value(playerIDContextKey{}).(string)
	return playerID, ok
}

// sessionPayload builds the pre-signing "base64url(player_id)|issued_at"
// payload that both IssueSessionCookie and ParseSessionCookie sign and
// verify. player_id is encoded on its own, rather than joined with
// issued_at before encoding, so a literal "|" inside a hand-typed player_id
// (AD-17 - no format constraint on the slug) can never be misread as the
// segment separator.
func sessionPayload(playerID, issuedAtRFC3339 string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(playerID)) + "|" + issuedAtRFC3339
}

// encodeSessionValue appends payload's HMAC-SHA256 signature (keyed by
// secret) as a hex-encoded suffix.
func encodeSessionValue(payload, secret string) string {
	return payload + "." + expectedSignature(payload, secret)
}

// expectedSignature returns the hex-encoded HMAC-SHA256 of payload, keyed by
// secret.
func expectedSignature(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
