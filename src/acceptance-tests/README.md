# Acceptance Tests

GoDog (Cucumber/Gherkin) acceptance tests for the fantasy-hockey application.

## What is tested

The Gherkin scenarios in `features/` describe the observable behaviour of the application as defined in the `*.feature` files:

- `view-home-page.feature` exercises `internal/web`'s HTTP handler end to end: a visitor opens the home page and sees the application name and the current date and time.

## How the tests work

These files are part of the main Go module (`src/`) and import production packages directly - no separate module, no `replace` directive.

```plain
TestAcceptanceSuite (suite_test.go)
  │
  └─ godog.TestSuite{...}.Run()
        Executes every Gherkin scenario in features/.
        Each step calls the production package directly (e.g. internal/web).
```

## Running the tests

```bash
# Acceptance tests only (from repo root)
task go:test:acceptance

# Directly from src/
cd src && go test -v ./acceptance-tests/...

# Full build pipeline - acceptance tests run as a gate before go build
task go:build
```

## Relation to unit tests

Unit test coverage is measured separately with `go test ./internal/...` and written to `src/coverage.out`. Acceptance tests are excluded from that scope deliberately - they exercise observable behaviour end-to-end, not individual package internals. Running the acceptance tests with `-coverpkg=./internal/...` produces a second coverage report, `src/acceptance-coverage.out`, which is a diagnostic for reachability rather than a gate metric.
