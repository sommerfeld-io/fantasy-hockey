---
title: 'Hoist duplicated test helpers within each package and make the GoDog suite strict'
type: 'refactor'
created: '2026-09-25'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '0fd5f8de7420a85669e2b7af730afd26198732a4'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Epic 4 re-grew the test-helper duplication that Epic 3 item 22 had removed (Epic 4 retro A9, `epic-4-retro-item-34`).

- The two acceptance step files that seed results each build their own YAML writers, in different styles and with different map ordering.
- Four packages re-type the same "write a seed file, then `store.New`" sequence.
- The literal `2026-09-20T10:00:00Z` is scattered across packages.
- Epic 5's step files would add yet another copy.

Separately, the GoDog suite isn't strict: an undefined step shows as "undefined" but the suite still passes.

**Approach:** Give each duplicated helper one home inside its own package. Delete the copies, and set `Strict: true` on the GoDog suite. Test code only. Every test and scenario keeps passing unchanged.

**Decisions (human, 2026-09-25):**

- Hoist within each package only: no new shared or exported package, the same rule as Epic 3 item 22.
- No new acceptance test. The proof is the unchanged suite passing, now in strict mode.

## Boundaries & Constraints

**Always:**

- Only `*_test.go` files change.
- Every `.feature` file and step regex stays unchanged, as do each helper's existing error-message wording and each test's assertions.
- Generated seed YAML must stay semantically equal for every scenario. Write sections in sorted key order, so output is deterministic.
- Split the writers into small per-section functions so every function stays within `gocyclo -over 10`, and leave no unused helper behind (golangci `unused`).

**Never:**

- Change production code, create a new non-test package, or export anything.
- Rewrite the static seeds, or the inline team fixtures of the other step files (`cup_and_presidents_picks`, `playoffs_cup_pick`, `division_picks`, `series_predictions`, `round_unlocking`, `browse_prediction_sets`, `award_finalists`). They each render teams once, and Epic 3 item 22 left them alone.

</frozen-after-approval>

## Code Map

- **Acceptance tests:**
    - `src/acceptance-tests/fixture_support_test.go`: the new home for the shared seed-section writers. Its `seedHeader` (`:59`) and `writeSeededStore` (`:75`) are the pattern.
    - Two sets of writers merge into one set of section writers here:
        - `automatic_scoring_steps_test.go`: `writeTeams :334`, `writeNHLPlayers :341`, `writeMatchups :351`, `writeResults :376`, `writeSeriesResults :400`, `writeFinalists :417`, and `addPrediction :109` (the prediction-row renderer).
        - `leaderboard_steps_test.go`: the inline teams, `nhl_players`, `results` and prediction rendering in `seedBody :109`, plus `writeMatchups :137` and `writeRecordedResults :149`.
    - The merged writers cover teams (with a division per team), `nhl_players`, `playoff_matchups` for any set, prediction rows for any player, and results (`team_marks`, trophies, `series` for any round, `award_finalists`), all sorted.
    - Both state structs keep their own data. Only the rendering is shared.
    - Keep `automaticScoringRounds :37` as the single round-name map, and have leaderboard use it.
    - Merge `automaticScoringSubmittedAt :26` and `leaderboardSubmittedAt :20` into one const in `fixture_support_test.go`.
- **`src/acceptance-tests/suite_test.go:29`:** add `Strict: true` to `godog.Options`. The suite currently has 0 undefined or pending steps (130 scenarios, 821 steps), and godog v0.16.0 supports `Options.Strict`.
- **`src/internal/store/store_test.go`:** rename `newResultsStore :1988` to a general seed helper. Replace the inline write/`New`/`Fatalf` sequences at `:58, 217, 260, 293, 337, 368, 411, 448, 482, 509, 543, 575, 600, 1616` with it. `:79` expects an error, so it stays inline. `newTestStore :88` and `seedPrediction :863` do different jobs, so leave them. Give the `2026-09-20T10:00:00Z` literal at `:1956, 1961` one const.
- **`src/internal/scoring`:** split the write-and-open part of `newScoringStore` (`scoring_test.go:47`) into a helper that `max_test.go:70-80` also uses. Put the submitted_at literal (`scoring_test.go:41`) in one const.
- **`src/internal/web`:** add one write/`New`/`Fatalf` helper and use it from:
    - `web_test.go`: `newTestStore :30`, `newTestStoreWithPredictionSets :53`, `…AndMatchups :80` and `…AndTeams :106`;
    - `sheet_series_test.go:151`, `sheet_awards_test.go:79`, `sheet_divisions_test.go:48` and `leaderboard_test.go:43`.

  Keep the thin `X` / `XAndDir` wrappers. Put the literal at `leaderboard_test.go:38` in one const.
- **`src/internal/standings`:** has a single helper and nothing to merge. Only the `:53` literal gets a const.

## Tasks & Acceptance

**Execution:**

- [x] `src/acceptance-tests/fixture_support_test.go`, `automatic_scoring_steps_test.go` and `leaderboard_steps_test.go`: shared sorted section writers, one submitted_at const, and the per-file copies deleted.
- [x] `src/acceptance-tests/suite_test.go`: `Strict: true`.
- [x] `src/internal/store/store_test.go`: the general seed helper replaces the inline sequences, plus a submitted_at const.
- [x] `src/internal/scoring/scoring_test.go` and `max_test.go`: a shared seed-open helper, plus the const.
- [x] `src/internal/web/*_test.go`: one seed-write helper used by the listed constructors, plus the const.
- [x] `src/internal/standings/standings_test.go`: the const.

**Acceptance Criteria:**

- Given the refactor, when `task go:test` and `task go:test:acceptance` run, then every test and all 130 scenarios pass with no test or scenario added, removed or changed.
- Given strict mode, when a scenario uses a step with no definition, then the suite fails. Check this once with a temporary undefined step, then remove it.
- Given the two refactored step files, when a scenario is seeded twice, then the generated YAML is byte-identical both times, because the ordering is deterministic.

## Implementation Notes

- **Strict-mode check:** a temporary scenario with an undefined step, appended to `app-shell.feature`, made the suite fail (131 scenarios, 1 undefined, non-zero exit). The same scenario on the baseline commit passed. The scenario was then removed. Strict mode also fails the suite on pending steps.
- **Seeded YAML stays semantically equal:** old and new seeds for a full season plus a leaderboard fixture, compared as parsed YAML, differ only in the following ways, all of which the store reads identically:
    - teams and NHL players are written in sorted order;
    - an empty leaderboard `division_winner` and empty trophy winners are left out instead of written empty.
- **Unlisted change:** `pickedAllSeriesCorrectly` in `automatic_scoring_steps_test.go` now iterates rounds in sorted order instead of over a map, so its prediction rows and their ids (`p1`, `p2`, ...) are deterministic.
- **Review follow-ups:**
    - the round map moved to `fixture_support_test.go` as `seedRounds`, and leaderboard looks up round 1 via `mustSeedRound`, which panics on an unknown key;
    - `writeSeedTeams` derives each team's conference from `seedConferenceByDivision`, moved from `division_picks_steps_test.go` so it has one home;
    - the remaining inline seed-and-open sequences in `web/login_test.go`, `web/sheet_test.go` and `app_shell_steps_test.go` now use their package's helper.

## Spec Change Log

## Review Triage Log

| #  | Source      | Finding                                                                   | Verdict | Route  | Evidence                                                                                              |
|----|-------------|---------------------------------------------------------------------------|---------|--------|-------------------------------------------------------------------------------------------------------|
| 1  | blind, edge | Inline seed sequences left in `login_test.go`, `sheet_test.go` and `app_shell_steps_test.go` | low | patch | The intent is one home per helper. The Code Map missed these (verified at `login_test.go:105`, `app_shell_steps_test.go:53`). |
| 2  | blind, edge | Leaderboard reads `automaticScoringRounds["round 1"]` unchecked            | low     | patch  | A missing key silently gives empty ids. Moved to a neutral shared name with a loud lookup.            |
| 3  | blind       | Shared `writeSeedTeams` hard-codes `conference: Eastern`                   | low     | patch  | Wrong for Central and Pacific teams in a helper Epic 5 will reuse. Now derived from the division.     |
| 4  | blind       | The seed helper's second return value is a path in one package and a dir in another | low | patch | The results are now named in each signature.                                                      |
| 5  | blind       | Redundant `st, dir := …; return st, dir` wrappers                          | low     | patch  | A direct simplification.                                                                              |
| 6  | blind       | Strict-mode check, YAML-equality check and the sorted-rounds fix aren't recorded | low | patch  | Recorded in Implementation Notes.                                                                     |
| 7  | blind       | Seeded Leaderboard YAML changed shape (empty values omitted)               | false   | reject | `requireNoResultProblems` plus all Leaderboard scenarios passing prove the store reads it the same.   |
| 8  | blind       | No automated test of writer output or render-twice determinism            | low     | reject | These are test-only helpers, determinism was checked manually over three runs, and the scenarios exercise the output. |
| 9  | blind       | `seedSubmittedAt` const placed far from its users                          | low     | reject | Cosmetic.                                                                                             |
| 10 | edge        | A matchup gets `a == b` when a series is picked but not recorded           | low     | reject | Test-only and pre-existing. No scenario does this.                                                    |
| 11 | edge        | Flow-mapping writers don't quote feature-supplied scalars                  | low     | reject | Feature values are ids and slugs without YAML metacharacters.                                         |
| 12 | blind, vg   | `src/fantasy-hockey.yml` modified in the working tree                      | —       | —      | The user's own login record, made during a verification run. Left untouched and not committed.        |
| 13 | vg          | No verification gaps                                                      | —       | —      | The layer reported none.                                                                              |

## Verification

**Commands:**

- `task go:test` (expected: pass, with coverage unchanged)
- `task go:test:acceptance` (expected: 130 scenarios pass in strict mode)
- `task go:run` (expected: lint including `unused`, gocyclo and govulncheck pass, and the app starts)
