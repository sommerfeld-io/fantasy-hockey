---
title: Fantasy Hockey
status: final
created: 2026-09-14
updated: 2026-09-14
---

# PRD: Fantasy Hockey
*Working title — confirm.*

## 0. Document Purpose

This PRD is for the builder (also the sole PM, architect, and one of the three players) and for the downstream BMad workflows that consume it next — UX, architecture, epics/stories. It builds on, and does not duplicate, `assets/requirements.md` (the consolidated epics/user-story baseline reconciled from the click-dummy prototype at `assets/clickdummy/` and an earlier, now-retired BMad planning pass) and `assets/clickdummy/fantasy-hockey-prototype.jsx` (the interactive mockup that validated most of the UI/UX behavior described here). It is structured around a Glossary-anchored vocabulary, features grouped with FRs nested and globally numbered, and inline `[ASSUMPTION]` tags indexed in §9 wherever this PRD pass inferred something `assets/requirements.md` left implicit.

That reconciliation was governed by one rule — where the click-dummy and the earlier BMad pass disagreed, the click-dummy won; §4/§5 below carry forward only the resulting outcomes, and `addendum.md`'s "Conflict resolutions" section preserves the full rationale for traceability now that the source document is slated for deletion. Technology and implementation choices are explicitly out of scope here — the next pass, `bmad-architecture`, is fed by `assets/architecture-requirements.md`.

## 1. Vision

Fantasy Hockey is a private, season-long NHL prediction pool for three friends — Basti, Sadl, and Tobbi — delivered as a mobile-first, dark-themed web app. It replaces a manually maintained Excel workbook (one file per NHL season) whose hand-calculated scoring and lack of deadline or name-typo enforcement had already caused a real, uncorrected scoring error in a past season.

Each player predicts outcomes before the season starts (Cup winner, Presidents' Trophy, playoff field, division winners, individual awards) and throughout the playoffs (a re-pick and per-series winner/length calls), all against fixed deadlines. The app scores every prediction automatically the moment results are recorded, and keeps a live standings view so nobody has to do the math — or gets it wrong — by hand again.

Nothing about this product needs to scale past three people or grow an admin layer: results, deadlines, and matchups are maintained directly in a data file by whoever is already running the pool, out of band. The value is entirely in getting the player-facing loop — predict, wait, get scored, compare, gloat — right, reliably, every season.

## 2. Target User

### 2.1 Jobs To Be Done

- **Functional:** Enter my predictions before each deadline, without needing to remember rules or do arithmetic; see where I stand at any moment; see everyone else's picks to compare and argue about them.
- **Social:** Keep a long-running, friendly competition with the same two friends alive, season after season, without the friction (or errors) of a shared spreadsheet.
- **Emotional:** Trust that a correct pick will always be credited correctly — the specific thing that broke the old spreadsheet and this app exists to fix.
- **Contextual:** Mostly used in short bursts on a phone — checking standings after a game, filling in a form before a deadline hits — not at a desk.

### 2.2 Non-Users (v1)

- Anyone outside the three named players (Basti, Sadl, Tobbi). There is no registration or invite flow; adding a fourth player is a deliberate, out-of-band change to the data file, not a product feature.
- Anyone wanting an admin/commissioner role in-app. There is no such role or screen — see §5 Non-Goals.

### 2.3 Key User Journeys

*Lighter form — single-line JTBD-style journeys per key flow, given the single "Player" role and behavior already validated by the click-dummy prototype.*

- **UJ-1.** Basti, on his phone the morning a deadline lands, opens Predict, fills in his still-open picks, and saves — confident that whatever he leaves blank simply scores zero rather than blocking him.
- **UJ-2.** Sadl, a few minutes after receiving a login-code email, taps in, is logged in, and lands straight on his own picks — never anyone else's.
- **UJ-3.** Tobbi, mid-playoffs, opens Compare, picks "Round 1," and scrolls the side-by-side table to see who called the upset he just watched.
- **UJ-4.** *[DEFERRED — not in MVP; see §4.5, FR-22]* Basti, after a slow week, gets a reminder email for a set he hasn't finished, taps through, and completes the missing picks before it locks.
- **UJ-5.** Sadl, right after Round 1 results are entered (and the app restarted to pick them up), opens Leaderboard to see Regular, Playoff, and Total points recomputed, and where he now ranks.
- **UJ-6.** Whoever is running the pool that season edits `fantasy-hockey.yml` directly to record a just-finished series result or open the next playoff round — no in-app action, no screen.

## 3. Glossary

- **Player** — the app's only role/actor: a participant in the pool who logs in, makes their own predictions, and views the Leaderboard and comparisons. Today's fixed set: Basti, Sadl, Tobbi. Not to be confused with **NHL Player** below.
- **NHL Player** — a real-world NHL athlete (skater, defenseman, or goalie) eligible as an individual-award finalist. Distinct from **Player** — the pool of NHL Players is large and maintained via the season's canonical list (FR-33), never logged in or otherwise an app actor.
- **Season** — one NHL season's instance of the game, from before the regular season through the Stanley Cup Final.
- **Conference** — Eastern or Western.
- **Division** — Atlantic, Metropolitan (Eastern); Central, Pacific (Western). 8 teams each; 32 teams total.
- **Playoff field** — the teams predicted (or, later, confirmed) to reach the playoffs, per Division.
- **Division winner** — the team predicted/confirmed to finish first in a Division.
- **Presidents' Trophy** — best regular-season record in the league.
- **Stanley Cup** — league champion.
- **Cup champion pick** — the season-opening Stanley Cup winner prediction, made before the season.
- **Playoffs Cup pick** — the second Stanley Cup winner prediction, made once the Playoff field is known, before Round 1. (Not to be confused with the season-opening **Cup champion pick** above — the term "Playoffs Cup pick" follows the UX spines' click-dummy-exact terminology; see the naming note below.)
- **Player awards** — Hart (MVP), Norris (defenseman), Vezina (goalie), Art Ross (points leader), Rocket Richard (goals leader). Each is predicted as a set of 3 finalists (more than 3 only when a real-world tie extends past 3rd place).
- **Series** — a best-of-seven Playoff matchup between two teams, predicted/recorded as a winning team plus an exact game count (4–7 games).
- **Prediction set** — one deadline-bound group of predictions (e.g. "Division picks", "Playoff Round 1"). Every prediction belongs to exactly one set and has exactly one deadline.
- **Result** — the real-world outcome recorded against a Prediction set once it's known, used to compute scoring.
- **Leaderboard** — the always-live, computed view of every Player's points: Regular, Playoff, and Total. (Named "Standings" in an earlier draft of this PRD; renamed to match the UX spines' click-dummy-exact terminology — see the naming note below.)
- **`fantasy-hockey.yml`** — the single data file, edited directly and out of band by whoever runs the pool, that holds all Results, deadlines, and Playoff matchups. The app treats those sections as read-only: no in-app UI creates or changes them, and the app picks up a hand edit only after a restart.

## 4. Features

### 4.1 Authentication & Session
**Description:** A Player logs in by requesting a one-time 6-digit code by email and submitting it; no password to remember or manage. Sessions time out after inactivity so a shared or left-open device doesn't stay logged in forever. Behavior-only — no screen layout or wording is prescribed here; the click-dummy hard-codes the current player (as "Basti") and doesn't cover the login flow itself, so its UI/UX is a later design pass. The single-player context behavior in FR-5, however, **is** prototype-validated — the click-dummy already demonstrates it, just without a login step in front of it. Realizes UJ-2.

**Functional Requirements:**

#### FR-1: Request a login code
A visitor can request a login code by submitting an email address.

**Consequences (testable):**
- The response is identical regardless of whether the submitted email matches one of the three Players — nothing about the response reveals whether an email is registered.
- If the email matches a Player, a 6-digit numeric code is generated and emailed to them.

#### FR-2: Enter the login code
A Player who requested a code can submit it and be logged in.

**Consequences (testable):**
- A code is valid for 10 minutes or until used once, whichever comes first.
- Requesting a new code does not invalidate an earlier still-valid one.
- A wrong, expired, or already-used code is rejected with an identical response in all three cases — nothing reveals which applies.

#### FR-3: Session timeout
A Player's session stays active while in use and expires after a period of inactivity.

**Consequences (testable):**
- A session ends after 30 minutes of inactivity.
- Any request resets the inactivity countdown.

#### FR-4: Log out
A logged-in Player can log out, ending their session immediately on that device.

#### FR-5: Single-player context
*[Prototype-validated]* The app always acts as the logged-in Player and only ever lets them edit their own picks. Realizes UJ-2.

**Consequences (testable):**
- The header shows the current Player's name and the season (e.g. "NHL 2026–27").
- There is no Player switcher anywhere in the app.

### 4.2 Prediction Sets & Deadlines
**Description:** Predictions are organized into deadline-bound Prediction sets, grouped into two phases — "Before the season" and "Playoffs." This feature governs how sets are browsed, their lifecycle, and deadline enforcement; §4.3 and §4.4 cover what's actually predicted in each set. Prototype-validated against the click-dummy's Predict tab. Realizes UJ-1.

**Functional Requirements:**

#### FR-6: Browse Prediction sets
A Player can see all Prediction sets grouped by phase.

**Consequences (testable):**
- Two sections: "Before the season" and "Playoffs."
- Each set shows a title, a short subtitle, its deadline (date + time, in the **Europe/Berlin** timezone shared by all three Players — not per-Player configurable), and a relative countdown (e.g. "in 27 days").

#### FR-7: Set lifecycle & status
A Player can see a clear status for each Prediction set.

**Consequences (testable):**
- Statuses are exactly: **Open** (deadline in future, nothing saved yet), **Submitted** (picks saved, still before deadline), **Closed** (deadline passed), **Upcoming** (not yet available — a later Playoff round whose matchups aren't set yet).

#### FR-8: Deadline enforcement
A Prediction set locks automatically at its deadline.

**Consequences (testable):**
- Before the deadline: the set is editable, and re-saving updates the picks.
- After the deadline: the set is fully read-only, with no grace period and no override for any Player, including whoever made the pick.

#### FR-9: Edit until deadline
A Player can revise a submitted set until its deadline.

**Consequences (testable):**
- A submitted-but-open set reopens in edit mode with the Player's current picks pre-filled.
- Any subset of a set's fields can be saved independently — there is no all-or-nothing submission, except a Series pick's winner and game count, which save together as one unit (see FR-19).

#### FR-10: Upcoming rounds are gated
A later Playoff round is shown but disabled/locked until its matchups exist in `fantasy-hockey.yml` (see FR-20).

**Consequences (testable):**
- Applies to Round 2, the Conference Finals, and the Stanley Cup Final.

#### FR-11: An empty required pick scores zero
A prediction left empty at its deadline scores zero for that item, and does not block any other prediction.

**Consequences (testable):**
- Does not block later deadlines or other items within the same set.

#### FR-12: Deadlines are data-driven
Every Prediction set's deadline value is read directly from `fantasy-hockey.yml`; there is no in-app settings page for it.

**Consequences (testable):**
- Changing a deadline is a direct edit to `fantasy-hockey.yml`, never an in-app action.
- **Not a fixed rule:** in the click-dummy's sample data, all four before-the-season sets happen to share one deadline and the Playoffs Cup pick has its own deadline a few days before Round 1's — this is an operational choice made per season, not a system constraint. The app must not assume or enforce that pattern (e.g. that before-the-season sets always share a deadline); each set's deadline is independent and can differ every season.

### 4.3 Before-the-Season Predictions
**Description:** Four separate Prediction sets/forms, all due before the regular season starts. Prototype-validated feature-for-feature against the click-dummy.

**Functional Requirements:**

#### FR-13: Cup champion (season-opening pick)
A Player can pick the Stanley Cup winner before the season, from all 32 teams grouped by Division.

#### FR-14: Presidents' Trophy pick
A Player can pick the team with the best regular-season record, from all 32 teams.

#### FR-15: Division picks — playoff teams
A Player can choose which teams make the Playoffs, per Division.

**Consequences (testable):**
- Each Conference must total exactly 8 teams across its two Divisions, in a 4/4 or 5/3 split (each Division holds 3–5 teams).
- Selection is capped live: a Division won't accept a 6th team; a Conference won't accept a 9th team.
- The set can't be submitted until both Conferences are valid, with a clear message explaining what's missing.

#### FR-16: Division picks — division winners
A Player can pick one winner per Division (4 total), chosen from that Division's own teams.

#### FR-17: Player awards (finalists)
A Player can pick 3 finalists for each of the five individual awards (Hart, Norris, Vezina, Art Ross, Rocket Richard).

**Consequences (testable):**
- Name entry offers autocomplete against the season's known NHL Player list (skaters for Hart/Art Ross/Rocket Richard, defensemen for Norris, goalies for Vezina — see FR-33) and rejects a name that doesn't match.
- A real-world tie extending past 3rd place is recorded directly in `fantasy-hockey.yml`, not entered by the Player.

### 4.4 Playoff Predictions
**Description:** Prototype-validated. Round unlocking (FR-20) additionally depends on that round's matchups being recorded in `fantasy-hockey.yml`.

**Functional Requirements:**

#### FR-18: Playoffs Cup pick
A Player can re-pick the Stanley Cup winner once the Playoff field is set, from all 32 teams, as its own Prediction set with its own deadline shortly before Round 1's.

#### FR-19: Per-round series predictions
For each Series in an unlocked round, a Player can pick the winning team and the exact game count (4, 5, 6, or 7).

**Consequences (testable):**
- Winner and game-count picks are shown and interacted with identically — neither is visually more prominent.
- Round 1 contains 8 Series.
- Winner and game-count save together as one unit (see FR-9).

#### FR-20: Round unlocking
A later Playoff round opens only once that round's real matchups are recorded in `fantasy-hockey.yml`. Realizes UJ-6.

**Consequences (testable):**
- Applies to Round 2, the Conference Finals, and the Stanley Cup Final (see FR-10).

#### FR-21: Matchup context labels
Each Series is labeled by its Conference ("Eastern Conference" / "Western Conference"), except the final round, labeled "Stanley Cup Final."

**Notes:** Real-world results, deadlines, and Playoff matchups are never entered through the app — there is no management or admin UI anywhere in this product (see §5 Non-Goals). Every FR above that reacts to `fantasy-hockey.yml` (deadline enforcement, round unlocking, scoring) only *reads* it; nothing in the app ever creates or changes that part of the file. (The app rewrites the whole file when it saves a pick or login code, carrying the hand-maintained sections over with their values unchanged — preserving their exact formatting is Story 7.4.)

### 4.5 Deadline Reminders — deferred, not in MVP
**Description:** [DEFERRED] Not present in the click-dummy at all — new for this rebuild, and explicitly removed from v1 scope during the UX pass: no trigger surface, button, or affordance for it exists anywhere in the UX spines (`_bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-14/EXPERIENCE.md`, "Deferred" section). Kept here as a specified-but-deferred capability for a future version, not deleted — see §6.2. Realizes UJ-4 (also deferred; see PRD §2.3).

**Functional Requirements:**

#### FR-22: Send a reminder email — deferred
A Player can trigger a reminder email for a specific Prediction set that hasn't closed yet.

**Consequences (testable):**
- Emails only the Players who haven't completed every prediction under that set; a Player who's already finished is never emailed, no matter how many times the reminder is triggered.
- Content is a plain, single-purpose "action required until {deadline}" nudge — no result summaries, no other content.
- Can be triggered any number of times before the set closes; there is no automatic or scheduled reminder.

### 4.6 Scoring & Leaderboard
**Description:** Leaderboard display is prototype-validated; the scoring rules themselves are new for this rebuild — the click-dummy explicitly left them open. Realizes UJ-5.

**Functional Requirements:**

#### FR-23: Leaderboard
A Player can see a Leaderboard view of all Players with Regular, Playoff, and Total points.

**Consequences (testable):**
- Four columns: Player, Regular, Playoff, and a visually emphasized Total (the deciding column).
- Always reflects the latest scoring computation — recomputed on every view, never cached; a newly saved pick shows immediately, a hand-recorded result after the app is restarted. There is no separate "in-progress"/projected tier.
- Players with an equal Total share the same rank; no tiebreaker of any kind is applied.

#### FR-24: Automatic scoring
The app computes results and scores automatically from what's recorded in `fantasy-hockey.yml`.

**Consequences (testable):**
- [ASSUMPTION: the point values below are the PM's own initial calibration for season 1 — new for this rebuild, since the click-dummy explicitly left scoring open — and may be revisited after a season's real results are in.] Point values:

  | Prediction                         | Points                                                              | Bucket  |
  |------------------------------------|---------------------------------------------------------------------|---------|
  | Award finalist (each of 3 names)   | 5 per name found anywhere in the actual top-3 (ties expand the set) | Regular |
  | Team makes the playoffs            | 5                                                                   | Regular |
  | Division winner                    | 15 (replaces, not adds to, the 5-point mark)                        | Regular |
  | Presidents' Trophy pick            | 20                                                                  | Regular |
  | Cup champion (season-opening pick) | 20                                                                  | Regular |
  | Playoffs Cup pick                  | 20                                                                  | Playoff |
  | Round 1 series                     | 15 correct winner / 25 exact result (replaces, not additive)        | Playoff |
  | Round 2 series                     | 25 / 35                                                             | Playoff |
  | Conference Finals series           | 30 / 45                                                             | Playoff |
  | Stanley Cup Final series           | 30 / 50                                                             | Playoff |

- All 5 awards score identically, against whatever is recorded in `fantasy-hockey.yml` for that award — no award is exempt.
- A required pick left empty at its deadline scores zero for that item (FR-11).

### 4.7 Compare Predictions
**Description:** Prototype-validated. Everyone's picks for a chosen set are always visible side by side — there is no deadline-based hiding of other Players' picks. Realizes UJ-3.

**Functional Requirements:**

#### FR-25: Choose what to compare
A Player can select which Prediction set to compare.

**Consequences (testable):**
- Selector grouped into two rows: "Before the season" and "Playoffs."
- The chosen set's deadline is displayed.

#### FR-26: Side-by-side table
A Player can see all Players' picks for the chosen set shown side by side.

**Consequences (testable):**
- Player names head the table, one column per Player.
- The current Player's own column is visually distinguished.

#### FR-27: Division comparison granularity
Division picks are broken out per Division in the comparison table.

**Consequences (testable):**
- One row per Division for the playoff-team picks, with that Division's winner directly beneath it.

#### FR-28: Consistent value formatting
Pick values render consistently everywhere in the comparison table.

**Consequences (testable):**
- Team-abbreviation values (Playoff teams, Division winners, Series winners) render the same way in every context.
- A value nobody has entered yet shows as an empty placeholder rather than blank space.

### 4.8 App Shell & Navigation
**Description:** Prototype-validated.

**Functional Requirements:**

#### FR-29: Bottom navigation
A Player can navigate between Predict, Leaderboard, and Compare via persistent bottom navigation, with the active destination clearly highlighted.

#### FR-30: Fixed app shell
The header and navigation stay fixed while only the content scrolls.

#### FR-31: Mobile-first, dark theme
The app is a single-column, phone-width layout by default, in a dark theme only — no light mode.

**Consequences (testable):**
- Other device sizes are a later concern, not required for v1.

### 4.9 Persistence & Season Data
**Description:** New for this rebuild — the click-dummy holds all state in memory (resets on refresh, nothing shared between Players); it's explicitly a static mockup.

**Functional Requirements:**

#### FR-32: Persistent, shared storage
Every Player's picks are saved and visible to everyone under the same rules, shared across devices and Players rather than resetting on refresh.

#### FR-33: A season's canonical team and NHL Player lists
The current season's 32 teams, and a working list of NHL Players for award-finalist autocomplete, are available for selection everywhere they're used (FR-17; results recorded in `fantasy-hockey.yml` use the same list).

**Consequences (testable):**
- Both lists follow the same out-of-band maintenance pattern as the rest of `fantasy-hockey.yml`'s data: maintained directly in the file by whoever runs the pool, not through any in-app UI. The NHL Player list is expected to need updates through a season (trades, injuries, call-ups) at the same maintainer's discretion — there is no automatic sync with an external NHL data source for v1.

#### FR-34: A new pool each season, with history kept
Each NHL season gets a new pool while past seasons' data is retained.

**Out of Scope:**
- A season-selector or any UI built around multiple seasons (e.g. a history/Hall-of-Fame view of past champions) — deferred until a second season's history actually exists to show. See §6.2.

## 5. Non-Goals (Explicit)

- **No admin/commissioner role or screen, anywhere.** Player is the only role the app has — no separate admin/scorekeeper account, no spectator/read-only account, and no approval step of any kind. Real-world results, each Prediction set's deadline, and each Playoff round's real matchups are maintained entirely by directly editing `fantasy-hockey.yml`, out of band. This resolves the click-dummy's own open "Commissioner/Admin (implied, not yet a real role)" question further than the click-dummy itself did.
- **No registration or invite flow.** The Player list changes only by an out-of-band edit, never through the app.
- **No real-time score updates during games.**
- **No in-app messaging or chat between Players.**
- **No public or ranked pools beyond this private group of three.**
- **No native mobile apps** — mobile web only.
- **No season-selector, multi-season views, or Hall-of-Fame-style history page** for v1 (see FR-34's Out of Scope note).
- **No light theme.**

## 6. MVP Scope

### 6.1 In Scope

[ASSUMPTION: `assets/requirements.md` is explicitly framed as "the requirements baseline to build the app from scratch," with no separate MVP-vs-later split beyond what it already marks out of scope — so this PRD treats all of §4 as the MVP, except FR-22, carved out during the UX pass (see below).]

- Everything in §4 except FR-22: authentication and session handling, all Prediction sets across both phases (before-the-season and Playoffs) with deadline enforcement, automatic scoring and the Leaderboard, Compare, the app shell, and persistent shared storage for a single season with prior-season history retained.

### 6.2 Out of Scope for MVP

- **Deadline reminder emails (FR-22, UJ-4)** — deferred, not deleted. Fully specified in §4.5 for a future version; carved out of v1 during the UX pass because no trigger surface exists in the click-dummy and none was designed fresh for it. Revisit once a trigger UI is actually wanted.
- Season-selector, multi-season browsing UI, or a Hall-of-Fame-style history page (FR-34) — deferred until a second season's history exists to show.
- Any admin/management UI (see §5) — permanently out of scope, not just deferred.
- A fourth Player, or any Player-list change made through the app — always an out-of-band, deliberate change.

## 7. Success Metrics

**Primary**
- **SM-1**: All three Players (Basti, Sadl, Tobbi) use the app through the entire NHL 2026–27 season, from before-the-season predictions through the Stanley Cup Final, without reverting to the old Excel workbook for any part of the pool. Validates FR-6–FR-21, FR-23–FR-24, FR-32 (FR-22 excluded — deferred, see §6.2).

**Secondary (leading indicator)**
- **SM-2**: All three Players complete every before-the-season Prediction set (FR-13–FR-17) in-app before that phase's deadline, without reverting to Excel — checkable at the first real deadline rather than only at season's end. An early miss here is the first honest signal that SM-1 is at risk. Validates FR-6–FR-9, FR-13–FR-17.

**Counter-metrics (do not optimize)**
- **SM-C1**: Do not optimize for feature breadth beyond what §4 already specifies. The three real failure modes this app exists to fix are scoring errors, deadline confusion, and name-typo scoring gaps (FR-8, FR-11, FR-17, FR-24) — added scope nobody asked for is not success, even if adoption (SM-1) looks fine.

[ASSUMPTION: given hobby/internal stakes for a 3-person private pool, this section is intentionally a single primary metric plus one counter-metric rather than a fuller quantitative breakdown.]

## 8. Open Questions

1. **Email delivery dependency (FR-1).** Login codes require an outbound email-sending mechanism. This PRD treats that as a real external dependency without specifying it further — it's an architecture concern, not a product one, but flagging it here since it's a hard MVP requirement, not a nice-to-have. (FR-22, the other email-dependent FR, is now deferred out of MVP — see §6.2 — so its email dependency is deferred along with it.) Local-dev handling (a disposable local email-capture server) and externally-configurable email settings are captured in `addendum.md` for `bmad-architecture`.
2. **Build-timeline risk against season start.** This PRD is dated 2026-09-14; the before-the-season Prediction sets (FR-13–FR-17) must close before the NHL 2026–27 season starts — roughly a month out, a tight window for a full solo rebuild (new auth, scoring engine, persistence, reminders — §0). No fallback is defined if v1 isn't ready in time (e.g., a temporary Excel fallback for before-the-season picks, cutting over for the Playoffs phase only). Not resolved in this PRD pass — surfaced for the builder to decide, since it's a feasibility/scheduling call rather than a product-requirements one.
3. **`fantasy-hockey.yml` schema ownership.** This PRD describes what the file is read for (deadlines, matchups, results, team/player lists) but not its shape. Deliberately deferred by the user to a later pass — `bmad-architecture`, fed by `assets/architecture-requirements.md` — flagged here only so it isn't dropped between documents.

*Resolved during PRD review:* deadlines/countdowns use a single shared **Europe/Berlin** timezone (folded into FR-6, no longer open). Concurrent- or malformed-write risk on `fantasy-hockey.yml` was raised and consciously deferred for MVP — accepted as low-risk with only 3 trusted participants; a future write-queuing mechanism is noted in `addendum.md` for a later architecture pass, not built now.

*Resolved during the UX pass (`bmad-ux`, `ux-fantasy-hockey-2026-09-14`):* three Glossary terms were renamed to match the UX spines' click-dummy-exact terminology, a deliberate choice made during UX Discovery to preserve the click-dummy's UI text unchanged rather than adapt it to this PRD's original wording — "Standings" → **Leaderboard**, "Awards (individual)" → **Player awards**, "Playoffs Cup re-pick" → **Playoffs Cup pick**. Applied throughout §1–§7 above. Also during that pass, FR-22 (deadline reminder emails) was carved out of MVP — see §6.2 and §4.5.

## 9. Assumptions Index

- §6.1 — MVP scope is all of §4 except FR-22 (FR-1–FR-21, FR-23–FR-34) as documented, since the source requirements doc frames itself as the from-scratch baseline with no separate MVP cut beyond what it already marks out of scope; FR-22 was subsequently carved out during the UX pass (see §8's UX-pass resolution note).
- §4.6, FR-24 — The scoring point-value table is the PM's own initial calibration for season 1, not a value carried forward with established rationale; may be revisited after a season's real results are in.
- §7 — Success Metrics kept to one primary + one leading secondary + one counter-metric, scaled to hobby/internal stakes for a 3-person private pool.
