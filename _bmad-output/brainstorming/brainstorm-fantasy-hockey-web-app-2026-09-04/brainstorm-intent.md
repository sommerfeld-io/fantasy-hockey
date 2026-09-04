# Intent: Fantasy Hockey Web App

## Problem / Context

A group of 3 friends (Basti, Sadl, Tobbi) currently runs a season-long NHL prediction game in Excel and wants a small web app to replace it. The real job to be done is low-effort, fun, low-stakes friendly competition with only a handful of touchpoints per season (pre-season predictions, pre-playoffs predictions) — not weekly engagement, not dispute-settling. The app is for exactly these 3 fixed participants; there is no registration/join flow and no admin or scorekeeper role. The guiding design principle is removing friction — both technical (manual data entry, disputes) and social (tie-breaks, scorekeeper judgment calls) — between the 3 friends.

## Domain & Scoring Rules

| Rule area                  | Resolved behavior                                                                                                                                                                     |
|-----------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Participants                | Fixed set of 3 (Basti, Sadl, Tobbi). No registration, no admin role, no scorekeeper role.                                                                                            |
| Awards predicted            | All 5: Hart, Norris, Vezina, Art Ross, Rocket Richard. Predicted pre-season.                                                                                                          |
| Award scoring               | Predict 3 names per award. Compare against actual top-3 by ranking; ties expand the set (e.g. 3-way tie for 3rd -> 5 "correct" names total). Each of a participant's 3 names scores 5pts if found anywhere in that set — order-independent, all 3 scored independently. |
| Award scoring timing        | Art Ross / Rocket Richard scorable continuously all season (stat leaderboards always available). Hart / Norris / Vezina scorable only once official nominees are published at season end (data-availability constraint, not a scope exclusion). |
| Playoff team marks          | Per conference, 8 teams make playoffs across 2 divisions (4/4 or 5/3 split, matching real NHL wildcard structure). Exactly one division-winner mark per division (4 divisions total). |
| Playoff series              | For every series in every round, predict winner + exact game count (4-0/4-1/4-2/4-3). Scoring per round, exact-result replacing (not adding to) correct-winner: Round 1 15/25, Round 2 25/35, Conference Finals 30/45, Stanley Cup Final 30/50. Progressive lock: Round 1 series lock before Round 1; each later round's series lock only once that round's matchups are known. |
| Presidents' Trophy          | Predict which team has the best regular-season record; 20pts if correct. Locked pre-season alongside awards, team marks, and the Early Pick. |
| Early Pick / Late Pick      | Cup winner picks (early + late), 20pts each. Counted in the Playoffs bucket (not Regular Season), since that's when they resolve.                                                    |
| Standings buckets           | Regular Season points and Playoffs points tracked separately per player; Total = sum of both determines ranking/winner. No deeper per-category breakdown.                            |
| Tie-break                   | None. Participants with equal Totals share the same rank.                                                                                                                             |
| Team identity               | Teams identified by a stable technical ID, not name (names/cities relocate, e.g. Arizona -> Utah). Per-season team list stored in the data store (not source code); loaded from a webservice when a new season is created. |
| Prediction visibility       | Own predictions always visible regardless of deadline. Other players' predictions visible only after their own deadline passes. Predictions become fully immutable after deadline — no grace period. |
| Live/in-progress standings  | None. No separate live/projected tier — the leaderboard is always simply whatever the last cron sync produced.                                                                       |
| Result ingestion            | Fully automated from an official/major NHL data webservice. No manual confirmation step, no scorekeeper, no admin data-entry page (contingent on the webservice assumption below).   |

## MoSCoW Scope (v1)

### Must

- Login (email -> 6-digit code) and logout
- Enter Predictions page: all 5 awards, playoff team marks, Presidents' Trophy pick, Early/Late Cup pick, and every playoff series (winner + exact game count) as each round unlocks
- Predictions (view-all) page: single scrollable page showing every player's predictions across all categories, respecting the visibility rule above
- Standings/leaderboard shown persistently on every page (Regular Season + Playoffs + Total buckets, no tie-break)
- Deadline reminder emails — deadline nudges only ("action required until TIMESTAMP"), no result summaries or newsletter content
- Automated result ingestion from official/major NHL webservice (no admin/scorekeeper page)
- Technical (stable) team ID data model, with per-season team list persisted in the data store

### Could

- Multi-season data model
- Season-selector dropdown
- Season Hub landing page (current season, your Total, quick links)
- Hall of Fame page (championship/runner-up/third-place counts across seasons)

### Won't (explicitly dropped from scope)

- Scorekeeper role / manual result confirmation
- Admin/management web page (as long as the free-webservice assumption below holds)
- Separate live/projected standings tier
- Deep per-category point breakdown beyond Regular Season / Playoffs buckets
- Tie-break mechanism
- Registration/join flow

## UI Shape

Screens: Login, Enter Predictions, Predictions (view-all), Logout — plus Season Hub and Hall of Fame deferred to Could.

Because there are only 3 players, Standings/Leaderboard does not get its own page — it is shown persistently as a sidebar/header widget on every page instead.

Guiding UI philosophy: transparency without friction — own picks always visible, other players' picks visible after their deadline, standings visible everywhere, no-tab single scrollable predictions page.

## Biggest Open Risk — RESOLVED (partial)

Resolved by `bmad-deep-recon` (technical): see `_bmad-output/planning-artifacts/research/technical-free-nhl-data-webservices-2026-09-04/research.md`.

- **Confirmed free and automatable**: team standings, Art Ross/Rocket Richard player stats, and playoff pairings/results — all live today via the NHL's undocumented `api-web.nhle.com` API (no key, no signup, JSON), verified by direct testing.
- **Confirmed NOT automatable via any free source**: Hart, Norris, Vezina award nominees/winners — no structured/free data source exists (checked against NHL, ESPN, TSN/Sportsnet, Hockey-Reference, Wikidata). These 3 trophies need a different plan for v1 (manual/rare entry once per season, or informational-only).
- **New risk surfaced**: the NHL API is unofficial, undocumented, has no ToS of its own, and was fully replaced without notice once already (2023), breaking third-party consumers. NHL.com's own ToS also technically prohibits automated scraping. The sync-container resilience design (parked below) should assume a breaking change will happen again, not treat it as unlikely — and the ToS ambiguity should be accepted knowingly, not resolved by assumption.

## Parked for Later (technical/architecture — not decided product scope)

- Multi-container docker-compose split: storage container, web-UI container, sync container
- Sync container fetches results on startup and on a cron schedule (interval configured via env var)
- Resilience to webservice downtime: write nothing on failure, rely on the next scheduled run
- No in-app alerting; metrics/logs/otel exported to Grafana Cloud for monitoring
