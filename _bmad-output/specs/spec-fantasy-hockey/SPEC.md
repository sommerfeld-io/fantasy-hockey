---
id: SPEC-fantasy-hockey
companions:
  - ../../planning-artifacts/prds/prd-fantasy-hockey-2026-09-14/prd.md
  - ../../planning-artifacts/prds/prd-fantasy-hockey-2026-09-14/addendum.md
  - ../../planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-14/DESIGN.md
  - ../../planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-14/EXPERIENCE.md
  - ../../planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md
  - scoring-rules.md
sources: []
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Fantasy Hockey

## Why

A vision to realize, for a specific, already-felt pain: Basti, Sadl, and Tobbi run a private, season-long NHL prediction pool today on a manually maintained Excel workbook (one file per season), whose hand-calculated scoring and lack of deadline or name-typo enforcement already caused a real, uncorrected scoring error in a past season. This rebuild replaces that workbook with a mobile-first web app that predicts, scores, and ranks automatically — the goal is getting the player-facing loop (predict, wait, get scored, compare) right and reliable, every season, for exactly these three people.

## Capabilities

- **CAP-1 — Authentication & Session**
  - **intent:** A Player logs in via a one-time 6-digit code emailed on request, and stays authenticated across a sliding-timeout session — no password, no player switcher.
  - **success:** A code request gets an identical response whether or not the email matches a player; a code valid for 10 minutes or one use logs the player in; the session expires after 30 minutes idle, reset by any request; logout ends it immediately.

- **CAP-2 — Prediction Sets & Deadlines**
  - **intent:** A Player browses all Prediction sets grouped by phase, sees each one's live status, and edits any subset of an open set's fields independently until its deadline.
  - **success:** Four statuses (Open/Submitted/Closed/Upcoming) render correctly; a set locks with no grace period at its deadline; later Playoff rounds stay locked until their matchups are recorded in `fantasy-hockey.yml`; an empty required pick scores zero without blocking other picks.

- **CAP-3 — Before-the-Season Predictions**
  - **intent:** A Player enters four before-season picks — Cup champion, Presidents' Trophy, Division playoff-team picks, Division winners, and Player awards finalists — each validated per its own rules.
  - **success:** Division playoff-team picks enforce an exact 8-per-conference, 4/4-or-5/3 split with live caps; Player awards autocomplete rejects a non-matching NHL Player name rather than silently accepting it.

- **CAP-4 — Playoff Predictions**
  - **intent:** A Player re-picks the Stanley Cup winner once the Playoff field is set, and predicts each Series' winner and exact game count per unlocked round, labelled by conference.
  - **success:** Winner and game-count save together as one atomic unit; a later round opens only once its real matchups are recorded in `fantasy-hockey.yml`.

- **CAP-5 — Scoring & Leaderboard**
  - **intent:** The app computes every player's Regular, Playoff, and Total points live from recorded predictions and results, with no stored/cached score value, and displays a ranked Leaderboard.
  - **success:** Point values match `scoring-rules.md`; players with an equal Total share the same rank, no tiebreaker.

- **CAP-6 — Compare Predictions**
  - **intent:** A Player selects any Prediction set and sees every player's picks for it side by side, Division picks broken out per division, with consistent value formatting.
  - **success:** An unfilled value renders as a faint em-dash, never blank; Division-pick rows are one-per-division with that division's winner directly beneath.

- **CAP-7 — App Shell & Navigation**
  - **intent:** A fixed, mobile-first, dark-theme-only shell with a pinned header, pinned bottom navigation (Predict/Leaderboard/Compare), and one scrolling content region.
  - **success:** Header and nav never scroll, only content does; no light-mode setting exists anywhere in the app.

- **CAP-8 — Persistence & Season Data**
  - **intent:** Every player's picks are saved and visible to everyone across devices under the same rules; the season's canonical team and NHL Player lists are available wherever needed; a new pool starts each season while prior seasons' data is retained.
  - **success:** Picks persist and are visible to all players across sessions; season rollover follows `ARCHITECTURE-SPINE.md` AD-30 (one file per season, archived, `DATA_FILE` repointed).

## Constraints

- No admin/management UI of any kind, ever. Real-world results, each Prediction set's deadline, and every Playoff round's matchups are entered only by a human directly editing `fantasy-hockey.yml`, out of band — no in-app write path exists or may be added for any of these.
- No database engine — a single `fantasy-hockey.yml` file is the sole datastore, read/written only by `internal/store`, with exactly one writer process assumed.
- Server-rendered HTML only — no JS framework, no SPA/API split, no ORM. The only client-side JS is the shared autocomplete widget and Division-pick live-cap disabling, both cosmetic-immediacy only — the server independently re-validates and enforces on submit.
- Session is a stateless HMAC-signed cookie, `HttpOnly` + `SameSite=Lax`, no server-side session store; `Secure` is added only once TLS lands via a reverse proxy.
- Every identity a Player picks by name (a Team, an NHL Player) is referenced everywhere — prediction, autocomplete submission, and the hand-maintained result record — by the same stable id (team abbreviation or NHL Player slug), never by free-text display name. This is the one rule that exists specifically to prevent the name-typo scoring gap this rebuild is for.
- Mobile-first, dark-theme-only UI; other device sizes and a light mode are not v1 targets.

## Non-goals

- Deadline reminder emails (PRD FR-22) — fully specified for a future version but deliberately not built now; no trigger surface exists anywhere in v1.
- Any second role beyond the single Player identity — no admin/commissioner/scorekeeper account, no spectator/read-only account, no approval step.
- Registration or invite flow — the fixed 3-player list changes only by a deliberate out-of-band edit to `fantasy-hockey.yml`, never through the app.
- Real-time score updates during games, in-app messaging/chat, public/ranked pools beyond this private group, native mobile apps.
- A season-selector or multi-season browsing UI, or a Hall-of-Fame-style history page — history is retained on disk but not exposed in-app until a second season's history exists to show.

## Success signal

All three Players (Basti, Sadl, Tobbi) use the app through the entire NHL 2026–27 season, from before-the-season predictions through the Stanley Cup Final, without reverting to the old Excel workbook for any part of the pool.

## Assumptions

- The scoring point-value table (`scoring-rules.md`) is the PM's own initial calibration for season 1, not an externally validated rule set — may be revisited after a season's real results are in.
- `fantasy-hockey.yml`'s exact field-level schema is illustrative only; its final shape is a build-time decision that must follow the binding structural rules in `ARCHITECTURE-SPINE.md` (AD-9, AD-17, AD-20, AD-23, AD-24, AD-28, AD-30) — not itself a blocker to starting epic/story breakdown.
- Terminology in this SPEC and its companions follows the click-dummy-exact wording the UX pass deliberately chose (Leaderboard, Player awards, Playoffs Cup pick) — already reconciled into the PRD Glossary, so no divergence exists between SPEC, PRD, UX, and Architecture.

## Open Questions

- Build-timeline risk against the NHL 2026–27 season start is unresolved: this is a full solo rebuild with no fallback defined if v1 isn't ready by puck-drop. Not a spec-content gap — a feasibility/scheduling call for the builder before or during epic breakdown.
- `ARCHITECTURE-SPINE.md` AD-30 requires a human to manually archive `fantasy-hockey.yml` and repoint `DATA_FILE` at season rollover — confirm this manual process is acceptable for CAP-8, or whether a scripted/in-app rollover helper should be added to scope.
