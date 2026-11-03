---
title: 'Reconcile Story 10.3 AC: Gmail Guide Named in docs/index.md, Linked Once on main'
type: 'docs'
created: '2026-11-03'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 10.3's acceptance criterion in `epics.md` says `docs/gmail-api-setup.md` is "linked from the Configuration section of `docs/index.md`". As built, `docs/index.md` names the guide in a code span. `docs/index.md` is copied byte for byte to `README.md`, so a relative link breaks one of the two copies, and an absolute GitHub URL fails the link checker until the branch is on `main` (Epic 10 retro, finding "Story 10.3, linked from `docs/index.md`", action item 94). `epic-10-context.md` repeats the same "linked from" wording.

**Approach:** Change the Story 10.3 criterion in `epics.md`, and the matching bullet in `epic-10-context.md`, to say the guide is named in `docs/index.md`'s Configuration section and linked with its GitHub URL once it is on `main`. Cite the reason, and point to code-review item 88 as the follow-up that adds the link. Planning documents only, no change to `docs/`. Mark item 94 `done`. Commit onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract` (user instruction: no new branch).

</frozen-after-approval>

## Implementation Notes

- `epics.md` Story 10.3: the third acceptance criterion now says "named in the Configuration section of `docs/index.md`". The reason (the byte-for-byte `README.md` copy, and the link checker before merge), the retro evidence and the follow-up id `epic-10-code-review-item-88-…` went into the story's References line, so the criterion stays short and checkable.
- `epic-10-context.md`: the docs bullet now says "named, not linked", with the same reason and the item 88 id.
- `sprint-status.yaml`: marked item 94 `done`. The item is human-owned ("human applies"); the user asked for it to be implemented in this session, and that is the approval.
- Left alone on purpose: `spec-10-3-document-the-mail-settings-and-the-gmail-api-setup.md`, which is a `done` story spec and historical record (same rule as item 70). `docs/index.md` and `README.md` are also unchanged, because adding the link is item 88, not this item.
- Committed onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract`, as the user instructed.
- No code changed, so `task go:run` does not apply. `task lint` passes.

## Review Triage Log

- Blind Hunter: the guide is already on `main`, so "once on `main`" in the criterion records a temporary state. Verdict: medium. Confirmed with `git cat-file -e main:docs/gmail-api-setup.md`. Patched: the criterion is now state-neutral and points to item 88 for the link. Doing item 88 itself is out of scope, so it was offered to the user.
- Blind Hunter: the spec was unfinished while item 94 was `done`. Verdict: false. Finalizing sets `done` and fills the notes in the same commit.
- Blind Hunter: no acceptance-test decision was recorded. Verdict: low. Story 10.3's References already record that no acceptance test applies to this docs story, and this change edits planning text only.
- Blind Hunter: no record of human approval for a human-owned item. Verdict: low. Patched with a note above.
- Blind Hunter: the context bullet had no reason and no follow-up reference. Verdict: low. Patched.
- Blind Hunter: "code-review item 88" was not a lookup-able id, and the retro was not cited. Verdict: low. Patched with the full id and a retro citation.
- Blind Hunter: the rationale sat inside the Given clause. Verdict: low. Patched: moved to References.
- Blind Hunter: no note on why the original Story 10.3 spec was left unchanged. Verdict: low. Patched with a note above.
- Blind Hunter: the branch name does not mention Epic 10. Verdict: low. Rejected because the user explicitly asked for this branch; the PR description lists all items.
