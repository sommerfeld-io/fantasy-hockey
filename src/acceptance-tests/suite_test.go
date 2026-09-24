// Package acceptance_test contains GoDog acceptance tests for the fantasy-hockey application.
// It exercises the application's behaviour end-to-end by importing production packages
// directly and running all Gherkin scenarios defined in the features/ directory.
//
// These tests are part of the main module and share its go.mod. They are intentionally
// excluded from the unit-test coverage run (go test ./internal/...) and are invoked
// explicitly via "task go:test:acceptance" or as a gate inside "task go:build".
package acceptance_test

import (
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
			InitializeBrowsePredictionSetsScenario(ctx)
			InitializeLoadCanonicalTeamListScenario(ctx)
			InitializeLoadCanonicalNHLPlayerListScenario(ctx)
			InitializeCupAndPresidentsPicksScenario(ctx)
			InitializePlayoffsCupPickScenario(ctx)
			InitializeDivisionPicksScenario(ctx)
			InitializeAwardFinalistsScenario(ctx)
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
	st, err := store.New(filepath.Join(dir, store.DataFileName))
	if err != nil {
		panic(fmt.Sprintf("store.New: %v", err))
	}
	return st
}

// noopSender never sends anything and never fails.
func noopSender(_, _, _ string) error { return nil }

var _ mailer.Sender = noopSender

// signedSessionCookieForTest builds a validly-signed session cookie for
// playerID issued at issuedAt, signed with the shared testSessionSecret via
// internal/auth's own IssueSessionCookieAt - unlike auth.IssueSessionCookie,
// which always stamps the current time, this lets a scenario pin issuedAt
// precisely to exercise the idle timeout deterministically. Shared by
// scenarios across multiple files so the signing logic can't drift.
func signedSessionCookieForTest(playerID string, issuedAt time.Time) *http.Cookie {
	return auth.IssueSessionCookieAt(playerID, issuedAt, testSessionSecret)
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
