---
title: 'Leaderboard Display'
type: 'feature'
created: '2026-09-25'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '44985f7a2775460d25647a103ba405d08353a7db'
context:
    - '{project-root}/_bmad-output/implementation-artifacts/epic-4-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 4.1 computes every player's points, but the Leaderboard tab still shows "The leaderboard is coming soon." Nobody can see who's winning.

**Approach:** Add an `internal/standings` package that turns `internal/scoring`'s points into ranked rows for every player. Render those rows on the existing Leaderboard tab as the table the click-dummy and DESIGN.md describe. Everything is recomputed on every request.

## Boundaries & Constraints

**Always:**

- One row per player in `players:`, with columns Player, Regular, Playoff and Total, sorted by Total descending. This includes players with no picks, who show 0.
- Equal Totals share a rank, with no tiebreaker of any kind. Tied players are listed in alphabetical order by name. That order is only for display and never changes a rank.
- `internal/standings` gets every point value from `scoring.PlayerPoints` and never derives one itself. It imports only `internal/scoring` and `internal/store`. `internal/web` renders the rows and computes no points or ranks.
- Nothing is cached. Every request to `/leaderboard` recomputes from the store.
- Look and copy (DESIGN.md and click-dummy):
    - Heading "Leaderboard", with the caption "Ranked by total points."
    - A `raised` heading row and `border-soft` row dividers.
    - The Total column has a left `border` and a `raised` tint, in bold monospace `tabular-nums` at about 19px.
    - The rank badge is a full circle, neutral by default.
    - Footer: "Regular-season picks feed the Regular column, playoff-round picks feed Playoff, and Total is what decides the standings."
- Decision, rank numbering: competition ranking. Players who share a Total share a rank, and the next rank skips accordingly (60, 60, 45 is ranked 1, 1, 3).
- Decision, leader: every rank-1 player is a leader, but only while the top Total is above 0. Leaders get `gold` on their rank badge and Total. Before anything scores, nobody is gold.
- `gold` appears only on a leader's rank badge and Total.
- No row carries a "(you)" marker, not even the signed-in player's own row.
- The Leaderboard tab stays active in the bottom nav, and the existing nav-link markup is unchanged.

**Never:**

- Show a projected or in-progress score tier.
- Add client-side JS.
- Cache or persist standings.
- Change scoring rules.
- Add a Compare view.

## I/O & Edge-Case Matrix

| Scenario           | Input / State                                  | Expected Output                                           |
|--------------------|------------------------------------------------|-----------------------------------------------------------|
| Clear leader       | Totals Basti 60, Sadl 45, Tobbi 20             | Ranks 1, 2, 3 in that order; only Basti gold              |
| Tie below the top  | Totals Basti 60, Sadl 45, Tobbi 45             | Sadl and Tobbi share rank 2, with Sadl listed first (A–Z) |
| Tie at the top     | Totals Basti 60, Sadl 60, Tobbi 45             | Basti and Sadl share rank 1 and are both gold; Tobbi is 3 |
| Nothing scored yet | Every Total is 0                               | Everyone is rank 1 and nobody is gold                     |
| Player with no picks | Tobbi has no prediction rows                 | Tobbi's row shows 0 / 0 / 0                               |
| Live update        | A result is recorded and the store reloaded    | The next request shows the new Totals and order           |
| Own row            | Signed in as Sadl                              | Sadl's row looks like every other row, with no "(you)"    |

</frozen-after-approval>

## Code Map

- `src/internal/scoring/scoring.go:43-56`: `Points{Regular, Playoff}`, `Total()` and `PlayerPoints(st, playerID)`, which returns zero for an unknown id. Consume these as they are.
- `src/internal/store/store.go:950`: `Players()` returns players in file order. Standings sorts them.
- `src/internal/scoring/imports_test.go`: the pattern for the standings import guard (only `internal/scoring` and `internal/store`).
- New `src/internal/standings/`: `Row{Rank int, PlayerID, Name string, Points scoring.Points, Leader bool}` and `Rows(st *store.Store) []Row`, plus a `README.md`.
- `src/internal/web/shell.go:46-97`: `shellData` gains `Leaderboard *leaderboardView`. `handleShell` fills it for `tabLeaderboard`, the same way the Predict branch at `:90-93` does. Remove the Leaderboard entry from `tabMessages` (`:33-36`).
- New `src/internal/web/leaderboard.go`: `leaderboardView` and its row view model. CSS classes are precomputed in Go, as `predict.go` does with `AccentCSS`.
- `src/internal/web/templates/shell.html:44-55`: add an `{{else if .Leaderboard}}` branch before the `coming-soon` fallback. Keep the nav markup at `:58-60` unchanged.
- `src/internal/web/static/styles.css:9-24`: add a `--border-soft: #21262d` token. Add leaderboard table, rank-badge and total styles, reusing the monospace stack from `.code-input` (`:107-113`).
- `src/internal/web/shell_test.go:35`: `shellRouteTests` expects "The leaderboard is coming soon." Update it.
- `src/internal/web/web_test.go:30`, `predict_test.go:63`: reuse `newTestStore` and `assertMarkersInOrder`.
- `src/acceptance-tests/features/app-shell.feature:15-18` and `app_shell_steps_test.go:162-178,253`: the Leaderboard placeholder step. Replace it with a Leaderboard content check. Fold in Epic 2 retro item 14 here too: add "And the shell shows the Predict content" to the Predict scenario.
- `src/acceptance-tests/fixture_support_test.go:59,128,178`: `seedHeader` seeds one player, so extra players go under `players:` from `seedBody`. Reuse `newLazyFixture` and `do`.

## Tasks & Acceptance

**Execution:**

- [x] `src/acceptance-tests/features/leaderboard.feature` and `src/acceptance-tests/leaderboard_steps_test.go`: write scenarios over HTTP for the matrix rows. Seed three players with picks and results, GET `/leaderboard` as a signed-in player, and assert row order, ranks, gold and the absence of "(you)". Register them in `suite_test.go` and confirm they fail (red) first.
- [x] `src/acceptance-tests/features/app-shell.feature` and `app_shell_steps_test.go`: replace the Leaderboard placeholder expectation, and add retro item 14's Predict content step.
- [x] `src/internal/standings/standings_test.go`, `standings.go`, `imports_test.go` and `README.md`: write table-driven tests (ordering, shared ranks, display order within a tie, leader flag, zero-pick player), each with a "should not" counterpart. Then implement.
- [x] `src/internal/web/leaderboard_test.go`, `leaderboard.go`, `shell.go`, `templates/shell.html`, `static/styles.css` and `shell_test.go`: test-first. Rows render in order with rank and name. Gold classes appear only on leader rows. There is no "(you)" marker. The caption and footer are present, and the placeholder is gone.
- [x] `src/internal/web/README.md`: mention the Leaderboard view.

**Acceptance Criteria:**

- Given `internal/standings`, when its imports are listed, then it imports `internal/scoring` and `internal/store` and no other `internal/` package.
- Given `internal/web`, when its imports are listed, then it doesn't import `internal/scoring`. It reads points only through `standings.Row`.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| #  | Source       | Finding                                                                 | Verdict | Route  | Evidence                                                                                                  |
|----|--------------|-------------------------------------------------------------------------|---------|--------|-----------------------------------------------------------------------------------------------------------|
| 1  | blind, edge  | Empty pool renders a header-only table with no message                  | false   | reject | You can only reach `/leaderboard` signed in, and signing in needs a player in `players:`, so the pool is never empty there. |
| 2  | blind        | Result problems never shown on the Leaderboard                          | false   | reject | The user decided in 4-1 that malformed results are startup log warnings. Showing them on the page is outside that intent. |
| 3  | blind, edge  | Restart scenario regenerates the seed instead of editing the file       | low     | patch  | The step rebuilds the file from `seedBody`, and its comment overstated it as a hand edit. Comment reworded. |
| 4  | blind        | Leader shown by colour alone; caption is a `<p>`, not `<caption>`       | low     | reject | A private 3-player pool. The rank number already shows who leads, and the fix adds markup and CSS.        |
| 5  | blind        | Tie order isn't locale-aware (umlauts sort after z)                     | low     | reject | The current names are ASCII. A collator adds a dependency.                                                |
| 6  | blind, edge  | Row regexp built without `QuoteMeta`                                    | low     | patch  | A direct correction.                                                                                      |
| 7  | blind, edge  | `ReplaceAll` anonymization could hit class names; `[^<]+` name pattern  | low     | reject | Test-only. The seeded ids and names can't collide with the markup.                                       |
| 8  | blind        | Placeholder step reuses the "(you)" marker helper and its error message | low     | patch  | A misleading failure message. A direct correction.                                                        |
| 9  | blind        | Duplicated import-guard logic in two test files                         | low     | reject | Two small test copies with no named harm.                                                                 |
| 10 | blind        | `--border-soft` equals `--raised`, so dividers disappear in the Total column | low | reject | Both values come from DESIGN.md as specified, so the effect is cosmetic.                                  |
| 11 | blind        | `lb-head-num` class undefined; `.lb-num` repeats tabular-nums           | false   | reject | `.lb-table th` already right-aligns every header, so the class has no visible effect.                    |
| 12 | blind        | Web README omits the no-gold-at-zero rule                               | low     | patch  | A direct doc correction.                                                                                  |
| 13 | blind        | Fixture pairs `conference: Eastern` with `Divisions()[0]`               | low     | patch  | Correct only while index 0 is Atlantic. Now uses the literal division name.                               |
| 14 | blind        | No stale-session scenario on `/leaderboard`                             | low     | reject | `handleShell`'s neutral-header path already covers it for every tab.                                      |
| 15 | edge         | Duplicate player ids produce two rows                                   | low     | reject | `players:` is hand-maintained, and duplicate ids already break login lookups. It's pre-existing and unlikely. |
| 16 | edge         | No single-lock snapshot across `Rows`                                   | low     | reject | A save racing a render changes at most one refresh. The pool is small.                                    |
| 17 | edge         | Empty or odd player id makes a malformed row id                         | low     | reject | Every hand-maintained id is a slug, and login requires a valid one.                                       |
| 18 | edge         | "Saves while the app runs" step before any open step adds a phantom player | low  | reject | Test-only. No current scenario orders its steps that way.                                                 |
| 19 | vg           | No verification gaps                                                    | —       | —      | The layer reported none.                                                                                  |

## Verification

**Commands:**

- `task go:test` (expected: pass; `internal/standings` coverage 100%)
- `task go:test:acceptance` (expected: pass, including leaderboard.feature and the updated app-shell.feature)
- `task go:run` (expected: builds and starts; `/leaderboard` returns 200 for a signed-in player)
