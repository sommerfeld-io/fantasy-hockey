# Epic 10 Context: Gmail API as a Second Mail Transport

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Let the person running the pool choose, with one setting, whether login-code emails go out over SMTP or through the Gmail API over HTTPS. This matters for production hosts where outbound SMTP is blocked or an app password is not an option. The epic also delivers a step-by-step guide for setting up the Gmail API. Development and tests stay on SMTP and Mailpit.

## Stories

- Story 10.1: Choose the Mail Transport With MAIL_TYPE
- Story 10.2: Send Login Codes Through the Gmail API
- Story 10.3: Document the Mail Settings and the Gmail API Setup

## Requirements & Constraints

- `MAIL_TYPE` selects `SMTP` or `API`, case-insensitive. The two are mutually exclusive. Unset or empty means SMTP, so existing deployments and the dev compose are unchanged.
- Any other `MAIL_TYPE` value stops the app at startup with an error that names `MAIL_TYPE` and lists the allowed values. There is no fallback.
- Settings of the unselected transport are ignored, with one startup warning that names the ignored variables and never their values.
- The API transport needs `GMAIL_CLIENT_ID`, `GMAIL_CLIENT_SECRET`, `GMAIL_REFRESH_TOKEN` and `GMAIL_SENDER`. The Gmail API cannot send with an API key, so the existing Cloud project API key is not used and is not a setting.
- The app must still start with no mail settings. A send with a missing setting returns an error naming the first missing variable and makes no HTTP request.
- Secrets never appear in errors, logs or metrics: not the client secret, refresh token, access token or recipient address, even if Google echoes them back. Reuse the SMTP sender's recipient redaction.
- Audit events and metrics are identical for both transports: no new metric label and no Player identity in metrics.
- The Gmail API is production-only. The dev compose and the repo `.env` stay untouched, with no `GMAIL_*` variable in either.
- The example production compose (`docs/examples/docker-compose.yml`) passes `MAIL_TYPE` and the four `GMAIL_*` variables with empty placeholders and a comment that real values come from the secret store. Story 9.6 decided on no drift test, so none applies.
- Docs must cover: the Configuration table in `docs/index.md`, the no-mail-settings paragraph for both transports, and a new `docs/gmail-api-setup.md` (production only, linked from `docs/index.md`). Stale SMTP-only statements in `docs/operator-guide.md` and `README.md` must point at the Configuration table.
- The guide must state:
    - API keys do not work for sending.
    - A consent screen left in Testing mode expires the refresh token after 7 days for external accounts. The fix is to publish the app to production or use a Workspace-internal app.
    - The YouTube walkthrough `https://youtu.be/LgcNxu79rwo` is a helpful extra that may not match Google's current screens.
    - Secrets are never committed, and the real Cloud project and account are not named.

## Technical Decisions

- Selection lives in `internal/mailer`: a constructor takes the settings and returns the existing `Sender` func type. `main.go` reads the environment once, passes the values on, and holds no transport logic.
- `internal/mailer` stays the single outbound email package, called only by `internal/auth` (and later `internal/predictions`).
- Sending flow: exchange the refresh token for an access token at the token URL, then POST to Gmail `users.messages.send` with a Bearer header and a JSON body whose `raw` field is the base64url-encoded RFC 5322 message. The OAuth scope is `https://www.googleapis.com/auth/gmail.send`.
- Message: `From` (`GMAIL_SENDER`), `To`, `Subject` and a plain-text body with the same login-code content as SMTP, using CRLF line endings. Reject CR or LF in `to` or `subject` to prevent header injection.
- Cache the access token and reuse it until it expires.
- Use a request timeout so a hung connection cannot hold a login request open. Non-2xx and transport errors return an error with the status and Google's error reason.
- The sender constructor takes the token URL and send URL as parameters. Tests use `httptest` servers and never call Google. Production passes Google's real URLs from constants, with no environment override.
- Use stdlib `net/http` and `encoding/json` unless a dependency is justified and approved at build time.
- Test-first is mandated by the user. Write unit tests and the Gherkin feature `src/acceptance-tests/features/mail-transport.feature` (with step definitions) before the implementation, and see them fail first.
- Gherkin scenarios cover: the default is SMTP; `API` sends through the fake Gmail server and not over SMTP; `SMTP` sends over SMTP and not through the fake Gmail server; an invalid value stops startup with the named error. The fake Gmail server must receive exactly one send request addressed to the player, with a decoded 6-digit code that logs the player in.
- Story 10.3 is docs-only, so no acceptance test applies. `task lint` (markdownlint, link checker, filename linters) must pass.

## Cross-Story Dependencies

- Story 10.1 depends on Story 10.2 for the real API sender. Build the selection with a fake API sender first, then wire in the real one.
- Story 10.3 depends on Stories 10.1 and 10.2, so the documented variable names are final.
- Builds on Epic 9: the example production compose and its drift test, and the audit and metrics behavior for login events.
- Builds on Epic 8's docs conventions, with `docs/` as the single place for documentation.
