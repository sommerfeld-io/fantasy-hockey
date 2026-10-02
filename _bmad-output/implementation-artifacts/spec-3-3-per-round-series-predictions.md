---
title: 'Per-Round Series Predictions'
type: 'feature'
created: '2026-09-24'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '1e6cf9295c7262d55be3c99ea473c73b9c7f8d3a'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Round 1, Round 2, the Conference Finals, and the Stanley Cup Final (`r1`/`r2`/`cf`/`scf`) are Prediction Sets today, but none are pickable — every one renders the generic static stub, even once a round is unlocked (Story 3.2) or has its matchups recorded.

**Approach:** Give all four ids a new "series" pick-entry sheet, reusing the read-only `playoff_matchups` data Story 3.2 already introduced. For each recorded matchup, render a series card with two full-width team buttons (winner) and four number buttons 4/5/6/7 (game count), all sharing identical selected styling. Cards group under "Eastern Conference"/"Western Conference" subheaders for every round except the Final, which groups under "Stanley Cup Final" instead. One Update/Submit button saves the whole sheet, mirroring the divisions/awards pattern; the winner and game-count within one series save together as a single row, never split. A half-filled series (team chosen but no game count, or vice versa) is simply not saved and shows no error — every other complete series in the same submission still saves, extending the "an empty series pick never blocks other picks" rule to "incomplete" as well as "empty."

</frozen-after-approval>

## Code Map

- `internal/store/store.go:194-197` (`PlayoffMatchup`) — add `Key string `yaml:"key"`` (hand-maintained per-series identity, e.g. `"s1"`; avoids the reordering-in-YAML data-corruption risk a list-position-derived key would carry). `TeamA`/`TeamB` stay as-is.
- `internal/store/store.go:77-103` (Kind consts) — add `KindSeries = "series"`, doc-commented like `KindDivisionPlayoffTeams`: a row is keyed by `(PlayerID, Kind, SeriesKey)`, never `(PlayerID, Kind)` alone.
- `internal/store/store.go:129-146` (`Prediction`) — add `SeriesKey string `yaml:"series_key,omitempty"`` (full key, e.g. `"r1.s1"` — Prediction Set id + a separator const + `PlayoffMatchup.Key`) and `Games string `yaml:"games,omitempty"`` (one of `"4"`/`"5"`/`"6"`/`"7"`, kept a string for consistency with every other `Prediction` field). Both zero-valued on every non-series row.
- `internal/store/store.go` — add `Store.FindSeriesPick(playerID, seriesKey string) (Prediction, bool)` and `Store.SaveSeriesPick(playerID, seriesKey, teamID, games string, now time.Time) error`, mirroring `FindDivisionPlayoffTeams`/`SaveDivisionPicks`'s upsert-by-extra-key pattern. Store does not validate `teamID`/`games` — AD-10's server-revalidation happens in `internal/web`, matching every existing pick kind.
- `internal/web/sheet.go:13-23` (`divisionsSetID`/`awardsSetID`) — add a `seriesSetIDs` map (`r1`/`r2`/`cf`/`scf`) alongside; add all four to `pickableSheetKinds` (line ~31-37).
- `internal/web/sheet.go:122-131` (`sheetData`) — add `SeriesPick *seriesPickView`.
- `internal/web/sheet.go:161-179` (`newSheetData` switch) — add a case for `seriesSetIDs[set.ID]`.
- New `internal/web/sheet_series.go` (mirrors `sheet_divisions.go`/`sheet_awards.go`'s one-file-per-kind split): series-key helpers (separator const + join/split), `seriesCardPick`/`seriesGroupPick`/`seriesPickView` types, `newSeriesPickView` (groups each set's matchups by `Team.Conference` into "Eastern Conference"/"Western Conference", or a single "Stanley Cup Final" group when `set.ID == stanleyCupFinalSetID`), `parseSeriesSubmission`, `handleSeriesSubmit`, `renderRejectedSeriesPick`. Unlike `sheet_awards.go`'s whole-form gate (any invalid slot blocks the entire POST), a series submission is per-series independent: a fully or partially blank series is simply not saved and never blocks another series in the same POST; only a non-blank `team_id` that matches neither of the series' two teams, or a non-blank `games` outside 4-7, rejects — and only that series, re-rendering the sheet with an inline error on that card while every other valid series in the same POST still saves.
- `internal/web/predict.go:27-40` (`roundGatedSetIDs`) — unchanged; `r1` stays excluded from matchup-based Upcoming-gating exactly as Story 3.2 left it (its own `upcoming`/deadline flag governs it, per the epics AC only naming `r2`/`cf`/`scf` for matchup-gating).
- `internal/web/templates/sheet.html` — add a `{{else if .SeriesPick}}` block (existing `.Pick`/`.DivisionPick`/`.AwardsPick` chain is the pattern), intro copy "For each series, tap the winner and how many games it takes.", grouped by `.SeriesPick.Groups[].Label`.
- `internal/web/static/styles.css` — add a full-width team-button/number-button class pair sharing `.chip:has(input:checked)`'s `sel`/`ice` selected-state rule (`~line 651`), per DESIGN.md: "no separate color for this versus the team-winner buttons next to it."

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/store.go` -- add `PlayoffMatchup.Key`, `KindSeries`, `Prediction.SeriesKey`/`Games`, `FindSeriesPick`/`SaveSeriesPick` -- new read-write kind, read-only matchup key.
- [x] `internal/store/store_test.go` -- tests for the new fields' YAML round-trip and `FindSeriesPick`/`SaveSeriesPick` (create, update-in-place, distinct series don't collide, distinct players don't collide).
- [x] `internal/web/sheet_series.go` -- series-key join/split helpers, view types, `newSeriesPickView`'s conference/final grouping, `parseSeriesSubmission`, `handleSeriesSubmit`, `renderRejectedSeriesPick`, per the per-series-independent validation policy in the Code Map.
- [x] `internal/web/sheet.go` -- `seriesSetIDs`, `pickableSheetKinds` additions, `sheetData.SeriesPick`, `newSheetData` case.
- [x] `internal/web/templates/sheet.html` -- `.SeriesPick` render block.
- [x] `internal/web/static/styles.css` -- shared selected-state class for team/number buttons.
- [x] `internal/web/sheet_series_test.go` -- unit tests: grouping (r1's 8 series split 4/4 Eastern/Western by `Team.Conference`; `scf`'s single series under "Stanley Cup Final"), a valid submission saves TeamID+Games atomically, a half-filled series (only team_id or only games) is silently not saved and doesn't block other series in the same POST, an invalid `team_id` (not one of the matchup's two teams) or invalid `games` (outside 4-7) rejects only that series with an inline error while other valid series in the same POST still save, resubmission updates the existing row in place, the Upcoming/Closed/pickable gates from Stories 2.1/3.2 all still apply unchanged.
- [x] `acceptance-tests/features/series-predictions.feature` + steps -- Given/When/Then for: series render grouped correctly per round, a valid winner+games pick saves and shows Submitted, a half-filled or invalid series pick doesn't block another valid series in the same submission, a closed round shows read-only.

**Acceptance Criteria:**
- Given `r1`'s 8 recorded matchups, when its sheet renders, then exactly 8 series cards appear, grouped 4 under "Eastern Conference" and 4 under "Western Conference".
- Given `scf`'s one recorded matchup, when its sheet renders, then one series card appears under a "Stanley Cup Final" heading, not a conference subheader.
- Given one series card, when a team button and a game-count button are both tapped and the sheet submitted, then both save together in one `Prediction` row keyed by `(PlayerID, KindSeries, SeriesKey)`.
- Given one series submitted with only a team_id or only a game count, or a team_id matching neither of the series' two teams, or a game count outside 4-7, when submitted alongside other complete, valid series, then nothing is saved for that one series but every other complete, valid series in the same submission still saves.

## Design Notes

`PlayoffMatchup` gains a hand-maintained `Key` field rather than deriving a series' identity from its list position — the architecture doc's own illustrative shape already implies a stable, human-readable per-series key (`round1.s1`), and a position-derived key would silently reassign an already-saved pick to a different series if a human ever reorders `playoff_matchups` entries by hand. The full `SeriesKey` stored on a `Prediction` row joins the Prediction Set id and this key (e.g. `"r1.s1"`), via one package-level separator constant — never hand-typed per call site (magic-value rule).

Round grouping needs no hand-maintained conference/round data: a series' two teams already carry `Team.Conference` (existing field), so Eastern/Western grouping falls out of the matchup data itself, exactly like `groupDivisionsByConference` already does for Story 2.4. No round gets a hardcoded expected series count anywhere in code — the sheet renders however many matchups are actually recorded for that Prediction Set id; "exactly 8 for `r1`" is a data-entry expectation on the human maintaining `fantasy-hockey.yml`, not a code-level assumption.

## Verification

**Commands:**
- `task go:test` -- expected: all unit tests pass, including new store/sheet_series cases.
- `task go:test:acceptance` -- expected: new `series-predictions.feature` scenarios pass alongside the existing suite.
- `task go:run` -- expected: lint, vet, full test suite, complexity, licenses, vulncheck all clean; server starts.

## Implementation Notes

- `store.SaveSeriesPick(playerID, seriesKey, teamID, games string, now time.Time) error` upserts one `(PlayerID, KindSeries, SeriesKey)` row per call, mirroring `SavePrediction`'s single-row rollback-on-write-failure pattern rather than `SaveDivisionPicks`/`SaveAwardPicks`'s whole-batch-snapshot pattern - `handleSeriesSubmit` calls it once per complete, valid series within its own per-series loop, so each series' own save is atomic but the whole POST is not a single write. This matches the Intent's "the winner and game-count within one series save together as a single row" (row-level atomicity) without requiring the stronger, unrequested "whole submission is one write" guarantee divisions/awards need for their own all-or-nothing gates.
- **Necessary addition beyond the Code Map:** `internal/web/sheet.go`'s `setSubmitted` needed a new `seriesSetIDs[set.ID]` case (checking whether any of the round's recorded matchups has a saved `FindSeriesPick` row). Without it, every series-pickable id would permanently read "not submitted" - `setSubmitted`'s existing default branch (`st.FindPrediction(playerID, set.ID)`) can never match a `KindSeries` row, since those rows carry `Kind == store.KindSeries`, not `Kind == set.ID`. This is required for the "shows Submitted" behavior in both the AC and the acceptance feature; the Code Map's own file-by-file list didn't call it out, but the Intent/AC are the source of truth per this task's instructions.
- **Validation semantics refined during implementation:** the Code Map's wording could be read as "half-filled OR invalid values" both landing in one "reject with inline error" bucket. Re-reading the frozen Intent block strictly ("a half-filled series ... shows no error") led to three distinct submission states instead of two: blank (skip, no error), half-filled - exactly one of team_id/games set (skip, no error, regardless of whether the one set value would itself be valid), and both-fields-set-but-invalid (skip, inline error). Only the third state ever renders `seriesErrorText`. This is implemented as `seriesSubmissionIsBlank`/`seriesSubmissionIsHalfFilled`/`seriesSubmissionIsComplete` in `internal/web/sheet_series.go`, each covered by its own store/web unit and acceptance test.
- Series winner/game-count buttons render as visually-hidden radio inputs inside `<label class="series-btn">` wrappers (one radio group per series for the team, one for the game count), reusing `.chip:has(input:checked)`'s selected-state approach under new `.series-btn`/`.series-team-btn`/`.series-games-btn` classes sharing the same `--sel`/`--ice` tokens - no JavaScript file was needed (unlike divisions.js/awards.js), since native radio-group semantics already give "exactly one winner, exactly one game count" for free and there is no live cap/autocomplete behavor to compute client-side.
- `internal/web/sheet.go`'s `handleSheetSubmit` was refactored to extract `dispatchSheetSubmitByKind` (routing to `handleDivisionsSubmit`/`handleAwardsSubmit`/`handleSeriesSubmit`) purely to keep `handleSheetSubmit` under the project's `gocyclo -over 10` gate once the new series branch was added (it would otherwise have hit 11); this is a pure extraction with no behavior change, verified by the full existing test suite passing unchanged.
- Story 3.3 made every remaining Epic 3 Prediction Set id (`r1`/`r2`/`cf`/`scf`) real, so the repo's established "next remaining stub id" test convention (used by Stories 2.3/2.4/2.6/3.1 to repoint their own stub-proof tests) had no genuine future id left to repoint onto. Introduced a synthetic `"mystery-set"` id (never added to `pickableSheetKinds`) in its place, updating `internal/web/sheet_test.go`'s three affected tests and the two affected acceptance scenarios (`browse-prediction-sets.feature`, `cup-and-presidents-picks.feature`) that previously used `"r1"` as their non-pickable example.
- `PlayoffMatchup.Key` and `Prediction.SeriesKey`/`Games` are additive, `omitempty`-guarded fields; no existing seeded fixture or the real `fantasy-hockey.yml` (which has no `playoff_matchups` section yet) needed any change - confirmed by the full pre-existing test suite passing unchanged before any new tests were added.
- `splitSeriesKey` (the reverse of `joinSeriesKey`) is exercised only by its own dedicated round-trip unit test today; production code only ever joins a known `(setID, key)` pair (both already in hand wherever a `SeriesKey` is needed) and never needs to decompose one. Kept per the Code Map's explicit "join/split helpers" ask, as a documented, tested inverse guarding the `"r1.s1"` key format against an accidental separator change.
- Verification: `task go:test` (all unit tests pass, `internal/web` at 95.9% statement coverage including every new `sheet_series.go` function), `task go:test:acceptance` (101 scenarios / 605 steps pass, including this story's 7 new `series-predictions.feature` scenarios), and `task go:run` (lint, vet, gocyclo, go-licenses, govulncheck all clean, no vulnerabilities found, server starts) all pass clean. `task docker:build` was also run as the authoritative final check.
- Nothing was left incomplete. `fantasy-hockey.yml` itself was intentionally left unedited, per the Design Notes ("a data-entry expectation on the human maintaining `fantasy-hockey.yml`, not a code-level assumption").

## Review Triage Log

- **medium, patch** (verification-gap, pre-verified): no test seeds an existing complete series pick and then submits a half-filled or invalid resubmission for that same series, asserting the original pick is unchanged. Current code is correct (the blank/half-filled/invalid branches never call `SaveSeriesPick`), but a future regression merging those branches with the complete-and-valid one could silently overwrite or erase an already-saved pick, and nothing would catch it. Fix: add a test seeding a prior pick, then a half-filled/invalid resubmission for the same series, asserting it's unchanged.
- **medium, patch** (blind-hunter): `round-unlocking.feature`'s pre-existing "cf" matchup fixture (`round_unlocking_steps_test.go`) writes only `a:`/`b:`, no `key:`. Since this story routes `cf` through the new series sheet, that fixture now produces a malformed `data-series=""` card with an empty `SeriesKey` - the test still passes (it only checks title/status), but the fixture no longer reflects valid data shape. Fix: add a `key:` to that one fixture line.
- **medium, patch** (blind-hunter + edge-case-hunter, same finding): a series-pickable set with zero recorded matchups (e.g. `r1` right after it opens, before a human enters the bracket - `r1` is deliberately ungated by `playoff_matchups`, per Story 3.2) renders just the hint text and a live Submit button with no cards and no explanation. Fix: render an explicit "no series recorded yet" message when `len(.SeriesPick.Groups) == 0`, mirroring the stub page's own "Not available yet." tone.
- **low, defer** (edge-case-hunter): `handleSeriesSubmit`'s per-series loop isn't atomic across the whole POST - if `SaveSeriesPick` fails partway through (a disk write error), series saved earlier in the same loop stay saved while the client only sees a generic 500 with no indication which succeeded. This is the Implementation Notes' own deliberate, documented trade-off (row-level atomicity, not whole-POST atomicity, matching the frozen Intent's literal wording), and a disk-write failure mid-request is the same class of low-likelihood operational fault already accepted elsewhere in this codebase (see `deferred-work.md`'s story-1-1 entries on timing side-channels and unguarded async sends) at this app's declared hobby scale. A proper fix would need a stronger batch-write primitive this story deliberately chose not to build.
- **low, reject** (blind-hunter): only `r1`/`scf` get direct series pick-entry test coverage; `r2`/`cf` are only exercised via `round-unlocking.feature`'s gating scenarios, never their series-card rendering/submission. Rejected: `sheet_series.go` has no per-id branching beyond the already-tested `stanleyCupFinalSetID` special case - every other id (including `r2`/`cf`) hits the identical `default: groupSeriesByConference` path `r1`'s 17 tests already exercise in full, so an `r2`/`cf`-specific test cannot discover anything those don't already cover.
- **low, reject** (blind-hunter): `teamRoster(st)` is rebuilt from scratch once per matchup and again once per card (up to ~16 redundant map builds per request for `r1`). Rejected: negligible at this app's declared hobby scale (a handful of in-memory 32-entry map builds), and threading a pre-built roster through would add parameters across multiple function signatures for no measurable benefit.
- **low, reject** (blind-hunter): `splitSeriesKey` is unused in production code, only exercised by its own round-trip test. Rejected: explicitly requested by this spec's own (non-frozen) Code Map as a documented, tested guard against an accidental separator-format change; harmless, and removing it trades a real (if small) regression guard for no functional benefit.
- **defer** (blind-hunter): series cards have no `aria-describedby` linking an error to its inputs, and group under one `<fieldset>` per conference rather than per series. Consistent with this app's existing baseline elsewhere (e.g. `sheet_awards.go`'s finalist slots use a plain `AriaLabel`, no error cross-referencing either) - not a regression this story introduces below that baseline.
- **defer** (blind-hunter): the HTML-fragment-isolation helper (`seriesGroupFragment`) is duplicated near-verbatim between `sheet_series_test.go` and `series_predictions_steps_test.go`. Matches this repo's own already-tracked, still-open Epic 2 retrospective action item (`epic-2-retro-item-12-hoist-the-duplicated-setrowfragment-acce...`, `sprint-status.yaml`) covering the identical pre-existing pattern across other feature files - not new to this story.
- **defer** (blind-hunter + edge-case-hunter, grouped - same root cause): `PlayoffMatchup` has no validation anywhere (at `store.New()` or otherwise) that `Key` is non-blank/unique within a round, or that `TeamA`/`TeamB` both resolve to real teams sharing one conference. Concretely: a duplicate or blank `Key` collides two series' `SeriesKey`s into one Prediction row; an unresolvable `TeamA` degrades `conferenceForMatchup` to an empty-string conference (a stray " Conference" heading); and a blank `TeamAID` combined with no saved pick yet makes the template's `{{if eq .SelectedTeamID .TeamAID}}` spuriously render that radio as checked (both sides equal `""`). All three require a human hand-edit mistake in `playoff_matchups`, the exact same class of risk this codebase already accepts everywhere else for hand-maintained ids: `newRoster`'s own doc comment states duplicates are unvalidated ("the first item with a given key wins"), and this very diff's own `teamName` degrades an unresolved id to the raw string rather than erroring. Story 3.3 applies this same established, deliberate convention to a new field rather than inventing a gap; a proper fix (structured validation/warnings for hand-maintained `playoff_matchups`) is a cross-cutting concern better solved once, deliberately, not patched piecemeal per-symptom here.
- **false, moot** (blind-hunter): spec `status: 'done'` while Implementation Notes say no formal review was run. Moot: this review pass is what resolves that - status is corrected to `done` only after this triage completes, per the workflow's own Finalize step.

### Review Findings

Code review on 2026-09-24 (commit `fc1d543`): Blind Hunter, Edge Case Hunter, Verification Gap, Acceptance Auditor.

- [x] [Review][Patch] The `r2`/`cf` series sheets are never checked for series output: GET should show keyed cards and conference legends, and POST should save under `"r2.<key>"`. Also fix the stale "static stub" comment and the keyless fixture in `TestGetPredictSheetShouldServeEveryRoundGatedSetOnceAMatchupIsRecorded` [src/internal/web/sheet_series_test.go, src/internal/web/sheet_test.go]
- [x] [Review][Patch] `pickableSheetKinds` repeats the `seriesSetIDs` entries by hand. Dropping an id from one map silently falls back to the single-team dropdown, and every test still passes [src/internal/web/sheet.go:52]
- [x] [Review][Patch] No test checks that a rejected re-render keeps the other cards' submitted values (a half-filled sibling's radio should render `checked`) [src/internal/web/sheet_series_test.go]
- [x] [Review][Patch] The out-of-range games rejection test does not assert the inline error, and there is no test for a non-numeric `games` value [src/internal/web/sheet_series_test.go]
- [x] [Review][Patch] The save-failure log line records the bare matchup key, not the `joinSeriesKey(set.ID, m.Key)` series key [src/internal/web/sheet_series.go:319]
- [x] [Review][Patch] `.series-btn:has(input:checked)` copies the `.chip:has(input:checked)` declarations and should share that rule, as the Code Map asks [src/internal/web/static/styles.css]

#### Rejected

- spec: half-filled series with an invalid value are skipped, not rejected. This conflicts with the Code Map text but matches the frozen Intent and AC 4; the fix would be a spec edit.
- spec: a POST with only half-filled series redirects without feedback. The frozen Intent says "shows no error".
- low: POST to a series set with zero matchups returns 302. The form is not rendered in that state.
- low: a closed set with zero matchups shows no closed banner.
- low: conference groups follow first-appearance order. The data is hand-entered, East first by convention.
- low: picks are orphaned when a matchup key is renamed.
- low: the team roster is rebuilt per card.
- low: `splitSeriesKey` is unused in production. The spec asks for join/split helpers.
- low: brittle `checked` / `<button type="submit">` substring assertions.
- low: acceptance steps hard-code `r1`.
- low: key naming convention is undocumented.
- low: a blank series in a rejected re-render shows unchecked. Only reachable with a tampered form.
- low: a progress counter such as "3/8 picked". A feature request, not a defect.
- low: the rejected re-render re-reads matchups. The race window is negligible.
- already tracked (deferred-work): `Key` validation (blank, duplicate, `.` separator), an unresolved team giving the " Conference" label, cross-conference matchups, non-atomic multi-series save, accessibility of series cards, duplicated fragment helper.
