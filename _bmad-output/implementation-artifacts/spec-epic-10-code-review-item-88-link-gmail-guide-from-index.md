---
title: 'Link the Gmail API Guide from docs/index.md'
type: 'docs'
created: '2026-11-03'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `docs/index.md` names the Gmail API guide in a code span (`docs/gmail-api-setup.md`) instead of linking to it. A relative link would break in `README.md`, which is a byte-for-byte copy, and an absolute GitHub URL would have failed the link checker before the guide reached `main`. The guide is on `main` now (Epic 10 code-review action item 88).

**Approach:** Replace the code span in `docs/index.md` with a link to `https://github.com/sommerfeld-io/fantasy-hockey/blob/main/docs/gmail-api-setup.md`, the same absolute-URL pattern the License link uses. Regenerate `README.md` with `task docs:readme`. Update the planning text that said "not linked" (`epics.md` Story 10.3 and its References line, and `epic-10-context.md`) to match. Mark item 88 `done`. Commit onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract` (user instruction: no new branch).

</frozen-after-approval>

## Implementation Notes

- `docs/index.md`: the code span became "The [Gmail API setup guide](https://github.com/sommerfeld-io/fantasy-hockey/blob/main/docs/gmail-api-setup.md) (`docs/gmail-api-setup.md`) is for production only". This is the absolute-URL pattern the License link uses, and the path stays visible for readers of a local clone. `README.md` was regenerated with `task docs:readme`; `cmp` shows both files are identical.
- `docs/operator-guide.md`: the first-deploy checklist row now links the guide relatively (the operator guide is not copied to `README.md`). The checklist table was re-padded.
- `epics.md` Story 10.3: the criterion now says "linked from"; the References line keeps the reasoning (README copy, link checker before merge, item 88). `epic-10-context.md`: the bullet says it is linked by absolute URL, with the reason.
- `sprint-status.yaml`: marked item 88 `done`.
- `task lint` passes; `lint-markdown-links` reports `[200]` for the new URL in both `docs/index.md` and `README.md`. No Go code changed, so `task go:run` does not apply. This is docs-only with no behavior change, so no acceptance test applies (same decision as items 69 and 94).
- Committed onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract`, as the user instructed.

## Review Triage Log

- Blind Hunter: the spec was unfinished. Verdict: false. Finalizing sets `done`.
- Blind Hunter: the spec type should be `docs`. Verdict: low. Patched.
- Blind Hunter: `deferred-work.md` still lists the link as deferred. Verdict: low. Rejected because the workflow forbids editing existing deferred entries.
- Blind Hunter: item 88's `ref` points to the Story 10.2 spec. Verdict: low. Rejected because `ref` records where the item came from.
- Blind Hunter: no record of the link-checker result. Verdict: false. `task lint` reported `[200]` for both files, recorded above.
- Blind Hunter: no acceptance-test decision recorded. Verdict: low. Recorded above.
- Blind Hunter: the criterion named the implementation ("by its absolute GitHub URL"). Verdict: low. Patched: that detail stays in References only.
- Blind Hunter: the file path was lost, and the link always opens `main`. Verdict: low. Patched: the path is shown next to the link. The `main` link matches the existing License link convention.
- Blind Hunter: "walks through the setup step by step" was redundant. Verdict: low. Patched.
- Blind Hunter: the guide was not linked from the operator guide. Verdict: low. Patched with a relative link in the first-deploy row.
- Blind Hunter: mixed tense in `epic-10-context.md`. Verdict: low. Patched: the history was removed.
