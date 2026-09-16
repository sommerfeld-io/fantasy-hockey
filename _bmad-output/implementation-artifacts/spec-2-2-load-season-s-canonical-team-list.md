---
title: 'Load Season''s Canonical Team List'
type: 'feature'
created: '2026-09-16'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'c4f98307ff0d83205b3bb5f1b67e1356831aaff7'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Stories 2.3 (Cup champion / Presidents' Trophy) and 2.4 (Division picks) both need the current season's 32 NHL teams, grouped by conference/division, to populate their dropdowns/chips — but no canonical team data exists anywhere in the app yet.

**Approach:** Add a hand-maintained `teams` section to `fantasy-hockey.yml` (32 real teams with conference/division), expose it through a new read-only `Store.Teams()` accessor mirroring `Season()`/`PredictionSets()`'s existing pattern, and add a small reusable `internal/web` helper that converts a team list into the shared `{"id", "label"}` embed shape (AD-19) so 2.3/2.4 can call it directly instead of re-deriving it. A minimal acceptance scenario guards that the new `teams:` section doesn't break app startup/parsing — decided 2026-09-16 (sebastian): add it even though this story exercises no new observable behavior.

## Boundaries & Constraints

**Always:** `teams` is hand-maintained directly in `fantasy-hockey.yml` (AD-23) — `internal/store` only ever reads it, mirroring `Season()`'s read-only, no-write pattern exactly. Every team's `id` is its standard 3-letter abbreviation (AD-17), never a UUID. `Store.Teams()` returns a defensive copy, like `PredictionSets()`. The shared embed-shape helper produces exactly `{"id": "<abbreviation>", "label": "<display name>"}` per AD-19, so any future embedding site (2.3, 2.4, 2.5) reuses the identical shape and object.

**Never:** No Prediction sheet, dropdown, or chip UI — no route or template changes. This story only makes the data and the shape-conversion helper available for 2.3/2.4 to consume; it renders nothing itself (mirrors 2.1's "Submitted" CSS existing before it was reachable). No admin/write path for editing teams — `internal/store`'s writers stay scoped to predictions and login codes (AD-9).

</frozen-after-approval>

## Code Map

- `src/internal/store/store.go:50-63` (`PredictionSet`, `document`) -- add a `Team` struct (`ID`, `Name`, `Conference`, `Division`, all `yaml`-tagged strings) and `document.Teams []Team \`yaml:"teams"\`` next to `PredictionSets`.
- `src/internal/store/store.go:150-162` (`Store.PredictionSets`) -- add `Store.Teams()` immediately after, following the identical read-lock + defensive-copy pattern.
- `src/internal/store/store_test.go:195-260` (`TestPredictionSetsShouldReturnTheSeededList` and siblings) -- structural pattern to follow for `Teams()`'s own tests (seeded list, empty list, copy-safety).
- `src/internal/web/web.go:111-152` (`predictSetView`, `newPredictSetView`) -- structural pattern for a small unexported view/option type + constructor; add `teamOption{ID, Label string}` (with `json` tags) and `newTeamOptions(teams []store.Team) []teamOption` nearby. Not called from any handler yet — 2.3/2.4 will call it.
- `src/fantasy-hockey.yml` -- add a `teams:` section (32 entries) after `prediction_sets`, sourced from the UX click-dummy's `DIVISIONS` map (`_bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-14/imports/faceoff-pool-source/fantasy-hockey/src/App.jsx:35-72`), the same authoritative reference Story 2.1 used for its placeholder data — real team names/abbreviations, not placeholders.
- `src/acceptance-tests/app_shell_steps_test.go` + `src/acceptance-tests/features/app-shell.feature` -- structural pattern (`Background:` seeded session/store, scenario-state struct, GoDog step wiring, `InitializeXScenario` registered in `suite_test.go`) to follow for this story's own feature file, since no existing acceptance fixture builder is shared across all features.

## Tasks & Acceptance

**Execution:**
- [x] `src/internal/store/store.go` -- add `Team` struct, `document.Teams`, `Store.Teams()` read-only accessor -- unit tests: populated list (32 teams, correct conference/division on a sample), empty list, returned slice is a defensive copy
- [x] `src/internal/web/web.go` -- add `teamOption` type + `newTeamOptions` helper -- unit test: converts a `[]store.Team` into JSON matching `{"id": "<abbr>", "label": "<name>"}` for every entry, preserving order
- [x] `src/fantasy-hockey.yml` -- seed `teams:` with all 32 real NHL teams (id = abbreviation, name, conference, division), grouped Atlantic/Metropolitan (Eastern) and Central/Pacific (Western)
- [x] `src/acceptance-tests/features/load-canonical-team-list.feature` + `src/acceptance-tests/load_canonical_team_list_steps_test.go` -- one scenario: given a data file whose `teams` section holds a representative sample of the season's canonical teams (4 teams, one per division), when the app starts and a logged-in player requests `/predict`, then the app starts without error and the response is still 200 -- registered in `suite_test.go`'s `ScenarioInitializer` -- must fail before the `teams:` parsing/struct exists (BDD red), then pass once implemented

**Acceptance Criteria:**
- Given `fantasy-hockey.yml`'s hand-maintained `teams` section, when the app starts or any code calls `Store.Teams()`, then it returns all 32 teams with conference/division intact, and no code path in this story ever writes to that section.
- Given a `[]store.Team` passed to `newTeamOptions`, when it runs, then every resulting entry has exactly `id` (the team's abbreviation) and `label` (its display name), matching the shape every other embedding site (2.3, 2.4, 2.5) will reuse.
- Given a data file whose `teams` section is present and well-formed, when the app starts, then it starts without error (parsing 32 real teams doesn't break `store.New`).

## Implementation Notes

- `Team.Conference`/`Team.Division` use full capitalized display strings ("Eastern"/"Western", "Atlantic"/"Metropolitan"/"Central"/"Pacific"), matching FR-21's "Eastern Conference"/"Western Conference" phrasing and the click-dummy's `label` values (`App.jsx`'s internal keys like `metro`/`east` are lowercase, but those are UI-internal, not the display strings this store field carries).
- `newTeamOptions` is unit-tested directly (JSON-shape assertion) but not yet exercised by any acceptance scenario, since no handler calls it in this story (per Boundaries: "renders nothing itself") -- `acceptance-coverage.out` correctly shows 0% for both `Store.Teams()` and `newTeamOptions` for that reason; unit coverage for both is 100%.
- The acceptance scenario's Given/When/Then step text was deliberately phrased differently from `app_shell_steps_test.go`'s near-identical "the player ... is signed in" / "... visits the ... destination" steps (e.g. "the seeded player ... has a valid session", "the player requests the Predict destination") to avoid two identical GoDog step regexes being registered in the same suite (`suite_test.go` registers every feature file's `Initialize*Scenario` into one shared `ScenarioContext`).
- Verified with `task go:test` (new `store`/`web` unit tests green, 100% coverage on `Teams`/`newTeamOptions`), `task go:test:acceptance` (45 scenarios passed, including the new one), `task lint` (clean after fixing a `gocyclo` complexity finding in the first draft of `TestTeamsShouldReturnTheSeededList`), `task go:build` (full pipeline green, no vulnerabilities), and a manual run of the built binary confirming `GET /login` returns 200 with the real `fantasy-hockey.yml`'s new 32-team `teams:` section loaded.

## Spec Change Log

## Review Triage Log

- **false** — Blind Hunter: the three new-file diff hunks carry malformed headers (duplicate `+++` lines, mixed `a/`/`b/` vs `c/`/`w/` prefixes). Verified: this is an artifact of how the review's diff file was hand-assembled for untracked files in this pass, not a property of the actual repository — `spec-2-2-...md`, `load-canonical-team-list.feature`, and `load_canonical_team_list_steps_test.go` all parse, lint, and test cleanly in the working tree.
- **low, rejected** — Blind Hunter + Edge Case Hunter (`src/internal/store/store.go:188-195`): no validation that `Team.ID` is non-blank or unique, and no validation of `Conference`/`Division` against a fixed value set. Reachable only via a hand-edit mistake in `fantasy-hockey.yml`'s `teams:` section, data mechanically sourced verbatim from the click-dummy reference (low likelihood of a typo). The fix would add new guard/dedup logic not demonstrated as necessary by any failing test — same grounds Story 2.1's own review already rejected the identical class of finding for `prediction_sets` ids (`spec-2-1-browse-prediction-sets-by-phase-and-status.md`'s Rejected section: "reachable only via a hand-edit mistake... the correct fix adds new guard/dedup logic").
- **low, rejected** — Blind Hunter + Edge Case Hunter (`src/internal/web/web.go:221-227`): `newTeamOptions` has no guard skipping a `Team` with a blank id/name, unlike `buildPredictPhases`'s skip-malformed-row pattern for `PredictionSet`. Verified: `PredictionSet`'s skip logic exists specifically to guard a `time.Parse` failure on `deadline_utc`; `Team`'s fields have no equivalent parse step, and `PredictionSet`/`Player` already carry zero id-uniqueness validation in this codebase (see the row above) — `Team` matches the existing convention rather than deviating from it. Same rejection grounds as the row above (same root cause).
- **false** — Blind Hunter: no test loads the actual shipped `src/fantasy-hockey.yml` and asserts `Store.Teams()` returns exactly 32 unique teams. Verified: this is a pre-existing pattern already true for `Season`/`Players`/`PredictionSets` in the same file (no test anywhere loads the real file for those either) — independently confirmed by the Verification Gap reviewer's own trace, which found this "a legacy condition the diff extends rather than newly regresses." This repo's own verification workflow (CLAUDE.md) treats `task go:run` against the real file as the accepted manual check, which the spec's Verification section already documents.
- **false** — Blind Hunter: the acceptance scenario only asserts `GET /predict` returns 200, never the team data's count/shape/uniqueness, so the scenario is weaker than its Gherkin title implies. Verified: the frozen `<frozen-after-approval>` Intent explicitly scopes this to "guards that the new `teams:` section doesn't break app startup/parsing" — a narrower claim than the title's plain-English phrasing, and the feature file's own step comment documents this narrowing. Matches the human-approved decision to add a minimal acceptance scenario "even though this story exercises no new observable behavior."
- **low, rejected** — Blind Hunter: `Team.Conference`/`Team.Division` are bare `string` fields with no enum/const type or validated value set, a future typo risk once 2.3/2.4/2.5 filter/group by them. No consumer of these fields exists in this story yet (`newTeamOptions` only reads `ID`/`Name`) — adding enum/const infrastructure now is speculative ahead of a demonstrated need, and the fix is more than a direct correction.
- **reject** — Blind Hunter: the Implementation Notes' claim that `TestTeamsShouldReturnTheSeededList`'s first draft tripped `gocyclo` may be stale/unverifiable documentation. Fix is editing the spec's Implementation Notes text — out of scope per this workflow's own triage rule ("reject any finding whose fix is to edit this build's spec").
- **reject** — Blind Hunter: the spec's `## Verification` → Commands list omits `task lint`/`task go:test:acceptance`/`task go:build`, which Implementation Notes says were also run. Fix is editing the spec's Verification section — out of scope per the same triage rule.
- **low** — Blind Hunter + Verification Gap (`src/acceptance-tests/load_canonical_team_list_steps_test.go:516-650` vs `src/fantasy-hockey.yml`): the full 32-team roster is duplicated verbatim between the production data file and the acceptance test's fixture, with nothing keeping them in sync — a future roster edit (relocation, rebrand, expansion team) could silently diverge from the fixture undetected. Verified: no test reads the real file, and the acceptance scenario only needs to prove `store.New` parses a well-formed `teams:` section without breaking startup — it doesn't need the full 32-entry copy to do that. Route: patch — shrink the fixture to a small representative subset (e.g. 3-4 teams spanning divisions) instead of duplicating the real file's exact contents. **Fixed 2026-09-16**: `loadCanonicalTeamListSeed` now seeds 4 teams (TOR/WSH/COL/VGK, one per division); the feature file's Given step was reworded to "a data file whose teams section holds a sample of the season's canonical teams" so it no longer claims all 32.

## Verification

**Commands:**
- `task go:test` -- expected: new `store`/`web` unit tests pass, coverage report unchanged elsewhere
- `task go:run` -- expected: app builds and starts with the new `teams:` section present

**Manual checks (if no CLI):**
- Inspect `src/fantasy-hockey.yml`'s new `teams:` section: exactly 32 entries, correct abbreviation/division/conference per team, no duplicates.
