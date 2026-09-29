---
title: 'Consolidate isGatedRound''s Duplicated Lookup and Validity Check'
type: 'refactor'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `isGatedRound` (`internal/web/compare.go`) re-derives two things `internal/web` already has canonical implementations of elsewhere in the same package: it loops `st.PredictionSets()` itself to find the matching set instead of calling `findPredictionSetByID` (`sheet.go`, already shared by `handleSheet`/`handleSheetSubmit` so the lookup can't drift between them), and it re-checks "known phase + parseable deadline" inline instead of sharing `selectableCompareSets`'s own version of that same check a few lines below it in the same file (epic-5 retrospective, item 37).

**Approach:** Extract a shared `compareSetDeadline(set) (time.Time, error)` helper - known phase and parseable `deadline_utc`, returning the parsed deadline or a description of what's wrong - used by both `isGatedRound` and `selectableCompareSets`. Change `isGatedRound` to look up its set via `findPredictionSetByID` instead of its own inline loop. Both functions' existing test-verified boolean/observable behavior (per `compare_test.go`'s `TestIsGatedRoundShouldReportFalseForARoundGatedIDWithABadDeadlineUnknownPhaseOrNoEntry` and friends) stays unchanged; `selectableCompareSets`'s two distinct `slog.Error` messages ("unknown prediction set phase" vs "build compare chip") collapse into one, since the shared helper's single returned error is the natural, minimal way to actually eliminate the duplicated validity check rather than leaving a residual duplication just to keep two separate log message strings - no test asserts on the exact message text.

</frozen-after-approval>

## Implementation Notes

Extracted `compareSetDeadline(set) (time.Time, error)` in `internal/web/compare.go`: known phase + parseable `deadline_utc`, returning the parsed deadline or a wrapped error describing what's wrong. `isGatedRound` now calls `findPredictionSetByID` (`sheet.go`) instead of its own inline loop, and calls `compareSetDeadline` instead of re-deriving the phase/deadline check inline. `selectableCompareSets` calls the same `compareSetDeadline`, logging its returned error (one `slog.Error("build compare chip", ...)` call) instead of two separately-conditioned `slog.Error` calls with two different message strings ("unknown prediction set phase" vs "build compare chip").

No new tests written first (not classic TDD red-green): this is a behavior-preserving refactor with no new observable behavior, fully covered by the existing test suite (`TestIsGatedRoundShouldReportFalseForARoundGatedIDWithABadDeadlineUnknownPhaseOrNoEntry` and siblings already exercise every branch `compareSetDeadline`/`isGatedRound`/`selectableCompareSets` can take). Ran the full `internal/web` suite before touching any code to confirm a clean baseline, then after each edit.

Verified `compareSetDeadline` genuinely gates both callers, not a dead-code duplicate left unwired: temporarily disabled its phase check (`if false` in place of the real condition), confirmed both `TestBuildCompareShouldLeaveOutSetsWithABadDeadlineOrUnknownPhase` and `TestIsGatedRoundShouldReportFalseForARoundGatedIDWithABadDeadlineUnknownPhaseOrNoEntry` fail, then restored from a `cp` backup (not `git checkout`, per this session's own established process fix after an earlier build's near-miss).

**Observable change, called out explicitly for review:** `selectableCompareSets`'s two previously-distinct log messages collapse into one (`"build compare chip"`, with the reason - unknown phase or bad deadline - folded into the `error` field instead of a separate `"unknown prediction set phase"` message with a `phase` field). No test asserts on either exact message string (verified via grep before making this change); this is the natural, minimal way to actually eliminate the duplicated validity check the retro item asked to consolidate, rather than leaving the phase-check condition duplicated a second time just to preserve two distinct log lines.

Verified: `gofmt -l` clean, `go vet ./...` clean, `go build ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:test:acceptance` full pipeline green (84.0% coverage, up fractionally from 83.8% - the consolidated function is now exercised from two call sites instead of duplicated code each covered once), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied three `patch`-routed blind-hunter findings — reworded `compareSetDeadline`'s doc comment away from a "reports whether..." predicate-style opening (Go convention reserves that phrasing for a function returning a bool; this one returns `(time.Time, error)`) to a plain "validates..." description of what it does and returns; added `TestCompareSetDeadline`, a direct three-case table test (unknown phase, unparseable deadline, success) pinning the new standalone function's own contract independent of either caller's indirect coverage; and added a one-sentence breadcrumb to the same doc comment naming `predict.go`'s `buildPredictPhases`, which still has its own third, independent "unknown prediction set phase" check out of this item's scope, so a future consolidation pass has an in-file pointer to it. One finding dispositioned `low, reject`: the structured `phase` field lost from the collapsed log message has no durable record beyond this spec's Review Triage Log and the commit message - not fixed further, since this repo has no separate CHANGELOG file (the project uses semantic-release, which generates release notes from commit messages), and the commit message for this change explicitly documents the log consolidation, which is exactly the durable, versioned record this concern was asking for. One finding dispositioned `false`: spec/sprint-status tracking still showing in-progress/open mid-review is expected workflow state.

**Re-verified after patches**: the new `TestCompareSetDeadline` table test passes (3/3 subtests), full `internal/web` suite green, `gofmt -l` clean, `go vet ./...` clean, `go build ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:test:acceptance` full pipeline green (84.0% coverage), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — `compareSetDeadline`'s doc comment opened with "reports whether...", a phrasing Go convention reserves for boolean-returning functions, but this one returns `(time.Time, error)` - misleading to a reader skimming signatures. Verified real. Fixed: reworded to describe what the function validates and returns.
- **low, patch** — no direct unit test existed for the new, now-standalone, reusable `compareSetDeadline` function; it was only exercised indirectly through `isGatedRound`'s and `selectableCompareSets`'s own tests. Verified real: a small table test pins the three-branch contract (unknown phase, bad deadline, success) independent of either caller. Fixed: added `TestCompareSetDeadline`.
- **low, patch** — the new doc comment didn't mention that `predict.go`'s `buildPredictPhases` still has its own, third, independent "unknown prediction set phase" check, leaving no in-file breadcrumb for a future consolidation pass touching that file. Verified real (confirmed the exact function name and line). Fixed: added a one-sentence pointer to the doc comment.
- **low, reject** — the collapsed log message drops the structured `phase` field that let log consumers filter on an unknown-phase failure independently of a bad-deadline one, with no durable record of this tradeoff beyond the spec itself. Not fixed further: this repo has no CHANGELOG file to add to (release notes are generated from commit messages via semantic-release), and this change's own commit message explicitly documents the log consolidation - the durable record the finding asked for already exists in the form this repo actually uses.
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-progress/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.

All three patched findings were independently re-verified after patching: the new table test passes, the full `internal/web` suite remains green, and the full `gofmt`/`go vet`/`go build`/`task go:lint`/`task go:test`/`task go:test:acceptance`/`task go:run` pipeline is clean.
