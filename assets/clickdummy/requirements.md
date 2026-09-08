# Face-Off Pool — Requirements (Epics & Stories)

A private, season-long NHL prediction game for a small fixed group of friends, delivered as a mobile-first web app. It replaces a manually maintained Excel workbook (one file per season). Players enter predictions before the season and throughout the playoffs, and the app scores them and ranks players on a leaderboard.

---

## 1. Actors

- **Player** — a participant in the pool. Makes predictions, sees the leaderboard, and compares everyone's picks. The app currently runs in the context of a single player ("you") and only that player's picks are editable. Participants today: Basti, Sadl, Tobbi.
- **Commissioner / Admin** *(implied, not yet a real role)* — defines the season, sets deadlines, enters the real playoff matchups once known, records actual results, and triggers scoring. In the prototype these are static/seeded.

## 2. Domain glossary

- **Conference** — Eastern or Western.
- **Division** — Atlantic, Metropolitan (East); Central, Pacific (West). 8 teams each; 32 teams total.
- **Playoff field** — the teams predicted to reach the playoffs, chosen per division.
- **Division winner** — the team predicted to finish first in a division.
- **Presidents' Trophy** — best regular-season record in the league.
- **Stanley Cup** — league champion.
- **Trophies (individual)** — Hart (MVP), Norris (defenseman), Vezina (goalie), Art Ross (points leader), Rocket Richard (goals leader). Each prediction is a set of **3 finalists**.
- **Series** — a best-of-seven playoff matchup, won "in N games" (4–7).
- **Prediction set** — one deadline-bound group of predictions (e.g. "Division picks", "Playoff round 1").

## 3. Status legend

- ✅ Implemented in the prototype
- 🟡 Implemented with sample/placeholder data
- 🔜 Planned, not yet built
- ❓ Open decision required

---

## Epic 1 — Prediction sets, phases & deadlines

Predictions are organised into deadline-bound *sets*, grouped into two phases: **Before the season** and **Playoffs**.

- **US-1.1 — Browse prediction sets** ✅
  As a player, I want to see all prediction sets grouped by phase, so I know what I can predict and when.
  - Two sections: "Before the season" and "Playoffs".
  - Each set shows a title, a short subtitle, its deadline (date + time), and a relative countdown (e.g. "in 27 days").

- **US-1.2 — Set lifecycle & status** ✅
  As a player, I want each set to show a clear status, so I know whether I still need to act.
  - Statuses: **Open** (deadline in future, not yet submitted), **Submitted** (I have saved picks, still before deadline), **Closed** (deadline passed), **Upcoming** (not yet available, e.g. a later playoff round).
  - Status is visually distinct (colour-coded pill + accent).

- **US-1.3 — Deadline enforcement** ✅
  As a player, I want a set to lock at its deadline, so predictions can't be changed after games start.
  - Before the deadline: the set is editable; saving is allowed and re-saving updates the picks.
  - After the deadline: the set is read-only; the form is shown but not submittable.

- **US-1.4 — Edit until deadline** ✅
  As a player, I want to revise a submitted set until its deadline, so I can change my mind.
  - A submitted-but-open set reopens in edit mode with my current picks pre-filled.

- **US-1.5 — Upcoming sets are gated** ✅
  As a player, I want later playoff rounds to be unavailable until their matchups exist, so I don't predict blind.
  - Rounds 2, Conference Finals, and Stanley Cup Final are shown but disabled/locked until unlocked by the admin.

- **US-1.6 — Configurable deadlines** 🔜❓
  As an admin, I want to set each set's deadline, so the schedule matches the real NHL calendar.
  - Prototype uses hard-coded sample deadlines. ❓ Should all four before-season sets share one deadline (opening night) or be staggered?

## Epic 2 — Before-the-season predictions

Split into four separate pages/sets.

- **US-2.1 — Cup champion** ✅
  As a player, I want to pick the Stanley Cup winner before the season, so it counts as my season-opening champion call.
  - Single team chosen from all 32 (grouped by division).

- **US-2.2 — Presidents' Trophy** ✅
  As a player, I want to pick the team with the best regular-season record.
  - Single team chosen from all 32.

- **US-2.3 — Division picks: playoff teams** ✅
  As a player, I want to choose which teams make the playoffs, per division, so the field reflects both conferences.
  - Teams are selected per division.
  - **Validation:** each conference must total exactly **8** teams across its two divisions, in a **4/4** or **5/3** split (each division 3–5).
  - Selection is capped live: a division won't accept a 6th team, a conference won't accept a 9th.
  - The set cannot be submitted until both conferences are valid; a clear message explains why when blocked.

- **US-2.4 — Division picks: division winners** ✅
  As a player, I want to pick one winner per division (4 total).
  - One team per division, chosen from that division's teams.

- **US-2.5 — Player awards (finalists)** ✅
  As a player, I want to pick 3 finalists for each individual trophy.
  - Five trophies: Hart, Norris, Vezina, Art Ross, Rocket Richard — 3 finalists each.
  - Input assists with suggestions appropriate to the trophy (skaters, defensemen for Norris, goalies for Vezina).
  - ❓ Should finalists be validated against a real player roster, or remain free text?

## Epic 3 — Playoff predictions

- **US-3.1 — Playoffs Cup re-pick** ✅
  As a player, I want to re-pick the Stanley Cup winner once the playoff field is set, so I get a second, better-informed call.
  - Single team from all 32; separate set with its own deadline.

- **US-3.2 — Per-round series predictions** ✅
  As a player, for each series in a round I want to pick the winner and in how many games, so my prediction captures both outcome and length.
  - Each series: choose the winning team and the number of games (4, 5, 6, or 7).
  - Round 1 contains 8 series (prototype seeds the matchups).

- **US-3.3 — Round unlocking** ✅🟡
  As a player, I want each later round to open only when its matchups are known.
  - Rounds 2 / Conference Finals / Stanley Cup Final are gated (US-1.5).
  - 🔜 Admin sets the real matchups per round (prototype seeds round 1 only).

- **US-3.4 — Matchup context labels** ✅
  As a player, I want each series labelled by its conference (or "Stanley Cup Final" for the last round), so I know what I'm predicting.
  - Series are grouped/labelled "Eastern Conference" / "Western Conference"; the final round is labelled "Stanley Cup Final".

## Epic 4 — Scoring & leaderboard

- **US-4.1 — Leaderboard** ✅🟡
  As a player, I want a leaderboard of all players with regular-season, playoff, and total points, so I can see who's winning.
  - Four columns: **Player**, **Regular**, **Playoff**, **Total**.
  - Sorted by **Total**, which is visually emphasised as the deciding column; leader is highlighted.
  - 🟡 Points are sample values in the prototype.

- **US-4.2 — Scoring rules** ❓🔜
  As an admin, I want defined point values for each prediction type, so results can be scored consistently.
  - Regular-season picks (playoff field, division winners, Presidents' Trophy, finalists, season-opening Cup) feed the **Regular** total.
  - Playoff-round picks feed the **Playoff** total.
  - ❓ **Open:** points per correct playoff team; per division winner; Presidents' Trophy; each finalist hit; Cup champion (season vs. playoff pick); series winner; exact-games bonus; partial credit rules; tie-breakers.

- **US-4.3 — Automatic scoring** 🔜
  As an admin, I want results recorded and scores computed automatically, so I don't maintain a spreadsheet.
  - Enter/import actual results; the app scores every player and updates the leaderboard.

## Epic 5 — Compare predictions

- **US-5.1 — Choose what to compare** ✅
  As a player, I want to select which prediction set to compare, so I can focus on one at a time.
  - Selector chips grouped into two rows: "Before the season" and "Playoffs".
  - The set's deadline is displayed.

- **US-5.2 — Side-by-side table** ✅
  As a player, I want all players' picks shown side by side, so I can compare them at a glance.
  - Player names form the table heading row (one column per player); my column is highlighted.
  - Each prediction category is a labelled row with one cell per player.

- **US-5.3 — Division comparison granularity** ✅
  As a player, I want division picks broken out per division, so the table is readable.
  - One row per division for the playoff-team picks, with that division's winner in a row directly beneath it.

- **US-5.4 — Consistent value formatting** ✅
  As a player, I want pick values shown consistently, so the table is easy to scan.
  - Team-abbreviation pick values use a single tag style (playoff teams, division winners, series winners).
  - Full team names and abbreviations that merely label a row (e.g. the matchup title) are plain text, not tags.

## Epic 6 — App shell, navigation & platform

- **US-6.1 — Bottom navigation** ✅
  As a player, I want persistent bottom navigation between Predict, Leaderboard, and Compare.
  - Three tabs; the active tab is highlighted.

- **US-6.2 — Fixed app shell** ✅
  As a player, I want the header and bottom nav to stay pinned while only the content scrolls, so navigation is always reachable.
  - Viewport-height frame; header and nav fixed; middle content scrolls.

- **US-6.3 — Mobile-first, dark mode** ✅
  As a player, I want a mobile-optimised dark interface, since I'll mostly use my phone.
  - Single-column, phone-width layout; dark theme; other devices are a later concern.

## Epic 7 — Identity & participants

- **US-7.1 — Single-player context** ✅
  As a player, I want the app to act as me and only let me edit my own picks.
  - Header shows the current player's name and the season ("NHL 2026–27"). No player switcher.

- **US-7.2 — Accounts & authentication** 🔜
  As a player, I want to log in as myself, so my picks are private to me until deadlines and attributable afterward.
  - ❓ Auth mechanism (the group already runs self-hosted identity tooling).

## Epic 8 — Persistence, backend & administration

- **US-8.1 — Persistent, shared storage** 🔜
  As a player, I want my picks saved and everyone's picks visible after deadlines, so the pool works across devices and people.
  - Prototype holds all state in memory (resets on refresh, not shared). Needs a backend/datastore + API.

- **US-8.2 — Season & matchup administration** 🔜
  As an admin, I want to create a season, set deadlines, enter real playoff matchups per round, and record results.

- **US-8.3 — Results ingestion** 🔜❓
  As an admin, I want results captured with minimal effort.
  - ❓ Manual entry vs. NHL data feed/API.

- **US-8.4 — Multi-season** 🔜
  As a player, I want a new pool each season while keeping history, replacing "one Excel file per season".

---

## Out of scope (for now)

- Real-time score updates during games.
- In-app messaging/chat between players.
- Public/ranked pools beyond the private group.
- Native mobile apps (mobile web only).
