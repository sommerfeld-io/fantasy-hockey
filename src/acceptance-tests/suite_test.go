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

	"github.com/cucumber/godog"

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
			InitializeHomePageScenario(ctx)
			InitializePortScenario(ctx)
			InitializeLoginScenario(ctx)
			InitializeEnterLoginCodeScenario(ctx)
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
