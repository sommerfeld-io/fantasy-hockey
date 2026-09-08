# Package: `web`

The presentation layer: serves the home page over the standard library's `net/http`, no third-party router or template engine.

## Responsibilities

- `NewServer()` wires the home page route and returns an `http.Handler`.
- `GET /` renders the application name ("Fantasy Hockey") and the current date and time, sourced from `internal/clock`.

## Design notes

- This package is exercised primarily by the GoDog acceptance tests in `src/acceptance-tests/`.
