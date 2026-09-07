# Package: `web`

The presentation layer: renders the Login page and handles login-code requests over the standard library's `net/http` and `html/template` - no third-party router or template engine.

## Responsibilities

- `NewServer(auth *auth.Service)` wires the Login page routes and returns an `http.Handler`.
- `GET /login` renders the email-entry form.
- `POST /login` calls `auth.Service.RequestLoginCode` and always renders the same generic confirmation message ("Check the entered email address."), regardless of whether the submitted email matched a Participant - this is what makes FR-1's no-enumeration guarantee hold end to end.
- `GET /static/*` serves the embedded Steel Ice stylesheet.

## Design notes

- Any error from `RequestLoginCode` is logged server-side (`slog`) but never changes the HTTP response - the visitor-facing behavior must never differ based on infrastructure failures any more than it differs based on a match/non-match.
- Templates and static assets are embedded (`//go:embed`) so the compiled binary has no runtime dependency on the filesystem layout.
- This package is exercised primarily by the GoDog acceptance tests in `src/acceptance-tests/`, run against in-memory fakes of `internal/auth`'s `Store`/`Mailer` interfaces, per this story's testing strategy.
