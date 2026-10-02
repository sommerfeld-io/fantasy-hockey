---
title: 'Rules Cross-Destination Identity Scenario'
type: 'test'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `src/acceptance-tests/features/app-shell.feature`'s "Navigating between destinations keeps showing the player's identity" scenario visits Predict, Leaderboard and Compare but not Rules — even though Rules is a 4th destination added in Epic 8, following the exact pattern that motivated this scenario's own creation (Epic 1 retro item 2: "extend... to cover /predict, /leaderboard, /compare, not just /"). Not a live defect (`handleShell` sets `PlayerName` unconditionally before any tab-specific branch — verified in the Epic 8 retrospective) but a coverage gap the epic-8 retrospective flagged (item 62).

**Approach:** Add `And the player visits the Rules destination` to the existing scenario in `app-shell.feature`, before its final `Then` step. `src/acceptance-tests/app_shell_steps_test.go`'s `thePlayerVisitsTheDestination` step regex already matches `Rules` (`^the player visits the (Predict|Leaderboard|Compare|Rules) destination$`, added when Rules was first introduced) — no Go code change needed, purely a one-line Gherkin addition.

</frozen-after-approval>

## Implementation Notes

Added `And the player visits the Rules destination` to `app-shell.feature`'s cross-destination identity scenario, before its final `Then`. No Go code change needed — `thePlayerVisitsTheDestination`'s step regex already matched `Rules`, confirmed by reading `app_shell_steps_test.go` before making the change.

Verified: `go test ./acceptance-tests/...` (full suite green, the new scenario line runs and passes — confirmed by inspecting the GoDog verbose output line-by-line, not just the exit code), `task go:test` and `task go:test:acceptance` (both full pipelines clean), `task go:run` (builds and starts).

Nothing incomplete or risky.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, defer** — Rules never backfilled into `stay-logged-in.feature`/`log-out.feature`'s cross-destination scenarios, or `app-shell.feature`'s own logout scenario. Verified real via `grep` (zero mentions of "rules" in either file); pre-existing since Epic 8, not caused by this change. Deferred to `deferred-work.md`.
- **low, defer** — `everyVisitedDestinationShowedThePlayersName` checks only the player's name, never the season. Verified against the step's implementation. Pre-existing scope, not caused by this change. Deferred.
- **low, defer** — the identity scenario never includes the root path `/`. Verified real. Pre-existing omission, not caused by this change. Deferred.
- **low, defer** — no single source of truth ties `navHrefsByTab`, the step regex, and each destination's title-casing together in the test helpers. Verified real. Pre-existing design, not caused by this change. Deferred.
- **low, defer** — `theShellShowsTheRulesContent` lacks a placeholder-absence check unlike its Compare sibling. Verified real. Pre-existing gap in a different scenario. Deferred.

All five findings are real but predate this change (adding one line to the identity-persistence scenario) — none caused or exposed by it, so none route to `patch`.
