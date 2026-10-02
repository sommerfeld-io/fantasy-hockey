# Package: `mailer`

The external gateway for outbound email, wrapping the standard library's `net/smtp` (AD-12).

## Responsibilities

- `Sender` is a func type - one method, dependency-injected per AD-3 - so callers and tests can substitute a one-line closure fake.
- `NewSMTPSender(host, port, username, password)` builds a `Sender` that sends over SMTP. All four inputs may be empty; the app must start with none of them set. An empty username skips SMTP AUTH (matches a local no-auth capture tool); a non-empty username always authenticates via `smtp.PlainAuth`.
- Calling the returned `Sender` with an empty host returns an error instead of silently no-op'ing or falling back to a hardcoded host.

## Design notes

- `internal/mailer` is called only by `internal/auth` (and, once built, `internal/predictions`).
- No email content beyond login codes (and later, deadline reminders) is ever sent from this package.
