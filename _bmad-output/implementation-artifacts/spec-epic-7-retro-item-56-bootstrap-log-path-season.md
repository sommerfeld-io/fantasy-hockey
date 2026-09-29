---
title: 'Add path/season Fields to the Bootstrap Log Line'
type: 'chore'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `writeLocked`'s single `slog.Info("store write", "reason", reason)` call logs only `reason`. This partially closed epic-6-retro-item-43 ("a bootstrap log line... should log at info level when it creates a fresh file (path + season)") — Story 7.1 added the bootstrap-distinct log line, but never the `path`/`season` fields the item specifically asked for (epic-7 retrospective, item 56).

**Approach:** Add an optional variadic `extra ...any` parameter to `writeLocked`, appended to the existing `"reason", reason` key-value pair before logging. Only `New`'s bootstrap branch passes `"path", path, "season", st.doc.Season` through it; every other one of `writeLocked`'s 8 other call sites stays unchanged (zero extra args), preserving Story 7.1's "one generic log call, no duplicated log statements per site" design and its established test shape (`assertExactlyOneInfoLine` still holds for every write).

</frozen-after-approval>

## Implementation Notes

`writeLocked` gained a variadic `extra ...any` parameter, appended to the logged fields via `append([]any{"reason", reason}, extra...)...`. Only `New`'s bootstrap branch passes `"path", path, "season", st.doc.Season`; all 8 other call sites unchanged (zero extra args). TDD: extended `TestNewShouldLogTheBootstrapWrite` first (confirmed red - failed on the missing `path=`/`season=` fields), then implemented (green).

Verified: full `internal/store` suite (156 tests, 0 failures - exact same count as before this change, confirming zero regressions to any other call site), `gofmt -l`, `go vet`, `task go:test` (full pipeline), `task go:run`. **Live end-to-end check with the real binary**: bootstrapping a fresh file logs `store write reason="bootstrap data file" path=./seed.yml season=2026-27` exactly as intended; a real login-code-creation write (via an actual `POST /login` through the running server) logs `store write reason="create login code"` with no extra fields, confirming every other call site's log line is genuinely unchanged, not just claimed to be.

Nothing incomplete or risky.

**Review patches:** applied all five surviving `patch`-routed findings — made the bootstrap test's `path=`/`season=` assertions robust to `slog`'s value-quoting (verified empirically: a value containing a space gets wrapped in quotes; the original `"path="+path` concatenation check would have silently failed to match in that case); added a should-not counterpart to a representative non-bootstrap test (`TestCreateLoginCodeShouldLogOnASuccessfulWrite`), asserting `path=`/`season=` never leak onto any other call site's log line; documented that `extra`, like `reason`, may never carry a raw login code, player email, or other identifying/secret value; and cross-referenced the bootstrap logging side effect from `New`'s own doc comment, not just `writeLocked`'s.

Two findings noted but not acted on: the logged `path` staying relative/unresolved (exactly whatever was passed to `New`) matches the literal ask and is arguably more useful for correlating against an operator's own `DATA_FILE` config than a resolved absolute path would be - not changed. The Gherkin/acceptance-test policy question was already resolved by the user's standing decision from the epic-8-item-63 build (pure hardening, no observable behavior change → decide silently) - this change (log fields only, no new HTTP-observable behavior) qualifies.

**Re-verified after patches**: full `internal/store` suite still 156/156 pass (0 regressions), `gofmt -l`, `go vet`, `task go:test`, `task go:run` all clean.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — test assertions assumed `slog`'s TextHandler always emits `path`/`season` unquoted; a value containing a space gets quoted, which a bare `"path="+path` concatenation check would miss. Verified empirically against the real handler. Fixed: check for the key and the raw value as two separate substrings instead.
- **low, patch** — no should-not counterpart proving the other 8 call sites don't leak `path=`/`season=`. Verified real. Fixed: added the check to a representative existing test.
- **low, patch** — no documented constraint on what `extra` may carry (PII/secrets). Verified real. Fixed: doc comment now states the same hash-code-never-logged rule applies to `extra` as to `reason`.
- **low, patch** — `New`'s own doc comment didn't mention the bootstrap logging side effect, only `writeLocked`'s did. Verified real. Fixed: cross-referenced.
- **low, reject** — the logged path stays relative/unresolved rather than absolute. Matches the literal ask; resolving to absolute adds complexity and a new error path for unclear benefit, and arguably makes the log line *less* useful for an operator correlating it against their own `DATA_FILE` config.
- **false** — exact-field-count/no-duplicate-`season=` check missing from the test. Not asked for by this item, not this file's existing test convention (no sibling log-line test in this file counts fields either), and the risk (a future hand-edit introducing a stray duplicate) is speculative, not demonstrated.

All four code/doc findings were independently re-verified after patching: full `internal/store` suite re-run (156/156, unchanged), and the quoting-robustness fix confirmed against the same empirical probe that found the gap.
