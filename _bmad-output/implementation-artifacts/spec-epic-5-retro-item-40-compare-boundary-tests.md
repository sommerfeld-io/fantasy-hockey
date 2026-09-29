---
title: 'Close Three Compare Story-Boundary Test-Coverage Gaps'
type: 'test'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Three Compare-related test-coverage gaps identified in the epic-5 retrospective: (1) every existing tagged-value test (`TestBuildCompareShouldTagDivisionPlayoffTeamsAndWinner`, `TestBuildCompareShouldTagTheSeriesWinnerAndKeepTheGamesCountPlain`) reads the pick of a non-signed-in player ("sadl") into a non-own column, so nothing proves the signed-in player's own column correctly renders a tagged value together with its own-cell/own-column styling; (2) `isGatedRound`'s only "reports true" coverage uses `store.Round2SetID` ("r2") - `store.ConferenceFinalsSetID`/`store.StanleyCupFinalSetID` ("cf"/"scf"), also members of `roundGatedSetIDs`, are never driven through the function at all; (3) nothing proves `roundGatedSetIDs` (predict.go) stays a subset of `seriesSetIDs` (sheet.go) - a future round-gated id added to one map without the other would silently break Story 3.3's series sheet for that round (epic-5 retrospective, item 40).

**Approach:** Add a test proving the signed-in player's own column renders a tagged value with `Own: true` set correctly at both the column and cell level. Extend/add `isGatedRound` coverage for `cf`/`scf` (both a "still gated, no matchups yet" case and, reusing the existing `compareDefaultMatchups` fixture's already-recorded `scf` matchup, an "unlocked" case). Add a consistency test asserting every `roundGatedSetIDs` key is also a `seriesSetIDs` key.

</frozen-after-approval>

## Implementation Notes

Added `TestBuildCompareShouldTagTheOwnColumnsValueAndKeepOwnStyling`: seeds a division-playoff-teams pick for the signed-in player ("basti") rather than a non-signed-in one, then asserts the own column (index 1) carries `Own: true`, the matching row's own cell is also `Own: true`, and its values are still correctly tagged (`compareValueTagCSS`).

Extended `TestIsGatedRoundShouldReportTrueOnlyForARealStillUpcomingSet` with an `scf`-already-unlocked assertion (reusing `compareDefaultMatchups`'s existing recorded `scf` matchup, per its own doc comment) and added `TestIsGatedRoundShouldAlsoReportTrueForConferenceFinalsAndStanleyCupFinal`, driving `isGatedRound`'s "still gated" path through `cf`/`scf`, not just `r2`.

Added `TestRoundGatedSetIDsShouldBeASubsetOfSeriesSetIDs`: iterates `roundGatedSetIDs` (predict.go) asserting every key is also a `seriesSetIDs` (sheet.go) key.

Not classic TDD red-green for the first two (behavior already correct, pure coverage additions) - proved each test's actual regression-catching power empirically instead: (1) temporarily forced `own := false` in `compare.go`'s cell-building loop, confirmed the tag test fails; (2) temporarily removed `store.StanleyCupFinalSetID` from `roundGatedSetIDs`, confirmed the cf/scf test fails; (3) temporarily added a phantom, `seriesSetIDs`-absent id to `roundGatedSetIDs`, confirmed the consistency test fails. All three restored via `cp`-backed copies (not `git checkout`, per this session's own established process fix), then re-verified clean.

Verified: `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:test:acceptance` full pipeline green (84.0% coverage, no regressions), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied four `patch`-routed blind-hunter findings — added explicit CSS-string assertions (`comparePlayerOwnCSS`/`compareCellOwnCSS`) to the own-column test, which previously asserted only the boolean `Own` field and would have missed a regression that dropped the own-CSS modifier while leaving the bool correct; added `TestIsGatedRoundShouldReportFalseForConferenceFinalsOnceMatchupsAreRecorded`, closing an asymmetry where only `scf`'s "unlocked" branch was exercised (by reusing an existing fixture) while `cf`'s own unlocked branch had zero coverage; added a `len(roundGatedSetIDs) == 0` guard to the subset consistency test, since a pure subset check passes vacuously if the map were ever accidentally emptied; and simplified `&v.Table.Rows[i].Cells[1]` to `&r.Cells[1]` directly off the range variable, matching the file's existing style elsewhere and removing an unnecessary re-index. One finding dispositioned `low, reject`: only the division-playoff-teams tag category got an own-column counterpart, not the series category or the division-*winner* row - not fixed, since the retro item's own wording asks for "an own-column-holds-a-tag scenario" (singular), and one representative scenario proving the mechanism (own styling + tag styling coexist correctly) is what was asked for; expanding to every tag-producing category is a materially larger scope than a "close three... gaps" test-hygiene item calls for. One finding dispositioned `false`: the BDD/acceptance-test policy question was already resolved as a standing decision (epic-8-item-63 build) for pure test-coverage additions with no observable behavior change. One finding dispositioned `false`: spec/sprint-status tracking still showing in-progress/open mid-review is expected workflow state.

**Re-verified after patches**: re-ran the empirical break-and-restore proof for both new/patched assertions (forced `comparePlayerCSS` instead of `comparePlayerOwnCSS` in `compare.go`, confirmed the new CSS assertion catches it; forced `isGatedRound`'s matchups check to always report gated, confirmed the new cf-unlocked test catches it) - both restored cleanly. `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:test:acceptance` full pipeline green (84.0% coverage, no regressions), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — the own-column test asserted only the boolean `Own` field, never the actual `comparePlayerOwnCSS`/`compareCellOwnCSS` strings its own name promises to cover; a regression dropping the CSS modifier while leaving the bool correct would go undetected. Verified real (grep confirmed neither constant was referenced anywhere in the test file). Fixed: added explicit CSS assertions, proven to catch a forced regression.
- **low, patch** — `cf`'s "unlocked" (matchups-recorded) branch had zero test coverage; only `scf`'s was exercised, via a fixture reused from a pre-existing test. Verified real. Fixed: added `TestIsGatedRoundShouldReportFalseForConferenceFinalsOnceMatchupsAreRecorded`, proven to catch a forced regression.
- **low, patch** — the subset consistency test would pass vacuously if `roundGatedSetIDs` were ever accidentally emptied. Verified real (an empty set is trivially a subset of anything). Fixed: added a non-emptiness guard.
- **low, patch** — `&v.Table.Rows[i].Cells[1]` re-indexed through the slice instead of using `&r.Cells[1]` directly off the range variable, inconsistent with how the rest of the file iterates rows/cells. Verified real and harmless either way (slice headers share the same backing array). Fixed: simplified to match file style.
- **low, reject** — only the division-playoff-teams tag category got an own-column counterpart, not the series category or the division-winner row. Not fixed: the retro item's own wording asks for "an own-column-holds-a-tag scenario" (singular) - one representative scenario satisfies that; covering every tag-producing category is a materially larger scope than this item calls for.
- **false** — reviewer flagged that CLAUDE.md's BDD policy requires asking whether an acceptance test is needed for non-feature work, with no record of that question being asked. Not a defect: already resolved as a standing decision in the epic-8-item-63 build - a pure test-coverage addition with zero observable behavior change squarely fits that standing policy.
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-progress/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.

All four patched findings were independently re-verified after patching: both new/strengthened assertions' regression-catching ability proven empirically, the full `internal/web` suite remains green, and the full `gofmt`/`go vet`/`task go:lint`/`task go:test`/`task go:test:acceptance`/`task go:run` pipeline is clean.
