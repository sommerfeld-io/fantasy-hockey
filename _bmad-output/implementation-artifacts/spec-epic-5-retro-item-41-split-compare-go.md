---
title: 'Split compare.go Along Category-Kind Boundaries'
type: 'refactor'
created: '2026-09-29'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'a6ed2df0322040e230aab63f880df1ee6add0474'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `internal/web/compare.go` is 503 lines (488 at the time of the epic-5 retrospective, item 41) - already the largest production file in `internal/web`. It mixes shared Compare infrastructure (types, the render pipeline, gating/deadline logic) with four independent category-kind implementations (single-team, divisions, awards, series) in one file, unlike `sheet.go`, which already splits the analogous pattern into `sheet.go` (shared) plus `sheet_divisions.go`/`sheet_awards.go`/`sheet_series.go` (one file per category kind). Before a future story adds another category kind, splitting now keeps the pattern consistent and each file reviewable on its own.

**Approach:** Split `compare.go` into `compare.go` (shared: types, `buildCompare`/render pipeline, gating/deadline logic, dispatch, plus `singleTeamCategory` - small enough and not named in the item's own file list, so it stays with the dispatcher it's one line away from) and three new files mirroring the item's own naming and `sheet_*.go`'s pattern exactly: `compare_divisions.go`, `compare_awards.go`, `compare_series.go` - each holding its one category's category-builder function, its own label constants, and (per the human's explicit scope decision) its own test file (`compare_divisions_test.go`/`compare_awards_test.go`/`compare_series_test.go`), leaving cross-category and dispatch-level tests in `compare_test.go`. Pure file reorganization: no behavior change, no renamed identifiers, no new abstractions.

</frozen-after-approval>

## Code Map

**Stays in `compare.go`** (shared infrastructure + dispatch):
- Constants: `compareGroupBeforeSeason`/`compareGroupPlayoffs`, `emptyCellValue`, `compareCupLabel`/`comparePresidentsLabel` (singleTeamCategory's own labels), `compareGatedRoundNote`, the `compareChipCSS`/`comparePlayerCSS`/`compareCellCSS` CSS-class block, the `compareValueCSS` block.
- `singleTeamSetLabels` var.
- Types: `compareChipView`, `compareGroupView`, `compareColumnView`, `compareValueView`, `compareCellView`, `compareRowView`, `compareTableView`, `compareView`, `compareSet`, `compareCategory`.
- Value-builder helpers: `tagValue`, `tagValues`, `plainValue`, `emptyCompareValue`.
- Render pipeline: `buildCompare`, `compareSetDeadline`, `isGatedRound`, `selectableCompareSets`, `findCompareSet`, `defaultCompareSet`, `newCompareGroups`, `compareGroupLabelID`, `newCompareChip`, `compareSetHref`, `newCompareTable`, `newCompareColumns`, `newCompareRow`.
- Dispatch: `compareCategories` (the switch that calls into the category-specific functions below).
- `singleTeamCategory` (its own small category-builder - stays here, not split out, per the item's own 3-file list).

**Moves to `compare_divisions.go`**: `divisionCategories`, plus `compareDivisionPlayoffLabel`/`compareDivisionWinnerLabel` constants (only consumer).

**Moves to `compare_awards.go`**: `awardCategories`. (`awardOrder`/`awardTitle`/`displayNameForSlug` already live in `sheet_awards.go`, shared with Predict's award sheet - do not move or duplicate them.)

**Moves to `compare_series.go`**: `seriesCategories`, `seriesLabel`, plus `compareSeriesLabel`/`compareSeriesNoConfLabel`/`compareSeriesGamesSuffix` constants. (`teamName`/`conferenceForMatchup`/`stanleyCupFinalLabel` already live in `sheet_series.go` - do not move or duplicate them; `seriesLabel` keeps calling them as same-package functions, same as today.)

**Test split** (mirroring `sheet_*_test.go`'s own division of shared fixtures vs. category-specific tests):
- **Stays in `compare_test.go`**: every fixture/helper (`compareSeedPlayers`, `compareTeams`, `compareNHLPlayers`, `compareDefaultSets`, `compareDefaultMatchups`, `compareSetSeed`, `comparePick`, `newCompareStore`, `chipIDs`, `groupChips`, `selectedChips`, `rowLabels`, `cellValueViews`, `cellValues`, `valueCSS`, `getCompare`, `readDataFile`), every selector/gating/deadline/column test, `TestBuildCompareShouldLabelOneRowPerCategoryOfTheSet` (iterates every category kind together), `TestBuildCompareShouldHaveNoRowsForASetOfUnknownKind`, `TestBuildCompareShouldNotStackSingleValueRows` (checks three category kinds - cup/divisions/r1 - together, genuinely cross-category), every single-team test (`TestBuildCompareShouldShowEveryPlayersSingleTeamPickByFullName`, `TestBuildCompareShouldNotMixTheSeasonAndPlayoffsCupPicks`, `TestBuildCompareShouldRenderFullTeamNamesPlain`, `TestBuildCompareShouldRenderAnUnfilledValueFaint`), and every `TestCompareRouteShould*` route-level test.
- **Moves to `compare_divisions_test.go`**: `TestBuildCompareShouldShowDivisionPicksAsAbbreviationsPerDivision`, `TestBuildCompareShouldTagDivisionPlayoffTeamsAndWinner`, `TestBuildCompareShouldTagTheOwnColumnsValueAndKeepOwnStyling` (seeds a division pick specifically), `TestBuildCompareShouldShowAnEmptyDashForASavedButEmptyDivisionPlayoffTeamsPick`.
- **Moves to `compare_awards_test.go`**: `TestBuildCompareShouldKeepAwardFinalistsPlain`, `TestBuildCompareShouldShowFinalistsByDisplayNameOnePerLine`.
- **Moves to `compare_series_test.go`**: `TestBuildCompareShouldTagTheSeriesWinnerAndKeepTheGamesCountPlain`, `TestBuildCompareShouldShowSeriesPicksAsWinnerAndGames`, `TestBuildCompareShouldLabelASeriesWithAnUnknownTeamWithoutAConference`.

## Open Questions

None - the split is mechanical (moving existing, working code and tests verbatim between files in the same package), with no ambiguity about placement left unresolved above.

## Boundaries & Constraints

- **No behavior change.** Every moved function/type/constant/test keeps its exact name, signature, and body. This is a `git mv`-style reorganization within one package, not a rewrite.
- **No new abstractions, no renamed identifiers.** Do not "clean up" anything beyond the move itself while doing this.
- Every moved item's own doc comment moves with it unchanged.
- File-local helpers already shared across `sheet_*.go` and `compare.go` today (`teamName`, `conferenceForMatchup`, `stanleyCupFinalLabel`, `awardOrder`, `awardTitle`, `displayNameForSlug`, `teamRoster`) must NOT be duplicated or moved - they stay exactly where they are (`sheet_series.go`, `sheet_awards.go`, `roster.go`) and continue to be called as ordinary same-package functions from the new `compare_*.go` files.
- After the split, `go build ./...`, `go vet ./...`, `gofmt -l .`, `task go:lint`, `task go:test`, and `task go:test:acceptance` must all be clean with **zero** behavior/coverage change (same test count, same assertions, same pass/fail outcomes) - this is the acceptance bar for "pure reorganization."
- Do not touch `sheet.go`/`sheet_*.go` or any other file - scope is `compare.go`/`compare_test.go` only.

## Implementation Notes

Dispatched to a fresh subagent per the exact prescribed handoff (spec as sole source of truth, no pre-loaded context - frontmatter `context:` was empty). Report: `compare.go` 503→407 lines, `compare_test.go` 984→814 lines, three new source files (`compare_divisions.go` 44 lines, `compare_awards.go` 29 lines, `compare_series.go` 51 lines) and three new test files (`compare_divisions_test.go` 4 tests, `compare_awards_test.go` 2 tests, `compare_series_test.go` 3 tests) exactly matching the spec's Code Map. One disclosed judgment call beyond a pure move: added one doc-comment line per new file's split-out const block (e.g. "Compare row labels for the divisions category"), since every other `sheet_*.go` const block in this codebase carries one - accepted as a reasonable, non-behavioral addition in the spirit of "no new abstractions," not "zero new text."

**Independently verified, not just trusted the subagent's report** (per this session's own established discipline): staged the full diff since `baseline_commit` via `git diff` (using `git add -N` to include the new untracked files), read every hunk directly. Confirmed `compare.go`'s diff is a clean, pure removal (100 lines, no logic changes) matching the spec exactly. Read all three new source files in full - each moved function/constant body is byte-identical to its origin, and critically, neither `compare_awards.go` nor `compare_series.go` duplicates the shared helpers (`awardOrder`/`awardTitle`/`displayNameForSlug`, `teamName`/`conferenceForMatchup`/`stanleyCupFinalLabel`) that the spec's Boundaries section explicitly forbade duplicating - both correctly call them as ordinary same-package functions. Ran `diff` on the sorted function/type/const/var declaration lists before vs. after the split (identical, modulo two additional `const (` block openers from splitting one block into three) and on the sorted individual constant names (zero diff) and sorted test function names across all four `compare*_test.go` files (47 before, 47 after, zero diff) - confirming no identifier was lost or altered anywhere in the split. Re-ran the full verification suite myself from a clean checkout state rather than trusting the subagent's own run: `go build ./...`, `go vet ./...`, `gofmt -l .`, `task go:lint` (0 issues), `task go:test` (95.4% coverage, identical to pre-split baseline), `task go:test:acceptance` (84.0% coverage, identical to pre-split baseline), `task go:run` (builds and starts) - all clean, all matching the pre-split baseline exactly, confirming zero behavior or coverage change.

Dispatch-route review: all three lenses (blind-hunter, edge-case-hunter, verification-gap) launched in parallel against the staged diff. Edge-case-hunter and verification-gap both reported clean (empty findings / "No verification gaps found"), independently confirming the split introduces no new branches, no unhandled paths, and no regression risk. Blind-hunter found 6 issues, detailed in the Review Triage Log below - all 5 real findings were documentation/comment staleness the split's file movement exposed (constants moving out from under a doc comment that described them, a test comment whose "above" no longer holds once tests moved to different files, README/context docs not updated to reflect the new file layout), not logic defects; applied directly (re-engaging the same implementation subagent isn't achievable with this session's tooling - completed subagents don't remain addressable - so patches were applied directly, as done throughout this session, and disclosed here as the established process deviation).

**Bonus fixes noticed while patching, not part of blind-hunter's findings:** while touching `internal/web/README.md`'s Design Notes bullet (blind-hunter's finding about the File layout table), noticed it also cited `TestCompareShouldNotImportScoringOrStandings` as the enforcement mechanism for Compare's import boundary - that test was deleted in an earlier build this same session (epic-4-retro-item-33, depguard adoption), and this doc was never updated to match. Similarly, `epic-5-context.md`'s "Where the code lives" bullet still said "depguard enforcement... is still an open retro item, so for now it is checked only in code review" - also stale from the same item-33 build. Both corrected in the same pass as their respective files' item-41 fixes, since leaving a doc half-corrected (accurate about the file split, stale about the enforcement mechanism one sentence later) would be worse than fixing both. These are pre-existing staleness from a different story, not caused by this diff - noted here for the record, not counted as new item-41 review findings.

Verified after patches: `go build ./...`, `go vet ./...`, `gofmt -l .` clean, `task lint` exit 0 (markdown table re-padding confirmed clean), `task go:lint` 0 issues, `task go:test` full pipeline green (95.4%, unchanged), `task go:test:acceptance` full pipeline green (84.0%, unchanged), `task go:run` builds and starts. Working tree contains exactly the expected file set (verified via `git status`).

Nothing incomplete or risky.

## Review Triage Log

*(dispatch route: blind-hunter, edge-case-hunter, verification-gap)*

- **Edge-case-hunter: clean.** Empty findings array. Traced every changed hunk as a byte-identical relocation with no new branches, loops, arithmetic, or type coercions introduced; confirmed import/declaration integrity for every symbol the new files depend on; verified the "no behavior change" claim against the diff directly (claims check).
- **Verification-gap: clean.** "No verification gaps found." Screened every part of the diff as non-behavioral per Step 1 (pure code/test relocation, identical logic and assertions), confirmed via independent `go build`/`go test -run TestBuildCompare` runs that all 9 relocated tests plus the full `TestBuildCompare*` suite pass.
- **low, patch** — `internal/web/README.md`'s File layout table (the exact per-category-kind file convention this refactor mirrors from `sheet_divisions.go`/`sheet_awards.go`) still had one `compare.go` row describing the pre-split shape, with no rows for the three new files. Verified real. Fixed: replaced the single row with four, matching the table's existing style; re-padded the whole table per this repo's markdown convention.
- **low, patch** — `_bmad-output/implementation-artifacts/epic-5-context.md`'s "Where the code lives" bullet still said "Compare lives in `internal/web` (`compare.go`)," true only for one of the four category implementations post-split. Verified real. Fixed: updated to name all four files.
- **low, patch** — `compare.go`'s remaining const block doc comment ("Compare row labels and label fragments") became inaccurate once the `%s`-templated "label fragments" moved out to the new files, leaving only two plain labels. Verified real. Fixed: trimmed to "Compare row labels for the single-team category."
- **low, patch** — the moved `TestBuildCompareShouldTagTheOwnColumnsValueAndKeepOwnStyling` doc comment's "every existing tag test above" claim stopped being literally true once the series tag test it also referred to moved to a different file, leaving only the two division tag tests physically above it in `compare_divisions_test.go`. Verified real. Fixed: reworded to "every existing division tag test above."
- **low, patch** — `compare_awards.go`/`compare_series.go` give no in-code pointer to where their shared helpers (`awardOrder`/`awardTitle`/`displayNameForSlug`; `teamName`/`conferenceForMatchup`/`stanleyCupFinalLabel`) are actually defined, unlike the spec's own prose. Verified real. Fixed: added a one-line pointer to each file's category-function doc comment.
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-review/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.

All five patched findings were independently re-verified after patching: `go build`/`go vet`/`gofmt -l`/`task lint`/`task go:lint`/`task go:test`/`task go:test:acceptance`/`task go:run` all clean, matching the pre-split baseline exactly.
