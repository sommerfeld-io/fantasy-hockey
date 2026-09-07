# Acceptance Tests

GoDog (Cucumber/Gherkin) acceptance tests for the fantasy-hockey application.

## What is tested

The Gherkin scenarios in `features/` describe the observable behaviour of the application as defined in the `*.feature` files:

- `request-login-code.feature` covers Story 1.1 (Request Login Code): it exercises `internal/auth` + `internal/web` + `internal/mailer` end-to-end through `internal/web`'s HTTP handlers, against in-memory fakes of `internal/auth`'s `Store`/`Mailer` interfaces defined in `login_steps_test.go`. No real PostgreSQL/SMTP connection is needed, since `task go:build`'s Docker-stage tests have no network path to a sibling database container.

## How the tests work

These files are part of the main Go module (`src/`) and import production packages directly - no separate module, no `replace` directive.

```plain
TestAcceptanceSuite (suite_test.go)
  │
  └─ godog.TestSuite{...}.Run()
        Executes every Gherkin scenario in features/.
        Each step calls the production package directly (e.g. internal/auth).
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
