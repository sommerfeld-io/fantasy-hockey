---
title: 'Tighten main_test.go''s Two Rollover Tests'
type: 'test'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `main_test.go`'s two rollover tests (`TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist`, `TestOpenStoreShouldNeverTouchAnArchivedFileAtADifferentPath`) have three gaps: (1) the bootstrap test's doc comment claims to prove the rollover entry point "end-to-end through main's actual startup path" and mentions "repointing DATA_FILE/--data-file," but the test calls `openStore` directly with an already-constructed path - it never exercises `resolveConfig`'s env/flag resolution or `run()`'s wiring, so the comment overclaims what's actually covered; (2) the archived-file test puts the archived and fresh files in two separate `t.TempDir()` calls, an unrealistic topology - a real rollover archives and creates the new file in the same directory; (3) both tests discard their log output into an anonymous, never-inspected `&bytes.Buffer{}`, so neither asserts log silence (no WARN lines) on a clean bootstrap the way the sibling `TestOpenStoreShouldNotWarnForAWellFormedFile` already does (epic-6 retrospective, item 45).

**Approach:** Correct the bootstrap test's doc comment to accurately scope what it proves (openStore's bootstrap behavior for an already-resolved nonexistent path, not the full env/flag/run() wiring). Colocate the archived-file test's two paths in one shared `t.TempDir()`. Capture both tests' log output into an inspectable buffer and assert no WARN lines plus the AC4 `count=0` summary line, matching `TestOpenStoreShouldNotWarnForAWellFormedFile`'s existing convention.

</frozen-after-approval>

## Implementation Notes

`TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist`: corrected the doc comment to accurately scope what it proves (`openStore`'s bootstrap behavior for an already-resolved path, not `resolveConfig`'s env/flag resolution or `run()`'s wiring - confirmed via `task go:test`'s own coverage report that `main`/`run` show 0.0% coverage); replaced the discarded anonymous `&bytes.Buffer{}` with a captured `logs` buffer; added assertions for no `level=WARN` lines and the AC4 `count=0` summary line, mirroring `TestOpenStoreShouldNotWarnForAWellFormedFile`'s existing convention.

`TestOpenStoreShouldNeverTouchAnArchivedFileAtADifferentPath`: colocated `archivedPath`/`freshPath` in one shared `t.TempDir()` instead of two separate ones, matching a real rollover's same-directory topology; corrected its doc comment with the same resolveConfig/run() scope note; applied the identical log-capture-and-assert pattern.

Verified both new log-silence assertions actually catch a real regression, not just pass vacuously: temporarily made `main.go`'s `openStore` emit a spurious `logger.Warn(...)` line before its AC4 summary, confirmed `TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist` fails with the expected message, then restored via `git checkout`.

Verified: `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied all four `patch`-routed blind-hunter findings — extracted `assertLogSilenceAndZeroProblems(t, logs, scenario)`, replacing the now-triplicated 4-line assertion block (across the pre-existing `TestOpenStoreShouldNotWarnForAWellFormedFile` and both tightened tests) with one shared helper, per `src/CLAUDE.md`'s own stated preference for helper functions over repeated inline blocks; gave the helper a `scenario` parameter so each call site's failure message stays legible to its actual test context, fixing the copy-pasted "freshly bootstrapped file" wording that previously also appeared verbatim in the archived-file test's failure message; reworded both doc comments' scope note away from a point-in-time measured coverage percentage ("main/run show 0% coverage per task go:test's own report") to a structural statement ("run() is exercised by neither"), avoiding the same staleness shape this codebase just added guardrails against elsewhere (`DefaultSeason`, epic-6 items 42/47) - a future `run()`-level test wouldn't leave this comment silently wrong; and documented in the new helper's own doc comment that it only observes what `openStore` itself logs through the injected logger, not `store.New`'s own bootstrap-write line (logged separately via the package-level default `slog` logger) - so "log silence" here is scoped precisely, not implied to cover everything a bootstrap actually logs. One finding dispositioned `false`: spec/sprint-status tracking still showing in-progress/open mid-review is expected workflow state.

**Re-verified after patches**: all three affected tests pass (`TestOpenStoreShouldNotWarnForAWellFormedFile`, `TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist`, `TestOpenStoreShouldNeverTouchAnArchivedFileAtADifferentPath`), the shared helper's regression-catching ability re-proven empirically across all three call sites (forced a spurious WARN in `openStore`, confirmed all three fail with their own scenario-specific message, restored via `git checkout`), `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — the 4-line "no WARN + count=0" assertion block was copy-pasted across three tests (one pre-existing, two newly tightened), contradicting `src/CLAUDE.md`'s stated preference for helper functions over repeated blocks. Verified real by citing the exact convention. Fixed: extracted `assertLogSilenceAndZeroProblems`, updated all three call sites.
- **low, patch** — the archived-file test's copy-pasted failure message ("expected no warnings for a freshly bootstrapped file") didn't reflect its actual archived-file-isolation scenario. Verified real. Fixed: gave the new helper a `scenario` parameter, so each call site gets a legible, context-specific message.
- **low, patch** — both corrected doc comments cited a point-in-time measured coverage percentage ("main/run show 0% coverage per task go:test's own report"), the same staleness shape this codebase just added guardrails against elsewhere (`DefaultSeason`, epic-6 items 42/47 landed earlier the same day). Verified real. Fixed: reworded to a structural statement that doesn't depend on a specific measurement staying accurate.
- **low, patch** — "assert log silence on a clean bootstrap" (the spec's own wording) reads as stronger than what's actually tested: `store.New`'s own bootstrap-write `slog.Info` call goes through the package-level default logger, not the `*slog.Logger` instance the test constructs and passes into `openStore`, so it's invisible to the captured `logs` buffer. Verified real by running with `-v` and observing the stray INFO line print outside the captured buffer. Fixed: documented the precise scope in the new helper's own doc comment, rather than silently letting the assertion imply broader coverage than it has.
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-progress/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.

All four patched findings were independently re-verified after patching: all three affected tests pass, the shared helper's regression-catching ability re-proven across all three call sites, and the full `gofmt`/`go vet`/`task go:lint`/`task go:test`/`task go:run` pipeline is clean.
