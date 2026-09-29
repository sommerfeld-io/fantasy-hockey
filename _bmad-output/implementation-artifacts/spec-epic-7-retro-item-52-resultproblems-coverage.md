---
title: 'Prove ResultProblems Reaches openStore WARN Loop and AC4 Count Line'
type: 'test'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 7.4 added two new `ResultProblems()` categories — a tolerated shape error (`toleratedShapeErrorProblemsLocked`) and an unknown/misspelled key (`unknownKeyProblemsLocked`) — and both are covered directly against `st.ResultProblems()` in `internal/store/store_results_test.go`. Neither is covered through the actual startup path in `main.go`'s `openStore`: the per-problem `logger.Warn("malformed result in data file", ...)` loop and the AC4 `logger.Info("result problems found", "count", ...)` summary line. `main_test.go` only exercises that integration for a team-abbreviation-validation problem (`TestOpenStoreShouldWarnAboutAMalformedResultAndStillSucceed`), not for these two specific categories (epic-7 retrospective, item 52).

**Approach:** Add two focused tests to `main_test.go`, mirroring `TestOpenStoreShouldWarnAboutAMalformedResultAndStillSucceed`'s shape (seed a temp data file, call `openStore`, assert on the captured log buffer): one seeding a wrongly-shaped `results:` entry (tolerated shape error) and one seeding a misspelled/unknown key, each asserting the WARN line appears and the AC4 count line reports the correct count. No production code changes — this closes a test-coverage gap only.

</frozen-after-approval>

## Implementation Notes

Added two tests to `main_test.go`, mirroring `TestOpenStoreShouldWarnAboutAMalformedResultAndStillSucceed`'s shape: `TestOpenStoreShouldWarnAboutAToleratedShapeErrorFromResultProblems` (seeds a wrongly-shaped `results.team_marks.atlantic.playoffs` entry, a scalar where a list is expected) and `TestOpenStoreShouldWarnAboutAnUnknownKeyFromResultProblems` (seeds `results.stanley_cup_winer` — a misspelled key). Both call `openStore` directly and assert on the captured `slog` buffer: exactly one WARN line naming the problem, and the AC4 `count=1` summary line. Pure test-coverage addition — no production code changed, `ResultProblems`/`openStore` already implement both categories from Story 7.4.

Not classic TDD red-green since no implementation changed; instead each test's ability to actually catch a regression was proven empirically: temporarily commented out `toleratedShapeErrorProblemsLocked`'s and `unknownKeyProblemsLocked`'s contributions to `ResultProblems` (one at a time) and confirmed the matching new test fails with `count=0` and no WARN line, then restored (`git checkout -- internal/store/store_results.go`) and re-ran clean.

The tolerated-shape-error seed needed a `teams:` list naming FLA — without it, `division_winner: FLA` itself flags as an unknown-team problem (no teams loaded), producing 2 problems instead of the intended 1, which the test caught during development.

Verified: `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied all three `patch`-routed blind-hunter findings — strengthened the tolerated-shape-error test's assertion from a loose `strings.Contains(logs, "results:")` to a section-prefixed, line-anchored check (`problem="results: ` + `"line"`), matching the pattern `store_results_test.go` already uses and avoiding a false pass against an unrelated `results:`-section problem; added a "should not" counterpart test (`TestOpenStoreShouldNotWarnAboutCorrectlyShapedTeamMarksOrKeyNames`) proving the correctly-shaped/spelled equivalents of both new seeds (a real `team_marks.playoffs` list, correctly-spelled `stanley_cup_winner`) don't false-positive a WARN through `openStore`; and added an inline comment in `main_test.go` explaining why the tolerated-shape-error seed needs `teams: FLA` (undocumented coupling with the unrelated team-abbreviation validation path). One finding dispositioned `false`: per-file spec/sprint-status tracking still showing in-progress/open mid-review is expected workflow state, resolved by this same build's Finalize step.

**Re-verified after patches**: all `TestOpenStoreShould*` tests pass (8/8, including the 3 new/patched ones), each new/patched test's ability to catch a real regression proven empirically (temporarily broke the underlying detection or forced an extra problem, confirmed the matching test fails, then restored via `git checkout`), `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — `TestOpenStoreShouldWarnAboutAToleratedShapeErrorFromResultProblems` asserted only a loose `strings.Contains(logs, "results:")`, the exact weaker pattern `store_results_test.go` moved away from for the same category (spec-7-4 review history). Verified real: would pass even if the WARN line named an unrelated `results:`-section problem. Fixed: section-prefixed (`problem="results: `) plus `"line"` check.
- **low, patch** — neither new test had a "should not" counterpart proving the correctly-shaped/spelled equivalent seed doesn't false-positive through `openStore` (CLAUDE.md TDD rule). Verified real: the existing generic `TestOpenStoreShouldNotWarnForAWellFormedFile` uses a different, simpler seed and wouldn't catch a regression specific to these two new detection paths. Fixed: added `TestOpenStoreShouldNotWarnAboutCorrectlyShapedTeamMarksOrKeyNames`, proven to catch a forced extra problem.
- **low, patch** — the tolerated-shape-error seed's non-obvious `teams: FLA` requirement (needed only to keep `division_winner: FLA` from tripping an unrelated unknown-team problem) was documented only in this spec file, not in `main_test.go` itself. Verified real: a future maintainer reading only the test file could "simplify" the seed and silently break test isolation. Fixed: added an inline comment.
- **low, reject** — coverage of the `award_finalists:` sibling section (in addition to `results:`) for the tolerated-shape-error category through `openStore`. Both sections are already covered at the `ResultProblems()` level (`store_results_test.go`); item 52's stated goal is proving the two *categories* (tolerated shape error, unknown key) reach the `openStore` integration path generically, not exhaustive per-section coverage through that path — out of scope for this item.
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-progress/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.
- **false** — reviewer flagged confirming the new spec file under `_bmad-output/` is intentionally meant to be committed. Not a defect: this is the established pattern for every prior build this session (all `spec-epic-*-retro-item-*.md` files are committed project history), not a scratch file.

All three patched findings were independently re-verified after patching: full `TestOpenStoreShould*` suite (8/8), each fix's regression-catching ability proven empirically, and the full `task go:test`/`task go:lint`/`task go:run` pipeline clean.
