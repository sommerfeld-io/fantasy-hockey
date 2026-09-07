# Package: `mailer`

Sends email through Gmail SMTP (`smtp.gmail.com:587`, STARTTLS) using the standard library's `net/smtp` - no third-party mail library.

## Responsibilities

- `New(username, password)` creates a `Mailer` authenticated with a Gmail address and an App Password (`SMTP_USERNAME`/`SMTP_APP_PASSWORD`), not the account's own password.
- `Send(ctx, to, subject, body)` delivers a plain-text email synchronously, request-triggered - there is no background worker or queue.

## Design notes

- `smtp.SendMail` negotiates STARTTLS automatically once the server advertises it, which `smtp.gmail.com` always does on port 587, so no separate TLS handshake code is needed here.
- The real SMTP host/port is fixed; an unexported `newWithAddr` constructor lets tests point at a local, unreachable, or fake listener instead, since `net/smtp` has no interface to mock.
- `net/smtp` has no context-aware API. `Send` checks `ctx.Err()` before dialing and races the blocking call against `ctx.Done()` so an already-cancelled or cancelled-mid-flight context is honored.
