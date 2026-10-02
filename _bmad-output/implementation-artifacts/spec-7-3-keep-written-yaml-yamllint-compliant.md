---
title: 'Keep Written YAML yamllint-Compliant'
type: 'feature'
created: '2026-09-27'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: []
baseline_commit: 'f4b64374bf34c327c5f3e70dcc2d8e5d39621e5c'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `writeLocked`'s plain `yaml.Marshal` produces a real yamllint **error** (not just the known-acceptable `document-start` warning) whenever a `Prediction` row carries a nested list field (`team_ids` on a division-playoff-teams row, `finalist_slugs` on an award row) — confirmed empirically: `go.yaml.in/yaml/v3`'s default marshal indents a sequence nested inside a mapping that's itself inside a list by only +2 spaces, while every top-level sequence in the same document gets +4, and yamllint's `indentation` rule (from `extends: default`) requires the increment to be consistent throughout the file.

**Approach:** Switch `writeLocked` from `yaml.Marshal` to `yaml.NewEncoder(...).CompactSeqIndent()` before encoding — confirmed empirically (via the real `cytopia/yamllint:latest` image and this repo's actual `.yamllint.yml`) to make every sequence in the document use the same relative indent, producing zero error-level violations (only the pre-existing, accepted `document-start` warning).

## Boundaries & Constraints

**Always:**
- The fix is confined to how `writeLocked` serializes `docToWrite` — swap the `yaml.Marshal(docToWrite)` call for `yaml.NewEncoder` + `CompactSeqIndent()` + `Encode`, keeping the exact same downstream temp-file-write-and-rename sequence (AD-27) unchanged.
- The automated test (AC3) writes a file through the real `store` package, then shells out to the actual `cytopia/yamllint:latest` image (the same image `docker-compose.yml`'s `lint-yaml` service uses) against this repo's actual `.yamllint.yml`, asserting the command's exit code is 0 (yamllint's own convention without `--strict`: warnings alone still exit 0; any error exits non-zero) — never a hand-rolled reimplementation of yamllint's rules.
- That test skips (not fails) when the `docker` binary isn't on `PATH`, so local `go test` still works for a contributor without Docker; CI (which has Docker) always exercises it for real.
- No `yamllint` invocation of any kind ships in production code or the running app — only the test file shells out.

**Never:**
- Don't reformat or resave the actual committed `src/fantasy-hockey.yml` as part of this story, and don't remove its entry from `.yamllint.yml`'s `ignore:` list yet. **Decided 2026-09-27 (human-confirmed):** deferred until Story 7.4 ships and the file can be safely resaved without losing hand-maintained formatting (comments, key order) that isn't protected yet.
- Don't add a `--strict` check anywhere, or make any lint-level stricter than what `task lint`'s `lint-yaml` service already enforces.
- Don't touch hand-maintained-section formatting preservation (comments, quoting, key order) — that's Story 7.4's job; this story only governs the bytes the app's own encoder produces for the fields it writes.
- **Decided 2026-09-27 (human-confirmed):** no Gherkin acceptance test for this story — nothing HTTP/UI-observable changes, and AC3 already fully specifies the required automated test.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| A write includes a nested-list Prediction row | `SaveDivisionPicks`/`SaveAwardPicks` writes a row with `team_ids`/`finalist_slugs` | resulting YAML has zero yamllint error-level violations (only the accepted `document-start` warning) | N/A |
| A write has no nested-list rows | e.g. `CreateLoginCode`, `SavePrediction` (cup/presidents) | still zero yamllint errors (unchanged from before, already clean) | N/A |
| Encoder fails to encode | (`Encode`/`Close` return an error — can't happen for this document shape, but the return path exists) | `writeLocked` returns a wrapped error, same as today's `marshal` error path | existing error wrapping unchanged |
| `docker` not on `PATH` | the new automated test's environment | test skips, not fails | N/A |

</frozen-after-approval>

## Code Map

- `internal/store/store.go:902-911` -- `writeLocked(reason string, now time.Time) error`: replace `out, err := yaml.Marshal(docToWrite)` with an `Encoder`: create a `bytes.Buffer`, `enc := yaml.NewEncoder(&buf)`, `enc.CompactSeqIndent()`, `enc.Encode(docToWrite)` (check error), `enc.Close()` (check error — required to flush; also returns an error), then use `buf.Bytes()` in place of `out` for the unchanged temp-file-write-and-rename code below it.
- `internal/store/` (new file, e.g. `yamllint_test.go`) -- new test per AC3. Needs: (a) the repo root, found via `runtime.Caller(0)` on the test file itself + `filepath.Join(filepath.Dir(thisFile), "..", "..", "..")` (three levels up from `src/internal/store/`), so `.yamllint.yml` resolves regardless of `go test`'s working directory; (b) skip via `exec.LookPath("docker")` if absent; (c) write a file via `store.New` + a mutating call producing a nested-list row (`SaveDivisionPicks` or `SaveAwardPicks` — a plain `CreateLoginCode`/`SavePrediction`-only file would not reproduce the bug this story fixes); (d) invoke `docker run` directly, per Design Notes (not `docker compose run`); (e) assert the command exits 0 (yamllint's own convention: warnings alone still exit 0, only an error exits non-zero — matching `task lint`'s non-strict gate exactly).
- `.yamllint.yml` -- do **not** edit yet, per Boundaries & Constraints' human-confirmed decision.

## Tasks & Acceptance

**Execution:**
- [x] `internal/store/store.go` -- switch `writeLocked`'s marshal call to `yaml.NewEncoder` + `CompactSeqIndent()`, per the Code Map; keep the existing temp-file/rename logic operating on the resulting bytes unchanged.
- [x] `internal/store/yamllint_test.go` (new) -- add the real-yamllint test per the Code Map, covering a nested-list row (`team_ids` or `finalist_slugs`) so it actually exercises the fixed code path, skipping when `docker` isn't available.
- [x] `internal/store/store_test.go` -- no changes expected (existing tests don't assert on exact YAML byte layout); confirm this by running the full suite.

**Acceptance Criteria:**
- Given any Store write whose document includes a nested-list Prediction row, when the resulting YAML is checked with the repo's real `yamllint`/`.yamllint.yml` (no `--strict`), then it produces no error-level violations.
- Given the app running normally, when it operates, then it never invokes the `yamllint` binary itself.
- Given a `go test`/CI run with Docker available, when it runs, then an automated test writes a file through the real `store` code path and verifies it against the actual `yamllint` binary/config.

### Review Findings

*(bmad-code-review, 2026-09-27, diff `f4b6437..bdd7b90`)*

- [x] **[Review][Decision]** `results.team_marks.<division>.playoffs` — a sequence nested inside plain map-of-map-of-map (not the list-of-maps shape this story fixed) — still fails yamllint on the *very next write* once any real season populates `results:` by hand, per the current runbook's own documented 4-space convention. **Verified empirically, live in the current codebase (after Story 7.4)**: seeded a store with a hand-typed `results.team_marks.atlantic.playoffs: [FLA, TOR]` at 4-space indent, ran a real `SavePrediction`, and linted the output with the real `yamllint`/`.yamllint.yml` — 3 error-level indentation violations (`expected 2 but found 4`, etc.), because `CompactSeqIndent()` makes every *sequence* indent 2 relative to its parent, but ordinary *map* nesting (unaffected by `CompactSeqIndent`) still renders at 4 — an internal inconsistency yamllint's `indentation: consistent` rule catches. I tried the two most obvious code-level fixes myself and both fail: `SetIndent(4)` (today's setting) leaves the mismatch; `SetIndent(2)` (tested) makes map-nesting match sequences, but then top-level sequences render at *0* indent, which yamllint's default ruleset separately rejects (`expected at least 1`). Root tension: `CompactSeqIndent`'s whole design is to indent a sequence *less* than its parent map — by construction, no single `SetIndent(n)` value can make map-nesting and sequence-nesting share one increment while `CompactSeqIndent` is in effect. Two independent review layers (edge-case-hunter, verification-gap) surfaced this same gap independently, and verification-gap reproduced the actual yamllint failure directly.
  **Options for the human to choose between:**
  1. **Relax `.yamllint.yml`'s indentation rule** (e.g. `indent-sequences: false` or a looser mode) so map- and sequence-indentation no longer need to match — a repo-wide lint-policy change affecting every YAML file, not just this one.
  2. **Update the runbook's documented indentation convention** from 4-space to 2-space for hand-typed `results:`/`award_finalists:` entries, matching the app's own generated style — cheap now, since no real season has hand-typed results yet, but doesn't help if `SetIndent`/`CompactSeqIndent` ever changes again later.
  3. **Accept as a known limitation for now**, documented in the runbook/Design Notes, and revisit if it causes real friction once a season actually records results.
  4. **Further R&D** into other `go.yaml.in/yaml/v3` encoder options not yet tried, to find a setting that satisfies both shapes at once.
  **Resolved (human-confirmed 2026-09-27): option 3, accept as a known limitation for now.** No season has hand-typed results yet, so nothing breaks today; documented in `docs/recording-results-and-playoffs.md`'s "Results and award finalists" section and deferred to `deferred-work.md`.
- [x] [Review][Patch] Both new tests require Docker; a contributor without it gets zero direct coverage of this story's actual behavior change (the indentation fix itself) — only the Docker-gated tests assert on it (blind-hunter) [internal/store/yamllint_test.go] — fixed: added `TestWriteLockedShouldIndentANestedListConsistentlyWithTopLevelSequences`, a Docker-free byte-level assertion.

**Rejected:**
- `defer` — "Both the new test and `docker-compose.yml`'s `lint-yaml` service pin the floating `cytopia/yamllint:latest` tag" (blind-hunter). Real, but pre-existing — `docker-compose.yml` already used `:latest` before this story; this story only added a second consumer of the same tag. Appended to `deferred-work.md`.
- `false` — "spec status `done` vs. sprint-status `review` is a contradiction" (blind-hunter). Same as every other story reviewed this session: the workflow's own intended distinction between "spec finished" and "awaiting independent code review," not a defect.
- `false` — "Nothing detects if CI itself lacks Docker, so the tests would silently skip there too" (blind-hunter). GH Actions' `ubuntu-latest` runners (this repo's actual CI, confirmed in `.github/workflows/pipeline.yml`) guarantee a working Docker daemon; no concrete mechanism for it to silently disappear was named.
- `false` — "`dockerOrSkip` runs `docker info` independently in each of the two tests, duplicating an expensive check" (blind-hunter). Two calls with a bounded 60s timeout each is a negligible cost, not a demonstrated problem.
- `false` — "`runYamllint`'s failure message conflates a real lint violation with a docker-execution failure" (blind-hunter). The common case (daemon unreachable) is already filtered out by `dockerOrSkip`'s own pre-check; the residual case (a transient failure mid-test, after setup succeeded) is rare and the fix disproportionate.
- `false` — "Both new tests are near-identical copy/paste; a table-driven refactor would help" (blind-hunter). A stylistic preference with a speculative future cost (a hypothetical third test case), not a current defect.
- `false` — "The Design Notes' `SetIndent(4)` claim isn't test-locked and could go stale" (blind-hunter). The actual code never calls `SetIndent`; if a future `go-yaml` upgrade changed default behavior, the existing yamllint-compliance tests would catch any real regression regardless.
- `false` — "Spec markdown table padding / MD036 bold-headings" (blind-hunter). Same as every other spec in this epic: inherited from the shared `spec-template.md`, not introduced by this diff.
- `false` — "The comment above `Encode`/`Close` refers to 'spec-7-3's I/O matrix' with no file path" (blind-hunter). Matches this codebase's own established convention throughout `store.go` (every other story's Design Notes reference the same way, e.g. "spec-7-1's Design Notes," "spec-7-2's Design Notes," never with a path) — adding one would be inconsistent, not an improvement, and this repo's own style guide discourages referencing task/fix history in code comments.
- `false` — "`dockerOrSkip` never checks the yamllint image can actually be pulled, only that the daemon is reachable" (edge-case-hunter). GH Actions runners have full internet access (confirmed this session for Story 7.3's own original build); unlikely in the real CI environment, and the fix (another docker call) is disproportionate to a scenario with no demonstrated occurrence.

## Implementation Notes

`writeLocked` (`internal/store/store.go`) now builds a `bytes.Buffer`, creates `yaml.NewEncoder(&buf)`, calls `enc.CompactSeqIndent()`, then `enc.Encode(docToWrite)` and `enc.Close()`, each checked and wrapped as `fmt.Errorf("marshal: %w", err)` on failure - the same error message and wrapping the old `yaml.Marshal` path used, so callers' existing error-handling is unaffected. `out := buf.Bytes()` feeds the unchanged temp-file-write-and-rename sequence below it. Added the `bytes` import.

TDD sequence followed: wrote `internal/store/yamllint_test.go` first and confirmed it failed (red) against the pre-fix `yaml.Marshal` code, reproducing the exact `26:9`-style `wrong indentation` error the spec's Design Notes describe (observed as `16:9 error wrong indentation: expected 10 but found 8` for this test's smaller document); then applied the `CompactSeqIndent()` fix and confirmed the same test goes green, with zero yamllint errors (only the accepted `document-start` warning).

`internal/store/yamllint_test.go` (new): `TestWriteLockedShouldProduceYamllintCompliantOutputForANestedListRow` skips via `exec.LookPath("docker")` when Docker isn't available; otherwise it resolves the repo root from `runtime.Caller(0)` (three `..` up from `src/internal/store/`), opens a `store.New` against a `t.TempDir()` path, calls `SaveDivisionPicks` with a `team_ids` row to reproduce the nested-list shape, then shells out directly to `docker run --rm -v <repo>/.yamllint.yml:/yamllint.yml:ro -v <tempdir>:/data:ro cytopia/yamllint:latest -c /yamllint.yml /data` (not `docker compose run`, per the Design Notes' plumbing gotcha about arbitrary temp paths not being mountable through the compose service), and asserts exit code 0.

`internal/store/store_test.go` was left unchanged, as the spec predicted - none of its existing tests assert on exact YAML byte layout, and the full suite stays green.

Verified: `go test ./internal/store/... -run Yamllint -v` (passes, docker was available in this environment so the real check ran, not just a skip); `task go:test` (full suite green across all packages, `internal/store` coverage 97.6%); `task go:run` (runs the full `go:build` pipeline first - `go vet`, `golangci-lint run` 0 issues, full test suite, `gocyclo -over 10` clean, `go-licenses` clean, `govulncheck` no vulnerabilities found - then builds and starts the binary, which logs `listening port=8080 addr=:8080` and serves until manually stopped).

No changes were made to `.yamllint.yml` or `src/fantasy-hockey.yml`, per the frozen Boundaries & Constraints' human-confirmed deferral to Story 7.4.

**Review patches (step-04):** applied all five surviving `patch`-routed findings from the Review Triage Log — extended the nested-list test to also cover `finalist_slugs`, added a `docker info` reachability check to `dockerOrSkip` (skip cleanly rather than fail with a misleading message), added a shared 60s timeout for both `docker run`/`docker info` calls, added a one-line comment on `writeLocked`'s unreachable `Encode`/`Close` error paths, and added a `README.md` bullet for the `CompactSeqIndent()` decision. `gofmt -l`, `go vet`, `golangci-lint run` (0 issues), `gocyclo -over 10 .` (clean), both yamllint tests, and the full `go test ./...` all re-run green after the patches.

**Matrix Test Audit (step-03):** the "A write has no nested-list rows" row wasn't covered by any test — `CompactSeqIndent()` changes the indent of *every* sequence (not only nested ones), so this row needed its own verification that the common no-nested-list case is still yamllint-clean, not just assumed safe by reasoning. Closed by refactoring the docker-invocation into a shared `runYamllint`/`dockerOrSkip` helper pair and adding `TestWriteLockedShouldProduceYamllintCompliantOutputForARowWithNoNestedLists` (seeds via `CreateLoginCode`+`SavePrediction`, asserts the same exit-0 outcome). Both yamllint tests, `gofmt -l`, `go vet`, `golangci-lint run` (0 issues), `gocyclo -over 10 .` (clean), and the full `go test ./...` re-run green after the addition.

## Spec Change Log

## Review Triage Log

- **low / patch** — `SaveAwardPicks`/`finalist_slugs` (one of the two documented bug-reproducing shapes) was never exercised by the new yamllint tests — only `SaveDivisionPicks`/`team_ids` was (blind-hunter). Fix: extended the nested-list test to also call `SaveAwardPicks` in the same document before linting.
- **low / patch** — `runYamllint` treats any non-zero `docker run` exit as a lint violation, but a non-zero exit can equally mean Docker itself failed (daemon unreachable, image not cached/no network) — `dockerOrSkip` only checked `exec.LookPath`, not that the daemon actually responds (edge-case-hunter + blind-hunter, same root cause, merged). Fix: `dockerOrSkip` now also runs `docker info` and skips (not fails) if that doesn't succeed; the failure message was also reworded to acknowledge a Docker-infra failure as a possibility.
- **low / patch** — no explicit timeout bounded the `docker run`/`docker info` calls; a hung pull or daemon would only fail via `go test`'s own overall binary timeout, producing a much less legible failure (edge-case-hunter). Fix: added a shared 60s `context.WithTimeout` used by both.
- **low / patch** — the `Encode`/`Close` error-return paths in `writeLocked` are documented in the spec as deliberately untested (unreachable for this document shape), but nothing in `store.go` itself said so — a future reader of the code alone wouldn't know this was considered, not missed (blind-hunter). Fix: added a one-line comment above the two checks.
- **low / patch** — `internal/store/README.md`'s "Design notes" section (which already documents the analogous AD-27 write-and-rename decision) didn't mention the new `CompactSeqIndent()` encoding decision or that part of the test suite now has a Docker dependency (blind-hunter). Fix: added one bullet.
- **false** — "`runYamllint` only asserts exit code 0, but the stated acceptance criterion says only the accepted `document-start` warning is allowed — a new warning-level regression would pass silently" (blind-hunter). Verified against the *epics.md* source AC (the authoritative wording, not this spec's own paraphrase): "a pre-existing, non-blocking warning (**e.g.** the default ruleset's `document-start` warning...) is acceptable and out of scope for this story" — "e.g." signals an example, not an exhaustive list; warnings in general are out of scope, not just that one. Exit-code-0 is exactly what the actual AC requires. This spec's own Intent/Design Notes prose was narrower ("only... the warning") than the source epic — a wording imprecision, not a code or test defect; its only fix would be editing this spec's prose.
- **false** — "`writeLocked` wraps both the `Encode` error and the `Close` error with the identical message `marshal: %w`, collapsing two distinct failure points" (blind-hunter). Both paths are already established (by the verification-gap layer's own pre-verified pass over this same diff) as unreachable for this document's field types; identical wrapper text for two failure modes that can never actually occur in practice causes no real harm.
- **false** — "The story title implies the real committed `src/fantasy-hockey.yml` becomes yamllint-compliant, but it's explicitly left untouched/deferred, and this isn't prominently surfaced" (blind-hunter). The frozen Boundaries & Constraints section already states this deferral explicitly and its rationale (human-confirmed); any further prominence is a spec-wording preference, not a defect, and its only fix is a spec edit.
- **false** — "I/O matrix table columns aren't padded to equal width" and "bold pseudo-headings (`**Always:**` etc.) violate MD036" (blind-hunter). Both true of the shared `spec-template.md` this and every other spec in this run are built from (the reviewer's own words: "likely a template-wide issue rather than one specific to this file") — pre-existing, not introduced by this story, and not something a code patch to this diff can fix.

## Design Notes

**Root cause, empirically confirmed:** built a document with the exact `Prediction` shape (`team_ids`/`finalist_slugs`) and ran it through both `yaml.Marshal` and a `CompactSeqIndent()` encoder against the real `cytopia/yamllint:latest` image + this repo's `.yamllint.yml`:
- Default marshal: `26:9 error wrong indentation: expected 10 but found 8` (and again for the second nested list) — a top-level sequence gets +4 indent from its key, but a sequence nested inside a list-item's own map only gets +2, and yamllint's `indentation` rule demands one consistent increment throughout the file.
- `CompactSeqIndent()`: zero errors, only the accepted `document-start` warning — it applies the *same* relative increment (+2) to every sequence regardless of nesting depth, satisfying "consistent."
- `SetIndent(4)` made no difference either way (confirmed byte-identical output with and without it) — only `CompactSeqIndent()` matters; no need to also call `SetIndent`.

**Test plumbing gotcha, empirically confirmed:** `docker compose run --rm --no-deps lint-yaml <path>` only works for a file physically located under the repo root (the compose service's `volumes:` bind-mounts `.` → `/workspaces/fantasy-hockey`, nothing else) — passing an extra `-v` flag to `docker compose run` does not add a working mount for an arbitrary `t.TempDir()` path. A direct `docker run` with its own explicit `-v` flags for `.yamllint.yml` and the temp directory works with any path and is simpler for a test to construct — use that, not `docker compose run`.

## Verification

**Commands:**
- `cd src && go test ./internal/store/... -run Yamllint -v` -- expected: passes (or skips if no Docker) and shows zero yamllint errors in its own diagnostic output on failure
- `task go:test` -- expected: full unit suite green, coverage report written
- `task go:run` -- expected: app still builds and starts
