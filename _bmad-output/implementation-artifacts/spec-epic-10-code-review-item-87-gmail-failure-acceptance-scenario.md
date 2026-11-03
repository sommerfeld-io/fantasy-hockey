---
title: 'Gmail Failure Acceptance Scenario (invalid_grant)'
type: 'chore'
created: '2026-11-03'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `mail-transport.feature` covers successful Gmail API sends, a missing Gmail setting and the 401 retry. It has no scenario where Google itself refuses, which is the common production failure when a refresh token is revoked or expires (`invalid_grant`). That behavior is unit-tested in `internal/mailer` but has no end-to-end acceptance coverage (Epic 10 code-review action item 87).

**Approach:** Add one scenario to `src/acceptance-tests/features/mail-transport.feature`. The fake Gmail token endpoint answers `400 {"error":"invalid_grant",…}`. The pool player asks for a login code, and the scenario asserts four outcomes:

- the send fails with an error naming `invalid_grant`
- the app logs a `send login code` error with the player's id and without the email address
- the fake Gmail server receives no send request, so no code is delivered
- no SMTP connection is opened

Add the needed step definitions in `mail_transport_steps_test.go` (a token-rejection switch on the fake, a captured `slog` default restored in `close`, and a polling log assertion, since `internal/auth` logs from its send goroutine after the sender returns). No production code changes. The scenario covers behavior that already exists, so show it can fail with a temporary breakage check (mutation check). Mark item 87 `done`. Commit onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract` (user instruction: no new branch).

</frozen-after-approval>

## Implementation Notes

- `mail-transport.feature`: new scenario "A revoked Gmail refresh token fails the send, logs the error and delivers no code". It checks, in order:
    - the player still gets the code-entry page, without the error reason
    - the send error names `invalid_grant`
    - a `send login code` ERROR line is logged for `player_id=basti`, and it names `invalid_grant`
    - the email address appears nowhere in the log
    - exactly 1 token request was made
    - no Gmail send request was made
    - no SMTP connection was opened
- `mail_transport_steps_test.go`:
    - The fake token endpoint can answer `400 application/json {"error":<code>,…}` (`tokenError`, `writeOAuthError`) and counts every token request (`tokenRequests`). `received no request` now counts token attempts too.
    - `theAppStarts` captures `slog` into a buffer and `close` restores the previous logger, reusing `syncWriter` from `login_steps_test.go`.
    - The log assertion polls, because `internal/auth` logs from its send goroutine after the sender returns.
    - `POST /login`'s status and body are now kept.
- Red: before the steps existed, the suite showed 194 scenarios with 1 undefined (4 undefined steps). Green: all 194 pass, including under `go test -race`.
- Mutation checks, each restoring `internal/auth/auth.go` from a saved copy in the same command, with `git diff -- internal/` empty afterwards:
    - Removing the `send login code` log line makes the log step fail.
    - Logging `player.Email` makes "the log does not contain" fail.
    - Logging a generic error instead of `err` makes "the logged error names" fail.
- `task lint` passes (gherkin-lint and gocyclo). `task go:run` builds, and all unit and acceptance tests pass. It then exits non-zero only on govulncheck `GO-2026-6809` (`crypto/x509`, fixed in Go 1.27.3), which comes from the toolchain the protected `Dockerfile` pins at `golang:1.27.2`. Per CLAUDE.md that file is not edited; the user is asked instead.
- No production code changed. Item 87 marked `done`. Committed onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract`, as the user instructed.

## Review Triage Log

- Blind Hunter: rejected token requests were not counted, so "exactly 1 token request" could not be asserted, and "received no request" could pass falsely. Verdict: medium. Patched with `tokenRequests` and a new step.
- Blind Hunter: the log assertion never checked that the error names `invalid_grant`. Verdict: medium. Patched with the "the logged error names" step, which mutation 3 proves.
- Blind Hunter: the email check covered only the matched line. Verdict: medium. Patched to check the whole captured log.
- Blind Hunter: `player_id` was matched as a substring. Verdict: low, and the fix was simple. Patched with a trailing-space anchor.
- Blind Hunter: what the player sees was never asserted. Verdict: medium. Patched: the response is kept, and the code-entry page (200, form `action="/login/code"`) must not contain the reason.
- Blind Hunter: the fake's error used text/plain, unlike Google. Verdict: low, and the fix was simple. Patched to `application/json`.
- Blind Hunter: no nil check on `s.gmail`. Verdict: low. Rejected because the sibling steps (`rejects the first send with 401`) follow the same pattern, and Background order guarantees it.
- Blind Hunter: the spec was unfinished. Verdict: false. Finalizing sets `done`.
- Blind Hunter: no evidence of the mutation check. Verdict: false at commit time. Recorded above.
- Blind Hunter: the sprint-status `ref` still points to the Story 10.2 spec. Verdict: low. Rejected because `ref` names the item's origin by convention.
- Blind Hunter: the spec type should be `test`. Verdict: low. Rejected because the template allows only feature/bugfix/refactor/chore. The commit uses `test`.
- Blind Hunter: the global slog swap has an undocumented concurrency assumption. Verdict: low. Patched with a comment.
