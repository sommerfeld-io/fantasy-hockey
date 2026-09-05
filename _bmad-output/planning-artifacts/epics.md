---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md
  - _bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-05/ARCHITECTURE-SPINE.md
  - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/DESIGN.md
  - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/EXPERIENCE.md
---

# Fantasy Hockey - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for Fantasy Hockey, decomposing the requirements from the PRD, UX Design (DESIGN.md/EXPERIENCE.md), and Architecture Spine (ARCHITECTURE-SPINE.md) into implementable stories.

**Known gap:** the UX design contract predates the PRD's sync-reversal update and does not cover the Management Page's newer capabilities (FR-23–27: team results, Cup winner, series results, deadline-setting, manual reminder trigger). Stories touching that surface are flagged `[NEEDS UX]` rather than inventing visual detail.

## Requirements Inventory

### Functional Requirements

FR-1: A visitor can request a login code by submitting an email address; response is identical whether or not the email matches a registered Participant.
FR-2: A generated login code is valid until used once or until 10 minutes elapse; multiple codes may be simultaneously valid per Participant.
FR-3: An authenticated session ends after 30 minutes of inactivity; any request resets the countdown.
FR-4: An authenticated Participant can log out, immediately ending their session.
FR-5: Every Team/Player name field offers autocomplete against the canonical per-Season list and rejects non-matching free text.
FR-6: A Participant can save any subset of their Predictions independently; a Series pick's winner and game count save together as one unit.
FR-7: A Participant can revise any of their own Predictions any number of times before its Deadline closes.
FR-8: Once a Deadline closes, all Predictions under it become permanently read-only, with no grace period or override.
FR-9: A required Prediction left empty at Deadline close scores zero for that item; does not block later Deadlines.
FR-10: Clicking "Send Reminder Email" (FR-27) for a Deadline emails every Participant who hasn't completed all Predictions under it; no automatic/scheduled reminder exists in v1; unlimited repeat sends.
FR-11: A Participant can view all of their own Predictions, locked or not, at any time.
FR-12: A Participant can view another Participant's Predictions under a Deadline only after that Deadline has closed; hidden entirely (not masked) beforehand.
FR-13: Any Participant can enter/edit the top-3-with-ties finalist/winner set for all 5 Awards (Hart, Norris, Vezina, Art Ross, Rocket Richard); supports more than 3 entries when a real tie extends past 3rd place; last-write-wins on concurrent edits.
FR-14 [DEFERRED v2+, not built in v1]: A scheduled background Sync process would run on a cron interval, fetching NHL data automatically.
FR-15 [DEFERRED v2+, not built in v1]: Team list would be bootstrapped from a live NHL webservice when a Season is created. (v1: Team list is seeded via database migration at deployment time instead.)
FR-16 [DEFERRED v2+, not built in v1]: The (deferred) Sync would be resilient to source failure/schema mismatch, writing nothing on failure.
FR-17: For each of the 5 Awards, each of a Participant's 3 predicted names scores 5 points if it appears anywhere in the actual top-3 (tie-inclusive); v1 scores against FR-13's manually entered data for all 5 Awards.
FR-18: A Team-makes-playoffs mark scores 5 points if correct; a division-winner mark scores 15 (replacing, not adding to, the 5) or 5 if that Team only makes the playoffs; v1 scores against FR-23's manually entered Results.
FR-19: The Early/Late Cup Pick each score 20 points if the predicted Team wins the Stanley Cup; the Presidents' Trophy pick scores 20 if the predicted Team has the best regular-season record; v1 scores against FR-24 (Cup winner) and FR-23 (Presidents' Trophy) manually entered Results.
FR-20: Playoff Series score by round (Round 1: 15/25, Round 2: 25/35, Conference Finals: 30/45, Final: 30/50), exact-result replacing correct-winner-only; v1 scores against FR-25's manually entered Results.
FR-21: Participants with equal Total points share the same rank; no secondary tiebreaker.
FR-22: Standings display Regular Season, Playoffs, and Total points per Participant, always reflecting the Scoring Engine's latest computation live (no cached/stored score, no separate "live/projected" tier).
FR-23: Any Participant can record, per Division, which Teams made the playoffs, which Team won the division, and which Team has the best regular-season record (Presidents' Trophy).
FR-24: Any Participant can record which Team won the Stanley Cup for the current Season.
FR-25: Any Participant can record, per playoff Series once it concludes, the winning Team and exact game count.
FR-26: Any Participant can set or edit the date/time for each of the game's Deadlines (before the season, before Round 1, before each subsequent round).
FR-27: Any Participant can trigger a reminder email for a specific Deadline; emails only Participants who haven't completed all Predictions under it; no cap on repeat sends.

### NonFunctional Requirements

NFR1: No partial writes — every write (Management Page Result entry, Scoring Engine recompute) either completes fully or leaves prior state untouched.
NFR2: Reproducibility — given the same entered Results and Predictions, the Scoring Engine always computes the same points; no hidden state, no manual score-adjustment path exists anywhere.
NFR3: No enumeration leaks — no user-facing response (login, Predictions visibility) may reveal information about the fixed Participant list or other Participants' state beyond what visibility rules explicitly allow; enforced at the response-shape/timing level (auth) and at the query level (visibility filtering must never share a code path with the Scoring Engine's read, per Architecture AD-25).
NFR4: No in-app alerting — Scoring Engine failures are surfaced only through externally exported metrics/logs, never through in-app UI (no admin role exists to act on one).
NFR5: Deployment constraint — ships only as a Docker image, run via docker-compose; no other deployment target is in scope.
NFR6: Manual-entry accuracy is an accepted risk in v1 — autocomplete (FR-5) prevents name typos but not factually wrong outcomes; no additional validation/approval layer is required beyond the existing single-Participant-identity trust model.

### Additional Requirements

- **No starter template — brownfield.** An existing minimal Go skeleton already lives in `src/`: `main.go` (currently a placeholder printing the time via `internal/clock`), Go 1.26.4, module `github.com/sommerfeld-io/fantasy-hockey`, GoDog acceptance tests under `src/acceptance-tests/features/`, golangci-lint + gocyclo(10) + go-licenses + govulncheck build gates, a multi-stage Alpine Dockerfile, and Taskfile orchestration (root `taskfile.yml` → `go:` namespace → `src/taskfile.yml`). **Epic 1 Story 1 must build on this existing structure, not scaffold from scratch** — ratify `internal/clock`, extend `main.go`, keep the existing task names and build-gate pipeline intact (Architecture AD-2–AD-8, [ADOPTED]).
- **Layered architecture (Architecture AD-9, AD-26).** Packages: `internal/web` (presentation), `internal/auth`, `internal/predictions`, `internal/scoring`, `internal/standings`, `internal/results` (renamed/broadened from the originally-planned `internal/awards` — owns all Management Page writes), `internal/mailer`, `internal/store` (owns all DB reads/writes, exports shared entity structs and enums), `internal/clock` (existing). Dependency direction: presentation → feature packages → store; store depends on nothing above it.
- **Deferred v2+ packages, not built in v1 (Architecture AD-1, AD-17, AD-23, AD-24):** `internal/sync`, `internal/nhlclient`. The entire automated-Sync/multi-mode-binary design is documented in the Architecture Spine as ADs kept "numbered and unchanged in substance" for a future version — do not build, do not stub.
- **Database (Architecture AD-10, AD-19, AD-27, AD-28, AD-34):** PostgreSQL 18 (`postgres:18-alpine`), `jackc/pgx` v5.10.0 driver, `golang-migrate/migrate/v4` v4.19.1 for schema migrations (run on startup, before serving). Connection string via `DATABASE_URL` env var or `--database-url` CLI flag (flag wins if both set); process fails fast at startup if neither is set. All entities use an application-generated `uuid` primary key except `Team`, which uses the NHL's own team identifier.
- **Season/Team bootstrap (Architecture AD-29):** on startup, the app creates a Season row for the year named by `SEASON_YEAR` env var if none exists yet. The Team list itself is pre-seeded by a `golang-migrate` migration (present before first startup) — not fetched at runtime in v1.
- **Session/auth (Architecture AD-12, AD-31):** stateless HMAC-signed cookie (Participant + issued-at), re-issued each request for the 30-min sliding timeout; no server-side session table. Signing key from a required `SESSION_SECRET` env var — process fails to start if unset.
- **Email (Architecture AD-13, AD-14, AD-33):** Gmail SMTP (`smtp.gmail.com:587`, STARTTLS) via stdlib `net/smtp`, wrapped in `internal/mailer`. Sending account needs 2-Step Verification + an App Password (manual, outside the app), supplied as a runtime secret. v1: both login-code and reminder sends are synchronous, request-triggered — no background ticker, no `ReminderLog` dedupe table.
- **Presentation (Architecture AD-11, AD-21):** server-rendered HTML via stdlib `html/template`; routing via stdlib `net/http` `ServeMux`; only client-side JS is the vanilla-JS autocomplete widget. No JS framework, no ORM, no third-party router, no separate JSON API — autocomplete candidate lists are embedded as JSON directly in the rendered page (`{"id": ..., "label": ...}` shape, shared across every embedding site).
- **Scoring computed live (Architecture AD-16):** `internal/scoring` and `internal/standings` compute points on every read directly from stored Predictions/Results/AwardFinalist data — no score value is ever written to storage.
- **Build-gate tooling (Architecture AD-6, AD-30):** existing golangci-lint/gocyclo/go-licenses/govulncheck gates, plus new `depguard` rules enforcing the layered dependency direction (feature packages never import `internal/web`) as a lint failure, not just review.
- **Acceptance testing (Architecture AD-5, existing):** every feature's acceptance criteria must be expressed as Gherkin `.feature` files under `src/acceptance-tests/features/`, executed via GoDog — per the project's CLAUDE.md TDD/BDD mandate, this applies to every story in this breakdown.
- **Deployment target (Architecture AD-15):** Raspberry Pi (arm64), already covered by the existing multi-arch Dockerfile; app serves plain HTTP, no TLS — a reverse proxy (out of scope, to be set up later) handles exposure.

### UX Design Requirements

UX-DR1: Dark-only theme ("Steel Ice" palette) — `surface-base` #151a1f, `surface-raised` #1c2229, `border-hairline` #2b333b, `accent` #6f9bb8, `text-primary` #e2e6ea, `text-secondary` #8b98a3, `danger` #c97b72. Deliberately no "success" color (no per-prediction correctness marking — cuts against the anti-gamification stance).
UX-DR2: System font stack; four text roles — `heading`, `body`, `meta`, `data` (tabular-nums for all point/score/game-count figures).
UX-DR3: Spacing scale 4/8/12/16/24/32px; single-column content everywhere; the Standings widget is a full-width top bar, never a sidebar, at every viewport width.
UX-DR4: No shadows/elevation anywhere — depth comes only from surface-tone contrast + hairline borders.
UX-DR5: `rounded/sm` (4px) on inputs/buttons/chips; nothing fully rounded, no pill buttons, no avatars.
UX-DR6: **Standings widget** — full-width bar on every authenticated page; columns Rank / Regular Season / Playoffs / Total (tabular data); tied Participants show the identical Rank value, no visual distinction; the signed-in Participant's own row gets a thin accent left-border only.
UX-DR7: **Nav bar** — flat text links, accent underline on the active link, no icons, no hamburger at any width (3 links max).
UX-DR8: **Pick chip** component — a bordered box (reused, unmodified, between the interactive Enter-Predictions controls and the read-only Predictions-home display) holding one Award finalist name, one Series winner, or one game count (`4-0`/`4-1`/`4-2`/`4-3` notation, never a raw games-played number). Award finalists render as multiple chips on one line, alphabetically ordered.
UX-DR9: **Prediction row** — one Series/Award per row; locked rows show plain read-only text with a "Locked" meta marker, never a disabled-looking input; Series rows carry a visible seed label (e.g. "Eastern · Series A") and are ordered by that seeding.
UX-DR10: **Login form** — single field visible at a time; requesting a code replaces the email field with the code field in the same slot (never both shown together); invalid-code error uses the same bordered/inline-message pattern as autocomplete rejection.
UX-DR11: **Autocomplete input** — suggestions drop below the field; a non-matching entry shows a `danger`-bordered field + inline `meta`-sized message, no toast/modal.
UX-DR12: **Button (primary)** — accent background, at most one per screen.
UX-DR13: Plain/neutral voice throughout — no exclamation marks, emoji, or banter. Exact strings specified: "Check the entered email address.", "Invalid code.", "Action required until {timestamp}.", "Locked.", "Session expired."
UX-DR14: Accessibility floor — WCAG AA contrast for body/data text, full keyboard operability (autocomplete included), comfortable touch targets, focus order follows visual/reading order. No higher formal compliance target.
UX-DR15: Single responsive layout across desktop/tablet/mobile — no separate mobile design; content stays single-column, nav stays flat links at every width.
UX-DR16: Explicit anti-patterns to avoid: sports-broadcast visual clichés (team-color gradients, ice/rink textures, trophy iconography), gamification chrome (streaks, badges, celebratory animations), carousels, hero animations, notification badge counts, any auto-refresh/polling UI.
UX-DR17: Information Architecture (as designed — **stale for FR-23–27**, see gap note above): Login (unauthenticated) → Predictions home (default landing, "My Predictions" entry point) → Enter Predictions → Award Data Entry (nav link, low-traffic) → Logout (header action). Standings is not a page — see UX-DR6.
UX-DR18 [NEEDS UX]: No design exists yet for FR-23 (team results entry), FR-24 (Cup winner entry), FR-25 (series results entry), FR-26 (deadline date/time fields), or FR-27 (send-reminder button) — these are new Management Page capabilities added after the UX spines were finalized. Stories touching this surface should reuse UX-DR1–14's established component language (Pick chip, autocomplete input, button primary, plain/neutral voice) by extension, but the actual layout/flow is undesigned.

### FR Coverage Map

| FR | Epic |
|---|---|
| FR-1 | Epic 1: Authentication — Request Login Code |
| FR-2 | Epic 1: Authentication — Code Validity and Expiry |
| FR-3 | Epic 1: Authentication — Login Session Duration |
| FR-4 | Epic 1: Authentication — Logout |
| FR-5 | Epic 3: Predictions — Autocomplete-Validated Name Entry |
| FR-6 | Epic 3: Predictions — Save Predictions Incrementally |
| FR-7 | Epic 3: Predictions — Edit Before Deadline |
| FR-8 | Epic 3: Predictions — Lock on Deadline |
| FR-9 | Epic 3: Predictions — Missing Predictions Score Zero |
| FR-10 | Epic 3: Predictions — Deadline Reminder Emails (behavioral rule for FR-27; requires Epic 3's own completion-check, so lives here not Epic 2) |
| FR-11 | Epic 3: Predictions — Own Predictions Always Visible |
| FR-12 | Epic 3: Predictions — Others' Predictions Visible Only Post-Deadline |
| FR-13 | Epic 2: Game Management — Enter Award Finalists/Winners |
| FR-14 | **Deferred v2+ — no epic** (Automated Sync) |
| FR-15 | **Deferred v2+ — no epic** (Team List Bootstrap) |
| FR-16 | **Deferred v2+ — no epic** (Resilient to Source Failure) |
| FR-17 | Epic 4: Scoring & Standings — Award Scoring (Tie-Inclusive) |
| FR-18 | Epic 4: Scoring & Standings — Regular-Season Team Scoring |
| FR-19 | Epic 4: Scoring & Standings — Cup Picks and Presidents' Trophy |
| FR-20 | Epic 4: Scoring & Standings — Playoff Series Scoring |
| FR-21 | Epic 4: Scoring & Standings — No Tie-Break |
| FR-22 | Epic 4: Scoring & Standings — Live Standings Display |
| FR-23 | Epic 2: Game Management — Enter Regular-Season Team Results |
| FR-24 | Epic 2: Game Management — Enter Stanley Cup Winner Result |
| FR-25 | Epic 2: Game Management — Enter Playoff Series Results |
| FR-26 | Epic 2: Game Management — Set Deadlines |
| FR-27 | Epic 3: Predictions — Send Reminder Email (moved from Epic 2 to avoid a forward dependency on `internal/predictions`) |

## Epic List

### Epic 1: Authentication
Participants can securely log in via an emailed one-time code and log out; sessions expire after 30 minutes of inactivity.
**FRs covered:** FR-1, FR-2, FR-3, FR-4
**Implementation notes:** First epic to introduce `internal/web`, `internal/auth`, `internal/mailer`, DB connection/migrations (Architecture AD-28, AD-34). Builds on the existing Go skeleton (`main.go`, `internal/clock`) rather than scaffolding fresh. UX: Login form component (UX-DR10), plain/neutral microcopy (UX-DR13).

### Epic 2: Game Management
Any Participant can set every Deadline's date/time and record all real-world outcomes (award winners for all 5 trophies, team standings, Stanley Cup winner, playoff series results).
**FRs covered:** FR-13, FR-23, FR-24, FR-25, FR-26
**Implementation notes:** New `internal/results` package (Architecture AD-32). Introduces Season/Team bootstrap (AD-29) — a Season row must exist before Results can be recorded against it. New `Deadline` entity (AD-32). **No UX design exists yet for this epic's screen(s)** (UX-DR18) — story-level UI detail should reuse the established component language (Pick chip, autocomplete input, button primary, plain/neutral voice) but layout/flow needs a UX pass.

### Epic 3: Predictions
Participants can enter, incrementally save, and edit their predictions across all 6 prediction types before each Deadline; predictions lock automatically at the Deadline; a Participant always sees their own picks and sees others' picks only once locked; and anyone can send a reminder to whoever hasn't finished predicting.
**FRs covered:** FR-5, FR-6, FR-7, FR-8, FR-9, FR-10, FR-11, FR-12, FR-27
**Implementation notes:** New `internal/predictions` package. Reads `Deadline` values written by Epic 2 (does not write them). Fully designed in UX (Pick chip UX-DR8, Prediction row UX-DR9, autocomplete UX-DR11) except the reminder button itself, which shares Epic 2's `[NEEDS UX]` gap. Reminder send is synchronous/request-triggered, no ticker (AD-33) — deliberately placed last in this epic since it needs this epic's own completion-check data plus Epic 2's Deadlines.

### Epic 4: Scoring & Standings
Participants see a live, always-accurate standings view (Total, Regular Season, Playoffs, Rank) computed automatically from their predictions and Epic 2's recorded results — no manual math, no stale data, ties share rank.
**FRs covered:** FR-17, FR-18, FR-19, FR-20, FR-21, FR-22
**Implementation notes:** New `internal/scoring`, `internal/standings` packages. Computed live on every read, nothing persisted (Architecture AD-16). The Standings widget (UX-DR6) renders on every authenticated page from Epic 1 onward — earlier epics may need a placeholder/stub widget until this epic makes it real.

**Deferred, no epic:** FR-14, FR-15, FR-16 (Automated Data Sync) — explicitly out of v1 per PRD §4.6 and Architecture AD-1/AD-17 (kept, numbered, unchanged for a future version).

## Epic 1: Authentication

Participants can securely log in via an emailed one-time code and log out; sessions expire after 30 minutes of inactivity.

### Story 1.1: Request Login Code

As a visitor,
I want to request a login code by submitting my email address,
So that I can begin logging in without a password.

**Acceptance Criteria:**

**Given** the login page is open
**When** a visitor submits any email address
**Then** the response is the same generic confirmation message ("check the entered email address") regardless of whether the email matches a registered Participant (FR-1, NFR3 no enumeration leaks)

**Given** the submitted email matches one of the three hardcoded Participant addresses
**When** the request is processed
**Then** a 6-digit numeric code is generated, persisted as a `LoginCode` row (participant, `code_hash` = sha256(code), issued-at, used-at nullable — Architecture AD-22), and emailed via Gmail SMTP through `internal/mailer` (AD-13, AD-14)

**Given** the submitted email does not match any Participant
**When** the request is processed
**Then** no `LoginCode` row is created and no email is sent, with no observable difference from the success case

**Given** a Participant already has an unused, unexpired code
**When** they request a new code
**Then** a second `LoginCode` row is created — the previous code remains valid until it expires or is used (FR-2 multiple-simultaneous-codes rule)

**And** this story establishes the foundational infrastructure the rest of the app builds on: DB connection resolution (`DATABASE_URL` env var or `--database-url` flag, flag wins — AD-34), `golang-migrate` running on startup (AD-28), the initial `internal/store`, `internal/web`, `internal/auth`, and `internal/mailer` packages, and the Login page's email-field state (UX-DR10)

**And** this story establishes the cross-cutting UX baseline every later story inherits, not re-tests: single responsive layout across desktop/tablet/mobile with no separate mobile design (UX-DR15); WCAG AA contrast on body/data text, full keyboard operability, comfortable touch targets, focus order following reading order (UX-DR14 accessibility floor); and the anti-pattern list is treated as a standing constraint on all UI work — no sports-broadcast visual clichés, no gamification chrome (streaks/badges/celebration animations), no carousels/hero animations/notification badges/auto-refresh polling (UX-DR16)

### Story 1.2: Validate Login Code and Establish Session

As a Participant who requested a code,
I want to submit my 6-digit code,
So that I'm logged in with a session that stays active while I'm using the app.

**Acceptance Criteria:**

**Given** a Participant has an unused `LoginCode` within its 10-minute window
**When** they submit the matching code
**Then** they are authenticated and issued an HMAC-signed session cookie carrying Participant + issued-at (Architecture AD-12), signed with the required `SESSION_SECRET` env var (AD-31 — process fails to start if unset)

**Given** a Participant submits an expired or already-used code
**When** the code is checked
**Then** a generic "Invalid code." error is shown inline and the code field is cleared (FR-2, UX-DR13)

**Given** an authenticated Participant's session
**When** 30 minutes pass with no request
**Then** the session ends and the next request requires a fresh code (FR-3)

**And** any request within the 30-minute window resets the countdown (sliding timeout), by re-issuing the cookie on every authenticated request

**And** the Login page shows one field at a time — submitting the email replaces that field with the code field in the same slot, never both at once (UX-DR10)

### Story 1.3: Logout

As an authenticated Participant,
I want to log out,
So that my session ends immediately on this device.

**Acceptance Criteria:**

**Given** an authenticated Participant
**When** they choose Logout (header action, every authenticated page)
**Then** their session cookie is cleared/not re-issued and any subsequent request requires a fresh login code (FR-4)

---

**Epic 1 summary:** 3 stories, FR-1–4 fully covered.

## Epic 2: Game Management

Any Participant can set every Deadline's date/time and record all real-world outcomes (award winners for all 5 trophies, team standings, Stanley Cup winner, playoff series results). (Reminder emails are Epic 3, Story 3.4 — see note below.)

**[NEEDS UX]** No design exists yet for this epic's screen(s) (UX-DR18) — stories below specify behavior precisely but reuse established component language (Pick chip, autocomplete input, button primary, plain/neutral voice) for layout, pending an actual UX pass.

### Story 2.1: Season & Team Bootstrap

As a Participant recording results for the first time in a Season,
I want the current Season and its Teams to already exist,
So that I can pick a Team from autocomplete instead of typing it by hand.

**Acceptance Criteria:**

**Given** a fresh deployment with no Season row for the current year
**When** the app starts
**Then** it creates a Season row for the year named by the `SEASON_YEAR` env var (Architecture AD-29)

**Given** the app is starting for the first time
**When** migrations run
**Then** the per-Season Team list (each Team keyed by the NHL's own team identifier, not a locally generated ID — AD-19) is already present from a `golang-migrate` seed migration, not fetched at runtime (PRD §4.6, AD-29 v1 rule)

**And** this Team list is what every autocomplete field in this epic and Epic 3 draws from (FR-5, AD-21)

### Story 2.2: Set Deadlines

As a Participant,
I want to set or edit the date/time for each of the game's Deadlines,
So that predictions lock at the right moments.

**Acceptance Criteria:**

**Given** the Management Page's deadline section
**When** a Participant sets a date/time for one of the game's Deadlines (before the season, before Round 1, before each subsequent round)
**Then** a `Deadline` row is saved (Season, `deadline_key` enum, date/time — Architecture AD-32) (FR-26)

**Given** a Deadline has already closed
**When** its date/time value is inspected
**Then** it cannot be edited to reopen it — FR-8's no-reopening guarantee holds regardless of this page

**And** no ordering validation between rounds is enforced — trusted to the Participant entering it (FR-26)

### Story 2.3: Enter Award Finalists/Winners

As a Participant,
I want to enter or edit the top-3-with-ties finalist/winner set for all 5 Awards,
So that the Scoring Engine can score everyone's award predictions.

**Acceptance Criteria:**

**Given** the Management Page's award section
**When** a Participant enters finalist names for Hart, Norris, Vezina, Art Ross, or Rocket Richard
**Then** each name field uses the same Team/Player autocomplete and rejects non-matching free text (FR-5, FR-13)

**Given** a real-world tie extends past 3rd place for an Award
**When** a Participant records the result
**Then** the field set supports entering more than 3 names for that Award — it does not hard-cap at exactly 3 (FR-13, FR-17's tie-inclusive rule)

**Given** finalists are saved for a trophy
**When** the save completes
**Then** they are immediately available to the Scoring Engine for that trophy — no separate publish step (FR-13)

**Given** two Participants save finalists for the same trophy at effectively the same time
**When** both writes land
**Then** the later write wins — no merge, no conflict error (FR-13)

**And** this page carries no Deadline of its own and can be edited at any time during the Season

### Story 2.4: Enter Regular-Season Team Results

As a Participant,
I want to record which Teams made the playoffs, which Team won each Division, and which Team has the best regular-season record,
So that the Scoring Engine can score everyone's team-mark and Presidents' Trophy predictions.

**Acceptance Criteria:**

**Given** the Management Page's team-results section
**When** a Participant records, per Division, which Teams made the playoffs and which Team won the division
**Then** Team fields use the same autocomplete/validation as FR-5, and the save makes the Results immediately available to the Scoring Engine (FR-18) — no separate publish step (FR-23)

**Given** a Participant records which Team has the best regular-season record
**When** the save completes
**Then** it is immediately available to the Scoring Engine for Presidents' Trophy scoring (FR-19, FR-23)

**And** saving a whole Division's results (multiple Team marks at once) is one atomic write (Architecture AD-27 transaction composition)

**And** editable at any time during the Season, same last-write-wins behavior as Story 2.3

### Story 2.5: Enter Stanley Cup Winner Result

As a Participant,
I want to record which Team won the Stanley Cup,
So that the Scoring Engine can score everyone's Early and Late Cup Picks.

**Acceptance Criteria:**

**Given** the Management Page's Cup-winner section
**When** a Participant records the Stanley Cup–winning Team
**Then** the Team field uses the same autocomplete/validation as FR-5, and the save is immediately available to the Scoring Engine for both the Early Pick and Late Pick (FR-19, FR-24)

**And** the same editability and last-write-wins behavior as Story 2.3 applies

### Story 2.6: Enter Playoff Series Results

As a Participant,
I want to record the winning Team and exact game count for each concluded playoff Series,
So that the Scoring Engine can score everyone's Series predictions.

**Acceptance Criteria:**

**Given** the Management Page's series-results section
**When** a Participant records a Series' winning Team and game count (4-0/4-1/4-2/4-3)
**Then** the Team field uses the same autocomplete/validation as FR-5, the game count is restricted to the four valid values, and the save is immediately available to the Scoring Engine (FR-20, FR-25)

**And** the same editability and last-write-wins behavior as Story 2.3 applies

---

**Epic 2 summary:** 6 stories, FR-13, FR-23–26 fully covered. (FR-10/FR-27 — Send Reminder Email — moved to Epic 3, Story 3.4: it needs `internal/predictions`' completion-check, which doesn't exist until Epic 3, so keeping it here would have made Epic 2 depend on Epic 3 to function.)

## Epic 3: Predictions

Participants can enter, incrementally save, and edit their predictions across all 6 prediction types before each Deadline; predictions lock automatically at the Deadline; a Participant always sees their own picks and sees others' picks only once locked; and anyone can nudge whoever hasn't finished predicting.

### Story 3.1: Enter and Edit Predictions

As a Participant,
I want to enter and revise my predictions across all 6 prediction types,
So that I can lock in my picks before each Deadline, at my own pace.

**Acceptance Criteria:**

**Given** the Enter Predictions form for any of the 6 Prediction types (Award finalist, Team mark, Series, Early Pick, Late Pick, Presidents' Trophy)
**When** a Participant types a partial Team or Player name
**Then** matching canonical entries from the current Season's list surface, and submitting a value not present in that list is rejected with an inline error — the Prediction is not saved (FR-5)

**Given** a Series Prediction
**When** a Participant saves it
**Then** the winner and game count save together as one unit — a Series cannot be half-saved with only one of the two set (FR-6)

**Given** any single Prediction
**When** a Participant saves it
**Then** no other Prediction sharing the same Deadline needs to be filled in first — each saves independently (FR-6)

**Given** a Prediction already has a saved value and its Deadline hasn't closed
**When** a Participant saves a new value for it
**Then** the new value overwrites the previous one, with no edit history required (FR-7)

**And** a partially completed set of Predictions persists across sessions until its Deadline closes

### Story 3.2: Lock Predictions at Deadline

As a Participant,
I want my Predictions to lock automatically at their Deadline,
So that the competition stays fair — nobody can change a pick after it should be final.

**Acceptance Criteria:**

**Given** a Deadline has closed (per Epic 2's Set Deadlines)
**When** the owning Participant attempts to edit a Prediction under it
**Then** the edit is rejected, even by its own owner, with no grace period and no admin override (FR-8)

**And** there is no mechanism, UI or otherwise, to reopen a closed Deadline

**Given** a required Prediction was left empty when its Deadline closed
**When** the Scoring Engine reads it (Epic 4)
**Then** it scores zero points for that item, and the Participant is not blocked from making Predictions under any later Deadline (FR-9)

### Story 3.3: Predictions Visibility

As a Participant,
I want to always see my own Predictions and see others' Predictions only once their Deadline has passed,
So that I can review my own picks anytime while the competition stays fair for everyone else.

**Acceptance Criteria:**

**Given** any of a Participant's own Predictions, locked or not
**When** they view the Predictions page
**Then** all of their own Predictions are visible, past and present (FR-11)

**Given** another Participant's Predictions under a Deadline that hasn't closed yet
**When** a Participant views the Predictions page
**Then** those Predictions are hidden entirely — not shown as blank or masked, simply not present in the view (FR-12)

**Given** a Deadline closes
**When** any Participant next views the Predictions page
**Then** all Participants' Predictions under that Deadline become visible to everyone (FR-12)

### Story 3.4: Send Reminder Email

As a Participant,
I want to trigger a reminder email for a specific Deadline,
So that anyone who hasn't finished predicting gets a nudge.

**Acceptance Criteria:**

**Given** a Deadline that hasn't closed yet (set via Epic 2, Story 2.2)
**When** a Participant clicks "Send Reminder Email" for that Deadline
**Then** every Participant who has not yet completed all Predictions under it (per this epic's Stories 3.1–3.3) receives a plain call-to-action email ("action required until TIMESTAMP") with no other content (FR-10, FR-27)

**Given** a Participant has already completed every Prediction under that Deadline
**When** the button is clicked
**Then** that Participant is not sent the email, even though the button was clicked (FR-10)

**And** the action can be triggered any number of times before the Deadline closes — no cooldown, no maximum-sends limit (FR-27)

**And** the send is synchronous — a direct call from the button's HTTP handler into this epic's own completion-check logic (Stories 3.1–3.3) and `internal/mailer` (built in Epic 1), no background ticker, no `ReminderLog` dedupe table (Architecture AD-33)

---

**Epic 3 summary:** 4 stories, FR-5–12 fully covered (FR-10 and FR-27 relocated here from Epic 2 — see Epic 2's summary note).

## Epic 4: Scoring & Standings

Participants see a live, always-accurate standings view (Total, Regular Season, Playoffs, Rank) computed automatically from their predictions and Epic 2's recorded results — no manual math, no stale data, ties share rank.

### Story 4.1: Award Scoring (Tie-Inclusive)

As a Participant,
I want my award predictions scored automatically, including tie cases,
So that a correct pick always earns its points, even when the real trophy result has a tie.

**Acceptance Criteria:**

**Given** the actual top-3 for an Award (from Epic 2's Story 2.3 data) has no ties
**When** the Scoring Engine computes a Participant's Award points
**Then** each of their 3 predicted names scores 5 points if it appears anywhere in that top-3, order-independent (FR-17)

**Given** a tie at any position expands the qualifying set (e.g., a 3-way tie for 3rd place makes 5 total qualifying names)
**When** the Scoring Engine computes a Participant's Award points
**Then** a Participant who predicted 0 of the top 2 but all 3 tied-for-3rd names scores 15 points (3 × 5) for that Award (FR-17)

**And** this applies identically to all 5 Awards in v1 — Art Ross and Rocket Richard score against Epic 2's manually entered data exactly like Hart, Norris, and Vezina, with no different code path

**And** points are computed live on every read from stored Predictions and Award Results — no score value is ever written to storage (Architecture AD-16)

### Story 4.2: Regular-Season Team and Cup/Presidents' Trophy Scoring

As a Participant,
I want my regular-season team marks and Cup/Presidents' Trophy picks scored automatically,
So that my Total reflects them correctly without anyone doing the math.

**Acceptance Criteria:**

**Given** a Team-makes-playoffs mark
**When** that Team actually made the playoffs (Epic 2's Story 2.4 data)
**Then** it scores 5 points (FR-18)

**Given** a division-winner mark
**When** that Team actually won its division
**Then** it scores 15 points, replacing (not adding to) the 5-point playoff-mark score; if that Team only made the playoffs, it scores 5 points instead of 20 (FR-18)

**Given** the Early Pick or Late Pick
**When** the predicted Team actually won the Stanley Cup (Epic 2's Story 2.5 data)
**Then** each scores 20 points independently (FR-19)

**Given** the Presidents' Trophy pick
**When** the predicted Team actually has the best regular-season record (Epic 2's Story 2.4 data)
**Then** it scores 20 points (FR-19)

**And** Early Pick and Late Pick points are attributed to the Playoffs bucket, not Regular Season, since they resolve during the playoffs (FR-19)

### Story 4.3: Playoff Series Scoring

As a Participant,
I want my playoff series predictions scored automatically, with more credit for an exact result,
So that calling the exact outcome is worth more than just picking the winner.

**Acceptance Criteria:**

**Given** a Series prediction and its actual result (Epic 2's Story 2.6 data)
**When** the Scoring Engine computes points, by round
**Then** it awards: Round 1 correct-winner 15 / exact-result 25; Round 2 correct-winner 25 / exact-result 35; Conference Finals correct-winner 30 / exact-result 45; Stanley Cup Final correct-winner 30 / exact-result 50 (FR-20)

**Given** a Participant correctly predicted both the winner and the exact game count
**When** points are computed
**Then** the exact-result points replace the correct-winner points — they are not additive (FR-20)

### Story 4.4: Live Standings Display

As a Participant,
I want to see live, always-accurate standings with shared ranks on ties,
So that I always know exactly where everyone stands, with no ambiguity about who's ahead.

**Acceptance Criteria:**

**Given** any point in the Season
**When** a Participant views Standings
**Then** it shows, per Participant: Regular Season points, Playoffs points, and Total (their sum), reflecting the Scoring Engine's latest computation at that exact moment (FR-22)

**Given** two or more Participants have equal Total points
**When** Standings are displayed
**Then** they share the same rank — no secondary tiebreaker is applied (FR-21)

**And** there is no separate "live/projected" tier — the displayed Standings always equal what the Scoring Engine last computed from Epic 2's entered Results, nothing more current and nothing stale-but-labeled-current (FR-22)

**And** a displayed Standings value always matches what a manual hand-check of the underlying Results and Predictions would produce — no code path computes or displays a score independently of the Scoring Engine (FR-22)

---

**Epic 4 summary:** 4 stories, FR-17–22 fully covered.
