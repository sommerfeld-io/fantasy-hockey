# Package: `store`

Owns all database access for the application: the PostgreSQL connection pool, embedded schema migrations, and the shared entity types (`Participant`, `LoginCode`). No package above it in the dependency graph (`auth`, `mailer`, `web`) redefines its own version of a store-owned entity - they consume these types through their own consumer-defined interfaces instead.

## Responsibilities

- `NewStore` opens and verifies a `pgxpool.Pool` connection for a given DSN.
- `Migrate` applies every embedded `migrations/*.sql` file via `golang-migrate`, using a short-lived `database/sql` connection as required by that library's driver contract.
- `ParticipantByEmail`, `InsertLoginCode`, and `UpsertParticipants` are the only queries the rest of the application needs for the login-code flow.

## Design notes

- `ParticipantByEmail` returns the sentinel `ErrParticipantNotFound` rather than a generic "not found" string so callers can match on it explicitly with `errors.Is`.
- `UpsertParticipants` is idempotent (`ON CONFLICT (slot) DO UPDATE`) so it can run on every application startup - changing a Participant's name or email in the environment just needs a redeploy, no manual migration. All given seeds are applied in one transaction, so a failure partway through never leaves some slots updated and others not.
- Email lookups and writes both normalize (trim + lowercase) via an internal `normalizeEmail` helper, so a Participant matches regardless of the casing they or the deploying operator used.
- Store depends on nothing above it in the architecture; it has no knowledge of HTTP, email, or the login flow itself.

## Testing

`store_test.go` runs against a real PostgreSQL instance identified by the `POSTGRES_TEST_DSN` environment variable and skips gracefully when it is unset, so `task go:test` and `task go:build` never require a live database. Run `docker compose up postgres -d` and set `POSTGRES_TEST_DSN` to exercise these tests locally.
