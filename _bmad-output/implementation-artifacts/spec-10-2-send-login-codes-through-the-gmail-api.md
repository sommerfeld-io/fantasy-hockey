---
title: 'Send Login Codes Through the Gmail API'
type: 'feature'
created: '2026-10-19'
baseline_commit: '4c8c339c379fe63059d2b5e00bde53bb0fc1dc04'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-10-context.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-10-1-choose-the-mail-transport-with-mail-type.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `MAIL_TYPE=API` selects a stub that fails every send. Login codes cannot go out through the Gmail API.

**Approach:** Replace the stub with a Gmail sender in `internal/mailer`: exchange the refresh token for an access token, then POST the message to `users.messages.send` over HTTPS. Stdlib only. Settings are `GMAIL_CLIENT_ID`, `GMAIL_CLIENT_SECRET`, `GMAIL_REFRESH_TOKEN`, `GMAIL_SENDER` (OAuth 2.0, scope `gmail.send`; an API key cannot send mail).

**Decisions (agent):** `Settings` gets two code-only fields, `GmailTokenURL` and `GmailSendURL`, never read from the environment; empty means Google's real URLs, which live in constants. Tests and the acceptance steps set them to `httptest` servers and still go through `mailer.New`. The acceptance scenario for 10.1's moved item ("API sends through the fake Gmail server") is added here. No real credential is needed to build or test; the user's OAuth client ID, secret and refresh token go only into the production secret store.

**Always:** Tests first and seen failing: unit tests, then Gherkin scenarios. App still starts with no Gmail settings. A send with a missing setting returns an error naming the first missing variable (order: client ID, client secret, refresh token, sender) and makes no HTTP request. HTTP client timeout of 10 seconds. No recipient, client secret, refresh token or access token in any error, log line or metric, even if Google echoes them.

**Never:** Touch `Dockerfile`, `.github/workflows/**`, the dev compose, the repo `.env` or docs (Story 10.3). Add a dependency. Read the URLs from the environment. Call Google in any test. Add a metric label or log a new event.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Happy path | all four settings, fake Google | one token call, one send call with Bearer header and JSON `raw` | N/A |
| Message | decode `raw` | From (sender), To, Subject, plain-text body, CRLF | CR or LF in to/subject rejected, no request |
| Token reuse | second send before expiry | no second token call | N/A |
| Token expiry | send after `expires_in` elapsed | new token call | N/A |
| Missing setting | any of four empty | error naming first missing variable | no HTTP request |
| Token endpoint fails | non-2xx or transport error | error with status and Google's error reason | no secret in text |
| Send endpoint fails | non-2xx | error with status and reason | recipient and tokens redacted |
| Hung server | no response | error after timeout | N/A |
| Echoing server | body repeats recipient and secrets | error text contains none of them | redacted |

</frozen-after-approval>

## Code Map

- `src/internal/mailer/transport.go` -- `New` API branch returns the stub (`errAPINotImplemented`); replace with the Gmail sender; add the two URL fields to `Settings`.
- `src/internal/mailer/gmail.go` -- new: constants for Google's URLs and scope, `newGmailSender`, token cache, message build, error/reason parsing.
- `src/internal/mailer/gmail_test.go` -- new, written first; `httptest` servers.
- `src/internal/mailer/mailer.go` -- `redactRecipient` and `redactedError`; reuse for the recipient, extend or add a helper for the secrets and tokens.
- `src/internal/mailer/transport_test.go` -- lines ~50-67 and the whitespace test assert "not implemented yet"; update to the missing-variable error.
- `src/acceptance-tests/features/mail-transport.feature` + `mail_transport_steps_test.go` -- add the fake Gmail server and scenarios: API sends exactly one request addressed to the player, its decoded body holds a 6-digit code that logs the player in (`POST /login/code`), the fake SMTP server sees nothing; a missing setting produces an error and no request.
- `src/internal/auth/auth.go` -- login-code body and subject; read-only, not changed.
- `src/internal/mailer/README.md` -- document the Gmail sender.

## Tasks & Acceptance

**Execution:**
- [ ] `src/internal/mailer/gmail_test.go` -- unit tests for every matrix row; run and see them fail -- TDD red
- [ ] `src/acceptance-tests/features/mail-transport.feature` + steps -- scenarios and fake Gmail server; run and see them fail -- BDD red
- [ ] `src/internal/mailer/gmail.go`, `transport.go`, `mailer.go` -- implement; remove the stub -- green
- [ ] `src/internal/mailer/transport_test.go` -- update stub assertions
- [ ] `src/internal/mailer/README.md` -- document
- [ ] Run `task go:test`, `task go:test:acceptance`, `task lint`, `task go:run`

**Acceptance Criteria:**
- Given `MAIL_TYPE=API` and a fake Gmail server, when a player requests a login code, then the fake received exactly one send request for that player and the decoded code logs them in.
- Given any error from this path, when I search it for the recipient, secrets or tokens, then none appears.

## Implementation Notes

- Added `src/internal/mailer/gmail.go` (token exchange with `gmail.send` scope, token cache with 30 s margin, `users.messages.send` with base64url `raw`, 10 s timeout, redaction of recipient, secret, refresh token and access token), `Settings.GmailTokenURL`/`GmailSendURL` (code-only), unit tests in `gmail_test.go`, two acceptance scenarios with a fake Gmail server, README text. The stub is gone.
- Review patches: default-URL test via `http.DefaultTransport`, `scope` assertion, single timeout source.
- Verified: `task go:test`, `task go:test:acceptance` (189 scenarios), `go test -race` on mailer and acceptance, `task lint`, `task go:run` (govulncheck clean, serves `/login`). Not run against real Google.

### Review Findings

Code review (2026-10-19) of stories 10.2 and 10.3 by four layers (blind hunter, edge case hunter, verification gap, acceptance auditor), full mode.

- [x] [Review][Patch] Redaction ran after truncation, so a secret cut by the limit left a prefix in the error [src/internal/mailer/gmail.go] -- fixed: redact first, then cap the whole error text; test added
- [x] [Review][Patch] An error reason of null or empty left the error undiagnosable [src/internal/mailer/gmail.go] -- fixed: fall back to the raw body; test added
- [x] [Review][Patch] The acceptance fake accepted any token request and any send, so a missing bearer token or wrong grant would pass [src/acceptance-tests/mail_transport_steps_test.go] -- fixed: the fake checks grant_type, credentials, bearer token and decodes strictly
- [x] [Review][Patch] Untested branches: token response without access_token, googleReason shapes, CR/LF in GMAIL_SENDER, error text cap [src/internal/mailer/gmail_test.go] -- fixed: tests added
- [ ] [Review][Defer] Decide whether to retry once with a fresh token after a 401; dropToken should clear only the token that was used [src/internal/mailer/gmail.go] -- deferred: new behavior beyond the spec, one lost login email per stale token
- [ ] [Review][Defer] Decide whether to log a startup warning that names missing GMAIL_* variables when MAIL_TYPE=API -- deferred: new behavior, the spec says a send fails with the named variable
- [ ] [Review][Defer] Add a Gmail failure acceptance scenario (for example invalid_grant) -- deferred: unit tests cover the failure paths
- [x] [Review][Defer] Non-ASCII subject, body and address encoding -- deferred: already in deferred-work.md from the Story 10.2 build review
- [ ] [Review][Defer] Lower priority hardening: single recipient validation, whitespace-only settings, expires_in clamp, one in-flight token refresh, strings.NewReplacer instead of a regexp per needle, inject the HTTP transport instead of swapping http.DefaultTransport -- deferred: no demonstrated harm

#### Rejected

- Data races in tests and the acceptance steps (blind, edge): false, `go test -race` is clean.
- `redactedError` undefined in the diff (blind): false, it is defined in `mailer.go`.
- `README.md` and the example compose missing from the diff (acceptance auditor): false, both were committed by earlier stories and the diff was scoped to `src/` and `docs/` of the last two commits.
- `redactRecipient` not reused (acceptance auditor): low, `redact` has the same case-insensitive semantics for the recipient and also covers the secrets.
- Client ID not redacted, access token redacted only after issuance, `redactedError` unwraps to the unredacted cause, mutex held during the token call, 10 s covers token plus send (blind, edge): low, accepted limits with no demonstrated harm, the first two are not secrets or are Google-issued.
- Expiry of 0 or under the margin, missing global timeout wiring test (blind, edge, verification gap): low, worst case is an extra token request.

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| Default Google URLs in `newGmailSender` never exercised; the constant-only test is tautological (verification-gap, blind) | medium | Real: deleting the fallback passed every test, and production sets neither URL. Patched: `TestGmailSenderShouldSendToGoogleWhenNoURLsAreSet` swaps `http.DefaultTransport` and asserts both Google URLs. Mutation check: with the fallback removed the test fails. Tautological tests removed. |
| Token request `scope` never asserted (verification-gap) | medium | Real: removing the `scope` line passed all tests. Patched: added to the token-form assertions. |
| Two timeouts (constant for the context, parameter for the client) can drift (blind, edge, verification-gap) | low | Real, direct fix: the send context now uses `g.client.Timeout`. |
| Non-ASCII subject, body, From/To encoding unverified; no Content-Transfer-Encoding (blind, edge, verification-gap) | low | Real but dormant: the login-mail subject and body are ASCII. Deferred. |
| Data race in unit tests reading `tokenCalls`/`sendCalls`; `sendErr` unsynchronised in the acceptance steps (blind, edge) | false | `go test -race` on `internal/mailer` and `acceptance-tests` is clean; reads happen after the synchronous send returns or after `sendDone`. |
| No retry after a 401 or transient 5xx; concurrent 401 can drop a fresh token (blind, edge) | low | A failed send loses one login email, and the player asks again. Retry is new behavior the spec did not ask for and adds branches. Rejected. |
| `expires_in` zero, missing or huge (blind, edge) | low | Worst case is a token refetch on every send. Rejected: a clamp adds a guard for a harmless case. |
| Recipient or settings are not validated as addresses or trimmed (edge) | low | Recipients come from the operator-edited data file; the failure surfaces from Google as an error. Rejected. |
| Redaction compiles a regexp per needle; short secrets mangle unrelated text; sender and client ID not redacted (blind, edge) | low | Error path only; the spec lists the recipient, client secret, refresh token and access token, and all four are redacted. Rejected. |
| `redactedError` still unwraps to the unredacted cause (verification-gap, edge) | low | Same pattern as the existing SMTP sender, callers log `Error()`. Rejected. |
| Timeout wiring through `New` not pinned (verification-gap) | low | The send context deadline backstops it, now tied to the client timeout. Rejected. |
| Acceptance fake ignores decode errors and form fields; nil `s.gmail` panics in steps (blind, edge) | low | Unit tests assert the form, header and payload; the nil case needs a mis-written scenario. Rejected. |
| README and docs gaps: how to get a refresh token, config files (blind) | false | Story 10.3 owns the documentation. |

## Verification

**Commands:**
- `task go:test` -- expected: pass
- `task go:test:acceptance` -- expected: pass
- `task lint` -- expected: pass
- `task go:run` -- expected: builds and starts
