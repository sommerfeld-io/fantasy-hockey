# Package: `auth`

Implements the login-code request/validation flow and the stateless session mechanism built on top of it: given a submitted email, decide whether to generate, persist, and email a one-time login code, later check a submitted code against it, and sign/verify the session cookie that keeps a Participant logged in - without ever revealing more than intended to the caller.

## Responsibilities

- `Service.RequestLoginCode` looks up the email against `Store`, and on a match generates a random 6-digit code, hashes it (sha256), persists it, and emails it via `Mailer`.
- On a non-matching email, it returns `nil` with no side effects - the same outcome as a successful match, so `internal/web` can render an identical response either way (no enumeration leak, see FR-1).
- `Service.ValidateLoginCode` checks a submitted code against every still-unused code issued for the Participant within the last 10 minutes, marking the matching one used the moment it succeeds. A wrong, expired, or already-used code all return the identical `ErrInvalidCode` - the caller can't tell which applies, and neither can a visitor.
- `Service.EncodeSession`/`Service.DecodeSession` sign and verify a stateless, HMAC-SHA256 session token (`SESSION_SECRET`-keyed, no server-side session table - AD-12). `DecodeSession` also enforces the 30-minute sliding timeout, returning `ErrSessionExpired` once a token's issued-at is too old; this is the only place that 30-minute constant lives.

## Consumer-defined interfaces

`Store` and `Mailer` are defined in this package, not in `internal/store`/`internal/mailer`, even though the real implementations live there. This lets the GoDog acceptance tests exercise the real `auth` (and `web`, `mailer`) code against in-memory fakes, since `task go:build`'s Docker-stage tests have no network path to a live PostgreSQL/SMTP server. Only `internal/store`'s own unit tests need a real database, and they skip gracefully when `POSTGRES_TEST_DSN` is unset.

## Security notes

- The plaintext code is never persisted - only `sha256(code)` (`CodeHash`).
- A non-nil error from `RequestLoginCode` only ever indicates an unexpected infrastructure failure (database or SMTP down), never "email didn't match" - callers must not use it to vary the response shown to the visitor.
