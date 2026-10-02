---
title: 'Incomplete Award Picks Are Flagged'
type: 'feature'
created: '2026-10-02'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
baseline_commit: 'd653d7a8b2c0abd95e942f10be68ffdf012301d4'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** On the "Player awards" sheet, an award with only 1 or 2 of its 3 finalists filled is silently not saved: the player is redirected as if everything worked (Story 2.6 behavior, now superseded).

**Approach:** Flag a partly filled award with the inline caption "Pick all 3 finalists for this award, or clear it." on its empty slots and re-render the sheet (200) with the input kept, while still saving every other fully filled award in the same submit. An entirely blank award stays silently unsaved (FR-11). A typed name that matches no suggestion, or a repeated name, keeps the existing whole-submit rejection. Clearing a slot of an already saved award flags it and leaves its saved picks untouched. Source: Story 2.7 in `_bmad-output/planning-artifacts/epics.md`.

</frozen-after-approval>

## Implementation Notes

Acceptance-first (CLAUDE.md BDD): add the Gherkin scenario(s) in `src/acceptance-tests/features/award-finalists.feature` with step definitions, and see them fail before touching `src/internal/web/sheet_awards.go`. The existing "Leaving some awards blank" scenario and unit test `TestPostAwardsSheetShouldAcceptAndSkipAPartiallyFilledAward` change meaning for partial awards. TDD for the unit tests.

## Review Triage Log

- false: whitespace-only slot counts as filled - `parseAwardsSubmission` trims every text/slug with `strings.TrimSpace`, so it is blank by the time `awardIsIncomplete` sees it.
- false: game-rules copy still says partial awards are skipped - grep of `docs/` and `src/internal/web/rules` finds no such wording; nothing to regenerate.
- false: gocyclo limit not checked - `task go:build` (which lints) passed.
- false: `epic-2-context.md` rewrite, hand-typed `last_updated`, tracking-state and observability remarks - generated planning artifact and sprint bookkeeping, outside the story's code; no counter or log was in scope.
- low (rejected): reload re-POSTs the 200 re-render - pre-existing pattern of the invalid-name path; fix would add a redirect/flash mechanism.
- low (rejected): extra coverage cases (1-of-3, duplicate plus partial, green check on an incomplete group) and the long doc-comment line - the 2-of-3, blank, cleared-saved and invalid-plus-partial paths are covered and share one code path.
- medium (deferred): no "other awards were saved" confirmation on the flagged re-render - UX addition beyond the story's ACs.
