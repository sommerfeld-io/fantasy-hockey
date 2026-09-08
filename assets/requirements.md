# Fantasy Hockey — Requirements (Epics & User Stories)

A private, season-long NHL prediction game for a small fixed group of friends (Basti, Sadl, Tobbi), delivered as a mobile-first web app. It replaces a manually maintained Excel workbook (one file per season) whose hand-calculated scoring and lack of deadline/name-typo enforcement had already caused a real, uncorrected scoring error. Players enter predictions before the season and throughout the playoffs; the app scores them automatically and ranks players on a standings view.

This document is the requirements baseline to build the app **from scratch**. It consolidates and reconciles two prior sources:

- `assets/clickdummy/requirements.md` and the working click-dummy prototype (`assets/clickdummy/fantasy-hockey-prototype.jsx`, `assets/clickdummy/faceoff-pool-source/`, `assets/clickdummy/faceoff-pool-dist/`) — an interactive React mockup ("Face-Off Pool") that defines the actual feature set and UI/UX behavior to build.
- `_bmad-output/` — earlier planning (brainstorm, brief, PRD, epic breakdown) that answers several questions the click-dummy leaves open (scoring point values, authentication mechanism, who can administer the game).

**Where the two sources conflicted, the click-dummy wins** — see [Conflicts resolved in favor of the click-dummy](#conflicts-resolved-in-favor-of-the-click-dummy) at the end of this document for the specific list. No architecture, technology, or implementation detail (database, session mechanism, package layout, deployment target, hosting, etc.) is included here — that is covered separately.

---

## 1. Actors

- **Player** — a participant in the pool. Logs in, makes their own predictions, sees standings, and compares everyone's picks. Today's fixed set of players: Basti, Sadl, Tobbi — no registration or invite flow; adding a fourth player is a deliberate, out-of-band change, not a feature.
- **Player is the only role the app has.** There is no admin/commissioner login and no in-app management action of any kind — real-world results, deadlines, and playoff matchups are maintained outside the app entirely, by directly editing `fantasy-hockey.yml` (see [Results, deadlines & playoff matchups](#results-deadlines--playoff-matchups)). This resolves the click-dummy's open "Commissioner/Admin (implied, not yet a real role)" question further than the click-dummy itself did: not just "no separate role," but no in-app role or screen for it at all.

## 2. Domain glossary

- **Season** — one NHL season's instance of the game, from before the regular season through the Stanley Cup Final.
- **Conference** — Eastern or Western.
- **Division** — Atlantic, Metropolitan (East); Central, Pacific (West). 8 teams each; 32 teams total.
- **Playoff field** — the teams predicted (or, later, confirmed) to reach the playoffs, per division.
- **Division winner** — the team predicted/confirmed to finish first in a division.
- **Presidents' Trophy** — best regular-season record in the league.
- **Stanley Cup** — league champion.
- **Cup champion pick / Playoffs Cup re-pick** — the two Stanley Cup winner predictions: the season-opening pick (before the season) and the re-pick made once the playoff field is known (before Round 1).
- **Awards (individual)** — Hart (MVP), Norris (defenseman), Vezina (goalie), Art Ross (points leader), Rocket Richard (goals leader). Each prediction is a set of 3 finalists (more than 3 when a real-world tie extends past 3rd place).
- **Series** — a best-of-seven playoff matchup between two teams, predicted/recorded as a winner plus an exact game count (4–7 games).
- **Prediction set** — one deadline-bound group of predictions (e.g. "Division picks", "Playoff round 1"). Every prediction belongs to exactly one set and has exactly one deadline.
- **Result** — the real-world outcome recorded against a prediction set once it's known, used to compute points.
- **Standings** — the always-live, computed view of every player's points: Regular, Playoff, and Total.

---

## Epic 1 — Authentication & identity

🆕 New — the click-dummy has no login screen; it hard-codes the current player as Basti. A real, shared app needs every player to log in as themselves and only ever edit their own picks. The stories below are deliberately behavior-only, with no screen layout or wording prescribed — the click-dummy doesn't cover this flow yet, so its UI/UX is left for a dedicated design pass later, not decided here.

- **US-1.1 — Request a login code**
    As a player, I want to log in by requesting a one-time code by email, so that I don't need to remember or manage a password.
    - A visitor submits an email address; the response is identical regardless of whether it matches one of the three players — no way to tell from the response whether an email is registered.
    - If the email matches a player, a 6-digit numeric code is generated and emailed to them.

- **US-1.2 — Enter the login code**
    As a player who requested a code, I want to submit it and be logged in, so that I can start using the app.
    - A code is valid for 10 minutes or until used once, whichever comes first. Requesting a new code does not invalidate an earlier still-valid one.
    - A wrong, expired, or already-used code is rejected without revealing which of the three applies.

- **US-1.3 — Stay logged in, then time out**
    As a player, I want my session to stay active while I'm using the app and expire when I've stepped away, so that a shared or left-open device doesn't stay logged in forever.
    - A session ends after 30 minutes of inactivity; any request resets the countdown.

- **US-1.4 — Log out**
    As a player, I want to log out, so that my session ends immediately on this device.

- **US-1.5 — Single-player context** [Prototype-validated]
    As a player, I want the app to act as me and only let me edit my own picks, so that there's no confusion about whose predictions I'm looking at.
    - The header shows the current player's name and the season (e.g. "NHL 2026–27"). No player switcher.

## Epic 2 — Prediction sets, phases & deadlines

🖥️ Prototype-validated — matches the click-dummy's Predict tab feature-for-feature.

Predictions are organized into deadline-bound *sets*, grouped into two phases: **Before the season** and **Playoffs**.

- **US-2.1 — Browse prediction sets**
    As a player, I want to see all prediction sets grouped by phase, so I know what I can predict and when.
    - Two sections: "Before the season" and "Playoffs". Each set shows a title, a short subtitle, its deadline (date + time), and a relative countdown (e.g. "in 27 days").

- **US-2.2 — Set lifecycle & status**
    As a player, I want each set to show a clear status, so I know whether I still need to act.
    - Statuses: **Open** (deadline in future, not yet submitted), **Submitted** (I have saved picks, still before deadline), **Closed** (deadline passed), **Upcoming** (not yet available — a later playoff round whose matchups aren't set yet).

- **US-2.3 — Deadline enforcement**
    As a player, I want a set to lock at its deadline, so predictions can't be changed after games start.
    - Before the deadline: editable, and re-saving updates the picks. After the deadline: fully read-only, with no grace period and no override — not even by the player who made the pick.

- **US-2.4 — Edit until deadline**
    As a player, I want to revise a submitted set until its deadline, so I can change my mind.
    - A submitted-but-open set reopens in edit mode with my current picks pre-filled. Any subset of a set's fields can be saved independently — there's no all-or-nothing submission (a Series pick's winner and game count are the one exception: they save together as a unit).

- **US-2.5 — Upcoming rounds are gated**
    As a player, I want later playoff rounds to be unavailable until their matchups exist, so I don't predict blind.
    - Round 2, the Conference Finals, and the Stanley Cup Final are shown but disabled/locked until unlocked (see [Results, deadlines & playoff matchups](#results-deadlines--playoff-matchups)).

- **US-2.6 — A required pick left empty scores zero**
    As a player, I want a prediction I never got around to filling in to simply score zero, so that one missed pick doesn't block me from predicting anything later.
    - Does not block later deadlines; the set is just scored zero for that item.

- **US-2.7 — Deadlines live in the data file**
    Each set's deadline value comes straight from `fantasy-hockey.yml` — there is no in-app settings page for it (see [Results, deadlines & playoff matchups](#results-deadlines--playoff-matchups)). In today's click-dummy sample data, all four before-the-season sets share one deadline and the Playoffs Cup re-pick has its own deadline a few days before Round 1's — this is an operational choice made per season, not a fixed rule.

## Epic 3 — Before-the-season predictions

🖥️ Prototype-validated. Four separate sets/forms.

- **US-3.1 — Cup champion (season-opening pick)**
    As a player, I want to pick the Stanley Cup winner before the season, so it counts as my season-opening champion call.
    - Single team chosen from all 32, grouped by division.

- **US-3.2 — Presidents' Trophy**
    As a player, I want to pick the team with the best regular-season record.
    - Single team chosen from all 32.

- **US-3.3 — Division picks: playoff teams**
    As a player, I want to choose which teams make the playoffs, per division, so the field reflects both conferences.
    - Teams selected per division. Each conference must total exactly 8 teams across its two divisions, in a 4/4 or 5/3 split (each division 3–5 teams).
    - Selection is capped live: a division won't accept a 6th team, a conference won't accept a 9th team.
    - The set can't be submitted until both conferences are valid; a clear message explains what's missing.

- **US-3.4 — Division picks: division winners**
    As a player, I want to pick one winner per division (4 total), chosen from that division's teams.

- **US-3.5 — Player awards (finalists)**
    As a player, I want to pick 3 finalists for each individual award.
    - Five awards: Hart, Norris, Vezina, Art Ross, Rocket Richard — 3 finalists each (more than 3 when a real tie extends past 3rd place — recorded directly in the data file, see [Results, deadlines & playoff matchups](#results-deadlines--playoff-matchups)).
    - Name entry offers autocomplete against the season's known player list (skaters for Hart/Art Ross/Rocket, defensemen for Norris, goalies for Vezina) and rejects a name that doesn't match — this resolves the click-dummy's open question in favor of validated entry, so a typo can never silently fail to score (the exact failure mode of the old spreadsheet).

## Epic 4 — Playoff predictions

🖥️ Prototype-validated. Round unlocking (US-4.3) additionally depends on that round's matchups being recorded in the data file — see [Results, deadlines & playoff matchups](#results-deadlines--playoff-matchups).

- **US-4.1 — Playoffs Cup re-pick**
    As a player, I want to re-pick the Stanley Cup winner once the playoff field is set, so I get a second, better-informed call.
    - Single team from all 32; its own separate set with its own deadline (shortly before Round 1's).

- **US-4.2 — Per-round series predictions**
    As a player, for each series in a round I want to pick the winner and how many games it takes, so my prediction captures both outcome and length.
    - Each series: pick the winning team and the game count (4, 5, 6, or 7). Winner and game-count picks are shown and interacted with the same way — neither is visually "more important."
    - Round 1 contains 8 series.

- **US-4.3 — Round unlocking**
    As a player, I want each later round to open only once its matchups are known, so I'm predicting real matchups, not guesses.
    - Rounds 2, Conference Finals, and the Stanley Cup Final stay gated ([US-2.5](#epic-2--prediction-sets-phases--deadlines)) until that round's real matchups are recorded directly in `fantasy-hockey.yml`.

- **US-4.4 — Matchup context labels**
    As a player, I want each series labelled by its conference (or "Stanley Cup Final" for the last round), so I know what I'm predicting.
    - Series are grouped/labelled "Eastern Conference" / "Western Conference"; the final round is labelled "Stanley Cup Final".

## Results, deadlines & playoff matchups

Real-world results (playoff series outcomes, award winners/finalists, division winners, the Presidents' Trophy, the Stanley Cup winner), each prediction set's deadline, and each playoff round's real matchups are **not entered through the app at all — there is no management or admin UI anywhere in this design.** They are maintained by directly editing `fantasy-hockey.yml`, out of band.

Every other epic in this document only *reads* that data: deadline enforcement ([US-2.3](#epic-2--prediction-sets-phases--deadlines)), round unlocking ([US-4.3](#epic-4--playoff-predictions)), and scoring ([US-6.2](#epic-6--scoring--standings)) all react to whatever is currently recorded in the file — nothing in the app itself ever writes to those parts of it. This is a deliberate simplification for a 3-person private pool, not a gap to fill later with a hidden page.

## Epic 5 — Deadline reminders

🆕 New — not present in the click-dummy at all. Behavior-only, like Epic 1: no screen or interaction detail is prescribed here.

- **US-5.1 — Send a reminder email**
    As a player, I want to trigger a reminder email for a specific prediction set that hasn't closed yet, so whoever hasn't finished predicting gets a nudge.
    - Emails only the players who haven't completed every prediction under that set — a player who's already finished isn't emailed, no matter how many times the reminder is triggered.
    - Plain, single-purpose content: an "action required until {deadline}" nudge, nothing else (no result summaries, no other content).
    - Can be triggered any number of times before the set closes; no automatic/scheduled reminder exists.

## Epic 6 — Scoring & standings

🖥️ Prototype-validated for the standings display; 🆕 New for the actual scoring rules, which the click-dummy explicitly leaves open and this document now resolves.

- **US-6.1 — Standings**
    As a player, I want a standings view of all players with Regular, Playoff, and Total points, so I can see who's winning.
    - Three columns: Player, Regular, Playoff, and a visually emphasized Total (the deciding column, ties broken by nothing — see below).
    - Always reflects the latest scoring computation live; there's no separate "in-progress"/projected tier.
    - Players with an equal Total share the same rank; no tiebreaker of any kind is applied.

- **US-6.2 — Automatic scoring**
    As a player, I want results and scores computed automatically from what's recorded in `fantasy-hockey.yml`, so nobody ever has to do the math by hand (the exact failure that motivated replacing the old spreadsheet — a correct pick going uncredited by a broken formula).
    - Point values, resolved from earlier planning (the click-dummy leaves these open):

      | Prediction                         | Points                                                              | Bucket  |
      |------------------------------------|---------------------------------------------------------------------|---------|
      | Award finalist (each of 3 names)   | 5 per name found anywhere in the actual top-3 (ties expand the set) | Regular |
      | Team makes the playoffs            | 5                                                                   | Regular |
      | Division winner                    | 15 (replaces, not adds to, the 5-point mark)                        | Regular |
      | Presidents' Trophy pick            | 20                                                                  | Regular |
      | Cup champion (season-opening pick) | 20                                                                  | Regular |
      | Playoffs Cup re-pick               | 20                                                                  | Playoff |
      | Round 1 series                     | 15 correct winner / 25 exact result (replaces, not additive)        | Playoff |
      | Round 2 series                     | 25 / 35                                                             | Playoff |
      | Conference Finals series           | 30 / 45                                                             | Playoff |
      | Stanley Cup Final series           | 30 / 50                                                             | Playoff |

    - All 5 awards score identically, against whatever is recorded in the data file for that award (no award is exempt).
    - A required pick left empty at its deadline scores zero for that item ([US-2.6](#epic-2--prediction-sets-phases--deadlines)).

## Epic 7 — Compare predictions

🖥️ Prototype-validated. Everyone's picks for a chosen set are always visible side by side — there is no deadline-based hiding of other players' picks (see [Conflicts resolved in favor of the click-dummy](#conflicts-resolved-in-favor-of-the-click-dummy)).

- **US-7.1 — Choose what to compare**
    As a player, I want to select which prediction set to compare, so I can focus on one at a time.
    - Selector grouped into two rows: "Before the season" and "Playoffs". The chosen set's deadline is displayed.

- **US-7.2 — Side-by-side table**
    As a player, I want all players' picks shown side by side, so I can compare them at a glance.
    - Player names head the table (one column per player); my own column is visually distinguished.

- **US-7.3 — Division comparison granularity**
    As a player, I want division picks broken out per division, so the table stays readable.
    - One row per division for the playoff-team picks, with that division's winner directly beneath it.

- **US-7.4 — Consistent value formatting**
    As a player, I want pick values shown consistently, so the table is easy to scan.
    - Team-abbreviation values (playoff teams, division winners, series winners) render the same way everywhere; a value nobody has entered yet shows as an empty placeholder rather than blank space.

## Epic 8 — App shell, navigation & platform

🖥️ Prototype-validated.

- **US-8.1 — Bottom navigation**
    As a player, I want persistent navigation between Predict, Standings, and Compare, so I can always get where I need to go.
    - Three destinations; the active one is clearly highlighted.

- **US-8.2 — Fixed app shell**
    As a player, I want the header and navigation to stay put while only the content scrolls, so navigation is always reachable.

- **US-8.3 — Mobile-first, dark theme**
    As a player, I want a mobile-optimized, dark interface, since I'll mostly use my phone.
    - Single-column, phone-width layout by default; other device sizes are a later concern. Dark theme only, no light mode.

## Epic 9 — Persistence, backend & multi-season

🆕 New — the click-dummy holds all state in memory (resets on refresh, nothing shared between players); it's explicitly a static mockup.

- **US-9.1 — Persistent, shared storage**
    As a player, I want my picks saved and everyone's picks visible to everyone under the same rules, so the pool actually works across devices and people instead of resetting on refresh.

- **US-9.2 — A season's canonical team and player lists**
    As a player, I want the current season's teams (and, for award finalists, a working list of players) available for selection/autocomplete, so predictions and results use the same validated names everywhere ([US-3.5](#epic-3--before-the-season-predictions); results recorded directly in the data file use this same list too).

- **US-9.3 — A new pool each season, with history kept**
    As a player, I want a new pool each season while keeping past seasons' data, so the app fully replaces "one Excel file per season" instead of just replacing the current one.
    - Out of scope for the very first version: a season-selector or any UI built around multiple seasons (e.g. a Hall of Fame view of past champions) — those come once a second season's history actually exists to show.

---

## Out of scope (for now)

- A management/admin UI for entering real-world results, deadlines, or playoff matchups — see [Results, deadlines & playoff matchups](#results-deadlines--playoff-matchups); these are maintained by directly editing `fantasy-hockey.yml`, not through the app.
- Any second role beyond the single player identity — no separate admin/scorekeeper account, no spectator/read-only account, and no approval step of any kind, since results aren't entered through the app at all.
- Registration or an invite/join flow — the player list changes only by an out-of-band change, never through the app.
- Real-time score updates during games.
- In-app messaging/chat between players.
- Public/ranked pools beyond the private group.
- Native mobile apps (mobile web only).
- A season-selector, multi-season views, or a Hall-of-Fame-style history page (see [US-9.3](#epic-9--persistence-backend--multi-season)).

## Conflicts resolved in favor of the click-dummy

Per this document's own instructions, the click-dummy is authoritative wherever it disagreed with earlier `_bmad-output` planning. Recorded here for traceability before that planning is retired:

- **Compare visibility.** Earlier planning specified that another player's picks under a set stay hidden until that set's deadline closes. The click-dummy's Compare tab shows every player's picks for any chosen set at any time, with no such gating — its sample data even shows "submitted" picks for sets whose deadline hasn't passed yet. This document follows the click-dummy: **Compare is always fully visible, for every player, at any time** ([Epic 7](#epic-7--compare-predictions)).
- **Cup-pick scoring bucket.** Earlier planning put both the season-opening Cup pick and the playoffs Cup re-pick in the Playoff points bucket. The click-dummy explicitly states the season-opening Cup pick feeds the **Regular** total. This document follows the click-dummy: the season-opening pick is Regular, the playoffs re-pick is Playoff ([US-6.2](#epic-6--scoring--standings)).
- **Series length notation.** Earlier planning specified series results as win-loss notation (e.g. "4-2"). The click-dummy records and displays a series purely as a game count (4, 5, 6, or 7) with a separately identified winner. This document follows the click-dummy — predictions use this notation ([US-4.2](#epic-4--playoff-predictions)), and results recorded in the data file use the same notation.
- **Admin/commissioner role.** The click-dummy's own domain glossary calls this "implied, not yet a real role" and leaves it open. This document resolves it further than either source: there is no in-app role or screen for it at all — results, deadlines, and matchups are maintained by directly editing `fantasy-hockey.yml` outside the app ([Actors](#1-actors), [Results, deadlines & playoff matchups](#results-deadlines--playoff-matchups)).
- **Finalist-name validation.** The click-dummy's player-award inputs offer free-text suggestions without enforcing a match; its own text flags this as an open question. Earlier planning resolved it (validated entry, non-matching names rejected) in a way that doesn't contradict anything the click-dummy asserts as final — carried forward as the resolution ([US-3.5](#epic-3--before-the-season-predictions)).
