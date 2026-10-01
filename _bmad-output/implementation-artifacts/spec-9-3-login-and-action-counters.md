---
title: 'Login and Action Counters'
type: 'feature'
created: '2026-10-01'
baseline_commit: '4339b27c79be274b9eb2af3d8b1223dd4417a223'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-9-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `/metrics` shows HTTP traffic but not how often people log in or save predictions, and `observe.Audit` has no call sites.

**Approach:** `Audit` increments `fantasy_hockey_login_events_total{event}` and `fantasy_hockey_prediction_saves_total{kind}`. `internal/web` calls it at the points AD-33 names. `store.Save*` return the `[]store.Prediction` they persisted (AD-35), and `auth.RequestLoginCode` returns the Player id.

## Boundaries & Constraints

**Always:** Follow AD-33, AD-35, AD-36. Login series are labelled by `event` only (`login_code_requested`, `login_succeeded`, `login_failed`, `logout`); save series by `kind` only. All series are pre-registered at zero: login events in `observe.New`, one save series per `store.Kind` when `NewServer` starts. `Audit` stays the single emitter and never takes an email or code; it omits `player_id` when empty. Call it only from `internal/web`, and only when the outcome succeeded: `login_code_requested` only for a known email whose code row was persisted; `login_failed` for `ok == false` (an error or empty id is a `slog.Error`); `logout` only with a valid session cookie. Emit one `prediction_saved` per returned row, with `kind` and `set`, even when the call also returned an error; nothing for rows not returned. `store.Save*` returns a row only if it carries a pick. `internal/auth`, `internal/store` and `observe` import no new internal package.

**Never:** Put a Player, email, code or `set` id in any metric label. Touch the proxy, compose files, `Dockerfile` or `.github/workflows/**`. Add a second emitter.

## I/O & Edge-Case Matrix

| Scenario            | Input / State                                  | Expected Output / Behavior                                           | Error Handling                 |
|---------------------|------------------------------------------------|----------------------------------------------------------------------|--------------------------------|
| Wrong code          | `POST /login/code` with an unknown code        | `login_failed` +1, `login_succeeded` unchanged                       | N/A                            |
| Correct code        | `POST /login/code` with the issued code        | `login_succeeded` +1, `login_failed` unchanged                       | N/A                            |
| Code requested      | `POST /login` with a known email               | `login_code_requested` +1                                            | SMTP failure stays `slog.Error` |
| Unknown email       | `POST /login` with no matching Player          | No counter change                                                    | N/A                            |
| Logout              | `POST /logout` with a valid session            | `logout` +1                                                          | N/A                            |
| Logout, no session  | `POST /logout` without a valid cookie          | No counter change                                                    | N/A                            |
| Cup pick saved      | Open set, valid pick                           | `prediction_saves_total{kind="cup"}` +1                              | N/A                            |
| Rejected save       | Deadline passed                                | No save counter rises                                                | 403 as before                  |
| Multi-row save      | Divisions or awards save persisting N rows     | Matching `kind` series rise by exactly N                             | N/A                            |
| Blank submission    | All-blank or incomplete awards                 | Nothing rises                                                        | N/A                            |
| Partial failure     | Series loop saves 2 rows, third write fails    | Series counter +2, response 500 as before                            | Rows returned before the error |

</frozen-after-approval>

## Code Map

- `src/internal/observe/observe.go` -- add the two `CounterVec`s in `New`, pre-initialise login events there; `Audit` increments by `event`, and by the `kind` attr for `prediction_saved`; add a method (e.g. `PreRegisterKinds(kinds ...string)`) using `WithLabelValues(k).Add(0)`.
- `src/internal/store/store.go:96-130,669,733,819,921` -- `SavePrediction`, `SaveSeriesPick`, `SaveDivisionPicks`, `SaveAwardPicks` return `([]Prediction, error)`; add an exported list of all `Kind*` constants for pre-registration. Division upsert helper (~852) and award helper (~945) report whether they persisted a row. Division map order is random, so do not assert order.
- `src/internal/auth/auth.go:37` -- `RequestLoginCode` returns `(playerID string, err error)`; empty id means nothing persisted (unknown email, `generateCode` failure).
- `src/internal/web/web.go:83-105` -- pass `ob` to `handleLoginSubmit`, `handleLoginCodeSubmit`, `handleSheetSubmit`; `handleLogout` becomes `handleLogout(secret, ob)` and reads the cookie with `auth.ValidateSession`; call `ob.PreRegisterKinds(store.Kinds...)`.
- `src/internal/web/login.go:63-120` -- audit calls for the four login events.
- `src/internal/web/sheet.go:310-385`, `sheet_divisions.go:375-394`, `sheet_awards.go:364-372`, `sheet_series.go:286-297` -- audit each returned row; the series loop accumulates rows and audits them before the 500 or the rejected-pick page.
- Tests to update for the new return values: `internal/store/*_test.go` (about 54 calls), `internal/web/sheet*_test.go`, `internal/scoring`, `internal/standings`, `internal/store/yamllint_test.go`, and the acceptance steps for cup/presidents, playoffs cup, division, awards, series and leaderboard.
- `src/acceptance-tests/features/login-and-action-counters.feature` + steps -- new; register in `suite_test.go`. Reuse `extractSixDigitCode` (`login_steps_test.go:205`) and the open-set fixtures from the pick suites.
- `src/internal/observe/README.md`, `src/internal/web/README.md`, `docs/architecture.md` -- document the new series and the `Audit` behaviour.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` -- story status sync.

## Tasks & Acceptance

**Execution:**
- [x] `src/acceptance-tests/features/login-and-action-counters.feature` + steps -- Gherkin first, red before implementation -- BDD per project rules
- [x] `src/internal/observe/*` (tests first) -- counters, pre-registration, `Audit` increments -- AD-33/36
- [x] `src/internal/store/*` (tests first) -- `Save*` return persisted rows, exported kinds list -- AD-35
- [x] `src/internal/auth/auth.go` + tests -- `RequestLoginCode` returns the Player id
- [x] `src/internal/web/*` -- audit call sites and `NewServer` pre-registration; update every test and acceptance call site
- [x] READMEs, `docs/architecture.md`, `sprint-status.yaml` -- sync

**Acceptance Criteria:**
- Given a wrong login code is submitted, when I read `/metrics`, then the failure counter rose by exactly one and the success counter did not.
- Given a correct login code is submitted, when I read `/metrics`, then the success counter rose by exactly one and the failure counter did not.
- Given a prediction is saved successfully, when I read `/metrics`, then the matching `kind` counter rose by exactly one.
- Given a save is rejected after the deadline, when I read `/metrics`, then no save counter rose.
- Given any counter series, when I inspect its labels, then none carries a Player, email, code or set id.
- Given a save that persists several rows, or a partial failure after some rows persisted, when I read `/metrics`, then the counter rose by exactly the number of persisted rows, and an all-blank submission raises nothing.
- Given a fresh server, when I read `/metrics`, then every login and save series exists at zero.

## Implementation Notes

- Implemented by a subagent from this spec; acceptance feature written first (10 scenarios). Verified: `go test ./...`, `task lint`, `task docker:build`, `task go:run` start.
- Known gap: the I/O row "Partial failure" (third series write fails) has no direct test because the store cannot inject a mid-loop write failure; the shared deferred audit path is covered by the "+2 rows and a rejected series" test.

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| `store.Kinds` can drift from `Kind*` constants; test compares to a hand-copied list (3 layers) | low | A new kind needs a new handler beside the same const block; a missed entry only delays one series until its first save. Guard needs a larger test. Rejected. |
| `Audit` accepts any `kind` string / missing kind or unknown event not logged | low | Only `web` calls it, passing `row.Kind` from the store; guards add branches. Rejected. |
| Exported mutable `store.Kinds` | low | No importer mutates it; rejected. |
| nil Observer panics | low | Carried from 9.2 triage: every caller passes one. Rejected. |
| `login_failed` skipped for empty-id match, malformed form, empty code, store error | false | AD-33: only `ok == false` is `login_failed`; an error or empty id is a `slog.Error`. |
| Logout counted on replayed cookie; idle-expired cookie not counted | false | AD-33 counts a logout carrying a valid session cookie; the session is stateless by AD-11. |
| Cleared division playoff-teams row not counted | false | AD-35: a row is returned only if it carries a pick. |
| `login_code_requested` counts codes whose send later fails | false | AD-33: SMTP success is not required. |
| Series audit runs in a `defer` after the response is written | low | net/http flushes after the handler returns, so counters are visible to the next scrape; no panic path shown. Rejected. |
| `prediction_saves_total` help text vs. "rows saved" semantics | false | Help says "Prediction rows saved"; non-empty upserts are exactly the rows AD-35 defines. |
| `Save*` single-row methods return a slice | false | Spec and AD-35 require `[]store.Prediction` on every `Save*`. |
| Audit omits `player_id` when empty | false | Spec decision; no consumer of the old shape exists (9.4 adds none yet). |
| Partial-failure matrix row has no direct test (3 layers) | low | The store cannot inject a mid-loop write failure. The same deferred audit is exercised by the "+2 rows and a rejected series" test; the 500 path differs only by an early return. Flagged to the human, not patched. |
| Acceptance covers only the cup save path end to end | low | Multi-row, blank and rejected saves are unit tested in `counters_test.go`; the spec asks for one scenario per AC at the observable level, met for login and cup. |
| Acceptance step: `requireCounter` substring match, regex compiled in loop, `"predict"` literal, sleep polling | low | Cosmetic; the " N\n" pattern needs a leading space, so `1` cannot match `10`. Rejected. |
| Architecture doc does not record AD-35/36 | false | Both live in the architecture spine; code comments cite them. |
| `Audit` relies on caller discipline for email/code | low | Same as 9.2; no caller passes either. Rejected. |

## Design Notes

`observe` cannot import `store`, so `NewServer` hands it the `store.Kind` list. Because the call sites emit the audit line as well as the counter, Story 9.4 mainly adds its log-line tests and any gaps; it adds no second emitter.

## Verification

**Commands:**
- `task go:test` -- expected: unit tests pass
- `task go:test:acceptance` -- expected: acceptance tests pass, including the new feature
- `task lint` -- expected: all linters pass
- `task go:run` -- expected: app builds and starts
- `task docker:build` -- expected: image builds and lints (feature complete)
