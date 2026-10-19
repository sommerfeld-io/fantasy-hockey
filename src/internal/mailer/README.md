# Package: `mailer`

The external gateway for outbound email, wrapping the standard library's `net/smtp` (AD-12).

## Responsibilities

- `Sender` is a func type - one method, dependency-injected per AD-3 - so callers and tests can substitute a one-line closure fake.
- `NewSMTPSender(host, port, username, password)` builds a `Sender` that sends over SMTP. All four inputs may be empty; the app must start with none of them set. An empty username skips SMTP AUTH (matches a local no-auth capture tool); a non-empty username always authenticates via `smtp.PlainAuth`.
- `New(Settings)` picks the transport from `MAIL_TYPE` (`SMTP` or `API`, case-insensitive; unset or empty means `SMTP`) and returns a `Sender`. Any other value is an error naming `MAIL_TYPE` and the allowed values - there is no fallback to SMTP. `SettingsFromEnv(getenv)` reads all mail variables in one place.
- `IgnoredVars(Settings)` returns the names (never values) of set variables that belong to the unselected transport, so the caller can log one warning.
- Until Story 10.2, the `API` sender returns a "not implemented yet" error and never touches SMTP.
- Calling the returned `Sender` with an empty host returns an error instead of silently no-op'ing or falling back to a hardcoded host.

## Design notes

- `internal/mailer` is called only by `internal/auth` (and, once built, `internal/predictions`).
- No email content beyond login codes (and later, deadline reminders) is ever sent from this package.
