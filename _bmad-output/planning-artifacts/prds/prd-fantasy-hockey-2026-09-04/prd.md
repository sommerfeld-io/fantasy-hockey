---
title: 'Fantasy Hockey'
status: final
created: '2026-09-04'
updated: '2026-09-05'
---

# PRD: Fantasy Hockey

## 0. Document Purpose

This PRD defines the requirements for a small, self-hosted web app that replaces a hand-maintained Excel sheet running a private NHL prediction game for three fixed participants. It builds on `brief.md` (product brief), `brainstorm-intent.md` (feature/UI brainstorm), and `research.md` (technical research into free NHL data sources) — all in this project's `_bmad-output/`. This document does not duplicate their reasoning; it distills them into features with nested, globally-numbered functional requirements (FR-N) that downstream UX and architecture work can reference directly. Vocabulary is Glossary-anchored (§3): every domain noun is defined once and used consistently throughout.

## 1. Vision

Three friends — Basti, Sadl, and Tobbi — run a private NHL prediction game once per season, currently tracked on a hand-maintained Excel sheet. The sheet works well enough that the game keeps happening, but it fails in ways that erode trust in it: scoring is hand-calculated and has produced a confirmed real error (a correct 20-point Stanley Cup pick that was never credited), nothing enforces prediction deadlines, and typo'd data makes any kind of automated checking impossible.

This app replaces the sheet with the same game, played the same way, minus the manual failure points. It automates the scoring *computation* — the Carolina-pick-style formula error becomes structurally impossible — enforces deadlines server-side, and gives the three participants a shared, always-correct standings view. Real-world Results are entered manually via a Management Page in v1 (a deliberate scope choice — automating that data pull remains a validated, deferred idea for later, see §4.6), not automatically fetched. It is not trying to grow beyond these three people or become a bigger product — it exists purely to remove friction from something they already enjoy playing every year.

## 2. Target User

### 2.1 Jobs To Be Done

- **Functional:** enter predictions in a couple of minutes, a handful of times per season, and trust the scoring is computed correctly without having to check it themselves.
- **Functional:** see a live, accurate standings view (Total, Regular Season, Playoffs) at any time without asking anyone or doing manual math.
- **Social:** maintain a fair, low-drama competition — nobody can view or alter a prediction after its deadline, and disagreements about scoring (like the Carolina incident) become structurally impossible.
- **Emotional:** enjoy a low-effort seasonal ritual — the app should be forgettable between deadlines, not something that demands ongoing attention.

### 2.2 Non-Users (v1)

This app is explicitly not for anyone beyond the three named participants (Basti, Sadl, Tobbi). There is no registration, invite, or join flow, and no path to add a fourth participant without a code change.

### 2.3 Key User Journeys

- **UJ-1. Basti enters his Round 1 playoff picks.**
  - **Persona + context:** Basti, having just watched the regular season wrap up, wants to lock in his playoff predictions before Round 1 starts.
  - **Entry state:** authenticated, opening the app deliberately for this purpose.
  - **Path:** checks the standings (regular season, freshly computed) → goes to Enter Predictions → works through each Round 1 series one at a time, picking a winner and game count, saving as he goes → turns to the Late Pick and enters it, informed by what he now knows from the regular season.
  - **Climax:** every Round 1 series plus the Late Pick shows as saved.
  - **Resolution:** done until the next deadline; nothing left to do until Round 2's matchups are known.
  - **Edge case:** if he only enters some series before running out of time in one sitting, the rest stay editable until the Round 1 deadline actually closes — no all-or-nothing submission.

- **UJ-2. Sadl checks how everyone stacks up after Round 1 locks.**
  - **Persona + context:** Sadl, curious now that Round 1 picks are frozen, wants to see how the group compares — not prompted by a notification, a deliberate visit.
  - **Entry state:** authenticated, deadline for Round 1 has passed.
  - **Path:** opens standings to see where he ranks → opens the Predictions page to see Basti's and Tobbi's now-visible Round 1 picks → compares picks informally (who called the upset, who's more aggressive on game counts).
  - **Climax:** seeing the full picture — his rank plus everyone's picks side by side.
  - **Resolution:** purely informational; he takes no action inside the app on what he sees.

## 3. Glossary

- **Season** — one NHL season's instance of the game, running from before the regular season starts through the Stanley Cup Final.
- **Participant** — one of the three fixed human users of the app (Basti, Sadl, Tobbi) who make predictions. Never called "Player" — that term is reserved for NHL athletes, to avoid confusion between the two.
- **Player** — an NHL athlete who can be the subject of a prediction (e.g., an Award finalist). Never used to refer to a Participant.
- **Team** — an NHL club, identified internally by a stable technical ID independent of its display name, so relocations/renames (e.g., Arizona → Utah) don't break historical data.
- **Conference / Division** — the NHL's structural grouping of Teams: two Conferences (Western, Eastern), each with two Divisions (Central/Pacific; Metropolitan/Atlantic).
- **Award** — one of five NHL individual trophies a Participant predicts finalists for: Hart, Norris, Vezina, Art Ross, Rocket Richard.
- **Series** — a best-of-7 playoff matchup between two Teams, predicted as a winner plus an exact game count (4–0, 4–1, 4–2, or 4–3).
- **Prediction** — any single pick a Participant makes (an Award finalist, a Team's playoff/division-winner mark, a Series outcome, the Early or Late Cup pick, the Presidents' Trophy pick). Every Prediction belongs to exactly one Participant and one Season, and has exactly one Deadline.
- **Deadline** — the point after which a given group of Predictions becomes read-only. Deadlines apply at three points: before the season (Awards, team marks, Early Pick, Presidents' Trophy), before Round 1 (Late Pick, Round 1 Series), and before each subsequent round (that round's Series only).
- **Early Pick / Late Pick** — the two Stanley Cup winner Predictions: Early (locked before the season) and Late (locked before Round 1, made with more information).
- **Presidents' Trophy** — the Prediction for which Team will have the best regular-season record.
- **Result** — the real-world outcome recorded against a Prediction, used to compute points. In v1, every Result is entered manually via the Management Page (§4.5); a v2+ automated Sync (§4.6, deferred) remains a valid future path for the same data.
- **Standings** — the live, computed view of every Participant's points, shown as Total, split into a Regular Season bucket and a Playoffs bucket.
- **Management Page** — the page where any Participant manually records real-world Results (award winners/finalists, team standings outcomes, playoff series results), sets each Deadline's date/time, and triggers reminder emails. Uses the same single Participant identity as everywhere else — not a separate admin role (§4.5).
- **Sync** — a deferred (v2+) automated, cron-scheduled process that would pull Results from the NHL's free data source instead of manual entry. Not built in v1 (§4.6).

_(Note: "Standings" deliberately covers both the live points view and the general concept of "who's winning" as one Glossary term — the product has no separate leaderboard concept to split it from.)_

## 4. Features

_Every feature below follows one UI principle carried from discovery: **transparency without friction** — nothing hidden except what competitive fairness requires (a Prediction before its Deadline), nothing behind a click that doesn't need to be there. UX work should treat this as inherited intent, not reinvent it._

### 4.1 Authentication

**Description:** Participants log in via a passwordless, code-based flow scoped to exactly three hardcoded email addresses. No registration, no password reset, no account management — the Participant list is fixed at deploy time. Provides the entry state for UJ-1, UJ-2.

**Functional Requirements:**

#### FR-1: Request Login Code

A visitor can request a login code by submitting an email address.

**Consequences (testable):**
- Submitting any email address returns the same generic confirmation message ("check the entered email address"), regardless of whether it matches a registered Participant.
- If the email matches one of the three hardcoded addresses, a 6-digit numeric code is generated and emailed to it.
- If the email does not match, no code is generated or sent, and the response is indistinguishable from the success case.

#### FR-2: Code Validity and Expiry

A generated code is valid until used once or until 10 minutes elapse, whichever happens first.

**Consequences (testable):**
- Submitting a valid, unused, unexpired code authenticates the Participant.
- Submitting an expired or already-used code returns a generic "invalid code" error.
- Requesting a new code before a previous one expires does not invalidate the previous code — multiple codes may be simultaneously valid for the same Participant; only 10-minute expiry or use invalidates a given code.

#### FR-3: Login Session Duration

An authenticated session is ended by 30 minutes of inactivity; any activity resets the 30-minute window.

**Consequences (testable):**
- After 30 continuous minutes with no request from an authenticated Participant, their session ends and further requests require re-authentication via a fresh code.
- Any request during the 30-minute window resets the countdown, so continuous activity never triggers a timeout.

#### FR-4: Logout

An authenticated Participant can log out, immediately ending their session.

**Consequences (testable):**
- After logout, any subsequent request requires a fresh code request.

### 4.2 Enter Predictions

**Description:** A Participant enters all Predictions for a Season through dedicated forms: 5 Awards (3 finalist names each), regular-season Team marks (playoff berth and/or division winner per Division), the Early Pick, the Presidents' Trophy pick, the Late Pick, and every Series once its round unlocks. Every Team or Player name field uses autocomplete against the canonical, per-Season list already known to the system and rejects any value that doesn't match — the typo problem that made the old spreadsheet unscoreable cannot recur. Realizes the entry path in UJ-1.

**Functional Requirements:**

#### FR-5: Autocomplete-Validated Name Entry

Every field that names a Team or a Player offers autocomplete against the canonical list for that Season and rejects free-text values that don't match an entry in it.

**Consequences (testable):**
- Typing a partial name surfaces matching canonical entries.
- Submitting a value not present in the canonical list is rejected with an inline error; the Prediction is not saved.

#### FR-6: Save Predictions Incrementally

A Participant can save any subset of their Predictions independently — there is no all-or-nothing submission for a batch of Predictions sharing a Deadline.

**Consequences (testable):**
- Saving one Series pick does not require any other Series in the same round to be filled in.
- A Series pick's winner and game count are saved together as one unit — a Series cannot be half-saved with only one of the two set.
- A partially completed set of Predictions persists across sessions until its Deadline closes.

#### FR-7: Edit Before Deadline

A Participant can revise any of their own Predictions any number of times up until its Deadline closes.

**Consequences (testable):**
- Saving a new value for a Prediction before its Deadline overwrites the previous value.
- No edit history or audit trail of prior values is required.

**Out of Scope:**
- Bulk import or copy-forward of a prior Season's Predictions.

### 4.3 Deadline Enforcement & Reminders

**Description:** Every Prediction is governed by one of the game's Deadlines (§3 Glossary). Once a Deadline closes, the Predictions under it become permanently read-only for their owner, with no grace period. Each Deadline's date/time is set via the Management Page (§4.5, FR-27); reminders are sent via the Management Page's manual trigger (§4.5, FR-28), never sent automatically on a schedule.

**Functional Requirements:**

#### FR-8: Lock on Deadline

Once a Deadline closes, all Predictions under it become read-only for the owning Participant, permanently, with no grace period or admin override.

**Consequences (testable):**
- An edit attempt on a locked Prediction is rejected, even by its own owner.
- There is no mechanism, UI or otherwise, to reopen a closed Deadline.

#### FR-9: Missing Predictions Score Zero

A required Prediction left empty when its Deadline closes scores zero points for that item; the Participant is not blocked from making Predictions under any later Deadline.

**Consequences (testable):**
- An empty Series pick at Round 1's close contributes 0 points for that Series and does not prevent entering Round 2 picks once Round 2 unlocks.

#### FR-10: Deadline Reminder Emails (Manually Triggered)

Clicking "Send Reminder Email" on the Management Page (FR-28) for a given Deadline emails every Participant who has not yet completed all Predictions under it, a plain call-to-action ("action required until TIMESTAMP") with no other content. No automatic, scheduled reminder exists — sending is entirely at the discretion of whoever uses the Management Page, and can be repeated any number of times before the Deadline closes.

**Consequences (testable):**
- A Participant who has already completed every Prediction under a Deadline is not sent the reminder, even if the button is clicked.
- No result summaries, standings updates, or other informational email content is ever sent.
- There is no cap on how many times the button can be clicked for the same Deadline before it closes.

### 4.4 Predictions Visibility

**Description:** A Participant can always see their own Predictions, regardless of Deadline status. Another Participant's Predictions under a given Deadline become visible to everyone only once that Deadline has closed. Realizes UJ-2.

**Functional Requirements:**

#### FR-11: Own Predictions Always Visible

A Participant can view all of their own Predictions, past and present, locked or not, at any time.

#### FR-12: Others' Predictions Visible Only Post-Deadline

A Participant can view another Participant's Predictions under a given Deadline only after that Deadline has closed; before that, they are hidden entirely (not shown as blank or masked — simply not present in the view).

**Consequences (testable):**
- Before Round 1's Deadline closes, no Participant can see any other Participant's Round 1 Series picks.
- Immediately after that Deadline closes, all three Participants' Round 1 picks become visible to all three.

### 4.5 Management Page

**Description:** In v1, there is no automated data source at all (§4.6 is deferred) — every real-world Result the Scoring Engine needs is recorded here: award finalists/winners for all 5 trophies, regular-season team outcomes (playoff qualifiers, division winners, Presidents' Trophy, Stanley Cup winner), and every playoff Series result. The same page also sets each Deadline's date/time and triggers reminder emails. This does not introduce a new role or account: it uses the same single Participant identity as everything else, on the trust that three intrinsically-motivated Participants are review enough — no approval/confirmation step beyond the autocomplete validation already used elsewhere (§4.2).

**Functional Requirements:**

#### FR-13: Enter Award Finalists/Winners

Any authenticated Participant can enter or edit the top-3-with-ties finalist/winner set (§4.7 FR-17) for all 5 Awards (Hart, Norris, Vezina, Art Ross, Rocket Richard) for the current Season.

**Consequences (testable):**
- Finalist name fields use the same Player autocomplete/validation as FR-5.
- The field set supports entering more than 3 names for a given Award when a real-world tie extends past 3rd place (FR-17) — the page does not hard-cap at exactly 3 entries.
- Saving finalists for a trophy makes them immediately available to the Scoring Engine (FR-17) for that trophy — no separate publish step.
- This page carries no Deadline of its own and can be edited at any time during the Season.
- If two Participants save finalists for the same trophy at effectively the same time, the later write wins — no merge, no conflict error. [ASSUMPTION: acceptable given three intrinsically-motivated Participants and the rarity of simultaneous edits to a page nobody visits often.]

#### FR-23: Enter Regular-Season Team Results

Any authenticated Participant can record, per Division, which Teams made the playoffs and which Team won the division, and can record which Team has the best regular-season record (for Presidents' Trophy scoring, FR-19).

**Consequences (testable):**
- Team fields use the same autocomplete/validation as FR-5.
- Saving a Division's results makes them immediately available to the Scoring Engine (FR-18) — no separate publish step.
- Editable at any time during the Season, same last-write-wins behavior as FR-13.

#### FR-24: Enter Stanley Cup Winner Result

Any authenticated Participant can record which Team won the Stanley Cup for the current Season, used to score both the Early Pick and the Late Pick (FR-19).

**Consequences (testable):**
- Same autocomplete/validation, editability, and last-write-wins behavior as FR-23.

#### FR-25: Enter Playoff Series Results

Any authenticated Participant can record, for each playoff Series once it concludes, the winning Team and the exact game count (4-0/4-1/4-2/4-3).

**Consequences (testable):**
- Team fields use the same autocomplete/validation as FR-5; game count is restricted to the four valid values.
- Saving a Series result makes it immediately available to the Scoring Engine (FR-20) — no separate publish step.
- Same editability and last-write-wins behavior as FR-23.

#### FR-26: Set Deadlines

Any authenticated Participant can set or edit the date/time for each of the game's Deadlines (§3 Glossary): before the season, before Round 1, and before each subsequent round.

**Consequences (testable):**
- Changing a Deadline's date/time before it has closed changes when the Predictions under it lock (FR-8); once a Deadline has closed per FR-8, its date/time can no longer be edited to reopen it — FR-8's no-reopening guarantee holds regardless of this page.
- No validation beyond the field being a valid date/time is required (e.g., no enforced ordering between rounds) — trusted to the Participant entering it.

#### FR-27: Send Reminder Email

Any authenticated Participant can trigger a reminder email for a specific Deadline from the Management Page (realizes FR-10).

**Consequences (testable):**
- Triggering the action for a Deadline emails only Participants who have not yet completed all Predictions under it (FR-10).
- The action can be triggered any number of times before that Deadline closes; there is no cooldown or maximum-sends limit.

**Out of Scope:**
- No approval, review, or second-Participant confirmation step exists for anything entered on this page — the same trust model as FR-13.

### 4.6 Automated Data Sync — DEFERRED TO v2+, NOT BUILT IN v1

**Status:** the user reversed this decision for v1 (see §6 MVP Scope) — all Result data is entered manually via the Management Page (§4.5) instead. This feature description and its FRs (FR-14–16) are kept, numbered, and unchanged as a validated future-enhancement design, not a live v1 requirement — nothing here is built until a later version explicitly picks it back up. FR IDs are preserved rather than removed so the architecture spine's existing citations of them stay accurate.

**Description (v2+):** a scheduled background process would pull Team standings, Art Ross/Rocket Richard stat leaders, and playoff bracket/series results from the NHL's free data API (per `research.md`) and make them available to the Scoring Engine, replacing the Management Page's manual Result entry (§4.5) for that data. Award finalists for the 3 voted trophies (Hart/Norris/Vezina) would remain manual even in v2+, per `research.md`'s confirmed data-availability split — only §4.5's award-entry FR (FR-13) is genuinely permanent, not merely deferred.

**In v1, the per-Season Team list (technical IDs, independent of display name — see Glossary) is seeded at deployment time rather than loaded at runtime by either a Sync or a Participant** — there is no Participant-facing "manage teams" UI in v1 or v2+; FR-15 below describes the v2+ automated alternative to that same one-time seeding step.

**Functional Requirements:**

#### FR-14: Scheduled Sync

The Sync runs on a cron schedule whose interval is configured via an environment variable, and once on startup.

**Consequences (testable):**
- Changing the configured interval and restarting changes the schedule without a code change.
- A fresh deployment performs one Sync run immediately on startup, without waiting for the first scheduled interval.

#### FR-15: Team List Bootstrap

Each Team is identified internally by a stable technical ID, independent of its display name. When a Season is created, the current NHL Team list is loaded from the data source and persisted for that Season.

**Consequences (testable):**
- A Team's display name can change (e.g., a relocation) without breaking any reference to it, past or present, since every reference uses the technical ID, never the name.
- Once persisted for a Season, that Season's Team list does not change, even if an NHL organizational change occurs mid-Season. [ASSUMPTION: mid-season relocations are not a real-world concern within a single season — confirm not needed.]

#### FR-16: Resilient to Source Failure

If the NHL data source is unreachable, returns an error, or returns a response that fails schema validation (e.g., an unexpected shape following an upstream API change, as happened in 2023 per `research.md`), the Sync writes no data and takes no other action; the next scheduled run retries independently.

**Consequences (testable):**
- A failed or malformed Sync run leaves all previously synced data unchanged — no partial writes, and no attempt to interpret or partially trust a response that doesn't match the expected schema.
- No alert, notification, or in-app indicator is raised on failure; observability is handled outside the app (metrics/logs/otel to an external monitoring destination — implementation detail, not a product requirement here).

**Feature-specific NFRs:**
- **Rate-limit tolerance.** The NHL data source publishes no official rate limit, but community evidence (`research.md`) shows throttling occurs in practice. The Sync must back off rather than retry aggressively on a throttling response (e.g., HTTP 429), and should avoid redundant requests within a single run (fetch each needed resource once per scheduled run, not once per consumer).

### 4.7 Scoring Engine

**Description:** Computes every Participant's points automatically from the Results and award finalists entered on the Management Page (§4.5), whenever either changes. No human ever calculates or overrides a score — the single failure mode that motivated this whole rebuild (the uncredited Carolina pick) becomes structurally impossible, regardless of whether the underlying Result data was entered manually (v1) or synced automatically (deferred §4.6).

**Functional Requirements:**

#### FR-17: Award Scoring (Tie-Inclusive)

For each of the 5 Awards, each of a Participant's 3 predicted names scores 5 points if it appears anywhere in the actual top-3 for that Award, where a tie at any position expands the counted set to include every tied name.

**Consequences (testable):**
- Given a 3-way tie for 3rd place (5 total qualifying names across 1st/2nd/tied-3rd), a Participant who predicted 0 of the top 2 but all 3 tied-for-3rd names scores 15 points (3 × 5) for that Award.
- In v1, all 5 Awards — including Art Ross and Rocket Richard — score against the finalist/winner set entered via FR-13; same scoring rule for all 5, same manual data source in v1. (In a v2+ that revives §4.6, Art Ross/Rocket Richard could instead score against a live stat leaderboard, with no change to this scoring rule.)

#### FR-18: Regular-Season Team Scoring

A Team-makes-playoffs mark scores 5 points if correct; a division-winner mark scores 15 points if that Team wins its division, or 5 points (not 20) if that Team only makes the playoffs. In v1, scores against the Results entered via FR-23.

#### FR-19: Cup Picks and Presidents' Trophy

The Early Pick and Late Pick each score 20 points if the predicted Team wins the Stanley Cup; the Presidents' Trophy pick scores 20 points if the predicted Team has the best regular-season record. In v1, scores against the Results entered via FR-24 (Cup winner) and FR-23 (Presidents' Trophy).

**Consequences (testable):**
- Early Pick and Late Pick points are attributed to the Playoffs bucket (§4.8), not Regular Season, since they resolve during the playoffs.

#### FR-20: Playoff Series Scoring (Exact Result Replaces Winner-Only)

Each Series scores by round: Round 1 correct-winner 15 / exact-result 25; Round 2 correct-winner 25 / exact-result 35; Conference Finals correct-winner 30 / exact-result 45; Stanley Cup Final correct-winner 30 / exact-result 50. Exact-result points replace correct-winner points; they are not additive. In v1, scores against the Results entered via FR-25.

#### FR-21: No Tie-Break

When two or more Participants have equal Total points, they share the same rank; no secondary criterion is applied.

### 4.8 Standings

**Description:** A live view of every Participant's points, always visible (per the brief's decision to show it on every page rather than a dedicated screen, given only three Participants). Realizes the standings-check beat in both UJ-1 and UJ-2.

**Functional Requirements:**

#### FR-22: Live Standings Display

Standings show, per Participant: Regular Season points, Playoffs points, and Total (their sum), reflecting the latest state from the Scoring Engine (§4.7) at all times.

**Consequences (testable):**
- There is no separate "live/projected" tier distinct from the last entered Result — the displayed Standings always equal what the Scoring Engine last computed from whatever's been entered on the Management Page (§4.5), nothing more current and nothing stale-but-labeled-current.
- Participants tied on Total are displayed at the same rank (FR-21).
- A displayed Standings value always matches what a manual hand-check of the underlying Results and Predictions would produce — there is no code path that computes or displays a score independently of the Scoring Engine (§4.7).

## Cross-Cutting NFRs

- **No partial writes.** Every write (Management Page Result entry, Scoring Engine recompute) either completes fully or leaves prior state untouched — never a half-updated Season.
- **Reproducibility.** Given the same entered Results and the same Predictions, the Scoring Engine always computes the same points — no hidden state, no manual adjustment path exists anywhere (§4.7 explains why this matters).
- **No enumeration leaks.** Any user-facing response (login, elsewhere) must not reveal information about the fixed Participant list or other Participants' state beyond what §4.4's visibility rules explicitly allow.
- **Observability without in-app alerting.** Scoring Engine failures are surfaced through externally exported metrics/logs (destination is an implementation detail, out of scope for this PRD) — never through in-app UI, since there is no admin role to act on such an alert. (§4.6's deferred Sync would add its own failure-observability need if revived in v2+.)

## Constraints and Guardrails

- **Cost.** v1 has no external data dependency at all (§4.6 deferred) — moot for v1. If a v2+ revives automated Sync, that dependency must remain free — no paid API tier, no paid data provider; `research.md` already confirmed this is feasible for standings/Art Ross/Rocket Richard/playoffs and infeasible for Hart/Norris/Vezina, which is why FR-13's manual entry is permanent regardless of §4.6's fate.
- **Deployment.** Ships only as a Docker image, run via docker-compose — no other deployment target is in scope.
- **Legal.** Moot for v1 (no automated access to the NHL's data source exists). If a v2+ revives §4.6: NHL.com's own Terms of Service prohibit automated scraping in general — but whether that ToS actually extends to the specific API subdomain this app would depend on is, per `research.md`, a genuinely open legal question, not a settled one, to be knowingly accepted or resolved at that time.
- **Manual-entry accuracy.** v1's reversal to manual Result entry (§4.5) reintroduces a class of risk the original automated-Sync design was meant to eliminate: FR-5's autocomplete prevents *name* typos, but nothing prevents a Participant from entering a factually wrong outcome (the wrong Team as series winner, a wrong game count). Accepted knowingly, consistent with the app's existing no-admin, no-approval-step trust model (§4.5) — the same three intrinsically-motivated Participants who enter their own Predictions also enter the real-world Results.

## 5. Non-Goals (Explicit)

- Not a multi-league or multi-group platform — the Participant list is exactly three people, hardcoded, forever (no invite/join flow ever planned).
- Not introducing any role beyond the single Participant identity — no admin, no scorekeeper, no spectator/read-only account.
- Not pursuing real-time or automated scoring inputs in v1 — freshness is bounded by how promptly a Participant enters a Result on the Management Page, by design (see §4.6 for the deferred automated alternative).
- Not a mobile app — web only.
- Not monetized, not built for or intended to attract any user beyond the three named Participants.

## 6. MVP Scope

### 6.1 In Scope

- Authentication (FR-1–4), Enter Predictions for all 6 Prediction types (FR-5–7), Deadline Enforcement & Reminders — manually triggered (FR-8–10), Predictions Visibility (FR-11–12)
- Management Page: award finalists/winners for all 5 Awards, regular-season team Results, Cup winner, playoff Series Results, Deadline scheduling, manual reminder trigger (FR-13, FR-23–27)
- Full Scoring Engine (FR-17–21) and persistent Standings (FR-22), scored entirely against manually entered Results in v1

### 6.2 Out of Scope for MVP

- **Automated Data Sync** (FR-14–16, §4.6). Reversed out of v1 Must by explicit user decision — all Result data is manually entered instead (Management Page, §4.5). The design remains valid and is kept, numbered, in the PRD for a future version to pick back up unchanged; it is a deferred enhancement, not an abandoned idea.
- **Multi-season data model and season-selector.** Deferred to v2 — v1 supports exactly one Season at a time.
- **Season Hub landing page.** Deferred to v2, grouped with multi-season support since its main content (season context, quick links) only earns its place once more than one Season exists.
- **Hall of Fame page** (championship/runner-up/third-place counts across seasons). Deferred to v2 for the same reason — needs at least two completed Seasons to be meaningful. [NOTE FOR PM: this was the idea the user was most visibly enthusiastic about during brainstorming — worth actively revisiting once Season 2 is in view, not just leaving parked indefinitely.]

## 7. Success Metrics

**Primary**
- **SM-1:** Zero manually-corrected scoring errors across a full Season, and displayed Standings always match a hand-check of the underlying Results and Predictions (the specific failure this rebuild exists to eliminate). Validates FR-17–22.
- **SM-2:** All three Participants have completed every required Prediction before each Deadline closes, every Deadline, every Season. Validates FR-8–10.

**Secondary**
- **SM-3:** The Excel sheet is never opened again for an active Season once this app is in use.

**Counter-metrics (do not optimize)**
- **SM-C1:** Session frequency / time-in-app. This app succeeds by being forgettable between Deadlines — rising engagement would signal friction (confusing UI driving repeat visits to figure something out), not success. Counterbalances any temptation to add engagement-driving features. Counterbalances SM-2 (don't chase completion by nagging via repeated FR-27 reminder sends — the button's lack of a cap is a convenience for whoever manages deadlines, not license to spam).

## 8. Open Questions

1. Whether a structured, automatable source for Hart/Norris/Vezina finalists might exist after all via a Wikidata SPARQL query against player-level `wdt:P166` statements — flagged as an unexecuted lead in `research.md`, not confirmed either way. Only relevant if a future version revives any automated data path; FR-13's manual entry is the permanent v1 (and likely v2+) answer for these 3 trophies regardless.
2. (Relevant only if a future version revives §4.6.) Whether the NHL's free data API changing or breaking again (as it did in 2023, per `research.md`) should have any user-visible handling beyond FR-16's silent retry-next-cycle behavior.
3. (Relevant only if a future version revives §4.6.) Exact backoff bound for FR-16's rate-limit tolerance (e.g., a maximum backoff delay or retry count within one Sync run).

## 9. Assumptions Index

- Inline assumption from §4.6 FR-15 — a Season's persisted Team list never needs to change mid-Season; NHL relocations are a between-seasons event only.
- Inline assumption from §4.5 FR-13 — simultaneous edits to award finalists resolve last-write-wins, no conflict handling, given how rarely that page is used.
