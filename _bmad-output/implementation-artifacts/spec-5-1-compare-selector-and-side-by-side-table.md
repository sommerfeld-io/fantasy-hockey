---
title: 'Story 5.1: Compare selector and side-by-side table'
type: 'feature'
created: '2026-09-25'
status: 'done'
baseline_commit: '4194fe970ecb9a36a7c39734f0f31303b6283974'
route: 'dispatch'
review_loop_iteration: 0
context:
    - '{project-root}/_bmad-output/implementation-artifacts/epic-5-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Compare tab is still a "Player comparison is coming soon." placeholder, so no player can see anyone else's picks (PRD FR-25, FR-26).

**Approach:** Replace the placeholder with an "Everyone's picks." section: a chip selector grouped into "Before the season" and "Playoffs" rows, the chosen set's deadline, and a server-rendered table with one bold-named column per player (the signed-in player's column distinguished) and one labelled row per prediction category of that set.

**Decisions (human, 2026-09-25):**

- **Default selection:** on first entry (no `?set=`), the earliest "Before the season" set is pre-selected.
- **Boundary with 5.2:** 5.1 builds every set's final row structure, including one playoff-teams row and one winner row per division, in `store.Divisions()` order, with values shown as plain text and an unfilled value as a plain "—". 5.2 adds the tag and faint styling and the gated-round note.
- **Selector contents:** a set whose `effectiveUpcoming` is true gets no chip. A gated round's chip appears once it unlocks.
- **Switching:** each chip is an `<a>` to `/compare?set=<id>`, rendered on the server. There is no JS.

## Boundaries & Constraints

**Always:** Every player's picks are visible to every player at any time, including before a set's deadline (PRD addendum). Columns follow `st.Players()` order. Values are stable ids mapped to display labels only at render time (team → `Team.Name`/abbreviation, NHL Player slug → display name, falling back to the raw id). The page is recomputed on every request. Chips wrap (`flex-wrap`) so nothing scrolls horizontally. Sentence case; tap targets ≥32px; the own column is marked by more than colour alone.

**Never:** No writes to the store. No import of `internal/scoring` or `internal/standings`. No new client-side JS. No new route (reuse `GET /compare`, already in `shellRoutes`). No new store methods unless a read is genuinely missing.

## I/O & Edge-Case Matrix

| Scenario            | Input / State                            | Expected Output / Behavior                                               | Error Handling          |
|---------------------|------------------------------------------|--------------------------------------------------------------------------|-------------------------|
| Pick a set          | chip for `presidents` chosen             | that set's deadline and its category rows; its chip shown selected       | N/A                     |
| Nobody picked       | player has no row for a category         | cell shows "—", never blank                                              | N/A                     |
| Other player's pick | Sadl's cup pick, deadline not yet passed | visible in Sadl's column to Basti                                        | N/A                     |
| Unknown set id      | `?set=nope`                              | same as the default selection                                            | none                    |
| Upcoming set id     | `?set=r2` while r2 is gated              | same as the default selection (5.2 replaces this with its gated note)    | none                    |
| Bad deadline        | set's `deadline_utc` unparseable         | set left out of the chips                                                | logged with `slog.Error` |
| Stale player        | session id has no player                 | page renders, no column is marked as own                                 | logged (existing shell) |

</frozen-after-approval>

## Code Map

- `src/internal/web/shell.go` -- `handleShell`, `shellData`, `tabMessages` (remove Compare's entry; add `Compare *compareView`, built only on `tabCompare`, as Predict/Leaderboard do).
- `src/internal/web/leaderboard.go` -- pattern to mirror: small view-model file with a `build…(st)` function.
- `src/internal/web/predict.go` -- `phaseBeforeSeason`/`phasePlayoffs`, `effectiveUpcoming`, `clock.FormatDeadline`; reuse for grouping, gating and deadline text.
- `src/internal/web/sheet.go` -- `divisionsSetID`, `awardsSetID`, `seriesSetIDs`: the dispatch keys for row building.
- `src/internal/web/sheet_awards.go` -- `awardOrder`, `awardTitle` (row labels), `displayNameForSlug`.
- `src/internal/web/sheet_series.go` -- `teamName`, `conferenceForMatchup`, `stanleyCupFinalLabel`; series row label "<Conference> · A vs B".
- `src/internal/web/roster.go` -- `teamRoster(st)` for id → team lookups.
- `src/internal/store/store.go` -- `Players`, `PredictionSets`, `FindPrediction`, `FindDivisionPlayoffTeams`/`FindDivisionWinner`, `FindAwardFinalists`, `FindSeriesPick`, `PlayoffMatchups`, `JoinSeriesKey`, `Divisions`.
- `src/internal/web/templates/shell.html`, `static/styles.css` -- add the compare section; chip look = existing `.chip` + checked state via a `--selected` modifier.
- `src/acceptance-tests/app_shell_steps_test.go:153`, `src/internal/web/shell_test.go:37` -- currently assert the Compare placeholder; must change.

## Tasks & Acceptance

**Execution:**

- [x] `src/acceptance-tests/features/compare-predictions.feature` + `compare_predictions_steps_test.go` -- scenarios for both ACs and the matrix (red first) -- BDD gate.
- [x] `src/internal/web/compare_test.go` -- unit tests for selection, grouping, column order, own-column flag, row labels per set kind, and "—" cells, plus their negatives -- TDD.
- [x] `src/internal/web/compare.go` -- `compareView`, `buildCompare(st, playerID, selectedID, now)` -- view model; the template computes nothing.
- [x] `src/internal/web/shell.go` -- read `r.URL.Query().Get("set")` on Compare, drop the placeholder -- wiring.
- [x] `src/internal/web/templates/shell.html`, `static/styles.css` -- selector rows, deadline line, `<table>` with player header and category rows -- rendering.
- [x] `src/acceptance-tests/app_shell_steps_test.go`, `features/app-shell.feature`, `src/internal/web/shell_test.go` -- replace the placeholder assertion with an "Everyone's picks." smoke check -- keep existing suites green.
- [x] `src/internal/web/README.md` -- the `/compare` row and the file table -- docs.

**Acceptance Criteria:**

- Given I open Compare, when the screen renders, then the section header reads "Everyone's picks." and the chips are grouped into labelled "Before the season" and "Playoffs" rows, with every selectable set present as a chip.
- Given I select a set's chip, when the table renders, then it shows that set's deadline, one column per player headed by their bold name, my column distinguished and no other, and one labelled row per prediction category of that set.

## Design Notes

Row labels per kind: cup/playoffcup "Stanley Cup winner"; presidents "Presidents' Trophy"; awards `awardTitle[a]` in `awardOrder`; rounds one row per `PlayoffMatchups(set.ID)` entry; divisions "<Division> — playoff teams" then "<Division> — winner" per division (click-dummy wording). Compare lives in `internal/web` (as Leaderboard does), since the architecture's `internal/predictions` package was never created. Plain-value rendering: full team name for cup/presidents/playoffcup, abbreviations for division/series picks, finalist display names one per line, series "FLA in 5". A `<table>` handles any player count without inline styles: a `<thead>` of names, then per category a label row (`<th colspan>`) above a cell row.

## Verification

**Commands:**

- `task go:build` (from repo root) -- expected: lint, vet, unit and acceptance tests pass.
- `task go:run` -- expected: app builds and starts; `/compare` renders the table.

## Implementation Notes

- "Earliest" default is read as earliest `deadline_utc` among selectable "Before the season" sets, ties broken by file order (the live data has all four on one deadline, so this is the first listed). If none is selectable, the first selectable set is shown; if nothing is selectable, only the two labelled (empty) selector rows render, each with a faint "Nothing to compare yet." note.
- A set whose phase is neither `before_season` nor `playoffs` is logged and left out of the chips, as Predict does.
- Series row labels use abbreviations and the bare conference ("Eastern · FLA vs TOR"); the Final uses "Stanley Cup Final · FLA vs COL"; an unmatched TeamA drops the conference prefix.
- The own column carries `cmp-player--own`/`cmp-cell--own` (ice left/right border) plus a "You" text label under the name.
- The `tabMessages`/`shellData.Message` placeholder mechanism and its `.coming-soon` CSS were removed, since Compare was its last user.
- `TestCompareShouldNotImportScoringOrStandings` guards `compare.go`'s imports (the package as a whole still imports `standings` via `leaderboard.go`).

## Spec Change Log

## Review Triage Log

### Pass 1 (2026-09-25)

| #   | Layer        | Finding                                                                | Verdict | Route  | Evidence                                                                                                                                   |
|-----|--------------|------------------------------------------------------------------------|---------|--------|--------------------------------------------------------------------------------------------------------------------------------------------|
| 1   | verification | "Nothing to compare yet." / no-table state never rendered by a test    | low     | patch  | Pre-verified gap: no `*_test.go` hit for the note; only the view model is asserted. The live data has an empty Playoffs row, so users see it. |
| 2   | verification | Stacked finalist class and countdown span only asserted on view model  | low     | patch  | Pre-verified gap: the acceptance regex accepts either class form; no test greps `compare-countdown`.                                       |
| 3   | blind        | Default "earliest deadline" rule indistinguishable from file order     | low     | patch  | The Background's cup is both earliest and first; the unit test covers it, the BDD gate does not.                                          |
| 4   | blind        | Empty state half tested; one-group-empty case untested                 | low     | patch  | Same root cause as #1 and fixed with it.                                                                                                   |
| 5   | blind, edge  | Series pick with empty Games renders "FLA in "                         | low     | reject | `SaveSeriesPick` is only reached after web-side `validSeriesGames`; only a hand edit of app-written rows can trigger it, and the fix adds a guard. |
| 6   | blind        | Category label rows not associated with value cells for screen readers | low     | patch  | `scope="colgroup"` on a separate `<tr>` labels no cells below it; a per-category `<tbody>` with `scope="rowgroup"` is a direct markup fix.  |
| 7   | blind        | Selector group labels not tied to their chips                          | low     | patch  | Plain `<span>` labels; `role="group"` + `aria-labelledby` is a direct fix.                                                                 |
| 8   | blind        | Own-column ice border breaks at every full-width label row             | false   | reject | The click-dummy's own full-width label blocks segment the "you" column the same way; the header and every value cell are still marked.     |
| 9   | blind        | Six or more players squeeze columns on a phone                         | false   | reject | The PRD fixes the pool at three players ("nothing needs to scale past three people").                                                      |
| 10  | blind        | Import guard adds a hand-written test against retro A8                 | low     | reject | It follows the existing `TestWebShouldNotImportScoring` pattern; adopting depguard is retro item A8's own work, not a direct fix here.     |
| 11  | blind        | epic-5-context.md still lists default and package as unconfirmed       | low     | patch  | Fixed directly: context now records the 2026-09-25 decisions and `internal/web` placement. Sprint status syncs in step 5.                 |
| 12  | blind        | Matrix "logged" error-handling not asserted                            | low     | reject | The expected behavior (set left out, page renders) is tested; asserting logs needs slog-capture scaffolding no other web test has.        |
| 13  | blind        | Gated-round fallback scenario checks only the chip                     | low     | patch  | Direct: add the cup deadline assertion as in the unknown-id scenario.                                                                     |
| 14  | blind        | Closed set's "· closed" countdown shows in ice                         | false   | reject | Predict renders the same `clock.Countdown` text in ice for closed rows (faint only when upcoming); Compare is consistent.                  |
| 15  | blind        | `singleTeamSetLabels` keyed by Kind constants but looked up by set id  | false   | reject | By design, the kind values equal their set ids (store's Kind comment, AD-17/AD-24/AD-28), and the sheet registry uses the same keys.      |
| 16  | blind        | Unit tests dereference `v.Table` without a nil check                   | low     | reject | A nil deref still fails the test run; unlikely to matter and not a product defect.                                                         |
| 17  | blind        | README fallback note incomplete                                        | low     | patch  | Direct doc correction.                                                                                                                     |
| 18  | edge         | Series winner outside the matchup still shown                          | false   | reject | Compare shows what the player actually saved; hiding it would misreport their pick.                                                        |
| 19  | edge         | Blank ids in TeamIDs/FinalistSlugs render an empty span                | low     | reject | Save paths validate every id through `roster`; only a hand edit of app-written rows can produce it, and the fix adds a guard.             |
| 20  | edge         | Selected set with zero rows (r1 unlocked, no matchups) shows header only | low   | reject | Needs a hand-flipped `upcoming: false` with no matchups; the matchups-not-set note is Story 5.2's gated-round state.                     |
| 21  | edge         | Empty `st.Players()` yields `colspan="0"`                              | low     | reject | With no players nobody can log in; only a stale session on an emptied pool reaches it.                                                    |
| 22  | edge         | Bad deadline logged on every request                                   | low     | reject | Identical to Predict's per-request logging of the same hand-edit mistake.                                                                  |
| 23  | edge         | Step regex accepts an unmapped round; `mustSeedRound` panics           | low     | reject | Pre-existing shared helper panics by design on a fixture typo; no scenario uses an unmapped round.                                        |
| 24  | edge         | `ownColumns` ignores cells beyond the header column count              | low     | patch  | Own-only scenarios don't compare the full grid, so an extra own-marked cell would pass; direct check.                                     |
| 25  | edge         | `noColumnIsOwn` doesn't assert the "You" label is absent               | low     | patch  | Verified at `compare_predictions_steps_test.go:395`; direct assertion.                                                                     |
