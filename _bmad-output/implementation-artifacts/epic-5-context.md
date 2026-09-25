# Epic 5 Context: Compare Predictions

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

A player opens the Compare tab, picks any one Prediction set, and sees every player's picks for that set side by side. Division picks are broken out per division, and every value is formatted the same way. This covers the "compare and argue" part of the pool's loop (predict, wait, get scored, compare, gloat). A typical use is checking who called an upset right after a playoff series ends. The epic is display-only. It reads the predictions saved in Epics 2–3 and never writes anything.

## Stories

- Story 5.1: Compare selector and side-by-side table
- Story 5.2: Division comparison granularity and consistent value formatting

## Requirements & Constraints

- **Always fully visible:** every player's picks for any set are shown at any time, to every player, including for sets whose deadline has not passed yet. Picks are never hidden until a deadline. This was a deliberate decision in favor of the click-dummy behavior.
- **Selector:** set options are grouped into two labelled rows, "Before the season" and "Playoffs". All options are visible at once, with no horizontal scrolling. The chosen set's deadline is shown.
- **Table:** one column per player, headed by the player's name in bold. The current player's own column is visually distinguished. There is one labelled row per prediction category.
- **Division granularity:** there is one row per division for the playoff-team picks, and that division's winner sits directly beneath it.
- **Value formatting:**
    - Team-abbreviation values (playoff teams, division winners, series winners) render as the same tag everywhere.
    - Full team names (Cup champion, Presidents' Trophy) render as plain text.
    - Series rows show the winner tag plus "in N".
    - A value nobody has entered shows a faint em-dash ("—"), never blank space.
- **Gated round:** if a still-gated playoff round were ever selected, a dashed "matchups not set" note replaces the table. This can't happen in practice while the round stays gated (Epic 3).
- **Default selection (confirmed 2026-09-25):** on first entry, the earliest "Before the season" set is pre-selected, so a table is shown instead of an empty screen.
- Section header copy: "Everyone's picks."

## Technical Decisions

- **Where the code lives:** the architecture names `internal/predictions` (data) and `internal/web` (rendering), but `internal/predictions` was never created; Compare lives in `internal/web` (`compare.go`), as Leaderboard does. It must not import `internal/scoring` or `internal/standings`. The only allowed feature-to-feature import is `standings` → `scoring`. `depguard` enforcement of this is still an open retro item, so for now it is checked only in code review.
- **Store access:** use exported `internal/store` methods and the shared structs and `kind` constants it exports. Never redefine them, and never touch store internals. If a query is missing, add a store read method. Reads are lock-synchronized.
- **Data shape:** there is one `Prediction` row per independently saveable pick, with a `kind` discriminator. A Series row carries both the winner and the game count. Values are stable IDs: team abbreviations such as `TOR`, and NHL Player slugs such as `mcdavid-connor`. Map IDs to display labels only at render time, using the season's canonical team and NHL Player lists.
- **Rendering:** server-side `html/template` on the stdlib `ServeMux`. No new client-side JS is allowed, because only the autocomplete widget and the division live-cap are sanctioned. Switching sets is server-rendered: each chip links to `/compare?set=<id>` (confirmed 2026-09-25).
- **Routing:** authenticated shell routes, `/compare` included, are registered on both the outer mux and the inner `authMux`. Any new Compare route must go on both lists (known drift risk).
- Wrap errors with `fmt.Errorf("context: %w", err)`. Failures surface only through logs.

## UX & Interaction Patterns

- **Selector chips:** use the shared Chip component. Selected is a `sel` fill with an `ice` border and primary text. Unselected is a `raised` fill with muted text. This is identical to the Division-picks chips.
- **Table:** a `raised` heading row, `border-soft` row dividers, and no shadows or gradients. The "you" column is marked with the `ice` accent, used as a border and never as a saturated fill.
- **Tags:** small, with their own rounding that is smaller than `rounded/lg`, in `muted` text. The empty dash uses `faint`.
- The layout is phone-width, single column, and dark only. Text is sentence case throughout. Tap targets are at least 32px. Color is never the only signal.
- **Flow:** the player opens Compare, taps a chip (for example "Playoff Round 1"), and the table swaps to that set. The player then scans a row to see who called it.

## Cross-Story Dependencies

- Story 5.2 extends the table from Story 5.1 with the division breakout, value formatting, and the gated-round state.
- Both stories read the prediction kinds saved in Epic 2 (Cup champion, Presidents' Trophy, division playoff teams and winners, award finalists) and Epic 3 (Playoffs Cup pick, series winner and game count). They also use Epic 3's round-gating to decide whether a round has matchups.
- Compare is reached through the bottom-navigation tab and shell from Epic 1. `/compare` already exists as a shell route.

## Open Retrospective Action Items

These are still open from the Epic 4 retro. Fold each into this epic's work or close it deliberately:

- **A8:** adopt `depguard` for the AD-8 import rules and delete the three hand-written import-guard tests.
- **A10:** put the story id in the final git trailer block (for example `Story: 5-1` directly above `Co-Authored-By`).
- **A11:** give each later reviewer the earlier Review Triage Logs and the accepted deviations, so rejected findings aren't raised again.
