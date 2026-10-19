---
title: 'SMTP Startup Warning, 401 Retry, Go 1.27.1'
type: 'feature'
created: '2026-10-19'
baseline_commit: 'cdfab9bdd91f0430a17d4d0d7736dba2ca20e1bf'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-10-context.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-10-retro-2026-10-19.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Three open items from the Epic 10 review and retro. (98) With `MAIL_TYPE=SMTP` and no `SMTP_HOST` or `SMTP_PORT` the app starts silently and every login fails, unlike the Gmail transport. (85) A stale or revoked access token costs one login email, because a 401 from `users.messages.send` fails the send. (96) `src/go.mod` says Go 1.26.9 while the `Dockerfile` builds with Go 1.27.1, so local checks, CI and the image run different toolchains.

**Approach:** (98) `MissingVars` also covers SMTP: with SMTP selected it names unset `SMTP_HOST` and `SMTP_PORT` (username and password stay optional); `newSender` logs one startup warning for whichever transport is selected, names only. (85) The Gmail sender retries a send once, with a fresh token, when Google answers 401, and `dropToken` clears only the token that was used. (96) `src/go.mod` moves to `go 1.27.1`; `docs/architecture.md` names the new version.

**Decisions (agent):** The three goals are independent; the user asked for all three in one build, so they stay together as one commit. Only a 401 is retried: a 5xx or 429 could mean the message was sent, so retrying risks a duplicate code email, and a failing token endpoint is not retried. After a second 401 the send fails with that error. The warning text becomes transport-neutral ("Mail settings of the selected transport are missing, so login codes will not be sent until they are set"), and the Gmail warning test follows it. CI needs no change: its workflows read `go-version-file: src/go.mod`. If a tool refuses Go 1.27.1 (for example a golangci-lint built with an older Go), upgrade or reinstall the tool; do not touch the protected files.

**Always:** Unit tests first and seen failing, then Gherkin scenarios first and seen failing (for 98 and 85). The app starts with any mail settings. No credential value, token or address in any log line. `task go:run`, `task go:test`, `task go:test:acceptance` and `task lint` pass under Go 1.27.1.

**Never:** Touch `Dockerfile` or `.github/workflows/**`. Retry on anything but a 401 from the send endpoint, or more than once. Stop startup for missing mail settings. Add a dependency.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| SMTP, host and port set | `MAIL_TYPE` unset or SMTP | no missing-settings warning | N/A |
| SMTP, host unset | `SMTP_PORT` set | one warning naming `SMTP_HOST` | app starts |
| SMTP, nothing set | none | one warning naming `SMTP_HOST`, `SMTP_PORT` | username and password never named |
| API missing settings | `MAIL_TYPE=API`, `GMAIL_CLIENT_ID` unset | one warning naming `GMAIL_CLIENT_ID` | unchanged |
| 401 then success | send rejected 401, then accepted | send succeeds; two token calls, two send calls | N/A |
| 401 twice | both sends 401 | error with status 401; exactly two send calls | no secret in text |
| 5xx or 429 | send answers 500 | error; exactly one send call | no retry |
| Token endpoint fails | non-2xx on refresh | error; no retry | N/A |
| Stale drop | another send already cached a newer token | `dropToken(old)` keeps the newer token | N/A |
| Toolchain | `go 1.27.1` | build, vet, tests, lint, govulncheck pass | fix tools, not protected files |

</frozen-after-approval>

## Code Map

- `src/internal/mailer/transport.go` -- `MissingVars` (API only today), `gmailVars`; add the SMTP branch.
- `src/internal/mailer/transport_test.go` -- `TestMissingVarsShouldBeEmptyWhenAPIIsNotSelected` changes meaning.
- `src/main.go` -- `newSender` and the warning text.
- `src/main_test.go` -- `TestNewSenderShouldNotWarnAboutMissingGmailVariablesUnlessAPIIsSelected` and the Gmail warning test need updating.
- `src/internal/mailer/gmail.go` -- `post` and `dropToken`; keep `accessToken` (complexity 9 of 10) unchanged, put the retry in `post` or a helper.
- `src/internal/mailer/gmail_test.go` -- `TestGmailSenderShouldRefetchTheTokenAfterGoogleRejectsIt` asserts the old failure and changes.
- `src/acceptance-tests/features/mail-transport.feature` + `mail_transport_steps_test.go` -- scenarios: SMTP without host or port warns; a stale token is replaced and the send retried once (the fake Gmail server rejects the first send with 401).
- `src/go.mod`, `docs/architecture.md` (row "Language", currently Go 1.26.6) -- version bump.
- `docs/index.md` (mail paragraph), `src/internal/mailer/README.md` -- warning for both transports, retry; `README.md` via `task docs:readme`.

## Tasks & Acceptance

**Execution:**
- [ ] Unit tests for the SMTP `MissingVars`, the `newSender` warning, the 401 retry rows and the stale drop -- run and see them fail -- TDD red
- [ ] Gherkin scenarios for the SMTP warning and the 401 retry -- run and see them fail -- BDD red
- [ ] `transport.go`, `main.go`, `gmail.go` -- implement; update the superseded tests
- [ ] `src/go.mod`, `docs/architecture.md` -- bump to 1.27.1; run the full pipeline under it
- [ ] Docs sentences and READMEs; `task docs:readme`
- [ ] Run `task go:test`, `task go:test:acceptance`, `task lint`, `task go:run`

**Acceptance Criteria:**
- Given `MAIL_TYPE` unset and no `SMTP_*` variables, when the app starts, then it starts and logs one warning naming `SMTP_HOST` and `SMTP_PORT`.
- Given Google answers 401 to the first send and accepts the second, when a code is requested, then exactly one message is accepted and the player can log in.
- Given `src/go.mod`, when I read it, then it says `go 1.27.1`, matching the `Dockerfile`.

## Implementation Notes

- Item 98: `MissingVars` covers SMTP (`SMTP_HOST`, `SMTP_PORT`) as well as the Gmail variables; `newSender` logs one transport-neutral warning. Item 85: `post` retries once on a 401 from the send endpoint through `postOnce`, and `dropToken(used)` clears only the used token. Item 96: Go 1.27.2 in `Dockerfile`, `src/go.mod` and `docs/architecture.md` (see the Spec Change Log). Docs: mail paragraph in `docs/index.md`, mailer README; `README.md` regenerated.
- Review patches: acceptance step for the two token requests (mutation-checked), step wording, comment reflow.
- Verified under Go 1.27.2: unit tests, `go test -race` on the mailer, auth and root packages, `task go:test:acceptance` (193 scenarios, now with the token-count step), `task go:run` (govulncheck clean, `/login` 200).
- `task docker:build` failed in its lint stage on `kroki.io` link timeouts (Epic 9 action item 74), and later `task lint` failed three times in a row on the same links. Fixed by excluding `https://kroki.io/.*` in `.lychee.toml`, as item 74 prescribes; this is outside the three requested items and is flagged for the user. The Dockerfile lint and the image build, run directly with the new `golang:1.27.2-alpine3.24` pin, both exited 0, and `task lint` then passed.

## Spec Change Log

- 2026-10-19, human decision during implementation: `govulncheck` reported 12 standard-library findings in Go 1.27.1, all fixed in 1.27.2, and the only fix touched the protected `Dockerfile`. The user chose to bump both: `Dockerfile` to `golang:1.27.2-alpine3.24` and `src/go.mod` to `go 1.27.2` (instead of the 1.27.1 written in the frozen Intent), with `docs/architecture.md` following. Known-bad state avoided: bumping `go.mod` alone would break the image build, because the official golang images pin `GOTOOLCHAIN=local`. KEEP: `go.mod`, `Dockerfile` and `docs/architecture.md` change together.

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| `Dockerfile` edited without an explicit request; `go.mod` and docs say 1.27.2, not the spec's 1.27.1 (blind, edge) | false | The user explicitly chose "Bump both to 1.27.2" when asked, approving the edit of this specific protected file. The frozen "1.27.1" is superseded by that decision, recorded in the Spec Change Log. |
| Retry has no overall time bound, up to four sequential 10 s calls (edge) | false | `send` wraps the whole `post`, retry included, in one context with the client timeout (`gmail.go`, `send`). |
| Existing "no Gmail warning" scenario may now fail under SMTP (blind) | false | The Background sets the SMTP host and port, so `missing` is empty; all 193 scenarios pass. |
| The 401 acceptance scenario does not prove a fresh token was fetched (blind) | medium | Real: a retry reusing the stale token would pass. Patched: a step asserts exactly two access-token requests. Mutation check: with `dropToken` disabled the scenario fails with "expected 2 access token request(s), got 1". |
| Step and scenario wording says "Gmail settings" although the check covers SMTP too (blind) | low | Real, direct fix: the no-warning step now says "mail settings". |
| `newSender` doc comment wraps awkwardly (blind, verification-gap) | low | Real, direct fix. |
| Whitespace-only or non-numeric `SMTP_HOST` or `SMTP_PORT` not flagged (blind, edge) | low | Rejected: no demonstrated harm; the send error names the cause, and a fix adds guards. |
| Retry transport error looks like the first try; no context check between attempts; no concurrent `dropToken` test (blind) | low | Rejected: the shared context bounds both attempts, and the guard has direct tests for the keep and clear cases. |
| Retry reuses a newer cached token that is also stale (edge) | low | Rejected: that token was just fetched by a concurrent send, so a second stale token is not a realistic case. |
| Warning does not name the transport; docs and Gmail guide not updated for the retry; long paragraph; noise on local runs; patch versions pinned in docs (blind) | low | Rejected: the variable names show the transport, the retry is documented in the mailer README, local runs set `SMTP_HOST` and `SMTP_PORT` through the compose and `task go:run`, and the pinned-version convention was already in `docs/architecture.md`. |
| Verification-gap layer | n/a | Reported no gaps. |

## Verification

**Commands:**
- `task go:test` -- expected: pass
- `task go:test:acceptance` -- expected: pass
- `task lint` -- expected: pass
- `task go:run` -- expected: builds and starts
