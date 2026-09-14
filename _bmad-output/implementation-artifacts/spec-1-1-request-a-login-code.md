---
title: 'Request a Login Code'
type: 'feature'
created: '2026-09-14'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: ['{project-root}/src/CLAUDE.md']
baseline_commit: 'c5eacd0c218c33485c903f399e65e3605d6b869f'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The app has no authentication at all yet. Requesting a login code (PRD FR-1) is the first step of the login flow, and none of `internal/auth`, `internal/mailer`, or `internal/store` exist in the current brownfield skeleton.

**Approach:** Add `internal/store` (loads/bootstraps `fantasy-hockey.yml`, persists `LoginCode` rows), `internal/mailer` (SMTP send behind a `Sender` func type), and `internal/auth` (matches the submitted email, hashes+persists+emails a code) — wired into two new `internal/web` routes that render the email-entry and code-entry screens.

## Boundaries & Constraints

**Always:**
- `POST /login`'s HTTP response body is identical whether or not the email matches a Player — render the code-entry screen with the same neutral copy either way.
- A `LoginCode`'s stored `code_hash` is `sha256` of the code; the plaintext code is never persisted or logged.
- A new request always appends a new `LoginCode` row; an existing valid row is never mutated or removed.
- `fantasy-hockey.yml`'s path resolves via `DATA_FILE` env var or `--data-file` flag (flag wins); if missing, `internal/store` creates it with an empty `players` list and `season: "2026-27"` — no invented player data in code.
- `SMTP_HOST`/`PORT`/`USERNAME`/`APP_PASSWORD` stay optional; the app must start with all four unset. A send attempted with `SMTP_HOST` unset returns an error that `internal/auth` logs via `slog.Error` — never surfaced in the response.
- Persisted timestamps use `internal/clock.NowTime().UTC().Format(time.RFC3339)`.
- `internal/store`'s mutex and in-memory struct stay unexported; every access goes through exported methods.

**Never:**
- No session cookie, `SESSION_SECRET`, or code-validation logic — that's Story 1.2.
- No admin/management UI for players — the list is hand-edited directly in `fantasy-hockey.yml` (AD-23).
- No docker-compose/mailpit wiring in this story — tests use a fake `mailer.Sender`.
- Don't touch the existing `GET /{$}` placeholder route — root-route auth redirect is a later story's job.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Matching email | `POST /login` with a known Player's email | 200, code-entry screen rendered; new `LoginCode` row persisted (`code_hash`, `issued_at`, `used_at=null`); `Sender` invoked with the plaintext code | N/A |
| Non-matching email | `POST /login` with an unknown address | 200, byte-identical code-entry body to the matching case; no row written; `Sender` never invoked | N/A |
| Repeat request | Player already has an unexpired, unused `LoginCode`; requests again | A second row is appended; the first row's fields are untouched | N/A |
| Mailer send fails | Matching email, `Sender` errors (e.g. `SMTP_HOST` unset) | Same 200 response as the happy path | Error logged via `slog.Error`, not surfaced |
| Store write fails | Matching email, disk write fails | 500, generic error page | Wrapped with `fmt.Errorf`, logged at the web boundary |

</frozen-after-approval>

## Code Map

- `src/main.go` — wiring only; resolve `DATA_FILE`/`--data-file` + `SMTP_*`, construct `store.Store` + `mailer.Sender`, pass into `web.NewServer(...)`
- `src/internal/web/web.go` — existing placeholder home route stays untouched; add `GET`/`POST /login`
- `src/internal/clock/clock.go` — reuse `NowTime()` as-is for `issued_at`
- `src/acceptance-tests/suite_test.go` — register the new scenario initializer
- new `src/internal/store/` — `Player`, `LoginCode` structs; `Store` with `New(path)` (bootstrap-create-if-missing), `FindPlayerByEmail`, `CreateLoginCode`
- new `src/internal/mailer/` — `Sender` func type; `NewSMTPSender(host, port, username, password string) Sender` wrapping `net/smtp`
- new `src/internal/auth/` — `RequestLoginCode(st *store.Store, send mailer.Sender, email string) error`
- new `src/internal/web/templates/login_email.html`, `login_code.html` — per `mockups/login-email.html`/`login-code.html`
- new `src/internal/web/static/styles.css` — shared dark-theme tokens, `go:embed`
- new `src/acceptance-tests/features/request-login-code.feature`, `src/acceptance-tests/login_steps_test.go`

## Tasks & Acceptance

**Execution:**
- [x] `src/internal/store/store.go` — `Player`/`LoginCode` structs + `New`/`FindPlayerByEmail`/`CreateLoginCode`, mutex-guarded atomic write-and-rename — AD-9/17/20/25/26/27/29
- [x] `src/internal/store/store_test.go` — bootstrap-creates missing file; find hit/miss; append doesn't touch other rows; concurrent-safe
- [x] `src/internal/mailer/mailer.go` — `Sender` func type + `NewSMTPSender`, empty username skips AUTH, empty host errors — AD-12/13
- [x] `src/internal/mailer/mailer_test.go` — host-unset error path; AUTH-skip branch
- [x] `src/internal/auth/auth.go` — match+hash+persist+send, identical outcome on match/no-match, logs send failures — FR-1, AD-1/2/3/9/17/20
- [x] `src/internal/auth/auth_test.go` — match path; no-match no-op; repeat request leaves prior row untouched; send failure still returns nil
- [x] `src/internal/web/templates/login-email.html`, `login-code.html` — server-rendered views — UX-DR9, UX-DR12
- [x] `src/internal/web/static/styles.css` — tokens shared by both templates — UX-DR1, UX-DR2
- [x] `src/internal/web/web.go` — `GET`/`POST /login` wired to `internal/auth` — FR-1
- [x] `src/internal/web/web_test.go` — GET renders email screen; POST renders identical body for match/no-match
- [x] `src/main.go` — resolve env/flags, wire `store`+`mailer` into `web.NewServer` — AD-25
- [x] `src/acceptance-tests/features/request-login-code.feature` — Gherkin mirroring the AC below — AD-4
- [x] `src/acceptance-tests/login_steps_test.go` + `suite_test.go` registration — fake `mailer.Sender` captures the emailed code and asserts it hashes to the stored `code_hash`

**Acceptance Criteria:**
- Given a submitted email matches a Player, when `POST /login` is handled, then a new `LoginCode` row is persisted with a sha256 `code_hash` and `issued_at`, and `Sender` is invoked with the plaintext code and the Player's email
- Given a submitted email matches no Player, when `POST /login` is handled, then no row is written, `Sender` is never invoked, and the response body is identical to the matching case
- Given a Player already has an earlier valid `LoginCode`, when they request a new code, then both rows exist afterward and the earlier row is unchanged
- Given `SMTP_HOST` is unset, when a matching request triggers a send, then the response is unaffected and an error is logged

## Implementation Notes

- Template filenames deviate from the Code Map's literal `login_email.html`/`login_code.html` spelling: they're named `login-email.html`/`login-code.html` (kebab-case) instead. The repo's `.ls-lint.yml` enforces kebab-case for `.html` files project-wide, and `task go:lint`/`docker:build` fail on a snake_case `.html` filename; renaming the templates was the fix (per the project rule to fix code, not the pipeline, on a lint failure). All internal references (`templates.ExecuteTemplate` calls in `internal/web/web.go`) use the same kebab-case names.
- `main.go` no longer calls `internal/server.ResolvePort` directly. `--port`/`-p` and the new `--data-file` flag must be declared on one `flag.FlagSet` (Go's `flag` package errors on any flag it doesn't recognize), so `main.go` now has its own `resolveConfig` that parses all three together, reusing `server.DefaultPort` as the default. `server.ResolvePort` itself is untouched and still fully covered by its own and the acceptance tests.
- `LoginCode` and `Player` both carry an `id` field (`LoginCode.ID` is an application-generated UUID via `github.com/google/uuid`, per AD-17); the Structural Seed's illustrative `login_codes` YAML omits an `id`, but AD-17's rule ("Prediction, LoginCode get an application-generated UUID string") is explicit, so the struct includes it.
- Two new direct dependencies were added: `go.yaml.in/yaml/v3` (per the Architecture Spine's Stack table) and `github.com/google/uuid` (already an indirect dependency via godog, promoted to direct).
- Unit tests that simulate a store write failure remove the store's backing directory (`os.RemoveAll`) rather than `chmod`-ing it read-only — a read-only bit is silently bypassed when the test process runs as root (as it does inside the `docker:build` container stage), which made the test pass locally but fail in the container build; removing the directory fails the write regardless of UID.

## Spec Change Log

## Review Triage Log

- **medium** — `auth.RequestLoginCode` calls `send(...)` synchronously on the match path only; the SMTP round-trip makes a match measurably slower than a no-match, undermining the frozen "identical response" invariant. Verified: `send` is awaited inline in `auth.go`. Route: patch (run it in a goroutine).
- **medium** — `store.CreateLoginCode` appends to `s.doc.LoginCodes` before calling `writeLocked()` and never rolls back the append on a write failure; a later unrelated successful write would silently persist the row a caller was told failed. Verified against `store.go:99-114`. Route: patch (rollback the append on error).
- **medium** — `store.FindPlayerByEmail` compares emails with `==`, no trim/case-fold; a hand-maintained YAML entry or a player's typed email that differs only in case/whitespace is silently treated as "no match" with no error signal. Verified against `store.go`. Route: patch (trim + case-fold both sides).
- **medium** — `internal/web/README.md` still documents a zero-arg `NewServer()` serving only the home page; it omits `store`/`mailer` params, `/login`, and `/static/`. Verified by reading the file. Route: patch (rewrite it).
- **medium** — `resolveConfig` in `main.go` (port/data-file precedence) has zero test coverage, and the existing `configure-port.feature` acceptance test calls `server.ResolvePort` directly, never `main.run`/`resolveConfig` — a regression in the real precedence logic ships silently. Verified: no `main_test.go` exists; `resolveConfig` has 0% coverage per `task go:test` output. Route: patch (add `main_test.go`).
- **medium** — the default data file `fantasy-hockey.yml`, created by `store.New` when `DATA_FILE`/`--data-file` is unset, has no `.gitignore` entry; a local `task go:run` creates it where a careless `git add` could commit real player emails/login-code hashes. Verified: `git check-ignore -v fantasy-hockey.yml` exits 1 (not ignored). Route: patch (gitignore it).
- **low** — `mailer.newSMTPSender` guards an empty `host` but not an empty `port`, giving an opaque dial error instead of a clear one. Verified against `mailer.go`. Fix is a one-line direct addition, so kept despite low likelihood. Route: patch.
- **low** — `generateCode`/`hashCode` errors in `auth.RequestLoginCode` can only occur on the match branch, so a (vanishingly rare) `crypto/rand` failure would 500 a match while a no-match always returns 200, contradicting the identical-response invariant. Fix is trivial (swallow and log, like an existing send failure) so kept despite near-zero real-world likelihood. Route: patch.
- **low** — `handleLoginSubmit`'s `r.ParseForm()` error branch (500) is untested. Fix is a trivial added test case. Route: patch.
- **low** — no test covers a malformed/empty submitted email; current behavior already falls through safely to the no-match path, so this is a coverage gap, not a code change. Route: patch (add a test case).
- **low** — `TestNewSMTPSenderShouldSkipAuthWhenUsernameIsEmpty` never asserts `from == defaultFrom`; deleting that line would still pass every test. Filed as a pre-verified gap finding. Route: patch (add the assertion).
- **low, rejected** — two players sharing one email in the hand-maintained YAML would make `FindPlayerByEmail` return only the first; requires an operator data-entry error to trigger, and a good fix (validation, or defined multi-match semantics) is more than a direct correction.
- **low, rejected** — no rate limiting on `POST /login`. Real in principle, but AD-14's own Deferred section confirms the app isn't reachable from the public internet yet (reverse proxy not set up); a proper fix (per-IP/email limits, storage, expiry policy) is well beyond a direct correction.
- **low, rejected** — `ExecuteTemplate` failing after the response has implicitly started would leave a truncated 200 instead of a 500. Today's two templates render with `nil` and no dynamic fields, so this can't actually trigger outside a client-disconnect; a proper fix (buffer-then-write) is more than a direct correction.
- **low, rejected** — `net/smtp.SendMail` has no dial timeout and could hang a handler goroutine against an unreachable SMTP host. OS-level TCP timeouts already bound the realistic worst case to roughly a minute against Gmail/mailpit; a proper fix needs a custom low-level SMTP client, well beyond a direct correction.
- **low, rejected** — `GET /static/` falls back to `http.FileServerFS`'s default directory listing, untested. `static/` only ever holds public CSS, so there's nothing sensitive to expose; not worth the churn.
- **false** — raw SMTP header construction in `mailer.go` has no CRLF-injection guard. Refuted: today's only caller (`auth.go`) passes a constant subject and a body built only from a machine-generated 6-digit code — no attacker-controlled input reaches `Sender`'s arguments in this diff.
- **false** — `LoginCode` rows accumulate with no pruning mechanism. Refuted: AD-30 rotates to a fresh data file every season, and at 3-player hobby scale a single season's row count is negligible — no architecture decision or story anywhere calls for pruning.
- **false** — an explicit `--data-file=""` is indistinguishable from an omitted flag and falls back to the default. Refuted: falling back on an empty value is the sensible, defensible behavior; nothing in the spec or architecture requires erroring on it instead.
- **out of scope** — `login-code.html` has no `<form>` and its "Log in" button isn't wired to a handler. Excluded by the frozen Boundaries' "no session cookie... or code-validation logic — that's Story 1.2"; the template's own comment already documents this.

- **high** (self-caught during re-verification after the patch round) — making `send(...)` async in `auth.go` (the first patch item above) introduced a real, reproducible data race in `src/acceptance-tests/login_steps_test.go`: the fake `mailer.Sender` writes `sentTo`/`sentCodes`/`logs` from its own goroutine with no synchronization, while step assertions read them immediately with no wait, matching neither `auth_test.go`'s own `waitForSendCalls` pattern (added in the same patch round) nor any lock. Verified: `go test -race -count=5 ./acceptance-tests/...` reliably reported `WARNING: DATA RACE` before the fix. Fixed directly (mirroring `auth_test.go`'s mutex + poll pattern): added a `sync.Mutex` to `loginScenarioState`, a mutex-guarded `syncWriter` around the log buffer, and a `waitUntil` poll used by every assertion that depends on the async `send` having landed. Re-verified: `go test -race -count=20 ./acceptance-tests/...` is clean.

## Design Notes

- Email copy: subject "Your Face-Off Pool login code", body "Your login code is {code}. It expires in 10 minutes." — plain, sentence case, no exclamation marks, matching `EXPERIENCE.md` Voice and Tone; trivial to edit later if the human wants different wording.
- First-run bootstrap seeds an empty `players` list, not invented names/emails — a human hand-edits `fantasy-hockey.yml` afterward to add real players, per AD-23's hand-maintained convention, and so no real person's email is ever committed to source.
- `GET /{$}` is left untouched; redirecting root to `/login` is deferred to whichever story adds session-checking (1.2/1.3) or the app shell (1.5) — this story's AC doesn't specify that behavior.
- `mailer.Sender` is a func type, not an interface — one method, DI'd per AD-3, and a one-line closure fake in tests.

## Verification

**Commands:**
- `task go:test` — expected: all unit tests pass, including new `store`/`mailer`/`auth`/`web` tests
- `task go:test:acceptance` — expected: the new Gherkin scenario passes end-to-end
- `task go:run` — expected: binary builds and starts with `SMTP_*` unset (`SESSION_SECRET` isn't required until Story 1.2)
- `task docker:build` — expected: full pipeline (lint, test, build image) passes
