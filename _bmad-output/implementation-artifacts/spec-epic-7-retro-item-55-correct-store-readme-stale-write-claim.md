---
title: 'Correct store/README.md Stale Write-Path Claims'
type: 'docs'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `internal/store/README.md` has two passages describing the pre-Story-7.4 whole-document-remarshal write path, both now factually wrong: the "Results (read-only)" section claims "every save re-marshals the whole document: their values are preserved, while comments, flow style and quoting are not" (the exact opposite is now true — Story 7.4 made preservation the point), and the "Design notes" section claims "Every write serializes the whole in-memory document... (AD-27)" (`writeLocked` now splices only `login_codes`/`predictions` into the originally-parsed raw node tree, per spec-7-4) — the specific staleness the epic-7 retrospective (item 55) flagged.

**Approach:** Correct both passages to describe `writeLocked`'s actual splice-based behavior, keeping each passage's existing structure/scope (no new content about AC1/AC2 tolerance or other Story 7.4 behavior not already covered by these two passages).

</frozen-after-approval>

## Implementation Notes

Corrected both passages in `internal/store/README.md`: the "Results (read-only)" section now describes the actual splice-based preservation (comments/flow style/quoting/key order survive, only block-style sequence indentation isn't guaranteed to match) instead of the old, wrong "comments... are not [preserved]" claim; the "Design notes" bullet now says `writeLocked` splices only `login_codes`/`predictions` instead of "serializes the whole in-memory document." Both changes reference `spec-7-4` for provenance, matching this file's existing citation style. No code touched — this is a documentation-only fix.

Verified: `task lint`'s `lint-markdown-links` service (0 errors), `task go:test` (full pipeline, unaffected since no Go code changed — sanity check only).

**Review patches:** applied all four surviving `patch`-routed findings — the Design Notes bullet now enumerates the seven hand-maintained sections by name (matching `store.go`'s own comment) and cross-references the block-style-sequence-indentation exception instead of silently dropping it (which risked a skim-reader concluding everything round-trips byte-for-byte); reworded "re-encoded untouched" (internally contradictory) to the precise "round-trips through the exact node objects... never reconstructed from typed fields" phrasing already used in `store.go`'s own comment; the Results-section passage's `CompactSeqIndent` mention now points to "Design notes below" instead of a bare, disconnected citation. Caught and fixed one wording bug of my own while applying these: my first patch said the exception was "noted above," but the `CompactSeqIndent` explanation bullet actually sits *below* in the same Design Notes list — corrected to point down, not up. Also normalized the two new em dashes to this file's existing spaced-hyphen convention (verified via `grep` that em dash appears nowhere else in the file). One finding (the `ResultProblems()` bullet's own, unrelated staleness) deferred to `deferred-work.md` — pre-existing, outside this item's named scope. `lint-markdown-links` re-run clean after all patches.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — Design Notes bullet dropped the block-style-sequence-indentation exception the Results-section passage states, risking a skim-reader concluding byte-identical round-tripping. Verified real. Fixed: cross-referenced.
- **low, patch** — "re-encoded untouched" is internally contradictory phrasing. Verified real. Fixed: reworded to match `store.go`'s own comment phrasing.
- **low, patch** — vague "every other section's own nodes untouched" without naming which sections. Verified real against `store.go`'s own enumeration. Fixed: named all seven.
- **low, patch** — no cross-reference from the `CompactSeqIndent` mention to its fuller Design Notes explanation. Verified real. Fixed: added "see Design notes below."
- **low, defer** — `ResultProblems()` bullet is stale (missing the two Story 7.4 categories), but pre-existing and outside this item's named scope (a different bullet than the one epic-7-retro-item-55 flagged). Deferred to `deferred-work.md`.
