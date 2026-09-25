---
title: 'Automatic Scoring Engine'
type: 'feature'
created: '2026-09-25'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '27f68681ee70310a29bf1ddbf930c6a918e4a8e4'
context:
    - '{project-root}/_bmad-output/implementation-artifacts/epic-4-context.md'
    - '{project-root}/_bmad-output/specs/spec-fantasy-hockey/scoring-rules.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Players' picks from Epics 2–3 are saved, but nothing turns them into points. `fantasy-hockey.yml` has nowhere to record real-world results, and there is no code that applies the point table, so the Leaderboard (Story 4.2) has nothing to show.

**Approach:** Add a hand-maintained, read-only results section to `fantasy-hockey.yml` that `internal/store` loads and exposes through read methods. Add a new `internal/scoring` package that computes each player's Regular and Playoff points from the store on every call, applying `scoring-rules.md` exactly. Nothing is cached and nothing is persisted.

## Boundaries & Constraints

**Always:**

- Point values live in one place in `internal/scoring`, as named constants or a single table, so season-2 recalibration means editing only that place.
- Picks are matched to results by team abbreviation or NHL player slug only.
- Every call recomputes from the store. A missing result, an empty pick, or a half-recorded series scores 0 and never returns an error.
- Division rule, per division: a correct winner pick earns 15. Every other team in the player's playoff list for that division that made the playoffs earns 5. The correctly picked winner never earns the extra 5 (15 replaces the mark; it does not add to it).
- Award rule: 5 points for each picked slug found anywhere in that award's recorded finalist list, which can hold more than 3 entries when there is a tie. This applies equally to all 5 awards.
- The season-opening Cup pick and the Playoffs Cup pick are each compared against the same recorded Cup winner. The first goes to Regular and the second to Playoff.
- Series rule: the winner and the game count both matching earns the round's "exact" value. The winner alone earns the round's "winner" value. They never add up.
- `internal/scoring` imports only `internal/store`, never `internal/web`, and nothing else imports it yet.
- Decision, results shape (the architecture's shape): `results.team_marks.<lowercase division>: {playoffs: [abbreviations], division_winner: ABBR}`, `results.presidents_trophy`, `results.stanley_cup_winner`, and `results.series.<roundN>.<matchup key>: {winner, games}`. `round1` to `round4` map to `r1`, `r2`, `cf` and `scf`, and the matchup key is the existing `playoff_matchups` `key`. Finalists go in a top-level `award_finalists.<award>: [{slug, display_name}]`, and only `slug` is compared. Unquoted `games: 5` must load.
- Decision, malformed results: at startup the app logs one stderr warning for each bad entry: an unknown team abbreviation, finalist slug, division, award or round, a series key with no matching `playoff_matchups` entry, or games outside 4–7. The app still starts, and a bad entry scores 0.

**Never:**

- Write any score, or anything from the results section, to `fantasy-hockey.yml`.
- Build `internal/standings`, a Leaderboard view, or ranking. Those belong to Story 4.2.
- Add a UI for entering results.
- Add a tiebreaker.

## I/O & Edge-Case Matrix

| Scenario                    | Input / State                                                   | Expected Output              |
|-----------------------------|-----------------------------------------------------------------|------------------------------|
| No results recorded yet     | All picks saved, results section absent                         | Regular 0, Playoff 0         |
| Winner picked and listed    | Picked FLA in playoff list and as winner; FLA won the division  | 15 for FLA, not 20           |
| Winner wrong, team made it  | Picked TOR as winner; TOR made playoffs but FLA won             | 5 for TOR                    |
| Tie expands award finalists | Hart recorded with 4 slugs; the player's 3 picks all appear in them | 15                       |
| Exact series                | Picked FLA in 5; result FLA in 5 (round 1)                      | 25, not 40                   |
| Winner-only series          | Picked FLA in 6; result FLA in 5 (Conference Finals)            | 30                           |
| Empty pick                  | No award row for Norris                                         | 0 for Norris, no error       |
| Unknown player              | Player id not in players                                        | Regular 0, Playoff 0         |

</frozen-after-approval>

## Code Map

- `src/internal/store/store.go:276-285` (`document`): add `Results` (`yaml:"results"`) and `AwardFinalists map[string][]AwardFinalist` (`yaml:"award_finalists"`). Unmarshalling is lenient, so a file without either section still loads.
- `src/internal/store/store.go:248` (`AwardFinalist`): reuse it for recorded finalists. `position` is simply absent there.
- `src/internal/store/store.go:202-214` (`Prediction`): read-only. A series row keeps the winner in `TeamID` and the game count in `Games` as a string ("4"–"7"). An award row uses `Award` plus `FinalistSlugs`. A division row uses `Division` (capitalised, as in `Divisions()`) plus `TeamIDs` or `TeamID`. Cup, presidents and playoffcup rows use `TeamID`.
- `src/internal/store/store.go:81-173`: reuse the `Kind*`, `Award*` and round set-id constants, `JoinSeriesKey`/`SplitSeriesKey` and `Divisions()`. Don't redefine them in `scoring`. The store owns the series-key vocabulary, so the `round1`–`round4` ↔ `r1`/`r2`/`cf`/`scf` mapping and the lowercase-division ↔ `Divisions()` mapping live in `store` too.
- `src/internal/store/store.go`, new read methods, each returning copies under the read lock:
    - `Players() []Player`
    - `PredictionsForPlayer(playerID) []Prediction`
    - `DivisionResult(division) (playoffs []string, winner string)`, which takes the capitalised division name
    - `PresidentsTrophyWinner() string` and `StanleyCupWinner() string`
    - `SeriesResult(seriesKey) (winner, games string, ok bool)`, which takes the prediction-side `r1.s1` key
    - `RecordedAwardFinalists(award) []string` (slugs)
    - `ResultProblems() []string`, which lists the malformed entries described in the Always rules. `store` only reports them; it never logs.
- `src/main.go:72`: after `store.New`, log each of `st.ResultProblems()` with `slog.Warn`.- `src/internal/store/store_test.go:87,862`: reuse `newTestStore` and `seedPrediction`.
- New `src/internal/scoring/`:
    - `scoring.go`: `Points{Regular, Playoff int}` with `Total()`, and `PlayerPoints(st *store.Store, playerID string) Points`. The rules read only the store methods above.
    - One small unexported function per rule (awards, divisions, trophies, series).
    - A `README.md`.
- `src/acceptance-tests/fixture_support_test.go:75,90`: reuse `writeSeededStore` and `newSeededStore` for YAML-seeded scenarios. Scoring has no HTTP surface, so the steps call `scoring.PlayerPoints` directly, the same way existing steps call store methods.
- `src/acceptance-tests/suite_test.go`: register `InitializeAutomaticScoringScenario`.
- `src/internal/store/README.md`: document the results section and the new read methods.

## Tasks & Acceptance

**Execution:**

- [x] `src/acceptance-tests/features/automatic-scoring.feature` and `src/acceptance-tests/automatic_scoring_steps_test.go`: write the scenarios from the matrix and the Always rules using seeded YAML, register them in `suite_test.go`, and confirm they fail (red) first.
- [x] `src/internal/store/store_test.go` and `store.go`: add the results and award-finalist sections and the read methods, test-first. Cover a file with neither section, the lowercase-division and `round3`→`cf` mappings, unquoted `games: 5` loading as `"5"`, returned copies not aliasing internal state, and `ResultProblems` for each malformed kind plus its counterpart (a clean file reports nothing).
- [x] `src/main_test.go` and `src/main.go`: a malformed result is logged as a warning and startup still succeeds.
- [x] `src/internal/scoring/scoring_test.go` and `scoring.go`: write table-driven tests for every matrix row and every point-table row, each with its "should not" counterpart (for example, a correct division winner is not also given the 5), and then implement.
- [x] `src/internal/scoring/README.md` and `src/internal/store/README.md`: document the package's purpose and the results section.

**Acceptance Criteria:**

- Given a seeded data file with picks and results, when scoring runs twice with an edit to the results in between, then the second call reflects the edit and the data file's bytes are unchanged by scoring.
- Given a player with every pick correct and every result recorded, when scoring runs, then Regular and Playoff equal the maximum totals the point table allows.
- Given `internal/scoring`, when its imports are listed, then it imports `internal/store` and no other `internal/` package.

### Review Findings

Code review of `bb9a7e4` (2026-09-25). Layers: Blind Hunter, Edge Case Hunter, Verification Gap and Acceptance Auditor.

- [x] [Review][Defer] Results and award_finalists are re-serialized on every prediction save. Comments and any unmodelled key (such as a misspelled `stanley_cup_winer`) are erased from the file, and unquoted `games: 5` comes back quoted. The frozen Never rule says nothing from the results section is written. Every other hand-maintained section (players, teams, playoff_matchups) already behaves this way. Deferred (user decision): the Never rule means the app never creates or changes results. Preserving comments and unknown keys belongs to story 7-3.
- [x] [Review][Decision] A division-winner pick earns 5 only if that team is also in the player's own playoff list. The matrix row "Picked TOR as winner; TOR made playoffs but FLA won → 5 for TOR" can be read either way, and the web form doesn't require the winner to be listed. Resolved (user decision): keep the current rule. It scores 0, and no change is needed.
- [x] [Review][Patch] Say that a hand edit to results only takes effect after a restart and is overwritten if saved while the app runs. Retitle the "Scoring is recomputed from the latest results" scenario to reflect the restart [src/internal/store/README.md, src/acceptance-tests/features/automatic-scoring.feature:92]
- [x] [Review][Patch] Only the result's game count is normalized, so a hand-edited pick like `games: "05"` loses the exact value. Compare both sides normalized [src/internal/scoring/scoring.go]
- [x] [Review][Patch] A capitalised `team_marks` key (`Atlantic:`) is reported as "unknown division" with no hint that keys are lowercase. Name the valid keys in the message [src/internal/store/store.go]
- [x] [Review][Patch] `pickedEverythingCorrectly` and `scoringRunsBeforeAndAfterACupEdit` discard step errors with `_ =` [src/acceptance-tests/automatic_scoring_steps_test.go:241]
- [x] [Review][Patch] `TestOpenStoreShouldNotWarnForAWellFormedFile` passes a path that doesn't exist, so it never loads a well-formed results section. Also, the malformed test doesn't check that there is one warning per problem [src/main_test.go:136]
- [x] [Review][Patch] Outdated `resultsFixtureBase` comment ("three Atlantic teams" when EDM/Pacific is also present). The unknown-round message repeats the round name instead of listing round1–round4 [src/internal/store/store_test.go:1925, src/internal/store/store.go:1107]
- [x] [Review][Patch] The `openStore` doc claims "a bad result never stops startup", but a wrongly shaped entry does stop it [src/main.go:63]
- [x] [Review][Defer] Nothing tests that `run()` goes through `openStore`, so the startup warning could silently disappear [src/main.go:86]. Deferred: `run()` has no test seam, and it's already in deferred-work from the Build review.
- [x] [Review][Defer] A wrongly shaped results entry makes the app refuse to start instead of warning [src/internal/store/store.go]. Deferred: the user chose on 2026-09-25 to handle this in story 7-3.
- [x] [Review][Defer] A misspelled results key is silently ignored with no warning [src/internal/store/store.go]. Deferred: the user chose on 2026-09-25 to handle this in story 7-3. Its removal from the file on the next save is part of the first decision above.

#### Rejected

- Duplicate prediction rows of one kind score twice (blind and edge): low. Only a hand edit of the app-written section can create one, since saves upsert. The fix would add dedupe logic.
- A team listed twice in the recorded playoffs list: false. Lookups use `slices.Contains`, so it can't score twice.
- A recorded playoffs list longer than a division's slots: low. Rare, and it would need a new check.
- division_winner missing from its own playoffs list: false. Treating the recorded winner as a playoff team is deliberate.
- games recorded without winner (or the reverse) not reported: false. A half-recorded series scoring 0 is the specified behavior.
- Acceptance fixture builds `a == b` matchups, or drops a third team (blind and edge): low. Test-only, and all current scenarios have at most two teams per series.
- Seed YAML written in map order: low. Test-only, and the fix is more than a direct correction.
- Missing acceptance scenarios for R2, SCF, Presidents' Trophy and startup warning values: low. The maximum-season scenario and unit tests cover them.
- `AwardFinalist.Position` omitempty changes nhl_players output: false. Every nhl_players entry has a position.
- No consistent store snapshot in `PlayerPoints`: low. The pool is small. Revisit in 4-2 if it matters.
- Duplicate `playoff_matchups` keys in one set: low. That shows up loudly as a "winner not in matchup" warning.
- `PlayerPoints(nil)` panics: false. That's a programmer error, and failing loudly is correct.
- Result-problem steps with no earlier scoring step: false. Every scenario runs "scoring runs" first.
- Feature lists written with spaces after commas: false. No current scenario does that.
- Implementation adds rules the spec doesn't list: rejected, because the fix would be to edit the spec under review. These rules were approved in the Build review triage (#4, #5).

## Implementation Notes

## Spec Change Log

## Review Triage Log

| #  | Source         | Finding                                                                 | Verdict     | Route  | Evidence                                                                                                    |
|----|----------------|-------------------------------------------------------------------------|-------------|--------|-------------------------------------------------------------------------------------------------------------|
| 1  | blind, edge, vg | `games: 05` passes Atoi validation but string compare denies exact      | low         | patch  | `validSeriesGames` uses Atoi, and `seriesPickPoints` compares `p.Games == games`. A direct fix: normalize in `SeriesResult`. |
| 2  | blind, vg      | "round-trips unchanged through a save" wording is false                 | low         | patch  | `writeLocked` re-marshals the whole doc. Values survive, formatting and comments don't. Fixed the wording.   |
| 3  | blind, edge    | Hand edits made while the app runs are clobbered by the next save       | medium      | defer  | Pre-existing for every hand-maintained section (AD-27: stop the app before editing). Epic 3 retro item 21 runbook covers it. |
| 4  | blind, edge, vg | Series winner outside the matchup's a/b is not reported                 | medium      | patch  | `seriesOutcomeProblemsLocked` checks only a known team. A plausible typo silently gives 0 to everyone.       |
| 5  | blind, edge    | Team recorded under the wrong division is not reported                  | medium      | patch  | `teamMarkProblemsLocked` never compares `Team.Division`. The player silently loses 5.                        |
| 6  | blind          | division_winner not in the playoffs list is not reported                | false       | reject | Scoring deliberately counts a recorded winner as a playoff team, so there is no bad outcome.                 |
| 7  | blind          | Duplicate slugs or teams in recorded lists                              | false       | reject | Lookups use `slices.Contains`, so a duplicate can't score twice.                                             |
| 8  | blind          | games without winner treated as in progress                             | false       | reject | A half-recorded series scoring 0 is the specified behavior.                                                  |
| 9  | blind, edge    | Duplicate prediction rows of one kind sum                               | low         | reject | Only a hand edit of the app-written section can create one, since saves upsert. The fix would add dedupe logic. |
| 10 | blind          | Recalibration claim vs tests that hard-code values                      | false       | reject | Tests are expected to change with a recalibration. Production values live in one block.                      |
| 11 | blind          | Tautological `235` check in max_test                                    | low         | patch  | Compares a constant expression to a literal. Deleted.                                                         |
| 12 | blind, vg      | Empty-pick rows and award de-dup untested                               | medium      | patch  | Removing `pick == ""` makes an empty cup row score 20 against an unrecorded winner, and no test fails.       |
| 13 | blind          | Position omitempty changes nhl_players bytes                            | false       | reject | Every nhl_players entry has a position (it's needed for eligibility), so the output is unchanged.            |
| 14 | blind          | Fixture YAML nondeterministic, a==b matchups                            | low         | reject | Test-only. The a==b matchups still satisfy winner-in-matchup. Rare to hit.                                    |
| 15 | blind          | No consistent snapshot, O(players×picks) lock round trips               | low         | reject | The pool has a handful of players. Revisit if 4.2 shows a cost.                                               |
| 16 | blind          | Acceptance test runs below the user boundary                            | low         | reject | Scoring has no page until 4.2. The 4.2 Leaderboard scenarios will cover it over HTTP.                         |
| 17 | blind          | Scenario title misattributes the 5 points                               | low         | patch  | The 5 comes from the listed team, not the winner pick. Retitled.                                              |
| 18 | blind          | README rows omit edge wording                                           | low         | reject | Cosmetic. The rules are covered by tests.                                                                     |
| 19 | blind          | imports_test ignores the Unquote error                                  | low         | patch  | A direct correction.                                                                                          |
| 20 | vg             | `run()` wiring to openStore is unverified                               | medium      | defer  | Filed as defer. `run()` has no test seam.                                                                     |
| 21 | edge           | Wrongly shaped results entry aborts startup                             | medium      | defer  | Probe confirmed "cannot unmarshal !!str into []string". The user chose to defer to story 7-3 (2026-09-25).   |
| 22 | edge           | Misspelled results key silently ignored                                 | medium      | defer  | Probe confirmed `stanley_cup_winer` loads with no error or warning. The user chose to defer to story 7-3.    |
| 23 | edge           | nil *store.Store panics                                                 | false       | reject | Programmer error, and failing loudly is correct. No caller passes nil.                                        |
| 24 | edge           | Problem steps with no prior scoring step panic                          | false       | reject | Every scenario that uses those steps runs "scoring runs" first.                                               |

## Verification

**Commands:**

- `task go:test` (expected: pass, and `internal/scoring` coverage is at least as high as the other packages')
- `task go:test:acceptance` (expected: pass, including automatic-scoring.feature)
- `task go:run` (expected: builds and starts; a failure that comes only from govulncheck findings is acceptable)
