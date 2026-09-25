# Epic 4 Context: Scoring & Leaderboard

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Every player's Regular, Playoff, and Total points are computed automatically from the predictions saved in Epics 2–3 and the results a human records by hand in `fantasy-hockey.yml`. The results appear on a live, ranked Leaderboard. This is the core reason the app is being rebuilt: the old Excel workbook was scored by hand and a scoring error once went uncorrected. Getting the scoring exactly right, and never storing a score that can go stale, matters more than anything else in this epic.

## Stories

- Story 4.1: Automatic scoring engine
- Story 4.2: Leaderboard display

## Requirements & Constraints

- Point table (season-1 calibration, may be revised later, so keep values easy to change):

    | Prediction                         | Points                                                    | Bucket  |
    |------------------------------------|-----------------------------------------------------------|---------|
    | Award finalist (each of 3 names)   | 5 per name found anywhere in actual top-3 (ties expand)   | Regular |
    | Team makes the playoffs            | 5                                                         | Regular |
    | Division winner                    | 15 (replaces, does not add to, the 5-point mark)          | Regular |
    | Presidents' Trophy pick            | 20                                                        | Regular |
    | Cup champion (season-opening pick) | 20                                                        | Regular |
    | Playoffs Cup pick                  | 20                                                        | Playoff |
    | Round 1 series                     | 15 correct winner / 25 exact result (replaces, not added) | Playoff |
    | Round 2 series                     | 25 / 35                                                   | Playoff |
    | Conference Finals series           | 30 / 45                                                   | Playoff |
    | Stanley Cup Final series           | 30 / 50                                                   | Playoff |

- The season-opening Cup pick counts toward Regular. The Playoffs Cup pick counts toward Playoff. They are scored independently.
- All 5 awards (Hart, Norris, Vezina, Art Ross, Rocket Richard) score the same way against whatever finalists are recorded for them. No award is exempt. A real-world tie past 3rd place expands the recorded finalist set, which is edited by hand and never entered by a player.
- An empty pick contributes zero without raising an error. A missing result (not yet recorded) scores nothing yet and must not fail.
- Total = Regular + Playoff, and Total is the column that decides rank. Players with equal Totals share the same rank, with no tiebreaker of any kind.
- The Leaderboard is always live. There is no cached value and no separate "in-progress"/projected tier: a new result or prediction shows up on the next open or refresh.
- The click-dummy's Leaderboard numbers are placeholder sample data and must never be used as expected values.
- Scope discipline: build only what is specified. Scoring errors are one of the failure modes this app exists to prevent.

## Technical Decisions

- **Where scoring lives:** `internal/scoring` is the only home of point-calculation logic (awards, team marks, series, Cup picks). It depends only on `internal/store` and never imports `internal/standings`.
- **How the Leaderboard reads it:** `internal/standings` computes Regular/Playoff/Total and the ranks by calling `internal/scoring`'s exported functions. This is the only feature-to-feature import allowed anywhere in the system, and it goes one way only. `standings` must never re-derive a point value itself. `internal/web` renders the result. The planned `depguard` enforcement of these import boundaries is not configured yet, so this rule is checked only in code review for now.
- **Live, never persisted:** every call recomputes fresh from `internal/store`'s in-memory predictions, results, and award finalists. No caching anywhere, and no score is ever written to `fantasy-hockey.yml`.
- **Store access:** go through exported `internal/store` methods only; the in-memory data and its mutex are never exposed. Reads are lock-synchronized, so Leaderboard reads can safely run alongside prediction saves. If a query is missing, add a store read method rather than reaching into internals. Use the shared structs and `kind` constants exported by `store`, and never redefine them in a feature package.
- **Read-only inputs:** results (`team_marks` playoff lists and division winners, `presidents_trophy`, `stanley_cup_winner`, `series` winner+games keyed like `round1.s1`) and `award_finalists` are hand-maintained. Scoring only reads them, and no code path writes them. The exact YAML field names are an implementation-time decision.
- **Matching by stable ID only:** compare prediction values to results by team abbreviation (`TOR`) or NHL Player slug (`mcdavid-connor`), never by display name. `AwardFinalist` is a `{slug, display_name}` struct and the slug is the value compared.
- **Prediction row shape:** there is one `Prediction` row per independently saveable pick, with a `kind` discriminator (Cup champion, Presidents' Trophy, one per division's playoff-team list, one per division winner, one per award trio, one per series). A Series row carries both winner and game count. An exact result means the winner and game count both match.
- Errors are wrapped with `fmt.Errorf("context: %w", err)`, with no custom error envelope. Failures surface only through logs.

## UX & Interaction Patterns

- A table with one row per player and columns Player, Regular, Playoff, Total, sorted by Total descending. The heading row uses a `raised` background and rows are separated by `border-soft` dividers. There are no shadows or gradients.
- The Total column is set apart with a left `border` and a `raised` tint. Totals use monospace with `tabular-nums` and are the largest text on the screen (about 18–19px), in bold to extrabold.
- The rank badge is a full circle. The leader's badge and Total are `gold` (`#d29922`), and gold is used nowhere else. Other badges are neutral.
- No "(you)" marker on any row, including the current player's own. This is intentional and differs from Compare.
- Caption: "Ranked by total points." A footer caption explains that Regular + Playoff make up Total.
- Server-rendered with no client-side JS. Where a player stands is visible right away, with no tap needed.

## Cross-Story Dependencies

- Story 4.2 depends on Story 4.1: the Leaderboard uses `internal/scoring`'s exported functions and has no scoring logic of its own.
- Story 4.1 uses the prediction kinds saved in Epic 2 (before-season picks) and Epic 3 (Playoffs Cup pick and series winner+game count), and the empty-pick-scores-zero rule defined there.
- Leaderboard is reached through the bottom-navigation tab built in Epic 1.

## Open Retrospective Action Items

These were carried over from earlier retrospectives (Epic 3 retro item 23). Fold each one into a story in this epic, or close it deliberately. Do not let it pass silently.

- **Epic 2 item 11:** correct two out-of-date doc comments, `newTeamOptions` in `internal/web` and `load_canonical_team_list_steps_test.go`.
- **Epic 2 item 14:** add "And the shell shows the Predict content" to the app-shell Predict scenario.
- **Epic 2 item 15:** scope the "closed" assertion in `TestPredictShouldShowAClosedPillAndCountdownForASetPastItsDeadline` to the countdown span, since right now it passes without really checking anything.
- **Epic 2 item 16:** add a regression test for how a set that is both Closed and Submitted is displayed (accent vs. pill).
- **Epic 3 item 21:** write a playoffs runbook in `docs/` for the person running the app.
- **Epic 3 item 24:** update the specs to match what was built. Story 3.2 AC2 needs an app restart after a hand edit, and the spec-3-3 Code Map should mention that half-filled series are skipped silently.
- **Epic 3 item 25:** name the story id in each story commit (e.g. a `Story: 4-1` footer).
