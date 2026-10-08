---
title: 'Compare page: badges, cross-player highlight, neutral separator'
type: 'feature'
created: '2026-10-08'
status: 'done'
route: 'dispatch'
baseline_commit: 'eef2a8615c4ae54663788854adbcc838c26a0305'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-compare-page-badges/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Compare renders predictions inconsistently (Cup/Presidents as plain "Team FLA", series "in N" as plain text), nothing connects identical picks across players, and the own column has blue borders that read as noise.

**Approach:** Render every entered value as a `cmp-tag` badge carrying a match key. A small dependency-free `compare.js` highlights same-key badges in the same row on hover, tap, or keyboard focus. The own column's borders switch to the neutral border token.

## Boundaries & Constraints

**Always:** Empty values stay a faint em dash (not a badge, never highlighted). Highlight is same row only, across all columns; the winner badge and the "in N" badge match independently. Page renders and works without JS (no highlight). The "You" label and own-column classes stay. Award finalists show display names.

**Never:** Change which sets, rows, or players Compare shows, or scoring. Touch Predict or Leaderboard. Add a dependency. Highlight across rows. Edit `Dockerfile` or `.github/workflows/**`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Cup / Presidents pick | Sadl picked FLA | Badge "Team FLA", match key "Team FLA" | N/A |
| Series pick | Sadl picked FLA in 5 | Badge "FLA" (key "FLA") plus badge "in 5" (key "in 5") | N/A |
| Same pick, 3 players | All picked DET as Atlantic winner | All three badges share key "DET"; hovering one highlights all three | N/A |
| Same team, other row | DET as winner and as playoff team | Different rows; highlight never crosses rows | N/A |
| Empty | No pick | Faint "—", no badge, no match key | N/A |
| JS missing | Script fails to load | Table renders normally, no highlight | N/A |

</frozen-after-approval>

## Code Map

- `src/internal/web/compare.go` -- `compareValueView` (Text, CSS), `tagValue`/`tagValues`/`plainValue`/`emptyCompareValue`, `singleTeamCategory` (plain team name today). Add a `Match` key to the view; `plainValue` has no non-test callers left after this change and is removed with its CSS constant if unused.
- `src/internal/web/compare_awards.go`, `compare_divisions.go`, `compare_series.go` -- each builds values via `plainValue`/`tagValue`; switch awards, cup/presidents, and the "in N" value to badges.
- `src/internal/web/templates/shell.html` -- `compare-table` template line 62 renders each value span; add `data-match` and `tabindex="0"` for badges only, and load `compare.js` in the Compare branch (pattern: `sheet.html:115`, `<script src="/static/divisions.js" defer>`).
- `src/internal/web/static/styles.css` -- `.cmp-tag` (~1236), `.cmp-player--own`/`.cmp-cell--own` (~1220, blue 2px borders), header comment ~1117. Add a match-highlight class and focus style.
- `src/internal/web/static/` -- new `compare.js` next to `divisions.js`/`awards.js` (embedded via `go:embed static`).
- `src/internal/web/compare*_test.go` -- assert on `compareValueTagCSS`/`compareValueCSS`; update.
- `src/acceptance-tests/features/compare-predictions.feature`, `compare_predictions_steps_test.go` -- value regexes (`compareCellValuePattern`, `compareValuePlainCSS`) assume the current span markup; adjust together with the markup change. The seeded finalist display names are "Player <slug>", so the awards scenario's expected cell text is unaffected by the slug-to-display-name wording in SPEC.md.

## Tasks & Acceptance

**Execution:**
- [x] `src/acceptance-tests/features/compare-predictions.feature` -- revise "Full team names never render as a tag" and the series scenario to badges; add scenarios for same-key badges in a row (three players, DET), no cross-row key reuse, empty value not a badge, own column has no ice-blue border -- BDD red first
- [x] `src/acceptance-tests/compare_predictions_steps_test.go` -- step definitions and markup patterns for the new scenarios -- supports the feature
- [x] `src/internal/web/compare*_test.go` -- unit tests: every entered value is a tag with a match key, empty is not, "in N" is its own badge -- TDD
- [x] `src/internal/web/compare*.go` -- add match key, make all entered values badges -- CAP-1
- [x] `src/internal/web/templates/shell.html` -- emit `data-match`/`tabindex`, load `compare.js` -- CAP-1/2
- [x] `src/internal/web/static/compare.js` -- on mouseover, focusin, and click of a badge, add the highlight class to every badge with the same `data-match` inside the same `tr`; clear on mouseout, focusout, and a click elsewhere -- CAP-2
- [x] `src/internal/web/static/styles.css` -- highlight and focus styles; own-column borders use `var(--border)`; update the header comment -- CAP-2/3

**Acceptance Criteria:**
- Given three players who all picked DET as Atlantic winner, when Compare renders, then the three DET badges carry the same match key and no other badge in that row does.
- Given the same team appears in a winner row and a playoff-teams row, then the two rows share no highlight scope (the script only scans the badge's own row).
- Given the Compare page, when its CSS is inspected, then no `.cmp-*` rule references `var(--ice)` for a border, and the own column still renders the "You" label.

## Implementation Notes

- Match key is the pick's identity (team id, finalist slug, "in N"), not its display text.
- `plainValue` and `compareValueCSS` removed; every entered value is a `cmp-tag` badge with `data-match` and `tabindex="0"`.
- Own-column borders are 1px `var(--border)`; the "You" label and own classes stay.
- JS highlight (`static/compare.js`) is verified manually only.

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Route | Evidence |
|---------|---------|-------|----------|
| Key by display text lets two distinct finalists with one name highlight together (Blind, Edge) | medium | patch | Real: same-name NHL players exist. Fixed: finalists key on slug, Cup/Presidents on team id (`keyedTagValue`). |
| JS highlight has no automated test (Verification Gap) | medium | defer | Real, but repo has no JS harness; awards.js/divisions.js have the same coverage. Logged in deferred-work.md. |
| `theValueIsNotABadge` passes vacuously when the text is absent (Edge) | low | patch | Real test weakness; step now errors when the value is not found. |
| Hover/click/tap interplay, flicker between adjacent badges (Blind, Edge) | low | rejected | Tap leaves the highlight until next tap elsewhere; mouse leave clears. Matches the spec triggers; fix needs pinning state. |
| Key collision between "in N" and team (Blind) | false | rejected | A row's badges are team ids or "in N" text; no team id has the form "in 5". |
| Many tab stops, colour-only highlight, contrast (Blind, Verification Gap) | low | rejected | Spec requires keyboard focus on every badge; highlight contrast not shown to fail. |
| Own column weakened to neutral 1px border; ice-border test is a regex (Blind, Edge, Gap) | low | rejected | Spec CAP-3 requires neutral separators and keeps class plus "You" label. |
| Script tag inside fragment, duplicate listeners (Blind, Edge) | false | rejected | One compare-table per page, so one script tag. |
| "in 0" badge, empty text badge (Edge) | false | rejected | Games validated 4 to 7 on write; display names fall back to the slug or id. |
| `data-match` escaping (Blind) | false | rejected | html/template escapes attributes; regex tolerates entities. |
| `flattenCellValues` text sniffing, stale comments, seed divergence, no cache busting (Blind, Gap, Edge) | low | rejected | Test-helper or cosmetic, no demonstrated failure. |

## Design Notes

The match key is the badge text (team id, "Team FLA", display name, or "in N"); keys cannot collide across kinds within one row because a row holds one kind. The godog suite asserts server-rendered markup only, so the JS highlight is verified manually in a browser.

## Verification

**Commands:**
- `task go:test` -- expected: all unit tests pass
- `task go:test:acceptance` -- expected: all scenarios pass
- `task go:run` -- expected: app builds and starts (govulncheck-only failures are acceptable per CLAUDE.md)
- `task docker:build` -- expected: image builds and lints

**Manual checks (if no CLI):**
- Open Compare with the three-player data (sebastian, yzerman, lidstrom): hover, tap, and Tab to a DET division-winner badge and see all three DET badges light up and nothing else; no blue column lines remain.
