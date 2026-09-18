# Package: `auth`

Login and session logic: matches a submitted email against a `Player`, issues and validates one-time login codes, and issues, verifies, and clears the signed session cookie (FR-1..FR-4).

## Responsibilities

- `RequestLoginCode(st, send, email)` looks up `email` via `internal/store`. On a match it generates a 6-digit code, persists its sha256 hash as a new `LoginCode` row (never mutating an earlier row), and emails the plaintext code via the injected `mailer.Sender`.
  - On no match, it is a no-op: no row is written, `send` is never called - and it still returns `nil`, so a caller's response is identical either way.
  - A failed send is logged via `slog.Error` here and never surfaces to the caller; only a failed store write is returned as an error.
- `ValidateLoginCode(st, code)` hashes the submitted code and delegates to `st.ConsumeLoginCode`. A wrong, expired, or already-used code all produce the identical `ok=false, err=nil` outcome - never distinguishing which, so a caller can't either.
- `IssueSessionCookie(playerID, secret)` builds a stateless, HMAC-SHA256-signed session cookie (`HttpOnly`, `SameSite=Lax`, no `Secure`/`Max-Age`/`Expires`) carrying the player id and issue time.
- `ParseSessionCookie(c, secret)` verifies a cookie's signature and shape, returning the player id and issued-at time it carries.
- `ValidateSession(c, secret)` layers on `ParseSessionCookie`: a missing/invalid/tampered cookie, an empty decoded player id, and an idle-expired `issued_at` (`SessionIdleTimeout`, 30 minutes) all collapse into one `ok=false`, so a caller can't distinguish which case occurred.
- `ClearSessionCookie()` builds a cookie that instructs the browser to delete the session cookie immediately (empty value, negative `Max-Age`) - the only way to end a session, since there's no server-side session store to invalidate against.

## Design notes

- The plaintext login code exists only in memory and in the outgoing email - it is never persisted or logged.
- The session cookie's value is `base64url(player_id) + "|" + issued_at_RFC3339 + "." + hex(HMAC-SHA256(secret, payload))`; `player_id` is base64url-encoded on its own (not jointly with `issued_at`) so a hand-typed player id containing a literal `"|"` can never be misread as the segment separator.
- Sessions are entirely stateless (AD-11): nothing in this package writes to `internal/store`, and there is no revocation list - `ClearSessionCookie` and the 30-minute idle timeout are the only ways a session ever ends.
- Depends on `internal/store` and `internal/mailer` only, per the layered dependency direction (AD-8).
