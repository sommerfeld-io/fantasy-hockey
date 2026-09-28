---
title: 'Split store.go by Concern'
type: 'refactor'
created: '2026-09-28'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: []
baseline_commit: '0e5c63d159a0c2d44a60edd2a532f0fd8548bf11'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `internal/store/store.go` grew 37% in Epic 7 alone and, at 1692 lines, is now the largest production file in the repo, mixing three distinct concerns — core CRUD, raw-YAML splice/preserve mechanics, and result-problem validation — that a future contributor has to read past to find what they need (epic-7 retrospective, item 57).

**Approach:** Split it into three files by concern, mirroring the `internal/web/sheet_*.go` precedent already established in this repo: `store.go` (core CRUD + shared domain types, keeps the package doc comment), `store_splice.go` (raw-YAML preservation mechanics added by Story 7.4), `store_results.go` (result-problem validation/reporting + the result read accessors). Pure reorganization — zero behavior change, zero exported-API change.

## Boundaries & Constraints

**Always:**
- Zero behavior change: same package `store`, every exported identifier keeps its exact name, signature, and doc-comment text (comments move verbatim with their declarations; reword only if factually stale — none found to be).
- Verify with `task go:build`/`task go:test` (full pipeline) and `task go:run` after the split.
- `writeLocked` stays whole, in `store.go` — every mutating CRUD method calls it, that's its primary conceptual home, even though it also calls `store_splice.go`'s `spliceNamedValueLocked`/`topLevelMapping` (cross-file calls within the same package are ordinary Go, not a design smell).
- `topLevelMapping`/`mappingValue`/`mappingKeys` stay in `store_splice.go` even though core (`loadExisting`, `writeLocked`) and `store_results.go` (`unknownKeyProblemsLocked` and its helpers) both call them — the single most cross-cutting dependency in this split; do not try to eliminate the cross-file call.

**Never:**
- Do not rename any exported or unexported identifier — this is a pure move, not a rename pass, even where a name seems improvable.
- Do not touch `store_test.go`/`yamllint_test.go`'s test logic or assertions — only file location.
- Do not touch any file outside `internal/store/` — no other package changes, since the exported API is unchanged.
- Do not further decompose the `ResultProblems` cluster (~15 mutually-referencing private helpers) or split `writeLocked`'s body internally — move each as one whole unit.
- No Gherkin acceptance test — pure internal reorganization with no observable app behavior change (nothing in an HTTP response, rendered page, or data-file content changes).

**Decided (2026-09-28, human-confirmed):**
- Token count (~2,300, above the 1,600 soft-limit): keep the full spec — the overage is the Code Map's mechanical line-range precision, load-bearing for a refactor like this, not scope creep; no natural secondary goal exists to split off.
- `store_test.go` (3231 lines) splits into three files too, mirroring the source split — matches this repo's own established `internal/web/sheet_*.go` precedent (every other concern-split source file here has a matching 1:1 split test file). `yamllint_test.go` is untouched either way (a whole-file integration check unrelated to any one concern).

</frozen-after-approval>

## Code Map

- `src/internal/store/store.go` (1692 lines today) — split source. Lines 1–349: package doc comment + shared domain types/consts (`Player`, `LoginCode`, `PredictionSet`, `Kind*`/`Award*`/`Round*SetID` consts, `JoinSeriesKey`/`SplitSeriesKey`, `Divisions()`, `Prediction`, `Team`, `Position*` consts, `AwardFinalist`, `PlayoffMatchup`, `document`, `Store`) stay in `store.go`, except `results`/`divisionMarks`/`seriesOutcome` structs (281–307), which move to `store_results.go` (legal: `document.Results results` references a type in a different file of the same package).
- **`store.go` (core CRUD, target ~1080–1110 lines)** keeps: `New`/`loadExisting` (351–445), `FindPlayerByEmail`/`FindPlayerByID`/`Season`/`PredictionSets`/`Teams`/`NHLPlayers`/`NHLPlayersByPosition`/`PlayoffMatchups` (592–711), `CreateLoginCode`/`ConsumeLoginCode` (713–783), `FindPrediction`/`SavePrediction`/`FindSeriesPick`/`SaveSeriesPick`/`FindDivision*`/`SaveDivisionPicks`/`upsertDivisionPredictionLocked` (785–1006), `AwardFinalistCount`/`FindAwardFinalists`/`awardHasAllFinalistSlugs`/`SaveAwardPicks`/`upsertAwardPredictionLocked` (1008–1105), `cleanupLoginCodes` (1107–1131), `writeLocked` (1156–1249), `Players`/`PredictionsForPlayer` (1311–1337). Imports: `bytes, errors, fmt, log/slog, os, path/filepath, slices, strings, sync, time, uuid, yaml`.
- **`store_splice.go` (new, target ~180–200 lines)**: `lenientTopLevelKeys`/`typeErrorLinePattern` vars, `parseErrorLine`, `duplicateKeyErrorSuffix`, `isDuplicateKeyError`, `namedLenientRange` type, `lenientRanges`, `typeErrorConfinedToLenientSections`, `sectionForLine`, `maxLine`, `topLevelMapping`, `mappingValue`, `mappingKeys` (447–590 plus `spliceNamedValueLocked`, 1133–1154). Imports: `regexp, strconv, strings, yaml`.
- **`store_results.go` (new, target ~440–460 lines)**: `results`/`divisionMarks`/`seriesOutcome` structs (moved from 281–307), `resultRounds`/`awards` vars, `minSeriesGames`/`maxSeriesGames` consts, `resultDivisionKey`/`resultRoundForSetID`/`validSeriesGames`/`parseSeriesGames`/`sortedKeys[V any]` (1251–1309), `DivisionResult`/`PresidentsTrophyWinner`/`StanleyCupWinner`/`SeriesResult`/`RecordedAwardFinalists`/`ResultProblems` and its full helper cluster (`toleratedShapeErrorProblemsLocked` through `knownSlugLocked`, 1339–1692). Imports: `fmt, slices, sort, strings, yaml`.
- `src/internal/web/sheet.go`/`sheet_awards.go`/`sheet_divisions.go`/`sheet_series.go` — the naming/no-per-file-doc-comment precedent this split follows (only `web.go`, matching the package name, carries the package doc comment — same rule applied here to `store.go`).
- `src/.golangci.yml` — confirmed no `gocyclo`/`funlen`/file-length rule exists; nothing in lint config reacts to file boundaries.
- `src/internal/store/store_test.go` (3231 lines) — split into three files by the same rule as the source: classify each test by which production function/behavior it most directly exercises; when a test's assertions span two concerns, group it with whichever concern its *outcome* surfaces (not the underlying mechanism it happens to touch).
  - **Lines ~58–2276** (core CRUD tests: `New`/`FindPlayer*`/`Season`/`PredictionSets`/`Teams`/`NHLPlayers`/`PlayoffMatchups`/`CreateLoginCode`/`ConsumeLoginCode`/`CleanupLoginCode`/`SavePrediction`/`FindPrediction`/`FindDivision*`/`SaveDivisionPicks`/`FindAwardFinalists`/`SaveAwardPicks`/`FindSeriesPick`/`SaveSeriesPick`/concurrency/`JoinSeriesKey`-`SplitSeriesKey`/`Divisions`) → `store_test.go`.
  - **Lines 2368–3231** ("mixed tail", not cleanly ordered — needs re-sorting, not a straight cut): `TestPlayersShouldReturnEveryPlayer` (2472) and `TestPredictionsForPlayerShouldReturnOnlyThatPlayersRows` (2481) are stray core tests → `store_test.go`. Every `TestDivisionResult*`/`TestTrophyWinnersShould*`/`TestSeriesResultShould*`/`TestRecordedAwardFinalists*`/`TestResultReadMethodsShould*`/`TestResultProblemsShould*`/`TestNewShouldLoadAFileWithNeitherResultsNorAwardFinalists`/`TestNewShouldTolerate*`/`TestNewShouldStillFailForAShapeError*` test — their outcome surface is `ResultProblems()`/the result read accessors, even though a few (`TestNewShouldTolerate*`) also exercise `store_splice.go`'s tolerance detection under the hood → `store_results_test.go`. `TestSavingAPredictionShouldLeaveAToleratedShapeErrorByteForByteUnchanged` (3004), `TestSavingAPredictionShouldPreserveCommentsFlowStyleAndUnusualKeyOrderInHandMaintainedSections` (3030), `TestWriteLockedShouldRollBackA*RawNodeWhenTheWriteFails` (3080, 3118), `TestNewShouldRoundTripACleanFileEndToEnd` (3159), `TestNewShouldAllowWritesAfterLoadingAnEmptyExistingFile` (3184), `TestNewShouldNotSilentlyDiscardWritesAfterLoadingABareNullFile` (3209) — their outcome surface is the splice/preserve mechanism itself (byte-for-byte preservation, splice rollback, raw-node shape handling) → `store_splice_test.go`. `TestSavingAPredictionShouldPreserveTheHandRecordedResults` (2649) and `TestSavingAPredictionShouldNotAddAResultsSectionToAFileWithoutOne` (2754) are about the splice not touching untouched sections → `store_splice_test.go`.
  - Shared test helpers (`newTestStore`, `newSeededStore`, `seedLoginCode`, `seedPrediction`, `captureLogs`, `assertExactlyOneInfoLine`, `resultsFixtureBase`, `resultsFixtureResults`, etc.) compile fine referenced across files in the same package — leave them wherever they currently live (or move to whichever new file uses them most; do not duplicate).
- `src/internal/store/yamllint_test.go` (172 lines) — untouched, a whole-file integration check unrelated to any one concern.

## Tasks & Acceptance

**Execution:**
- [x] `src/internal/store/store_splice.go` -- create, move the concern-2 code listed in Code Map verbatim, own trimmed import block -- isolates the raw-YAML splice/preserve mechanics.
- [x] `src/internal/store/store_results.go` -- create, move the concern-3 code listed in Code Map verbatim (including the `results`/`divisionMarks`/`seriesOutcome` structs), own trimmed import block -- isolates result-problem validation/reporting.
- [x] `src/internal/store/store.go` -- remove everything moved to the two new files, trim its import block to what remains -- leaves only core CRUD + shared types + the package doc comment.
- [x] `src/internal/store/store_splice_test.go` -- create, move the concern-2 tests listed in Code Map verbatim -- mirrors `store_splice.go`.
- [x] `src/internal/store/store_results_test.go` -- create, move the concern-3 tests listed in Code Map verbatim -- mirrors `store_results.go`.
- [x] `src/internal/store/store_test.go` -- remove everything moved to the two new test files, keep the stray core tests (`TestPlayersShouldReturnEveryPlayer`, `TestPredictionsForPlayerShouldReturnOnlyThatPlayersRows`) and every shared test helper still referenced from more than one file -- leaves only core-CRUD tests.

**Acceptance Criteria:**
- Given the split is complete, when `go build ./...` and `go vet ./...` run from `src/`, then both compile/vet clean with no changes required in any file outside `internal/store/`.
- Given the split is complete, when `task go:test` runs, then the full suite passes green with the same total test count (moved, not added or removed) and no coverage regression in `internal/store`.
- Given the six files (three source, three test, `yamllint_test.go` unchanged), when inspected, then each contains only the functions/types/consts/vars/tests the Code Map assigns to it, and `store.go` alone carries the package-level doc comment.

## Implementation Notes

`store.go` (1692 → 1078 lines), `store_splice.go` (new, 177 lines), `store_results.go` (new, 454 lines) implemented per the Code Map exactly. `store_test.go` (3231 → 2305 lines), `store_splice_test.go` (new, 312 lines), `store_results_test.go` (new, 633 lines) implemented per the test-split rule. `yamllint_test.go` untouched.

One deviation from the letter of the Code Map: `store_results.go` also imports `strconv` (not listed in the Code Map's import line) — `parseSeriesGames`/`resultDivisionKey`'s neighbors use `strconv.Atoi`/`Itoa`, so the file wouldn't compile without it. Documentation gap in the Code Map, not a scope change; no identifiers renamed, no logic changed.

Verified independently (not just trusting the implementation subagent's report): read the full unified diff against `baseline_commit` end to end (3199 lines) — confirmed every moved block is byte-for-byte identical to its origin, only package/import boilerplate and the `strconv` addition differ. Ran `gofmt -l`, `go build ./...`, `go vet ./internal/store/...`, `go test ./internal/store/... -v` (156 tests, all pass, 95.8% package coverage — same as the implementation subagent's reported pre/post-split baseline), `task go:test` (full pipeline: lint 0 issues, gocyclo clean, licenses clean, govulncheck no findings), and `task go:run` (builds and starts, `fantasy-hockey listening port=8080`). `git diff --stat` against baseline confirms exactly 6 files changed, all within `internal/store/`.

Nothing incomplete or risky.

## Spec Change Log

## Review Triage Log

*(blind-hunter, edge-case-hunter, verification-gap on the full diff since `baseline_commit`)*

- **low / patch** — `internal/store/README.md` has no "File layout" table naming what `store.go`/`store_splice.go`/`store_results.go` each own, no note on where shared test fixtures live, and no record of the "group by outcome surface" test-classification rule this split used — all three only ever existed in this ephemeral spec. Verified real: `internal/web/README.md` (this split's own cited precedent) carries exactly this table plus an explicit "General test helpers... live in web_test.go" sentence for its own `sheet_*.go` split; `internal/store/README.md` has neither. Real navigability gap for the next contributor, direct fix (add the same two elements, no new public surface). (blind-hunter findings 1, 2, 10 — grouped, same missing-README-update root cause.)
- **low / patch** — `store_results_test.go`'s `newSeededStore`/`resultsFixtureBase`/`resultsFixtureResults` doc comments describe them as results-scoped ("every results test builds on"), not as the shared, package-wide fixtures they actually are (used 16x in `store_test.go`, 3x in `store_splice_test.go`). Verified against the actual doc comments in the diff. Cheap fix: reword the three comments. (blind-hunter finding 3.)
- **low / patch** — `store_splice_test.go` interleaves pure splice/rollback tests with results-section-preservation tests with no banner comment separating them, unlike `store_results_test.go`'s own `// --- spec-7-4: ... ---` banner convention. Verified against the file's actual structure. Cheap fix: add an equivalent banner. (blind-hunter finding 4.)
- **low / patch** — `resultsFixtureMatchups` is declared ~280 lines after its sibling fixtures `resultsFixtureBase`/`resultsFixtureResults`, right before the one test that uses it, rather than grouped with them. Verified against line positions in the diff. Cheap fix: move the declaration up. (blind-hunter finding 5.)
- **low / patch** — `store_splice.go`'s `lenientTopLevelKeys = []string{"results", "award_finalists"}` hardcodes the two section names that are `store_results.go`'s domain, with no comment cross-referencing that coupling — pre-existing coupling, but the split increased the distance between the two related declarations (previously visible side-by-side in one file). Verified against both files' current content. Cheap fix: one cross-reference comment. (blind-hunter finding 6.)
- **low / informational, no code patch** — the spec doc itself (`spec-epic-7-retro-item-57-split-store-go-by-concern.md`) isn't part of this diff and would need including when this work is committed, per this repo's own established convention. Verified against git history: commit `f9c5c71` committed `spec-7-4-hand-edited-results-are-safe-to-edit.md` alongside its implementing code in one commit. Correctly excluded from diff review by the workflow's own design (the spec is `claims_file`, reviewed separately by edge-case-hunter, deliberately not part of the code diff) — nothing to patch now; noted for the eventual commit. (blind-hunter finding 7.)
- **low, rejected** — a partial-file split (new files created via extraction, not `git mv`) means `git blame`/`git log --follow` won't automatically trace the moved code's history back through `store.go`/`store_test.go`. Real and verified (inherent to any split-by-extraction, git's rename/copy heuristics need much higher content-similarity than any of these three new files has with the original). Rejected: the fix (elaborate git-history engineering, or not doing the split as designed) is disproportionate to a cost that isn't everyday-use pain — `git log -- store.go` still shows the pre-split history for anyone who checks the original file first. (blind-hunter finding 8.)
- **low, defer** — `yaml "go.yaml.in/yaml/v3"` aliases a package whose own declared name is already `yaml`, so the alias is redundant — pre-existing in the original `store.go` before this diff, now repeated once per new file (3x instead of 1x) as an unavoidable consequence of each file needing its own import block. Verified: the alias already existed pre-split. Not caused by this story. Appended to `deferred-work.md`. (blind-hunter finding 9.)


## Design Notes

The cross-file dependency graph is not a strict DAG by design: `store.go`'s `writeLocked`/`loadExisting` call into `store_splice.go`, and `store_results.go` also calls `store_splice.go`'s `topLevelMapping`/`mappingValue`/`mappingKeys`. This is expected and fine — Go resolves same-package symbols regardless of file, so this is purely an organizational split for human readers, not a dependency-inversion refactor. Do not "fix" this by duplicating or re-homing the splice helpers to avoid the cross-file calls.

## Verification

**Commands:**
- `cd src && go build ./...` -- expected: compiles clean, zero errors, zero changes needed outside `internal/store/`.
- `cd src && go vet ./internal/store/...` -- expected: clean.
- `task go:test` -- expected: full suite green (lint, vet, tests, gocyclo, licenses, govulncheck), no test-count change, no coverage regression.
- `task go:run` -- expected: app still builds and starts.
