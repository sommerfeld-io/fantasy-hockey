# Package: `web`

The presentation layer: renders the Login page, handles login-code requests and validation, and gates authenticated routes behind a session cookie - all over the standard library's `net/http` and `html/template`, no third-party router or template engine.

## Responsibilities

- `NewServer(auth *auth.Service)` wires every route and returns an `http.Handler`.
- `GET /login` renders the email-entry form.
- `POST /login` calls `auth.Service.RequestLoginCode` and always renders the same generic confirmation message ("Check the entered email address.") alongside the code-entry step, regardless of whether the submitted email matched a Participant - this is what makes FR-1's no-enumeration guarantee hold end to end.
- `POST /login/code` calls `auth.Service.ValidateLoginCode`. On success it sets the signed session cookie and redirects to `/`; on failure (wrong, expired, or already-used code) it re-renders the code step with the generic "Invalid code." message, never distinguishing the reason.
- `GET /` is wrapped in `requireSession`: a valid session cookie re-issues the cookie with a fresh issued-at (the sliding 30-minute timeout) and serves the minimal authenticated home placeholder; a missing, malformed, or expired cookie instead renders the Login page - showing "Session expired." only when a cookie was present and had genuinely timed out.
- `GET /static/*` serves the embedded Steel Ice stylesheet.

## Design notes

- Any error from `RequestLoginCode`/`ValidateLoginCode` that isn't the expected `auth.ErrInvalidCode` is logged server-side (`slog`) but never changes the HTTP response - the visitor-facing behavior must never differ based on infrastructure failures any more than it differs based on a match/non-match.
- The session cookie carries an HMAC-SHA256-signed token (see `internal/auth`) - this package never inspects or trusts its contents beyond calling `auth.Service.DecodeSession`.
- Templates and static assets are embedded (`//go:embed`) so the compiled binary has no runtime dependency on the filesystem layout.
- This package is exercised primarily by the GoDog acceptance tests in `src/acceptance-tests/`, run against in-memory fakes of `internal/auth`'s `Store`/`Mailer` interfaces, per this story's testing strategy.
