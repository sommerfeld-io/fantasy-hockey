---
title: 'Validate Login Code and Establish Session'
type: 'feature'
created: '2026-09-07'
status: 'done'
review_loop_iteration: 0
baseline_commit: '3f830ab30b55aae70dc665cab80d3d1e3e04909c'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/src/CLAUDE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 1.1 can request and email a login code but nothing checks a submitted code, authenticates the Participant, or keeps them signed in — the Login page's code step and the session mechanism (AD-12) don't exist yet.

**Approach:** Add code validation to `internal/auth`, a stateless HMAC-signed session cookie (stdlib only, keyed by the already-required `SESSION_SECRET`), a `POST /login/code` handler that authenticates and sets the cookie, and a minimal session-gated placeholder route that re-issues the cookie on every request to implement the 30-minute sliding timeout.

## Boundaries & Constraints

**Always:**
- A wrong, expired (>10 min), or already-used code all show the identical generic "Invalid code." message with the code field cleared — never differentiate the reason (mirrors FR-1's no-enumeration discipline).
- Session cookie is an HMAC-SHA256-signed token (stdlib `crypto/hmac`/`crypto/sha256` only, no third-party session library), carrying Participant ID + issued-at, signed with `SESSION_SECRET`. No server-side session table (AD-12).
- Every request to a session-gated route re-issues the cookie with a fresh issued-at when the existing session is still valid (sliding timeout), and treats a session as ended once 30 minutes have elapsed since its issued-at.
- A `LoginCode` is marked used the moment it successfully authenticates a session; a used code can never authenticate again, even before its 10-minute window elapses.
- `SESSION_SECRET` (already required at startup since Story 1.1's `loadConfig`, but currently discarded) must actually be threaded into `auth.NewService`.

**Ask First:** If sliding-timeout or code-validation logic seems to need a new package beyond `internal/auth`/`internal/web`, HALT and ask before adding one.

**Never:** Build the real Predictions/home page content — the session-gated route this story adds is a minimal placeholder only, replaced by a future epic. Never add a fake clock or inject a `Clock` interface — time-based scenarios are tested by constructing fixtures with explicit past timestamps, not by faking `clock.NowTime()`. Never introduce a server-side session store.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Valid code | Unused `LoginCode` issued <10 min ago, matching code submitted | Session cookie set (HMAC-signed, Participant + issued-at); code marked used | N/A |
| Wrong code | Code doesn't match any stored hash for the Participant | Generic "Invalid code.", field cleared, no cookie | N/A |
| Expired code | Matching hash but `issued_at` >10 min ago | Same generic "Invalid code." | N/A |
| Already-used code | Matching hash but `used_at` already set | Same generic "Invalid code." | N/A |
| Sliding re-issue | Valid session cookie, issued-at <30 min ago, request to session-gated route | Cookie re-issued with new issued-at; request proceeds | N/A |
| Session timed out | Session cookie present, issued-at >30 min ago | Login page rendered (email step) with "Session expired."; no cookie re-issued | N/A |

</frozen-after-approval>

## Code Map

- `src/internal/store/store.go:149-162,~203` -- add `UnusedLoginCodesForParticipant(ctx, participantID string, issuedAfter time.Time) ([]LoginCode, error)` (`WHERE participant_id=$1 AND used_at IS NULL AND issued_at > $2`) and `MarkLoginCodeUsed(ctx, id string) error` (`UPDATE ... SET used_at=$2 WHERE id=$1 AND used_at IS NULL`; 0 rows affected is not an error here, caller already has the code in hand).
- `src/internal/store/store_test.go` -- Postgres-gated tests for the two new methods, same skip-if-`POSTGRES_TEST_DSN`-unset pattern as existing tests.
- `src/internal/auth/auth.go:24-39,42-50` -- extend `Store` interface with the two new methods; change `NewService(s Store, m Mailer)` to `NewService(s Store, m Mailer, sessionSecret string)`; add `Session{ParticipantID string; IssuedAt time.Time}`, `ErrInvalidCode`, `ErrSessionExpired` sentinels, `ValidateLoginCode(ctx, email, code string) (Session, error)`, `EncodeSession(s Session) (string, error)`, `DecodeSession(token string) (Session, error)`. Reuse existing `hashCode` for comparison. `clock.NowTime()` (already imported) computes the 10-min cutoff and the 30-min expiry check — do not add a clock abstraction.
- `src/internal/auth/auth_test.go` -- unit tests for `ValidateLoginCode` (valid/wrong/expired/used, via a local fake `Store`) and `EncodeSession`/`DecodeSession` round-trip + tamper + expiry (construct `Session{IssuedAt: ...}` with explicit past times, no real waiting).
- `src/internal/web/web.go:42-65,82-95` -- `loginPageData` gains `Error string`; `NewServer` registers `POST /login/code` and a session-gated `GET /` via a `requireSession` wrapper; `handleLoginSubmit` now renders `Step: "code"` (was `"confirmation"`) with the existing `confirmationMessage`; new `handleCodeSubmit` calls `auth.Service.ValidateLoginCode`, sets the `session` cookie via `EncodeSession` and redirects to `/` on success, or re-renders `Step: "code", Error: "Invalid code."` on failure; new `handleHome` (placeholder) and `requireSession` middleware read the `session` cookie, call `DecodeSession`, re-issue the cookie on success or render the login page (`Step: "email"`, `Message: "Session expired."` only when a cookie was present and `DecodeSession` returned `ErrSessionExpired`) otherwise.
- `src/internal/web/templates/login.html:12-24` -- rename the `"confirmation"` step to `"code"`, add the code `<input>` (reuse existing `.invalid`/`.field-error` CSS classes already in `style.css`, no CSS changes needed) shown when `.Error` is set; hoist `{{.Message}}` above the step conditional so both the code step (send confirmation) and the email step (session-expired notice) can show it.
- `src/main.go:122-159,206` -- `config` gains `sessionSecret string`; `loadConfig` captures the already-read `SESSION_SECRET` value (currently discarded at line ~137) into it; `run()` passes `cfg.sessionSecret` as `auth.NewService`'s third argument.
- `src/acceptance-tests/features/validate-login-code-and-establish-session.feature` -- new Gherkin file covering the six matrix rows above.
- `src/acceptance-tests/login_steps_test.go` -- `fakeLoginMailer` must capture the message body (not just recipient) so scenarios can extract the emailed code; `fakeLoginStore` implements the two new `auth.Store` methods plus a test-only helper to backdate a stored code's `issued_at` (simulates the 10-min window without waiting); `newLoginScenarioState`'s `httptest.Client` needs a cookie jar so the session cookie persists across requests within a scenario; new step functions for submitting a code, asserting authenticated access to `/`, and backdating a session (construct via `auth.Service.EncodeSession(auth.Session{IssuedAt: past})` directly rather than waiting 30 real minutes).

## Tasks & Acceptance

**Execution:**
- [x] `src/internal/store/store.go` -- add `UnusedLoginCodesForParticipant` and `MarkLoginCodeUsed` -- code validation needs to check against all still-valid codes and consume the matching one
- [x] `src/internal/store/store_test.go` -- cover both new methods -- keep store's Postgres-gated test coverage complete
- [x] `src/internal/auth/auth.go` -- add `Session`, sentinels, `ValidateLoginCode`, `EncodeSession`, `DecodeSession`; thread `sessionSecret` through `NewService` -- implements AD-12/AD-31
- [x] `src/internal/auth/auth_test.go` -- unit-test the new behavior including the I/O matrix's edge cases -- TDD mandate
- [x] `src/internal/web/web.go` -- add `POST /login/code`, `GET /`, `requireSession` -- wires validation and session mechanism into the HTTP layer
- [x] `src/internal/web/templates/login.html` -- add the code step -- UX-DR10 (one field at a time, same slot)
- [x] `src/main.go` -- thread `SESSION_SECRET` into `auth.NewService` -- currently read then discarded
- [x] `src/acceptance-tests/features/validate-login-code-and-establish-session.feature` + step defs -- express every I/O matrix row as an executable scenario -- BDD mandate

**Acceptance Criteria:**
- Given a valid unused code within its 10-minute window, when submitted, then the Participant is authenticated and issued a signed session cookie (epics.md Story 1.2 AC1)
- Given an expired or already-used code, when submitted, then "Invalid code." shows inline and the field is cleared (FR-2, UX-DR13)
- Given an authenticated session, when 30 minutes pass with no request, then the next request needs a fresh code (FR-3)
- Given an authenticated session within 30 minutes, when any request is made, then the sliding countdown resets via cookie re-issuance
- Given the Login page, when an email is submitted, then that field is replaced by the code field in the same slot (UX-DR10)

## Design Notes

**Session token format:** `base64url(participantID|issuedAtUnix) + "." + base64url(HMAC-SHA256(payload, SESSION_SECRET))`. `DecodeSession` recomputes and compares with `hmac.Equal`, then checks `clock.NowTime().Sub(issuedAt) <= 30*time.Minute`, returning `ErrSessionExpired` if not — this is the only place the 30-minute constant lives.

**Why `EncodeSession` takes `IssuedAt` explicitly** rather than stamping `clock.NowTime()` internally: callers (the real login-success path, the sliding-timeout re-issue, and tests) all need control over the stamped time — tests exercise expiry/sliding-timeout by passing an explicit past `time.Time`, avoiding any clock abstraction in production code.

**Session-expired vs. no-session:** both render the plain Login page (email step, HTTP 200) rather than a redirect — a missing cookie shows no message, a present-but-expired cookie shows "Session expired.". A malformed/tampered cookie is treated the same as missing (no message) rather than surfaced as an error, since it isn't a real prior session.

**Home placeholder:** `GET /` is a bare authenticated stand-in (e.g. plain text confirming login) purely to prove and test the session/sliding-timeout mechanism — the real Predictions home lands in a later epic.

## Verification

**Commands:**
- `task go:run` -- expected: builds and starts cleanly; a non-zero exit solely from `govulncheck` is acceptable per project policy
- `task go:test` -- expected: all unit tests pass, including new `auth`/`store` cases
- `task go:test:acceptance` -- expected: the new feature file's scenarios pass
- `task docker:build` -- expected: full authoritative pipeline (lint, test, acceptance, vulncheck, image build) passes, run once the feature is complete

**Verified during review (2026-09-07):** `task go:test` (all packages pass, `auth` 91.0% coverage), `task go:test:acceptance` (13 scenarios / 60 steps pass, 50.7% end-to-end coverage), and `task go:lint` all green after the review's patch fixes. `task go:run` fails solely on the pre-existing, project-accepted `govulncheck` stdlib gap (local sandbox Go 1.26.4 vs. the Dockerfile's pinned 1.26.6) -- confirmed identical on the pre-change baseline commit, not caused by this story.

## Suggested Review Order

**Code validation and session mechanism (`internal/auth`)**

- Entry point: validates a submitted code against all still-unused, unexpired codes and issues a session on match.
  [`auth.go:139`](../../src/internal/auth/auth.go#L139)

- Closes the concurrent-redemption race: a code claimed by another request in flight now fails instead of silently succeeding twice.
  [`auth.go:166`](../../src/internal/auth/auth.go#L166)

- Constant-time comparison of the stored code hash, consistent with the session signature check below.
  [`auth.go:146`](../../src/internal/auth/auth.go#L146)

- Stateless HMAC-signed session encode/decode -- the whole session mechanism (AD-12), no server-side session table.
  [`auth.go:193`](../../src/internal/auth/auth.go#L193)

- Sentinel errors driving the generic "Invalid code."/"Session expired." responses without differentiating the reason.
  [`auth.go:43`](../../src/internal/auth/auth.go#L43)

**Persistence (`internal/store`)**

- `MarkLoginCodeUsed` now reports whether it actually won the update race via `ErrLoginCodeAlreadyUsed`, the fix `ValidateLoginCode` above relies on.
  [`store.go:206`](../../src/internal/store/store.go#L206)

- Query backing code validation: every still-unused, unexpired code for the participant, not just the most recent.
  [`store.go:171`](../../src/internal/store/store.go#L171)

**HTTP layer (`internal/web`)**

- `requireSession` middleware: the sliding-timeout re-issuance and the expired-vs-no-session message distinction live here.
  [`web.go:164`](../../src/internal/web/web.go#L164)

- New code-submission handler wiring `ValidateLoginCode` into the HTTP layer and setting the session cookie on success.
  [`web.go:123`](../../src/internal/web/web.go#L123)

- Clears a dead session cookie client-side whenever decoding fails, so the browser stops resending it.
  [`web.go:205`](../../src/internal/web/web.go#L205)

- Minimal session-gated placeholder route proving the mechanism -- not the real Predictions home.
  [`web.go:148`](../../src/internal/web/web.go#L148)

**Startup wiring (`main.go`)**

- `SESSION_SECRET` is now actually threaded into `auth.NewService` instead of being read and discarded.
  [`main.go:209`](../../src/main.go#L209)

**UI (login form)**

- Code-entry step replaces the email field in the same slot (UX-DR10), with OTP-autofill-friendly input attributes.
  [`login.html:12`](../../src/internal/web/templates/login.html#L12)

**Tests and supporting**

- I/O matrix scenarios plus the sliding-timeout, boundary, and no-cookie-message edge cases.
  [`validate-login-code-and-establish-session.feature:1`](../../src/acceptance-tests/features/validate-login-code-and-establish-session.feature#L1)

- Acceptance step definitions: cookie-jar client, code-extraction from captured mail bodies, session backdating.
  [`login_steps_test.go:1`](../../src/acceptance-tests/login_steps_test.go#L1)

- Unit tests for the concurrent-redemption fix and the exact validity-window/session-timeout boundaries.
  [`auth_test.go:1`](../../src/internal/auth/auth_test.go#L1)
