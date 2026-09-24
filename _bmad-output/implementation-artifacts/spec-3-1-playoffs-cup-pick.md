---
title: 'Playoffs Cup Pick'
type: 'feature'
created: '2026-09-24'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Once the playoff field is set, players should get a second, better-informed Stanley Cup pick, made independently of the season-opening Cup champion pick and never overwriting it. The `playoffcup` Prediction set already exists in the data file (its own id, deadline, `phase: playoffs`) but is not yet wired up as a pickable set, so it currently renders as a static, unpickable stub.

**Approach:** Register `playoffcup` as a pickable single-team, division-grouped pick by adding a `store.KindPlayoffsCup` kind constant and adding it to the `pickableSheetKinds` routing map in `internal/web/sheet.go`, reusing the existing generic single-team-id branch (the same one `cup`/`presidents` already use) — no new handler, template branch, or store logic is needed. Add a `playoffs-cup-pick.feature` acceptance spec (mirroring `cup-and-presidents-picks.feature`) and matching unit tests, and repoint `cup-and-presidents-picks.feature`'s "unknown/stub id" negative-case scenario away from `playoffcup` (onto an id that remains a genuine stub, e.g. `r1`) since it will no longer be a valid stub example once this ships.

</frozen-after-approval>

## Implementation Notes

- Added `store.KindPlayoffsCup = "playoffcup"` (`internal/store/store.go`) and one entry in `pickableSheetKinds` (`internal/web/sheet.go`). No other production code changed — `newSheetData`/`handleSheet`/`handleSheetSubmit`/`teamRoster` were already kind-agnostic, confirming the Approach's "no new handler, template branch, or store logic" assumption.
- Repointed the repo's established "next remaining stub id" test convention (unit tests in `sheet_test.go` plus the negative-case scenarios in `browse-prediction-sets.feature` and `cup-and-presidents-picks.feature`) from `playoffcup` onto `r1`, matching the precedent set by Stories 2.3/2.4/2.6 of repointing this stub-proof off the id each story just made real.
- Added `store.KindPlayoffsCup` to `sheet_test.go`'s `upcomingGateCases` table (extends the existing Upcoming-gate table tests for free), plus two dedicated tests: one mirroring the presidents happy-path end-to-end test, and one (`TestPostPredictSheetShouldNotOverwriteTheSeasonCupPickWhenSavingPlayoffsCup`) proving the Intent's "never overwrites" requirement directly against the store.
- Added `acceptance-tests/features/playoffs-cup-pick.feature` (10 scenarios) with its own steps file (`playoffs_cup_pick_steps_test.go`), covering the full pick lifecycle plus the cross-set independence scenario. First pass covered 6 scenarios; a Blind Hunter review flagged the closed-set, resubmission, and Upcoming-gate scenarios as missing relative to the sibling `cup-and-presidents-picks.feature` this spec's own Approach said to mirror — added all four in response.
- Review also flagged the spec missing Boundaries/I/O Matrix/Design Notes/Verification sections and a `baseline_commit` field: verified against sibling specs that these are `dispatch`-route-only (a `oneshot`-routed spec, which never enters the review-loop code-revert cycle, correctly omits them per step-02's own instructions) — no change needed.
- `.task`/`.vscode` submodule pointer bumps were already dirty in the working tree before this story started (unrelated environment drift, not part of this change) — excluded from the commit below.
- First commit attempt was rejected by the repo's pre-commit `lint-gherkin` hook (`no-dupe-scenario-names`): four scenario titles in `playoffs-cup-pick.feature` were copied verbatim from `cup-and-presidents-picks.feature`. Renamed all four to be unique (e.g. "Submitting a valid Playoffs Cup pick saves it and returns to Predict with the set Submitted") and re-verified no duplicates remain across any feature file before recommitting.

## Review Triage Log

- **patch (fixed):** `playoffs-cup-pick.feature` was missing the closed-set read-only-banner scenario present in the sibling `cup-and-presidents-picks.feature` this spec's Approach said to mirror. Real gap — added the scenario plus its two assertion steps.
- **patch (fixed):** Same file was also missing the resubmission and both Upcoming-gate scenarios from the sibling file. Real gap — added all three (resubmission reused existing steps; Upcoming-gate needed two new step defs).
- **patch (fixed):** `## Implementation Notes` was left empty despite implementation being finished. Filled in with the decisions, files touched, and review outcome above.
- **false:** Spec missing `## Boundaries & Constraints`, `## I/O & Edge-Case Matrix`, `## Design Notes`, `## Verification` compared to sibling specs. Disproved: those siblings are all `route: 'dispatch'`; step-02's own instructions say a `oneshot`-routed spec (no intent gaps, nothing irreversible, small change) keeps only Intent + Implementation Notes and deletes the rest.
- **false:** Missing `baseline_commit` field compared to sibling specs. Disproved: every spec with that field is `route: 'dispatch'` (grepped all `_bmad-output/implementation-artifacts/*.md`) — it exists to support the dispatch-only bad_spec review-loop code-revert mechanism (see spec-2-4's Spec Change Log), which a `oneshot` spec never enters.
- **false (premature):** `status: 'in-progress'` and `sprint-status.yaml` still `in-progress` rather than `review`/`done`. Not a defect — both are set by this same Finalize Spec step, which runs immediately after classifying these findings.
- **true, handled procedurally (not a code fix):** `.task`/`.vscode` submodule pointer bumps are unrelated dirty state that predates this story. Not part of the diff to patch — handled by committing only Story 3.1's files.
- **false (moot after patch):** `epic-3-context.md` not flagging "reuse the mechanic but not its test coverage" — moot once the acceptance-coverage gap above was closed; there is no longer a gap to inherit.

### Review Findings

Code review on 2026-09-24 (commit `5722479`): Blind Hunter, Edge Case Hunter, Verification Gap, Acceptance Auditor.

- [x] [Review][Patch] Playoffs Cup pick is Open and submittable in preseason. Decision (2026-09-24): set `upcoming: true` on `playoffcup` in the data file, to be flipped by hand once the playoff field is set, and remove the stray test pick (`sebastian`, `DET`) [src/fantasy-hockey.yml]
- [x] [Review][Patch] Close the playoffcup test-mirror gaps: add the sibling's "never force-creates a Prediction row" scenario; assert the `<select>` is disabled when closed; add the reverse-independence and no-leak checks (saving `cup` leaves `playoffcup` alone, and a saved season `cup` pick is not preselected on the playoffcup sheet); keep an existing pick unchanged after a late POST [src/acceptance-tests/features/playoffs-cup-pick.feature]

#### Rejected

- false: the stub examples moved to `r1` would need another rewrite. Story 3.3 already moved them to the fixture-only id `mystery-set`.
- false: scope went beyond the Approach. The extra stub moves are required and documented in Implementation Notes.
- low: "all 32 teams" is not verified for playoffcup. The fixture matches the sibling feature, and the code path is shared.
- low: loose `<button` / `<optgroup` assertions.
- low: kind constants used as set ids in tests; comments that name story numbers.
- low: inherited "Pick a team" message for an unknown team id.
- low: after an unknown-team re-render no option is selected. This is only reachable with a tampered form.
- low: acceptance-step hygiene (a nil store before `ensureReady`, unencoded form body, duplicate Given ids). Test-only, and not reachable with the current scenarios.
- low: data-seed deadline ordering. Informational, not a defect.
- already tracked (retro item 12): the copied `setRowFragment` and fixture helpers.
