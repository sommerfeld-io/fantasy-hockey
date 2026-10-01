# Sprint Change Proposal: Incomplete Player Awards Are Flagged, Not Silently Dropped

- **Date:** 2026-10-01
- **Trigger:** A new requirement from the product owner: when a Player predicts an award, all 3 finalists are required for it; skipping an award entirely stays fine.
- **Mode:** incremental (five proposals, all approved)
- **Scope classification:** Minor. One small story, three document edits and a status update; no architecture or data-model change.

## 1. Issue summary

On the Player awards sheet, filling 1 or 2 of an award's 3 finalists is silently dropped: the submit succeeds, the Player is redirected to Predict, and that award is never saved. A Player can believe a pick is in when it will score zero.

This is current, documented behavior, not a regression:

- Story 2.6's boundaries said "an award left partially or fully blank simply isn't saved and never blocks the rest".
- `awardFinalistSlugsToSave` in `src/internal/web/sheet_awards.go` keeps only awards with all 3 slots resolved.
- `TestPostAwardsSheetShouldAcceptAndSkipAPartiallyFilledAward` and UX Flow 2 ("whatever he left blank simply isn't blocking him") encode it.
- It contradicts Story 2.6's own purpose: "a typo can never silently fail to score".

The decision (with the product owner): an award left entirely blank is still fine (FR-11), but an award with any finalist filled needs all 3, and an incomplete one is flagged with an inline error. Fully filled awards in the same submit are still saved; a typed name that does not match a suggestion, or a repeated name, keeps today's rule of rejecting the whole submit.

Evidence: `src/internal/web/sheet_awards.go` (`awardSlotInvalid`: "a blank Text with no slug is never invalid"; `awardFinalistSlugsToSave`), `src/internal/web/sheet_awards_test.go:368-386`, `src/acceptance-tests/features/award-finalists.feature` ("Leaving some awards blank saves only the complete ones"), `epics.md` Story 2.6, `prd.md` FR-11 and FR-17, `EXPERIENCE.md` lines 69, 90 and 125-128.

## 2. Impact analysis

- **Epic impact:** Epic 2 is `done` and gains one story (2.7), so it moves back to `in-progress`. Epics 3 to 9 are unaffected: awards are their own prediction set and nothing downstream reads partial awards, because they were never persisted.
- **Story impact:** Story 2.6 stays `done` and gets a pointer line. New Story 2.7 owns the change.
- **Artifact conflicts:**
    - PRD FR-17 is silent on 1 or 2 finalists (needs one consequence bullet).
    - UX `EXPERIENCE.md` defines "name rejected" but not "incomplete award".
    - `docs/game-rules.md` (and its embedded copy) says "3 finalists each" without the all-or-none rule.
    - Architecture: no change. AD-10 already requires server-side re-validation; no schema, API or layering effect.
- **Technical impact:**
    - `sheet_awards.go`: detect an incomplete award, save the complete ones, then re-render the sheet (200) with the caption on the empty slots.
    - Tests: the unit test above changes meaning; new unit and acceptance scenarios.
    - Observability: nothing new. Saved awards still audit and count one row each (9.3/9.4); an incomplete award saves nothing, so it emits nothing.

## 3. Recommended approach

Direct adjustment (one new story). Effort low, risk low.

- A rollback is not warranted: nothing completed needs reverting, only a behavior is being tightened.
- No MVP change: the five awards and 3 finalists per award stay as specified.
- Rejected alternative: blocking the whole submit on an incomplete award. It would match the existing all-or-nothing rule for invalid names, but the product owner chose to save the complete awards, so a Player does not lose work for one unfinished award.

## 4. Detailed change proposals

### Proposal 1: `epics.md`, add Story 2.7 and a pointer in Story 2.6

Story 2.7: Incomplete Award Picks Are Flagged.

```
As a player,
I want a clear error when I fill only some of an award's 3 finalists,
So that a half-entered award can never silently fail to save.

Given the "Player awards" set is open
When I submit with an award left entirely blank
Then that award is simply not saved and shows no error (FR-11), and my other awards save

Given I fill 1 or 2 of an award's 3 finalists with valid names
When I submit
Then that award is not saved, its empty slots show an inline caption
  "Pick all 3 finalists for this award, or clear it." and the sheet re-renders (200) with my input kept
And every other fully filled award in the same submit is saved

Given any award has a typed name that does not match a suggestion, or a repeated name
When I submit
Then the existing rule is unchanged: the whole submit is rejected and nothing is saved

Given an award I have already saved
When I clear one of its slots and submit
Then it is flagged as incomplete and its saved picks are left as they were
```

Story 2.6 gains: "Partial awards: superseded by Story 2.7." Rationale: 2.6's partial-skip boundary conflicts with its own goal.

### Proposal 2: `prd.md` FR-17, one consequence bullet

Add: "A Player may leave an award entirely blank (it scores zero, FR-11). If any of an award's 3 finalists is filled, all 3 are required: an incomplete award is rejected with an inline error naming the problem, and any fully filled awards in the same submission are still saved." FR-11 is unchanged.

### Proposal 3: `EXPERIENCE.md`, Player awards pattern and a new state row

- Component Patterns, "Player awards" row: append that a group with 1 or 2 finalists filled is incomplete, its empty slots show the caption "Pick all 3 finalists for this award, or clear it." and it is not saved; an entirely blank group is fine.
- State Patterns: new row "[ASSUMPTION] Player awards: incomplete award", reusing the `goal`-colored border and inline caption treatment of a rejected name, no group check, complete awards in the same submit saved. Marked as an assumption to confirm because the click-dummy does not define it.

### Proposal 4: `docs/game-rules.md` line 45

Append to the Player awards bullet: "You can skip an award entirely, but if you pick any finalist for an award you must pick all 3 for it." The embedded copy at `src/internal/web/rules/game-rules.md` is regenerated by `task docs:embed-game-rules`; the drift test requires them to match.

### Proposal 5: `sprint-status.yaml`

Set `epic-2: in-progress` and add `2-7-incomplete-award-picks-are-flagged: backlog` after `2-6-player-awards-finalists`.

## 5. Implementation handoff

- **Minor scope: Developer agent**, via `bmad-build` on Story 2.7.
    - Write the Gherkin scenarios in `src/acceptance-tests/features/award-finalists.feature` first and see them fail: a partly filled award is flagged while a complete one saves; clearing a slot of a saved award leaves its saved picks as they were. Keep "Leaving some awards blank saves only the complete ones" for fully blank awards.
    - Replace `TestPostAwardsSheetShouldAcceptAndSkipAPartiallyFilledAward` with tests for the new behavior, including "should not": an entirely blank award shows no error, and a typed-but-unresolved name still rejects the whole submit.
    - Run `task docs:embed-game-rules`, then `task go:run`, then `task docker:build`.
- **Product owner:** confirm the error caption wording and the "clearing a slot leaves saved picks as they were" rule when reviewing the story.
- **Success criteria:** a partly filled award never redirects as if saved; the caption appears on its empty slots with the input kept; complete awards in the same submit are saved and counted; the game rules page and the PRD describe the same rule.
