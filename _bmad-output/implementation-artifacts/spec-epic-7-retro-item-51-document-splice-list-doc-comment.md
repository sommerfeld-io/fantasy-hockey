---
title: 'Tie document Field Set to writeLocked Splice List'
type: 'docs'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Before Story 7.4, `writeLocked` persisted whatever was currently in `s.doc` wholesale, so any future mutation of any `document` field persisted correctly by construction. Since 7.4, `writeLocked` only ever re-derives two of `document`'s ten fields (`LoginCodes`, `Predictions`) from `s.doc`; every other field is permanently frozen to whatever `s.raw` held at load time. Nothing documents this at `document`'s own definition — a future engineer adding a new app-writable field (following the existing `SavePrediction` pattern) would have it compile, mutate `s.doc` in memory, and return success from `writeLocked`, while the value silently never reaches disk (epic-7 retrospective, item 51).

**Approach:** Add a doc comment on the `document` type stating this constraint explicitly: only `LoginCodes`/`Predictions` are re-derived on every write; every other field must go through `writeLocked`'s splice list (`store_splice.go`'s `spliceNamedValueLocked`) to ever be persisted, and a new app-writable field needs a corresponding splice added, not just a struct field.

</frozen-after-approval>

## Implementation Notes

Added a doc comment on `document` (`store.go`) stating that only `LoginCodes`/`Predictions` are re-derived on a write, and that a new app-writable field needs its own `spliceNamedValueLocked` call inside `writeLocked`, not just a struct field. Doc-only change, no logic touched.

Verified: `gofmt -l`, `go build ./...`, `go vet ./internal/store/...`, `golangci-lint run ./internal/store/...` (0 issues), `task go:test` (full pipeline, unaffected since no logic changed), `task go:run` (builds and starts).

Nothing incomplete or risky.

**Review patches:** applied all six `patch`-routed blind-hunter findings — split the run-on final sentence into two clean sentences; cross-referenced `document`'s new doc comment from `writeLocked`'s own doc comment (bidirectional pointer, not one-directional); cross-referenced the same constraint from `README.md`'s "Every write splices only..." Design Notes bullet, so all three descriptions of the splice list point at each other; named the concrete failure mode explicitly (compiles, mutates `s.doc`, reports success, never persists) directly in the doc comment rather than leaving it implicit; named `spliceNamedValueLocked`'s restore-closure/`restoreSplices` wiring as a second, distinct thing a new field needs (not just the splice call itself) — omitting it leaves a write failure for the new field unrolled-back while `s.raw` stays mutated; and called out that new splice/rollback coverage should mirror `store_splice_test.go`'s existing coverage of the two current splices. Two findings dispositioned `false`: the spec still showing `status: in-progress` mid-review is expected workflow state, not a defect; and a request for a fully worked "here's the exact diff a future engineer should write" example was rejected as out of scope for what item 51 actually asked for (a doc-comment constraint, not a tutorial) and risks going stale relative to the surrounding code faster than a prose constraint does.

**Re-verified after patches**: `gofmt -l` clean, `go build ./...` clean, `go vet ./internal/store/...` clean, `golangci-lint run ./internal/store/...` 0 issues, `docker compose up lint-markdown-links` exit 0 clean (README.md link check), `task go:test` full pipeline green (95.4% total coverage, no regressions — doc-only change), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — the doc comment's final sentence ran two ideas together without a clear break, making it read as one long unbroken clause. Verified real. Fixed: split into two sentences.
- **low, patch** — `writeLocked`'s own doc comment described the splice list without pointing back to `document`'s new, more complete explanation. Verified real. Fixed: added a cross-reference.
- **low, patch** — `README.md`'s Design Notes bullet made the same "splice-only" claim as `document`'s doc comment with no link between the two, so the two descriptions could drift apart silently. Verified real. Fixed: added a cross-reference sentence pointing to `document`'s doc comment.
- **low, patch** — the doc comment described the *mechanism* (splice list) but not the concrete *symptom* a future engineer would actually see if they got it wrong. Verified real: the original item 51 wording is specifically about the failure being silent and success-reporting. Fixed: named the failure mode explicitly (compiles, mutates `s.doc`, reports success, never persists).
- **low, patch** — the doc comment only mentioned adding a `spliceNamedValueLocked` call, omitting that the call's restore closure must also be wired into `restoreSplices`, or a failed write for the new field leaves `s.raw` mutated without rollback. Verified real against `store_splice.go`'s actual rollback mechanics. Fixed: named both requirements explicitly.
- **low, patch** — no mention that a new splice needs its own test coverage, unlike the existing two splices which are covered in `store_splice_test.go`. Verified real. Fixed: added a sentence directing new coverage to mirror the existing pattern.
- **false** — spec frontmatter still showed `status: 'in-progress'` at review time. Not a defect: expected mid-review workflow state, flipped to `'done'` as the final step of this same build.
- **false** — reviewer suggested embedding a fully worked example diff of a hypothetical new field's splice/rollback wiring directly in the doc comment. Rejected: out of scope for what item 51 asked for (a doc-comment *constraint*, not a tutorial), and a worked example risks silently going stale relative to the surrounding code faster than a prose constraint does.

All six patched findings were independently re-verified after patching: `gofmt -l`, `go build`, `go vet`, `golangci-lint run` all clean, `docker compose up lint-markdown-links` clean for the README.md cross-reference, and the full `task go:test` suite green with zero regressions (doc-only change, no logic touched).
