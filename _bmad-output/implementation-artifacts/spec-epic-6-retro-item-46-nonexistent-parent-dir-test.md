---
title: 'Add a Nonexistent-Parent-Directory Test for store.New'
type: 'test'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `store.New`'s bootstrap branch (triggered by `os.ErrNotExist` on the initial `os.ReadFile`) is exercised today only for a resolved path whose parent directory exists. Nothing proves what happens when an operator misconfigures `DATA_FILE`/`--data-file` to point inside a directory that doesn't exist yet - a real, plausible operator mistake (typo in the path, wrong volume mount) - or that the resulting error is legible enough to diagnose (epic-6 retrospective, item 46).

**Approach:** Add a test to `internal/store/store_test.go` seeding a path whose parent directory was never created, calling `New(path)`, and asserting: it returns a non-nil error (not a panic, not a silently-created store), the returned `*Store` is nil, the error satisfies `errors.Is(err, fs.ErrNotExist)` (a portable, non-string-matching legibility check), and the error's message names the actual attempted path so an operator can see exactly which location failed.

</frozen-after-approval>

## Implementation Notes

Added `TestNewShouldReturnAnOperatorLegibleErrorForANonexistentParentDirectory` to `internal/store/store_test.go`: seeds a path whose parent directory was never created (`t.TempDir()/nonexistent-subdir/fantasy-hockey.yml`), calls `New(path)`, and asserts a non-nil error, a nil `*Store`, `errors.Is(err, fs.ErrNotExist)` (portable, doesn't depend on OS-specific error text), and that the error's message contains the actual attempted path. Added `errors`/`io/fs` to the file's import block. Pure test-coverage addition - no production code changed; `New`'s existing bootstrap-branch behavior already handles this correctly (traced first: the bootstrap branch's `os.CreateTemp(dir, ...)` inside `writeLocked` fails with a wrapped `*PathError`/`ENOENT` when `dir` doesn't exist, surfacing as `store: bootstrap <path>: create temp file: open <path>/.fantasy-hockey-<n>.tmp: no such file or directory` - confirmed empirically before writing the test).

Not classic TDD red-green since no implementation changed - this is a characterization test proving already-correct existing behavior, not guarding a subtle invariant with two divergent state representations (unlike, e.g., item 53's persisted-vs-memory gap). Confirmed via a standalone probe that the error genuinely satisfies `errors.Is(err, fs.ErrNotExist)` and names the path before writing the final assertions, rather than guessing.

Verified: `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, full `internal/store` suite green, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied all four `patch`-routed blind-hunter findings — added a matching `openStore`-level test (`TestOpenStoreShouldReturnAnOperatorLegibleErrorForANonexistentParentDirectory` in `main_test.go`) since the retro item's own wording explicitly named "openStore/store.New," not `store.New` alone, and only the `store.New`-level test existed; added an assertion that the failed attempt leaves no stray `"nonexistent-subdir"` behind (verified empirically first: `os.CreateTemp` requires its directory argument to already exist and never creates it, so nothing today creates a stray directory - but the original test's "neither panics nor silently succeeds" claim didn't actually check for a silent partial side effect); and expanded the test's doc comment to name `os.CreateTemp` as the exact source of the `ENOENT`/`fs.ErrNotExist` (previously only documented in this spec file, not in the test itself) and to state that a single missing directory level is representative of the general case (a deeper missing chain fails identically at the same call). Two findings dispositioned `false`: the BDD/acceptance-test policy question was already resolved as a standing decision (epic-8-item-63 build) for pure test/doc hardening with no observable behavior change; and spec/sprint-status tracking still showing in-progress/open mid-review is expected workflow state.

**Re-verified after patches**: both `TestNewShouldReturnAnOperatorLegibleErrorForANonexistentParentDirectory` and the new `TestOpenStoreShouldReturnAnOperatorLegibleErrorForANonexistentParentDirectory` pass, `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — the retro item's own wording says "openStore/store.New," but only `store.New` was tested; `openStore`'s own error-wrapping path (`fmt.Errorf("open data file %s: %w", ...)`) was never exercised for this scenario, and nothing recorded why that narrowing was acceptable. Verified real. Fixed: added a matching `main_test.go` test.
- **low, patch** — the test's doc comment claimed to prove `New` "neither panics nor silently succeeds," but didn't check for a silent partial side effect (a stray directory left behind). Verified empirically that nothing today creates one (`os.CreateTemp` never creates its directory argument), but the test didn't actually assert this. Fixed: added an `os.Stat` assertion that the subdirectory was never created.
- **low, patch** — the `errors.Is(err, fs.ErrNotExist)` assertion's rationale (that it's specifically `writeLocked`'s `os.CreateTemp` producing the `ENOENT`, not `os.ReadFile` or `New` itself) lived only in this spec file's Implementation Notes, not in the test or `New`'s/`writeLocked`'s own doc comments. Verified real. Fixed: expanded the test's doc comment to name the exact source.
- **low, patch** — the test only probes a single missing directory level with no note on whether that's representative of the general "parent doesn't exist" class. Verified real: a deeper missing chain fails identically at the same `os.CreateTemp` call, since it only requires its own immediate `dir` argument to exist. Fixed: added a one-line note to the same doc comment.
- **false** — reviewer flagged that CLAUDE.md's BDD policy requires asking whether an acceptance test is needed for non-feature work, with no record of that question being asked. Not a defect: this exact question was already resolved as a standing decision in the epic-8-item-63 build - a test-only addition with zero observable behavior change squarely fits that standing policy.
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-progress/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.

All four patched findings were independently re-verified after patching: both the `internal/store` and `main_test.go` tests pass, and the full `gofmt`/`go vet`/`task go:lint`/`task go:test`/`task go:run` pipeline is clean.
