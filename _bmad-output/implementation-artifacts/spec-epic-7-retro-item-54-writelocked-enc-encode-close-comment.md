---
title: 'Document Why writeLocked''s enc.Encode(s.raw)/enc.Close() Are Untested'
type: 'docs'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `writeLocked` (`internal/store/store.go`) has a comment above `loginCodesNode.Encode(cleaned)`/`predictionsNode.Encode(s.doc.Predictions)` justifying why those two error branches are defensive and intentionally untested. A few lines later, `enc.Encode(s.raw)` and `enc.Close()` have no such comment, even though their own error branches are equally untested (confirmed via coverage: `store.go:1046.42,1049.3` and `1050.36,1053.3` both show `0` executions) and the existing nearby comment only speaks to the two smaller typed-slice `Encode` calls, not these two (epic-7 retrospective, item 54).

**Approach:** Add a doc comment above `enc.Encode(s.raw)` stating why both its and `enc.Close()`'s error branches are unreachable in practice: the writer is a `*bytes.Buffer` (whose `Write` never returns an error, per the stdlib), and `s.raw` is always either freshly parsed from valid YAML or has just had two splices written into it that already round-tripped through their own successful `Encode()` calls immediately above — so none of the encoder's actual failure modes (writer I/O error, invalid UTF-8 scalar content, unknown `Node.Kind`, malformed `!!binary` data) can occur here.

</frozen-after-approval>

## Implementation Notes

Added a doc comment above `enc.Encode(s.raw)`/`enc.Close()` in `internal/store/store.go`'s `writeLocked`, explaining why both error branches are unreachable in practice: the emitter's write handler can't fail (writer is `&buf`, a `*bytes.Buffer`, whose `Write` always returns nil per the stdlib), and `s.raw` can't hold content the emitter itself can't serialize (invalid UTF-8, unknown `Node.Kind`, malformed `!!binary`) because it's always either freshly parsed from valid YAML or just had the two typed splices written into it, both already proven encodable moments earlier. Doc-only change, no logic touched.

Traced the failure surface in `go.yaml.in/yaml/v3@v3.0.5`'s `yaml.go`/`encode.go`/`emitterc.go`/`writerc.go` before writing the comment, rather than assuming: both `Encoder.Encode` and `Encoder.Close` only return non-nil when the internal emitter panics with a `yamlError`, reachable via (a) a writer-handler I/O error (`writerc.go:44`, routed through `bytes.Buffer.Write`, which never errors), (b) an internal emitter event-ordering/protocol-invariant violation (`emitterc.go`'s many `yaml_emitter_set_emitter_error` sites, e.g. "expected DOCUMENT-START or STREAM-END") - unreachable because `Encode` always drives the emitter through its own single, correctly-ordered event sequence, never a hand-built or externally-driven one - or (c), `Encode` only, one of three content-validation failures (invalid UTF-8 scalar data, an unrecognized `Node.Kind`, or badly-tagged `!!binary` data) that `s.raw`'s provenance rules out.

Verified: `gofmt -l` clean, `go vet ./...` clean, `go build ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions since no logic changed), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied all three `patch`-routed blind-hunter findings — added the bootstrap path (`st.raw.Encode(st.doc)` inside `New`, not `loadExisting`) as a third provenance branch for `s.raw`, which the original comment's two-way dichotomy missed entirely; corrected the Implementation Notes' overclaimed "exactly four call sites" to the fuller, verified picture including the emitter's internal event-ordering/protocol-invariant checks (real failure surface, confirmed unreachable here because `Encode` always drives the emitter through its own single correctly-ordered event sequence); and clarified that `Close` never re-traverses content, so the three content-validation failure modes apply to `Encode` only, not both functions equally as the original wording implied. One finding dispositioned `low, reject`: consolidating this comment with the pre-existing one above `loginCodesNode.Encode`/`predictionsNode.Encode` - rejected because item 54's own frozen Intent explicitly asked for this pair to get its own comment (the existing one "only justifies two different, smaller typed-slice encode calls"), and merging would blur that intentional distinction. Two findings dispositioned `false`: spec/sprint-status tracking still showing in-progress/open mid-review is expected workflow state; and the BDD/acceptance-test policy question was already resolved as a standing decision (epic-8-item-63 build) for pure hardening changes with no observable behavior - a doc-only comment change qualifies.

**Re-verified after patches**: `gofmt -l` clean, `go vet ./...` clean, `go build ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions since no logic changed), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — the comment's provenance dichotomy for `s.raw` ("freshly parsed from valid YAML (loadExisting) or has just had the two splices above written into it") omitted a third real path: `New`'s bootstrap branch builds `s.raw` via `st.raw.Encode(st.doc)` before `writeLocked` and its splices ever run. Verified real against `store.go` lines 359-361. Fixed: named all three provenance paths explicitly.
- **low, patch** — the comment and Implementation Notes claimed "exactly four call sites" for the encoder's failure surface, omitting a real fifth class: the emitter's internal event-ordering/protocol-invariant checks (`emitterc.go`'s `yaml_emitter_set_emitter_error` sites), reachable via `e.must(yaml_emitter_emit(...))` on the same traversal path this code exercises. Verified real against library source; also verified unreachable in practice (correct event ordering is always guaranteed by `Encode`'s own internal driving, never influenced by `s.raw`'s content). Fixed: named this failure class and why it can't occur here.
- **low, patch** — the comment lumped `Encode` and `Close` under one undifferentiated OR-list of failure causes, but `Close` never re-traverses content (confirmed via `emitterc.go`: `finish()` only emits a stream-end event), so the three content-validation failure modes apply to `Encode` only. Verified real. Fixed: clarified the asymmetry inline.
- **low, reject** — suggested consolidating this comment with the pre-existing one above `loginCodesNode.Encode`/`predictionsNode.Encode`. Rejected: item 54's own frozen Intent explicitly asked for a comment covering this specific pair because the existing one only justifies the two smaller typed-slice calls; merging would blur that intentional distinction between the two different (if related) reasoning chains.
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-progress/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.
- **false** — reviewer flagged that CLAUDE.md's BDD policy requires asking whether an acceptance test is needed for non-feature work, with no record of that question being asked. Not a defect: this exact question was already resolved as a standing decision in the epic-8-item-63 build ("keep deciding silently for pure test/doc hardening") - a doc-only comment change with zero observable behavior change squarely fits that standing policy.

All three patched findings were independently re-verified after patching: `gofmt -l`, `go vet`, `go build`, `task go:lint` all clean, and the full `task go:test`/`task go:run` pipeline green with zero regressions (doc-only change, no logic touched).
