# Epic 3 Context: Playoff Predictions

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Once the playoff field is set, a player gets a second, better-informed shot at the Cup pick, and can then predict each series' winner and exact game count as rounds unlock with real matchups — without ever predicting a round blind. This closes the loop the season-opening Cup pick started, replacing the old workbook's manual, error-prone playoff tracking with data-driven unlocking and atomic, typo-proof series picks.

## Stories

- Story 3.1: Playoffs Cup pick
- Story 3.2: Round unlocking based on recorded matchups
- Story 3.3: Per-round series predictions

## Requirements & Constraints

- The Playoffs Cup pick is a full 32-team, division-grouped pick — its own Prediction set, with its own deadline (shortly before Round 1's, but not assumed to be a fixed offset), stored completely separately from the season-opening Cup champion pick; neither ever overwrites the other.
- Round 2, the Conference Finals, and the Stanley Cup Final each stay in "Upcoming" status — shown but disabled, not just hidden — until that round's real matchups exist in the data file. There is no in-app or admin action that unlocks a round; a round becomes Open purely because a human recorded its matchups.
- Round 1 always contains exactly 8 series. Each series is labeled by conference ("Eastern Conference" / "Western Conference"), except the final round, labeled "Stanley Cup Final."
- For each series, a player picks the winning team and the exact game count (4, 5, 6, or 7). These two picks must be visually and interactively equal — neither is more prominent — and they save together as a single atomic unit. This is the one exception to the rest of the app's rule that any subset of a set's fields can be saved independently.
- An empty series pick at deadline scores zero and never blocks other picks in the same set or other deadlines.
- Real-world matchups, results, and deadlines are never entered or edited through the app in any form — this epic only reads them.

## Technical Decisions

- ID strategy: entities the app creates get generated UUIDs; everything hand-maintained (including playoff matchups) uses a short human-readable key — a round/series key such as `round1.s1`, team abbreviations for series winners. Nothing hand-typed is ever a UUID.
- Persistence granularity: one `Prediction` row per independently-saveable pick, with a `kind` discriminator. The Series kind is the sole exception with two fields (winner, game count) saved together in one row — never split into two rows or partially saved.
- Round unlocking and matchup data are read-only inputs: the relevant package reads `playoff_matchups` from the data file; no code path ever writes to that section. This mirrors how deadlines and results are handled elsewhere in the app.
- Server-rendered pages remain the default; no new client-side JS responsibility is introduced by this epic beyond the app's existing sanctioned widgets (autocomplete, live-cap checkboxes) — series winner/game-count selection is plain tap-to-select with server-side validation on save.

## UX & Interaction Patterns

- Playoffs Cup pick reuses the single-team dropdown pattern exactly (teams grouped by division, label gets a check on selection, editable until deadline, read-only after).
- Upcoming rounds render dimmed with a grey status pill and a lock icon in place of the chevron, and are not tappable — identical treatment to any other Upcoming set.
- Series prediction UI: intro copy "For each series, tap the winner and how many games it takes," series grouped under conference subheaders (or "Stanley Cup Final" for the last round). Two full-width team buttons for the winner plus four number buttons (4/5/6/7) for game count, all using the identical selected-state styling (neutral fill + accent border) — no control reads as more important than another, and no separate correct/incorrect coloring.
- Interaction is tap-only (no swipe/drag); selection feedback is immediate and local with no page reload; validation for the atomic winner+game-count save happens at submit time.
- Closed sets still open in read-only form with every input disabled and no action bar, consistent with the rest of the app.

## Cross-Story Dependencies

- Story 3.1 reuses the single-team-pick mechanics built for the season-opening Cup champion pick (Epic 2), applying them to a second, independently-stored prediction.
- Story 3.3's series predictions for a given round are gated by Story 3.2's unlocking logic — a round's Prediction sheet cannot be reached or filled in until its matchups are recorded.
- The Compare feature (a different epic) depends on this epic's round-unlocking state: an upcoming round is not reachable there either while gated.
