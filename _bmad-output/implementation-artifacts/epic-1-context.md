# Epic 1 Context: Account Access & App Shell

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

A player can log into the app on their phone via an emailed one-time code, stay logged in through a sliding-timeout session, log out, and land in the persistent header/bottom-nav shell used for the rest of the season. This is the gateway epic: nothing else in the app is reachable without it, and it is the first real feature built on top of the existing brownfield skeleton (`main.go`, `internal/server`, a placeholder `internal/web`, `internal/clock`) rather than a greenfield start.

## Stories

- Story 1.1: Request a Login Code
- Story 1.2: Enter Login Code and Establish Session
- Story 1.3: Stay Logged In With Sliding Session Timeout
- Story 1.4: Log Out
- Story 1.5: Persistent App Shell With Player Identity and Navigation

## Requirements & Constraints

- A code request's HTTP response must be identical whether or not the submitted email matches a player — no timing or shape difference reveals a match.
- A matching request emails a 6-digit numeric code; requesting a new code never invalidates an earlier still-valid one.
- A submitted code is valid for 10 minutes or one use, whichever comes first; a wrong, expired, or already-used code all produce the exact same rejection, with nothing distinguishing which case applied.
- A session stays alive while used and expires after 30 minutes of inactivity; any authenticated request resets that countdown.
- Logout ends the session immediately on that device.
- The app always acts as the single logged-in player and only ever lets them edit their own picks — no player switcher exists anywhere.
- The header and bottom navigation are pinned and never scroll; only the content region does. Bottom nav always has exactly three tabs: Predict, Leaderboard, Compare.
- The UI is mobile-first (~360–430px), single-column, dark-theme only, with no light-mode setting reachable anywhere; on wider viewports the column stays centered at the same max width rather than stretching.
- A required runtime secret signs sessions; the app must refuse to start rather than generate one in-process if it's missing.
- Outbound email settings must all be independently optional at startup (the app must still start with none of them set) so local, non-container verification keeps working; a send attempted without them logs an error rather than failing silently or defaulting to a hardcoded host.
- Login/session persistence is shared across devices, not per-device state.

## Technical Decisions

- Layered flow: `internal/web` (HTTP handlers/templates) → `internal/auth` (login-code issuance/validation, session cookie) → `internal/store` (reads/writes `fantasy-hockey.yml`); `internal/auth` also calls `internal/mailer` (stdlib `net/smtp`) to send codes. `main.go`/`internal/server` stay wiring/bootstrap only — no auth or session logic there.
- Sessions are stateless HMAC-signed cookies (identity + issued-at), `HttpOnly`, `SameSite=Lax`; `Secure` is deliberately omitted until a TLS reverse proxy exists. No server-side session store or revocation list. Every authenticated request re-issues the cookie with a fresh issued-at to implement the sliding timeout; this renewal is middleware inside `internal/web`, wrapping only the authenticated routes on its own `ServeMux` — never in `internal/server`.
- The signing key comes from a required `SESSION_SECRET` env var, read once at startup; unset means the process must fail to start.
- `LoginCode` is a tracked, persisted entity (not transient/in-memory): one entry per issued code with player, `code_hash` (sha256 of the code, never plaintext), `issued_at`, `used_at` (nullable). Validation hashes the submitted code and compares hashes. It gets an application-generated UUID (runtime-created entity), unlike hand-maintained entities which use human-readable keys.
- SMTP config (`SMTP_HOST`/`SMTP_PORT`/`SMTP_USERNAME`/`SMTP_APP_PASSWORD`) is env-only, all optional at startup, unlike `SESSION_SECRET`. Empty username skips SMTP AUTH (matches the local dev capture tool); non-empty always authenticates via `smtp.PlainAuth`. Production points at Gmail with an App Password; local dev's docker-compose points at a Mailpit-style capture service with a web UI.
- Every persisted timestamp (`issued_at`, `used_at`) is RFC3339 via `internal/clock.NowTime().UTC().Format(time.RFC3339)`.
- Persistence follows the whole-repo rule: single `fantasy-hockey.yml`, one in-memory structure behind a never-exported mutex in `internal/store`, atomic write-and-rename on every write; the fixed player list (name, email, slug id) is hand-maintained/bootstrap-seeded, never created through the app.
- No JS framework, SPA/API split, or ORM anywhere — server-rendered `html/template` and stdlib `net/http` `ServeMux` only; Epic 1 has no sanctioned client-side JS use case of its own (that's Epic 2's autocomplete/live-cap work).

## UX & Interaction Patterns

- Login is a full-screen flow that precedes the app shell entirely — no header, no bottom nav, not one of the three nav destinations. Email step: single email field + "Send code" button, transitioning to the code step with a neutral, identical confirmation regardless of match. Code step: single 6-digit field + "Log in" button; wrong/expired/used all show the same copy, e.g. "That code didn't work — check it and try again." — never which case applied.
- On successful code entry, go straight into the app shell on Predict — no intermediate "welcome" screen.
- On session timeout, silently route back to Login (email) with no "you were logged out" messaging [ASSUMPTION, flagged for confirmation in the UX spine].
- Header: pinned, player name bold on the left, season label (muted, e.g. "NHL 2026–27") on the right, no logo, `surface` background with bottom border.
- Bottom navigation: pinned, three equal tabs (Predict/checklist, Leaderboard/medal, Compare/people), active tab in `ice`, inactive `muted`, `surface` background with top border.
- Dark-only color/typography/shape tokens apply throughout (near-monochrome greys, `ice` accent reserved for selection/active-nav borders, sentence case everywhere, no shadows/gradients).
- Accessibility floor: tap targets ≥32px, color never the sole signal, every form input carries a programmatic label, focus order follows visual top-to-bottom order.
- Tap-only interactions; no page reload for nav switching.

## Cross-Story Dependencies

- Story 1.1 is the first feature layered onto the existing skeleton (`main.go`, `internal/server`, placeholder `internal/web`, `internal/clock`) — it's what introduces `internal/auth`, `internal/mailer`, and `internal/store`'s write paths for the first time.
- Story 1.2 depends on 1.1 having created a valid `LoginCode` to validate against, and introduces the session cookie mechanism Stories 1.3 and 1.4 both build on.
- Story 1.3's sliding renewal and Story 1.4's logout both extend the same session-cookie middleware established in 1.2 — they should land as one coherent auth middleware, not three independent implementations.
- Story 1.5's header/nav shell reads player identity from the session established in 1.2–1.4, so it depends on that middleware existing and correctly identifying the logged-in player.
- Every other epic (2–6) builds its screens inside the shell this epic produces and behind the authentication this epic enforces.
