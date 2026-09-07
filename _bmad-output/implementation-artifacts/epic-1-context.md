# Epic 1 Context: Authentication

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Let the three fixed Participants (Basti, Sadl, Tobbi) log in without a password, via an emailed one-time code, and stay signed in for a bounded idle window. This is the foundation epic: it also stands up the database connection, migrations, and the core packages (store, web, auth, mailer) every later epic builds on. There's no registration, no admin role, and no password to manage — just three known email addresses and a code.

## Stories

- Story 1.1: Request Login Code
- Story 1.2: Validate Login Code and Establish Session
- Story 1.3: Logout

## Requirements & Constraints

- A login-code request always returns the same generic response, whether or not the submitted email matches one of the three Participants — no way to probe which emails are registered.
- A generated code is a 6-digit number, valid for 10 minutes or until used once, whichever comes first. Requesting a new code does not invalidate a still-valid earlier one — more than one code can be valid for the same Participant at once.
- An authenticated session ends after 30 minutes of no requests; any request resets the countdown (sliding timeout, not a fixed expiry).
- Logout ends the session immediately; the next request needs a fresh code.
- No password ever exists anywhere in this system — the code is the only credential.
- No enumeration leak anywhere in the login flow: response shape and (as far as practical) timing must not reveal whether an email is registered.

## Technical Decisions

- **Brownfield base.** An existing minimal Go skeleton lives in `src/`: `main.go` (placeholder using `internal/clock`), Go 1.26.4, module `github.com/sommerfeld-io/fantasy-hockey`, GoDog acceptance tests, golangci-lint + gocyclo(10) + go-licenses + govulncheck build gates, a multi-stage Alpine Dockerfile, Taskfile orchestration. Build on this — extend `main.go`, keep existing task names and gates intact, don't scaffold fresh.
- **Layered architecture.** Packages this epic introduces: `internal/web` (presentation), `internal/auth`, `internal/mailer`, `internal/store` (owns all DB access, exports shared entity structs/enums — no feature package redefines its own version of a store-owned entity). Dependency direction: presentation → feature packages → store; store depends on nothing above it. Feature packages never import `internal/web` or each other directly; this is enforced by a `depguard` golangci-lint rule, not just review.
- **Database.** PostgreSQL 18, `jackc/pgx` v5.10.0 driver, `golang-migrate/migrate/v4` v4.19.1, migrations run automatically on startup before serving anything (its Postgres advisory lock makes concurrent startup attempts safe). Connection is a single connection-string value, resolved from a `DATABASE_URL` env var or a `--database-url` CLI flag (stdlib `flag`, no third-party lib) — if both are set the flag wins; if neither is set, the process fails fast at startup rather than starting against an empty/default connection. This resolution must behave identically for the raw binary and the Docker container.
- **LoginCode persistence.** A `LoginCode` row records Participant, a hash of the code (never the code itself in plaintext), issued-at, and a nullable used-at. Multiple simultaneously-valid rows per Participant are expected and fine.
- **Session mechanism.** Stateless HMAC-signed cookie carrying Participant identity + issued-at, re-issued on every authenticated request to implement the sliding 30-minute timeout — no server-side session table. The HMAC signing key comes from a required `SESSION_SECRET` env var; the process must fail to start if it's unset (never generate a key in-process, which would silently invalidate every session on restart).
- **Email.** Gmail SMTP (`smtp.gmail.com:587`, STARTTLS) via stdlib `net/smtp`, wrapped in `internal/mailer`. The sending account needs 2-Step Verification + an App Password, supplied as a runtime secret (not this epic's concern to provision, just to consume). Sending is synchronous, request-triggered — no background worker.
- **Presentation.** Server-rendered HTML via stdlib `html/template`, routing via stdlib `net/http` `ServeMux`. No JS framework, no third-party router, no separate JSON API for this epic.
- **Deployment target.** Raspberry Pi (arm64) via the existing multi-arch Dockerfile; plain HTTP, no TLS in-app (a reverse proxy, set up separately, handles exposure).
- **Testing.** Every story's acceptance criteria must be expressed as Gherkin `.feature` files under `src/acceptance-tests/features/`, executed via GoDog, per the project's TDD/BDD mandate.

## UX & Interaction Patterns

- Dark-only "Steel Ice" visual system (see DESIGN.md for exact tokens) — no light mode.
- Login form: one field visible at a time. Submitting the email replaces that field with the code field in the same slot — never both shown together, no navigation between them.
- Voice: plain and neutral, no exclamation marks or banter. Exact strings: "Check the entered email address." (after any code request, success or not), "Invalid code." (expired/wrong code, field cleared, no lockout), "Session expired." (idle-timeout redirect to Login).
- A rejected/invalid state uses a `danger`-bordered field plus an inline `meta`-sized message — no modal, no toast.
- This story also establishes cross-cutting UX baseline every later epic inherits rather than re-testing: single responsive layout (no separate mobile design), WCAG AA contrast + full keyboard operability + comfortable touch targets + reading-order focus order, and a standing anti-pattern list (no sports-broadcast visual clichés, no gamification chrome, no carousels/hero animations/notification badges/auto-refresh polling).

## Cross-Story Dependencies

Story 1.1 is foundational: it stands up the DB connection/migrations, `internal/store`, `internal/web`, `internal/auth`, and `internal/mailer` packages, plus the cross-cutting UX baseline — Stories 1.2 and 1.3 build directly on that base rather than introducing new infrastructure. Story 1.2 depends on Story 1.1's `LoginCode` persistence. Story 1.3 depends on Story 1.2's session mechanism existing.
