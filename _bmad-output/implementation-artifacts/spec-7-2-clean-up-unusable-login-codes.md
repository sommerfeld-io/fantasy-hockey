---
title: 'Clean Up Unusable Login Codes'
type: 'feature'
created: '2026-09-27'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: []
baseline_commit: 'ef8af9071af89dc9d14e4fb75c212f80454b14f4'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `fantasy-hockey.yml`'s `login_codes` list only ever grows — expired and already-used rows stay in the hand-maintained file forever, so it never stays small and never shows only genuinely redeemable codes.

**Approach:** Opportunistically prune expired and used `LoginCode` rows from `writeLocked`, the store's single write path, so any write that already happens (for any reason) also drops rows that can no longer be used to log in.

## Boundaries & Constraints

**Always:**
- Cleanup runs inside `writeLocked` only, so every one of its call sites benefits uniformly — a write triggered by an unrelated mutation (e.g. `SavePrediction`) also prunes stale login codes, matching every AC's "when the store next writes the data file" wording.
- Cleanup commits its result to `s.doc.LoginCodes` in memory only after a successful write; on failure `s.doc.LoginCodes` is left exactly as the calling method's own mutation left it, so `CreateLoginCode`'s and `ConsumeLoginCode`'s existing per-caller rollbacks stay correct, untouched.
- A row whose `issued_at` fails to parse is left alone — never destroy data the code can't confirm is expired.
- `CreateLoginCode`'s `issuedAt string` parameter becomes `now time.Time`; `issuedAt` is computed internally from it exactly like every other mutating method computes its own timestamp.
- A resubmission of a code whose row cleanup already removed is rejected with the exact same generic (`ok=false, err=nil`) outcome as any other wrong/expired/used code — already guaranteed by `ConsumeLoginCode`'s existing no-match linear scan; add a regression test, no behavior change needed.
- **Decided 2026-09-27 (human-confirmed):** no Gherkin acceptance test for this story — nothing in an HTTP response or rendered page changes, so unit coverage on `internal/store` is the right and only tool.

**Never:**
- No new write mechanism, background goroutine, lock, or scheduled job, and no on-demand CLI flag — explicitly out of scope, parked as a fallback per the epic.
- Don't force a write when `store.New` loads an *existing* file — cleanup only piggybacks on a write that already happens for another reason; no AC calls for a write at ordinary startup.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Expired row present | row's `issued_at` > 10 min before `now`, any write occurs | row removed from `s.doc.LoginCodes` and the persisted file | N/A |
| Used row present | row has `used_at` set, any write occurs | row removed | N/A |
| Fresh, unused row | row's `issued_at` < 10 min before `now`, any write occurs | row stays, fields unchanged | N/A |
| Row with unparseable `issued_at` | malformed `IssuedAt`, any write occurs | row stays untouched | N/A |
| Resubmission after cleanup | a code's row was removed by an earlier write, then resubmitted | `ConsumeLoginCode` returns `ok=false, err=nil` — identical to any other non-match | N/A |
| Write fails mid-cleanup | any mutating call, `writeLocked`'s rename fails | error returned as before; `s.doc.LoginCodes` left uncleaned, in-flight caller rollback still correct | unchanged existing error wrapping |

</frozen-after-approval>

## Code Map

- `internal/store/store.go:867` -- `writeLocked(reason string) error`, the sole write path: add a `now time.Time` param; compute `cleaned := cleanupLoginCodes(s.doc.LoginCodes, now)`; marshal a copy of `s.doc` with `LoginCodes` swapped to `cleaned` (see Design Notes for why not `s.doc` itself); assign `s.doc.LoginCodes = cleaned` only after a successful rename.
- `internal/store/store.go:479` -- `CreateLoginCode(playerID, codeHash, issuedAt string)` -> `(playerID, codeHash string, now time.Time)`, computing `issuedAt := now.UTC().Format(time.RFC3339)` internally (`SavePrediction`'s pattern, :557).
- `internal/store/store.go:508,332,557,621,707` -- `ConsumeLoginCode`/`SavePrediction`/`SaveSeriesPick`/`SaveDivisionPicks`/`SaveAwardPicks` already have `now`; just forward it to `writeLocked`. `New`'s bootstrap branch has no `now` in scope; pass `time.Now().UTC()` inline (no `internal/clock` import; `LoginCodes` is always empty there, so the value is inert). `ConsumeLoginCode`'s existing no-match scan already satisfies the resubmission-after-cleanup AC untouched.
- `internal/store/store_test.go` -- 7 `CreateLoginCode("basti", "hash-1", "2026-09-14T10:00:00Z")`-shaped calls need the new signature. One genuine regression: `TestConsumeLoginCodeShouldNotTouchAnyOtherRow` asserts `st.doc.LoginCodes[1].UsedAt` by fixed index after consuming the first row — cleanup now removes that row in the same write, shifting the second to index `[0]`; find it by `CodeHash` instead.
- `internal/auth/auth.go:50-51`, `internal/auth/validate_test.go:32`, `acceptance-tests/enter_login_code_steps_test.go:110`, `internal/web/login_test.go:242` -- each already holds a `time.Time` before formatting it for the old signature; drop the `.Format(time.RFC3339)` call and pass the value directly.

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/store.go` -- add `cleanupLoginCodes(rows []LoginCode, now time.Time) []LoginCode`: drops a row when `UsedAt != nil`, or when `IssuedAt` parses and `now.Sub(issuedAt) > loginCodeValidity`; keeps order; keeps a row whose `IssuedAt` fails to parse.
- [x] `internal/store/store.go` -- thread `now time.Time` through `writeLocked` and all 7 call sites per the Code Map; commit the cleaned slice to memory only after a successful write.
- [x] `internal/store/store_test.go` -- unit-test `cleanupLoginCodes` directly: removes an expired row, removes a used row, keeps an unexpired+unused row, keeps a row with unparseable `issued_at` (should-not case), preserves order of kept rows.
- [x] `internal/store/store_test.go` -- Store-level tests: a stale row is gone after the store's next write; a used row is gone after the next write; an unexpired/unused row survives a write untouched; a resubmission of a cleaned-up code still returns `ok=false, err=nil` via `ConsumeLoginCode`.
- [x] `internal/store/store_test.go` -- update the 7 existing `CreateLoginCode` call sites for the new signature; fix `TestConsumeLoginCodeShouldNotTouchAnyOtherRow`'s index-based assertion per the Code Map.
- [x] `internal/auth/auth.go`, `internal/auth/validate_test.go`, `acceptance-tests/enter_login_code_steps_test.go`, `internal/web/login_test.go` -- update each `CreateLoginCode` call site for the new signature.

**Acceptance Criteria:**
- Given a `LoginCode` row older than `loginCodeValidity` relative to `now`, when the store next writes the data file, then that row is removed from `login_codes`.
- Given a `LoginCode` row that has already been used, when the store next writes, then that row is removed.
- Given a `LoginCode` row that is unexpired and unused, when the store next writes, then that row remains untouched.
- Given a resubmission of a code whose row was just removed by cleanup, when it is validated, then it is rejected with the exact same generic outcome as any other wrong/expired/used code.

## Implementation Notes

Implemented as designed: `cleanupLoginCodes` is a pure helper; `writeLocked` marshals a copy of `s.doc` with `LoginCodes` swapped to the cleaned slice, only committing `s.doc.LoginCodes = cleaned` after a successful rename. `CreateLoginCode` now takes `now time.Time` (issuedAt computed internally); all 9 literal `writeLocked` call sites (bootstrap + 8 mutating-method sites) forward their own `now`. The 4 external `CreateLoginCode` callers (`internal/auth/auth.go` and 3 test helpers) each already held a `time.Time` before formatting it for the old signature, so each edit was a one-line simplification, not a behavior change.

Two pre-existing tests needed fixing as a direct, inevitable consequence of cleanup now running on every write (not called out individually in the Code Map, but anticipated by its "resubmission" and "index shift" notes): `TestConsumeLoginCodeShouldMatchAnUnusedUnexpiredCode` asserted `used_at` was set on the just-consumed row, but that row is itself pruned by the very same write (per the matrix's "Used row present" row) — rewritten to assert the row's absence instead. The acceptance step `theLoginCodeIsMarkedUsed` in `enter_login_code_steps_test.go` had the identical stale assumption — same fix, with a comment explaining why.

**Matrix Test Audit (step-03):** the "Write fails mid-cleanup" row's error-return half was covered by existing rollback tests, but none of them seeded an unrelated stale/used row and confirmed it survives untouched when the write fails (the actual safety property this story's Design Notes are about). Closed by adding `TestSavePredictionShouldNotCommitLoginCodeCleanupWhenTheWriteFails`, which seeds an expired row, forces a `SavePrediction` write to fail, and asserts the stale row is still present afterward. `gocyclo -over 10 .` and the full test suite (`go test ./...`) re-run clean after the addition.

**Review patches (step-04):** applied the three surviving `patch`-routed findings from the Review Triage Log — corrected `CreateLoginCode`'s stale doc comment (it can still trigger cleanup of *other* rows even though it never mutates one itself), added `TestCleanupLoginCodesShouldKeepARowExactlyAtTheValidityBoundary` proving the strict `>` comparison, and renamed the "marked used" step/binding (feature text, `ctx.Step` regex, and Go function) to "the login code is no longer usable" to match the actually-observed behavior. `go vet`, `gofmt -l`, `gocyclo -over 10 .`, and the full `go test ./...` (including the acceptance suite with the renamed step) all re-run clean.

## Review Triage Log

- **low / patch** — `CreateLoginCode`'s doc comment still says "It never mutates or removes any existing row," but its own `writeLocked` call now runs cleanup, which can remove *other* expired/used rows as a side effect of this same write (edge-case-hunter, verified against `store.go:475`). Fix: correct the comment.
- **low / patch** — `enter-login-code.feature`'s step text ("And the login code is marked used") and its Go binding (`theLoginCodeIsMarkedUsed`, `ctx.Step` regex) still describe the pre-7.2 behavior (a `used_at` field) when the actually-observed behavior is the row's removal (blind-hunter + edge-case-hunter, both flagged independently; verified the step text is used in exactly one feature file, so the rename is isolated). Fix: rename the step text, regex, and function consistently.
- **low / patch** — `cleanupLoginCodes`'s unit tests cover clearly-expired (-11 min) and clearly-fresh (-5 min) rows but never the exact boundary (`now.Sub(issuedAt) == loginCodeValidity`), leaving the strict `>` comparison's edge untested (blind-hunter). Fix: add one boundary test asserting an exactly-10-minute-old row is kept.
- **false** — "Removed the direct assertion that `ConsumeLoginCode` sets `used_at`... a future bug that removes the row without ever setting `used_at` would pass undetected" (edge-case-hunter, two instances: the unit test and the acceptance step). Verified: by design, a successfully-consumed row's `used_at` is *never* observable on disk or in `s.doc` after the call returns — `cleanupLoginCodes` sees it set and prunes it in the same write, before either ever surfaces it. A hypothetical alternate implementation that deleted the row directly (never setting `used_at` at all) would be **permanently indistinguishable** from the current one in every externally observable way (returned `playerID`, persisted file, `s.doc` state) while still satisfying every AC. Per this repo's own testing philosophy ("test observable behavior, not internal state"; the Implementation-Swap Test), asserting `used_at` specifically was set is no longer a behavior claim this design can make — it's an implementation detail the change has deliberately made unobservable. No real regression could occur here that the existing playerID-match and row-absence assertions don't already catch.
- **false** — "Code Map's call-site inventory reads as five sites... nine literal call sites needed threading" and "Code Map calls out only one genuine regression... a second surfaced" (blind-hunter). Both true as written, but their only fix is editing this build's spec's own prose — out of scope for a code patch, and the underlying facts are already correctly captured elsewhere (Implementation Notes' "9 literal call sites," and the second regression's actual fix, both already done).
- **false** — "Verification section omits `task lint` and `task docker:build`" (blind-hunter). True of the spec text, but its only fix is a spec edit; `task lint` already runs via this repo's pre-commit hook regardless of what the spec lists, and `task docker:build` was run manually during implementation (its Go-level stages passed; only the final image-pull step failed on this sandbox's lack of Docker Hub network access, a documented environmental limitation, not a code issue).
- **false** — "Review Triage Log ships empty... ambiguous" (blind-hunter). Expected per this template's own documented convention ("Empty until the first review pass") — this review pass is what populates it.
- **false** — "Residual limitation (stale rows persist if no other write ever happens again) never discussed as a known limitation" (blind-hunter). Already a deliberate, named trade-off in the epic's own source material (an on-demand CLI flag is explicitly parked as a fallback "if opportunistic cleanup proves insufficient"), not a gap introduced by this spec; its only fix would be a spec edit either way.
- **false** — "No way to tell from production logs whether cleanup removed any rows... a count would help" (blind-hunter). An enhancement suggestion, not a defect — no AC requires it and no concrete harm is named beyond general "would help confirm the feature is doing something."
- **false** — "`context: []`/frontmatter metadata unexplained" (blind-hunter). Cosmetic spec-metadata observation with no behavioral consequence; its only fix is a spec edit.

## Design Notes

`writeLocked` currently marshals `s.doc` directly. Cleanup must not mutate `s.doc.LoginCodes` before a write is confirmed durable, or a failed write would leave the caller's own narrower rollback (e.g. `CreateLoginCode`'s `s.doc.LoginCodes = s.doc.LoginCodes[:len(s.doc.LoginCodes)-1]`) truncating the wrong element once cleanup has already spliced out unrelated rows earlier in the slice. Fix: compute the cleaned slice, marshal a *copy* of `s.doc` with `LoginCodes` swapped to it, and only assign `s.doc.LoginCodes = cleaned` after `os.Rename` succeeds. This keeps every existing per-caller rollback correct without touching any of their rollback logic.

## Verification

**Commands:**
- `cd src && go test ./internal/store/... ./internal/auth/... ./internal/web/... ./acceptance-tests/...` -- expected: all new and existing tests pass
- `task go:test` -- expected: full unit suite green, coverage report written
- `task go:test:acceptance` -- expected: GoDog suite green (call sites in `acceptance-tests/` still compile and pass)
- `task go:run` -- expected: app still builds and starts
