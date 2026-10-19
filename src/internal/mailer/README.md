# Package: `mailer`

The external gateway for outbound email, wrapping the standard library's `net/smtp` (AD-12).

## Responsibilities

- `Sender` is a func type - one method, dependency-injected per AD-3 - so callers and tests can substitute a one-line closure fake.
- `NewSMTPSender(host, port, username, password)` builds a `Sender` that sends over SMTP. All four inputs may be empty; the app must start with none of them set. An empty username skips SMTP AUTH (matches a local no-auth capture tool); a non-empty username always authenticates via `smtp.PlainAuth`.
- `New(Settings)` picks the transport from `MAIL_TYPE` (`SMTP` or `API`, case-insensitive; unset or empty means `SMTP`) and returns a `Sender`. Any other value is an error naming `MAIL_TYPE` and the allowed values - there is no fallback to SMTP. `SettingsFromEnv(getenv)` reads all mail variables in one place.
- `IgnoredVars(Settings)` returns the names (never values) of set variables that belong to the unselected transport, so the caller can log one warning.
- The `API` sender (`gmail.go`) sends through the Gmail API with stdlib `net/http` only. It exchanges `GMAIL_REFRESH_TOKEN` (with `GMAIL_CLIENT_ID` and `GMAIL_CLIENT_SECRET`) for an access token at Google's token endpoint, caches it until shortly before it expires, then POSTs the message to `users.messages.send` with a Bearer header and a JSON `raw` field (base64url RFC 5322 message, CRLF line endings, `From` is `GMAIL_SENDER`). The OAuth scope is `https://www.googleapis.com/auth/gmail.send`; an API key cannot send mail.
- A send with a missing Gmail setting returns an error naming the first missing variable (client ID, client secret, refresh token, sender) and makes no HTTP request. CR or LF in the sender, recipient or subject is rejected before any request. The HTTP client times out after 10 seconds.
- Errors carry the HTTP status and Google's error reason. The recipient, client secret, refresh token and access token (plain and URL-escaped) are redacted from every error, even if Google echoes them.
- `Settings.GmailTokenURL` and `Settings.GmailSendURL` exist for tests only and are never read from the environment; empty means Google's real URLs. Tests point them at `httptest` servers and never call Google.
- Calling the returned `Sender` with an empty host returns an error instead of silently no-op'ing or falling back to a hardcoded host.

## Design notes

- `internal/mailer` is called only by `internal/auth` (and, once built, `internal/predictions`).
- No email content beyond login codes (and later, deadline reminders) is ever sent from this package.
