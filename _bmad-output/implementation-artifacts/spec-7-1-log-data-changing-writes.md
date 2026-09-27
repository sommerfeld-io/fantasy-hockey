---
title: 'Log Data-Changing Writes'
type: 'feature'
created: '2026-09-27'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: []
baseline_commit: 'e6bcf11b6007ed4cbbe0507b22415fc43a0b70cc'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `internal/store`'s single write path never logs anything, so the person running the pool has no way to see what changed on disk without opening `fantasy-hockey.yml` or grepping raw code hashes.

**Approach:** Emit exactly one structured `slog.Info` line whenever a Store method's write actually reaches disk, worded distinctly for the one-time bootstrap write versus every other in-life write, and never containing a raw login code or player email.

## Boundaries & Constraints

**Always:**
- Log only after a write that actually succeeded; a failed write logs nothing new (existing error-return behavior is unchanged), and a call that never reaches `writeLocked` at all logs nothing.
- Every logged field is an id/hash/count — never a raw login code or player email, matching the existing hash-code-never-logged convention.
- Use the existing `log/slog.Info` convention already used elsewhere in this codebase (short message + key-value pairs).
- The bootstrap write (`store.New` creating a file that didn't exist) reads as distinct from every other mutating call's line.
- **Decided 2026-09-27 (human-confirmed):** no Gherkin acceptance test for this story — nothing in an HTTP response or rendered page changes, so unit coverage on `internal/store`/`internal/auth` (asserting `slog` output via the existing `captureLogs` pattern) is the right and only tool.

**Never:**
- No second write mechanism, lock, background goroutine, or scheduled job — log only from the existing `writeLocked` path and its 7 call sites (`New`'s bootstrap branch, `CreateLoginCode`, `ConsumeLoginCode`, `SavePrediction`, `SaveSeriesPick`, `SaveDivisionPicks`, `SaveAwardPicks`).
- No logging inside error branches — errors keep returning/wrapping, not logging, per this repo's existing layering rule.
- Don't touch `main.go`'s existing `ResultProblems` warning logging.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Bootstrap write | `store.New(path)`, path doesn't exist | one info line, wording distinguishes it as the bootstrap write | N/A |
| Existing file loaded | `store.New(path)`, path exists and parses | no write occurs — no log line | N/A |
| Successful mutating call | any of the 6 mutating methods succeeds | exactly one info line, fields limited to ids/hashes/counts | N/A |
| No-op: unmatched login request | `auth.RequestLoginCode` with an email matching no Player | `CreateLoginCode`/`writeLocked` never called — no log line | N/A |
| No-op: unmatched code | `ConsumeLoginCode` with no matching row | no write — no log line | N/A |
| Write fails (disk/rename error) | any mutating method, `writeLocked` returns an error | error returned as today; no success log line | unchanged existing error wrapping |

</frozen-after-approval>

## Code Map

- `internal/store/store.go` -- sole write path; `writeLocked` (~L861-886) is where every mutating method converges; `New`'s bootstrap branch (~L334-341) is the one write outside a named mutation method. The 6 other call sites: `CreateLoginCode` (~L478), `ConsumeLoginCode` (~L507, has a no-match branch that never reaches `writeLocked`), `SavePrediction` (~L556), `SaveSeriesPick` (~L620), `SaveDivisionPicks` (~L706), `SaveAwardPicks` (~L808). No `log/slog` import exists in this file today.
- `internal/store/store_test.go` -- package `store` (white-box); add a package-local `captureLogs` helper mirroring `internal/auth/auth_test.go`'s existing one (that file's `captureLogs`, ~L22-30) — can't be imported across packages, so duplicate the ~8-line helper here.
- `internal/auth/auth.go` -- `RequestLoginCode`'s no-match branch (~L39-42) already returns before calling `st.CreateLoginCode`, so the no-op-means-no-log requirement is satisfied by existing control flow; no production change needed, only a regression test.
- `main.go` -- `openStore` (~L63-77): once `store.New`'s bootstrap branch logs distinctly, this closes the open epic-6 retro action item asking for a bootstrap log line here — no change needed to `openStore` itself.

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/store.go` -- import `log/slog`; give the write path a way to log one distinct message per caller after a successful write (see Design Notes), covering all 7 sites including `New`'s bootstrap branch.
- [x] `internal/store/store_test.go` -- add the `captureLogs` helper; add one test per mutating method (bootstrap, `CreateLoginCode`, `ConsumeLoginCode` match, `SavePrediction`, `SaveSeriesPick`, `SaveDivisionPicks`, `SaveAwardPicks`) asserting exactly one info line, bootstrap wording distinct from the rest, and no raw code/email in any field.
- [x] `internal/store/store_test.go` -- add a should-not test: `ConsumeLoginCode` with no matching row logs nothing.
- [x] `internal/auth/auth_test.go` -- add a should-not test: `RequestLoginCode` with an unmatched email logs nothing about a write. (Already present: `TestRequestLoginCodeShouldNotLogOnNoMatch`, from an earlier story, already asserts zero log output on the unmatched-email path -- covers the write-silence requirement too, since `CreateLoginCode` is never reached. No new test added here.)

**Acceptance Criteria:**
- Given any Store mutating method succeeds in writing the file, when the write completes, then exactly one `slog.Info` line is emitted with no raw login code or player email in its fields.
- Given `store.New` bootstraps a brand-new file, when that write completes, then its log line's wording is distinct from every other mutating method's line.
- Given a call results in no write at all, when it completes, then no log line is emitted.

## Implementation Notes

`writeLocked` now takes a `reason string` parameter and logs `slog.Info("store write", "reason", reason)` once, immediately after a successful rename, before returning nil. Every one of the 7 call sites passes its own literal reason: `"bootstrap data file"` (`New`), `"create login code"`, `"consume login code"`, `"save prediction"` (both the update-in-place and append branches), `"save series pick"` (both branches), `"save division picks"`, `"save award picks"`. No other field is logged, so the "never a raw login code or player email" constraint holds trivially - `reason` is always a fixed literal, never derived from caller input.

Added to `internal/store/store_test.go`: `captureLogs` and `assertExactlyOneInfoLine` helpers, plus `TestNewShouldLogTheBootstrapWrite`, `TestNewShouldNotLogWhenLoadingAnExistingFile`, `TestCreateLoginCodeShouldLogOnASuccessfulWrite`, `TestConsumeLoginCodeShouldLogOnASuccessfulWrite`, `TestConsumeLoginCodeShouldNotLogOnNoMatch`, `TestSavePredictionShouldLogOnASuccessfulWrite`, `TestSaveSeriesPickShouldLogOnASuccessfulWrite`, `TestSaveDivisionPicksShouldLogOnASuccessfulWrite`, `TestSaveAwardPicksShouldLogOnASuccessfulWrite`.

`internal/auth/auth.go` and `internal/auth/auth_test.go` were left untouched, per the Code Map's note that the no-op no-match path already returns before any store write, and an existing regression test already proves no log output for that path.

Verified: `go test ./internal/store/... ./internal/auth/...`, `task go:test` (full suite + coverage, golangci-lint 0 issues, govulncheck no findings), and `task go:run` (binary builds, starts, listens on :8080; no bootstrap log line appeared since `fantasy-hockey.yml` already existed).

**Matrix Test Audit (step-03):** the "Write fails" row's error-return half was already covered by the pre-existing rollback tests, but none of them asserted the "no success log line" half. Closed the gap by adding a `captureLogs` call and a "no log output" assertion to `TestCreateLoginCodeShouldRollBackTheAppendWhenTheWriteFails` (representative of all 7 call sites, since the log call sits structurally after every error-return branch in `writeLocked` -- it's unreachable on failure regardless of which caller invoked it). `go test ./internal/store/...` re-run green after the addition.

**Review patches (step-04):** applied all three surviving `patch`-routed findings from the Review Triage Log — added a log assertion to the update-in-place resubmission in both `TestSavePredictionShouldUpdateAnExistingRowInPlaceOnResubmission` and `TestSaveSeriesPickShouldUpdateAnExistingRowInPlaceOnResubmission`, and added the same `captureLogs` + "no log output" assertion to the remaining 9 write-failure rollback tests (`ConsumeLoginCode`, `SavePrediction` x2, `SaveDivisionPicks` x2, `SaveAwardPicks` x2, `SaveSeriesPick` x2). `go test ./internal/store/...` and the full `task go:test` suite re-run green with no regressions; `gofmt -l` clean.

**Commit-time lint fix:** the pre-commit hook's `gocyclo -over 10` check failed on `TestSaveDivisionPicksShouldRollBackTheUpdatesWhenTheWriteFails` (complexity 11) after the review patch above added one more inline `if logs.Len() != 0 { ... }` branch to an already-branch-heavy test. Fixed by extracting a shared `assertNoLogOutput(t, logs)` helper (mirroring `assertExactlyOneInfoLine`) and using it at all 12 no-log-assertion call sites in the file, removing one branch per caller. `gocyclo -over 10 .` and the full test suite both re-run clean.

## Spec Change Log

## Review Triage Log

- **low / patch** — `SavePrediction`'s update-in-place branch (`store.go`, `writeLocked("save prediction")` at the update site) has no test asserting its log line. `TestSavePredictionShouldLogOnASuccessfulWrite` only drives the append branch (fresh store, no existing row); `TestSavePredictionShouldUpdateAnExistingRowInPlaceOnResubmission` drives the update branch but never captures logs. Confirmed independently by the verification-gap and blind-hunter layers reading the same two tests. Fix: add a log assertion to a second call in the resubmission test (or a dedicated test).
- **low / patch** — Same gap for `SaveSeriesPick`'s update-in-place branch — `TestSaveSeriesPickShouldLogOnASuccessfulWrite` only drives the append branch; `TestSaveSeriesPickShouldUpdateAnExistingRowInPlaceOnResubmission` drives the update branch untested for logging. Same fix pattern.
- **low / patch** — The "no log on write failure" guarantee is only asserted for 1 of the 7 write call sites (`TestCreateLoginCodeShouldRollBackTheAppendWhenTheWriteFails`, added during the Matrix Test Audit). The other 9 rollback tests (`ConsumeLoginCode`, `SavePrediction` x2, `SaveSeriesPick` x2, `SaveDivisionPicks` x2, `SaveAwardPicks` x2) don't carry the same assertion, even though the guarantee is structurally true today (the log call sits after every error-return in `writeLocked`). Fix: add the same `captureLogs` + zero-log assertion to each of the other 9.
- **low / patch** (self-fix, not code) — `sprint-status.yaml` still reads `in-progress` for this story while code review is underway; its own header comment says a story moves to `review` once code-review starts. Harmless, one-line bookkeeping fix.
- **false** — "Logging placed in a lower-level package, not a boundary layer" (blind-hunter). `src/CLAUDE.md`'s layering rule ("lower-level packages should return or wrap errors, not log them... boundary layers decide whether to log") is scoped to error handling; this diff adds INFO logging only on write *success*, a path that rule doesn't address. Story 7.1's own acceptance criteria (epics.md) explicitly requires `internal/store` itself to emit this line, since only `writeLocked` reliably knows whether a write occurred at all (the no-op requirement, AC3) — this is spec-mandated design (frozen Intent + Design Notes), not an oversight.
- **false** — "Status vocabulary mismatch: spec `in-review` vs sprint-status.yaml `review`" (blind-hunter). The two fields are read/written by disjoint tooling within this same workflow (spec-template.md's own enum vs. sync-sprint-status.md's separate enum); nothing cross-references them, so the differing spelling causes no actual confusion. Pre-existing design in the workflow's own templates, not introduced by this diff.
- **false** — "'7 call sites' framing imprecise" (blind-hunter). Its only fix is rewording this spec's own Code Map/Implementation Notes text, which is out of scope for a code patch; the concrete consequence it raised (untested branches) is already captured and fixed by the first two rows above.
- **false** — "Global-logger test seam is a latent flakiness risk" from a future `t.Parallel` addition (blind-hunter). No test in `internal/store` or `internal/auth` uses `t.Parallel` today, so the described race cannot occur now; this is a pre-existing risk inherent to the `captureLogs` pattern already used in `internal/auth/auth_test.go` before this story, not something this diff introduced or changed the exposure of.
- Edge-case-hunter layer: returned `[]` (no findings).

## Design Notes

`writeLocked` itself doesn't know which caller invoked it. Give it (or a thin wrapper each of the 7 sites calls instead of `writeLocked` directly) a `reason string` parameter — each site passes its own literal (e.g. `"bootstrap data file"`, `"create login code"`, `"consume login code"`, `"save prediction"`, `"save series pick"`, `"save division picks"`, `"save award picks"`) — and log once from that one place with `reason` as a field. This keeps "one generic mechanism" (AD-29: store is the sole writer) while still letting the bootstrap line read differently, without duplicating a log call at every site.

## Verification

**Commands:**
- `cd src && go test ./internal/store/... ./internal/auth/...` -- expected: all new and existing tests pass
- `task go:test` -- expected: full unit suite green, coverage report written
- `task go:run` -- expected: app still builds and starts
