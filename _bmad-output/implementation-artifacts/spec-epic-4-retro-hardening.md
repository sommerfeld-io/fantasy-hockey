---
title: 'Epic 4 retro hardening: results runbook, shape wording, cross-story tests, Epic 2 test fixes'
type: 'chore'
created: '2026-09-25'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '5f5cf4aec7e63b7d33fb3e22a550a67a00fc45ab'
context:
    - '{project-root}/_bmad-output/implementation-artifacts/epic-4-retro-2026-09-25.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Epic 4 retro left several gaps that make hand-recording results risky before the playoffs:

- There is no operator guide, and Epic 3 item 21 is still open (A4).
- The approved results shape is documented as a flat `round1.s1` key that doesn't load (A1).
- Three cross-story behaviors have no tests: results surviving a save (A5), division, award and series points reaching the Leaderboard (A6), and the two round lists staying in sync (A7).
- Three small Epic 2 test and comment debts remain (items 11, 15 and 16).

**Approach:** Write a single operator runbook in `docs/`, correct the shape wording to the nested form the code accepts, add the three missing tests, and clear the Epic 2 items. No production behavior changes.

**Decisions (human, 2026-09-25):**

- Keep only the nested `results.series.roundN.<key>` form.
- A9 (hoisting test helpers) is split out to `deferred-work.md`.
- A6's Leaderboard scenario is the only acceptance-test addition, since nothing observable changes.

## Boundaries & Constraints

**Always:**

- Production Go code is unchanged except for doc comments (item 11). All tests keep passing.
- The runbook describes only what the code does today:
    - stop the app before editing `fantasy-hockey.yml`, and restart afterwards;
    - the exact nested `results:` and `award_finalists:` shape, with a commented example;
    - the `playoff_matchups` shape, with a unique `key` per series;
    - which `prediction_sets` `upcoming` flags to flip and when: `r1` and `playoffcup` by hand, while `r2`, `cf` and `scf` unlock once their matchups are recorded;
    - how to read the startup "malformed result" warnings, and which mistakes aren't warned about (a misspelled key is ignored, a wrongly shaped entry stops startup);
    - that the app rewrites the whole file on every save, dropping comments.
- The runbook satisfies the repo lints:
    - kebab-case filename;
    - `docs` allowed in `.folderslintrc`;
    - every link resolves (lychee);
    - `.markdownlint.yml` style: fenced code, `-` lists, 4-space nesting, padded tables.
- A1 corrects only wording, and states the nested form. That applies to the spec-4-1 "Decision, results shape" bullet (inside the frozen block, corrected per the human's 2026-09-25 decision) and to ARCHITECTURE-SPINE's illustrative `series:` lines and its `round1.s1` mention at ~L157.

**Never:**

- Change scoring, store, standings or web behavior.
- Export `store.resultRounds`.
- Hoist test helpers (A9).
- Change a `.feature` step regex another file relies on.
- Touch `src/fantasy-hockey.yml`.

</frozen-after-approval>

## Code Map

- **Runbook:** new `docs/recording-results-and-playoffs.md`.
    - Link it from `README.md`.
    - Add `docs` to `.folderslintrc`.
    - Sources to describe from: `src/internal/store/README.md` "Results (read-only)", `store.go:255-273` (PlayoffMatchup), `src/internal/web/predict.go:21-31` (effectiveUpcoming: `r1` uses its flag, `r2`/`cf`/`scf` are gated by matchups), and `src/fantasy-hockey.yml:48-96` (`upcoming:` flags).
- **A1:**
    - `_bmad-output/implementation-artifacts/spec-4-1-automatic-scoring-engine.md:34`: drop "(the architecture's shape)" and show the nested `series: roundN: <key>: {winner, games}` form explicitly.
    - `_bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md:364-365` and `~157`: switch to the nested form.
- **A5:** `src/internal/store/store_test.go:2282`, `TestSavingAPredictionShouldPreserveTheHandRecordedResults`.
    - The fixture `resultsFixtureBase + resultsFixtureResults` is at `:1966-1999`.
    - After the reopen, also assert:
        - `DivisionResult("Atlantic")` = `[FLA TOR]`, `FLA`;
        - `PresidentsTrophyWinner()` = `TOR`;
        - `SeriesResult(JoinSeriesKey(Round1SetID,"s1"))` = `FLA`, `"5"`, `true`, and the `cf.s1` result. Check that the base has matching `playoff_matchups`.
    - Read `os.ReadFile(path)` and assert that neither `position: ""` nor `display_name: ""` appears. The neighbouring test at `:2303-2311` already reads the file.
- **A6:** `src/acceptance-tests/features/leaderboard.feature` and `leaderboard_steps_test.go` (`seedBody` at `:74-95`, steps at `:278-289`).
    - Add one scenario in which the players' division, award and series picks meet recorded `team_marks`, `award_finalists` and `results.series`. Assert the rendered Regular and Playoff cells, with series points in Playoff.
    - This needs `nhl_players`, `playoff_matchups.r1` and row kinds for division, award and series in `seedBody`.
    - New step regexes keep the `"<name>" picked …` prefix or a leaderboard suffix, because every step file shares one ScenarioContext (`suite_test.go:36-53`). Avoid colliding with `automatic_scoring_steps_test.go:550-569`.
- **A7:** `src/internal/scoring/scoring_test.go`. Add a test that loops over every key of `seriesPoints` (`scoring.go:34-39`), seeds that round's `roundN` result (the round-name map lives in the test, since `store.resultRounds` at `store.go:890` stays unexported), and asserts non-zero points. Add a `store_test.go` counterpart asserting each round set id round-trips through `SeriesResult`.
- **Item 11:**
    - `src/internal/web/options.go:17-21`: the "2.4/2.6 embed it" promise never happened, and `newTeamOptions` has no production caller. State that.
    - `options.go:30-34`: `newAwardFinalistOptions` is now used by `sheet_awards.go:266`.
    - `src/acceptance-tests/load_canonical_team_list_steps_test.go:30-31`: `Store.Teams()` is now read by `sheet.go:210`, `sheet_divisions.go:143` and `roster.go:51`.
- **Item 15:** `src/internal/web/predict_test.go:129`. Scope the check to `<span class="set-row-countdown">&nbsp;&middot; closed</span>` (`shell.html:35`).
- **Item 16:** new test in `predict_test.go`. A Closed set with a saved submission renders `set-row set-row--submitted` together with the `status-pill--closed` "Closed" pill (`predict.go:104-111,133-143`). This is deliberate, per `epic-2-retro-2026-09-18.md:42`.

## Tasks & Acceptance

**Execution:**

- [x] `docs/recording-results-and-playoffs.md`, `README.md` and `.folderslintrc`: the runbook, the README link and the folder allowance.
- [x] `spec-4-1-automatic-scoring-engine.md` and `ARCHITECTURE-SPINE.md`: the A1 wording to the nested form.
- [x] `src/internal/store/store_test.go`: A5 assertions, plus the A7 store counterpart.
- [x] `src/internal/scoring/scoring_test.go`: the A7 round-coverage test.
- [x] `src/acceptance-tests/features/leaderboard.feature` and `leaderboard_steps_test.go`: the A6 scenario (red first: it must fail if series points are routed into Regular).
- [x] `src/internal/web/options.go` and `src/acceptance-tests/load_canonical_team_list_steps_test.go`: item 11 comments.
- [x] `src/internal/web/predict_test.go`: the item 15 scoped assertion and the item 16 regression test.

**Acceptance Criteria:**

- Given the runbook's example `results:` block, when it is pasted into a scratch copy of the data file and the app starts, then it loads with no warnings. Verify by loading it through `store.New` in a test or by a manual run.
- Given the new tests, when each guarded behavior is broken temporarily, then that test fails. That means: dropping `team_marks` from a save for A5, swapping the Playoff and Regular series routing for A6, removing a `seriesPoints` entry for A7, and making the Closed row use a closed accent for item 16.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| #  | Source      | Finding                                                                  | Verdict | Route  | Evidence                                                                                              |
|----|-------------|--------------------------------------------------------------------------|---------|--------|-------------------------------------------------------------------------------------------------------|
| 1  | edge        | Runbook series example warns on a fresh file (no `r1` `s1` matchup yet)  | medium  | patch  | `seriesOutcomeProblemsLocked` reports a missing matchup, so the example contradicts the AC unless it says to record the matchup first. |
| 2  | self        | Runbook says "no warnings means every value accepted"                     | medium  | patch  | Contradicts its own list of mistakes that get no warning.                                              |
| 3  | blind       | Runbook doesn't show how to add an `nhl_players` entry                    | medium  | patch  | A finalist missing from the list can't be recorded, and a guessed slug scores 0.                       |
| 4  | blind       | No backup or recovery guidance                                           | low     | patch  | A direct doc addition. A bad shape stops startup.                                                      |
| 5  | blind       | Runbook never says what results are worth                                | low     | patch  | Link `scoring-rules.md` (a direct doc addition).                                                       |
| 6  | blind       | The negative round-mapping store test passes for the wrong reason        | low     | patch  | `resultsFixtureBase` has no `r2` matchup, so the test holds regardless of the mapping (verified at `store_test.go:2368`). |
| 7  | blind       | `strings.Replace` on the fixture silently no-ops if the fixture drifts    | low     | patch  | A direct guard.                                                                                        |
| 8  | blind       | `embedOption` comment still lists `newTeamOptions` as an embedding site   | low     | patch  | Leftover from item 11 (`options.go:7-11`).                                                             |
| 9  | blind       | Teams comment claims other scenarios assert teams, without evidence      | low     | patch  | Overclaims (`load_canonical_team_list_steps_test.go:31-32`).                                           |
| 10 | blind, edge | Per-round scoring test only checks non-zero                              | false   | reject | `TestPlayerPointsShouldScoreEachRule` already pins exact and winner values for all four rounds (verification-gap confirmed). |
| 11 | blind       | `resultRoundNames` copies aren't tied to `store.resultRounds`             | false   | reject | Both tests go through `store.SeriesResult`, so a store-side rename fails them. Exporting is out of scope. |
| 12 | blind       | `newTeamOptions` is dead code and should be deleted                      | low     | reject | Item 11 is about the comments only. Deleting it is beyond scope.                                       |
| 13 | blind       | Leaderboard scenario lacks "should not" rows for the new kinds            | low     | reject | Unit tests cover wrong picks. The scenario targets column routing.                                     |
| 14 | blind       | Whole-string HTML assertions in `predict_test.go` are brittle             | low     | reject | Matches the file's existing assertion style.                                                           |
| 15 | edge        | Fixture helpers accept case-variant divisions, duplicate or orphan series | low     | reject | Test-only, and no current scenario does this.                                                          |
| 16 | vg          | No verification gaps                                                     | —       | —      | The layer reported none.                                                                               |

## Verification

**Commands:**

- `task go:test` (expected: pass)
- `task go:test:acceptance` (expected: pass, including the new Leaderboard scenario)
- `task lint` (expected: pass, including the folders, filenames and markdown-links checks for `docs/`)
- `task go:run` (expected: builds and starts)
