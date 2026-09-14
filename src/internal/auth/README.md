# Package: `auth`

Login-code issuance: matches a submitted email against a `Player`, and on a match generates, hashes, persists, and emails a one-time code (FR-1).

## Responsibilities

- `RequestLoginCode(st, send, email)` looks up `email` via `internal/store`. On a match it generates a 6-digit code, persists its sha256 hash as a new `LoginCode` row (never mutating an earlier row), and emails the plaintext code via the injected `mailer.Sender`.
- On no match, it is a no-op: no row is written, `send` is never called - and it still returns `nil`, so a caller's response is identical either way.
- A failed send is logged via `slog.Error` here and never surfaces to the caller; only a failed store write is returned as an error.

## Design notes

- The plaintext code exists only in memory and in the outgoing email - it is never persisted or logged.
- Depends on `internal/store` and `internal/mailer` only, per the layered dependency direction (AD-8).
