---
title: 'Story 6.1: Season Rollover With Retained History'
type: 'feature'
created: '2026-09-25'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
    - '{project-root}/_bmad-output/implementation-artifacts/epic-6-context.md'
baseline_commit: 'c7a8e8cc25a4dbfa9c8a1a7683a6fac443be2eb1'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The pool must restart cleanly for a new NHL season without losing prior seasons' data, but there is no test proving that a fresh `DATA_FILE`/`--data-file` path actually bootstraps a clean season, or that an archived file at the old path is left untouched once the app is repointed.

**Approach:** Investigation found the mechanism already exists and needs no new production code: `store.New` already bootstraps a missing path with the hardcoded current season and an empty player list (matching AD-23's "human-maintained" model — the same way Epic 1's own first run works), and there is structurally only one `*store.Store` per process, so no code path can reach a second file. This story adds the missing regression tests that pin that behavior down specifically as a season-rollover scenario, so a future change can't silently break it.

## Boundaries & Constraints

**Always:** The bootstrapped skeleton's season value and empty player list match the existing `store.New` behavior (`internal/store/store_test.go:15-47` already proves this at the store level) — this story does not change what gets written, only proves it end-to-end through `main`'s actual startup path (`resolveConfig` → `openStore`) and proves the archived-file-untouched guarantee explicitly.

**Never:** No new bootstrap mechanism, no canonical/fixed player list added anywhere in code (AD-23: player list stays human-maintained, hand-edited into the file after bootstrap — this is not a gap, it is the deliberate existing model). No in-app season-selector, cross-season query, or history view. No changes to `internal/web`'s screens — Predict/Leaderboard/Compare already only ever see the one `*store.Store` they're given.

**Decisions (human, 2026-09-25):** No Gherkin acceptance scenario for this story — it changes no user-visible screen/HTTP behavior; unit tests directly on `openStore`/`resolveConfig` (the real startup entry points) are the right level, matching how Epic 1's own bootstrap is already tested.

## I/O & Edge-Case Matrix

| Scenario                                                    | Input / State                                                                                                             | Expected Output / Behavior                                                                                       | Error Handling |
| ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | -------------- |
| Fresh path, nothing there yet                               | `openStore` called with a path that does not exist                                                                        | A file is created there with `Season() == "2026-27"` and `len(Players()) == 0`                                   | N/A            |
| Archived file at a different path                           | An old file with real season/prediction data exists at path A; `openStore` is called with a different, nonexistent path B | The file at A is byte-for-byte unchanged after B is created and opened                                           | N/A            |
| Existing file at the resolved path (unchanged 5.x behavior) | `openStore` called against a path with a well-formed file already there                                                   | The existing file is loaded as-is, not overwritten (already proven by `TestNewShouldNotOverwriteAnExistingFile`) | N/A            |

</frozen-after-approval>

## Code Map

- `src/main.go` — `resolveConfig` (~L38, flag-vs-env precedence) and `openStore` (~L65, calls `store.New` then logs `ResultProblems`) are the real startup entry points; both already unit-tested for other cases, never for "path doesn't exist yet" or "an unrelated file exists elsewhere."
- `src/main_test.go` — sibling `openStore` tests exist for malformed/well-formed/unreadable *existing* files (`TestOpenStoreShouldWarnAboutAMalformedResultAndStillSucceed` etc.); none exercise a missing path. Match their style: `path := filepath.Join(t.TempDir(), store.DataFileName)`, `slog.NewTextHandler(&bytes.Buffer{}, nil)`.
- `src/internal/store/store.go` — `New` (~L328) bootstraps `document{Season: DefaultSeason, Players: []Player{}}` on `os.ErrNotExist`; `DefaultSeason = "2026-27"` (~L34, exported during Pass 2 review; hand-maintained per AD-23, not computed — do not change its value). `Season()` (~L392)/`Players()` (~L950) are the accessors to assert against.
- `src/internal/store/store_test.go` — `TestNewShouldBootstrapCreateAMissingFile` (L15) already proves bootstrap at the `store.New` level; `TestNewShouldNotOverwriteAnExistingFile` (L49) covers the "existing file, unchanged" row. Neither needs to change.
- Nothing to do in `internal/web`/`internal/scoring`/`internal/standings` — confirmed structurally single-`Store`, single-path.

## Tasks & Acceptance

**Execution:**
- [x] `src/main_test.go` — `TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist`: assert the I/O matrix's first row (red first, since no such test exists today) — TDD.
- [x] `src/main_test.go` — `TestOpenStoreShouldNeverTouchAnArchivedFileAtADifferentPath`: assert the I/O matrix's second row — TDD.

**Acceptance Criteria:**
- Given `DATA_FILE`/`--data-file` resolves to a path with no file yet, when the app starts, then `internal/store` creates it with the hand-maintained current season and an empty player list — nothing inherited from any other file.
- Given a prior season's file still exists at a different path, when the app runs against the new path, then the prior file is never read from or written to.
- Given the current season's file, when any screen renders, then it shows only that file's data (already true; not re-tested here, since it holds for any single-file config, not something specific to rollover).

### Review Findings

- [x] [Review][Patch] `store_test.go` still hardcoded `"2026-27"` in two assertions despite being in the same package as the now-exported `DefaultSeason` [src/internal/store/store_test.go:38,183]
- [x] [Review][Patch] `epic-6-context.md`'s "the fixed player list" wording was the source of a confusion this spec had to correct downstream [_bmad-output/implementation-artifacts/epic-6-context.md]
- [x] [Review][Patch] Spec's Implementation Notes/Code Map were stale relative to the actual diff [_bmad-output/implementation-artifacts/spec-6-1-season-rollover-with-retained-history.md]
- [x] [Review][Patch] `task docker:build` (CLAUDE.md's mandated authoritative check) had never been run for this story

## Implementation Notes

`store.New`'s bootstrap-on-missing-path behavior was already correct; no new bootstrap logic was written. Added two tests to `src/main_test.go`, exercising the real startup entry point (`openStore`) rather than `store.New` directly:

- `TestOpenStoreShouldBootstrapAFreshSeasonWhenTheResolvedPathDoesNotExist` — asserts the file is created, `Season() == store.DefaultSeason`, and `len(Players()) == 0`.
- `TestOpenStoreShouldNeverTouchAnArchivedFileAtADifferentPath` — seeds a distinct "archived" file with real season/prediction data, calls `openStore` against a different, nonexistent path, asserts the returned store's `Season()`/`Players()` are correctly bootstrapped, and asserts the archived file's bytes are unchanged before vs. after.

One production change: `internal/store`'s `defaultSeason` const was exported to `DefaultSeason` (matching the `DataFileName` precedent) so both the new tests and `store_test.go`'s own pre-existing assertions reference one authoritative value instead of each hardcoding the literal season string separately (Pass 1 finding #1, and its store_test.go follow-through in Pass 2).

Verified with `task go:build` (lint, vet, unit tests, coverage, complexity, license check, govulncheck, compile — all green) and `task go:run` (binary starts and listens on :8080). `task docker:build` (Pass 2 review, per CLAUDE.md's mandated authoritative check) was attempted but failed on this sandbox for an environment reason unrelated to this change: no registry access, and a Docker credential-helper hook expecting a VS Code Dev Containers IPC socket that doesn't exist in this session (`Unable to connect to VS Code Dev Containers extension`, failing at the base-image pull in the Dockerfile's `FROM` line, before any of this story's code is even touched). Flagging rather than claiming it passed; someone with a working Docker registry connection should re-run it before the story is fully trusted as container-build-clean.

## Spec Change Log

## Review Triage Log

### Pass 1 (2026-09-25)

| #   | Layer | Finding                                                                                                                                                                                                                                                  | Verdict | Route  | Evidence                                                                                                                                                                                                                                                                                                                                                                    |
| --- | ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | blind | Both new tests hardcode the literal `"2026-27"` instead of one authoritative source                                                                                                                                                                      | medium  | patch  | `store.defaultSeason` (store.go:37) is unexported, so `main_test.go` can't reference it — confirmed by grep, the literal is now duplicated across store.go, store_test.go, and two new main_test.go tests. Direct violation of CLAUDE.md's magic-value rule; fix is exporting the const (matches the existing `store.DataFileName` precedent) and referencing it.           |
| 2   | blind | `TestOpenStoreShouldNeverTouchAnArchivedFileAtADifferentPath` discards the fresh store's return value, so it never actually proves the fresh path bootstraps correctly *in the presence of* the archived file — only that the archived file is untouched | low     | patch  | Confirmed: `if _, err := openStore(freshPath, logger); err != nil` discards the store. The doc comment claims this test proves "the second half of a season rollover," but the fresh-bootstrap-succeeds half is only proven elsewhere, under a scenario with no coexisting archived file. Trivial fix: keep the returned store and assert `Season()`/`Players()` on it too. |
| 3   | blind | Archived-file check only compares byte content, not file mode or stray temp files in its directory                                                                                                                                                       | low     | reject | The I/O matrix's "never read from or written to" guarantee is fully captured by byte-content equality; mode/directory-listing checks test a different concern (atomic-write hygiene) explicitly out of this story's scope ("no new bootstrap mechanism").                                                                                                                   |
| 4   | blind | `before` is produced via write-then-read-back instead of just using the literal seed bytes                                                                                                                                                               | false   | reject | No bad outcome named — a failed write already fails the preceding `t.Fatalf`; this is a style preference, not a defect.                                                                                                                                                                                                                                                     |
| 5   | blind | Doc comments added only to the two new tests, not their siblings in the same file                                                                                                                                                                        | false   | reject | Not a rule violation (CLAUDE.md's GoDoc mandate covers exported identifiers, not test functions) and no bad outcome named — more documentation on new tests is a positive, not a regression.                                                                                                                                                                                |
| 6   | blind | No test covers multiple coexisting archived files (only one)                                                                                                                                                                                             | false   | reject | `store.New`/`openStore` take a single literal path and never enumerate a directory — confirmed structurally in this story's own investigation. A second archived file can't exercise any code path a single one doesn't already.                                                                                                                                            |
| 7   | blind | Bootstrap test checks in-memory `Season()`/`Players()` and file existence, not the actual serialized on-disk bytes                                                                                                                                       | low     | reject | Matches the exact same rigor level as the pre-existing `TestNewShouldBootstrapCreateAMissingFile` (store_test.go:15) this story deliberately mirrors — not a new gap introduced here, parity with established precedent.                                                                                                                                                    |
| 8   | blind | I/O matrix's "Error Handling: N/A" isn't proven as specifically caused by the unrelated file's existence                                                                                                                                                 | false   | reject | No code path in `store.New`/`openStore` conditions on another file's existence at all (single literal path, no directory scan) — the concern doesn't correspond to any reachable branch.                                                                                                                                                                                    |

### Pass 2 (2026-09-25)

Ad hoc review after Build's internal review (Pass 1) closed the story. 3 of 4 layers independently found the same follow-through gap (#9).

| # | Layer | Finding | Verdict | Route | Evidence |
|---|-------|---------|---------|-------|----------|
| 9 | edge, acceptance, blind | `store_test.go` still hardcoded the literal `"2026-27"` in two assertions, even though it's in the same package as the now-exported `DefaultSeason` | low | patch | Confirmed at the two sites Pass 1's own evidence named as duplication (`TestNewShouldBootstrapCreateAMissingFile` and `TestSeasonShouldReturnTheSeededSeason`) — both now reference `DefaultSeason` (no import needed, same package). The many `season: "2026-27"` YAML seed-string literals elsewhere in the file are file-content fixtures, not equality checks, and are out of scope. |
| 10 | blind | `epic-6-context.md`'s own wording ("the fixed player list") was the source of the confusion this story's spec had to add a Design Notes section to correct | low | patch | Fixed at the source: reworded to "empty player list... sourced out-of-band, hand-edited into the file afterward (AD-23)", matching epics.md's own phrasing, so future stories reading the context doc don't hit the same misconception. |
| 11 | acceptance | Spec's Implementation Notes/Code Map were stale relative to the actual diff ("no production code changed"; named the old unexported `defaultSeason`) | low | patch | Both sections (non-frozen) updated to describe the `DefaultSeason` export and its store_test.go follow-through. |
| 12 | blind | `task docker:build` — CLAUDE.md's mandated "authoritative check" for a completed feature — was never run for this story | medium | patch | Confirmed: only `task go:build`/`task go:run` had been run. Real compliance gap (this session skipped it for prior stories too); run now and record in Verification. |
| 13 | blind | `epic-6-context.md` recommends Story 7.4 land before Epic 6, but this diff advances Epic 6 anyway | false | reject | Already an explicit, informed human decision made earlier this same session (asked directly; user chose to proceed with 6.1) — not an oversight. |
| 14 | acceptance, blind | Spec frontmatter `status: 'done'` vs. `sprint-status.yaml`'s `review` in the same diff | false | reject | By design, same as Story 5.2 Pass 2 #12: `bmad-build`'s step-05 sets the spec's own status to `done` once its internal review concludes, while syncing `sprint-status.yaml` to `review` specifically so this ad hoc review can still run first. |

## Design Notes

The apparent gap in AC1 ("the fixed player list") is not a gap: `store.New`'s bootstrap writes an *empty* player list by design, matching AD-23 (canonical team/NHL-Player lists are "written by a human directly editing `fantasy-hockey.yml` — no Go code path ever writes any of them"). "Sourced out-of-band" means a human hand-edits the freshly bootstrapped file afterward, exactly like Epic 1's own first run. `store_test.go:41-43` already asserts the bootstrapped list has length 0. No new player-seeding mechanism belongs in this story.

## Verification

**Commands:**
- `task go:build` (from repo root) — expected: lint, vet, unit and acceptance tests pass.
- `task go:run` — expected: app builds and starts.
