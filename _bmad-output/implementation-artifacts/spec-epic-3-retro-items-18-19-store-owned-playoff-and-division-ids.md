---
title: 'Store owns the series-key format, playoff round-set ids, and division vocabulary'
type: 'refactor'
created: '2026-09-24'
status: 'done'
baseline_commit: '847488b5346bd4671bd5172b50df44ec6873a271'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The persisted series-key format (`"r1.s1"`, written to `fantasy-hockey.yml` as `series_key`), the playoff round-set ids (`r1`/`r2`/`cf`/`scf`) and the division vocabulary (`Atlantic`/`Metropolitan`/`Central`/`Pacific`) live in `internal/web`. Story 4-1's `internal/scoring` needs all three, and it may not import `internal/web` (layering `web -> feature -> store`, AD-24). This is epic-3 retro item 19 and settles epic-2 retro item 18.

**Approach:** Move the three into `internal/store` as exported API. `internal/web` and the acceptance tests use the store's versions and no longer define their own. Behavior stays byte-for-byte the same.

**Decisions:** No new acceptance test (human decision, 2026-09-24): nothing observable changes, and the existing acceptance suite passing unchanged is the regression proof. The spec is kept at about 1,700 tokens without splitting.

## Boundaries & Constraints

**Always:** Keep the persisted key format exactly `<setID>.<matchupKey>`, so existing `series_key` rows still match. Keep the division order `Atlantic, Metropolitan, Central, Pacific`. Doc-comment every new exported identifier (revive `exported` is enforced). Test first: move or add the store tests before moving the code.

**Never:** Move policy that stays in the presentation layer: `roundGatedSetIDs`, `seriesSetIDs`, `pickableSheetKinds`, `effectiveUpcoming`, `stanleyCupFinalLabel`, `divisionsSetID`, `awardsSetID`. Don't create `internal/predictions` or `internal/scoring` (retro F2 stays an open question). Don't change conference handling, the YAML schema or any rendered output. Don't touch the acceptance-test helpers that item 22 will hoist (deferred work).

## I/O & Edge-Case Matrix

| Scenario            | Input / State                          | Expected Output / Behavior               | Error Handling          |
|---------------------|----------------------------------------|------------------------------------------|-------------------------|
| Join a series key   | `store.JoinSeriesKey("r1", "s1")`      | `"r1.s1"`                                | N/A                     |
| Round-trip          | `SplitSeriesKey(JoinSeriesKey(a, b))`  | `a, b, true`                             | N/A                     |
| Malformed key       | `store.SplitSeriesKey("malformed")`    | `ok == false`                            | Caller skips the row    |
| Division order      | `store.Divisions()`                    | `[Atlantic Metropolitan Central Pacific]` | N/A                    |
| Caller mutates list | Caller edits the returned slice        | Next `store.Divisions()` call is unchanged | Returns a fresh copy  |

</frozen-after-approval>

## Code Map

- `src/internal/web/sheet_series.go:13-34` -- `seriesKeySeparator`, `joinSeriesKey`, `splitSeriesKey`: move to store. Call sites at `:162`, `:318-319`, plus `sheet.go:129`.
- `src/internal/web/predict.go:21-30` -- `round2SetID`/`conferenceFinalsSetID`/`stanleyCupFinalSetID` constants: move to store. `roundGatedSetIDs` (`:32-40`) stays, built from the store constants.
- `src/internal/web/sheet.go:25-31` -- `round1SetID`: move to store. `seriesSetIDs` (`:38-43`) stays, built from the store constants.
- `src/internal/web/sheet.go:69-72` -- `teamDivisionOrder`: replace with `store.Divisions()`. Other uses: `sheet.go:91,114`, `sheet_divisions.go:243-245`, and comments at `sheet_divisions.go:20-21,240` and `sheet_series.go:197`.
- `src/internal/store/store.go:80-110` -- the existing `Kind*` constant block. Put the new constants and functions next to it, following the `KindSeries`/`SeriesKey` doc style.
- `src/internal/web/sheet_series_test.go:798-825` -- the join/split tests: move to `store_test.go`, rewritten against the exported names.
- `src/internal/web/{predict,sheet,sheet_series,sheet_divisions}_test.go` -- update references to the renamed identifiers only. Don't rewrite the assertions.
- `src/acceptance-tests/division_picks_steps_test.go:27-30` -- `divisionPicksDivisionOrder` literal that "mirrors" web: replace with `store.Divisions()` (the package already imports store).

## Tasks & Acceptance

**Execution:**

- [x] `src/internal/store/store_test.go` -- add `TestJoinSeriesKeyShould…`/`TestSplitSeriesKeyShould…` (moved) and `TestDivisionsShouldReturnTheFixedDisplayOrder` plus a "should not be mutated by a caller" counterpart -- red first.
- [x] `src/internal/store/store.go` -- add exported `Round1SetID`, `Round2SetID`, `ConferenceFinalsSetID`, `StanleyCupFinalSetID`, `JoinSeriesKey`, `SplitSeriesKey` (keep the separator unexported) and `Divisions()` -- green.
- [x] `src/internal/web/{sheet,predict,sheet_series,sheet_divisions}.go` -- delete the local definitions and use the store's, updating the comments that name the old identifiers.
- [x] `src/internal/web/*_test.go` -- switch references to the store identifiers and delete the moved join/split tests.
- [x] `src/acceptance-tests/division_picks_steps_test.go` -- use `store.Divisions()`.
- [x] `src/internal/store/README.md` -- one line noting that the store owns the series-key format, round-set ids and division vocabulary.

**Acceptance Criteria:**

- Given the change, when grepping `src/internal/web` for `seriesKeySeparator|joinSeriesKey|splitSeriesKey|teamDivisionOrder|round1SetID|round2SetID|conferenceFinalsSetID|stanleyCupFinalSetID`, then there are no definitions left, and no Go code outside `internal/store` hard-codes the four division names.
- Given an existing `fantasy-hockey.yml` with saved series picks, when the app renders or saves a series sheet, then the same rows are found and written as before (the existing web and acceptance tests pass unchanged apart from renames).

## Implementation Notes

- Implemented by a fresh subagent from this spec. Store tests were written first and failed to compile before the store code was added.
- New store API: `Round1SetID`, `Round2SetID`, `ConferenceFinalsSetID` and `StanleyCupFinalSetID`, `JoinSeriesKey`/`SplitSeriesKey` (the separator stays unexported) and `Divisions()` (returns a `slices.Clone` copy), all placed after the `Award*` constants in `store.go`.
- The reasons `r1` is outside `roundGatedSetIDs` but inside `seriesSetIDs` used to live on the deleted `round1SetID`/round-id constants. They now sit on those two maps.
- `parseDivisionsSubmission` calls `store.Divisions()` once and keeps the result in a local variable.
- Interpretation of AC1's "no hard-coded division names": it applies to production code and to division-order lists. The test fixtures that model data-file content (for example `divisionTeamsByDivision` and the team-to-division maps) still spell out the names as fixture data. This matches the Code Map's "don't rewrite the assertions" rule.
- The orchestrator reflowed two over-long comment lines after the move (`sheet_divisions.go` `groupDivisionsByConference`, `sheet_series.go` `stanleyCupFinalLabel`).

## Spec Change Log

## Review Triage Log

| #   | Source            | Finding                                                                         | Verdict | Route  | Evidence                                                                                                                                                                                                             |
|-----|-------------------|---------------------------------------------------------------------------------|---------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| B1  | blind-hunter      | `SplitSeriesKey` exported with no production caller                             | false   | reject | Exporting it for story 4-1's `internal/scoring` is the stated Intent. The function had no production caller before the move either.                                                                                  |
| B2  | blind-hunter      | Join/Split are not inverses when a key or set id contains `.`                   | low     | reject | Real, but the code is unchanged, just moved. Hand-maintained keys never contain `.` in practice, and a fix would add validation guards.                                                                            |
| B3  | blind-hunter      | Nothing pins the literal values of the round-set-id constants                   | false   | reject | Web tests seed literal `id: cf`/`r2` (`predict_test.go:140,174,249,347`) and the series acceptance steps seed literal `r1`/`scf`, so a typo in a constant fails those tests.                                        |
| B4  | blind-hunter      | Series acceptance steps hard-code `"r1."+key` and the `r1`/`scf` ids            | false   | reject | The acceptance tests are black-box. The literals pin the on-disk `series_key` format and the data-file ids, which is exactly what should catch format drift.                                                        |
| B5  | blind-hunter      | Store never validates `Team.Division` or `SaveDivisionPicks` keys               | medium  | defer  | Real: a hand-edit typo like `Metropolitian` silently drops teams from every division form. The behavior predates this change; it's in the same cross-cutting validation class already deferred for `playoff_matchups`. |
| B6  | blind-hunter      | Acceptance `seedBody` silently skips a division missing from its fixture maps   | low     | patch  | The loop now ranges over `store.Divisions()` (introduced by this change). A missing key seeds nothing and fails later with a confusing error. The fix is a comma-ok lookup and an error return.                     |
| B7  | blind-hunter      | `seriesSetIDs` and `roundGatedSetIDs` list the same three ids separately        | low     | reject | This duplication predates the change, and the spec's Never clause keeps both maps in web. A fix would restructure policy maps this change deliberately leaves alone.                                               |
| B8  | blind-hunter      | `store.go` comments still spell raw ids and the key shape                       | low     | patch  | Confirmed at `store.go:65,106,196`. A comment-only correction pointing at the new constants and `JoinSeriesKey`.                                                                                                     |
| B9  | blind-hunter      | `Divisions()` allocates a copy on every call                                    | low     | reject | Negligible: a 4-element clone on each render. Returning a copy is the spec's recorded design choice (Design Notes).                                                                                                  |
| B10 | blind-hunter      | Rename left ragged comment lines                                                | low     | patch  | Confirmed at `predict_test.go:202-204`, `sheet_test.go:899-901`, `sheet.go:69-72` and `sheet_series.go:119-121`. Reflow only.                                                                                          |
| B11 | blind-hunter      | README bullet cites AD-24 for constants and vocabulary                          | false   | reject | AD-24's rule text says "`internal/store` also exports typed constants for every cross-cutting enumeration".                                                                                                          |
| E1  | edge-case-hunter  | Acceptance seed drifts silently if a division is missing from the fixture       | low     | patch  | Same root cause as B6, and grouped with it.                                                                                                                                                                         |
| E2  | edge-case-hunter  | A team with an unknown Division is silently left out of the grouped forms       | medium  | defer  | Same root cause as B5, and grouped with it.                                                                                                                                                                         |
| E3  | edge-case-hunter  | Empty key, or a set id containing `.`, gives an ambiguous series key            | low     | reject | Same claim as B2. The code is unchanged and moved as-is; guarding it would add validation branches for input never seen.                                                                                          |
| E4  | edge-case-hunter  | Test fixtures still hard-code division names despite AC1                        | low     | reject | Fixtures model data-file content, and a rename there would fail loudly. The only fix is to reword this spec's AC, and fixes that edit this build's spec are rejected.                                              |
| V   | verification-gap  | No verification gaps                                                            | n/a     | n/a    | The reviewer reported no gaps.                                                                                                                                                                                         |

## Design Notes

`Divisions()` is a function, not an exported `var`. A shared slice could be mutated by any importer, and the magic-value rule prefers a getter whenever the value deserves a test. The round ids stay plain constants, like `KindCupChampion`. `SeriesKey` is also the name of the `Prediction` field, and `JoinSeriesKey`/`SplitSeriesKey` read naturally next to it.

## Verification

**Commands:**

- `task go:build` (from repo root) -- expected: lint, vet, unit and acceptance tests all pass.
- `task go:run` -- expected: the app starts, listening on :8080.
- `task docker:build` -- expected: the image builds.
