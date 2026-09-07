---
title: 'Request Login Code'
type: 'feature'
created: '2026-09-07'
status: 'done'
review_loop_iteration: 0
baseline_commit: 'bf2ade00b874ae6889e53c9b83862396eaf0780a'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/src/CLAUDE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** There is no way for a Participant to log in yet, and none of the app's foundational infrastructure (DB connection, migrations, the `store`/`web`/`auth`/`mailer` packages) exists.

**Approach:** A visitor submits an email on a Login page; if it matches one of the 3 hardcoded Participants, a 6-digit code is generated, persisted, and emailed — with an identical response either way, so the flow can never reveal which emails are registered. This story also stands up the DB connection, migrations, and the four foundational packages every later story builds on.

## Boundaries & Constraints

**Always:**
- Response to a login-code request is identical (shape, and as far as practical timing) whether or not the email matches a Participant — no enumeration leak.
- The code is never stored in plaintext — only `sha256(code)`.
- `DATABASE_URL` env var or `--database-url` CLI flag resolves the connection string; flag wins if both set; process fails fast at startup if neither is set. Same resolution path for the raw binary and the container.
- Migrations run automatically on startup, before serving anything, via `golang-migrate`.
- The 3 Participants' real name/email pairs are supplied only via env vars (`PARTICIPANT_1_NAME`/`PARTICIPANT_1_EMAIL`, `_2_`, `_3_`) and upserted into the DB on startup — never committed to git in any form (matches `SESSION_SECRET`'s fail-fast-if-unset posture). All 6 vars required; missing any fails startup.
- Real secrets (`DATABASE_URL`, `SESSION_SECRET`, `SMTP_USERNAME`, `SMTP_APP_PASSWORD`, the 6 participant vars, Postgres creds) live only in a new git-ignored `.env.secrets`, loaded by docker-compose via `env_file:` — never in the existing tracked `.env` (Taskfile dotenv + compose variable substitution only).
- `internal/auth` depends on small `Store`/`Mailer` interfaces it defines and consumes; `internal/store`/`internal/mailer` provide the real Postgres/SMTP implementations. GoDog acceptance tests exercise `internal/auth`+`internal/web`+`internal/mailer` against in-memory fakes of those interfaces — no live Postgres/SMTP needed inside `task go:build`'s Docker-stage tests.
- Dependency direction and package boundaries per existing convention: presentation → feature packages → store; store depends on nothing above it.

**Ask First:**
- Any change to `Dockerfile` or `.github/workflows/**` (protected; not touched by this story).
- Any migration or schema shape not already implied by this spec's Code Map.

**Never:**
- Never commit `.env.secrets` or any real participant email/secret value.
- Never let a login-code request's response or timing differ based on whether the email matched.
- Never introduce a third-party web framework, ORM, ID/router, or test-container library — stdlib `net/http`/`html/template`/`net/smtp` and the already-approved `pgx`/`golang-migrate` only.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Matching email | POST /login, email = a seeded Participant's | `LoginCode` row inserted (participant, `code_hash`, issued-at, used-at=null); email sent via mailer; generic confirmation shown | N/A |
| Non-matching email | POST /login, email not seeded | No `LoginCode` row; no email sent; identical generic confirmation shown | N/A |
| Repeat request | POST /login twice for the same matching email before the first code expires | A second `LoginCode` row is inserted; the first row is untouched and still valid | N/A |
| Required env var missing at startup | One of `DATABASE_URL`/flag, `SESSION_SECRET`, or a `PARTICIPANT_*` var unset | Process exits immediately with a clear error, before serving any request | Fail fast, no partial startup |
| DB unreachable at startup | Migrations cannot connect | Process exits with the migration error, before serving | Fail fast |

</frozen-after-approval>

## Code Map

- `src/go.mod` -- add `jackc/pgx/v5` v5.10.0, `golang-migrate/migrate/v4` v4.19.1 (+ pgx & iofs drivers); promote `google/uuid` from indirect to direct
- `src/internal/store/store.go` -- `Store` (wraps `*pgxpool.Pool`), `NewStore(ctx, dsn)`, `Migrate(ctx)`, `Participant`/`LoginCode` structs, `ParticipantByEmail`, `InsertLoginCode`, `UpsertParticipant`
- `src/internal/store/migrations/0001_init.up.sql` + `.down.sql` -- `participant` (uuid pk, slot smallint unique, name, email unique, created_at) and `login_code` (uuid pk, participant_id fk, code_hash, issued_at, used_at nullable) tables
- `src/internal/store/migrations_embed.go` -- `//go:embed migrations/*.sql` `embed.FS`, wired into `Migrate`
- `src/internal/auth/auth.go` -- `Store`/`Mailer` interfaces (consumer-defined), `Service`, `RequestLoginCode(ctx, email) error` (generates code via `crypto/rand`, hashes, persists, sends; returns nil on no-match)
- `src/internal/mailer/mailer.go` -- SMTP-backed `Mailer` (`smtp.gmail.com:587`, STARTTLS, `net/smtp`), config from `SMTP_USERNAME`/`SMTP_APP_PASSWORD`; `Send(ctx, to, subject, body) error`
- `src/internal/web/web.go` -- `NewServer(auth *auth.Service) http.Handler`; `GET /login` renders the form, `POST /login` calls `RequestLoginCode`, renders the generic confirmation
- `src/internal/web/templates/login.html` -- Steel Ice email-entry form (embedded via `html/template`)
- `src/internal/web/static/style.css` -- shared Steel Ice tokens (surface/border/accent/text colors, system font stack), served under `/static/`
- `src/main.go` -- resolve `DATABASE_URL`/`--database-url` (flag wins, fail-fast), open store, run migrations, read+validate the 6 `PARTICIPANT_*` vars and upsert slots 1-3, wire mailer/auth/web, serve on `HTTP_ADDR` (default `:8080`)
- `docker-compose.yml` -- add `postgres` service (`postgres:18-alpine`, `pgdata` volume); `fantasy-hockey` and `postgres` both gain `env_file: .env.secrets`
- `.env.secrets.example` -- new, committed; documents all required keys (`DATABASE_URL`, `SESSION_SECRET`, `SMTP_USERNAME`, `SMTP_APP_PASSWORD`, `PARTICIPANT_1..3_NAME/EMAIL`, `POSTGRES_DB/USER/PASSWORD`) with placeholder values
- `.gitignore` -- add `.env.secrets`
- `src/acceptance-tests/features/request-login-code.feature` -- Gherkin scenarios for the I/O matrix above
- `src/acceptance-tests/login_steps_test.go` -- step defs using in-memory fakes of `auth.Store`/`auth.Mailer`
- `src/internal/auth/auth_test.go`, `src/internal/mailer/mailer_test.go`, `src/internal/store/store_test.go` (real Postgres via `POSTGRES_TEST_DSN`, skips gracefully if unset)

## Tasks & Acceptance

**Execution:**
- [x] `src/go.mod` -- add pgx, golang-migrate, promote uuid -- new persistence deps
- [x] `src/internal/store/migrations/0001_init.{up,down}.sql` + `migrations_embed.go` -- schema -- foundation for Participant/LoginCode
- [x] `src/internal/store/store.go` (+ `store_test.go`) -- Postgres-backed Store -- real persistence, TDD first
- [x] `src/internal/mailer/mailer.go` (+ `mailer_test.go`) -- SMTP send -- code delivery
- [x] `src/internal/auth/auth.go` (+ `auth_test.go`) -- `RequestLoginCode` against fakes -- enumeration-safe business logic, TDD first
- [x] `src/internal/web/web.go`, `templates/login.html`, `static/style.css` -- login form + POST handler -- UX-DR1/UX-DR10/UX-DR13
- [x] `src/main.go` -- wiring, env/flag resolution, participant bootstrap, fail-fast checks
- [x] `docker-compose.yml`, `.env.secrets.example`, `.gitignore` -- local dev DB + secrets plumbing
- [x] `src/acceptance-tests/features/request-login-code.feature` + `login_steps_test.go` -- GoDog coverage of the I/O matrix, written before the handler (red-green)

**Acceptance Criteria:**
- Given the login page is open, when a visitor submits any email, then the response is the same generic confirmation regardless of match (FR-1, NFR3)
- Given the submitted email matches a seeded Participant, when the request is processed, then a `LoginCode` row is persisted and an email is sent via `internal/mailer`
- Given the submitted email does not match, when the request is processed, then no `LoginCode` row is created and no email is sent
- Given a Participant already has an unused, unexpired code, when they request a new one, then a second row is created and the first remains valid (FR-2)
- Given the app starts with all required env vars set, when migrations run, then the `participant` and `login_code` tables exist and the 3 Participant rows are upserted from env vars

## Design Notes

Two decisions were made explicitly with the human before drafting, both binding for every later DB-touching story, not just this one:

1. **Participant emails via env vars, not a committed migration.** Unlike the Team list (seeded by migration, per the deferred-sync architecture), Participant identity is personal data and must never enter git history on a public repo. `main.go` reads and validates 6 env vars, then calls `store.UpsertParticipants` for all 3 slots in one transaction on every startup — idempotent, so a later email change just needs a redeploy.
2. **Store/Mailer interfaces + in-memory fakes for acceptance tests.** `task go:build` runs its acceptance-test stage inside the Docker build, which has no network path to a sibling Postgres container. Rather than touching the protected `Dockerfile`/workflows, `internal/auth` defines the two interfaces it needs; GoDog scenarios exercise real `auth`/`web`/`mailer` code against fakes. Only `internal/store`'s own unit tests need a real Postgres, and they skip gracefully (`POSTGRES_TEST_DSN` unset → `t.Skip`) so `task go:test`/`task go:build` never require one.

## Verification

**Commands:**
- `task go:build` -- expected: lint, vet, unit tests, acceptance tests, complexity, licenses, vulncheck, and compile all pass with no live Postgres required
- `task go:test` -- expected: unit tests pass; `internal/store`'s Postgres-backed tests skip cleanly if `POSTGRES_TEST_DSN` is unset
- `task go:test:acceptance` -- expected: the new `request-login-code.feature` scenarios pass against the in-memory fakes
- `docker compose up postgres -d && POSTGRES_TEST_DSN=... task go:test` -- expected: `internal/store`'s real-DB tests run and pass

**Verified during review (2026-09-07):** `docker build --target build` with the bumped `golang:1.26.6-alpine3.24` base image ran the full `task build` chain end-to-end inside the real container — `govulncheck` reported "No vulnerabilities found" (was 6 Go-stdlib CVEs against 1.26.4), and `go-licenses` still passed cleanly against the new toolchain.

## Suggested Review Order

**Enumeration-safe login flow (core logic)**

- Entry point: identical response whether or not the email matches, the whole story's central invariant.
  [`auth.go:57`](../../src/internal/auth/auth.go#L57)
- Case-insensitive lookup so a Participant's casing can't cause a silent false "no match".
  [`store.go:28`](../../src/internal/store/store.go#L28)
- Handler always renders the same confirmation regardless of the service call's outcome.
  [`web.go:82`](../../src/internal/web/web.go#L82)

**Persistence (internal/store)**

- Migrations now run on a goroutine bounded by `ctx`, so an unresponsive database fails fast instead of hanging boot.
  [`store.go:91`](../../src/internal/store/store.go#L91)
- Participant lookup normalizes email before querying.
  [`store.go:134`](../../src/internal/store/store.go#L134)
- All 3 Participant slots upsert in one transaction, never partially applied.
  [`store.go:178`](../../src/internal/store/store.go#L178)
- Schema: `participant` and `login_code` tables, the foundation every later epic reads from.
  [`0001_init.up.sql:1`](../../src/internal/store/migrations/0001_init.up.sql#L1)

**Startup wiring and hardening (main.go)**

- Reads and validates all 6 `PARTICIPANT_*` vars up front, rejecting a duplicate email before it becomes an opaque DB error.
  [`main.go:93`](../../src/main.go#L93)
- Resolves every required secret/config value before any I/O, so startup fails fast and atomically.
  [`main.go:132`](../../src/main.go#L132)
- Connects, migrates, and upserts Participants under one bounded startup timeout.
  [`main.go:165`](../../src/main.go#L165)
- Serves with real HTTP timeouts and graceful SIGINT/SIGTERM shutdown, replacing the bare `http.ListenAndServe` call.
  [`main.go:191`](../../src/main.go#L191)

**Email delivery (internal/mailer)**

- Strips CR/LF from header values before composing the raw message — defense-in-depth against header injection.
  [`mailer.go:70`](../../src/internal/mailer/mailer.go#L70)
- Send is context-aware and wraps the underlying `smtp.SendMail` failure.
  [`mailer.go:41`](../../src/internal/mailer/mailer.go#L41)

**Local dev/deployment plumbing**

- Go base image bumped to clear 6 govulncheck findings (see the verification note above).
  [`Dockerfile:1`](../../Dockerfile#L1)
- New `postgres` service plus `restart: unless-stopped` on both services; secrets flow in via `env_file`, never the tracked `.env`.
  [`docker-compose.yml:83`](../../docker-compose.yml#L83)
- Template for the git-ignored `.env.secrets` — deliberately generic placeholder names, not the real Participants'.
  [`.env.secrets.example:21`](../../.env.secrets.example#L21)

**Tests and supporting changes**

- Acceptance scenarios cover the full I/O matrix plus the plain `GET /login` render path, against in-memory fakes.
  [`request-login-code.feature:1`](../../src/acceptance-tests/features/request-login-code.feature#L1)
- `internal/clock` gained `NowTime()` so `auth.go` no longer bypasses the project's single time source.
  [`clock.go:13`](../../src/internal/clock/clock.go#L13)
- `internal/store`'s own Postgres-backed tests (skip without `POSTGRES_TEST_DSN`).
  [`store_test.go:33`](../../src/internal/store/store_test.go#L33)
