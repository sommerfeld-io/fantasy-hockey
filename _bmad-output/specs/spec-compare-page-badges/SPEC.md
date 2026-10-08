---
id: SPEC-compare-page-badges
companions: []
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Compare Page: Badges, Cross-Player Highlight, Neutral Separator

## Why

A pain found by the owner testing the app with three players (sebastian, yzerman, lidstrom). On Compare, the point is spotting who agrees with whom, but predictions render inconsistently (division picks use team badges; Cup and Presidents' Trophy show plain "Team FLA"; awards show plain slugs; series mix a badge with plain text), and nothing connects identical picks across columns. The blue column borders also read as noise rather than structure.

## Capabilities

- **CAP-1**
    - **intent:** A player can read every prediction on Compare as a badge, styled like the team abbreviations in division picks.
    - **success:** For each Compare set (cup, presidents, divisions, awards, playoff cup, rounds), every entered value renders as a badge; no entered value renders as plain text. A series pick renders as a winner badge plus a separate game-count badge ("in 5"). Award finalists show display names (Connor McDavid), not slugs. Empty values stay a faint em dash, not a badge.
- **CAP-2**
    - **intent:** A player can hover, tap, or keyboard-focus a badge and see every badge carrying the same prediction in the same row, across all players' columns, highlighted.
    - **success:** With three players who all picked DET as Atlantic division winner, hovering one of the three DET badges highlights all three; badges with a different value are not highlighted; leaving or unfocusing clears the highlight. A badge never highlights badges in other rows (DET as winner does not light DET as playoff team). A winner badge matches the same winner and an "in N" badge matches the same N.
- **CAP-3**
    - **intent:** The Compare table's column separators are not blue.
    - **success:** No border on the Compare table or its columns uses the ice-blue token; separators use a neutral border token. The signed-in player's column keeps its own-column marker and "You" label.

## Constraints

- Existing acceptance scenarios in `compare-predictions.feature` stay green except where this spec changes the behavior. The scenarios "Full team names never render as a tag" and "A series row shows the winner as a tag plus plain ' in N' text" contradict CAP-1 and must be revised, not deleted silently. The awards scenario must change from slugs to display names.
- Compare is server-rendered with no JS today; the highlight must work without a new dependency and without breaking the page when scripts fail to load.
- Phone-first single layout. Hover has no touch equivalent, so tap and keyboard focus must trigger the same highlight.
- Changes to observable behavior need a Gherkin feature under `src/acceptance-tests/features/` before implementation.

## Non-goals

- Changing which sets, rows, or players Compare shows, or how picks are scored.
- Any change to the Predict or Leaderboard pages.
- Redesigning the "You" marker, which stays as is.
- Highlighting across rows or categories.

## Success signal

The owner opens Compare with three players' picks, hovers DET as division winner, and sees all three players' DET badges light up and nothing else; no blue column lines remain.

## Assumptions

- Two badges match when they show the same value (team, finalist, series winner, or game count) in the same row.
- Series highlight is per badge: the winner badge and the "in N" badge match independently.
