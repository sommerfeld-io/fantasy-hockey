---
title: 'Round Unlocking Based on Recorded Matchups'
type: 'feature'
created: '2026-09-24'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '57224792527b6560978c98100a6302227e336e3e'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Round 2, the Conference Finals, and the Stanley Cup Final currently stay Upcoming or Open purely by a hand-maintained `upcoming` flag on each Prediction Set — nothing ties that flag to whether the round's real matchups actually exist yet, so a player could be shown a pickable round before its opponents are known.

**Approach:** Add a new hand-maintained, read-only `playoff_matchups` section to `fantasy-hockey.yml` (per AD-23), keyed by the same Prediction Set id vocabulary already used everywhere else (`r2`, `cf`, `scf`). For those three ids only, compute effective "Upcoming" from whether any matchup is recorded for that id — ignoring their own `upcoming` YAML value — through one shared helper used by both the Predict list and the direct-URL sheet gate, so the two can never disagree. Round 1 (`r1`) and every non-round Prediction Set are untouched: their status keeps coming from the existing hand-maintained `upcoming` flag exactly as today.

</frozen-after-approval>

## Code Map

- `internal/store/store.go:57-73` — `PredictionSet.Upcoming` doc comment currently says "a later story may replace it with round-unlocking logic" — this is that story; update the comment to say it's now computed for `r2`/`cf`/`scf`, authoritative only for every other id.
- `internal/store/store.go:185-193` (`document` struct) — add `PlayoffMatchups map[string][]PlayoffMatchup `yaml:"playoff_matchups"`` alongside `Teams`/`NHLPlayers`; no loader change needed, `New()` already unmarshals the whole document in one pass.
- `internal/store/store.go:294-315` (`Teams()`/`NHLPlayers()`) — pattern to mirror for a new `Store.PlayoffMatchups(setID string) []PlayoffMatchup`: RLock, copy-and-return, no write method (AD-23 — read-only, hand-maintained).
- `internal/web/predict.go:67-113` (`newPredictSetView`, `predictStatus`) — the only place `set.Upcoming` currently gates status/accent/countdown/actionable; needs an effective-upcoming value in place of the raw field for `r2`/`cf`/`scf`.
- `internal/web/predict.go:125-145` (`buildPredictPhases`) — already holds `*store.Store` and iterates `st.PredictionSets()`; the natural place to resolve the effective-upcoming value per set before calling `newPredictSetView`.
- `internal/web/sheet.go:217-229` (`findOpenablePredictionSet`) — independently reads `set.Upcoming` to gate direct `/predict/{id}` access; must call the same shared helper as predict.go, not duplicate the check, or direct-URL access could diverge from what the Predict list shows.
- `internal/web/sheet.go:13-23` (`divisionsSetID`/`awardsSetID`) — precedent for a fixed-id constant, but that pattern is for pick-entry form dispatch, a different concern; the new round-gated ids belong in `predict.go` instead, next to the status logic they affect.

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/store.go` -- add `PlayoffMatchup{TeamA, TeamB string}` (`yaml:"a"`/`"b"`, matching the architecture doc's illustrative shape) and `document.PlayoffMatchups map[string][]PlayoffMatchup` -- new read-only hand-maintained section (AD-23).
- [x] `internal/store/store.go` -- add `Store.PlayoffMatchups(setID string) []PlayoffMatchup`, mirroring `Teams()`'s RLock/copy pattern; a missing key or empty list both return an empty slice.
- [x] `internal/store/store.go` -- update `PredictionSet.Upcoming`'s doc comment (no field/behavior change to the type itself).
- [x] `internal/web/predict.go` -- add `round2SetID = "r2"`, `conferenceFinalsSetID = "cf"`, `stanleyCupFinalSetID = "scf"` consts and a `roundGatedSetIDs` map, plus `effectiveUpcoming(st *store.Store, set store.PredictionSet) bool`: for a round-gated id, `len(st.PlayoffMatchups(set.ID)) == 0`; for every other id, `set.Upcoming` unchanged.
- [x] `internal/web/predict.go` -- thread `effectiveUpcoming` through `buildPredictPhases` into `newPredictSetView` in place of the four direct `set.Upcoming` reads.
- [x] `internal/web/sheet.go` -- `findOpenablePredictionSet` calls `effectiveUpcoming(st, set)` instead of reading `set.Upcoming` directly.
- [x] `internal/store/store_test.go` -- new tests for `PlayoffMatchups` parsing (present, absent key, empty list) and the accessor's copy semantics, mirroring `TestTeamsShouldReturnACopyThatCannotMutateTheStore`.
- [x] `internal/web/predict_test.go` -- new tests: a round-gated id with no matchups shows Upcoming regardless of its own `upcoming: false`; the same id with a matchup recorded shows Open/Closed per its deadline; `r1` and a non-round id are unaffected by `playoff_matchups` being absent or present.
- [x] `internal/web/sheet_test.go` -- new test: `findOpenablePredictionSet`/`handleSheet` return 404 for a round-gated id with no matchups recorded, even when its own `upcoming` flag is `false`.
- [x] `acceptance-tests/features/round-unlocking.feature` + steps -- Given/When/Then for: no matchups recorded (Upcoming, not tappable, direct URL 404), matchups recorded (Open, tappable, subject to deadline), Round 1 unaffected by `playoff_matchups` being empty.

**Acceptance Criteria:**
- Given `cf` has no entry under `playoff_matchups`, when Predict renders, then `cf` shows Upcoming (dimmed, locked, not tappable) and `GET /predict/cf` returns 404, regardless of `cf`'s own `upcoming` value in the data file.
- Given a human adds a matchup entry for `cf` to `fantasy-hockey.yml`, when the player next loads Predict, then `cf` shows Open (or Closed, per its own deadline) and is reachable — no in-app or admin action performs this.
- Given `r1` has no entry under `playoff_matchups`, when Predict renders, then `r1`'s status is governed only by its own `upcoming`/deadline exactly as before this story.

## Design Notes

`playoff_matchups` is keyed by Prediction Set id (`r2`/`cf`/`scf`, and `r1` for Story 3.3's later use) rather than the architecture doc's illustrative `round2:` naming — this matches the established "Kind == Set id, one identifier vocabulary" convention (AD-17/24/28) the rest of the codebase already follows, and the architecture doc marks its own shape as explicitly non-binding. No production `fantasy-hockey.yml` edit is needed for this story: the section is simply absent today, which already means "no matchups recorded" under the new logic — the same Upcoming state `r2`/`cf`/`scf` show today is preserved until a human adds real matchups later.

## Verification

**Commands:**
- `task go:test` -- expected: all unit tests pass, including new store/predict/sheet cases.
- `task go:test:acceptance` -- expected: new `round-unlocking.feature` scenarios pass alongside the existing suite.
- `task go:run` -- expected: lint, vet, full test suite, complexity, licenses, vulncheck all clean; server starts.

## Implementation Notes

- `newPredictSetView`'s signature changed to take an explicit `upcoming bool` parameter (resolved by the caller via `effectiveUpcoming`) instead of reading `set.Upcoming` internally. This meant a third call site not listed in the Code Map, `internal/web/sheet.go`'s `newSheetData` (which also calls `newPredictSetView`), needed the same `effectiveUpcoming(st, set)` treatment so its `Closed` field stays consistent - `newSheetData` is only reached once `findOpenablePredictionSet` has already let a set through, so this is a consistency/defensiveness fix rather than one that changes observable behavior today.
- Added a test helper, `newTestStoreWithPredictionSetsAndMatchups` (`internal/web/web_test.go`), mirroring the existing `newTestStoreWithPredictionSets`/`newTestStoreWithPredictionSetsAndTeams` pair, to seed a `playoff_matchups:` block alongside `prediction_sets:` for the new predict/sheet unit tests.
- The acceptance feature (`round-unlocking.feature`) and its steps (`round_unlocking_steps_test.go`) use feature-scoped step wording ("round-unlocking Prediction Set", "the signed-in player for round unlocking is", etc.) rather than reusing `browse-prediction-sets.feature`'s more generic phrasing - this repo's convention (seen in `cup-and-presidents-picks.feature` et al.) is to keep each feature file's step regexes unique, since every `Initialize*Scenario` function registers on the same shared `godog.ScenarioContext` per scenario run.
- All verification commands (`task go:test`, `task go:test:acceptance`, `task go:run`) pass clean, including lint, vet, gocyclo, go-licenses, and govulncheck; the server starts successfully.
- Nothing was left incomplete. `fantasy-hockey.yml` itself was intentionally left unedited, per the Design Notes (the section's absence already means "no matchups recorded").

## Review Triage Log

- **medium, patch** (verification-gap, pre-verified): every "matchup recorded" test/fixture hardcodes `upcoming: false` on the round-gated id; production `fantasy-hockey.yml` has `r2`/`cf`/`scf` at `upcoming: true` today, so the actual real-world unlock path (`upcoming: true` + matchup recorded → Open) is untested. Current code is correct (`effectiveUpcoming` fully ignores the flag for gated ids), but a future regression coupling the flag back in would ship undetected. Fix: add a test with `upcoming: true` + a recorded matchup asserting Open.
- **medium, patch** (blind-hunter): only `"cf"` is ever used for a positive round-gated assertion; `"r2"` is untested entirely and `"scf"` only appears in the negative/absent-key store test. A typo in `round2SetID`/`stanleyCupFinalSetID`'s literal value would go uncaught. Fix: extend coverage to all three ids (can be folded into the same test added for the finding above via a table).
- **low, patch** (blind-hunter): `roundGatedSetIDs`'s doc comment says the trio can't drift out of sync with "effectiveUpcoming's own switch" — the function is an `if`/`return`, not a `switch`. Trivial wording fix, no reason not to take it.
- **false** (blind-hunter): claimed `newSheetData`'s "no observable behavior change" claim is untested (only status/title checked, not `Closed`/`CountdownFaint`). Disproved: `findOpenablePredictionSet` and `newSheetData` are called sequentially within the same request handler, against the same `*store.Store` snapshot, with `effectiveUpcoming` a pure function of `(st, set)` — so `newSheetData`'s call is provably identical to the gate's own already-proven result; the two can never diverge given the call ordering, so there is no live behavior for a test to catch.
- **low, reject** (blind-hunter): explicit-empty-list (`playoff_matchups: {cf: []}`) form is proven identical to absent-key only at the store layer, not re-proven at predict/sheet/acceptance layers. Rejected: `effectiveUpcoming` contains no logic capable of distinguishing the two inputs (`len(st.PlayoffMatchups(...)) == 0` is the only check), so a layer above the store has nothing left to differentiate — re-testing there would be redundant with what the store test already proves by construction.
- **low, reject** (blind-hunter): no test pairs "no matchups" with the gated id's own `upcoming: true` (only `upcoming: false` is tested for that case). Rejected: this exact input combination cannot discriminate correct code from a hypothetical "always Upcoming" bug in the gated branch — both produce the same Upcoming result — so it has no verification value; the combination that would actually discriminate (matchup present + `upcoming: true`) is the first finding above, already routed to patch.
- **defer** (blind-hunter): no in-repo documentation of the new `playoff_matchups` YAML shape outside the Go GoDoc and this spec. Confirmed the production `fantasy-hockey.yml` (317 lines) has zero comment lines anywhere — every existing hand-maintained section (players, teams, prediction_sets, etc.) already relies solely on Go GoDoc for schema documentation. Real observation, but a pre-existing repo-wide convention this story doesn't worsen, not something to fix in isolation here.
- **defer** (blind-hunter): the "signed-in player" acceptance step performs no sign-in action, just validates a name against one hardcoded fixture. Confirmed this exact pattern is already used verbatim by `cup_and_presidents_picks_steps_test.go`'s own `theSignedInPlayerIs` - pre-existing repo convention, not introduced or worsened by this story.
- **defer** (blind-hunter): `roundGatedSetIDs`/its constants duplicate the `r2`/`cf`/`scf` id vocabulary with no cross-check against `fantasy-hockey.yml`'s actual ids. Confirmed this is the same trade-off every existing special-cased id already accepts (`divisionsSetID`, `awardsSetID`, every `store.Kind*` constant) - none of them cross-validate against the YAML file either; an existing architectural pattern, not something this diff introduces.
- **empty** (edge-case-hunter): zero findings reported after tracing all call sites, lock semantics, deletions, and falsification-testing every checkable spec claim.

### Review Findings

Code review on 2026-09-24 (commit `1e6cf92`): Blind Hunter, Edge Case Hunter, Verification Gap, Acceptance Auditor.

- [x] [Review][Patch] No test checks that rounds unlock independently: seed `r2`/`cf`/`scf` at `upcoming: true` with matchups only under `r2`, then assert `r2` Open, `cf`/`scf` Upcoming, and `GET /predict/cf` returns 404 [src/internal/web/predict_test.go, src/internal/web/sheet_test.go]
- [x] [Review][Patch] No test for `r1` with `upcoming: true` plus `r1` matchups present staying Upcoming. The spec task asks for "absent or present" [src/internal/web/predict_test.go]
- [x] [Review][Patch] Using `effectiveUpcoming` in `newSheetData` is not pinned by a test: a closed gated series sheet (`cf`, `upcoming: true`, matchup recorded, past deadline) must show `closed-banner` and disabled inputs [src/internal/web/sheet_series_test.go]
- [x] [Review][Defer] A hand edit to `playoff_matchups` while the app runs is not seen until a restart, and the next app write overwrites it [src/internal/store/store.go:247] - deferred: pre-existing AD-27 trait of every hand-maintained section, but this is the first story whose AC depends on a live hand edit

#### Rejected

- spec: `upcoming: true` is ignored for gated rounds, and there is no admin lock. AC1 says "regardless of `cf`'s own `upcoming` value".
- spec: gating logic lives in `internal/web` and not in `internal/predictions`. The spec's Code Map chose this placement.
- low: the acceptance feature covers only `cf` and `upcoming: false`. Unit tests cover all three ids and `upcoming: true`.
- low: "dimmed/locked" not asserted in acceptance tests. The unit test checks `set-row--upcoming`.
- low: blank `a`/`b` entries unlock a round. This belongs to the already-deferred `playoff_matchups` validation item.
- low: `playoff_matchups` keys are case-sensitive; extra YAML keys are dropped on write; the fixture writes an unquoted map key.
- low: the first write adds `playoff_matchups: {}` and reorders keys. Harmless, and the same as the other sections.
- low: a slice is copied under the lock just to test its length.
- low: store-test boilerplate; the store doc comment names a web function; Gherkin titles mention the YAML key.
- already tracked (retro item 12): `setRowFragment` truncation fragility.
- already tracked (deferred-work): hard-coded round ids.
