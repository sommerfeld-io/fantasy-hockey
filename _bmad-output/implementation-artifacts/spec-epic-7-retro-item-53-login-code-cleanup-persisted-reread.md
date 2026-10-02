---
title: 'Add a Persisted-File Re-Read Assertion to a Login-Code Cleanup Test'
type: 'test'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Every login-code cleanup test in `internal/store/store_test.go` (`TestSavePredictionShouldPruneAStaleLoginCodeRowOnItsNextWrite` and its used-row/should-not-prune siblings) asserts only against `st.doc.LoginCodes`, the in-memory field, never the actual bytes `writeLocked` persisted to disk. Since `s.doc.LoginCodes` is only assigned `cleaned` after a successful rename (`store.go`'s own doc comment on `writeLocked`), a test that only checks `st.doc.LoginCodes` can pass even if the on-disk splice were somehow wrong while the in-memory assignment stayed correct — the in-memory field and the persisted file are two different pieces of state that happen to be set together on the success path, and nothing today proves the second one directly (epic-7 retrospective, item 53).

**Approach:** Extend `TestSavePredictionShouldPruneAStaleLoginCodeRowOnItsNextWrite` (the central "cleanup piggybacks on any write" test) with a fresh `store.New(st.path)` re-read after the write, asserting the freshly-loaded store's `LoginCodes` also reflects the pruned state — proving the persisted file, not just the in-memory struct, was actually updated. One test is enough per the item's own "at least one" scope; the other two cleanup tests are unchanged.

</frozen-after-approval>

## Implementation Notes

Extended `TestSavePredictionShouldPruneAStaleLoginCodeRowOnItsNextWrite` in `internal/store/store_test.go`: after the existing `st.doc.LoginCodes` in-memory assertion, captures `st.path` under the read lock, then calls `New(path)` for a fresh re-read and asserts the freshly-loaded store's `LoginCodes` also reflects the pruned state. Pure test-coverage addition — no production code changed.

Not classic TDD red-green since no implementation changed. Instead, proved the new assertion actually catches a regression the old one wouldn't: temporarily made `writeLocked` encode `s.doc.LoginCodes` (uncleaned) into the persisted `login_codes` node while leaving the final `s.doc.LoginCodes = cleaned` in-memory assignment untouched (`internal/store/store.go` line ~1018) — a bug class where the on-disk state silently diverges from the in-memory state. Confirmed the new re-read assertion fails (`expected the pruned state to be persisted to disk, re-read got [...]`) while the pre-existing `st.doc.LoginCodes` check alone would have stayed green. Restored via `git checkout`.

Verified: `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, full `internal/store` suite green, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied the two `patch`-routed blind-hunter findings — added a second re-read assertion for `reread.doc.Predictions` (the same test's own `SavePrediction` call already exercises the sibling `predictions` splice `writeLocked` performs alongside `login_codes`, via the identical `spliceNamedValueLocked` mechanism; the bug class this item exists to catch was equally possible there and had zero persisted-file coverage anywhere in the suite); and expanded the explanatory comment to flag that only `login_codes`/`predictions` are safe to verify via this reread idiom — every other section (e.g. `newTestStore`'s in-memory-only `Players` seed) round-trips through `st.raw`'s parsed nodes rather than `st.doc`, so copying this pattern onto a Players/Teams-flavored test would silently always read back empty regardless of what was actually written. Four findings dispositioned `false`/`reject`: the `st.mu.RLock()` around capturing `st.path` is consistent with this file's existing pattern of reading Store fields under the lock (harmless, not actually ambiguous); an `if reread == st` identity guard defends against a hypothetical future caching change to `New()` that doesn't exist today (speculative, no signal it's coming); and a `t.Parallel()`-safety comment is moot since no test in this file uses `t.Parallel()` (verified: zero matches).

**Re-verified after patches**: `TestSavePredictionShouldPruneAStaleLoginCodeRowOnItsNextWrite` passes with both re-read assertions; the new `Predictions` assertion's regression-catching ability proven empirically the same way as `LoginCodes` (temporarily made `writeLocked` persist an empty `predictions` node while leaving `s.doc.Predictions` untouched, confirmed the test fails, restored via `git checkout`). `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, full `internal/store` suite green, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — the new re-read assertion covered `LoginCodes` but not `Predictions`, even though the same test's `SavePrediction` call exercises the sibling `predictions` splice through the identical `spliceNamedValueLocked` mechanism, leaving that bug class with zero persisted-file coverage anywhere in the suite. Verified real and nearly free to close in the same test. Fixed: added a `reread.doc.Predictions` assertion, proven to catch a forced regression.
- **low, patch** — no signal that checking only `LoginCodes` (before the fix above) was a deliberate scope decision rather than an oversight, and more importantly no warning that this reread idiom is unsafe to copy onto sections like `Players`/`Teams` that round-trip through `st.raw` rather than `st.doc` (`newTestStore`'s in-memory-only `Players` seed is a landmine sitting right next to this pattern). Verified real. Fixed: expanded the comment to name which sections are safe to verify this way and why.
- **low, reject** — `path := st.path` captured under `st.mu.RLock()` flagged as undocumented/inconsistent defensive style since `path` is never reassigned after construction. Not a defect: this file consistently reads Store fields inside the same locked section as other field reads (e.g. `st.doc.LoginCodes` immediately above it); harmless and not actually ambiguous to a reader familiar with the file's convention.
- **low, reject** — suggested a `reread == st` identity guard to protect against a future caching/memoization change to `New()`. Rejected: speculative, defends against a scenario `New()`'s current implementation gives no signal is coming (CLAUDE.md: don't validate scenarios that can't happen).
- **low, reject** — suggested a comment noting the reread idiom isn't `t.Parallel()`-safe without extra synchronization. Rejected: moot — verified zero tests in this file use `t.Parallel()`, so there is no current risk to document.
- **false** — spec frontmatter still showed `status: 'in-progress'` at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.

All patched findings were independently re-verified after patching: the extended test passes, the new `Predictions` assertion's regression-catching ability proven empirically, and the full `gofmt`/`go vet`/`task go:lint`/`task go:test`/`task go:run` pipeline clean.
