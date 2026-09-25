---
title: 'Story 5.2: Division Comparison Granularity and Consistent Value Formatting'
type: 'feature'
created: '2026-09-25'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
    - '{project-root}/_bmad-output/implementation-artifacts/epic-5-context.md'
baseline_commit: '402050eff04cd4f1db844eb162e6420872585236'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Compare (Story 5.1) already renders one row per division for playoff-team picks with that division's winner directly beneath it, but every value is plain text, and a hand-typed `?set=` for a still-gated round silently falls back to the default set instead of saying so.

**Approach:** Give team-abbreviation values (playoff teams, division winners, series winners) a consistent small "tag" style; make the empty "—" render faint; and make a gated-round selection show a dashed "Matchups not set." note in place of the table.

## Boundaries & Constraints

**Always:** Full team names (Cup champion, Presidents' Trophy) stay plain text — never tagged. Series rows render the winner as a tag plus plain " in N" text, not one opaque string. The gated-round note only replaces the table for an id that names a real, still-`Upcoming` set — an unknown/nonexistent id keeps falling back to the default set (5.1 behavior, unchanged). Division row order, column order, and the "always fully visible" rule are unchanged from 5.1.

**Never:** No new store methods (`st.PredictionSets()` + the existing `effectiveUpcoming` cover the gated check). No new client-side JS. No import of `internal/scoring`/`internal/standings`.

## I/O & Edge-Case Matrix

| Scenario                   | Input / State                                  | Expected Output / Behavior                                                                         | Error Handling |
| -------------------------- | ---------------------------------------------- | -------------------------------------------------------------------------------------------------- | -------------- |
| Division playoff-teams row | Sadl picked 3 teams for the Atlantic           | 3 tag-styled abbreviations in Sadl's cell, one per team                                            | N/A            |
| Series row                 | Basti picked TOR in 5 for a round-1 matchup    | cell shows a tag "TOR" plus plain text " in 5"                                                     | N/A            |
| Cup / Presidents row       | a player picked a Cup champion                 | full team name, plain text, no tag                                                                 | N/A            |
| No value entered           | a player has no pick for a category            | faint "—", not full-strength text                                                                  | N/A            |
| Gated round selected       | `?set=r2` while `r2` is still `Upcoming`       | dashed "Matchups not set." note replaces the table; chips/groups still render, no chip is selected | N/A            |
| Unknown/bad id (unchanged) | `?set=nope` or a bad-deadline/unknown-phase id | same as the default selection (5.1 behavior)                                                       | none           |

</frozen-after-approval>

## Code Map

- `src/internal/web/compare.go` — `compareCellView.Values`: `[]string` → `[]compareValueView{Text, CSS}`; each category's `values` closure picks tag/plain/empty CSS per value. Fixes `divisionCategories`'s playoff-teams `ok`-discard along the way (Story 5.1 Pass 2 finding #27). `buildCompare` adds `isGatedRound(st, id)` (via `st.PredictionSets()` + existing `effectiveUpcoming`) before falling back to `defaultCompareSet`, setting new `compareView.Note` instead. Drop `compareSeriesValue = "%s in %s"`; add per-value CSS consts beside `compareCellCSS`.
- `templates/shell.html` — value loop (line 58) → `{{range .Values}}<span class="{{.CSS}}">{{.Text}}</span>{{end}}`; `{{with .Compare.Table}}` (line 118) gets `{{else if .Compare.Note}}` → `<p class="cmp-note-dashed">{{.Compare.Note}}</p>`.
- `static/styles.css` — Compare block: `.cmp-tag` (6px radius like `.status-pill`, `color: var(--muted)`, `background: var(--raised)`), `.cmp-value--empty` (`color: var(--faint)`), `.cmp-note-dashed` (new dashed-border style).
- `compare_test.go` — update `Cells[i].Values` assertions for the new shape; add tag/empty/`Note` cases.
- `acceptance-tests/compare_predictions_steps_test.go` — `compareCellValuePattern` must capture the full class attribute (currently only `cmp-value`); add a gated-note step (mirror `theSheetShows`'s `strings.Contains`).
- `features/compare-predictions.feature` — rewrite the "still-gated round falls back" scenario to assert the note instead; add tag/plain/faint scenarios.
- `internal/web/README.md` — update the Compare bullet.

## Tasks & Acceptance

**Execution:**
- [x] `src/acceptance-tests/features/compare-predictions.feature` + `compare_predictions_steps_test.go` — rewrite the gated-round scenario, add tag/plain/faint scenarios (red first) — BDD gate.
- [x] `src/internal/web/compare_test.go` — unit tests for `compareValueView` CSS per category kind, the `ok`-guard fix, and `isGatedRound` — TDD.
- [x] `src/internal/web/compare.go` — `compareValueView`, updated categories, `isGatedRound`, `compareView.Note`.
- [x] `src/internal/web/templates/shell.html`, `static/styles.css` — tag/faint/dashed-note rendering.
- [x] `src/internal/web/README.md` — update the Compare bullet.

**Acceptance Criteria:**
- Given the compared set includes division playoff-team picks, when the table renders, then there is one row per division for the playoff-team picks with that division's winner directly beneath it (already true from 5.1; confirm unchanged).
- Given any cell's value, when the table renders, then team abbreviations render as a consistent tag, full team names render as plain text, series rows show the tag plus " in N", and an unfilled value shows a faint "—".
- Given a still-gated playoff round's id is selected via `?set=`, when the table would render, then a dashed "Matchups not set." note replaces it.

### Review Findings

- [x] [Review][Patch] `isGatedRound` doesn't validate a round-gated id maps to a real, well-formed Prediction Set before trusting the matchup count [src/internal/web/compare.go:217]
- [x] [Review][Patch] Series row's tag + " in N" suffix likely renders over-spaced (CSS margin + literal leading space) [src/internal/web/static/styles.css:1100]
- [x] [Review][Patch] `awardCategories` discards `ok` from `st.FindAwardFinalists`, unlike the just-fixed `divisionCategories` sibling [src/internal/web/compare.go]
- [x] [Review][Patch] This spec's own markdown tables aren't column-padded per house style [_bmad-output/implementation-artifacts/spec-5-2-division-comparison-granularity-and-consistent-value-formatt.md]

## Implementation Notes

- `compareCellView.Values` is now `[]compareValueView{Text, CSS}` instead of `[]string`; every category's `values` closure returns tag- or plain-styled values via new `tagValue`/`tagValues`/`plainValue` helpers, and `newCompareRow` substitutes a single `emptyCompareValue()` (faint) when a cell has none.
- `divisionCategories`'s playoff-teams row now explicitly checks `ok` from `st.FindDivisionPlayoffTeams` (Story 5.1 Pass 2 finding #27) instead of discarding it; a saved-but-empty pick renders the same faint dash as no pick at all (covered by a dedicated unit test).
- Series rows now return two values per player: a tagged `TeamID` plus a plain `" in %s"`-formatted games suffix, replacing the old single opaque `compareSeriesValue = "%s in %s"` string.
- `isGatedRound(st, id)` re-checks the same phase/deadline validity `selectableCompareSets` already applies (so a bad-deadline or unknown-phase id still falls back to the default set unchanged), then reports `effectiveUpcoming` for a real match. `buildCompare` returns `compareView{Groups, Note: compareGatedRoundNote}` (no `Table`) when the requested id is unselectable but `isGatedRound` is true; groups still render with no chip selected (`newCompareGroups(sets, "")`).
- `shell.html`'s `{{with .Compare.Table}}...{{else if .Compare.Note}}` from the spec's Code Map does not parse (`html/template` does not support `else if` after `with`, confirmed by a throwaway repro) - implemented as `{{if .Compare.Table}}{{template "compare-table" .Compare.Table}}{{else if .Compare.Note}}<p class="cmp-note-dashed">{{.Compare.Note}}</p>{{end}}` instead, which is semantically equivalent.
- Acceptance tests: `compareCellValuePattern` now captures the full `class="..."` attribute (not just literal `cmp-value`) so parsing still works once cells mix tag/plain/empty classes. The feature-table flattening (`flattenCellValues`) concatenates a value directly (no separator) when it starts with a space - the series row's `" in N"` suffix - and otherwise joins with `", "`, so existing multi-value scenarios (division playoff teams, award finalists) keep their old flattened text unchanged. New tag/plain/faint/note scenarios use small dedicated steps (`the Compare value "..." is tag-styled/is plain text/is faint`, `the Compare note reads "..."`, `no chip is selected on Compare`) that do a `theSheetShows`-style `strings.Contains` against a constructed HTML fragment, in preference to embedding raw HTML/CSS literally in the `.feature` prose.
- Verified with `task go:build` (lint, vet, unit tests, acceptance tests, complexity, licenses, govulncheck, binary build - all green) and `task go:run` (binary builds and starts cleanly). Manual `/compare` click-through with a hand-edited `?set=` was not additionally performed with a live browser session; the HTTP-level behavior (tag/faint CSS classes and the gated note, byte-for-byte) is covered by both the unit-level route tests (`compare_test.go`) and the GoDog acceptance suite, which exercise the same `net/http` handler.

### Pass 2 review fixes (2026-09-25)

- `isGatedRound` now short-circuits on `roundGatedSetIDs[id]`, then re-validates the id's own phase/deadline (same rules `selectableCompareSets` applies) before trusting `PlayoffMatchups`, so a malformed or missing round-gated entry falls back to the default set instead of wrongly showing the note. Added `TestIsGatedRoundShouldReportFalseForARoundGatedIDWithABadDeadlineUnknownPhaseOrNoEntry` pinning down all three cases.
- `compareSeriesGamesSuffix` dropped its literal leading space (`"in %s"` instead of `" in %s"`); the visual gap between a series row's tag and its games suffix now comes only from `.cmp-value`'s existing `margin-right: 6px`, avoiding the previous double-gap (CSS margin plus a literal space character). Updated the unit test, feature scenario, and the acceptance-test's `flattenCellValues`/`compareCellValue` helpers (now CSS-aware, joining a tag directly followed by a plain value with a single space instead of sniffing for a leading space in the text).
- `awardCategories` now checks `ok` from `st.FindAwardFinalists`, matching the `divisionCategories` fix already in this diff.
- Reformatted this spec's own I/O matrix and Review Triage Log tables to padded, equal-width columns.
- Re-verified with `task go:build` (all green, 148/148 acceptance scenarios) and a `task go:run` smoke test (binary starts, serves `/`).

## Spec Change Log

## Review Triage Log

### Pass 1 (2026-09-25)

| #   | Layer               | Finding                                                                                                         | Verdict | Route  | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| --- | ------------------- | --------------------------------------------------------------------------------------------------------------- | ------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | blind, verification | `isGatedRound` fires for ANY still-`Upcoming` set, not just a round-gated one                                   | high    | patch  | Confirmed: `isGatedRound` checks phase/deadline validity + `effectiveUpcoming`, but `effectiveUpcoming` returns the set's own `Upcoming` flag for anything outside `roundGatedSetIDs` (`predict.go`). A before-season set (e.g. `cup`) or `r1` (hand-gated, per its own comment) marked `Upcoming: true` now shows "Matchups not set." instead of falling back — wrong copy for a non-playoff-round id. Verification-gap demonstrated this against the untouched `TestBuildCompareShouldHaveNoTableWhenNoSetIsSelectable`. |
| 2   | blind, edge         | `isGatedRound` re-scans `st.PredictionSets()` independently of `selectableCompareSets`                          | low     | patch  | Duplicated eligibility logic risks drift if the rules change in one place and not the other; the independent second read is also a narrow TOCTOU window if a write landed mid-request. Resolved as a byproduct of fixing #1 by keying off `roundGatedSetIDs` + `PlayoffMatchups` directly instead of re-deriving phase/deadline validity.                                                                                                                                                                                  |
| 3   | blind               | Acceptance-level tag/plain assertions are whole-body `strings.Contains`, not scoped to a row/cell               | low     | reject | Real gap (division-winner row's "FLA" isn't independently exercised since playoff-teams row already renders the same tag; series tag+" in N" pairing/order isn't confirmed) but the corresponding unit tests (`TestBuildCompareShouldTagDivisionPlayoffTeamsAndWinner`, `TestBuildCompareShouldTagTheSeriesWinnerAndKeepTheGamesCountPlain`) already verify this correctly at the unit level; scoping the acceptance step to a row/cell is more than a direct correction.                                                  |
| 4   | blind               | CSS class-name constants duplicated across `compare.go` and `compare_predictions_steps_test.go`                 | low     | reject | An intentional, pre-existing black-box boundary in this same file (e.g. `compareYouLabel`, `compareChipCSS` already hardcode markup independently); not a new pattern introduced by this diff, and sharing constants would cross the deliberate acceptance-suite/production import boundary.                                                                                                                                                                                                                               |
| 5   | blind               | `flattenCellValues`'s "leading space glues to the previous value" convention is an implicit, test-only contract | low     | reject | Test-helper-only; no current category value starts with a space for an unrelated reason, and making it robust (an explicit tag instead of string-prefix sniffing) is more than a direct correction.                                                                                                                                                                                                                                                                                                                        |
| 6   | blind               | `internal/web/README.md`'s Compare bullet is one long run-on sentence                                           | low     | patch  | Real readability issue; fix is a direct split into shorter sentences, no added complexity.                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| 7   | blind               | `sprint-status.yaml` says `in-progress` while the spec says `in-review`                                         | false   | reject | Expected mid-workflow state: step-05 (`sync-sprint-status.md`, target `review`) reconciles this once implementation and this review conclude.                                                                                                                                                                                                                                                                                                                                                                              |

### Pass 2 (2026-09-25)

Ad hoc review after Build's internal review (Pass 1) closed the story. Pass 1's fixes (items 1/2/6) verified correct; items 3/4/5/7 not re-raised.

| #   | Layer               | Finding                                                                                                                                  | Verdict | Route  | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| --- | ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 8   | verification, blind | `isGatedRound` never validates that a round-gated id maps to an existing, well-formed `PredictionSet` before trusting the matchup count  | medium  | patch  | Confirmed: `roundGatedSetIDs[id] && len(st.PlayoffMatchups(id)) == 0` — `PlayoffMatchups` (`store.go`) is a bare map lookup that returns an empty slice for any id, well-formed or not, present or not. Verification-gap empirically demonstrated a round-gated id with a bad deadline or unknown phase wrongly shows "Matchups not set." instead of falling back, violating this spec's own "Unknown/bad id (unchanged)" row. Blind hunter separately flagged the same root gap for an id absent from `PredictionSets()` entirely. |
| 9   | blind               | Series row's tag + " in N" suffix likely renders over-spaced in the browser                                                              | low     | patch  | Confirmed: `.cmp-values .cmp-value` gives every value span (including the tag) `margin-right: 6px`, and the plain suffix's own text starts with a literal space — stacking a 6px CSS gap plus a space character, wider than the single-space "FLA in 5" reading the Boundaries describe. Never manually verified in a browser.                                                                                                                                                                                                      |
| 10  | blind               | `awardCategories` still discards `ok` from `st.FindAwardFinalists`, unlike the just-fixed `divisionCategories` sibling in this same diff | low     | patch  | Confirmed at `p, _ := st.FindAwardFinalists(playerID, award)`. Currently benign (a false `ok` and a genuinely-empty list both yield the same empty-dash fallback), but inconsistent with the `ok`-guard pattern this diff just established elsewhere; same trivial fix.                                                                                                                                                                                                                                                             |
| 11  | blind               | This spec's own markdown tables (I/O matrix, Review Triage Log) aren't column-padded                                                     | low     | patch  | Confirmed — violates the user's own global markdown-style convention (equal-width padded columns). Direct formatting fix, no code involved.                                                                                                                                                                                                                                                                                                                                                                                         |
| 12  | blind               | Spec frontmatter `status: 'done'` while `sprint-status.yaml` reads `review` looks like a premature bump                                  | false   | reject | By design: `bmad-build`'s step-05 sets the spec's own status to `done` once its internal implementation+review concludes, while syncing `sprint-status.yaml` to `review` specifically so a further ad hoc `bmad-code-review` pass (this one) can still run before the story counts as fully done — not a mismatch.                                                                                                                                                                                                                  |
| 13  | blind               | I/O matrix says "3 teams" but the "consistent tag" scenario seeds 4                                                                      | low     | reject | Trivial doc/scenario-count mismatch with no functional impact; fixing it means editing this frozen spec text or the scenario for no behavioral gain.                                                                                                                                                                                                                                                                                                                                                                                |
| 14  | blind               | The 4-team "consistent tag" scenario only independently asserts 2 of the 4 teams tag-styled                                              | low     | reject | Same category as Pass 1 #3 (already-established: unit tests correctly verify this; acceptance-level exhaustiveness is more than a direct correction).                                                                                                                                                                                                                                                                                                                                                                               |

## Design Notes

`compareValueView{Text, CSS}` mirrors the file's existing precomputed-CSS pattern (`compareCellCSS`) instead of template-side conditionals. Series rows return two values per player (tag `TeamID`, plain `" in N"`), since only the abbreviation is tagged. `isGatedRound` is unreachable via any rendered chip (gated rounds get none) — it only matters for a hand-typed URL.

## Verification

**Commands:**
- `task go:build` (from repo root) — expected: lint, vet, unit and acceptance tests pass.
- `task go:run` — expected: app builds and starts; `/compare` shows tags, faint dashes, and (via a hand-edited `?set=` on a gated round) the dashed note.
