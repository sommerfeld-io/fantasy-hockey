---
title: 'Add a GoDoc Guardrail on DefaultSeason Against Cross-Package Branching'
type: 'docs'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `store.DefaultSeason` is exported and currently used only inside `internal/store` itself (the bootstrap branch). Nothing stops a future change from importing it elsewhere (e.g. `internal/web`) and branching on it - `if season == store.DefaultSeason { ... }` - to build season-aware behavior. Epic 6's own Requirements & Constraints explicitly forbid exactly that class of logic in v1: no in-app season-selector, cross-season query, or history/Hall-of-Fame view; every screen must show only the current season's data (epic-6 retrospective, item 47).

**Approach:** Add a sentence to `DefaultSeason`'s GoDoc comment (`internal/store/store.go`) stating that other packages must never branch on this constant to build season-aware behavior - `Store.Season()` is the one supported way to read a loaded store's current season - and that doing so would start building the cross-season-aware logic Epic 6 explicitly forbids in v1.

</frozen-after-approval>

## Implementation Notes

Added a paragraph to `DefaultSeason`'s GoDoc comment in `internal/store/store.go` stating that other packages must never branch on the constant to build season-aware behavior, naming `Store.Season()` as the one supported way to read a loaded store's current season, and naming the specific v1 constraints (no season-selector, cross-season query, or history/Hall-of-Fame view) that such branching would start building toward. Confirmed via grep that no non-test code outside `internal/store` currently imports/references `DefaultSeason` - this is a preventive guardrail, not a fix for an existing violation. Doc-only change, no logic touched.

Verified: `gofmt -l` clean, `go vet ./...` clean, `go build ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions since no logic changed), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied all four `patch`-routed blind-hunter findings — added the test-code carve-out the retro item's own wording ("outside store/test code") and `sprint-status.yaml`'s action-item text both specify, which the first draft's absolute "must never branch" wording dropped entirely, contradicting existing legitimate comparisons in `main_test.go`/`store_test.go`; removed the Markdown-style backtick code span around the example (confirmed via `go doc ./internal/store DefaultSeason` that Go doc comments have no inline-code-span syntax - the backticks rendered as literal characters, not code formatting, and no other comment in this file uses backticks in prose); lowercased "Epic 6" to "epic-6" to match this file's and the codebase's established lowercase convention for epic references; and added a one-clause bridge ("Separately:") between the two independently-motivated paragraphs on `DefaultSeason` (staleness/rebuild risk, then anti-branching rule) for readability. One finding dispositioned `low, reject`: no mechanical backstop (test/lint rule) enforces the anti-branching rule the way item 42's paired format test enforces its own guardrail - rejected because the retro item's own wording explicitly scopes this to "a GoDoc guardrail," a documentation-only ask; a mechanical enforcement mechanism (e.g. a custom lint rule or depguard-style restriction) is a materially larger, separate piece of work already tracked as its own backlog item (epic-4-retro-item-33, adopting depguard for import rules). Two findings dispositioned `false`: spec/sprint-status tracking still showing in-progress/open mid-review is expected workflow state.

**Re-verified after patches**: `go doc ./internal/store DefaultSeason` renders cleanly with no stray backticks and consistent lowercase epic references, `gofmt -l` clean, `go vet ./...` clean, `go build ./...` clean, `task go:lint` 0 issues, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **medium, patch** — the first draft's absolute "must never branch on this constant" wording dropped the test-code carve-out both the source retro finding and the sprint-status action item explicitly specify ("outside store/test code"), directly contradicting existing legitimate comparisons in `main_test.go:303-304,333-334` and `store_test.go:120-121,353-354`. A reader taking the comment literally would flag those tests as violations. Verified real. Fixed: added the carve-out explicitly, naming both files as sanctioned examples.
- **low, patch** — the example used Markdown-style backticks (`` `if season == store.DefaultSeason` ``), which Go doc comments don't support as code-span syntax. Verified via `go doc ./internal/store DefaultSeason`: backticks rendered as literal characters, not code formatting - also inconsistent with the rest of the file, where every other backtick is a struct tag, never doc-comment prose. Fixed: removed the backticks.
- **low, patch** — "Epic 6" was capitalized as a prose noun, the only such instance in the file; every other epic reference in this codebase's comments is lowercase (`epic-6 retrospective`, `epic-7 retro finding`, etc.), including two lines away in the same paragraph's own citation. Verified real. Fixed: lowercased to "epic-6".
- **low, patch** — the two paragraphs on `DefaultSeason` (staleness/rebuild risk, then anti-branching rule) sat stacked with no transition between them. Verified real, minor readability issue. Fixed: added a one-clause bridge ("Separately:").
- **low, reject** — no mechanical backstop (test or lint rule) enforces the anti-branching rule, unlike item 42's paired `TestDefaultSeasonShouldMatchTheExpectedFormat`. Not fixed: the retro item's own wording explicitly scopes this to "a GoDoc guardrail" - a documentation-only ask. Mechanical enforcement (e.g. a custom lint rule restricting cross-package references) is a materially larger, separate piece of work, already tracked as its own open backlog item (epic-4-retro-item-33, adopting depguard).
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-progress/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.

All four patched findings were independently re-verified after patching: `go doc` output checked directly, and the full `gofmt`/`go vet`/`go build`/`task go:lint`/`task go:test`/`task go:run` pipeline is clean.
