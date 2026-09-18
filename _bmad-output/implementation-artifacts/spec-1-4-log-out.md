---
title: 'Log Out'
type: 'feature'
created: '2026-09-14'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: ['{project-root}/src/CLAUDE.md']
baseline_commit: '03b103b89ae736b5b35b41ea340cfa903d618657'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A player can't end their own session — nothing clears the session cookie, so once issued it keeps sliding forward until it idles out on its own (PRD FR-4).

**Approach:** Add `auth.ClearSessionCookie` (an immediately-expiring cookie with the same name/path, per AD-11's stateless design — no server-side invalidation list) and a new `POST /logout` route that sets it and redirects to `/login`. `requireSession`'s existing behavior (Story 1.3) already treats the very next request without a valid cookie as unauthenticated, so no other code needs to change for that half of the AC.

## Boundaries & Constraints

**Always:**
- `POST /logout` clears the session cookie (empty value, `MaxAge<0`, same `Name`/`Path` as `IssueSessionCookie`) and redirects (302) to `/login`, regardless of whether the existing cookie was valid, expired, tampered, or absent — logout is always safe and idempotent to call.
- `ClearSessionCookie` lives in `internal/auth`, alongside `IssueSessionCookie`/`ParseSessionCookie`/`ValidateSession` — cookie-shape knowledge stays in one place.

**Never:**
- No server-side session-invalidation list or store — clearing is purely the client-side `Set-Cookie` instruction; AD-11's stateless design is unchanged.
- No CSRF token on `POST /logout` — matches the precedent already accepted for `POST /login/code` in Story 1.2's review at this app's current scale/trust level.
- No UI logout button/link — that's Story 1.5's app-shell navigation; this story only builds the endpoint it will call.

## I/O & Edge-Case Matrix

| Scenario                     | Input / State                                  | Expected Output / Behavior                                                              | Error Handling |
|-------------------------------|--------------------------------------------------|-----------------------------------------------------------------------------------------|----------------|
| Logged in, valid session      | `POST /logout` with a valid session cookie      | Cookie cleared (`MaxAge<0`); 302 to `/login`; the next request is treated as unauthenticated | N/A            |
| Already logged out            | `POST /logout` with no session cookie           | Same clearing response and redirect (idempotent)                                        | N/A            |
| Expired/invalid cookie present | `POST /logout` with an idle-expired or tampered cookie | Same clearing response and redirect                                              | N/A            |

</frozen-after-approval>

## Code Map

- `src/internal/auth/session.go` — add `ClearSessionCookie() *http.Cookie`: same `Name`/`Path`/`HttpOnly`/`SameSite` as `IssueSessionCookie`, empty `Value`, `MaxAge: -1` (Go's `net/http` convention for "delete this cookie now")
- `src/internal/auth/session_test.go` — assert `ClearSessionCookie`'s attributes
- `src/internal/web/web.go` — add `handleLogout`; register `POST /logout` directly on the outer mux (not behind `requireSession` — logout must work even with an invalid/expired/absent cookie)
- `src/internal/web/web_test.go` — `POST /logout` clears the cookie and redirects, regardless of what cookie (if any) was presented
- new `src/acceptance-tests/features/log-out.feature`, `src/acceptance-tests/log_out_steps_test.go` — full flow using a real `net/http/cookiejar` (so the client genuinely drops the cookie on `MaxAge<0`, proving the "next request" half of the AC end-to-end)
- `src/acceptance-tests/suite_test.go` — register the new scenario initializer

## Tasks & Acceptance

**Execution:**
- [x] `src/internal/auth/session.go` + `session_test.go` — `ClearSessionCookie` — AD-11
- [x] `src/internal/web/web.go` — `handleLogout`; `POST /logout` wired outside `requireSession`
- [x] `src/internal/web/web_test.go` — clears cookie + redirects for valid/expired/absent cookie cases
- [x] `src/acceptance-tests/features/log-out.feature` + `log_out_steps_test.go` — mirror the AC below, using a real cookie jar
- [x] `src/acceptance-tests/suite_test.go` — register the new scenario initializer

**Acceptance Criteria:**
- Given a player is logged in, when they trigger logout, then their session cookie is cleared and they are redirected to `/login`
- Given a player has just logged out, when they make their very next request to a protected route, then they are treated as unauthenticated and routed to `/login`

### Review Findings

No `decision-needed` or `patch` findings for this story from the ad hoc `bmad-code-review` pass (combined with story 1.5, range `03b103b..HEAD`). Rejected appendix:

- **low, rejected** — Implementation Notes says "All five acceptance scenarios" but `log-out.feature` has six (a jar-emptiness scenario was split out after this line was written). Fix requires editing the spec under review — out of scope for code review.
- **low, rejected** — The frozen "Never" bullet's CSRF rationale ("matches the precedent... for `POST /login/code`") is now inconsistent with Design Notes' corrected, endpoint-specific rationale. Fix requires editing the spec under review (including its frozen block) — out of scope for code review.
- **low, rejected** — The I/O & Edge-Case Matrix table isn't column-padded per the repo's Markdown style rules (also flagged on spec-1-5). Fix requires editing the spec under review, out of scope for code review.

## Implementation Notes

- `ClearSessionCookie` sets `MaxAge: -1`; Go's `net/http` renders that as `Max-Age=0` on the wire (its documented convention for "delete this cookie now"), confirmed via manual `curl` verification.
- All five acceptance scenarios use a shared `*http.Client` with a real `net/http/cookiejar`, not per-request manual cookie handling — the jar both drops the session cookie once the `Max-Age<0` response is processed and lets `CheckRedirect: http.ErrUseLastResponse` still inspect each 302 directly.
- Step wording in `log-out.feature` was deliberately kept distinct from `stay-logged-in.feature`'s (e.g. "the player has an active session" vs. "a player is logged in with a session issued N minutes ago") to avoid ambiguous-step registration in GoDog, since every `Initialize*Scenario` function registers into the same shared `godog.ScenarioContext`.

## Spec Change Log

## Review Triage Log

- **medium** — the acceptance scenario "The very next request after logging out is treated as unauthenticated" is described (in its own rationale/Implementation Notes) as proving the browser genuinely drops the cookie via the real `cookiejar`, but its only assertion is the follow-up redirect — which `ClearSessionCookie`'s empty `Value` alone would already produce even if `MaxAge` were lost, so it doesn't actually distinguish real jar-level removal from a coincidentally-empty retained cookie. Verified: read `ParseSessionCookie`'s empty-value handling and the scenario's assertions. Route: patch (assert the jar itself holds zero cookies for the server's origin after logout).
- **low** — `log_out_steps_test.go` defines its own `logOutSecret` and `signedLogOutSessionCookie`, duplicating `testSessionSecret` (already shared via `suite_test.go`) and `stay_logged_in_steps_test.go`'s near-identical signing helper — both in the same `acceptance_test` package. Route: patch (reuse `testSessionSecret`; extract one shared signing helper for both files).
- **low** — `ClearSessionCookie` hand-duplicates `IssueSessionCookie`'s `Name`/`Path`/`HttpOnly`/`SameSite` literals rather than building on a shared base. A test guards against drift, but the production code still carries two copies of the same shape. Route: patch (extract a small shared base-cookie constructor).
- **low** — no test pins that `GET /logout` doesn't clear the session (Go's `ServeMux` 405s an unmatched method today, but nothing asserts it). Route: patch (add the test).
- **low** — the "no CSRF token" Design Notes bullet cites `POST /login/code`'s precedent, but that endpoint's risk (guessing a code) isn't `POST /logout`'s risk (a forced logout). Route: patch (reword to analyze this endpoint's own risk directly).
- **low** — the `Max-Age=0` wire-format claim in Implementation Notes was "confirmed via manual curl verification," a one-off, non-reproducible check; only the parsed `MaxAge` field is asserted in tests. Route: patch (add an automated assertion on the raw `Set-Cookie` header text).
- **low** — `internal/auth/README.md` and `internal/web/README.md` don't mention `ClearSessionCookie`/`POST /logout` (and were already stale from Stories 1.2/1.3's session/cookie additions before this diff). Route: patch (bring both up to date with the packages' current actual contents).
- **low** — the last-byte-flip tampering logic for building an invalid-signature cookie is duplicated between `stay_logged_in_steps_test.go` and `log_out_steps_test.go` (same `acceptance_test` package). Route: patch (extract one shared tamper-cookie helper for that package; `web_test.go`'s copy stays, per the established cross-package-duplication precedent).
- **false** — `sprint-status.yaml` tracks this story as `in-progress` while the spec's own frontmatter says `in-review`. Refuted: this is expected mid-review-cycle state — `sprint-status.yaml` syncs to `review` (then `done`) at the present step, exactly like every prior story in this epic; the two files use deliberately different vocabularies for two different tracking layers, not a contradiction.
- **low, rejected** — `handleLogout`'s redirect carries no `Cache-Control: no-store`, unlike `requireSession`'s authenticated responses. Refuted: the risk that header guards against (a shared cache leaking one player's page content to another) doesn't apply to a contentless 302 redirect to the same neutral `/login` page for everyone, and compliant caches already don't replay `Set-Cookie` across different clients (RFC 7234 §5.4) regardless.
- **medium** (self-caught while applying the above patches) — strengthening the "next request after logout" scenario by inserting a `Then` (jar-emptiness check) followed by another `When` (the next request) violated `gherkin-lint`'s `keywords-in-logical-order` rule, failing `task docker:build`'s lint gate. Fixed by splitting into two single-responsibility scenarios instead: "Logging out genuinely empties the browser's cookie jar" (Given/When/Then) and the original "next request" scenario, restored to its clean Given/When/And/Then shape. Re-verified: `docker compose run --rm lint-gherkin` exits 0, and the full `task docker:build` pipeline passes.

## Design Notes

- `POST /logout` is intentionally NOT wrapped by `requireSession`: gating logout behind a valid session would make it fail exactly when a player most wants it to work (an already-expired or tampered cookie) — clearing is unconditional and idempotent instead.
- No CSRF protection is added on `POST /logout`. Its own risk is a forced logout (a malicious page silently ending a victim's session) — annoying, but not data-exposing: the victim simply has to log back in, and nothing about their account or picks is read or changed. Accepted at this app's current scale (a 3-person, low-trust hobby pool not yet reachable from outside the host network, per AD-14 Deferred); revisit if that changes.

## Verification

**Commands:**
- `task go:test` — expected: all unit tests pass, including new `auth`/`web` tests
- `task go:test:acceptance` — expected: the new Gherkin scenarios pass end-to-end
- `task go:run` — expected: binary builds and starts; manually confirm `POST /logout` clears the cookie and a subsequent `/` request redirects to `/login`
- `task docker:build` — expected: full pipeline (lint, test, build image) passes
