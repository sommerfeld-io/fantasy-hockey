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
