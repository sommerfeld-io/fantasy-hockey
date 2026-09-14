# Package: `web`

The presentation layer: serves the home page and the login-code request flow over the standard library's `net/http`, rendering server-side `html/template` views. No third-party router or template engine.

## Responsibilities

- `NewServer(st *store.Store, send mailer.Sender) http.Handler` wires every route and returns an `http.Handler` ready to be served.
- `GET /` renders the application name ("Fantasy Hockey") and the current date and time, sourced from `internal/clock`.
- `GET /login` renders the email-entry step of the login flow.
- `POST /login` matches the submitted email against `st` via `internal/auth` and renders the code-entry step. The response is identical whether or not the email matched - only a failure to persist the new login code surfaces as a 500; a failure to email it is handled entirely inside `internal/auth` and never reaches this layer.
- `GET /static/` serves the embedded `static/` assets (e.g. `styles.css`) via `http.FileServerFS`.

## Design notes

- This package is exercised primarily by the GoDog acceptance tests in `src/acceptance-tests/`.
