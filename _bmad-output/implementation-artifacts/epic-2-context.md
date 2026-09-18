# Epic 2 Context: Before-the-Season Predictions

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

A player can browse all Prediction sets grouped by phase and status, then fill in and submit all four before-season predictions — Cup champion, Presidents' Trophy, Division picks (playoff teams + division winners), and Player awards finalists — before each set's own deadline. Any subset of a set's fields can be edited independently until it locks, and every pick persists centrally so it's visible and consistent across devices and players. This epic exists to close two of the three real failure modes the rebuild targets: deadline confusion and name-typo scoring gaps (award-finalist picks must resolve to a stable id, never free text).

## Stories

- Story 2.1: Browse Prediction Sets by Phase and Status
- Story 2.2: Load Season's Canonical Team List
- Story 2.3: Cup Champion and Presidents' Trophy Picks
- Story 2.4: Division Picks — Playoff Teams and Division Winners
- Story 2.5: Load Season's Canonical NHL Player List
- Story 2.6: Player Awards Finalists

## Requirements & Constraints

- Predict screen shows two sections ("Before the season" / "Playoffs"); each set shows title, subtitle, deadline (Europe/Berlin) with a relative countdown, and one status: Open, Submitted, Closed, or Upcoming. Deadlines are read directly from the data file per set — no shared/implied pattern across sets, no in-app settings page for them.
- A set locks automatically at its deadline, with no grace period and no override for anyone; a locked set still opens but read-only.
- A submitted-but-open set can be revised until its deadline; any subset of a set's fields saves independently — except a Series winner+game-count pair (Epic 3), which is the sole multi-field exception.
- A pick left empty at its deadline scores zero for that item only and never blocks any other prediction.
- Cup champion and Presidents' Trophy: single-team pick from all 32 teams grouped by division.
- Division picks: choose playoff teams per division (each conference must total exactly 8, as a 4/4 or 5/3 split, capped live as it's selected) and one division winner per division (4 total), each dropdown scoped to that division's own teams. Submit is blocked with an explanatory message until both conferences are valid.
- Player awards: 3 finalists for each of 5 awards, entered via autocomplete scoped by eligible position (skaters for Hart/Art Ross/Rocket Richard, defensemen for Norris, goalies for Vezina); a name not matching the season's NHL Player list is rejected, never silently accepted as free text.
- The season's canonical team list and NHL Player candidate list must be available everywhere a pick is made, sourced from one place and reused consistently.
- Success criterion (PRD SM-2): all three players complete every before-season set before its deadline without reverting to the previous Excel-based process — checkable at the first real deadline, not just season's end.

## Technical Decisions

- Deadlines, the team list, and the NHL Player candidate list are all human-maintained directly in `fantasy-hockey.yml`; no code path in this epic ever writes any of them — `internal/store` only exposes read-only accessors for them.
- `internal/store` holds one `Prediction` row per independently-saveable pick (one for Cup champion, one for Presidents' Trophy, one per division's playoff-team list, one per division winner, one per award's finalist trio), each carrying a `kind` discriminator; there is no single row or payload blob representing a whole set.
- Every identity a player picks by name (team, NHL Player) is referenced everywhere by the same stable id — a team abbreviation or an NHL Player slug — never free-text display name. `store.AwardFinalist` is a real struct (`{Slug, DisplayName}`), scoped by position, not a bare string list; a slug is generated once when an NHL Player is first added and reused everywhere.
- The team list and NHL Player list are embedded as JSON directly in the rendered page (shape `{"id": ..., "label": ...}`) for a shared vanilla-JS autocomplete widget — no dedicated `/api/...` endpoint. The widget always submits `id`, never `label`.
- Presentation is server-rendered HTML (stdlib `html/template`/`net/http`); the only client-side JS in this epic is the autocomplete widget and the Division-picks live-cap disabling — both cosmetic-immediacy only, since the server independently re-validates and enforces every cap on submit regardless of what the client allowed.
- All reads and writes against `internal/store`'s in-memory data are mutex-synchronized; a save triggers exactly one atomic write-and-rename of the whole file.
- Every persisted/exchanged timestamp is RFC3339 via `internal/clock.NowTime().UTC().Format(time.RFC3339)`; errors are wrapped with `fmt.Errorf("context: %w", err)`.

## UX & Interaction Patterns

- Set row: 3px left accent stripe by state (blue/green/red/faint), status pill naming the state in words (not color alone), subtitle, deadline+countdown line, chevron for actionable rows or a lock icon on dimmed, non-tappable Upcoming rows.
- Prediction sheet: full-screen over the app frame, pinned header (back arrow, title, deadline+countdown), scrolling body, pinned action bar reading "Submit predictions" or "Update predictions" once already submitted; a Closed set shows a read-only banner with every input disabled and no action bar.
- Single-team pick: one dropdown grouped by division; the label gets a green check the moment a team is chosen.
- Division picks: team chips toggle with `sel` fill + `ice` border; each division shows a live n/5 counter and each conference a live n/8 indicator (check when valid, dot when not, always paired with the count text); once a scope hits its cap, remaining chips in that scope dim to 40% opacity and stop being tappable; an invalid state shows a red caption stating exactly what's missing.
- Player awards: five trophy groups of three finalist text inputs; a rejected name shows a `goal`-colored border with an inline caption; a fully-filled award group's label gets a green check.
- Winner/game-count-style controls (chip, number button) share one selected-state language — neutral fill + accent border — with no separate correctness coloring; validity is always communicated via captions/pills, not by recoloring picks.
- Tap-only interaction, immediate local selection feedback with no page reload; live-cap validation at input time, disabled-button-plus-caption validation at submit time otherwise.
- Accessibility floor: tap targets ≥32px, tabular numerals for any updating count, every form input labeled, color never the sole signal.

## Cross-Story Dependencies

- Story 2.3 (Cup champion / Presidents' Trophy) and Story 2.4 (Division picks) both depend on Story 2.2's canonical team list for their team dropdowns/chips.
- Story 2.6 (Player awards) depends on Story 2.5's canonical NHL Player list for autocomplete and validation.
- Story 2.3's single-team-pick Prediction-sheet mechanics are reused as-is by Epic 3's Playoffs Cup pick (Story 3.1), which stores its pick separately without overwriting the season-opening Cup champion pick from this epic.
