// Package acceptance_test contains GoDog acceptance tests for the fantasy-hockey application.
// It exercises the application's behaviour end-to-end by importing production packages
// directly and running all Gherkin scenarios defined in the features/ directory.
//
// These tests are part of the main module and share its go.mod. They are intentionally
// excluded from the unit-test coverage run (go test ./internal/...) and are invoked
// explicitly via "task go:test:acceptance" or as a gate inside "task go:build".
package acceptance_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// TestAcceptanceSuite runs all GoDog Gherkin scenarios as a regular Go test so that coverage
// data collected by -coverpkg is flushed properly before the test binary exits.
func TestAcceptanceSuite(t *testing.T) {
	opts := godog.Options{
		Format: "pretty",
		Paths:  []string{"features"},
	}

	suite := godog.TestSuite{
		Name: "acceptance",
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			InitializePortScenario(ctx)
			InitializeLoginScenario(ctx)
			InitializeEnterLoginCodeScenario(ctx)
			InitializeStayLoggedInScenario(ctx)
			InitializeLogOutScenario(ctx)
			InitializeAppShellScenario(ctx)
		},
		Options: &opts,
	}

	if suite.Run() != 0 {
		t.Fatal("acceptance test suite returned non-zero exit code")
	}
}

// testSessionSecret signs session cookies for every test server built by
// newTestServer or a scenario's own web.NewServer call.
const testSessionSecret = "test-session-secret"

// newTestServer wires web.NewServer with a throwaway store (bootstrapped
// fresh in a temp directory) and a no-op mailer.Sender, for scenarios that
// only need *a* server and don't care about login-code delivery.
func newTestServer() http.Handler {
	return web.NewServer(newTempStore(), noopSender, testSessionSecret)
}

// newTempStore bootstraps a fresh store.Store backed by a data file in a new
// temp directory, for scenarios that don't manage their own store.
func newTempStore() *store.Store {
	dir, err := os.MkdirTemp("", "fantasy-hockey-acceptance-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	st, err := store.New(filepath.Join(dir, "fantasy-hockey.yml"))
	if err != nil {
		panic(fmt.Sprintf("store.New: %v", err))
	}
	return st
}

// noopSender never sends anything and never fails.
func noopSender(_, _, _ string) error { return nil }

var _ mailer.Sender = noopSender

// signedSessionCookieForTest builds a validly-signed session cookie value
// for playerID issued at issuedAt, mirroring internal/auth's cookie format
// (base64url(player_id) + "|" + issued_at RFC3339, HMAC-SHA256-signed with
// the shared testSessionSecret) - unlike auth.IssueSessionCookie, which
// always stamps the current time, this lets a scenario pin issuedAt
// precisely to exercise the idle timeout deterministically. Shared by
// scenarios across multiple files so the signing logic can't drift.
func signedSessionCookieForTest(playerID string, issuedAt time.Time) *http.Cookie {
	payload := base64.RawURLEncoding.EncodeToString([]byte(playerID)) + "|" + issuedAt.UTC().Format(time.RFC3339)
	mac := hmac.New(sha256.New, []byte(testSessionSecret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return &http.Cookie{Name: auth.SessionCookieName, Value: payload + "." + sig}
}

// tamperSessionCookie flips c's last signature byte to a value guaranteed
// different from the original, invalidating its HMAC - swapping in a fixed
// digit unconditionally would be a no-op on the ~1-in-16 runs where that
// digit was already there, making the caller flaky. Shared by scenarios
// across multiple files so this fiddly byte-flip logic can't drift.
func tamperSessionCookie(c *http.Cookie) *http.Cookie {
	last := c.Value[len(c.Value)-1]
	replacement := byte('0')
	if last == replacement {
		replacement = '1'
	}
	c.Value = c.Value[:len(c.Value)-1] + string(replacement)
	return c
}
