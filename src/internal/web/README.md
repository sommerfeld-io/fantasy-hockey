# Package: `web`

The presentation layer: serves the app shell (Predict/Leaderboard/Compare), the per-Prediction-Set pick sheets, and the full login/session flow over the standard library's `net/http`, rendering server-side `html/template` views. No third-party router or template engine.

## Routes

`NewServer(st *store.Store, send mailer.Sender, secret string) http.Handler` wires every route. Authenticated routes are registered on an inner `ServeMux` wrapped by `requireSession`; public routes sit directly on the outer mux.

| Route                      | Auth | Behavior                                                                                                                                                                                                    |
|----------------------------|------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `GET /{$}`, `GET /predict` | yes  | App shell with Predict active: the phase-grouped Prediction Set lists (Before the season / Playoffs), each row with its status pill and countdown. Upcoming rows are dimmed and not linked.                 |
| `GET /leaderboard`         | yes  | App shell with Leaderboard active: every player ranked by Total (Player, Regular, Playoff, Total), recomputed from the store on every request. Leaders are gold, but nobody is while the top Total is 0.    |
| `GET /compare`             | yes  | App shell with Compare active: selectable Prediction Sets as chips (Before the season / Playoffs) and every player's picks for the chosen `?set=` side by side. Read-only.                                  |
| `GET /predict/{id}`        | yes  | The sheet for one Prediction Set: the cup/presidents team dropdown, the divisions chip form, the awards finalist form, or a static stub for every other id. Closed sets render read-only.                   |
| `POST /predict/{id}`       | yes  | Submits a pickable set (cup, presidents, divisions, awards). Revalidates every id server-side, saves, and redirects to `/predict`. A rejected submission re-renders with an inline error and saves nothing. |
| `GET /login`               | no   | Email-entry step. An already-valid session is redirected into the shell.                                                                                                                                    |
| `POST /login`              | no   | Requests a login code via `internal/auth`. The response is identical whether or not the email matched.                                                                                                      |
| `POST /login/code`         | no   | Validates the code. A match sets the signed session cookie and redirects to `/`. A wrong, expired, or used code re-renders with one generic error.                                                          |
| `POST /logout`             | no   | Clears the session cookie and redirects to `/login`, whatever cookie was presented.                                                                                                                         |
| `GET /static/`             | no   | Embedded `static/` assets (`styles.css`, `divisions.js`, `awards.js`).                                                                                                                                      |

### Sheet status codes

An unknown id and an id whose set is marked `upcoming: true` get the same generic `404 Not Found`, whatever the set's kind or deadline, so a locked set can't be opened or submitted by direct URL. This is the gate later round unlocking builds on.

`GET /predict/{id}` checks, in this order:

1. Unknown or upcoming id: `404`.
2. Deadline that fails to parse: `404`.

`POST /predict/{id}` checks, in this order:

1. Non-pickable id: `404`.
2. Unknown or upcoming id: `404`.
3. Deadline that fails to parse: `404`.
4. Past deadline: `403 Forbidden`, with no override.
5. Form parse failure (e.g. an oversized body): `500`.

## File layout

| File                 | Contents                                                                                                         |
|----------------------|------------------------------------------------------------------------------------------------------------------|
| `web.go`             | Embeds and templates, shared constants, `NewServer` and routing, `requireSession`, static files, render helpers. |
| `shell.go`           | Bottom-nav tabs, shell view data, `handleShell`.                                                                 |
| `predict.go`         | The Predict list view model: row status, pill, accent, and phase grouping.                                       |
| `leaderboard.go`     | The Leaderboard view model: `internal/standings` rows with precomputed rank-badge and Total classes.             |
| `compare.go`         | The Compare view model: set chips, default selection, and per-category rows of every player's values.            |
| `options.go`         | The shared `{id, label}` autocomplete embed shape.                                                               |
| `roster.go`          | `roster[T]`, the one id-keyed membership lookup behind every server-side team and NHL Player check.              |
| `sheet.go`           | Sheet-kind registry, `sheetData`, `handleSheet`, `handleSheetSubmit`, and the cup/presidents sheet.              |
| `sheet_divisions.go` | Divisions sheet view model, validation, and submit.                                                              |
| `sheet_awards.go`    | Award tables, awards sheet view model, validation, and submit.                                                   |
| `login.go`           | Login, login-code and logout handlers.                                                                           |

Each file has a matching `*_test.go`. General test helpers (base store fixtures, the cup/presidents seeds) live in `web_test.go`. Kind-specific helpers (e.g. `newTestStoreWithAwardsRoster`, `postSheet`, `markUpcoming`) live beside their kind's tests and are shared package-wide.

## Design notes

- `requireSession` guards every authenticated route. A valid, unexpired cookie (`auth.ValidateSession`) is re-issued with a fresh `issued_at` before the request proceeds, sliding the idle timeout forward. Anything else redirects (302) to `/login` with no distinguishing message. Every response it lets through also carries `Cache-Control: no-store`, since a shared cache in front of the app could otherwise serve one player's page to another.
- Compare is display-only: it reads picks through the store's `Find…` methods, maps ids to team names, abbreviations and NHL Player display names only at render time, and never imports `internal/scoring` or `internal/standings` (guarded by `TestCompareShouldNotImportScoringOrStandings`). A set that is effectively Upcoming, or whose deadline fails to parse, gets no chip. An unknown or unselectable `?set=`, or no `?set=` at all, falls back to the earliest-deadline Before the season set (the first listed on a tie), or to the first selectable Playoffs set when none is selectable there; when nothing at all is selectable, both selector rows render a "Nothing to compare yet." note and no table is shown. The one exception is a hand-typed `?set=` naming a round-gated id (`r2`, the conference finals, or the Final) whose matchups aren't recorded yet: it shows a dashed "Matchups not set." note in place of the table instead of falling back, since a gated round never gets a rendered chip to begin with. Every other Upcoming id - a Before the season set, or `r1`'s own hand-maintained flag - still falls back normally. Team-abbreviation values (playoff teams, division winners, series winners) render as a small tag; full team names, award finalist names, and a series row's "in N" suffix stay plain text; an unfilled value renders as a faint em dash.
- The Leaderboard computes no points or ranks. It renders `internal/standings.Rows` and never imports `internal/scoring` (guarded by `TestWebShouldNotImportScoring`).
- Every submitted team id or NHL Player slug is revalidated against the store's canonical lists through `roster` (AD-10), whatever the client allowed.
- This package is exercised by its unit tests and by the GoDog acceptance tests in `src/acceptance-tests/`.
