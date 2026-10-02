# Epic 2 Context: Before-the-Season Predictions

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

A player can browse all prediction sets, fill in and submit the four before-season predictions (Cup champion, Presidents' Trophy, division picks, player awards) before each deadline, edit any subset of fields until the set locks, and have it all persist and sync across devices. The epic also covers the 2026-10-01 change: an award with only some of its 3 finalists filled is flagged with an error instead of being silently dropped (Story 2.7).

## Stories

- Story 2.1: Browse Prediction Sets by Phase and Status
- Story 2.2: Load Season's Canonical Team List
- Story 2.3: Cup Champion and Presidents' Trophy Picks
- Story 2.4: Division Picks - Playoff Teams and Division Winners
- Story 2.5: Load Season's Canonical NHL Player List
- Story 2.6: Player Awards Finalists
- Story 2.7: Incomplete Award Picks Are Flagged

## Requirements & Constraints

- Sets are grouped by phase ("Before the season" / "Playoffs"), each showing title, subtitle, Europe/Berlin deadline with relative countdown, and a status (Open, Submitted, Closed, Upcoming). Deadlines come only from the data file; sets need not share a deadline.
- A set locks automatically at its deadline: no grace period, no override, read-only for everyone including the author.
- A Player may revise a submitted set until the deadline; any subset of fields saves independently.
- A pick left empty at the deadline scores zero for that item and blocks nothing else.
- Division picks: exactly 8 playoff teams per conference (4/4 or 5/3 split), at most 5 per division, plus one winner per division from that division's own teams.
- Player awards: Hart, Norris, Vezina, Art Ross, Rocket Richard, 3 finalists each. Autocomplete is scoped by position (skaters for Hart/Art Ross/Rocket Richard, defensemen for Norris, goalies for Vezina).
- Award completeness (Story 2.7): an award left entirely blank is fine and not saved, with no error. If any finalist of an award is filled, all 3 are required. An incomplete award is not saved and shows the inline caption "Pick all 3 finalists for this award, or clear it." on its empty slots. The sheet re-renders (200) with input kept, and fully filled awards in the same submit are still saved. Clearing a slot of an already-saved award flags it as incomplete and leaves its saved picks untouched.
- A typed name that matches no suggestion, or a repeated name, still rejects the whole award submit and saves nothing.
- Every team or NHL Player pick is stored by stable id (team abbreviation, player slug), never by display name, so typos cannot silently fail to score.

## Technical Decisions

- Server-rendered HTML (stdlib `html/template`, `net/http` ServeMux). Client JS is limited to the autocomplete widget and the Division-pick live-cap disabling. The server re-validates every submit independently.
- `internal/store` is the only package touching `fantasy-hockey.yml` and owns the entity structs. `AwardFinalist` is a struct `{slug, display_name}`, never a bare string list. Team and NHL Player lists, deadlines, and results are hand-maintained and read-only to app code; only predictions and login codes are written at runtime.
- Autocomplete data is embedded as JSON in the page, with the identical shape `{"id": "<id>", "label": "<display name>"}` at every embedding site, and the shared widget always submits `id`. There is no JSON API endpoint.
- Prediction row granularity: one row per independently-saveable pick (Cup champion, Presidents' Trophy, each division's playoff-team list, each division winner, each award's finalist trio). Runtime-created rows get UUIDs.
- Timestamps are RFC3339 via `internal/clock`. Errors are wrapped with `fmt.Errorf("context: %w", err)`. Complexity limit 10 (gocyclo).
- Persistence is a single YAML file with atomic write-and-rename, and store reads and writes are lock-synchronized.
- Observability from Epic 9: each saved award counts and audits as one row. An incomplete award saves nothing and emits nothing.

## UX & Interaction Patterns

- Predict screen: set rows with a 3px left accent stripe by state, status pill (Open blue, Submitted green, Closed red, Upcoming grey), chevron on tappable rows, and a dimmed, lock-icon, non-tappable Upcoming row.
- Full-screen Prediction sheet: pinned header (back, title, deadline and countdown), scrolling body, pinned action bar. The button reads "Submit predictions", or "Update predictions" when editing. A closed set shows a read-only banner with no action bar.
- Team picks are dropdowns grouped by division. Division chips use sel fill and an ice border, with live `n/5` and `n/8` counters (check or dot always paired with the count). Capped chips dim to 40% and become non-tappable. Submit stays disabled with a red caption naming what is missing.
- Award groups show a green check when all 3 finalists are valid. Invalid names and incomplete awards use the `goal`-colored border plus an inline caption (the incomplete state is an [ASSUMPTION] pending product owner confirmation).
- Tap-only interaction, with immediate local feedback and no reload for chip or dropdown picks. Tap targets are at least 32px and color is never the sole signal.

## Cross-Story Dependencies

- Story 2.3 depends on 2.2 (team list). Story 2.4 depends on 2.2. Story 2.6 depends on 2.5 (NHL Player list). Story 2.7 amends 2.6, whose "partial award is silently skipped" boundary is superseded.
- Story 2.7 needs a Gherkin scenario in `src/acceptance-tests/features/` (award-finalists.feature) written first. It replaces the unit test `TestPostAwardsSheetShouldAcceptAndSkipAPartiallyFilledAward`, and the "Leaving some awards blank" scenario stays valid for fully blank awards only. It also requires regenerating the embedded game-rules copy (`task docs:embed-game-rules`).
- Epic 3 (playoff predictions) reuses the sheet, status and deadline mechanics. Epic 4 scoring reads saved predictions, so partial awards are never persisted. Epic 9 counters and audit log hook into prediction saves.
