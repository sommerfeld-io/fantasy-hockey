---
name: 'Fantasy Hockey'
type: architecture-spine
purpose: build-substrate
altitude: initiative
paradigm: 'Layered Architecture'
scope: 'Fantasy Hockey web app -- the whole initiative, a greenfield feature build on the existing minimal Go skeleton'
status: final
created: '2026-09-05'
updated: '2026-09-05'
binds: []
sources:
    - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md
    - _bmad-output/planning-artifacts/briefs/brief-fantasy-hockey-2026-09-04/brief.md
    - _bmad-output/brainstorming/brainstorm-fantasy-hockey-web-app-2026-09-04/brainstorm-intent.md
    - _bmad-output/planning-artifacts/research/technical-free-nhl-data-webservices-2026-09-04/research.md
    - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/DESIGN.md
    - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/EXPERIENCE.md
companions: []
---

# Architecture Spine — Fantasy Hockey

## Design Paradigm

Layered Architecture, three layers, one direction. **v1 note:** `internal/sync` and `internal/nhlclient` are DEFERRED to v2+ (see AD-1, AD-17, Deferred) — they are listed here as the documented future shape, but no v1 code implements them; v1 is a single package/single container app plus PostgreSQL. `internal/awards` (original name) is renamed `internal/results`, broadened per PRD §4.5 to own all Management Page write-paths (award finalists/winners for all 5 Awards, team/Cup/Series Results, Deadline scheduling — FR-13, FR-23–26).

| Layer                  | Namespace                                                                                                   |
|-------------------------|---------------------------------------------------------------------------------------------------------------|
| Presentation             | `internal/web` (HTTP handlers, `html/template` views, the autocomplete widget's JS)                          |
| Feature / domain         | `internal/auth`, `internal/predictions`, `internal/scoring`, `internal/standings`, `internal/results` — v1 live. `internal/sync` — **deferred v2+** |
| Shared data access       | `internal/store` (owns all reads/writes for Participants, Seasons, Teams, Deadlines, Predictions, Results, LoginCodes, AwardFinalists; `StatLeaders` deferred v2+) |
| External gateways        | `internal/mailer` (Gmail SMTP; consumed by `internal/auth` and `internal/predictions`) — v1 live. `internal/nhlclient` (NHL API access; would be consumed only by `internal/sync`) — **deferred v2+** |
| Orchestration            | `main.go` (wiring only; no mode dispatch in v1 — see AD-1 — no business logic)                                |

Dependency direction rule: presentation depends on feature/domain packages; feature/domain packages depend on the store; the store depends on nothing above it. Feature packages must never import `internal/web`. (Deferred v2+: `internal/nhlclient` depended on only by `internal/sync`; `internal/sync` never imports `internal/predictions`, per AD-23.)

```mermaid
graph TD
    main["main.go"] --> web["internal/web"]
    web --> auth["internal/auth"]
    web --> predictions["internal/predictions"]
    web --> scoring["internal/scoring"]
    web --> standings["internal/standings"]
    web --> results["internal/results"]
    auth --> store["internal/store"]
    auth --> mailer["internal/mailer"]
    predictions --> store
    predictions --> mailer
    scoring --> store
    standings --> store
    results --> store
```

Deferred v2+ shape (not built in v1 — see AD-1, AD-17, AD-23, AD-24, Deferred):

```mermaid
graph TD
    main2["main.go (mode=sync)"] --> sync["internal/sync"]
    sync --> store2["internal/store"]
    sync --> nhlclient["internal/nhlclient"]
```

## Invariants & Rules

### AD-1 — [v1: N/A — DEFERRED v2+] Single binary, mode-selected process topology

- **Binds:** deployment, §4.6 Automated Data Sync (deferred), all
- **Prevents:** publishing and maintaining two separate Docker images for the web process and the sync process — a v2+ concern only, once §4.6's Sync is built.
- **Rule (v2+, not v1):** one Go binary with a mode argument/subcommand (`serve` | `sync`); one Docker image; docker-compose runs that same image as different services by varying only the command.
- **Rule (v1):** the binary has no mode dispatch at all — it does one thing (serve HTTP) and is deployed as one container. This AD is kept, numbered, and unchanged in substance for when v2+ revives automated Sync (PRD §4.6) — it is not deleted, per explicit decision.

### AD-2 — [ADOPTED] Go toolchain and module identity

- **Binds:** all
- **Prevents:** divergent Go versions or module paths across the codebase.
- **Rule:** Go 1.26.4; module path `github.com/sommerfeld-io/fantasy-hockey`.

### AD-3 — [ADOPTED] `main.go` is wiring-only

- **Binds:** all
- **Prevents:** business logic accumulating in `main.go`.
- **Rule:** all application logic lives in `internal/` packages; `main.go` only loads config and wires dependencies (v1: no mode to dispatch on — see AD-1).

### AD-4 — [ADOPTED] Dependency injection via function parameters

- **Binds:** all
- **Prevents:** code coupled to global state (system clock, stdout, network) that resists testing.
- **Rule:** side effects (clock, I/O, network) are injected as function parameters, following the existing `run(out io.Writer, now func() string)` pattern in `main.go`.

### AD-5 — [ADOPTED] GoDog acceptance tests

- **Binds:** all features, §4.1–4.8
- **Prevents:** feature behavior validated only at the unit level, with no end-to-end coverage.
- **Rule:** acceptance criteria are expressed as Gherkin `.feature` files under `src/acceptance-tests/features/`, executed via GoDog.

### AD-6 — [ADOPTED] Build-gate tooling

- **Binds:** all
- **Prevents:** quality, complexity, license, or vulnerability regressions merging silently.
- **Rule:** the build pipeline runs golangci-lint, gocyclo (complexity limit 10), go-licenses, and govulncheck as gates; a failing gate blocks the build.

### AD-7 — [ADOPTED] Multi-stage Alpine container image

- **Binds:** deployment, all
- **Prevents:** bloated, non-reproducible, or root-owned runtime images.
- **Rule:** the Dockerfile stays a multi-stage build: `golang:1.26.4-alpine3.24` for build, `alpine:3.24.0` for runtime, non-root user.

### AD-8 — [ADOPTED] Taskfile orchestration

- **Binds:** all
- **Prevents:** undocumented or ad-hoc build/lint/test invocations.
- **Rule:** build, lint, test, and run commands are defined as Taskfile tasks; the root `taskfile.yml` delegates to `src/taskfile.yml` via the `go:` namespace.

### AD-9 — Layered architecture with fixed dependency direction

- **Binds:** all features, §4.1–4.8
- **Prevents:** feature packages depending on `internal/web`, feature packages depending on each other in ways that bypass `internal/store`, and circular dependencies between layers.
- **Rule:** presentation (`internal/web`) depends on feature/domain packages; feature/domain packages depend on `internal/store`; `internal/store` depends on nothing above it. `internal/mailer` is depended on only by `internal/auth` and `internal/predictions`. `internal/results` (renamed from `internal/awards`, broadened — PRD §4.5) follows the same rule as any other feature package: depends only on `internal/store`, never imported by another feature package. (Deferred v2+: `internal/nhlclient` depended on only by `internal/sync`; `internal/sync` never depends on `internal/predictions` or any other feature package, AD-23 — vacuous in v1 since neither package exists yet.)

### AD-10 — PostgreSQL as sole datastore, own container

- **Binds:** §4.2, §4.5, §4.6 (deferred), §4.7, §4.8, all persisted state
- **Prevents:** SQLite, or any single-writer file-locked store, being used where multiple containers write concurrently. **v1 note:** with `internal/sync` deferred and no mode split (AD-1), v1 is a single app container — the original concurrent-writer justification is currently dormant, not false: it re-activates unchanged the moment v2+ adds the sync container back. Postgres is not reversed for v1 regardless, to avoid a database-engine swap now and back later.
- **Rule:** all persisted state lives in PostgreSQL, run as its own container; `internal/store` is the only package that opens a database connection.

### AD-11 — Server-rendered presentation, minimal-dependency stance

- **Binds:** §4.2, §4.4, §4.5, §4.8
- **Prevents:** a JS framework, a separate API+SPA split, a third-party router, or an ORM.
- **Rule:** `internal/web` renders HTML server-side with stdlib `html/template`; routing uses stdlib `net/http` `ServeMux` method+path patterns; the only client-side JS is vanilla JS driving the autocomplete widget (FR-5).

### AD-12 — Stateless signed-cookie sessions

- **Binds:** §4.1 (FR-3, FR-4)
- **Prevents:** a server-side session table/store, or any built-in revocation mechanism.
- **Rule:** a session is an HMAC-signed cookie carrying participant + issued-at; every authenticated request re-issues the cookie, implementing the 30-minute sliding idle timeout; logout means the cookie is no longer re-issued/is cleared — no session table exists in `internal/store`.

### AD-13 — Gmail SMTP for all outbound email

- **Binds:** §4.1 (FR-1), §4.3 (FR-10)
- **Prevents:** adding a transactional-email SaaS dependency or library.
- **Rule:** outbound email (login codes, deadline reminders) is sent via stdlib `net/smtp` to `smtp.gmail.com:587` with STARTTLS, wrapped in a single `internal/mailer` package that `internal/auth` and `internal/predictions` both call — no other package sends email directly. No email content beyond login codes and the FR-10 reminder is ever sent. **v1 note:** login-code sends remain request-triggered (FR-1, unchanged); deadline-reminder sends are also request-triggered in v1 — a synchronous call from the Management Page's "Send Reminder Email" HTTP handler (FR-27) into `internal/predictions`, not a background ticker (see AD-23, deferred).

### AD-14 — Gmail App Password as runtime secret

- **Binds:** §4.1, §4.3
- **Prevents:** hardcoding the sending account's credentials or assuming plain password auth.
- **Rule:** the sending Gmail account must have 2-Step Verification enabled and an App Password generated manually, outside the app; the App Password is supplied to the app only as a runtime secret/env var.

### AD-15 — Plain-HTTP app behind a reverse proxy to be added later; arm64 target

- **Binds:** deployment, all
- **Prevents:** the app terminating TLS itself, or assuming an x86 runtime.
- **Rule:** the app serves plain HTTP (no TLS); TLS/exposure is a reverse proxy's job, set up outside this architecture (see Deferred); the existing multi-arch Dockerfile already builds for arm64, the Raspberry Pi deployment target.

### AD-16 — Standings and scores computed live, never cached or persisted

- **Binds:** §4.7 (FR-17–21), §4.8 (FR-22)
- **Prevents:** a stored score column or cache that can drift from Predictions+Results — the stale/miscalculated-score failure mode this rebuild exists to eliminate.
- **Rule:** `internal/scoring` and `internal/standings` compute points on every read directly from `internal/store`'s Predictions, Results, and AwardFinalist data; no score value is ever written to storage.

### AD-17 — [DEFERRED v2+, not built in v1] Ticker-driven sync with write-nothing-on-failure

- **Binds:** §4.6 (FR-14, FR-16) — deferred
- **Prevents:** partial or corrupt writes on a failed or schema-mismatched NHL API response; redundant per-consumer fetches within one run; a lenient decode silently reproducing the 2023 API migration's silent-field-loss failure mode.
- **Rule (v2+, not v1):** `mode=sync` runs a `time.Ticker` loop that fires once on startup then on the FR-14-configured interval; `internal/nhlclient` (stdlib `net/http`) is the only package that calls the NHL API. A response is valid only if it decodes AND every field the caller depends on is explicitly checked present/non-zero after decoding — decode success alone is not sufficient, since Go's JSON unmarshaling silently zero-values missing fields rather than erroring. On any transport failure, non-2xx response, or failed field-presence validation, the run writes nothing to `internal/store`, and the next scheduled run retries independently.
- **v1:** no Sync exists; all Results are entered manually via the Management Page (`internal/results`, PRD §4.5, FR-23–25). This AD is kept, numbered, and unchanged in substance for a future version, per explicit decision — not deleted.

### AD-18 — [ADOPTED] RFC3339 timestamps everywhere

- **Binds:** all
- **Prevents:** mixed or ambiguous date-time formats across features.
- **Rule:** every date/timestamp is an RFC3339 string, produced via `internal/clock.Now()` (existing convention).

### AD-19 — ID strategy for every `internal/store`-owned entity

- **Binds:** §4.1 (FR-1, FR-2), §4.2, §4.5 (FR-13, FR-23–26), §4.6 (FR-15, FR-16 — deferred), §4.7
- **Prevents:** minting a synthetic ID for Team (which would break FR-15's rename/relocation stability); two builders picking different PK types (`uuid` vs `bigserial`) for the same entity because only some entities were covered.
- **Rule:** every entity `internal/store` owns has one fixed ID strategy, covering: Participant, Season, Prediction, LoginCode, Result, AwardFinalist, and Deadline use a Postgres-native `uuid` primary key, generated application-side (not `bigserial`/auto-increment) so no code path ever needs the database to hand back a generated key before it can proceed. Team is the sole exception: it uses the NHL API's own team identifier as its primary key/technical ID, never a locally generated UUID. `StatLeader` and `ReminderLog` are deferred v2+ entities (AD-24, AD-23) — not present in v1's schema, so not covered here until revived.

### AD-20 — [ADOPTED] Error wrapping, no custom error envelope

- **Binds:** all
- **Prevents:** a bespoke error-response/envelope type.
- **Rule:** errors are wrapped with `fmt.Errorf("context: %w", err)` per Go idiom; no custom error-envelope type exists.

### AD-21 — Autocomplete data delivered embedded, not via a JSON endpoint

- **Binds:** §4.2 (FR-5), §4.5 (FR-13, FR-23–25)
- **Prevents:** a separate JSON API surface being introduced for autocomplete, which would contradict AD-11's server-rendered, no-API-split stance.
- **Rule:** the canonical Team and Player candidate lists are embedded as JSON directly in the rendered page (server-side, at render time) for the vanilla-JS autocomplete widget to filter client-side — no dedicated `/api/...` JSON endpoint exists anywhere in the app. Every embedding site (FR-5 predictions entry; FR-13 award finalist entry; FR-23–25 Management Page team/Cup/Series Result entry) uses the identical object shape `{"id": "<uuid-or-nhl-team-id>", "label": "<display name>"}` and the same shared widget script — no feature invents its own field names.

### AD-22 — LoginCode is a persisted, independently-tracked entity

- **Binds:** §4.1 (FR-1, FR-2)
- **Prevents:** treating a login code as transient/in-memory state, which cannot satisfy FR-2's rule that multiple codes may be simultaneously valid per Participant, each independently expiring.
- **Rule:** `internal/store` persists a `LoginCode` row per issued code (participant, `code_hash`, issued-at, used-at nullable); the stored value is `sha256(code)`, never the plaintext code — validation hashes the submitted code and compares hashes. A code is valid only while unused and within the FR-2 10-minute window; requesting a new code inserts a new row rather than mutating/replacing an existing one.

### AD-23 — [DEFERRED v2+, not built in v1] Reminder-email ticker runs alongside sync, as a sibling, not inside it

- **Binds:** §4.3 (FR-10) — deferred; see AD-32 for v1's actual mechanism
- **Prevents:** the deadline-reminder check having no defined home in either mode; `internal/sync` importing `internal/predictions` and blurring its single NHL-data responsibility; a builder hardcoding the reminder interval because no env var was wired for it; duplicate reminder emails on every tick before a Participant completes their Predictions.
- **Rule (v2+, not v1 — only meaningful if a future version brings back an *automatic, scheduled* reminder cadence; FR-10 as currently written in the PRD is manually triggered, see AD-32):** in `mode=sync`, `main.go` starts two independent `time.Ticker` loops as siblings: one driving `internal/sync` (AD-17, NHL data, interval from `SYNC_INTERVAL`), one driving `internal/predictions`' reminder check (interval from its own `REMINDER_INTERVAL` env var) — `internal/sync` never imports `internal/predictions`. The reminder ticker calls `internal/predictions` (owns the "has this Participant completed this Deadline" check and email content) and a shared mailer package for delivery (AD-13). Each send is recorded in a `ReminderLog` row (`internal/store`; Participant, Deadline, sent-at) before the email is sent; a tick sends only for a (Participant, Deadline) pair with no existing `ReminderLog` row for that reminder slot — never on completion status alone.
- **Rule (process lifecycle, v2+):** both tickers in `mode=sync` derive their context from one `signal.NotifyContext(..., syscall.SIGTERM)` set up once in `main.go`; `main.go` blocks on both via one coordinated mechanism (e.g. `errgroup.Group`), so a SIGTERM lets any in-flight cycle on either ticker finish before the process exits — no ticker is fired-and-forgotten without being part of that shutdown wait.
- **v1:** no ticker, no `REMINDER_INTERVAL`, no `ReminderLog` — see AD-32. Kept, numbered, and unchanged in substance for a future automatic-cadence version, per explicit decision.

### AD-24 — [DEFERRED v2+, not built in v1] Stat-leader sync depth, discriminator column, and tie computation split

- **Binds:** §4.6 (FR-16 — deferred), §4.7 (FR-17)
- **Prevents:** `internal/nhlclient` truncating stat-leader data to exactly 3 entries, which would make tie detection past 3rd place (FR-17's tie-inclusive rule) impossible after the fact; `internal/nhlclient`/`internal/store` and `internal/scoring` independently guessing different values for "large enough" N; the two automated awards (Art Ross, Rocket Richard) sharing one `StatLeader` table with no agreed discriminator, causing one side's query to silently match zero rows.
- **Rule (v2+, not v1):** `StatLeader` carries an `award` column with exactly two values, `art_ross` and `rocket_richard` (the same vocabulary as `AwardFinalist`'s trophy names, not the NHL API's internal stat-category names) — `internal/nhlclient` translates the API's `points`/`goals` categories into this column on write. `internal/nhlclient` fetches and `internal/store` persists the top **10** ranked entries per award (one named constant, defined once in `internal/store` and imported by `internal/nhlclient` and `internal/scoring` — no second copy of the number). `internal/scoring` — not `internal/nhlclient` — computes the tie-expanded top-3 set from that stored list at read time, per AD-16.
- **v1:** Art Ross and Rocket Richard are entered manually via the broadened `AwardFinalist` (FR-13, PRD §4.5) exactly like Hart/Norris/Vezina — no `StatLeader` table exists in v1's schema. `internal/scoring` still computes the tie-expanded top-3 set (FR-17), reading from `AwardFinalist` instead of `StatLeader`. This AD is kept, numbered, and unchanged in substance for when v2+ automates Art Ross/Rocket Richard again.

### AD-25 — No-enumeration-leak response shape, and visibility filtering never touches the scoring read path

- **Binds:** §4.1 (FR-1), §4.4 (FR-12), §4.7 (FR-17–21), §4.8 (FR-22)
- **Prevents:** a timing or response-shape difference at the login or Predictions-visibility boundary revealing whether an email is registered, or whether a hidden Prediction exists, before it should; a shared store query that filters hidden Predictions by default silently starving the Scoring Engine of picks it must still count (a hidden Prediction still belongs to and scores for its own Participant — FR-12 hides it only from *other* Participants' view).
- **Rule:** `internal/auth`'s login-code request path returns an identical response (shape and, so far as practical, timing) whether or not the submitted email matches a Participant (FR-1). `internal/predictions`' visibility filtering (FR-12) omits hidden Predictions from the rendered data entirely, server-side, before any template render — never included-then-hidden via client-side CSS/JS. This filtering lives only in the presentation-facing read path in `internal/predictions`/`internal/web`; `internal/store` exposes a separate, unfiltered read (or an `includeHidden bool` parameter) that `internal/scoring` and `internal/standings` always use — no store method both feeds the Scoring Engine and applies FR-12's visibility rule.

### AD-26 — `internal/store` owns domain entity structs and shared enums

- **Binds:** all features, §4.1–4.8
- **Prevents:** two feature packages independently defining their own `Prediction` (or other shared entity) Go struct with incompatible shapes; two features encoding the same `Prediction.kind` (or other cross-cutting) value with different casing/vocabulary, since AD-9 forbids feature packages from importing each other to agree on one.
- **Rule:** `internal/store` exports the canonical Go struct for every entity it owns (`store.Prediction`, `store.Participant`, `store.AwardFinalist`, etc.) — feature packages import and use these structs as-is; no feature package redefines its own version of a store-owned entity. `internal/store` also exports typed constants for every cross-cutting enumeration, starting with `Prediction.Kind` (`store.KindAwardFinalist`, `store.KindTeamMark`, `store.KindSeries`, `store.KindEarlyPick`, `store.KindLatePick`, `store.KindPresidentsTrophy`) and `AwardFinalist.Award` (`store.AwardHart`, `store.AwardNorris`, `store.AwardVezina`, `store.AwardArtRoss`, `store.AwardRocketRichard` — all 5 in v1, per PRD FR-13's broadened scope — a Postgres `text` column storing the constant's string value, not a bare `smallint`, since the same value must render as a display string with no translation table). No feature package re-derives its own string/int literal for a value it doesn't own.

### AD-27 — `internal/store` exposes transaction composition; multi-write operations use it

- **Binds:** §4.5 (FR-23, saving a whole Division's team results in one action), §4.6 (FR-14–16, deferred), §4.1 (FR-1–2)
- **Prevents:** a multi-entity write partially committing when a later step fails — in v1, FR-23 saving multiple Team marks (playoff qualifiers + division winner) for one Division at once; in deferred v2+, a sync run's Team-bootstrap + StatLeader writes (AD-17).
- **Rule:** `internal/store` exposes one transaction-composition primitive (`store.WithTx(ctx, func(tx Queries) error) error`); any operation that writes more than one entity/row as one logical unit is wrapped in a single call to it. A single-entity write may still use a lone auto-committing method.

### AD-28 — Migrations run on startup; `golang-migrate`'s advisory lock serializes concurrent attempts

- **Binds:** deployment, §4.6 (deferred)
- **Prevents:** an undecided "who runs migrations" leaving a container to start against a not-yet-migrated schema, or (v2+) two containers racing a manual migration step.
- **Rule:** the app runs `golang-migrate` against the resolved database connection string (AD-34: `DATABASE_URL` env var or `--database-url` flag) before doing anything else at startup — no separate migration mode or manual step exists. `golang-migrate`'s built-in Postgres advisory lock makes concurrent attempts safe (one waits, then finds the schema already current and proceeds), which matters once v2+'s `serve`/`sync` split (AD-1, deferred) reintroduces two containers both starting up against the same database.

### AD-29 — Season-zero bootstrap on startup

- **Binds:** §4.2, §4.5 (FR-23–26), §4.6 (FR-15, deferred), all Season-scoped data
- **Prevents:** every feature package assuming a Season row exists (per the ERD's `SEASON ||--o{ ...` relationships) with no code path that ever creates the first one, since there is no admin UI or Season-management screen in v1.
- **Rule (v1):** on startup, the app checks whether a Season row exists for the year named by a `SEASON_YEAR` env var; if not, it creates one. The Team list itself is not created here — it comes from data already seeded by a `golang-migrate` migration (PRD §4.6: "seeded at deployment time"), present before the app's first startup, not fetched at Season-creation time.
- **Rule (v2+, deferred):** if §4.6's Sync is revived, Season-zero creation would additionally trigger the FR-15 Team-list bootstrap against a live webservice call at that moment, replacing the v1 migration-seed approach; `mode=sync` would assume a Season already exists (created by `mode=serve`) and would not create one itself.

### AD-30 — Import-boundary enforcement via golangci-lint `depguard`

- **Binds:** all, AD-9, AD-23
- **Prevents:** AD-9's and AD-23's dependency-direction rules (feature packages never import `internal/web`; `internal/sync` never imports `internal/predictions`; only `internal/sync` imports `internal/nhlclient`) existing only as prose that a build can silently violate.
- **Rule:** `.golangci.yml` enables the `depguard` linter with rules encoding AD-9's forbidden import edges (feature packages never import `internal/web`); a violation fails `task go:lint` (AD-6's existing build gate), not just code review. AD-23's `internal/sync`/`internal/nhlclient` edges are currently vacuous — those packages don't exist in v1 — and get added to `.golangci.yml` when v2+ introduces them.

### AD-31 — Session-signing secret is a required runtime env var

- **Binds:** §4.1 (FR-3)
- **Prevents:** an in-process-generated HMAC secret silently invalidating every session (logging every Participant out) on every container restart.
- **Rule:** the HMAC key used to sign session cookies (AD-12) is read from a required `SESSION_SECRET` env var at startup; the process fails to start if it is unset — it is never generated in-process.

### AD-32 — `internal/results` owns all Management Page writes; Deadline is a persisted, editable entity

- **Binds:** §4.5 (FR-13, FR-23–26)
- **Prevents:** the Management Page's five write-paths (award finalists, team results, Cup winner, series results, deadline scheduling) being scattered across multiple packages with no single owner; Deadlines staying implicit/hardcoded when FR-26 requires them to be editable data.
- **Rule:** `internal/results` (the renamed, broadened `internal/awards`) is the sole feature package that writes `AwardFinalist`, `Result` (team marks, Cup winner, Series results), and `Deadline` rows — all via `internal/store`. `Deadline` is a persisted entity (Season, `deadline_key` enum — `preseason`/`round1`/`round2`/`conference_finals`/`final` — date/time value), read by `internal/predictions` for FR-8's lock check and by `internal/results`' own reminder trigger (AD-33) — `internal/predictions` never writes a `Deadline` row, only reads it.

### AD-33 — v1 reminder send is a synchronous, request-triggered action — no ticker, no dedupe log

- **Binds:** §4.3 (FR-10), §4.5 (FR-27)
- **Prevents:** a builder assuming AD-23's deferred ticker/`ReminderLog` design is still live in v1 and wiring a background process or dedupe table that doesn't belong.
- **Rule:** clicking "Send Reminder Email" on the Management Page calls an `internal/web` handler that synchronously invokes `internal/predictions`' existing "which Participants haven't completed this Deadline" check and sends via `internal/mailer` (AD-13) — within the same HTTP request, no goroutine, no queue. No `ReminderLog` entity exists in v1: since sending is an explicit, unlimited-repeat human action (not a repeating automatic tick), there is nothing to deduplicate against — every click independently re-evaluates completion status and sends fresh. `REMINDER_INTERVAL` does not exist as an env var in v1.

### AD-34 — Database connection: env var or CLI flag, flag wins, identical for binary and container

- **Binds:** deployment, all (every feature depends on `internal/store` reaching the right database)
- **Prevents:** the raw Go binary and the Docker container resolving `DATABASE_URL` two different ways; no way to point the same built binary at different databases (dev/test/prod) without rebuilding or without editing a committed file; silent startup against an unset/wrong database.
- **Rule:** the database connection is a single connection-string value (`postgres://user:pass@host:port/db` — not separate host/user/password fields), resolvable two ways: the `DATABASE_URL` env var, or a `--database-url` CLI flag, parsed with the stdlib `flag` package (no third-party CLI/config library, consistent with AD-11's minimal-dependency stance). If both are set, **the CLI flag wins**. If neither is set, the process fails fast at startup with a clear error — it never starts against an empty or default connection string. This resolution logic lives once in `main.go` (AD-3) and behaves identically whether invoked as `./fantasy-hockey` directly or as the Docker container's entrypoint — docker-compose supplies it via `environment:` (already shown in Structural Seed below); a bare local/test invocation may use either the flag or an exported env var.

## Consistency Conventions

| Concern                                                | Convention                                                                                                                                                                                                                       |
|----------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Naming (entities, files, interfaces, events)              | Package names are short, lowercase, singular, no underscores. v1: `auth`, `predictions`, `scoring`, `standings`, `results`, `mailer`, `store`, `web`, `clock`. Deferred v2+: `sync`, `nhlclient`. No generic package names (`util`, `common`).                |
| Data & formats (ids, dates, error shapes)                 | IDs: Postgres native `uuid`, application-generated, for every `internal/store`-owned entity except Team, which uses the NHL API's own team identifier (AD-19). Shared entity structs and cross-cutting enum constants (`Prediction.Kind`, `AwardFinalist.Award`) are exported once from `internal/store`, never redefined per feature (AD-26). Dates/timestamps: RFC3339 strings via `internal/clock.Now()` (AD-18). Errors: `fmt.Errorf("context: %w", err)`, no custom error-envelope type (AD-20). |
| State & cross-cutting (session/auth, config/secrets, alerting) | Session/auth: stateless HMAC-signed cookie, sliding 30-min idle timeout, no server-side session table (AD-12), signing key from required `SESSION_SECRET` env var (AD-31). Config/secrets, v1: `DATABASE_URL` env var or `--database-url` CLI flag, flag wins (AD-34); `SMTP_USERNAME`/`SMTP_APP_PASSWORD`, `SESSION_SECRET`, `SEASON_YEAR` (AD-29) as env vars only — no secrets committed (AD-14). Deferred v2+: `SYNC_INTERVAL`, `REMINDER_INTERVAL` (AD-17, AD-23) return once Sync/the ticker are revived. No in-app alerting: Scoring Engine failures are surfaced only through externally exported metrics/logs, never through in-app UI (per PRD Cross-Cutting NFRs; no admin role exists to act on such an alert). |

## Stack

| Name                          | Version                                   |
|---------------------------------|----------------------------------------------|
| Go                                | 1.26.4                                        |
| PostgreSQL                       | 18 (`postgres:18-alpine`)                    |
| jackc/pgx                        | v5.10.0                                       |
| cucumber/godog                   | v0.16.0 (already in `go.mod`)                |
| golangci-lint                    | latest (build-gate tool)                     |
| gocyclo                          | latest (build-gate tool, complexity limit 10) |
| go-licenses                      | latest (build-gate tool)                     |
| govulncheck                      | latest (build-gate tool)                     |
| golang-migrate/migrate/v4         | v4.19.1 (schema migrations)                  |
| net/http                         | stdlib                                        |
| html/template                    | stdlib                                        |
| net/smtp                         | stdlib                                        |

## Structural Seed

### Deployment topology

Target host: Raspberry Pi (arm64), already covered by the existing multi-arch Dockerfile. Reverse proxy is out of scope here — see Deferred. **v1 is two containers** (app + postgres) — the `fantasy-hockey-sync` container is deferred v2+ (AD-1, AD-17), shown dashed below for the documented future shape.

```mermaid
graph LR
    proxy["Reverse proxy<br/>(not yet set up)"] -. "HTTP, no TLS" .-> web

    subgraph host["Raspberry Pi (arm64) host"]
        web["fantasy-hockey container<br/>(single mode, v1)"]
        sync["fantasy-hockey-sync container<br/>mode=sync — DEFERRED v2+"]
        db[("postgres container")]
        vol[("pgdata volume")]
    end

    web -->|"Postgres wire protocol"| db
    sync -.->|"Postgres wire protocol (v2+)"| db
    db --- vol

    style sync stroke-dasharray: 5 5
```

### docker-compose additions

The existing `docker-compose.yml` only has `lint-*` services plus an unparameterized `fantasy-hockey` build service today. **v1 adds env vars to that service and one new `postgres` service — no `fantasy-hockey-sync` service exists in v1** (AD-1, AD-17 deferred). The `lint-*` services are unchanged and omitted below. `DATABASE_URL` here is the env-var path of AD-34's two resolution options — a bare local/test invocation of the same binary may use `--database-url` instead, e.g. to point at a different database per dev/test/prod without touching this file.

```yaml
services:
  fantasy-hockey:                     # existing service -- gains env (no command needed, v1 has no mode)
    environment:
      - DATABASE_URL=postgres://...
      - SMTP_USERNAME=...
      - SMTP_APP_PASSWORD=...
      - SESSION_SECRET=...
      - SEASON_YEAR=...
    depends_on:
      - postgres

  postgres:                            # new
    image: postgres:18-alpine
    environment:
      - POSTGRES_DB=...
      - POSTGRES_USER=...
      - POSTGRES_PASSWORD=...
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

**Deferred v2+ addition** (when AD-1/AD-17 are revived): a `fantasy-hockey-sync` service — same image, `command: ["./fantasy-hockey", "sync"]`, `depends_on: postgres`, with its own `SYNC_INTERVAL` and `REMINDER_INTERVAL` env vars — and `fantasy-hockey`'s command becomes `["./fantasy-hockey", "serve"]` to match the revived mode split.

### Core-entity ERD

Names and relationships only; `Prediction.kind` (award finalist pick, team mark, series pick, Early Cup pick, Late Cup pick, Presidents' Trophy pick — per the PRD Glossary, §3) is a data value, not a structural split. **v1:** `AWARDFINALIST` covers all 5 Awards, entered manually (FR-13, AD-26) — `STATLEADER` and `REMINDERLOG` are deferred v2+ entities (AD-24, AD-33) and do not exist in v1's schema; `DEADLINE` is new (AD-32, FR-26).

```mermaid
erDiagram
    PARTICIPANT ||--o{ PREDICTION : makes
    PARTICIPANT ||--o{ LOGINCODE : "issued to (FR-2)"
    SEASON ||--o{ PREDICTION : scopes
    SEASON ||--o{ TEAM : "seeded at deployment (FR-15 deferred, AD-29)"
    SEASON ||--o{ AWARDFINALIST : "has manually entered finalists/winners for all 5 Awards"
    SEASON ||--o{ DEADLINE : "has one editable date/time per (AD-32, FR-26)"
    TEAM ||--o{ PREDICTION : "is subject of"
    AWARDFINALIST ||--o{ PREDICTION : "is subject of (award finalist pick)"
    PREDICTION ||--o| RESULT : "scored against"
```

**Deferred v2+ additions to this ERD** (not in v1's schema): `STATLEADER` (SEASON ||--o{ STATLEADER, AWARDFINALIST-like relationship to PREDICTION — AD-24) and `REMINDERLOG` (PARTICIPANT ||--o{ REMINDERLOG, dedupe key (Participant, Season, `deadline_key`) — AD-23/AD-33) return unchanged in substance when v2+ revives automated Sync and the ticker-driven reminder cadence.

### Source tree

```text
src/
  main.go                # wiring only; no mode dispatch in v1 (AD-1 deferred)
  internal/
    web/                 # presentation: HTTP handlers, html/template views
      static/            # CSS (DESIGN.md tokens) + vanilla autocomplete JS, served via go:embed
    auth/                 # login-code issuance/validation, session cookie
    predictions/           # prediction entry, edit-before-deadline, deadline enforcement/visibility, reminder check (AD-33)
    scoring/                # scoring engine: awards, team marks, series, cup picks
    standings/              # live standings computation
    results/                # Management Page writes: award finalists (all 5), team/Cup/Series Results, Deadlines (AD-32; renamed+broadened from "awards")
    mailer/                 # Gmail SMTP delivery (stdlib net/smtp, AD-13); used by auth + predictions
    store/                  # Postgres data access (pgx); owns all reads/writes
    clock/                  # existing: current time, RFC3339 (adopted)
  acceptance-tests/
    features/               # Gherkin .feature files
```

**Deferred v2+ additions to this tree** (not present in v1): `internal/sync/` (NHL data sync loop body, AD-17) and `internal/nhlclient/` (NHL API gateway, used only by sync) — both return unchanged in substance when a future version revives automated Sync.

## Capability → Architecture Map

| Capability / Area                                       | Lives in                                                | Governed by                          |
|-------------------------------------------------------------|-------------------------------------------------------------|------------------------------------------|
| §4.1 Authentication (FR-1–4)                                 | `internal/auth`, `internal/web`                             | AD-9, AD-12, AD-13, AD-14, AD-19, AD-22, AD-25, AD-31 |
| §4.2 Enter Predictions (FR-5–7)                               | `internal/predictions`, `internal/web`                       | AD-9, AD-11, AD-19, AD-21, AD-26          |
| §4.3 Deadline Enforcement & Reminders (FR-8–10)               | `internal/predictions`, `internal/web`                       | AD-9, AD-13, AD-14, AD-32, AD-33 (v1); AD-23 (deferred v2+) |
| §4.4 Predictions Visibility (FR-11–12)                        | `internal/predictions`, `internal/web`                       | AD-9, AD-25                              |
| §4.5 Management Page (FR-13, FR-23–27)                        | `internal/results`, `internal/predictions` (FR-27 delegates), `internal/web` | AD-9, AD-19, AD-21, AD-26, AD-27, AD-32, AD-33 |
| §4.6 Automated Data Sync (FR-14–16) — **DEFERRED v2+**        | `internal/sync`, `internal/nhlclient`, `internal/store` (none exist in v1) | AD-1, AD-10, AD-15, AD-17, AD-19, AD-23, AD-24, AD-27, AD-28, AD-29 — all deferred v2+ per those ADs' own text |
| §4.7 Scoring Engine (FR-17–21)                                | `internal/scoring`                                           | AD-9, AD-16, AD-20, AD-24, AD-25, AD-26  |
| §4.8 Standings (FR-22)                                        | `internal/standings`                                         | AD-9, AD-16, AD-25                       |

## Deferred

- **Whole automated-Sync / multi-mode design, kept for v2+.** The mode-selected process topology (AD-1), the sync ticker with write-nothing-on-failure (AD-17), the stat-leader sync depth/discriminator design (AD-24), the ticker-driven reminder cadence with its `ReminderLog` dedupe (AD-23), and their supporting rules in AD-9/AD-10/AD-19/AD-27/AD-28/AD-29/AD-30 are all kept, numbered, and unchanged in substance — a deliberate, explicit decision, not an oversight (PRD §4.6: "Reversed out of v1 Must... a deferred enhancement, not an abandoned idea"). Revisit as one coherent unit when a future version automates data entry again; the ADs already describe exactly what to build.
- **Wikidata SPARQL lead for Hart/Norris/Vezina automation.** An unexecuted, unconfirmed lead (`wdt:P166` statements) that might make part of §4.5's manual entry automatable (PRD Open Question 1). Only relevant alongside the deferred Sync work above; FR-13's manual entry stands regardless as the v1 (and likely permanent) answer for these 3 trophies.
- **User-visible handling of a broken/changed NHL API beyond silent retry, and exact backoff/retry parameters.** Both explicitly open only if a future version revives §4.6 (PRD Open Questions 2–3) — moot for v1, which makes no NHL API calls at all.
- **Reverse proxy / public exposure setup.** TLS termination and exposing the app beyond the host network is explicitly out of this architecture's scope (memlog decision on deployment, AD-15). Revisit before the app needs to be reachable from outside the host network.
- **Go/Alpine patch versions are slightly behind current.** `go.mod` pins Go 1.26.4 and the Dockerfile pins `alpine:3.24.0`; as of this spine's authoring, Go 1.26.6 (fixes security issues govulncheck's build gate cares about) and Alpine 3.24.1 are available. Not a spine error — both were accurately ratified from existing brownfield reality — but a routine patch bump is due before/during the next build.
- **`pgdata` volume backup/restore.** No backup, restore, or disaster-recovery story exists for the Postgres data volume on the Raspberry Pi. Revisit once real season data exists that would be painful to lose.
- **NHL.com Terms of Service ambiguity.** Moot for v1 (no NHL API calls exist). Whether the ToS's anti-scraping clause extends to the specific API subdomain a revived §4.6 would depend on is a genuinely open legal question (PRD Constraints and Guardrails, "Legal"). Revisit only alongside the deferred Sync work above.
