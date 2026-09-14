package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
)

// sessionCookieName is the cookie IssueSessionCookie sets and
// ParseSessionCookie reads back.
const sessionCookieName = "session"

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

	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    encodeSessionValue(payload, secret),
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
