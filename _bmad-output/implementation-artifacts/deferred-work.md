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
