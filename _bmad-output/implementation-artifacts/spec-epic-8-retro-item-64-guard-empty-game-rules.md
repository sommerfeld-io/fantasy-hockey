---
title: 'Guard Against an Emptied game-rules.md'
type: 'bugfix'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** An emptied or whitespace-only `game-rules.md` currently renders a silently blank Rules tab with the nav still showing it active and zero error anywhere. Confirmed empirically in the epic-8 retrospective: rendering whitespace-only markdown through `gameRulesRenderer` returns `err=nil, output=""`; since `shellData.Rules` is `template.HTML` (empty-string falsy) and `templates/shell.html`'s tab chain (`{{if .Predict}}...{{else if .Rules}}{{end}}`) has no final bare `{{else}}`, none of the four branches match and `<main class="shell-content">` renders completely empty for the app's entire uptime (computed once at package init). Low likelihood — `game-rules.md` is a repo-committed, PR-reviewed file, not user input — but cheap to close and consistent with `mustRenderGameRules`'s existing fail-fast philosophy for its other two failure modes (epic-8 retrospective, item 64).

**Approach:** Add a third check to `mustRenderGameRules` (`rules.go`) that panics when the rendered output is empty/whitespace-only after trimming, matching the existing "programmer error caught at build/startup time" pattern for the function's other two panic branches.

</frozen-after-approval>

## Implementation Notes

`mustRenderGameRules` (`rules.go`) now panics when the rendered output is empty after `strings.TrimSpace`, matching its existing two panic branches' doc comment and philosophy. Added `TestGameRulesRendererShouldRenderWhitespaceOnlyMarkdownAsEmpty` to `rules_test.go`, proving the precondition the check relies on (since `mustRenderGameRules` itself always reads the real, non-empty embedded file and can't be unit-tested for the panic directly).

Verified independently, including an end-to-end check beyond the unit test: `gofmt -l`, `go build ./...`, `go vet ./internal/web/...`, all Rules tests pass. **Empirically proved the panic actually fires**: temporarily replaced the embedded `rules/game-rules.md` copy with whitespace-only content, rebuilt the real binary, and ran it — it panicked at package-init time with a clear, operator-legible message (`web: embedded game rules rendered empty - rules/game-rules.md may be blank or whitespace-only`), before even starting to listen. Reverted immediately after (confirmed via `git status`). `task go:test` (full pipeline) and `task go:run` (with the real file restored) both clean.

Nothing incomplete or risky.

**Review patches:** applied all four surviving `patch`-routed findings — extracted the blank-check into a small named `gameRulesRenderedBlank` helper so the test calls the exact function the panic acts on, instead of duplicating its logic independently (which would have kept passing even if the real check were weakened or removed); fixed the panic message, which pointed operators at `rules/game-rules.md` (the generated copy) instead of `docs/game-rules.md` (the canonical source they'd actually need to fix) — a real, if low-likelihood, operator-facing bug in the original fix; tightened the doc comment to note `TrimSpace` is defense-in-depth, not something the test proves a real difference for, since goldmark's output for this class of input is confirmed always the exact empty string, never non-empty whitespace; and added `TestGameRulesRendererShouldNotFlagOrdinaryMarkdownAsBlank`, a should-not counterpart (per this repo's own TDD convention), now cheap to add given the extracted helper.

Two findings noted but not acted on: the sprint-status.yaml/spec-status "still open" observation described expected mid-review workflow state, not a defect (resolved by this same build's own Finalize/Commit steps that follow); the BDD/Gherkin-policy question was already resolved by the user's standing decision from the epic-8-item-63 build (pure hardening, no observable behavior change → decide silently, no need to re-ask).

**Re-verified end-to-end after the patches**: repeated the empty-embed-and-rebuild check a second time — confirmed the panic now correctly names `docs/game-rules.md` as the file to fix. `gofmt -l`, `go build ./...`, `go vet ./internal/web/...`, all Rules tests (8 subtests + 2 new + existing, all pass), `task go:test` (full pipeline), and `task go:run` all clean after the patches.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — the test only duplicated the panic's `strings.TrimSpace(...) == ""` check independently, never exercising `mustRenderGameRules`'s actual guard clause; a future weakening of that condition would go undetected. Verified real. Fixed: extracted `gameRulesRenderedBlank` as a small named helper, called directly by both the panic and the test.
- **low, patch** — the panic message named `rules/game-rules.md` (the generated copy) as what to fix, not `docs/game-rules.md` (the canonical source) — a real operator-facing bug in the original fix, verified against `rules.go`'s own doc comment on `gameRulesFS`. Fixed.
- **low, patch** — the doc comment overclaimed that `TrimSpace` was proven necessary by the test, when goldmark's output for whitespace-only input is confirmed (empirically, by the reviewer and independently by me) to always be the exact empty string, never non-empty whitespace. Fixed: reworded to call it defense-in-depth.
- **low, patch** — no should-not counterpart proving ordinary markdown doesn't trip the new guard. Real, and now cheap given the extracted helper. Fixed: added `TestGameRulesRendererShouldNotFlagOrdinaryMarkdownAsBlank`.
- **false** — sprint-status.yaml/spec `status: 'in-progress'` still open. Not a defect: expected mid-review workflow state, resolved by this same build's Finalize/Commit steps.
- **false, already resolved** — whether the Gherkin-policy question was explicitly put to the user for this change. Already settled by the user's standing decision made during the epic-8-item-63 build (silently decide "no Gherkin" for pure hardening with no observable behavior change) — this change qualifies (a startup panic on a corrupted build artifact, no HTTP-observable path), so it's covered by that existing decision, not a new gap.

All four code/doc findings were independently re-verified after patching: repeated the empty-embed-and-rebuild end-to-end check a second time and confirmed the panic message now correctly names the canonical source.
