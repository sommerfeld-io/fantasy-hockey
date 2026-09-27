# Epic 7 Context: Data File Hygiene

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

This epic keeps the app's single hand-maintained data file observable, lean, and lint-clean: every write that changes it is logged, login codes that can no longer be used are cleaned up, and the file's own format always satisfies the repo's yamllint rules. Unlike the app's other epics, this one is sourced from a dedicated brainstorming session rather than a PRD requirement — no functional requirement is covered by it, and each story's need for a Gherkin acceptance test should be confirmed with the human rather than assumed, since this may be infra/observability work rather than user-facing behavior.

## Stories

- Story 7.1: Log Data-Changing Writes
- Story 7.2: Clean Up Unusable Login Codes
- Story 7.3: Keep Written YAML yamllint-Compliant
- Story 7.4: Hand-Edited Results Are Safe to Edit

## Requirements & Constraints

- Login-code validation must still reject a wrong, expired, or already-used code with an identical outcome in all cases — this must hold even after a cleanup pass has physically removed the row for an expired/used code.
- Login code lifetime/hashing behavior is unchanged; only cleanup timing is new.
- Logging must never emit a raw login code or a player's email address — only ids/hashes — matching the existing rule; a request that results in no write at all must produce no log line.
- YAML output must satisfy the repo's actual `.yamllint` config (plain, non-strict) as verified by the real `yamllint` binary against real written output, not a hand-rolled reimplementation of its rules; `yamllint` itself must never run inside the shipped, running app.
- A malformed or wrongly-shaped hand-edited entry in a human-maintained section must never be silently accepted or silently drop the human's input: the app still starts, logs a warning naming the offending entry, scores it as 0, and reports one startup summary line with a count of problems found (including zero).

## Technical Decisions

- All four stories build exclusively on the existing mutex-guarded, atomic write-and-rename path that is the store's sole write mechanism — none introduces a second lock, a new write mechanism, a background goroutine, or a cron/schedule.
- Login-code cleanup triggers opportunistically (on the next write, or at store bootstrap), not via an on-demand CLI flag (parked as a fallback) or a scheduled job.
- Removing login-code rows shifts slice indices/length; any existing test that asserts on a login-code collection by numeric index needs re-checking.
- Human-maintained sections of the data file (results, award finalists, and the other sections the app never writes) are loaded once at startup and must be carried over byte-faithfully on every whole-file rewrite — comments, flow/block style, quoting, key order, and unknown keys all preserved exactly. This is distinct from lint-cleanliness of the app's own newly-written values, which is a separate concern.
- Confirmed empirically: the YAML library's default marshal output already produces no yamllint error-level violations under the repo's non-strict config; a pre-existing document-start warning is accepted as out of scope. A temporary yamllint ignore-list entry that was added for the data file should be removed once compliant writes are verified.
- Structured log lines follow the app's existing logging convention; a bootstrap-time write (creating a brand-new file) must be worded distinctly from a normal in-life write.
- Story 7.4's scope traces to findings surfaced in an earlier epic's retrospective/hardening follow-through (malformed keys, wrongly-shaped entries, missing startup summary, rewrite-shape drift) — this epic is where that follow-through is expected to land; detecting a hand edit made while the app is already running remains explicitly out of scope (the operator's existing stop-edit-restart workflow stands).

## Cross-Story Dependencies

- Story 7.3 owns making the app's own newly-written values lint-clean; Story 7.4 owns preserving the exact formatting of hand-maintained sections on rewrite — coordinate between them rather than duplicating either concern.
- Story 7.2's login-code cleanup must not break the identical-rejection-outcome guarantee that another epic's login-code story already established.
