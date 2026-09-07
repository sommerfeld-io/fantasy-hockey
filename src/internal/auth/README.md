# Package: `auth`

Implements the login-code request flow: given a submitted email, decide whether to generate, persist, and email a one-time login code - without ever revealing that decision to the caller.

## Responsibilities

- `Service.RequestLoginCode` looks up the email against `Store`, and on a match generates a random 6-digit code, hashes it (sha256), persists it, and emails it via `Mailer`.
- On a non-matching email, it returns `nil` with no side effects - the same outcome as a successful match, so `internal/web` can render an identical response either way (no enumeration leak, see FR-1).

## Consumer-defined interfaces

`Store` and `Mailer` are defined in this package, not in `internal/store`/`internal/mailer`, even though the real implementations live there. This lets the GoDog acceptance tests exercise the real `auth` (and `web`, `mailer`) code against in-memory fakes, since `task go:build`'s Docker-stage tests have no network path to a live PostgreSQL/SMTP server. Only `internal/store`'s own unit tests need a real database, and they skip gracefully when `POSTGRES_TEST_DSN` is unset.

## Security notes

- The plaintext code is never persisted - only `sha256(code)` (`CodeHash`).
- A non-nil error from `RequestLoginCode` only ever indicates an unexpected infrastructure failure (database or SMTP down), never "email didn't match" - callers must not use it to vary the response shown to the visitor.
