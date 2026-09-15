---
title: 'Persistent App Shell With Player Identity and Navigation'
type: 'feature'
created: '2026-09-15'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: ['{project-root}/src/CLAUDE.md']
baseline_commit: 'e74d6fb4adef3dbe40c322f0d02d26d622b5991d'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The only authenticated route (`GET /{$}`) renders a placeholder string, and there are no `Predict`/`Leaderboard`/`Compare` destinations or player-identity propagation for a logged-in player to reach them — nothing Epics 2–6 build has anywhere to mount.

**Approach:** Add a reusable `html/template` app shell (pinned header + pinned bottom nav around one scrolling content region) mounted on new `/predict`, `/leaderboard`, `/compare` routes (with `/` aliasing to `/predict`), threading the session's player id through request context so the header can resolve and show the logged-in player's name and the stored season.

## Boundaries & Constraints

**Always:**
- Header and bottom nav are pinned; only the content region scrolls.
- Exactly 3 bottom-nav tabs, in order: Predict, Leaderboard, Compare — active tab in `ice`, inactive in `muted`.
- Header shows only the player's name (bold, left) and season label (muted, right) — no logo, no player switcher.
- Single-column, dark-theme only; column stays centered at max phone width (430px) on wider viewports.
- The 3 new routes are registered on the existing `authMux` behind `requireSession` — same as `GET /{$}` today.
- Player identity for the header comes from the session (via context propagation added to `internal/auth`), not from re-parsing the cookie per handler.
- The header includes a small tappable logout control (e.g. near the season label) that calls the existing `POST /logout` endpoint — the header is otherwise static.
- Predict, Leaderboard, and Compare each render a short static "Coming soon" message unique to that tab, inside the real shell — not an empty content region.

**Never:**
- No JS framework, SPA routing, or client-side state — server-rendered `html/template`, full-page navigation only.
- Do not build real Predict/Leaderboard/Compare functionality (Epics 2/4/5) — shell-level placeholder content only.
- Do not change `/login`, `/login/code`, or `POST /logout`'s existing handler behavior beyond adding a UI affordance that calls the existing logout endpoint.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Logged-in player opens `/` | Valid session cookie | Serves the Predict shell page; header shows name + season; Predict tab active | N/A |
| Logged-in player opens `/leaderboard` or `/compare` | Valid session cookie | Shell renders with that tab active; header unchanged | N/A |
| Unauthenticated/expired/tampered session on any shell route | Missing/invalid/idle-expired cookie | 302 to `/login` (existing `requireSession` behavior, unchanged) | N/A |
| Valid session, but its player id has no matching player in the store | Stale/deleted player id in an otherwise-valid session | Page still renders (no 500/panic); header degrades to a neutral non-crashing state | Log the lookup miss server-side |

</frozen-after-approval>

## Code Map

- `src/internal/auth/session.go` — add context propagation: an unexported context-key type + `ContextWithPlayerID(ctx, id) context.Context` / `PlayerIDFromContext(ctx) (string, bool)`. `ValidateSession` itself is unchanged.
- `src/internal/web/web.go` — `requireSession` (currently `web.go:98-111`) calls `auth.ContextWithPlayerID` and passes `r.WithContext(ctx)` onward instead of discarding `playerID`. Add `authMux.Handle` entries for `GET /predict`, `GET /leaderboard`, `GET /compare`. Replace `handleHome`'s placeholder body (`web.go:126-133`) so `GET /{$}` serves the same content as `/predict`. Add a small handler factory that resolves player name + season and renders the shell template with the right active tab.
- `src/internal/web/templates/shell.html` (new) — composed via the existing `template.ParseFS(templatesFS, "templates/*.html")` glob (`web.go:22-30`); reuses `renderTemplate` (`web.go:215-220`). Data: `{PlayerName, Season, ActiveTab, ...per-tab content}`.
- `src/internal/web/static/styles.css` — add the missing design tokens (`--ice`, `--ice-deep`, `--gold`, `--green`, `--sel`) and the shell's fixed-header/fixed-nav/scrolling-middle layout rules, consistent with the existing `.screen` (max-width 430px, centered) convention.
- `src/internal/store/store.go` — add `func (s *Store) Season() string` and `func (s *Store) FindPlayerByID(id string) (Player, bool)`, mirroring `FindPlayerByEmail`'s existing lock discipline (`store.go:96`).
- `src/internal/web/web_test.go` — cover each new route's 200/active-tab/name/season, `/` aliasing to Predict, the unauthenticated-redirect case for all 3 new routes, and the stale-player-id degrade-without-panic case.
- `src/acceptance-tests/features/app-shell.feature` + `app_shell_steps_test.go` — a logged-in player sees the header/nav on all 3 destinations, can navigate between them, and can log out via the shell's logout affordance and land back on `/login`. Register the new `InitializeAppShellScenario` in `suite_test.go`, reusing its shared `signedSessionCookieForTest`/`newTempStore` helpers.

## Tasks & Acceptance

**Execution:**
- [x] `src/internal/auth/session.go` + `session_test.go` -- add player-id context propagation -- lets handlers read identity without re-parsing the cookie
- [x] `src/internal/store/store.go` + `store_test.go` -- add `Season()` and `FindPlayerByID` -- header needs both to render
- [x] `src/internal/web/web.go` -- thread context through `requireSession`; add `/predict`, `/leaderboard`, `/compare` routes; alias `/` to Predict; render shell with per-tab "Coming soon" content and a header logout control -- makes the shell reachable and identity-aware
- [x] `src/internal/web/templates/shell.html` -- new shared shell template -- avoids duplicating header/nav markup across 3+ future epics' pages
- [x] `src/internal/web/static/styles.css` -- add missing color tokens + shell layout rules -- fixed header/nav, scrolling content, matches DESIGN.md
- [x] `src/internal/web/web_test.go` -- unit-test the I/O matrix above
- [x] `src/acceptance-tests/features/app-shell.feature` + `app_shell_steps_test.go` + `suite_test.go` registration -- end-to-end shell/nav/identity/logout-affordance proof

**Acceptance Criteria:**
- Given I am logged in and on any of Predict, Leaderboard, or Compare, when the page renders, then a pinned header shows my player name (bold, left) and the season label (muted, right), with no logo and no player switcher, and a pinned bottom nav shows exactly Predict/Leaderboard/Compare with the active tab in `ice` and the others `muted`
- Given I am logged in, when I tap a different nav tab, then that destination renders with only its content region scrolling — header and nav never move
- Given I am on a phone-width viewport (~360–430px), when any shell page renders, then the layout is single-column and dark-theme only, and on a wider viewport the column stays centered at the same max width rather than stretching
- Given I have no valid session, when I request `/predict`, `/leaderboard`, or `/compare` directly, then I am routed to `/login` exactly as `GET /{$}` already behaves today

### Review Findings

From the ad hoc `bmad-code-review` pass (combined with story 1.4, range `03b103b..HEAD`):

- [x] [Review][Patch] Bottom-nav tabs render text-only, omitting the checklist/medal/people icons DESIGN.md's Bottom Navigation component (UX-DR5) specifies — `src/internal/web/templates/shell.html`'s nav anchors have no icon markup. Resolved: added inline SVG line icons, matching DESIGN.md's token system (no shadows/gradients, ≥32px tap target) [src/internal/web/templates/shell.html]
- [x] [Review][Patch] README's Usage section documents login and session behavior but never mentions that a player can log out [README.md]
- [x] [Review][Patch] README says the pool's players are "added by hand-editing that file," but the documented `docker run` uses a named volume (`fantasy-hockey-data:/data`) with no bind-mount, and never explains how an operator reaches that file (e.g. `docker cp`/`docker exec`) [README.md]
- [x] [Review][Patch] README doesn't document that if no `SMTP_*` vars are set, login-code emails silently fail to send (logged server-side only) while the HTTP response stays identical — an operator deploying without SMTP configured gets a completely broken login flow with no visible symptom [README.md]
- [x] [Review][Patch] New `--green: #3fb950` CSS token sits next to the pre-existing, differently-valued `--green-btn: #238636` with no comment distinguishing intended use, inviting a future contributor to grab the wrong one [src/internal/web/static/styles.css]
- [x] [Review][Patch] Unit coverage for expired/tampered/empty-player-id sessions exists only for `GET /{$}`, not for `/predict`/`/leaderboard`/`/compare` — only the missing-cookie case is tested across all 4 routes; `requireSession` is shared and unchanged so the risk is low, but it's a literal shortfall against the Code Map's own test assignment [src/internal/web/web_test.go]
- [x] [Review][Patch] `appShellScenarioState.thePlayerIsSignedIn` discards its captured player-name parameter (`_ string`) and always logs in as the hardcoded `basti` fixture — a future scenario naming a different player would silently still test `basti` [src/acceptance-tests/app_shell_steps_test.go:71-74]

**Rejected:**
- **false** — "README's Configuration table doesn't document env-var-vs-flag precedence" — the `DATA_FILE` row already states "a `--data-file` flag overrides it," and there is no port env var at all (only `--port`/`-p`), so there's no undocumented ambiguity to fix.
- **false** — "No test asserts the season label renders correctly on Leaderboard/Compare, only player name is checked across destinations" (acceptance-level claim) — `web_test.go`'s `TestNewServerShouldRenderTheShellForEveryDestination` already runs `assertShellHeader` (name + season) for every one of the 4 shell routes at the unit level; the acceptance suite not duplicating that is consistent with this codebase's established acceptance-complements-unit-tests convention.
- **false** — "The logout acceptance scenario's own name implies deriving the request from the rendered form, but it POSTs directly, so a form-divergence regression would go undetected" — refuted: `web_test.go`'s `assertLogoutControl` already unit-tests the rendered form's exact markup (`action="/logout"`, method, button) for every shell render; a regression there would be caught, just not by the acceptance layer.
- **false** — "The dual-mux-registration risk has no scheduled point where it gets fixed since it's only recorded as prose in `deferred-work.md`" — `deferred-work.md` is this repo's established backlog for exactly this kind of item, per identical precedent from stories 1-1 and 1-2's own deferred findings; it is not expected to also appear in `sprint-status.yaml`.
- **low, rejected** — "`store.Season()`/`formatSeason` have no guard or test for an empty/malformed season value" (blind-hunter + edge-case-hunter, same root cause) — `Season` is a hand-maintained operator config value seeded non-empty by default, unlikely to be hit in practice, and the fix (input-validation guards) is more than a direct correction.
- **low, rejected** — "Spec's I/O & Edge-Case Matrix table isn't column-padded per the repo's Markdown style rules" (also flagged on spec-1-4) — fix requires editing the spec under review, out of scope for code review.

## Implementation Notes

## Spec Change Log

## Review Triage Log

- **false** (blind-hunter) — "No Gherkin scenario for unauthenticated redirect on the 3 new routes" — the Code Map explicitly assigns this coverage to `web_test.go` (unit level), matching CLAUDE.md's "acceptance tests complement, not replace unit tests"; the existing generic `stay-logged-in.feature` already proves `requireSession` end-to-end on `/`.
- **medium** (blind-hunter) — "No test asserts the rendered shell page contains the logout form/button" — verified: `app_shell_steps_test.go`'s logout scenario POSTs to `/logout` directly rather than deriving it from the rendered page, and no test greps `shell.html`'s output for `logout-form`/`action="/logout"`; a regression that silently broke or removed the rendered control would go undetected. Routed to `patch`.
- **false** (blind-hunter) — "sprint-status.yaml (`in-progress`) disagrees with spec status (`in-review`)" — by design: `sync-sprint-status.md` is only invoked at step-03 (`in-progress`) and step-05 (`review`); the mismatch is a transient, expected mid-workflow state, not a defect.
- **false** (blind-hunter) — "`--ice-deep`/`--gold`/`--green`/`--sel` CSS tokens are dead/unused" — the spec's own Code Map calls for adding exactly these 5 tokens, and DESIGN.md's Colors section defines them as the shared, product-wide palette later epics (leaderboard leader, prediction-status badges, selected-cell borders) consume — forward-declared shared tokens, not dead code.
- **low** (blind-hunter) — "Active nav tab has no `aria-current` — accessibility gap" — verified: `shell.html`'s active `<a>` carries only a CSS class, no `aria-current="page"`. Real, trivial one-line-per-anchor fix, not excluded by intent. Routed to `patch`.
- **false** (blind-hunter) — "`shell.html`'s title 'Face-Off Pool' is unexplained/inconsistent branding" — verified: DESIGN.md names the product "Face-Off Pool", and `login-email.html`/`login-code.html` already use the identical `<title>Face-Off Pool - ...</title>` convention; matches established, spec-compliant branding.
- **low, rejected** (blind-hunter) — "Tab labels are duplicated as literal strings in both `web.go`'s `tabTitles` map and `shell.html`'s nav anchors" — real but developer-only (a future tab rename could drift between page `<title>` and nav label); fix requires reshaping `shellData`/the template beyond a direct correction, and drift is unlikely given the 3 tab names are spec-fixed. Rejected per the low-finding criteria.
- **defer** (blind-hunter) — "Registering all 4 shell routes on both the outer mux and `authMux` doubles the routes needing to stay in sync" — verified as real, but the dual-mux pattern is a pre-existing architectural decision (AD-2/AD-11) already in place for `GET /{$}` before this story; this story only extended the established pattern to 3 more routes exactly as the Code Map instructed. Deferred as pre-existing, not caused by this story.
- **false** (blind-hunter) — "Stale-player-id lookup miss logs via `slog.Error`, should be `slog.Warn` for an expected/handled case" — verified: the codebase has zero `slog.Warn` call sites; every comparable expected-but-notable condition (wrong login code, malformed form, empty-player-id session) already logs at `Error` level, so this matches established convention exactly.
- **low, rejected** (blind-hunter) — "`formatSeason` is untested against malformed/multi-hyphen season values" — real but `Season` is a hand-maintained operator config value (per Design Notes), not user input; no other config value gets this kind of fuzz coverage. Unlikely to be hit in practice and the fix (validation/guards) is more than a direct correction. Rejected per the low-finding criteria.
- **false** (edge-case-hunter) — "`handleShell` silently degrades with no log when `PlayerIDFromContext` returns `ok=false`" — verified unreachable: `handleShell` is only ever invoked behind `requireSession`, which unconditionally populates the context before calling `next`; the branch cannot currently be hit in production.
- **false** (edge-case-hunter) — "Duplicate player IDs in `fantasy-hockey.yml` could make `FindPlayerByID` resolve the wrong row" — verified: this mirrors `FindPlayerByEmail`'s pre-existing, already-shipped lack of duplicate detection over the same hand-maintained config file; not a new risk this story introduces.
- **no verification gaps found** (verification-gap layer) — clean pass, no findings filed.
- **patched** (post-patch pipeline check, not a reviewer finding) — applying the two `patch` fixes above pushed `TestNewServerShouldRenderTheShellForEveryDestination`'s cyclomatic complexity to 12, failing `task go:complexity`'s limit of 10 — extracted `assertShellHeader`/`assertActiveTab`/`assertInactiveTabs`/`assertLogoutControl` helpers out of the test body; `task go:build` passes clean afterward.

## Design Notes

- Player-id context propagation lives in `internal/auth` (not `internal/web`) so identity stays owned by the layer that already validates and issues sessions, consistent with the established `web` → `auth` → `store` layering.
- The shell uses one shared template (`shell.html`) rather than copy-pasting header/nav markup into three standalone documents like the login pages — every later epic's screens mount inside this same shell, so duplication would compound fast.
- The stored `Season` value is the raw seed string (e.g. `"2026-27"`); the header's display transform ("NHL 2026–27") is presentation-only and belongs in the template/handler layer, matching how the login mockups already hardcode that same display string.

## Verification

**Commands:**
- `task go:test` -- expected: all unit tests pass, including new `auth`/`store`/`web` tests
- `task go:test:acceptance` -- expected: the new Gherkin scenarios pass end-to-end
- `task go:run` -- expected: binary builds and starts; manually confirm logging in lands on a real Predict shell page with working nav to Leaderboard/Compare and a working logout affordance
- `task docker:build` -- expected: full pipeline (lint, test, build image) passes
