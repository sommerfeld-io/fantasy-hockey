---
title: 'Document and Test the Rules Page XSS-Safety Invariant'
type: 'chore'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `rules.go:38`'s `var rulesHTML = template.HTML(mustRenderGameRules())` deliberately disables Go's `html/template` auto-escaping for the rendered game rules. Safety today rests entirely on an unstated assumption — goldmark's default `Unsafe: false` renderer strips raw HTML blocks and neutralizes dangerous URL schemes — verified empirically in the epic-8 retrospective (`<script>...</script>` renders as `<!-- raw HTML omitted -->`; `[text](javascript:...)` renders with an empty `href`). No live vulnerability exists today, but nothing documents this as a load-bearing invariant or tests it, and `gosec` (the linter that flags `template.HTML(...)` conversions, rule G203) isn't enabled in this repo's `.golangci.yml` (epic-8 retrospective, item 63).

**Approach:** Add a doc comment on `gameRulesRenderer`/`rulesHTML` stating the safety invariant explicitly and warning against enabling `html.WithUnsafe()` or any raw-HTML-permitting option without re-reviewing it. Add a regression test that renders a literal markdown string containing raw `<script>` HTML and a `javascript:` link directly through `gameRulesRenderer.Convert` (mirroring the pattern the Linkify fix already established for `TestRulesHTMLShouldNotContainLinks`), asserting neither appears unescaped/executable in the output.

</frozen-after-approval>

## Implementation Notes

`rules.go`'s `rulesHTML` doc comment now states the safety invariant explicitly and warns against `html.WithUnsafe()`/raw-HTML-permitting options without re-review. Added `TestGameRulesRendererShouldNeutralizeRawHTMLAndDangerousLinks` to `rules_test.go`, rendering a literal `<script>` block and a `javascript:` link directly through `gameRulesRenderer.Convert`, asserting neither survives unescaped.

Verified independently: `gofmt -l`, `go build ./...`, `go vet ./internal/web/...`, and all 6 Rules-related tests pass. **Empirically proved the test actually guards the invariant**, not just trusts it: temporarily enabled `html.WithUnsafe()` on `gameRulesRenderer` (simulating the exact regression the new doc comment warns against) and re-ran the test — it correctly failed, showing the raw `<script>` tag and the live `javascript:` link that would otherwise ship. Reverted immediately after (confirmed via `grep` that the unsafe-HTML import and option are both gone). `task go:test` (full pipeline) and `task go:run` both clean on the restored, fixed code.

Nothing incomplete or risky.

**Review patches:** applied five of the six surviving `patch`-routed findings — corrected the doc comment's "empty comment" claim to the actual `<!-- raw HTML omitted -->` string (verified against `goldmark@v1.8.6/renderer/html/html.go`); expanded the test from 2 loose substring checks to an 8-case table covering block *and* inline raw HTML, all four dangerous URL schemes (`javascript:`/`vbscript:`/`file:`/`data:`, verified against `IsDangerousURL`'s exact scheme list), image `src`, and autolinks; tightened every assertion from a loose substring check to the specific expected output (`<!-- raw HTML omitted -->`, `href=""`, `src=""`) rather than just an absence check; added a goldmark version anchor (`v1.8.6`) to the doc comment; and clarified the threat-model framing to match `mustRenderGameRules`'s own "never runtime input" comment rather than leaving the two seemingly in tension. Every new test case was empirically confirmed against the real renderer (a throwaway probe) before being asserted, including the autolink case's non-obvious safe rendering (the URL text itself stays visible as safe plain text; only the `href` is neutralized) — the test's `mustNotContain` check for that row targets `href="javascript` specifically, not a bare `"javascript:"` substring, to avoid a false failure against that safe text. One finding (no automated `gosec`/CI guardrail against a *future* unrelated `template.HTML(...)` call site) deferred as a larger, separate, pre-existing repo-wide gap. Asked the user about CLAUDE.md's Gherkin-decision policy (surfaced by the review, applicable to several of this session's recent builds, not just this one) — confirmed: keep deciding "no Gherkin" silently for pure internal test/doc hardening with no observable behavior change, without asking each time.

**Independently re-verified the strengthened test actually guards the invariant**: temporarily re-enabled `html.WithUnsafe()` on `gameRulesRenderer` a second time (after the test expansion) — the test failed exactly as expected, showing the raw `<script>` and live `javascript:`/`vbscript:`/etc. hrefs the doc comment warns against. Reverted immediately after. `gofmt -l`, `go build ./...`, `go vet ./internal/web/...`, all 8 subtests, `task go:test` (full pipeline), and `task go:run` all re-run clean.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — doc comment said raw HTML is "rendered as an empty comment"; goldmark actually emits the literal `<!-- raw HTML omitted -->`. Verified against `goldmark@v1.8.6/renderer/html/html.go`. Fixed.
- **low, patch** — test only exercised block-level raw HTML, not the separate inline raw-HTML code path. Verified real. Fixed: added a case, empirically confirmed the actual safe output first.
- **low, patch** — test only covered `javascript:`, not `vbscript:`/`file:`/`data:` (all four are in goldmark's `IsDangerousURL`). Verified against the exact scheme list in `renderer/html/html.go`. Fixed: added all three, each empirically confirmed before asserting.
- **low, patch** — dangerous URLs reachable via image `src` and autolinks weren't tested (only link `href`). Verified real (`renderImage`/`renderAutoLink` both call `IsDangerousURL`). Fixed: added both, including the non-obvious autolink case (URL text stays visible as safe plain text; only `href` is neutralized) via an empirical probe first.
- **low, patch** — assertions were loose substring checks that would pass even if the safe output changed to a different unsafe form. Verified real. Fixed: tightened to the exact expected output per case.
- **low, patch** — no goldmark version anchor in the comment. Verified real. Fixed: added "goldmark v1.8.6, go.mod."
- **low, patch** — threat-model framing didn't reconcile with `mustRenderGameRules`'s own "never runtime input" comment. Fixed: clarified the comment cross-references it.
- **low, defer** — no automated `gosec`/CI guardrail against a *future*, different `template.HTML(...)` call site. Verified real (`gosec` absent from `.golangci.yml`). Pre-existing repo-wide gap, larger than this item's one-call-site scope. Deferred to `deferred-work.md`.
- **process, resolved via user decision** — CLAUDE.md's "ask before deciding whether Gherkin applies" policy hadn't been explicitly confirmed for this session's recent internal-hardening builds. Asked the user directly: confirmed to keep deciding "no" silently for pure test/doc hardening with no observable behavior change, without asking each time going forward.

All eight code/doc findings were independently re-verified after patching: reverted the fix a second time (against the now-expanded 8-case test table) and confirmed every subtest fails exactly as expected, proving the strengthened test genuinely guards the invariant end to end.
