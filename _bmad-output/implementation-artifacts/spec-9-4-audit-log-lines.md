---
title: 'Audit Log Lines for Logins and Prediction Saves'
type: 'feature'
created: '2026-10-01'
baseline_commit: '886abc095c98a18c022bf69ce13e06f1f8c063b6'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-9-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 9.3's call sites already write the `audit` lines, but nothing verifies them, and `internal/mailer` returns SMTP errors that can echo the recipient address into the `slog.Error` line.

**Approach:** Pin the audit lines with tests (one line per event, `player_id` only, no email or code, no line for unknown email, logout without session or rejected save, one line per persisted row), and make `internal/mailer` strip the recipient address from the errors it returns.

## Boundaries & Constraints

**Always:** Follow AD-33 and AD-35. Each line is `slog.Info("audit", "event", …)` with `player_id` when known; saves add `kind` and `set`. The line's own timestamp is the "when". The store's `store write` line stays unchanged. Failures stay `slog.Error`.

**Decisions (human):** Test the audit lines with a Gherkin feature in `src/acceptance-tests/features/` (steps capture `slog` output) plus unit tests; the feature is written first and red before any production change.

**Never:** Add a second emitter or new events. Put an email address or raw login code in any log field. Touch `Dockerfile`, `.github/workflows/**`, the proxy or compose files.

## I/O & Edge-Case Matrix

| Scenario              | Input / State                                  | Expected Output / Behavior                                         | Error Handling                      |
|-----------------------|------------------------------------------------|--------------------------------------------------------------------|-------------------------------------|
| Code requested        | `POST /login`, known email                     | One `login_code_requested` line with `player_id`                   | N/A                                 |
| Unknown email         | `POST /login`, no matching Player              | No audit line                                                      | N/A                                 |
| Login succeeded       | Correct code                                   | One `login_succeeded` line with `player_id`                        | N/A                                 |
| Login failed          | Wrong code                                     | One `login_failed` line, no `player_id`                            | N/A                                 |
| Logout                | `POST /logout`, valid session                  | One `logout` line with `player_id`                                 | N/A                                 |
| Logout, no session    | `POST /logout`, no valid cookie                | No audit line                                                      | N/A                                 |
| Save                  | Valid pick                                     | One `prediction_saved` line: `player_id`, `kind`, `set`            | N/A                                 |
| Multi-row save        | Save persisting N rows                         | N lines, one per row                                               | N/A                                 |
| Rejected save         | Deadline passed                                | No `prediction_saved` line                                         | 403 as before                       |
| No PII                | Any of the above                               | No email address or login code in any line                         | N/A                                 |
| SMTP send fails       | Server error echoing the recipient             | `slog.Error` line without the recipient address                    | Error still returned and logged     |

</frozen-after-approval>

## Code Map

- `src/internal/observe/observe.go` -- `Audit` (done in 9.2/9.3); no production change expected beyond what tests reveal.
- `src/internal/web/login.go`, `sheet.go`, `sheet_divisions.go`, `sheet_awards.go`, `sheet_series.go` -- the call sites (done in 9.3); tests only.
- `src/internal/mailer/mailer.go:51` -- wraps the SMTP error with `%w`; it does not remove the recipient. Strip `to` from the returned error text, keep the cause usable for `errors.Is`.
- `src/internal/auth/auth.go:56` -- logs the send error; no change if the mailer strips.
- `src/internal/web/counters_test.go` -- has `quietAuditLogs`; add a log-capturing helper beside it.
- `src/acceptance-tests/login_steps_test.go:164` -- precedent for capturing `slog` via `slog.SetDefault`.
- `src/internal/mailer/mailer_test.go` -- `TestNewSMTPSenderShouldWrapAnUnderlyingSendError` is the model for the new strip test.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` -- status sync.

## Tasks & Acceptance

**Execution:**
- [x] `src/acceptance-tests/features/audit-log-lines.feature` + steps + suite wiring -- one scenario per matrix row, log capture via `slog.SetDefault`; red before any production change -- BDD per project rules
- [x] `src/internal/web/*_test.go` -- unit tests for the same rows where cheaper (multi-row save, rejected save)
- [x] `src/internal/mailer/mailer.go` -- strip the recipient from returned errors -- AD-33
- [x] `src/internal/mailer/mailer_test.go` -- a send error that echoes the recipient comes back without it
- [x] `src/internal/observe/README.md`, `sprint-status.yaml` -- sync

**Acceptance Criteria:**
- Given a login code is requested, a login succeeds, a login fails, or a Player logs out, when I read the log, then exactly one line records that event and names the Player by id where known.
- Given a prediction is saved, when I read the log, then one line names the Player id, kind and set id; a rejected save leaves no such line.
- Given any audit line, then it contains no email address and no raw login code.
- Given a login code for an unknown email, or a logout without a valid session, then no audit line appears.
- Given a save persisting several rows, then there is one line per row.
- Given the SMTP send fails with a server reply echoing the recipient, then the logged error does not contain the address.

## Implementation Notes

- Audit call sites shipped in 9.3, so the new acceptance feature and web tests passed on first run; the mailer strip test was red first. Recipient redaction is case-insensitive and skips an empty recipient.
- Multi-row save and SMTP failure rows are covered by unit tests (`audit_log_test.go`, `mailer_test.go`), not by the feature.

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| Empty recipient makes the redaction pattern match between every character | medium | Verified: `(?i)` matches empty; a Player with an empty email could reach `to == ""`. Patched with a guard and a test. |
| Acceptance `post` discards the HTTP status, so "no audit line" scenarios could pass on a wrong route; the 403 is only unit tested (3 layers) | medium | Verified in `audit_log_lines_steps_test.go`. Patched: status recorded, new step asserts 200, 302 and 403 in the three no-line scenarios. |
| `requireOneLine` matches by substring (`player_id=basti` vs `bastian`, `set=cup` vs `cup2`) | low | Real but fixtures are fixed; direct fix applied (whole-token match via `hasToken`). |
| Failed-send log line through `auth` untested | low | `auth.go` passes only `err` to `slog.Error`; a fake sender in an auth test would not run the mailer redaction, so such a test proves nothing. The mailer test covers the text slog prints. Rejected. |
| Redaction covers only the exact recipient, not local part, display-name or encoded forms, or `from`/username | low | AD-33 requires stripping the recipient; case-insensitive literal match covers net/smtp echoes. Rejected. |
| Regexp compiled per error | low | Error path only; no functional gap. Rejected. |
| Six-digit code could match digits in a log timestamp | low | Chance far below 1e-4 per run; fix is a handler redesign. Rejected. |
| Global `slog.SetDefault` swap is order or parallel sensitive | low | godog runs scenarios serially here; same pattern as `login_steps_test.go`. Rejected. |
| Audit emit may be async, so "no line" checks race | false | `Audit` is called synchronously in the handler before the response; only the SMTP send is async. |
| Multi-row coverage only for awards; rejected save only for cup; hardcoded 5; other login-failure shapes | low | Per-kind counting is covered by 9.3 tests; the fixture count is the point of the test. Rejected. |
| No timestamp assertion; README placement; duplicated capture helpers | low | `slog` supplies `time=` itself; cosmetic. Rejected. |
| Audit production code missing from diff, no red-first for audit lines | false | Call sites shipped in 9.3 (`a69e5f3`); noted in Implementation Notes. |

## Verification

**Commands:**
- `task go:test` -- expected: unit tests pass
- `task go:test:acceptance` -- expected: acceptance tests pass
- `task lint` -- expected: all linters pass
- `task go:run` -- expected: app builds and starts
- `task docker:build` -- expected: image builds and lints (feature complete)
