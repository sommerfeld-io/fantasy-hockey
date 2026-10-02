---
title: 'Rules Page No-Links and List-Rendering Test'
type: 'refactor'
created: '2026-09-29'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context: []
baseline_commit: '68dc970f12ba4ecf98d47fb9d377c87c6e72e464'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `internal/web/rules_test.go`'s `TestRulesHTMLShouldNotContainRawMarkdownSyntax` only asserts the absence of raw, unrendered markdown syntax (`"## "`, `"**"`) — it never checks the rendered HTML for `<a href=` tags, and no other test checks `docs/game-rules.md`'s source for markdown links either. Worse, `rules.go`'s `gameRulesRenderer` uses `extension.GFM`, which bundles `extension.Linkify` — confirmed at the library source level (`goldmark@v1.8.6/extension/gfm.go`'s `GFM.Extend` calls `Linkify.Extend(m)`) and empirically (a bare `https://example.com` or `pool@example.com` in prose auto-converts to a real `<a>`/`mailto:` link, no markdown link syntax needed) — so a future edit to `game-rules.md` that merely mentions a URL or email in passing prose would silently violate the page's own documented "no links" convention (epics.md Story 8.2 AC2, and `docs/game-rules.md`'s own header comment: "Avoid links in this file... a link to another docs/ page has nothing to resolve against"), with nothing to catch it (epic-8 retrospective, item 61).

**Approach:** Add a `<a `-absence assertion and a list-rendering assertion (`<ul>`/`<li>`, exercising the existing "Before the season"/"Playoffs" bullet list) to `rules_test.go`. Whether to also remove `extension.Linkify` from `gameRulesRenderer`'s options (closing the gap at the source, not just testing for it) is the one open decision below.

## Boundaries & Constraints

**Always:**
- New assertions run against the real embedded `rulesHTML` (the actual rendered `docs/game-rules.md`), not a synthetic string — consistent with this test file's existing style.
- Verify with `task go:test` and `task go:run` after the change.
- If `extension.Linkify` is dropped, `extension.Table`/`extension.Strikethrough`/`extension.TaskList` (GFM's other three components, per `goldmark@v1.8.6/extension/gfm.go`) must still be enabled — the Scoring table (`<table>`) already relies on `extension.Table`, and `TestRulesHTMLShouldRenderTheEmbeddedMarkdownToHTML` must keep passing unchanged.

**Never:**
- Do not edit `docs/game-rules.md`'s content — it already contains no links or bare URLs (verified), so no source-content change is needed to make new assertions pass.
- Do not touch `TestEmbeddedGameRulesShouldMatchTheCanonicalSource` or any other existing test's assertions.
- No Gherkin acceptance test — this hardens existing internal/web test coverage; no user-facing behavior changes (the rendered page already has no links today, before or after this change).

**Decided (2026-09-29, human-confirmed):**
- Remove `extension.Linkify` from `gameRulesRenderer`'s options — closes the gap at the source, not just at the test. Keep `extension.Table`/`extension.Strikethrough`/`extension.TaskList` enabled.
- Token count (~1,900 estimated, above the 1,600 soft-limit): keep the full spec — the Design Notes' verified goldmark/Linkify behavior would otherwise need re-deriving by the implementer; this remains a small, single-goal spec regardless.

</frozen-after-approval>

## Code Map

- `src/internal/web/rules.go:16` — `var gameRulesRenderer = goldmark.New(goldmark.WithExtensions(extension.GFM))` → change to `goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList)` (GFM's other three components minus `Linkify`, confirmed via `goldmark@v1.8.6/extension/gfm.go`'s `GFM.Extend`). Add `"github.com/yuin/goldmark/extension"`'s existing import stays; no new import needed since `extension.Table`/`extension.Strikethrough`/`extension.TaskList` are already exported from the same package `extension.GFM` came from.
- `src/internal/web/rules_test.go` (69 lines today) — add two assertions to `TestRulesHTMLShouldNotContainRawMarkdownSyntax` (or a new test, implementer's call): `!strings.Contains(string(rulesHTML), "<a ")` and `strings.Contains(string(rulesHTML), "<ul>")` (or `<li>`, either proves list rendering). `rulesHTML` (`rules.go:31`) is already package-visible to this test file.
- `docs/game-rules.md` — confirmed via `grep` to contain zero markdown links (`[text](url)`) and zero bare URLs/emails today, so the list-rendering assertion has real content to exercise (`## The two phases`'s two-item bullet list, "Before the season"/"Playoffs") without needing any source edit.
- `src/internal/web/rules_test.go`'s existing `TestRulesHTMLShouldRenderTheEmbeddedMarkdownToHTML` already asserts `<table>` (proving `extension.Table` stays enabled after the `Linkify` removal) — no change needed there.

## Tasks & Acceptance

**Execution:**
- [x] `src/internal/web/rules_test.go` -- add a `<a `-absence assertion and a `<ul>`/`<li>` list-rendering assertion against `rulesHTML` -- closes the epic-8 retro's identified test-coverage gap.
- [x] `src/internal/web/rules.go` -- narrow `goldmark.WithExtensions(extension.GFM)` to `goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList)` -- removes `Linkify`, closing the gap at the source per the human's decision.

**Acceptance Criteria:**
- Given the real embedded `game-rules.md`, when `rulesHTML` is rendered, then no `<a ` tag appears anywhere in it and both new tests pass.
- Given the change, when `task go:test` runs, then the full suite passes green with no coverage regression in `internal/web`.
- Given a hypothetical future edit to `game-rules.md` that adds a bare URL or email in prose, when the test suite runs, then `TestRulesHTMLShouldNotContainRawMarkdownSyntax` (or its replacement) passes anyway, because the URL/email now renders as plain text (Linkify removed) rather than a real link -- proving the fix closes the gap at the source, not just at the test.

## Implementation Notes

`rules.go`'s `gameRulesRenderer` narrowed to `goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList)`, doc comment updated to explain why `Linkify` is deliberately excluded. `rules_test.go` gained `TestRulesHTMLShouldNotContainLinks` and `TestRulesHTMLShouldRenderLists`, both against the real embedded `rulesHTML`. No other file touched.

Verified independently: read the full diff against `baseline_commit` (scoped exactly to the two Code Map entries, nothing else). `gofmt -l`, `go build ./...`, `go vet ./internal/web/...`, and `go test ./internal/web/... -run TestRulesHTML -v` (4/4 pass) all re-run clean. **Empirically proved AC3**, not just trusted it: temporarily appended a bare `https://example.com` and `pool@example.com` to the embedded `game-rules.md` copy, re-ran the tests — `TestRulesHTMLShouldNotContainLinks` still passed, confirming the fix prevents linkification at the source rather than merely passing against today's link-free content. Reverted the file immediately after (confirmed clean via `git status`). `task go:test` (full pipeline) and `task go:run` both clean. Built the binary and did a full live walkthrough — real login flow, real session, real `GET /rules` — confirming the only `<a href>` tags on the rendered page are the nav items and the one GitHub link the shell template adds (not part of `game-rules.md`'s own content), with both bullet lists and the Scoring table still rendering correctly.

Nothing incomplete or risky.

**Review patches:** applied all four surviving `patch`-routed findings from the Review Triage Log — added a synthetic-input case to `TestRulesHTMLShouldNotContainLinks` (renders a literal bare-URL/email string directly through `gameRulesRenderer.Convert`, decoupled from `docs/game-rules.md`'s actual content), corrected the "fourth component" ordinal error, tightened both `rules.go`'s comment and the two test docstrings to state precisely what's proven, and fixed the error-message text mismatch. **Independently re-verified the synthetic-input fix specifically**: reverted `gameRulesRenderer` back to `extension.GFM` a second time and confirmed `TestRulesHTMLShouldNotContainLinks` now correctly fails (previously it passed silently) — the gap the review found is genuinely closed, not just patched over. `gofmt -l`, `go build ./...`, `go vet ./internal/web/...`, `go test ./internal/web/... -run TestRulesHTML -v` (4/4 pass), `task go:test`, and `task go:run` all re-run clean after the patches.

## Spec Change Log

## Review Triage Log

*(blind-hunter, edge-case-hunter, verification-gap on the full diff since `baseline_commit`)*

- **medium / patch** — `TestRulesHTMLShouldNotContainLinks` doesn't actually pin the Linkify-removal fix independent of `docs/game-rules.md`'s current content — it only asserts against the real embedded file, which happens to contain nothing linkifiable today. **Independently re-verified, twice**: rendered the real file through the *old* `extension.GFM` config (Linkify included) in a throwaway program — zero `<a ` tags, confirming the file has nothing to linkify either way; then I directly reverted `rules.go`'s fix back to `extension.GFM` in the working tree and re-ran the test — it still passed. So a future code regression (someone widening the extension list back to include Linkify) would ship with this test green, and the gap would only surface once a bare URL/email happens to be added to `game-rules.md`'s prose. Independently flagged by both the verification-gap layer and blind-hunter (2 findings, same root cause, merged). **Disposition: patch** — add a test that renders a literal markdown string containing a bare URL/email directly through `gameRulesRenderer.Convert` (not `rulesHTML`), decoupling the guarantee from `docs/game-rules.md`'s current wording.
- **low / patch** — `rules.go`'s new comment calls Linkify "`extension.GFM`'s fourth component," but `goldmark@v1.8.6/extension/gfm.go`'s `GFM.Extend` calls `Linkify.Extend(m)` **first**, before `Table`/`Strikethrough`/`TaskList`. Verified directly against the vendored source. **Disposition: patch** — reword to avoid the wrong ordinal claim (e.g. "one of `extension.GFM`'s four components").
- **low / patch** — several new comments/docstrings overclaim precision: `rules.go`'s comment and `TestRulesHTMLShouldNotContainLinks`'s docstring read as if removing Linkify (plus the test) fully enforces the page's "no links" convention, but CommonMark core still supports explicit `[text](url)` link syntax and `<https://...>` autolinks regardless of which goldmark extensions are enabled — Linkify-removal only closes the "bare URL/email in prose" vector specifically. Verified: neither of those two syntaxes is part of `extension.GFM`/`Linkify`. Separately, `TestRulesHTMLShouldRenderLists`'s docstring frames itself as proving list rendering "still works... after `gameRulesRenderer` was narrowed," implying list rendering was at risk from the extension change — it wasn't (bullet-list parsing is CommonMark core, unaffected by any of `Table`/`Strikethrough`/`TaskList`/`Linkify`). **Disposition: patch** — tighten both comments' wording to state precisely what's proven (auto-linkification of bare URLs/emails specifically is blocked at the source; an explicit link would still render if added, caught only by the blanket content-check test, not prevented at the config level; the lists test is a plain regression-safety check, not evidence tied to the Linkify removal specifically).
- **low / patch** — `TestRulesHTMLShouldNotContainLinks`'s `t.Errorf` message reads `"...no <a > tags..."` (implying a closed `<a >` pattern) but the actual assertion checks for the substring `"<a "` (open-tag prefix, no closing bracket) — could mislead someone debugging a failure. Verified against the exact source. **Disposition: patch** — align the message text with the actual matched substring.
- **low, accepted, handled outside this diff** — `sprint-status.yaml`'s `epic-8-retro-item-61-...` entry is still `open`; this diff (correctly) doesn't touch it since the spec has no `story_key` (freeform, not an epic story) and the repo's own convention (confirmed in this session's prior work) is to update it in a separate follow-up commit after the main change lands, not to bundle it into the implementation diff. Will be updated after this spec is marked `done` and committed, matching precedent.


## Design Notes

Empirically confirmed (epic-8 retrospective): `goldmark.New(goldmark.WithExtensions(extension.GFM))` auto-converts a bare `https://example.com` to `<a href="https://example.com">https://example.com</a>` and a bare `pool@example.com` to `<a href="mailto:pool@example.com">pool@example.com</a>` — no markdown link syntax required. A plain relative reference like `docs/foo.md` (no scheme) does not get linkified. This is `extension.Linkify`'s behavior specifically, confirmed at the library source (`goldmark@v1.8.6/extension/gfm.go`): `GFM.Extend` calls `Linkify.Extend(m)`, `Table.Extend(m)`, `Strikethrough.Extend(m)`, `TaskList.Extend(m)` — each is independently addressable.

## Verification

**Commands:**
- `cd src && go build ./...` -- expected: compiles clean.
- `cd src && go test ./internal/web/... -run TestRulesHTML -v` -- expected: both existing and new assertions pass.
- `task go:test` -- expected: full suite green (lint, vet, tests, gocyclo, licenses, govulncheck), no coverage regression.
- `task go:run` -- expected: app still builds and starts; manually verify `/rules` still renders the Scoring table and both bullet lists correctly (only `Linkify` is removed; `Table`/`Strikethrough`/`TaskList` stay enabled).
