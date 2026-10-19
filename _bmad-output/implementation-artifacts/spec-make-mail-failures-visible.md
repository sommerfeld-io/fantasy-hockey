---
title: 'Make Mail Failures Visible'
type: 'feature'
created: '2026-10-19'
baseline_commit: '0819d6d88c462c66d5686ca2f1e626e209ecf78e'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-10-context.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-10-retro-2026-10-19.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** With `MAIL_TYPE=API` and a missing `GMAIL_*` variable the app starts cleanly and the first login fails silently (Epic 10 action item 86). A failed code send logs no Player id, so the operator cannot tell whose code was lost (Epic 9 action item 68).

**Approach:** (1) `internal/mailer` gets `MissingVars(Settings)`, which names the unset `GMAIL_*` variables when `MAIL_TYPE` is `API` and nothing otherwise; `newSender` in `main.go` logs one startup warning that names them, never values, and the app still starts. (2) `internal/auth` adds `player_id` to the `send login code` error line (never the email). The docs say `login_code_requested` means persisted and send attempted, not delivered.

**Decisions (agent):** The startup warning covers the Gmail transport only, as the action item asks; SMTP keeps its send-time error. No success log line (the Epic 9 item suggested one; it would sit outside the fixed audit vocabulary). No new metric. The change to `docs/index.md` and `docs/gmail-api-setup.md` is one sentence each, and `README.md` follows via `task docs:readme`.

**Always:** Unit tests first and seen failing, then Gherkin scenarios first and seen failing. The app starts with any mail settings. No credential value or email address in any log line.

**Never:** Stop startup for missing Gmail settings. Touch `Dockerfile`, `.github/workflows/**`, the dev compose or `.env`. Add a dependency, a metric, an audit event or a log line on send success.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| API, all four set | `MAIL_TYPE=API`, four `GMAIL_*` | no missing-settings warning | N/A |
| API, some missing | `MAIL_TYPE=API`, `GMAIL_CLIENT_ID` unset | one warning naming `GMAIL_CLIENT_ID`; app starts | values never logged |
| API, none set | `MAIL_TYPE=API` only | one warning naming all four, stable order | app starts |
| SMTP | any `GMAIL_*` state | no missing-settings warning | N/A |
| Failed send | known player, sender returns error | one `send login code` error line with `player_id`, no email | response still 200 |
| Successful send | known player | no new log line | N/A |

</frozen-after-approval>

## Code Map

- `src/internal/mailer/transport.go` -- `IgnoredVars` is the pattern; add `MissingVars` beside it.
- `src/internal/mailer/transport_test.go` -- unit tests first.
- `src/main.go` -- `newSender` logs the ignored-settings warning; add the missing-settings warning.
- `src/main_test.go` -- `TestNewSenderShouldWarnOnce...` is the pattern for the log assertion.
- `src/internal/auth/auth.go` -- the send goroutine's `slog.Error("send login code", "error", err)`.
- `src/internal/auth/auth_test.go` -- failing-sender test, first.
- `src/acceptance-tests/features/request-login-code.feature` -- "A mailer failure does not affect the response"; add the player id assertion (step in `login_steps_test.go`).
- `src/acceptance-tests/features/mail-transport.feature` + `mail_transport_steps_test.go` -- scenario for the startup warning; steps call the `mailer` functions as the ignored-settings steps do.
- `src/internal/observe/README.md`, `src/internal/web/README.md` -- document that `login_code_requested` is persisted and send attempted, not delivered.
- `docs/index.md`, `docs/gmail-api-setup.md` -- one sentence each on the startup warning; `README.md` via `task docs:readme`.

## Tasks & Acceptance

**Execution:**
- [ ] Unit tests for `MissingVars`, `newSender` warning and the auth log line -- run and see them fail -- TDD red
- [ ] Gherkin: player id on the failed-send error line; startup warning for missing Gmail settings -- run and see them fail -- BDD red
- [ ] `transport.go`, `main.go`, `auth.go` -- implement -- green
- [ ] READMEs and docs sentences; `task docs:readme`
- [ ] Run `task go:test`, `task go:test:acceptance`, `task lint`, `task go:run`

**Acceptance Criteria:**
- Given `MAIL_TYPE=API` with `GMAIL_CLIENT_ID` unset, when the app starts, then it starts and logs one warning naming `GMAIL_CLIENT_ID` and no value.
- Given a known player and a failing sender, when a code is requested, then the response is 200 and the error line carries the player id and not the email address.

## Implementation Notes

- Added `mailer.MissingVars` and the single `gmailVars` list, one startup warning in `newSender`, `player_id` on the `send login code` error line, README sentences (`internal/observe`, `internal/web`), the docs sentences and a Troubleshooting entry. Acceptance: player id assertion on the failed-send scenario and two scenarios in `mail-transport.feature`.
- Review patches: single variable list, subtests, comment fix, stricter warning test, Troubleshooting entry. A real data race in the new failed-send test (`captureLogs` returned an unsynchronized buffer read while the send goroutine wrote) was found by `go test -race` and fixed with a locked `syncBuffer`.
- Verified: `task go:test`, `go test -race` on `internal/...` and the root package, `task go:test:acceptance` (191 scenarios), `task lint`, `task go:run` (govulncheck clean, `/login` 200), and the built binary showed the warning and `player_id` in its output.

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| `captureLogs` doc comment glued onto `syncBuffer` (blind, edge) | low | Real, direct fix: each type and helper has its own comment. |
| Leftover `logs.Reset()`; warning test did not check the set variable names are absent (blind) | low | Real, direct fixes in `main_test.go`. |
| Table test without subtests, random failure order (blind) | low | Real, direct fix: `t.Run` per case. |
| `MissingVars` repeats the Gmail variable list kept by `checkSettings` (blind) | low | Real drift risk: a fifth `GMAIL_*` variable would need two edits. Patched: one `gmailVars` list behind the send-time check, `MissingVars` and `IgnoredVars`. |
| Troubleshooting has no entry for the startup warning (blind) | low | Real, direct docs fix. |
| Acceptance steps assert `MissingVars` output, not the logged warning; the leak step is vacuous (blind, edge) | medium | Real, but the log content is pinned by the `newSender` unit tests and the binary run. Deferred and tracked with the earlier open item on acceptance-level log capture; the fix moves logging into `internal/mailer` and adds public surface. |
| SMTP gets no startup warning for a missing host or port (blind) | low | A deliberate decision in the spec, as the action item asked for Gmail only. Tracked as a new open item. |
| Failed send could leak the email through the sender error (edge, blind) | false | Both senders redact the recipient (`mailer.go` `redactRecipient`, `gmail.go` `redact`), and the new auth test asserts no email in the line. |
| No `run()`-level test of the warning; 50 ms settle in the negative success test; `player_id=` substring match; whitespace-only Gmail values (blind, edge) | low | Rejected: `newSender` tests cover the wiring, the settle can only false-pass, and the others add guards with no demonstrated harm. |
| README missing from the diff (edge); spec left untracked, three concerns in one commit (blind) | false | The diff was scoped to `src/` and `docs/`; the spec is committed with the change as one commit per the stated convention. |
| Verification-gap layer | n/a | Reported no gaps. |

## Verification

**Commands:**
- `task go:test` -- expected: pass
- `task go:test:acceptance` -- expected: pass
- `task lint` -- expected: pass
- `task go:run` -- expected: builds and starts
