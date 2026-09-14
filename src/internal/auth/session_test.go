package auth_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
)

// signPayload signs payload the same way internal/auth's unexported
// expectedSignature does, so a test can build a validly-signed cookie value
// around a deliberately malformed payload - without a matching signature,
// ParseSessionCookie would reject on the HMAC check alone, before ever
// reaching the shape check under test.
func signPayload(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestIssueSessionCookieShouldSetTheExpectedAttributes(t *testing.T) {
	c := auth.IssueSessionCookie("basti", "test-secret")

	if !c.HttpOnly {
		t.Error("expected HttpOnly to be true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("expected Path=/, got %q", c.Path)
	}
	if c.Secure {
		t.Error("expected Secure to be false (AD-14 - plain HTTP today)")
	}
	if !c.Expires.IsZero() {
		t.Errorf("expected no Expires attribute, got %v", c.Expires)
	}
	if c.MaxAge != 0 {
		t.Errorf("expected no Max-Age attribute, got %d", c.MaxAge)
	}
}

func TestParseSessionCookieShouldRoundTripAPlayerIDContainingAPipeCharacter(t *testing.T) {
	c := auth.IssueSessionCookie("bas|ti", "test-secret")

	playerID, _, ok := auth.ParseSessionCookie(c, "test-secret")
	if !ok {
		t.Fatal("expected a player id containing '|' to round-trip successfully")
	}
	if playerID != "bas|ti" {
		t.Errorf("expected player id %q, got %q", "bas|ti", playerID)
	}
}

func TestParseSessionCookieShouldRoundTripAnIssuedCookie(t *testing.T) {
	before := time.Now().UTC()
	c := auth.IssueSessionCookie("basti", "test-secret")
	after := time.Now().UTC()

	playerID, issuedAt, ok := auth.ParseSessionCookie(c, "test-secret")
	if !ok {
		t.Fatal("expected the freshly issued cookie to parse successfully")
	}
	if playerID != "basti" {
		t.Errorf("expected player id %q, got %q", "basti", playerID)
	}
	if issuedAt.Before(before.Add(-time.Second)) || issuedAt.After(after.Add(time.Second)) {
		t.Errorf("expected issuedAt close to the current time, got %v", issuedAt)
	}
}

func TestParseSessionCookieShouldRejectATamperedSignature(t *testing.T) {
	c := auth.IssueSessionCookie("basti", "test-secret")

	// Flip the last hex digit to a value guaranteed different from the
	// original, so this always tampers the signature - swapping in a fixed
	// digit (e.g. always "0") would be a no-op on the ~1-in-16 runs where
	// that digit was already there, making the test flaky.
	last := c.Value[len(c.Value)-1]
	replacement := byte('0')
	if last == replacement {
		replacement = '1'
	}
	c.Value = c.Value[:len(c.Value)-1] + string(replacement)

	_, _, ok := auth.ParseSessionCookie(c, "test-secret")
	if ok {
		t.Fatal("expected a tampered signature to be rejected")
	}
}

func TestParseSessionCookieShouldRejectAWrongSecret(t *testing.T) {
	c := auth.IssueSessionCookie("basti", "test-secret")

	_, _, ok := auth.ParseSessionCookie(c, "a-different-secret")
	if ok {
		t.Fatal("expected a cookie signed with a different secret to be rejected")
	}
}

func TestParseSessionCookieShouldRejectAWrongShapeCookie(t *testing.T) {
	const secret = "test-secret"

	// noPipePayload and badBase64Payload are correctly signed (with secret)
	// so ParseSessionCookie is rejected by the shape check under test, not
	// by an HMAC mismatch it would never get past otherwise.
	noPipePayload := "no-pipe-here"
	badBase64Payload := "not-base64!!!|2026-09-14T13:00:00Z"

	tests := []struct {
		name  string
		value string
	}{
		{"empty value", ""},
		{"no separator dot", "not-a-valid-cookie"},
		{"payload missing pipe separator", noPipePayload + "." + signPayload(noPipePayload, secret)},
		{"invalid base64 in the player-id segment", badBase64Payload + "." + signPayload(badBase64Payload, secret)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &http.Cookie{Name: "session", Value: tt.value}
			if _, _, ok := auth.ParseSessionCookie(c, secret); ok {
				t.Errorf("expected value %q to be rejected", tt.value)
			}
		})
	}
}

func TestParseSessionCookieShouldRejectANilCookie(t *testing.T) {
	if _, _, ok := auth.ParseSessionCookie(nil, "test-secret"); ok {
		t.Fatal("expected a nil cookie to be rejected")
	}
}
