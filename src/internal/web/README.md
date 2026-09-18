# Package: `web`

The presentation layer: serves the home page and the full login/session flow over the standard library's `net/http`, rendering server-side `html/template` views. No third-party router or template engine.

## Responsibilities

- `NewServer(st *store.Store, send mailer.Sender, secret string) http.Handler` wires every route and returns an `http.Handler` ready to be served. `GET /{$}` is registered on its own inner `ServeMux`, wrapped by `requireSession` before being mounted on the outer mux, so a future protected route joins it the same way without touching how public routes are wired.
- `GET /{$}` (authenticated) renders the application name ("Fantasy Hockey") and the current date and time, sourced from `internal/clock`.
- `GET /login` renders the email-entry step of the login flow.
- `POST /login` matches the submitted email against `st` via `internal/auth` and renders the code-entry step. The response is identical whether or not the email matched - only a failure to persist the new login code surfaces as a 500; a failure to email it is handled entirely inside `internal/auth` and never reaches this layer.
- `POST /login/code` validates the submitted code via `internal/auth`. A match sets a signed session cookie and redirects to `/`; a wrong, expired, or already-used code all re-render the same code screen with an identical generic error and the submitted value retained.
- `POST /logout` clears the session cookie and redirects to `/login`, unconditionally - it is deliberately not wrapped by `requireSession`, so logout still works with a missing, expired, or tampered cookie.
- `GET /static/` serves the embedded `static/` assets (e.g. `styles.css`) via `http.FileServerFS`.

## Design notes

- `requireSession` (used only for `GET /{$}` today) is this package's session-guarding middleware: a valid, unexpired cookie (`auth.ValidateSession`) is re-issued with a fresh `issued_at` before the request proceeds, sliding the idle timeout forward; anything else redirects (302) to `/login` with no distinguishing message. Every response it lets through also carries `Cache-Control: no-store`, since a shared cache in front of the app could otherwise serve one player's authenticated page to another.
- This package is exercised primarily by the GoDog acceptance tests in `src/acceptance-tests/`.
