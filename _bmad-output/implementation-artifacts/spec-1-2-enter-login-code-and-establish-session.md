---
title: 'Enter Login Code and Establish Session'
type: 'feature'
created: '2026-09-14'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: ['{project-root}/src/CLAUDE.md']
baseline_commit: '8a9f497ae10d86d0b50b416ecc4cff1a92b01a7b'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A player who requested a login code has no way to submit it — nothing validates the code, marks it used, or establishes a session (PRD FR-2); Story 1.1 explicitly left the code screen's button unwired for this story to pick up.

**Approach:** Add `auth.ValidateLoginCode` (hash+match+mark-used, identical outcome for wrong/expired/used) and `auth.IssueSessionCookie`/`ParseSessionCookie` (HMAC-signed session cookie), a `store.ConsumeLoginCode` method, and a new `POST /login/code` route that sets the cookie and redirects on success or re-renders the code screen with a generic error otherwise. `main.go` now requires `SESSION_SECRET` at startup.

## Boundaries & Constraints

**Always:**
- Wrong, expired (>10 minutes old), and already-used codes all produce the identical generic error ("That code didn't work — check it and try again.") on the same code-entry screen, with the submitted value retained in the input.
- A successful match hashes the submission, marks that one `LoginCode` row's `used_at`, and never touches any other row.
- The session cookie value is `base64url(player_id + "|" + issued_at_RFC3339) + "." + hex(HMAC-SHA256(secret, payload))`; `HttpOnly`, `SameSite=Lax`, `Path=/`, no `Secure` (AD-14 — plain HTTP today), no `Max-Age`/`Expires` (the 30-minute sliding check is Story 1.3's job).
- `SESSION_SECRET` is required; `main.go`'s `resolveConfig` returns an error (failing startup) when it's unset — never generated in-process.
- `internal/store`'s mutex/document stay unexported; `ConsumeLoginCode` goes through the same locked-method pattern as `CreateLoginCode`.

**Never:**
- No sliding-timeout renewal, no logout, and nothing yet reads/enforces the session cookie on any route — that's Stories 1.3/1.4.
- No real Predict screen or app shell — a successful login redirects to the existing `GET /{$}` placeholder; Epic 2/Story 1.5 build the real destination.
- Don't add a `Max-Age`/expiry to the cookie itself; the 30-minute idle window is enforced server-side, not by the browser.

## I/O & Edge-Case Matrix

| Scenario            | Input / State                                                       | Expected Output / Behavior                                                                    | Error Handling                    |
|----------------------|-----------------------------------------------------------------------|---------------------------------------------------------------------------------------------------|--------------------------------------|
| Valid code           | `POST /login/code`, code matches an unused row issued <10 min ago  | Session cookie set (signed, `HttpOnly`, `SameSite=Lax`); that row's `used_at` marked; 302 to `/` | N/A                                |
| Wrong code           | Submitted code hashes to no `LoginCode` row                          | 200, code screen re-rendered with the generic error, submitted value retained                  | N/A                                |
| Expired code         | Code matches a row, but `issued_at` is >10 min before now            | Same generic error as wrong code                                                                | N/A                                |
| Already-used code    | Code matches a row whose `used_at` is already set                   | Same generic error as wrong code                                                                | N/A                                |
| `SESSION_SECRET` unset | App starts with `SESSION_SECRET` unset                             | Process fails to start                                                                          | `resolveConfig` returns an error   |

</frozen-after-approval>

## Code Map

- `src/internal/auth/auth.go` — unchanged (`RequestLoginCode`)
- new `src/internal/auth/validate.go` — `ValidateLoginCode(st, code) (playerID string, ok bool, err error)`: hashes `code`, calls `st.ConsumeLoginCode`
- new `src/internal/auth/session.go` — `IssueSessionCookie(playerID, secret string) *http.Cookie`; `ParseSessionCookie(c *http.Cookie, secret string) (playerID string, issuedAt time.Time, ok bool)` — built and unit-tested now; no route guard calls it yet (Story 1.3)
- `src/internal/store/store.go` — add `ConsumeLoginCode(codeHash string, now time.Time) (playerID string, ok bool, err error)`: locked scan for hash+unused+within-10-min, marks `used_at`, persists
- `src/internal/web/web.go` — add `POST /login/code`; `NewServer` gains a `secret string` parameter; `renderTemplate` gains a `data any` parameter
- `src/internal/web/templates/login-code.html` — add `<form method="post" action="/login/code">`, error-state conditional (`class="error"` + retained value + `.error-text`), button becomes `type="submit"`
- `src/internal/web/static/styles.css` — add a `.code-input` rule (monospace, tabular-nums, letter-spacing) per `mockups/login-code.html`, applied only to the code field
- `src/main.go` — `resolveConfig` reads `SESSION_SECRET`, errors if unset; wires `secret` into `web.NewServer`
- new `src/acceptance-tests/features/enter-login-code.feature`, `src/acceptance-tests/enter_login_code_steps_test.go`
- `src/acceptance-tests/suite_test.go`, `home_steps_test.go`, `port_steps_test.go`, `src/internal/web/web_test.go` — update existing `NewServer`/`newTestServer` call sites for the new `secret` parameter

## Tasks & Acceptance

**Execution:**
- [x] `src/internal/store/store.go` — `ConsumeLoginCode` — AD-9/20/27/29
- [x] `src/internal/store/store_test.go` — valid/wrong/expired/used/write-failure cases; doesn't touch other rows
- [x] `src/internal/auth/validate.go` + `validate_test.go` — hash+match delegates to store; identical `ok=false` for wrong/expired/used
- [x] `src/internal/auth/session.go` + `session_test.go` — issue+parse round-trip; tampered signature rejected; wrong secret rejected; wrong-shape cookie rejected
- [x] `src/internal/web/web.go` — `POST /login/code` wired to `auth`+session; `NewServer(st, send, secret)`
- [x] `src/internal/web/web_test.go` — valid code sets cookie + redirects; wrong/expired/used render an identical error body
- [x] `src/internal/web/templates/login-code.html`, `static/styles.css` — form, error state, code-input styling
- [x] `src/main.go` + `main_test.go` — `SESSION_SECRET` required; `resolveConfig` errors when unset
- [x] `src/acceptance-tests/features/enter-login-code.feature` + `enter_login_code_steps_test.go` — mirror the AC below
- [x] `src/acceptance-tests/suite_test.go` — register the new scenario initializer; update shared `NewServer` call sites for the new `secret` param

**Acceptance Criteria:**
- Given a valid, unused, unexpired code, when `POST /login/code` is handled, then the session cookie is set, that `LoginCode`'s `used_at` is marked, and the response redirects toward the app
- Given a wrong, expired, or already-used code, when `POST /login/code` is handled, then the identical generic error is shown on the same screen in all three cases
- Given `SESSION_SECRET` is unset, when the app starts, then it fails to start

## Implementation Notes

- An early version of `TestValidateLoginCodeShouldReturnIdenticalOutcomesForWrongExpiredAndUsedCodes` exceeded `gocyclo`'s complexity limit (12 > 10); refactored to a table-driven subtest loop, which resolved it (per the project rule to fix the code, not relax the gate, on a lint failure).

## Spec Change Log

## Review Triage Log

- **low** — the code-input's error-state CSS class (`class="code-input error"`) is asserted nowhere; only the error text and retained value are tested. Verified: `grep` for `code-input\|" error"` across the test files returns no matches. Route: patch (assert the class on wrong/expired/used, and its absence on success).
- **medium** — the submitted code isn't trimmed before hashing (`r.FormValue("code")` used as-is in `handleLoginCodeSubmit`); a code pasted from an email client with a trailing space/newline would hash differently and show the generic "wrong code" error even though the visible digits are correct. Verified against `web.go`. Route: patch (`strings.TrimSpace` before validating).
- **low** — no test asserts the valid-code response is free of the generic error text/class ("should not" counterpart). Verified: `TestPostLoginCodeShouldRedirectAndSetASessionCookieOnAValidCode` only checks the redirect and cookie. Route: patch.
- **low** — no test covers a missing/empty `code` form field on `POST /login/code`; current behavior already falls through safely to the generic-error path, so this is a coverage gap, not a code change. Route: patch.
- **low** — `handleLoginCodeSubmit` issues a session cookie for whatever `playerID` `ValidateLoginCode` returns without checking it's non-empty; a hand-edited `login_codes` row with a blank `player_id` (self-inflicted, since the app itself never writes one) would silently produce a cookie for an empty identity. Fix is a trivial guard, so kept despite requiring an operator data-entry error to trigger. Route: patch.
- **low** — `SESSION_SECRET` is only checked via `== ""`; a whitespace-only value passes and becomes a blank-equivalent HMAC key. Verified against `resolveConfig`. Fix is a trivial `strings.TrimSpace` addition. Route: patch.
- **low** — the session payload joins `playerID + "|" + issuedAt` with no escaping; a hand-typed `player_id` (per AD-17, a human-chosen slug with no format constraint) containing a literal `"|"` would break `ParseSessionCookie`'s split. Verified against `session.go`. Fix stays contained to the two private helpers (base64url-encode each segment independently before joining, so `"|"` can never appear inside either encoded half) — no public API change. Route: patch.
- **low, rejected** — no rate limiting/lockout on `POST /login/code` against the 1,000,000-value code space. Same disposition as Story 1.1's identical finding on `POST /login`: AD-14's own Deferred section confirms the app isn't reachable from the public internet yet; a proper fix (per-IP/account limits, storage, expiry policy) is well beyond a direct correction.
- **low, rejected** — login CSRF is possible (no CSRF token/Origin check on `POST /login/code`). Real in principle, but this is a 3-person, low-trust hobby deployment not yet reachable from the public internet (AD-14 Deferred); a proper fix (CSRF tokens or a double-submit-cookie pattern) is well beyond a direct correction.
- **low, rejected** — `SESSION_SECRET` has no minimum-length/strength check, and only `main.go`'s `resolveConfig` validates it at all. `main.go` is the only real entry point in the running app; enforcing an arbitrary strength policy at every layer that merely accepts an already-validated secret is disproportionate gold-plating for this app's scale.
- **low, rejected** — `POST /login/code` responses carry no `Cache-Control: no-store`. Refuted in effect: POST responses aren't cached by HTTP-compliant caches or browsers regardless of headers, so the practical exposure is negligible.
- **false** — `ConsumeLoginCode`'s plain `!=` comparison on `CodeHash` is a timing side-channel. Refuted: this compares a hash *output*, not a secret directly; recovering the underlying 6-digit code from comparison timing would still require breaking SHA-256 preimage resistance, unlike the HMAC signature check (correctly done with `hmac.Equal`) which guards the secret itself.
- **false** — `LoginCode` rows accumulate with no pruning mechanism. Refuted, same as Story 1.1: AD-30 rotates to a fresh data file every season, and at 3-player hobby scale a season's row count is negligible.
- **low, rejected** — the `SESSION_SECRET`-unset acceptance criterion has no Gherkin scenario, only a `main_test.go` unit test. Verified: `package main` cannot be imported by `src/acceptance-tests` (a Go executable package isn't importable), so true acceptance-level coverage would need a subprocess-spawning test harness — the same constraint already surfaced and accepted for `main.go`'s `run()` wiring in Story 1.1's review, where the project's mandatory manual `task go:run` check (re-verified for this story: it exits 1 with the exact expected error) was accepted as the compensating control.
- **low, rejected** — two different players' concurrently-valid codes could rarely share the same 6-digit value, and `ConsumeLoginCode` would log the submitter in as the wrong (earlier-matched) player. Astronomically unlikely at 3-player hobby scale within any real 10-minute overlap window; a proper fix would mean redesigning the login flow to carry the submitter's own identity (e.g. re-adding an email field to the code screen), a product-level decision beyond a direct correction.

## Design Notes

- The cookie carries no expiry attribute; validity is entirely server-side (issued-at comparison), matching AD-11's stateless design — Story 1.3 adds the actual 30-minute sliding check.
- `ParseSessionCookie` is built and unit-tested now but has no caller yet outside its own tests; wiring it into route-guarding middleware is Story 1.3's explicit job, mirroring how Story 1.1 built `RequestLoginCode` before this story consumed it.
- The redirect target on success is the existing `GET /{$}` placeholder, unchanged; the real Predict screen/app shell is Epic 2/Story 1.5.
- Local `task go:run` (and any future `docker-compose up`) now needs `SESSION_SECRET` set in the environment; not wired into `docker-compose.yml` in this story, matching how Story 1.1 left mailpit wiring out of scope.

## Verification

**Commands:**
- `task go:test` — expected: all unit tests pass, including new `store`/`auth`/`web`/`main` tests
- `task go:test:acceptance` — expected: the new Gherkin scenarios pass end-to-end
- `task go:run` (with `SESSION_SECRET` exported) — expected: binary builds and starts; without it, fails to start
- `task docker:build` — expected: full pipeline (lint, test, build image) passes
