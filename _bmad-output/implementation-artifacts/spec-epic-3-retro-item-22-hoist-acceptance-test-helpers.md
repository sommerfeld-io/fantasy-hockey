---
title: 'Hoist duplicated acceptance-test helpers into fixture_support_test.go'
type: 'refactor'
created: '2026-09-24'
status: 'done'
baseline_commit: 'd0c4cdef7cc904e54dea15e132e420e1b2ed11fa'
route: 'dispatch'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Four acceptance step files carry identical copies of `setRowFragment`. Seven carry the same signed-in-player guard, under two names (`theSignedInPlayerIs`/`theFixturePlayerIs`). Two carry the same sample-team-name map. Epic 4 would add more copies (epic-3 retro item 22, which supersedes epic-2 item 12).

**Approach:** Give each helper one home in `src/acceptance-tests/fixture_support_test.go`, and delete the per-file copies. The shared player guard compares names exactly, which also settles epic-2 retro item 13. Every scenario keeps passing unchanged.

**Decisions (human, 2026-09-24):** The `seriesGroupFragment` pair (`acceptance-tests/series_predictions_steps_test.go:246` and `internal/web/sheet_series_test.go:247`) stays duplicated. The two copies are in different Go packages, and sharing them would need a new exported helper package, which isn't worth it for one small function. No new acceptance test, because the change is test-only and the existing suite passing unchanged is the proof.

## Boundaries & Constraints

**Always:** Keep every `.feature` file and step regex unchanged. Keep the existing error-message wording of each helper. Test code only: `src/acceptance-tests/*_test.go`.

**Never:** Change production code or `internal/web` tests. Don't create a new non-test package. Don't touch scenario-specific helpers that exist only once (`awardGroupFragment`, `divisionChipGroupFragment`, `divisionPicksTeamsByDivision`, `seriesPredictionsTeams`).

</frozen-after-approval>

## Code Map

- `src/acceptance-tests/fixture_support_test.go:110-122` -- `lazyFixture`, which already holds `playerName` and `lastBody`. Seven scenario states embed it (cup_and_presidents_picks, browse_prediction_sets, round_unlocking, playoffs_cup_pick, series_predictions, award_finalists, division_picks), so methods on `*lazyFixture` are promoted to every one of them. New helpers go in this file.
- `setRowFragment` copies (identical bodies): `cup_and_presidents_picks_steps_test.go:282-301`, `browse_prediction_sets_steps_test.go:126-145`, `round_unlocking_steps_test.go:137-156`, `playoffs_cup_pick_steps_test.go:281-300`. Becomes a `*lazyFixture` method, so call sites stay `s.setRowFragment(id)`.
- Signed-in guard copies, using `strings.ToLower(name) != <file>PlayerID`: `cup_and_presidents_picks:83`, `award_finalists:138`, `division_picks:110`, `series_predictions:113`, `round_unlocking:55`, `playoffs_cup_pick:75`, and `browse_prediction_sets:58` (named `theFixturePlayerIs`, registered at `:262`). Becomes `(*lazyFixture).theSignedInPlayerIs`, which compares `name` to `f.playerName` exactly. Update browse's `ctx.Step` registration to `s.theSignedInPlayerIs`.
- Exact-case guards without `lazyFixture`: `load_canonical_team_list:92` and `load_canonical_nhl_player_list:85`. Both compare the name and then issue a cookie. Move the name check onto a shared free function `requireSeededPlayer(name, want string) error`, which the `lazyFixture` method also uses. Each keeps its own cookie line.
- Team-name maps with identical content: `cupAndPresidentsTeamNames` (`cup_and_presidents_picks_steps_test.go:38`) and `playoffsCupPickTeamNames` (`playoffs_cup_pick_steps_test.go:29`), both `{TOR: Toronto Maple Leafs, VGK: Vegas Golden Knights}`. Becomes one `sampleTeamNames`.
- After the edits, drop any `strings`/`fmt` imports that are no longer used.

## Tasks & Acceptance

**Execution:**

- [x] `src/acceptance-tests/fixture_support_test.go` -- add `TestRequireSeededPlayerShouldAcceptTheExactName` and `...ShouldRejectADifferentCase` (plain Go tests in the package, next to the suite) -- red first, proves AC2.
- [x] `src/acceptance-tests/fixture_support_test.go` -- add `(*lazyFixture).setRowFragment`, `(*lazyFixture).theSignedInPlayerIs`, `requireSeededPlayer` and `sampleTeamNames`, each doc-commented -- one home per helper.
- [x] The 7 `lazyFixture` step files -- delete the local `setRowFragment`/`theSignedInPlayerIs`/`theFixturePlayerIs` copies and the two team-name maps, and point their users at the shared ones -- removes the duplicates.
- [x] `src/acceptance-tests/load_canonical_{team,nhl_player}_list_steps_test.go` -- use `requireSeededPlayer` for the name check.

**Acceptance Criteria:**

- Given the change, when grepping `src/acceptance-tests` for `func .*setRowFragment`, `func .*theSignedInPlayerIs|theFixturePlayerIs` and `"Toronto Maple Leafs"`, then each is defined exactly once, in `fixture_support_test.go`.
- Given a feature naming the seeded player in a different case (e.g. `"basti"`), when the step runs, then it fails with the existing "no fixture for player" error (exact-case, item 13), and `"Basti"` still passes.
- Given the unchanged `.feature` files, when `task go:test:acceptance` runs, then every scenario passes.

## Implementation Notes

- Implemented by a fresh subagent. The `requireSeededPlayer` tests were written first, failed to compile, and now pass. The full suite passes: 105 scenarios and 638 steps, with the `.feature` files unchanged.
- The shared helpers are methods on `*lazyFixture` (promoted to all 7 states) plus a free `requireSeededPlayer`, so the step-file call sites didn't need to change. Browse's registration now points at `s.theSignedInPlayerIs`.
- AC1's `"Toronto Maple Leafs"` grep has a second hit in `seriesPredictionsTeams` (`series_predictions_steps_test.go:49`). That's a struct list the Never clause keeps, not the duplicated name map, so the criterion is met for the map it targets.

## Spec Change Log

## Review Triage Log

| #   | Source           | Finding                                                                             | Verdict | Route  | Evidence                                                                                                                                                                                       |
|-----|------------------|-------------------------------------------------------------------------------------|---------|--------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| B1  | blind-hunter     | `app_shell_steps_test.go:86-88` still has its own "no fixture for player" check     | low     | patch  | Confirmed: an eighth copy that the Code Map missed. The fix is a direct swap to `requireSeededPlayer` that keeps the cookie line, and it adds no surface.                                     |
| B2  | blind-hunter     | `theCanonicalTeamListIncludes` and its fixture struct are still in two step files   | low     | defer  | Real, but this duplication predates the change and isn't one of the Intent's named helpers. Hoisting it needs a new shared fixture type.                                                   |
| B3  | blind-hunter     | No test proves the promoted method reads `playerName` rather than `playerID`        | false   | reject | Every lazyFixture feature says `"Basti"` while every `playerID` is `"basti"`. If the method read `playerID`, all 7 features would fail, and the suite passes.                            |
| B4  | blind-hunter     | Negative tests are thin (no `"Alice"`, `"BASTI"` or padded-name cases)              | low     | reject | `requireSeededPlayer` is a single `!=` check, so the different-case test already covers every non-equal input. More cases add no confidence.                                               |
| B5  | blind-hunter     | The new unit tests are hidden from `task go:test`                                   | false   | reject | The verification-gap reviewer confirmed they run under `task go:test:acceptance`, which `task go:build` runs (`src/taskfile.yml:84`).                                                        |
| B6  | blind-hunter     | The shared `setRowFragment` has no direct tests                                     | low     | reject | Moved unchanged, and still exercised by about 10 acceptance assertions across 4 features. Its template coupling predates this change.                                                        |
| B7  | blind-hunter     | `setRowFragment`'s error dumps the whole page                                       | low     | reject | The wording predates this change, and the spec's Always clause keeps each helper's error wording.                                                                                            |
| B8  | blind-hunter     | `sampleTeamNames` doc is vague and defers its rationale to another file             | low     | patch  | Confirmed. A comment-only correction.                                                                                                                                                         |
| B9  | blind-hunter     | The `theSignedInPlayerIs` doc says "every scenario", which is inaccurate            | low     | patch  | Confirmed: it's only promoted to lazyFixture-embedding states. A comment-only correction.                                                                                                     |
| B10 | blind-hunter     | The spec's claims (the seven count, missing grep proof, empty triage log)           | low     | reject | The fix would be an edit to this build's spec, and fixes that edit the spec are rejected. The triage log is written in this step.                                                            |
| E1  | edge-case-hunter | AC1's `"Toronto Maple Leafs"` grep returns two hits                                 | low     | reject | The second hit is `seriesPredictionsTeams`, which the Never clause keeps. The only fix is to reword the spec's AC (rejected), and Implementation Notes already records it.                  |
| V   | verification-gap | No verification gaps                                                                | n/a     | n/a    | The reviewer reported no gaps, with evidence.                                                                                                                                                  |

## Verification

**Commands:**

- `task go:build` (from repo root) -- expected: lint, vet, unit and acceptance tests all pass.
- `task go:run` -- expected: the app starts, listening on :8080.
