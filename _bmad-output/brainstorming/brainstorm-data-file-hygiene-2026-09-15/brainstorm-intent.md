# Brainstorm Intent: Data-File Hygiene for fantasy-hockey.yml

## Context

fantasy-hockey is a small Go fantasy-hockey prediction-pool app whose only persistent state is a single hand-maintained YAML file, `src/fantasy-hockey.yml`. The `internal/store` package is the sole owner of that file: all reads and writes go through it, and every mutation is a mutex-guarded atomic write-and-rename (AD-27/AD-29). Three backlog stories fell out of this session, converged via affinity clustering and MoSCoW: making data-changing writes observable, keeping the login-code rows lean, and guaranteeing the file's own output stays yamllint-clean without adding runtime risk.

## Story A: Log data-changing actions

**Goal:** Emit a generic log line for every write to the data file.

- MUST: one generic store-level `slog.Info` line on every `writeLocked` call, reusing the existing logging convention (established in the earlier auth-logging story).
- MUST: never log raw login codes or emails — keep the existing hash-code-never-logged rule.
- SHOULD: distinguish a bootstrap-time write (`store.New` creating a fresh file) from a normal in-life write in the log line's wording.
- COULD: include a per-collection row-count delta (e.g. `login_codes: 7 -> 4`).
- WON'T: a separate persisted audit trail inside the YAML file itself — stdout logging only.

**Open question for the story-writer:** should a write that changes nothing (e.g. a no-op `RequestLoginCode` with no email match) still produce a log line, or only writes that actually touch disk? Unresolved in the memlog — decide before implementation.

## Story B: Cleanup unusable login codes

**Goal:** Remove login-code rows that can no longer be used to log in.

- MUST: "no longer usable" = expired (per the existing `loginCodeValidity` constant, measured against `clock.NowTime()`) OR already-used. **User-confirmed correction: unused-but-still-valid codes stay in the file** — do not remove anything still redeemable.
- MUST: cleanup goes through the existing mutex-guarded `writeLocked` path — no second write mechanism, no new lock.
- SHOULD: trigger cleanup opportunistically (on `store.New` / on the next write) rather than a background goroutine or cron.
- WON'T (this time): an on-demand CLI cleanup flag — parked as a fallback if opportunistic cleanup proves insufficient.

**Open risk for the story-writer:** per FR-2 (Story 1.2), a deleted expired/used row must still reject a resubmitted code identically to how it behaved before deletion — verify this is preserved. Also: `LoginCodes` is a single slice, so removing rows shifts indices/length — any existing test asserting `doc.LoginCodes[i]` by position needs re-checking.

## Story C: YAML output complies with yamllint (no runtime enforcement)

**Goal:** Guarantee the app's marshaled YAML output already satisfies the repo's `.yamllint` config, verified only outside the running binary.

- MUST: written output complies with the repo's existing `.yamllint` config — not a parallel, hand-rolled rule set.
- MUST: compliance is verified by an acceptance/integration test that runs the real `yamllint` binary against a file the app just wrote, as part of the test suite/CI.
- MUST NOT (**user-confirmed correction, supersedes earlier draft scope**): no runtime yamllint enforcement at all — not shelled out on every write, not checked on load. CI/pipelines already own this check; the binary never runs inside the shipped app. This removes the earlier flagged Dockerfile runtime-dependency risk entirely.
- WON'T: reimplementing yamllint's ruleset natively in Go.

**Open question for the story-writer:** an earlier draft assumed `go.yaml.in/yaml/v3`'s marshal defaults (indent width, quoting style) might not already satisfy `.yamllint` — confirm concretely whether a gap exists before building anything to close it.

## Cross-cutting

**Shared architectural principle:** all three stories read/write exclusively through the existing `store.writeLocked` path (mutex-guarded atomic write-and-rename, AD-27/AD-29). No story introduces a parallel write mechanism, a second lock, or bypasses `internal/store`.

**BDD/acceptance-test note:** per this repo's `CLAUDE.md`, do not silently decide whether a Gherkin acceptance test is needed for any of these three stories. Ask the human explicitly for each — they may be infra/observability work rather than user-facing behavior, as was the case for the earlier auth-logging story.
