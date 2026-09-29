## Deferred from: code review of story-1-1-request-a-login-code (2026-09-14)

- No test proves `internal/auth.RequestLoginCode` returns before its `send` call completes (i.e. that the async dispatch introduced to close the timing side-channel actually stays async). Reliably asserting response latency in a fast unit test needs an artificially slow fake sender plus a wall-clock assertion, which is easy to make flaky; the intent is already documented in the story's own Review Triage Log history.
- Residual timing side-channel from the synchronous store write in `auth.RequestLoginCode` (`src/internal/auth/auth.go`): a match still runs `generateCode`/`hashCode`/`st.CreateLoginCode` synchronously before responding, while a no-match returns almost instantly. Deferred: hobby-scale threat model (3-person pool, not internet-facing yet); the disk-write timing gap is small and not worth the complexity of masking it.
- Async `send` goroutine in `auth.RequestLoginCode` has no shutdown/lifecycle coordination with `main.go`'s graceful shutdown: a restart shortly after responding can silently drop an in-flight SMTP send with no log line at all. Deferred: self-healing via retry — restarts are rare and operator-timed; a dropped code just means the player requests a new one.

## Deferred from: code review of story-1-2-enter-login-code-and-establish-session (2026-09-14)

- ~~Empty player-id validation is only guarded at the `internal/web` handler level (`handleLoginCodeSubmit`); `auth.ParseSessionCookie` and `store.ConsumeLoginCode` don't validate it in their own contracts. Deferred: nothing yet calls `ParseSessionCookie` for real route-guarding — Story 1.3 is the one that wires it into middleware, and should decide there whether to harden it at that layer too.~~ **Resolved in Story 1.3**: `auth.ValidateSession` (the middleware's actual entry point) now rejects an empty decoded player id directly, alongside a tampered signature or an idle-expired `issued_at` — see `spec-1-3-stay-logged-in-with-sliding-session-timeout.md`.

## Deferred from: code review of story-1-5-persistent-app-shell-with-player-identity-and-navigation (2026-09-15)

- source_spec: `_bmad-output/implementation-artifacts/spec-1-5-persistent-app-shell-with-player-identity-and-navigation.md`
  summary: Every authenticated shell route (`GET /{$}`, `/predict`, `/leaderboard`, `/compare`) must be registered on both the outer `mux` and the inner `authMux`, so the two lists can silently drift out of sync as future stories add routes.
  evidence: Verified real in this story's review (blind-hunter): `NewServer` in `src/internal/web/web.go` repeats each path once per mux. Pre-existing pattern (AD-2/AD-11) already in place for `GET /{$}` before this story — this story only extended it to 3 more routes exactly as the spec's Code Map instructed, so it's not this story's defect to fix, but the drift risk grows with every route added on top of it.

## Deferred from: code review of story-2-1-browse-prediction-sets-by-phase-and-status (2026-09-15)

- source_spec: `_bmad-output/implementation-artifacts/spec-2-1-browse-prediction-sets-by-phase-and-status.md`
  summary: `epic-2-context.md` (a new file this story added) states the set-row accent stripe follows "blue/green/red/faint by state," but neither the code nor DESIGN.md's own token map gives Closed a distinct red stripe.
  evidence: Verified real (acceptance-auditor): DESIGN.md's `set-row` component only lists `open`/`submitted`/`upcoming` accent colors (no `closed`), and `newPredictSetView` in `src/internal/web/web.go` only special-cases `Upcoming`, leaving Closed to reuse Open's blue class — matching the UX click-dummy's actual behavior. The prose in `epic-2-context.md` is factually imprecise, not the application behavior; best fixed by regenerating or hand-editing that compiled planning doc outside an ad hoc code review.

## Deferred from: code review of story-2-5-load-season-s-canonical-nhl-player-list (2026-09-18)

- source_spec: `_bmad-output/implementation-artifacts/spec-2-5-load-season-s-canonical-nhl-player-list.md`
  summary: No automated test (unit or acceptance) ever loads and parses the real, shipped `src/fantasy-hockey.yml` directly — every test builds its own temp-dir fixture instead, so a typo'd key in the real file (e.g. `postion:`) would silently parse to a zero-valued field with no automated signal.
  evidence: Verified real (verification-gap): confirmed via `grep` that `store_test.go`, every `acceptance-tests/*_steps_test.go`, and `main_test.go` all build hand-typed YAML fixtures rather than reading the real file. Pre-existing pattern already true for `teams:`/`players:`/`predictions:` before this diff (not introduced by Story 2.5) — `yaml.Unmarshal`'s non-strict parsing means this class of error is currently only caught by the manual `task go:run`/`task docker:build` verification step this repo already relies on.
- source_spec: `_bmad-output/implementation-artifacts/spec-epic-2-retro-items-8-9-10-17-pre-epic-3-hardening.md`
  summary: Refresh stale godoc comments in internal/web (package comment, NewServer and predictSheetSubmitPattern still say "cup/presidents" only, maxSheetFormBytes says "a single team id").
  evidence: These comments predate the web.go split, which moved them verbatim. The POST /predict/{id} route also serves divisions and awards. They fit with retro item #11 (stale doc comments).
- source_spec: `_bmad-output/implementation-artifacts/spec-epic-2-retro-items-8-9-10-17-pre-epic-3-hardening.md`
  summary: Move the Epic 1 acceptance step files (app_shell, login, enter_login_code, log_out, stay_logged_in) and the two eager load_canonical_* servers onto the shared lazyFixture/do recorder in fixture_support_test.go.
  evidence: enter_login_code still has its own ensureReady + store.New + httptest + CheckRedirect. login, log_out and stay_logged_in each build their own no-redirect client. app_shell has its own eager seeded-store builder. Retro item #17 only named the Epic 2 files.

## Deferred from: code review of story-3-2-round-unlocking-based-on-recorded-matchups (2026-09-24)

- source_spec: `_bmad-output/implementation-artifacts/spec-3-2-round-unlocking-based-on-recorded-matchups.md`
  summary: The new `playoff_matchups` YAML section has no in-repo documentation of its shape (keys `a`/`b`, grouped by Prediction Set id) outside its Go GoDoc comment and the implementation spec.
  evidence: Verified real (blind-hunter): confirmed the production `fantasy-hockey.yml` (317 lines) has zero comment lines anywhere - every existing hand-maintained section (players, teams, prediction_sets, etc.) already relies solely on Go GoDoc for schema documentation. Pre-existing repo-wide convention, not worsened by this story specifically.
- source_spec: `_bmad-output/implementation-artifacts/spec-3-2-round-unlocking-based-on-recorded-matchups.md`
  summary: Acceptance-test "signed-in player" steps perform no sign-in action - they only validate a name against one hardcoded fixture, which reads as an action to anyone reading the `.feature` file but forecloses ever testing a second player or an unauthenticated request within the feature.
  evidence: Verified real (blind-hunter): confirmed this exact no-op pattern already exists verbatim in `cup_and_presidents_picks_steps_test.go`'s own `theSignedInPlayerIs`, predating this story; `round_unlocking_steps_test.go` only followed the established convention.
- source_spec: `_bmad-output/implementation-artifacts/spec-3-2-round-unlocking-based-on-recorded-matchups.md`
  summary: `roundGatedSetIDs` and its `round2SetID`/`conferenceFinalsSetID`/`stanleyCupFinalSetID` constants duplicate the `r2`/`cf`/`scf` id vocabulary with no cross-check against `fantasy-hockey.yml`'s actual ids, so a silent rename elsewhere would desync with no compiler or test signal.
  evidence: Verified real (blind-hunter), but the same trade-off every existing special-cased Prediction Set id already accepts (`divisionsSetID`, `awardsSetID`, every `store.Kind*` constant) - none of them cross-validate against the YAML file either. Pre-existing architectural pattern, not introduced by this story.

## Deferred from: code review of story-3-3-per-round-series-predictions (2026-09-24)

- source_spec: `_bmad-output/implementation-artifacts/spec-3-3-per-round-series-predictions.md`
  summary: Series cards have no `aria-describedby` linking an inline error to its inputs, and group per conference rather than per series in the markup, giving screen readers no structural cue between one series' two radio groups and the next.
  evidence: Verified real (blind-hunter), but consistent with this app's existing baseline elsewhere - `sheet_awards.go`'s finalist slots use a plain `AriaLabel` with no error cross-referencing either. Not a regression this story introduces below that baseline.
- source_spec: `_bmad-output/implementation-artifacts/spec-3-3-per-round-series-predictions.md`
  summary: The HTML-fragment-isolation test helper (`seriesGroupFragment`) is duplicated near-verbatim between `internal/web/sheet_series_test.go` and `acceptance-tests/series_predictions_steps_test.go`.
  evidence: Verified real (blind-hunter), matches this repo's own already-tracked, still-open Epic 2 retrospective action item (`epic-2-retro-item-12-hoist-the-duplicated-setrowfragment-acce...` in `sprint-status.yaml`) covering the identical pre-existing pattern across other feature files - not new to this story.
- source_spec: `_bmad-output/implementation-artifacts/spec-3-3-per-round-series-predictions.md`
  summary: `PlayoffMatchup` has no validation that `Key` is non-blank/unique within a round, or that `TeamA`/`TeamB` both resolve to real teams sharing one conference - a duplicate/blank `Key` collides two series into one Prediction row, an unresolvable `TeamA` produces a stray " Conference" heading, and a blank `TeamAID` with no saved pick yet makes the template spuriously render that radio as checked.
  evidence: Verified real (blind-hunter + edge-case-hunter), but the same class of risk this codebase already accepts everywhere for hand-maintained ids: `newRoster`'s own doc comment states duplicate ids are unvalidated ("the first item with a given key wins"), and this same diff's `teamName` degrades an unresolved id to the raw string rather than erroring. Story 3.3 applies this established convention to a new field rather than inventing a gap; a proper fix (structured validation/warnings for hand-maintained `playoff_matchups`) is a cross-cutting concern better solved once, not patched piecemeal here.
- source_spec: `_bmad-output/implementation-artifacts/spec-3-3-per-round-series-predictions.md`
  summary: `handleSeriesSubmit`'s per-series save loop isn't atomic across the whole POST - if `SaveSeriesPick` fails partway through (a disk write error), series saved earlier in the same loop stay saved while the client only sees a generic 500 with no indication which succeeded.
  evidence: Verified real (edge-case-hunter), but this is the Implementation Notes' own deliberate, documented trade-off (row-level atomicity, matching the frozen Intent's literal wording, not whole-POST atomicity), and a disk-write failure mid-request is the same class of low-likelihood operational fault already accepted in this file's story-1-1 entries (timing side-channels, unguarded async sends) at this app's declared hobby scale.

## Deferred from: code review of spec-3-2-round-unlocking-based-on-recorded-matchups (2026-09-24)

- source_spec: `_bmad-output/implementation-artifacts/spec-3-2-round-unlocking-based-on-recorded-matchups.md`
  summary: Hand edits to `playoff_matchups` made while the app is running are not picked up until a restart, and the app's next whole-file write (any prediction save or login-code issue) overwrites them from memory. AC2 ("when the player next loads Predict") therefore only holds across a restart.
  evidence: Verified real (acceptance-auditor): `PlayoffMatchups` reads `s.doc`, which `New()` loads once, and the store has no reload path. This is the AD-27 trait every hand-maintained section already has; the fix (reload on change, or an operator note to stop the app before editing) is cross-cutting.
- source_spec: none
  summary: Hoist the duplicated acceptance-test helpers (`setRowFragment` x4, `theSignedInPlayerIs` variants, team-name fixture maps, series group fragment) into `fixture_support_test.go` before Epic 4 adds step files (epic-3 retro item 22, supersedes epic-2 item 12).
  evidence: Split from the pre-epic-4 cleanup intent at the multi-goal check - it is a test-only change independently shippable from the items 19 + 18 store-ownership move; the user chose to build it as the next Build right after.

## Deferred from: code review of spec-epic-3-retro-items-18-19-store-owned-playoff-and-division-ids (2026-09-24)

- source_spec: `_bmad-output/implementation-artifacts/spec-epic-3-retro-items-18-19-store-owned-playoff-and-division-ids.md`
  summary: The store now owns the division vocabulary (`store.Divisions()`) but never enforces it - a hand-maintained `Team.Division` typo (e.g. `Metropolitian`) silently drops those teams from every division-grouped dropdown and chip form, and `SaveDivisionPicks` accepts any division key.
  evidence: Verified real (blind-hunter + edge-case-hunter): `groupTeamsByDivision` only emits divisions listed in `store.Divisions()`, and nothing checks team data on load. Pre-existing behavior, not introduced by this move; it belongs with the already-deferred structured validation of hand-maintained sections (see the `playoff_matchups` validation entry from spec-3-3).

## Deferred from: code review of spec-epic-3-retro-item-22-hoist-acceptance-test-helpers (2026-09-24)

- source_spec: `_bmad-output/implementation-artifacts/spec-epic-3-retro-item-22-hoist-acceptance-test-helpers.md`
  summary: `theCanonicalTeamListIncludes` and its `{id, name, division}` team-fixture struct are still duplicated between `cup_and_presidents_picks_steps_test.go` and `playoffs_cup_pick_steps_test.go` - only their `sampleTeamNames` lookup map was hoisted.
  evidence: Verified real (blind-hunter), but pre-existing and not among the helpers the item-22 Intent names; hoisting needs a shared fixture type in `fixture_support_test.go`. Worth doing before an Epic 4 step file adds a third copy.

- source_spec: `_bmad-output/implementation-artifacts/spec-4-1-automatic-scoring-engine.md`
  summary: A misspelled key in the hand-maintained results or award_finalists section (e.g. `stanley_cup_winer`) is silently ignored with no startup warning.
  evidence: yaml.v3 lenient decoding drops unknown fields. A probe showed the value loads as empty with a nil error. The user chose to defer this to story 7-3 on 2026-09-25.
- source_spec: `_bmad-output/implementation-artifacts/spec-4-1-automatic-scoring-engine.md`
  summary: A wrongly shaped results entry (e.g. `playoffs: FLA` instead of a list) makes store.New fail and the app refuse to start, rather than warning.
  evidence: A probe confirmed "cannot unmarshal !!str `FLA` into []string". Any fix must not drop the hand-recorded results on the next save. The user chose to defer this to story 7-3 on 2026-09-25.
- source_spec: `_bmad-output/implementation-artifacts/spec-4-1-automatic-scoring-engine.md`
  summary: Hand edits to results made while the app runs are overwritten by the next prediction save.
  evidence: writeLocked marshals the in-memory doc loaded at startup. This is pre-existing for every hand-maintained section (AD-27), and the playoffs runbook (Epic 3 retro item 21) should say to stop the app before editing.
- source_spec: `_bmad-output/implementation-artifacts/spec-4-1-automatic-scoring-engine.md`
  summary: No test proves run() routes store opening through openStore, so the startup malformed-result warning could silently disappear.
  evidence: The openStore tests call it directly, and nothing exercises run(). Reverting run() to store.New passes every test.

## Deferred from: code review of spec-4-1-automatic-scoring-engine.md (2026-09-25)

- Nothing tests that `run()` in src/main.go goes through `openStore`, so the startup malformed-result warning could silently disappear. `run()` has no test seam.
- ~~A wrongly shaped results entry (e.g. `playoffs: FLA`) makes store.New fail and the app refuse to start instead of warning. Deferred to story 7-3 by user decision.~~ **Resolved in Story 7.4**: `New` now tolerates a shape error confined to `results:`/`award_finalists:` — see `spec-7-4-hand-edited-results-are-safe-to-edit.md`.
- ~~A misspelled key under `results:` (e.g. `stanley_cup_winer`) is silently ignored with no warning, and it is dropped from the file on the next save. Deferred to story 7-3 by user decision.~~ **Resolved in Story 7.4**: `unknownKeyProblemsLocked` now reports it, and it survives a save unchanged.
- ~~Every prediction save re-serializes the hand-maintained results and award_finalists sections, which drops comments and unmodelled keys and re-quotes `games`. Reason (user): the Never rule means the app never creates or changes results. Byte and comment preservation belongs to story 7-3.~~ **Resolved in Story 7.4**: `writeLocked` now splices only `login_codes`/`predictions` into the original parsed node tree; every hand-maintained section round-trips through its own original nodes.

- source_spec: none
  summary: Epic 4 retro A9, which hoists the duplicated YAML-seed store helpers, prediction-row renderers, acceptance seed builders and the submitted_at literal into shared test support before Epic 5.
  evidence: The user split it out of the Epic 4 retro hardening spec on 2026-09-25. It rewrites the same step files that A6 extends, so it should ship as its own change.

## Deferred from: code review of spec-epic-8-retro-item-63-rules-xss-safety-doc-and-test.md (2026-09-29)

- source_spec: `_bmad-output/implementation-artifacts/spec-epic-8-retro-item-63-rules-xss-safety-doc-and-test.md`
  summary: This change documents and tests the `template.HTML(...)` XSS-safety invariant at one call site (`rulesHTML` in `internal/web/rules.go`), but adds no automated guardrail against a second, less-careful `template.HTML(...)` conversion being added elsewhere later — `gosec` (rule G203 flags exactly this pattern) is confirmed absent from `src/.golangci.yml`'s enabled linters.
  evidence: Verified real (blind-hunter): confirmed `gosec` is not in `linters.enable`. Pre-existing repo-wide linter-config gap, not introduced by this change; enabling `gosec` repo-wide is a larger, separate decision (could surface findings across the whole codebase) than this item's narrow scope (one call site's doc comment and test).

## Deferred from: code review of spec-epic-7-retro-item-55-correct-store-readme-stale-write-claim.md (2026-09-29)

- source_spec: `_bmad-output/implementation-artifacts/spec-epic-7-retro-item-55-correct-store-readme-stale-write-claim.md`
  summary: `internal/store/README.md`'s `ResultProblems()` bullet (in "Results (read-only)") is stale relative to the current implementation — it lists only the original malformed-value categories (unknown team/slug/division/award/round, bad series key, games out of range, mismatched winner, wrong-division team) and omits the two categories Story 7.4 added: a misspelled/unknown field name (`unknownKeyProblemsLocked`) and a tolerated shape error (`s.toleratedShapeErrors`).
  evidence: Verified real (blind-hunter), found while fact-checking this build's own correction pass. Pre-existing since Story 7.4 shipped — not caused by this build, which corrected a different, specifically-named bullet (epic-7-retro-item-55's stated scope: the write-path claim, not `ResultProblems()`'s category list). Kept out of scope to avoid drift from the retro item's own narrow ask.

## Deferred from: code review of spec-epic-7-retro-item-57-split-store-go-by-concern.md (2026-09-28)

- source_spec: `_bmad-output/implementation-artifacts/spec-epic-7-retro-item-57-split-store-go-by-concern.md`
  summary: `yaml "go.yaml.in/yaml/v3"` aliases a package whose own declared name is already `yaml`, so the alias is redundant — now repeated in three files (`store.go`, `store_splice.go`, `store_results.go`) instead of one, tripling the number of places a future import-path or library change has to touch.
  evidence: Verified real (blind-hunter): confirmed `go.yaml.in/yaml/v3`'s own `decode.go` declares `package yaml`. Pre-existing in the original `store.go` before this split — the split only repeated existing import syntax across the new files as an unavoidable consequence of each needing its own import block, not something this refactor introduced as new redundancy.

## Deferred from: code review of spec-epic-8-retro-item-62-rules-cross-destination-scenario.md (2026-09-29)

- source_spec: `_bmad-output/implementation-artifacts/spec-epic-8-retro-item-62-rules-cross-destination-scenario.md`
  summary: The Rules destination was never backfilled into `stay-logged-in.feature`'s idle-timeout scenarios, `log-out.feature`'s post-logout-unauthenticated scenarios, or `app-shell.feature`'s own logout scenario when Rules was added as a 4th destination in Epic 8 — the same class of gap this build fixed for the identity-persistence scenario, just left open in three other places.
  evidence: Verified real (blind-hunter): confirmed via `grep` that neither `stay-logged-in.feature` nor `log-out.feature` mentions "rules" anywhere. `/rules` sits behind the identical `requireSession` middleware as `/predict`/`/leaderboard`/`/compare` (`internal/web/web.go`), so nothing structurally exempts it. Pre-existing since Epic 8, not caused by this build's one-line change (which only touched the identity-persistence scenario this item specifically named).
- source_spec: `_bmad-output/implementation-artifacts/spec-epic-8-retro-item-62-rules-cross-destination-scenario.md`
  summary: `everyVisitedDestinationShowedThePlayersName` (the step the new Rules line exercises) only asserts the player's *name* persists across visited destinations, never the season — a regression that broke the season display specifically on one tab would pass this scenario undetected.
  evidence: Verified real (blind-hunter): confirmed the step's own implementation only checks `PlayerName`; `theShellShowsTheSeason` is a separate, unused-here step. Pre-existing scope of this helper, not introduced by adding Rules to the destination list.
- source_spec: `_bmad-output/implementation-artifacts/spec-epic-8-retro-item-62-rules-cross-destination-scenario.md`
  summary: The "Navigating between destinations..." identity scenario never includes the root path `/` (which aliases to Predict) even though it's a distinct route through `handleShell` and the scenario's stated purpose is proving identity persists across every route into the shell.
  evidence: Verified real (blind-hunter). Pre-existing omission, not caused by this build's change.
- source_spec: `_bmad-output/implementation-artifacts/spec-epic-8-retro-item-62-rules-cross-destination-scenario.md`
  summary: Nothing enforces that `app_shell_steps_test.go`'s `navHrefsByTab` map, its `InitializeAppShellScenario` regex alternation, and each destination's title-casing stay in sync — unlike production code's `shellRoutes`, there's no single source of truth, so a future 5th destination could update one site and silently miss another.
  evidence: Verified real (blind-hunter): confirmed all three sites currently agree for the 4 existing destinations, but nothing derives one from another. Pre-existing test-helper design, not caused by this build's change.
- source_spec: `_bmad-output/implementation-artifacts/spec-epic-8-retro-item-62-rules-cross-destination-scenario.md`
  summary: `theShellShowsTheRulesContent`'s smoke check only asserts the presence of "Game Rules"/"Scoring," unlike `theShellShowsTheCompareContent`, which also asserts the *absence* of leftover placeholder text ("coming soon") — an asymmetry in test rigor between the Rules and Compare tabs' own content checks.
  evidence: Verified real (blind-hunter). Pre-existing gap in a different scenario than the one this build changed.

## Deferred from: code review of spec-7-4-hand-edited-results-are-safe-to-edit.md (2026-09-27)

- source_spec: `_bmad-output/implementation-artifacts/spec-7-4-hand-edited-results-are-safe-to-edit.md`
  summary: `unknownKeyProblemsLocked`'s four hardcoded key-vocabulary slices (`resultsTopLevelKeys`, `divisionMarksKeys`, `seriesOutcomeKeys`, `awardFinalistEntryKeys`) duplicate the yaml struct tags already on `results`/`divisionMarks`/`seriesOutcome`/`AwardFinalist` with no structural safeguard tying the two together.
  evidence: Verified real (blind-hunter): a future rename or field addition on any of those structs silently desyncs the unknown-key detector (a false positive on a legitimate new field, or a missed typo) unless both places are updated by hand. `AwardFinalist.Position` being flagged as "unknown" inside `award_finalists.<award>` entries (accepted, since the runbook's own documented example never shows `position` there) is a live instance of exactly this drift. Fixing it structurally (e.g. reflection over the struct tags) is a bigger change than this story's spec asked for, given the schema is small and rarely changes.

## Deferred from: code review of spec-7-3-keep-written-yaml-yamllint-compliant.md (2026-09-27)

- source_spec: `_bmad-output/implementation-artifacts/spec-7-3-keep-written-yaml-yamllint-compliant.md`
  summary: The new `yamllint_test.go` and the pre-existing `docker-compose.yml` `lint-yaml` service both pin the floating `cytopia/yamllint:latest` tag — a future image update could silently change yamllint's behavior/version and make tests flake with no code change on this repo's side.
  evidence: Verified real (blind-hunter), but pre-existing: `docker-compose.yml` already used `:latest` before this story; this story only added a second consumer of the same tag. Worth pinning to a fixed tag or digest given a second consumer now depends on it staying stable, but not introduced by this diff.
- source_spec: `_bmad-output/implementation-artifacts/spec-7-3-keep-written-yaml-yamllint-compliant.md`
  summary: `results.team_marks.<division>.playoffs` **and `playoff_matchups.<round>`** (both a sequence nested inside a plain map-of-maps) still fail yamllint on the next write once hand-typed, because `CompactSeqIndent()` makes sequences indent 2 relative to their parent while ordinary map-nesting still renders at 4.
  evidence: Verified empirically (edge-case-hunter + verification-gap, independently; verification-gap reproduced the actual yamllint failure). Tried `SetIndent(4)` (today's setting, mismatch remains), `SetIndent(2)` (map-nesting matches sequences, but top-level sequences then render at 0 indent, which yamllint's default ruleset also rejects), and — during the epic-7 retrospective (2026-09-28) — `SetIndent(2)` combined with `CompactSeqIndent()` together (fails differently: top-level sequences still render at 0 indent, since a "compact" relative step against a base of 2 computes to 0). By construction, `CompactSeqIndent`'s "sequence indents less than its parent map" design can't be reconciled with plain map-nesting sharing the same increment via a single `SetIndent(n)` value tried so far.

  **Scope correction (epic-7 retrospective, 2026-09-28):** the original "no season has hand-typed results yet, so nothing breaks today" framing does not hold for `playoff_matchups:` — reproduced live: hand-typing it exactly per the runbook's own documented example, then triggering one real write, immediately produces real yamllint errors. Per the runbook's own round-progression rule, `playoff_matchups` must be hand-populated *before* `results.series` can even be recorded — i.e. at the start of every playoff round, not deferred to a hypothetical future season. **Also corrected: hand-typing at a different indent width (e.g. 2-space, to match the app's own generated style) does not avoid the mismatch either** — verified empirically that `yaml.Node` re-encoding never preserves original map-nesting indentation at all; it always re-derives it from the encoder's default step (4) regardless of what was hand-typed, so the mismatch reappears on the very next write no matter which convention the runbook recommends. This rules out "change the runbook's documented indent" as a viable fix, narrowing the real options to relaxing `.yamllint.yml`'s indentation rule or accepting the limitation with accurate framing.

  **Re-confirmed human decision (2026-09-28): accept as a known limitation, corrected framing.** No code change. `src/fantasy-hockey.yml` stays excluded from `.yamllint.yml`'s `ignore:` list (this never fails CI either way), so the only real-world impact is cosmetic to whoever opens the file by hand — which is real, not "nothing," but not worth the cost of relaxing the repo-wide lint rule or further encoder R&D at this project's scale. `docs/recording-results-and-playoffs.md` updated to state the mismatch accurately (real yamllint errors, not "purely cosmetic") and to cover `playoff_matchups:` explicitly, not just `results:`/`award_finalists:`.
