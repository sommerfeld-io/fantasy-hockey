---
title: 'Choose the Mail Transport With MAIL_TYPE'
type: 'feature'
created: '2026-10-19'
baseline_commit: 'dfe5b3a27454a38fc69192208de2049b49e88d4f'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-10-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The app can only send mail over SMTP, and `main.go` builds that sender directly. The operator cannot choose another transport, and nothing validates mail settings at startup.

**Approach:** Add `MAIL_TYPE` (`SMTP` or `API`, case-insensitive, unset or empty means `SMTP`). A new constructor in `internal/mailer` takes the settings and returns the existing `Sender`. An invalid value stops startup. Settings of the unselected transport are ignored with one startup warning that names the variables. `main.go` reads the environment once and holds no transport logic.

**Decisions (agent, not user-visible enough to ask):** The real Gmail sender is Story 10.2. Until then the `API` branch returns a sender that fails with a clear "not implemented yet" error and never touches SMTP. The Gherkin scenario "`API` sends through the fake Gmail server" therefore moves to Story 10.2, where the fake Gmail server exists; Story 10.1's feature covers default SMTP, explicit `SMTP`, `API` opening no SMTP connection and warning about ignored `SMTP_*`, and an invalid value. Story 9.6 decided on no drift test for the example compose, so none exists to keep passing.

**Always:** Tests first and seen failing: unit tests, then the Gherkin feature. App still starts with no mail settings. No secret value in any log line.

**Never:** Touch `Dockerfile`, `.github/workflows/**`, the dev compose or the repo `.env`. Add any `GMAIL_*` to dev files. Fall back to SMTP on an invalid value. Add a dependency. Write docs/index.md or the setup guide (Story 10.3).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Default | `MAIL_TYPE` unset or empty | SMTP sender | N/A |
| Explicit SMTP | `smtp`, `SMTP`, `Smtp` | SMTP sender, Gmail never called | N/A |
| API | `api`, `API` | API sender, no SMTP connection | N/A |
| Invalid | `pigeon` | startup error naming `MAIL_TYPE`, `SMTP`, `API` | no fallback |
| Ignored settings | API + `SMTP_HOST`; SMTP + any `GMAIL_*` | one warning listing variable names | values never logged |
| No settings | nothing set | app starts, SMTP send errors as today | N/A |

</frozen-after-approval>

## Code Map

- `src/internal/mailer/mailer.go` -- `Sender`, `NewSMTPSender`; reuse as-is for the SMTP branch.
- `src/internal/mailer/transport.go` -- new: `Settings`, env-name constants, `SettingsFromEnv`, `New`, `IgnoredVars`.
- `src/internal/mailer/transport_test.go` -- new: unit tests, written first.
- `src/main.go` -- lines 99-104 build the SMTP sender; replace with `SettingsFromEnv` + `New` + one warning.
- `src/acceptance-tests/suite_test.go` -- register the new scenario initializer.
- `src/acceptance-tests/mail_transport_steps_test.go` -- new: steps with a tiny fake SMTP server; pattern from `login_steps_test.go`.
- `src/acceptance-tests/features/mail-transport.feature` -- new, written first.
- `docs/examples/docker-compose.yml` -- add `MAIL_TYPE` and four `GMAIL_*` empty placeholders with a secret-store comment.
- `src/internal/mailer/README.md` -- document the constructor.

## Tasks & Acceptance

**Execution:**
- [ ] `src/internal/mailer/transport_test.go` -- write unit tests for the matrix, run, see them fail -- TDD red
- [ ] `src/acceptance-tests/features/mail-transport.feature` + steps + suite registration -- write, run, see them fail -- BDD red
- [ ] `src/internal/mailer/transport.go` -- implement minimal selection; API branch is the not-implemented sender -- green
- [ ] `src/main.go` -- wire it, log the ignored-settings warning -- keeps main free of transport logic
- [ ] `docs/examples/docker-compose.yml` and `src/internal/mailer/README.md` -- placeholders and package doc
- [ ] Run `task go:test`, `task go:test:acceptance`, `task lint`, `task go:run`

**Acceptance Criteria:**
- Given the dev compose and `.env`, when I diff them, then they are unchanged.
- Given `MAIL_TYPE=API` with `SMTP_HOST` set, when the app starts, then one warning names `SMTP_HOST` and not its value.

## Implementation Notes

- Added `src/internal/mailer/transport.go` (`Settings`, `SettingsFromEnv`, `New`, `IgnoredVars`; API branch is a not-implemented stub for Story 10.2), `newSender` in `src/main.go`, unit tests in `transport_test.go` and `main_test.go`, `mail-transport.feature` with steps and suite registration, example-compose placeholders and the mailer README.
- Bumped `src/go.mod` from `go 1.26.6` to `go 1.26.9` to clear 10 standard-library `govulncheck` findings (all fixed in 1.26.9). Outside the spec's file list but required by the repo's verification rule; `Dockerfile` (already on Go 1.27.1) untouched.
- Review patches applied (run() test, whitespace cases, temp-dir cleanup, non-blocking sendDone).
- Verified: `task go:test` pass, `task go:test:acceptance` 187 scenarios pass, `task lint` pass, `task go:run` builds and serves `/login` with 200, `govulncheck` clean.

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| run() wiring of newSender not verified by any test (verification-gap) | medium | Real: reverting run() to build SMTP directly passed every test. Patched: `TestRunShouldStopStartupOnAnInvalidMailType`-style test in `main_test.go` calls `run()` with `MAIL_TYPE=pigeon`. Mutation check: with the error dropped, the test hangs on the real server instead of passing. |
| Acceptance "warned" steps assert `IgnoredVars`, not log output (blind, edge, verification-gap) | low | Real but covered: `newSender` unit tests in `main_test.go` assert the single WARN line and no values. Deferred: acceptance-level log capture. |
| Whitespace in `MAIL_TYPE` untested; blank means SMTP (blind) | low | Real gap, direct fix: added `"   "`, `" smtp "` and `" api "` unit cases. Blank equals unset by spec, so no contradiction with "no fallback". |
| Temp data dir never removed in acceptance steps (blind, edge, verification-gap) | low | Real. Patched: `close()` removes the dir. |
| `sendDone` buffer 1 can block a second send (edge) | low | Real but unreachable today (one send per scenario). Patched with a non-blocking send, one line. |
| "No connection" step has no settle period (blind) | false | `thePoolPlayerAsksForALoginCode` waits on `sendDone` before any assertion, so the async send has run. |
| `go.mod` bump unexplained, Dockerfile compatibility (blind, verification-gap) | false | Documented in Implementation Notes: 10 stdlib govulncheck findings fixed in 1.26.9; Dockerfile already uses Go 1.27.1. |
| Example compose lists GMAIL secrets as inline entries (blind) | low | Rejected: empty placeholders plus a comment that real values come from the secret store; fix would add structure. |
| API stub fails silently at send time (blind, edge, verification-gap) | low | Intended and recorded in the spec: stub until Story 10.2 in the same branch. Rejected. |
| Missing GMAIL_* validation at startup for API (edge) | false | FR-44: missing credentials must not stop startup; the send error comes in Story 10.2. |
| Docs/PRD/README env tables not updated (blind) | false | Story 10.3 owns the docs. |
| Invalid `MAIL_TYPE` echoed into the error; whitespace-only vars counted as ignored; `Settings` has no redaction; RCPT parsing; listener close; struct copy; missing extra scenarios (blind, edge) | low | Rejected: contrived or cosmetic, and each fix adds guards or surface for no demonstrated harm. |
| `sendWaitTimeout`, `testSessionSecret` undefined (blind) | false | Defined in `login_steps_test.go` and `suite_test.go`. |

## Verification

**Commands:**
- `task go:test` -- expected: pass
- `task go:test:acceptance` -- expected: pass
- `task lint` -- expected: pass
- `task go:run` -- expected: builds and starts
