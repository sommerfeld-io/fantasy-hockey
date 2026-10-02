---
title: 'Correct epic-6-context.md''s "Read-Only History" Overstatement'
type: 'docs'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `_bmad-output/implementation-artifacts/epic-6-context.md`'s Goal section (line 7) states old seasons' files "remain untouched on disk as read-only history." Nothing in the implementation makes this true at the OS level - there is no file permission change, no lock, nothing that would stop the app (or a human) from writing to an archived file. It stays untouched only as long as `DATA_FILE`/`--data-file` is never pointed back at it. If it ever is, `store.New` treats it exactly like any other existing file: it becomes the app's writable current state again - by design, since the store has no concept of "archived" vs "current," only whichever single path it's told to open (epic-6 retrospective, item 44).

**Approach:** Reword the sentence to state plainly that this is a procedural guarantee (the human never repoints the app at an old file), not an OS-enforced one, and add a sentence noting what happens if `DATA_FILE` is ever pointed back at an archived file: it becomes writable current state again, by design.

</frozen-after-approval>

## Implementation Notes

Reworded `epic-6-context.md`'s Goal section (line 7): old seasons' files "remain untouched on disk as history - procedurally, not because anything enforces it at the OS level (no permission change, no lock)," and added a sentence stating what happens if `DATA_FILE`/`--data-file` is ever repointed at an archived file: it becomes the app's writable current state again, by design, since the store has no concept of "archived" vs "current," only whichever single path it's told to open. Doc-only change, no logic touched, one sentence in one implementation-artifact file.

Verified: `task lint` exit 0 (includes `lint-markdown-links`, gocyclo, go-licenses, etc. - no markdown-structure violations introduced since no headings/lists/tables/links were touched, only prose), manually re-checked the edited paragraph against markdownlint's rules from CLAUDE.md (no lists, headings, tables, or code blocks in the edit - none of MD004/MD007/MD024/MD035/MD036/MD046 apply).

Nothing incomplete or risky.

**Review patches:** applied all five `patch`-routed blind-hunter findings — replaced the two plain-hyphen parentheticals with em dashes to match this file's own established punctuation convention (every other aside in the document uses `—`); added a lightweight cross-reference for the "no archived/current concept" claim (`internal/store`'s `store.New`, plus a pointer to Requirements & Constraints/Technical Decisions below) instead of leaving it uncited; split the single run-on addition into three shorter sentences for readability; removed the `DATA_FILE`/`--data-file` flag-name detail from the Goal section (Technical Decisions, line 26, already owns that level of detail - Goal stays at the what/why level) in favor of plain "repoints the app back at an archived file's path"; and fixed the dangling "the guarantee" antecedent by naming it explicitly ("this guarantee"). Also reconciled the Requirements & Constraints bullet (line 19, "no code path reads from or writes to an archived/previous season's file") with the Goal section's now-truthful procedural framing, adding one sentence noting the same caveat there so the two sections don't read as making claims of different strength. One finding routed to `defer`: `docs/operator-guide.md`'s rollover row has no operator-facing warning against repointing at an old path - a real, pre-existing gap this wording correction makes more visible but didn't introduce, and adding an operator warning is a different file with its own scope, better suited to its own action item (appended to `deferred-work.md`).

**Re-verified after patches**: `task lint` exit 0 again (clean across markdown-links, gocyclo, go-licenses).

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — the new text used plain hyphens with spaces for parenthetical asides (`as history - procedurally...`, `by design - the store...`) where every other aside in the file uses an em dash (lines 17, 18, 19, 25, 26, 28, 29, 33). Verified real by inspection. Fixed: switched both to em dashes.
- **low, patch** — the "store has no concept of archived vs current" claim was asserted with no citation, unlike the file's convention of backing design assertions with an AD-number or code/section pointer. Verified real. Fixed: cited `store.New` and cross-referenced Requirements & Constraints/Technical Decisions.
- **low, patch** — the added content was one run-on sentence carrying four distinct ideas. Verified real. Fixed: split into three shorter sentences.
- **low, patch** — the Goal section named the `DATA_FILE`/`--data-file` flags, duplicating detail Technical Decisions (line 26) already owns; Goal stays at the what/why level elsewhere. Verified real. Fixed: removed the flag names from Goal, kept them where Technical Decisions already states them.
- **low, patch** — "The guarantee holds only as long as..." referred back to an antecedent never named "a guarantee" in the prior sentence. Verified real. Fixed: reworded to "This guarantee holds only as long as...".
- **low, patch** — Requirements & Constraints line 19's absolute-sounding wording ("no code path reads from or writes to an archived/previous season's file") reads as a stronger, OS-enforced claim than the Goal section's now-truthful procedural framing, with no cross-reference tying the two together. Verified real. Fixed: added a sentence to line 19 reconciling the two.
- **medium, defer** — `docs/operator-guide.md`'s rollover row has no warning against repointing `DATA_FILE`/`--data-file` at an *old* path, even though the Goal section now documents that doing so silently makes the old file writable current state again. Verified real: confirmed no such warning exists at `docs/operator-guide.md:19`. Deferred: pre-existing gap this wording correction makes more visible, not one it introduced; the fix (an operator-facing warning) touches a different file with a different scope (operator-facing safety guidance vs. this item's narrow "correct the overstatement" ask) and is better tracked as its own action item.

All five patched findings were independently re-verified after patching: `task lint` clean (markdown-links, gocyclo, go-licenses), and the edited paragraphs re-read against the file's own established conventions for punctuation, citation style, and section-level detail placement.
