---
stepsCompleted: [step-01, step-02, step-03, step-04]
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-14/prd.md
  - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-14/addendum.md
  - _bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md
  - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-14/DESIGN.md
  - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-14/EXPERIENCE.md
  - _bmad-output/specs/spec-fantasy-hockey/SPEC.md
  - _bmad-output/specs/spec-fantasy-hockey/scoring-rules.md
---

# Fantasy Hockey - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for Fantasy Hockey, decomposing the requirements from the PRD, UX design contract, and Architecture spine into implementable stories.

## Requirements Inventory

### Functional Requirements

```
FR-1: A visitor can request a login code by submitting an email address; the response is identical regardless of whether it matches a Player, and a matching Player is emailed a 6-digit code.
FR-2: A Player who requested a code can submit it and be logged in; a code is valid for 10 minutes or one use; requesting a new code doesn't invalidate an earlier valid one; wrong/expired/used codes are rejected identically.
FR-3: A Player's session stays active while in use and expires after 30 minutes of inactivity; any request resets the countdown.
FR-4: A Player can log out, ending their session immediately on that device.
FR-5: [Prototype-validated] The app always acts as the logged-in Player and only ever lets them edit their own picks; header shows Player name + season; no Player switcher.
FR-6: A Player can see all Prediction sets grouped by phase ("Before the season" / "Playoffs"), each with title, subtitle, deadline (Europe/Berlin timezone) and relative countdown.
FR-7: Each Prediction set shows a clear status: Open, Submitted, Closed, or Upcoming.
FR-8: A Prediction set locks automatically at its deadline with no grace period and no override for any Player.
FR-9: A Player can revise a submitted-but-open set until its deadline; any subset of fields saves independently, except a Series pick's winner+game-count, which save together as one unit.
FR-10: A later Playoff round is shown but disabled/locked until its matchups exist in fantasy-hockey.yml.
FR-11: A prediction left empty at its deadline scores zero for that item and doesn't block other items.
FR-12: Every Prediction set's deadline is read directly from fantasy-hockey.yml; there is no in-app settings page for it, and no fixed pattern for how deadlines relate to each other across sets.
FR-13: A Player can pick the Stanley Cup winner (Cup champion pick) before the season, from all 32 teams grouped by division.
FR-14: A Player can pick the Presidents' Trophy winner from all 32 teams.
FR-15: A Player can choose which teams make the playoffs per Division; each Conference must total exactly 8 across a 4/4 or 5/3 split; selection is capped live; submit is blocked with an explanatory message until valid.
FR-16: A Player can pick one Division winner per Division (4 total), scoped to that Division's teams.
FR-17: A Player can pick 3 finalists for each of 5 individual awards, with autocomplete scoped by position that rejects a non-matching NHL Player name.
FR-18: A Player can re-pick the Stanley Cup winner (Playoffs Cup pick) once the Playoff field is set, as its own Prediction set with its own deadline.
FR-19: For each unlocked-round Series, a Player can pick the winning team and exact game count (4-7); winner and game-count save together as one unit; Round 1 has 8 Series.
FR-20: A later Playoff round opens only once its real matchups are recorded in fantasy-hockey.yml.
FR-21: Each Series is labeled by Conference ("Eastern Conference"/"Western Conference"), except the final round, labeled "Stanley Cup Final".
FR-22: [DEFERRED, not in MVP] A Player can trigger a reminder email for a Prediction set that hasn't closed yet; emails only Players who haven't completed every prediction under that set; plain nudge content; unlimited repeat triggers.
FR-23: A Player can see a Leaderboard of all Players with Regular, Playoff, and Total points; Total is the deciding, visually emphasized column; equal Totals share the same rank with no tiebreaker; always live, never cached.
FR-24: The app computes results and scores automatically from fantasy-hockey.yml per the point-value table (see scoring-rules.md / PRD FR-24); all 5 awards score identically; an empty required pick scores zero (FR-11).
FR-25: A Player can select which Prediction set to compare, grouped into two rows ("Before the season"/"Playoffs"); the chosen set's deadline is displayed.
FR-26: A Player can see all Players' picks for the chosen set shown side by side, with the current Player's own column visually distinguished.
FR-27: Division-pick comparisons break out one row per Division, with that Division's winner directly beneath it.
FR-28: Pick values render consistently: team-abbreviation values as tags, full team names as plain text, empty values as a faint placeholder.
FR-29: A Player can navigate between Predict, Leaderboard, and Compare via persistent bottom navigation with the active destination highlighted.
FR-30: The header and navigation stay fixed while only the content scrolls.
FR-31: The app is a single-column, phone-width, dark-theme-only layout; no light mode.
FR-32: A Player's picks are saved and everyone's picks are visible to everyone under the same rules, shared across devices.
FR-33: The current season's canonical team list and NHL Player candidate list (for award-finalist autocomplete) are available everywhere they're used.
FR-34: A new pool starts each season while prior seasons' data is retained; no in-app season-selector or history view for v1.
```

### NonFunctional Requirements

```
NFR-1: Session cookies are stateless, HMAC-signed, HttpOnly, and SameSite=Lax; Secure is added only once the app is behind a TLS-terminating reverse proxy (Architecture AD-11, AD-14).
NFR-2: All persisted state lives in a single fantasy-hockey.yml file with no database engine; exactly one process writes it at a time, via atomic write-and-rename (Architecture AD-9, AD-27, AD-29).
NFR-3: internal/store's in-memory structure and mutex are never exported; every read is lock-synchronized against every write (Architecture AD-29) -- prevents data races between concurrent Leaderboard reads and prediction-save writes.
NFR-4: The UI is server-rendered HTML only (stdlib html/template, net/http ServeMux) with no JS framework, SPA/API split, or ORM; the only client-side JS is the autocomplete widget and Division-pick live-cap disabling (Architecture AD-10).
NFR-5: Every persisted/exchanged timestamp is RFC3339 via internal/clock.NowTime().UTC().Format(time.RFC3339) (Architecture AD-16).
NFR-6: Errors are wrapped with fmt.Errorf("context: %w", err); no custom error envelope (Architecture AD-18).
NFR-7: The build pipeline gates on golangci-lint, gocyclo (complexity limit 10), go-licenses, and govulncheck; a failing gate blocks the build (Architecture AD-5).
NFR-8: The container is a multi-stage Alpine build running as non-root, targeting arm64 for Raspberry Pi deployment; the app serves plain HTTP, with TLS handled by a reverse proxy added later (Architecture AD-6, AD-14).
NFR-9: No in-app alerting -- failures surface only through externally exported logs (Architecture consistency conventions).
NFR-10: Mobile-first, dark-theme-only UI; no other device sizes or light mode are v1 targets (PRD FR-31, UX DESIGN.md Brand & Style).
NFR-11: Every identity a Player picks by name (Team, NHL Player) is referenced everywhere by the same stable id, never free-text display name -- the rule that exists specifically to prevent the name-typo scoring gap this rebuild is for (Architecture AD-17, AD-19).
```

### Additional Requirements

```
- No starter/greenfield scaffolding tool -- this is a brownfield-lite build: src/ already has main.go, internal/server (HTTP bootstrap), internal/web (placeholder), and internal/clock scaffolded. Epic 1 Story 1 extends this existing skeleton rather than starting from an empty repo.
- Session-signing secret (SESSION_SECRET) is a required runtime env var; the process fails to start if unset (Architecture AD-22).
- SMTP config (SMTP_HOST/SMTP_PORT/SMTP_USERNAME/SMTP_APP_PASSWORD) is NOT required at startup -- unlike SESSION_SECRET, the app must start with all four unset so `task go:run` keeps working outside docker-compose; a send-time call with SMTP_HOST unset logs an error instead of silently failing or defaulting (Architecture AD-12).
- Local dev docker-compose needs a mailpit (or equivalent MailHog-style) SMTP-capture service with a web UI, wired via the same env-configurable SMTP_HOST/PORT so the same code path works in dev and prod (PRD addendum.md; Architecture AD-12, Structural Seed).
- Data-file location resolves via DATA_FILE env var or --data-file CLI flag, flag wins; first-run bootstrap creates the file with an initial skeleton (current season + fixed player list) if it doesn't exist (Architecture AD-25, AD-26).
- Season rollover: at season start, a human archives the current fantasy-hockey.yml and repoints DATA_FILE at a fresh file; first-run bootstrap creates the new season's skeleton there (Architecture AD-30) -- flagged as an open question in SPEC.md whether this manual process is acceptable long-term.
- Image build/publish already exists (.github/workflows/release.yml, protected file, pushes to Docker Hub); the Raspberry Pi host's own image-pull/update mechanism is not yet decided (Architecture Deferred).
- depguard import-boundary linting (Architecture AD-21) is specified but not yet configured in .golangci.yml -- a near-term build task, not yet real enforcement.
- Prediction row granularity: one store.Prediction row per independently-saveable pick, with the Series winner+game-count pair as the sole multi-field exception (Architecture AD-28).
- AwardFinalist is a real Go struct ({slug, display_name}), not a bare string list; NHL Player identity uses a human-readable slug generated once and reused everywhere it's referenced (Architecture AD-17, AD-24).
- The shared autocomplete widget always submits the selected option's `id` (team abbreviation or NHL Player slug), never its display label (Architecture AD-19).
- internal/standings may import internal/scoring (the one sanctioned feature-to-feature import) to consume computed point values for the Leaderboard, rather than re-deriving scoring logic independently (Architecture AD-8).
```

### UX Design Requirements

```
UX-DR1: Implement the full dark-only color token system (bg/surface/raised/border/border-soft/text/muted/faint/ice/ice-deep/sel/green/green-btn/green-btn-border/gold/goal) per DESIGN.md Colors -- ice reserved for selection/active-nav only as a border (never a fill), gold reserved for the Leaderboard leader only, goal reserved for errors/Closed state only.
UX-DR2: Implement the typography system -- system sans for UI text, monospace+tabular-nums for the Leaderboard/any aligned numbers, the 6-step size scale (11-19px), semibold labels/buttons, bold-to-extrabold titles/totals, sentence case everywhere (no all-caps).
UX-DR3: Implement the shape/spacing tokens -- rounded/lg (~10px) for buttons/chips/inputs, rounded/xl (~12px) for cards/containers, full-circle for the rank badge only; 12-16px page gutters, 12px card padding; no shadows or gradients anywhere (elevation via surface-color steps only).
UX-DR4: Build the Header component -- pinned, player name (bold) left, season label (muted) right, no logo, surface background with bottom border.
UX-DR5: Build the Bottom Navigation component -- 3 pinned tabs (Predict/Leaderboard/Compare) with icons (checklist/medal/people), active tab in ice, inactive muted, surface background with top border.
UX-DR6: Build the Set Row component (Predict screen) -- 3px left accent stripe by state, status pill, subtitle, deadline line with countdown colored ice (open/submitted) or faint (upcoming), chevron/lock affordance.
UX-DR7: Build the Division-Picks Progress Indicator component -- live n/5 per-division counter and n/8 per-conference indicator (check when valid, dot when not, always paired with the count text, never a bare dot), chips dimming to 40% opacity when a scope is capped.
UX-DR8: Build the Chip component (team selection, Compare selector) and the Number Button component (series game-count) with identical selected-state styling (sel fill + ice border) -- winner and game-count picks must read as equally important, no distinguishing color.
UX-DR9: Build the Input/Dropdown Field component (Login fields, single-team dropdowns, division-winner dropdowns, award-finalist text inputs) -- raised fill, border, lg radius, text/faint colors.
UX-DR10: Build the Leaderboard Table component -- raised heading row, border-soft dividers, Total column visually set apart with a left border + raised tint, leader's rank badge and Total in gold, no "(you)" marker on any name.
UX-DR11: Build the Compare Table component -- one column per Player with the current Player's column visually distinguished, per-division breakout rows, tag-vs-plain-text value formatting, faint em-dash for empty values.
UX-DR12: Build the two Login screens (email request, code entry) per mockups/login-email.html and mockups/login-code.html -- full-screen, pre-shell, no header/nav; email screen has a single field + Send code button; code screen has a single 6-digit field + Log in button with an identical error state for wrong/expired/used codes.
UX-DR13: Implement Accessibility Floor -- tap targets >= 32px (36px for number buttons), color never the sole signal (status pills carry the word, validity pairs a check/dot with text), tabular numerals for anything that updates, form inputs labeled, focus order follows visual order.
UX-DR14: Implement State Patterns per EXPERIENCE.md -- Set states (Open/Submitted/Closed/Upcoming), Division-picks invalid state (red caption), empty-value placeholder, Compare's gated-round dashed "matchups not set" state, Login wrong/expired/used-code state, and the two explicitly-flagged [ASSUMPTION] states needing confirmation: session-timeout redirect behavior and Compare's default/initial selection.
UX-DR15: Implement Interaction Primitives -- tap-only (no swipe/drag), immediate local selection feedback with no page reload for chip/dropdown/button picks, input-time validation for live caps + submit-time validation otherwise, no page reload for Prediction sheet open/close.
```

### FR Coverage Map

```
FR-1: Epic 1 - Request a login code
FR-2: Epic 1 - Enter the login code
FR-3: Epic 1 - Session timeout
FR-4: Epic 1 - Log out
FR-5: Epic 1 - Single-player context
FR-6: Epic 2 - Browse Prediction sets
FR-7: Epic 2 - Set lifecycle & status
FR-8: Epic 2 - Deadline enforcement
FR-9: Epic 2 - Edit until deadline
FR-10: Epic 3 - Upcoming rounds are gated
FR-11: Epic 2 - Empty pick scores zero
FR-12: Epic 2 - Deadlines are data-driven
FR-13: Epic 2 - Cup champion pick
FR-14: Epic 2 - Presidents' Trophy pick
FR-15: Epic 2 - Division picks: playoff teams
FR-16: Epic 2 - Division picks: division winners
FR-17: Epic 2 - Player awards (finalists)
FR-18: Epic 3 - Playoffs Cup pick
FR-19: Epic 3 - Per-round series predictions
FR-20: Epic 3 - Round unlocking
FR-21: Epic 3 - Matchup context labels
FR-22: Deferred - Send a reminder email (not in MVP, no epic)
FR-23: Epic 4 - Leaderboard
FR-24: Epic 4 - Automatic scoring
FR-25: Epic 5 - Choose what to compare
FR-26: Epic 5 - Side-by-side table
FR-27: Epic 5 - Division comparison granularity
FR-28: Epic 5 - Consistent value formatting
FR-29: Epic 1 - Bottom navigation
FR-30: Epic 1 - Fixed app shell
FR-31: Epic 1 - Mobile-first, dark theme
FR-32: Epic 1 & 2 - Persistent, shared storage (session/login persistence in Epic 1; prediction persistence in Epic 2)
FR-33: Epic 2 - Season's canonical team and NHL Player lists
FR-34: Epic 6 - New pool each season, history kept
```

## Epic List

### Epic 1: Account Access & App Shell
A player can log into the app on their phone via emailed code, stay logged in through a sliding-timeout session, log out, and land in the persistent header/bottom-nav shell used all season.
**FRs covered:** FR-1, FR-2, FR-3, FR-4, FR-5, FR-29, FR-30, FR-31, FR-32 (partial: session/login persistence)

### Epic 2: Before-the-Season Predictions
A player can browse all prediction sets, fill in and submit all four before-season predictions (Cup champion, Presidents' Trophy, Division picks, Player awards) before their deadlines, edit any subset of fields until locked, and have it all persist and sync across devices.
**FRs covered:** FR-6, FR-7, FR-8, FR-9, FR-11, FR-12, FR-13, FR-14, FR-15, FR-16, FR-17, FR-32 (partial: prediction persistence), FR-33

### Epic 3: Playoff Predictions
A player can re-pick the Cup winner once the playoff field is set, and predict each series' winner and exact game count as rounds unlock with real matchups.
**FRs covered:** FR-10, FR-18, FR-19, FR-20, FR-21

### Epic 4: Scoring & Leaderboard
A player can see a live, always-accurate leaderboard of every player's Regular, Playoff, and Total points, computed automatically from recorded predictions and results.
**FRs covered:** FR-23, FR-24

### Epic 5: Compare Predictions
A player can select any prediction set and see every player's picks for it side by side, with division picks broken out and consistent value formatting.
**FRs covered:** FR-25, FR-26, FR-27, FR-28

### Epic 6: Season Rollover & Multi-Season History
The pool can restart cleanly for a new NHL season without losing prior seasons' data.
**FRs covered:** FR-34

*(FR-22, deadline reminder emails, is explicitly deferred out of MVP per the PRD/UX/SPEC and has no epic.)*

## Epic 1: Account Access & App Shell

A player can log into the app on their phone via emailed code, stay logged in through a sliding-timeout session, log out, and land in the persistent header/bottom-nav shell used all season.

### Story 1.1: Request a Login Code

As a visitor,
I want to request a login code by submitting my email address,
So that I can log in without a password.

**Acceptance Criteria:**

**Given** the login screen (email step)
**When** I submit an email address that matches a Player
**Then** a 6-digit numeric code is generated, hashed (sha256), and stored as a LoginCode entry (Player, code_hash, issued_at, used_at=null), and the same code is emailed to that address
**And** the HTTP response is identical to the case below (no way to tell from the response whether the email matched)

**Given** the login screen (email step)
**When** I submit an email address that does NOT match any Player
**Then** no code is generated or emailed, and the response is identical to the matching case
**And** nothing about timing or response shape reveals which case occurred

**Given** I already have an earlier, still-valid code
**When** I request a new code for the same email
**Then** a new LoginCode entry is added and the earlier one remains valid — requesting a new code never invalidates an existing valid one

*References: PRD FR-1; Architecture AD-1, AD-2, AD-3, AD-9, AD-17, AD-20, AD-25, AD-26; UX-DR12 (mockups/login-email.html); UX EXPERIENCE.md Voice and Tone (identical neutral confirmation copy).*

### Story 1.2: Enter Login Code and Establish Session

As a player who requested a code,
I want to submit it and be logged in,
So that I can start using the app.

**Acceptance Criteria:**

**Given** I am on the login screen (code step) with a valid, unused, unexpired code
**When** I submit that code
**Then** my submission is hashed and matched against a LoginCode entry for my email with used_at still null and issued_at within the last 10 minutes
**And** on match, the app sets a stateless HMAC-signed session cookie (HttpOnly, SameSite=Lax) carrying my Player identity + issued-at, marks the LoginCode's used_at, and I land on the Predict screen inside the app shell

**Given** I submit a code that is wrong, expired (older than 10 minutes), or already used
**When** the submission is validated
**Then** I see the same generic error ("That code didn't work — check it and try again.") in all three cases, with no indication which one applied, and I remain on the code-entry screen

**Given** SESSION_SECRET is unset in the environment
**When** the app starts
**Then** it fails to start rather than generating a signing key in-process

*References: PRD FR-2; Architecture AD-11, AD-17, AD-20, AD-22; UX-DR12, UX-DR14 (Login: wrong/expired/used code state); UX EXPERIENCE.md Component Patterns (Login — code field).*

### Story 1.3: Stay Logged In With Sliding Session Timeout

As a player,
I want my session to stay active while I'm using the app and expire when I've stepped away,
So that a shared or left-open device doesn't stay logged in forever.

**Acceptance Criteria:**

**Given** I am logged in and make any authenticated request
**When** the request completes
**Then** my session cookie is re-issued with a fresh issued-at, sliding my idle timeout forward by 30 minutes

**Given** I have been idle for more than 30 minutes since my last request
**When** I make another request
**Then** I am treated as logged out and routed to the login (email) screen with no special "you were logged out" messaging

*References: PRD FR-3; Architecture AD-11; UX-DR14 ([ASSUMPTION] Session timeout state); UX EXPERIENCE.md State Patterns.*

### Story 1.4: Log Out

As a player,
I want to log out,
So that my session ends immediately on this device.

**Acceptance Criteria:**

**Given** I am logged in
**When** I trigger logout
**Then** my session cookie is cleared/stops being re-issued and my very next request is treated as unauthenticated, routing me to the login (email) screen

*References: PRD FR-4; Architecture AD-11.*

### Story 1.5: Persistent App Shell With Player Identity and Navigation

As a player,
I want a fixed header and bottom navigation that stay in place while only the content scrolls,
So that I always know who I'm logged in as and can always reach Predict, Leaderboard, and Compare.

**Acceptance Criteria:**

**Given** I am logged in and on any of Predict, Leaderboard, or Compare
**When** the page renders
**Then** a pinned header shows my Player name (bold, left) and the season label (muted, right, e.g. "NHL 2026–27"), with no logo and no player switcher
**And** a pinned bottom navigation shows exactly three tabs (Predict/checklist icon, Leaderboard/medal icon, Compare/people icon), with the active tab shown in the `ice` accent and others muted
**And** only the middle content region scrolls — header and navigation never move

**Given** I am on a phone-width viewport (~360–430px)
**When** any authenticated screen renders
**Then** the layout is single-column, dark-theme only, with no light-mode setting reachable anywhere in the app
**And** on a wider viewport the column stays centered at the same max phone width rather than stretching

*References: PRD FR-5, FR-29, FR-30, FR-31; UX-DR4, UX-DR5, UX-DR13; UX DESIGN.md Components (Header, Bottom navigation); UX EXPERIENCE.md Foundation.*

## Epic 2: Before-the-Season Predictions

A player can browse all prediction sets, fill in and submit all four before-season predictions before their deadlines, edit any subset of fields until locked, and have it all persist and sync across devices.

### Story 2.1: Browse Prediction Sets by Phase and Status

As a player,
I want to see all prediction sets grouped by phase with a clear status,
So that I know what I can predict and when.

**Acceptance Criteria:**

**Given** I am logged in and open Predict
**When** the screen renders
**Then** I see two sections, "Before the season" and "Playoffs", each with a section-header icon (target / trophy)
**And** each set shows a title, subtitle, deadline (date + time in Europe/Berlin) with a relative countdown, and a status pill: Open (blue), Submitted (green), Closed (red), or Upcoming (grey)
**And** an Open/Submitted/Closed row shows a chevron; an Upcoming row is dimmed, shows a lock icon, and is not tappable

**Given** a set's deadline value in `fantasy-hockey.yml`
**When** the Predict screen renders that set
**Then** the deadline/countdown shown comes directly from that file — there is no in-app settings page for it, and no assumption that any two sets share a deadline

*References: PRD FR-6, FR-7, FR-12; Architecture AD-23 (read-only); UX-DR6, UX-DR14 (set status states); UX EXPERIENCE.md Component Patterns (Set row), Information Architecture.*

### Story 2.2: Load Season's Canonical Team List

As a player,
I want the current season's 32 NHL teams available wherever I pick a team,
So that my predictions and results use the same validated names.

**Acceptance Criteria:**

**Given** `fantasy-hockey.yml` contains a hand-maintained `teams` section (id = standard abbreviation, e.g. "TOR")
**When** the app starts or a page needing team data renders
**Then** `internal/store` exposes the full team list, grouped by conference/division, via a read-only method — no code path ever writes to this section

**Given** a page embeds the team candidate list for a dropdown
**When** it renders
**Then** the embedded JSON uses the shared `{"id": "<abbreviation>", "label": "<display name>"}` shape, matching every other embedding site

*References: PRD FR-33; Architecture AD-17, AD-19, AD-23; UX-DR9.*

### Story 2.3: Cup Champion and Presidents' Trophy Picks

As a player,
I want to pick the Stanley Cup winner and the Presidents' Trophy winner before the season,
So that they count as my season-opening calls.

**Acceptance Criteria:**

**Given** the "Cup champion" or "Presidents' Trophy" set is Open
**When** I tap it
**Then** a full-screen Prediction sheet opens (pinned header with back arrow/title/deadline+countdown; scrolling body; pinned action bar) with a single team dropdown grouped by division, teams sourced from Story 2.2
**And** choosing a team shows a green check on the label; tapping Submit predictions saves my pick keyed by the team's `id` (never its display name) and returns me to Predict with the set now Submitted (green)

**Given** the set is Submitted and still before its deadline
**When** I reopen it
**Then** it opens in edit mode with my current pick pre-filled, the action button reads "Update predictions", and re-saving updates my pick

**Given** the set's deadline has passed
**When** I open it
**Then** it shows a read-only banner, every input is disabled, and there is no action bar — not even for the player who made the pick

**Given** I never made a pick for this set before its deadline
**When** results are scored later
**Then** this item scores zero without having blocked anything else I predicted

*References: PRD FR-8, FR-9, FR-11, FR-13, FR-14; Architecture AD-10, AD-17, AD-24, AD-28; UX-DR1–3, UX-DR9; UX EXPERIENCE.md Component Patterns (Single-team pick, Prediction sheet), Voice and Tone ("Editable until the deadline.").*

### Story 2.4: Division Picks — Playoff Teams and Division Winners

As a player,
I want to choose which teams make the playoffs per division and pick one winner per division,
So that my field reflects both conferences.

**Acceptance Criteria:**

**Given** the "Division picks" set is open
**When** I tap a team chip under a division
**Then** the chip toggles selected (sel fill + ice border) and the division's live `n/5` counter and the conference's live `n/8` indicator (check when valid, dot when not, always paired with the count) update immediately

**Given** a division already holds 5 selected teams, or a conference already holds 8
**When** I try to select another team in that scope
**Then** the remaining chips in that scope dim to 40% opacity and become non-tappable — the selection cannot exceed the cap

**Given** both conferences are not yet valid (not exactly 8 across a 4/4 or 5/3 split)
**When** I try to submit
**Then** the Submit button stays disabled and a red caption explains exactly what's missing

**Given** both conferences are valid
**When** I also pick one division winner per division (4 dropdowns, each scoped to that division's own teams) and submit
**Then** all picks save keyed by team `id`, and the set becomes Submitted

*References: PRD FR-15, FR-16; Architecture AD-17, AD-28; UX-DR7, UX-DR8, UX-DR14 (Division picks: invalid state), UX-DR15 (live-cap input-time validation); UX DESIGN.md Components (Division-picks progress indicator); UX EXPERIENCE.md Voice and Tone (the 4/4-or-5/3 hint text).*

### Story 2.5: Load Season's Canonical NHL Player List

As a player,
I want a working list of NHL players available for award-finalist autocomplete,
So that my picks and recorded results use the same validated names.

**Acceptance Criteria:**

**Given** `fantasy-hockey.yml` contains a hand-maintained NHL Player list, each entry with a human-readable slug (e.g. `mcdavid-connor`) and a display name
**When** the app starts or a page needing this data renders
**Then** `internal/store` exposes it as a real `AwardFinalist`-shaped struct (`{slug, display_name}`), scoped by position (skaters, defensemen, goalies) — never a bare list of name strings
**And** the embedded autocomplete JSON for this list uses the same `{"id": "<slug>", "label": "<display name>"}` shape as the team list (Story 2.2), and the shared widget always submits `id`

*References: PRD FR-33; Architecture AD-17, AD-19, AD-24; UX-DR9.*

### Story 2.6: Player Awards Finalists

As a player,
I want to pick 3 finalists for each of the 5 individual awards,
So that a typo can never silently fail to score.

**Acceptance Criteria:**

**Given** the "Player awards" set is open
**When** I type into a finalist field for Hart, Norris, Vezina, Art Ross, or Rocket Richard
**Then** autocomplete suggests matches scoped to that award's eligible position (skaters for Hart/Art Ross/Rocket Richard, defensemen for Norris, goalies for Vezina), sourced from Story 2.5

**Given** I type a name that doesn't match any suggestion
**When** I try to save that field
**Then** the input is rejected — shown with a `goal`-colored border and an inline caption naming the problem — rather than silently accepted as free text

**Given** all three finalists for one award are filled with valid names
**When** the group re-renders
**Then** that award group's label shows a green check

**Given** I submit the set with all 5 awards' finalists filled
**When** the save completes
**Then** each finalist is stored by NHL Player `id` (slug), and the set becomes Submitted

*References: PRD FR-17; Architecture AD-17, AD-19, AD-24; UX-DR9; UX EXPERIENCE.md Component Patterns (Player awards), State Patterns ([ASSUMPTION] Player awards: name rejected).*

## Epic 3: Playoff Predictions

A player can re-pick the Cup winner once the playoff field is set, and predict each series' winner and exact game count as rounds unlock with real matchups.

### Story 3.1: Playoffs Cup Pick

As a player,
I want to re-pick the Stanley Cup winner once the playoff field is set,
So that I get a second, better-informed call.

**Acceptance Criteria:**

**Given** the "Playoffs Cup pick" set, with its own deadline shortly before Round 1's
**When** it is Open
**Then** it behaves exactly like the single-team-pick sheet from Story 2.3 (dropdown of all 32 teams by division, check-on-selection, edit-until-deadline, read-only after)
**And** my pick is stored separately from the season-opening Cup champion pick (Story 2.3), never overwriting it

*References: PRD FR-18; reuses Story 2.3's Prediction-sheet mechanics.*

### Story 3.2: Round Unlocking Based on Recorded Matchups

As a player,
I want later playoff rounds to be unavailable until their matchups exist,
So that I don't predict blind.

**Acceptance Criteria:**

**Given** Round 2, the Conference Finals, or the Stanley Cup Final
**When** that round's matchups are NOT yet recorded under `playoff_matchups` in `fantasy-hockey.yml`
**Then** the round's set shows as Upcoming on Predict (dimmed, lock icon, not tappable), per Story 2.1's Upcoming treatment

**Given** a human directly adds that round's matchups to `fantasy-hockey.yml`
**When** I next load Predict
**Then** the round's set becomes Open — no in-app action, no admin screen, ever triggers this

*References: PRD FR-10, FR-20; Architecture AD-23.*

### Story 3.3: Per-Round Series Predictions

As a player,
for each series in an unlocked round I want to pick the winner and how many games it takes,
So that my prediction captures both outcome and length.

**Acceptance Criteria:**

**Given** an unlocked round's Prediction sheet
**When** it renders
**Then** series are grouped under "Eastern Conference" / "Western Conference" subheaders, or "Stanley Cup Final" for the final round, with the intro copy "For each series, tap the winner and how many games it takes."
**And** Round 1 shows exactly 8 series

**Given** one series card
**When** I tap a team button (winner) and a number button 4/5/6/7 (game count)
**Then** both use the identical selected styling (`sel` fill + `ice` border) — neither reads as more important
**And** the winner + game-count pair saves together as one atomic unit — I cannot save one half without the other, unlike every other field in the app

*References: PRD FR-19, FR-21; Architecture AD-28; UX-DR8, UX-DR15 (tap-only, immediate local feedback); UX EXPERIENCE.md Component Patterns (Playoff round series).*

## Epic 4: Scoring & Leaderboard

A player can see a live, always-accurate leaderboard of every player's Regular, Playoff, and Total points, computed automatically from recorded predictions and results.

### Story 4.1: Automatic Scoring Engine

As a player,
I want results and scores computed automatically from what's recorded in `fantasy-hockey.yml`,
So that nobody ever has to do the math by hand.

**Acceptance Criteria:**

**Given** predictions saved in Epics 2–3 and hand-maintained results in `fantasy-hockey.yml`
**When** `internal/scoring` computes a player's points
**Then** it applies the point table from `scoring-rules.md` exactly — award finalist 5pts per name found in the actual top-3 (ties expand the set), playoff-team pick 5pts, division winner 15pts (replacing, not adding to, the 5pt mark), Presidents' Trophy 20pts, Cup champion 20pts (Regular), Playoffs Cup pick 20pts (Playoff), and each round's series winner/exact-result values (15/25, 25/35, 30/45, 30/50, replacing not additive)
**And** it re-derives every value fresh on every call, directly from `internal/store`'s in-memory data — no score value is ever written to `fantasy-hockey.yml`

**Given** a required pick was left empty at its deadline (Epics 2–3's empty-scores-zero rule)
**When** scoring runs
**Then** that item contributes zero without erroring

**Given** `internal/standings` needs a player's Total
**When** it computes the Leaderboard
**Then** it calls `internal/scoring`'s exported functions directly (the one sanctioned feature-to-feature import) rather than re-deriving any point value itself

*References: PRD FR-24, SM-C1; Architecture AD-8 (sanctioned exception), AD-15, AD-18, AD-24, AD-28, AD-29; scoring-rules.md.*

### Story 4.2: Leaderboard Display

As a player,
I want a leaderboard of all players with Regular, Playoff, and Total points,
So that I can see who's winning.

**Acceptance Criteria:**

**Given** I open Leaderboard
**When** the screen renders
**Then** I see one row per player with Player, Regular, Playoff, and a visually emphasized Total column (left border + tint), sorted by Total descending, in tabular-numeral monospace
**And** the leader's rank badge and Total render in `gold`, and no row carries a "(you)" marker

**Given** two or more players share the same Total
**When** the Leaderboard renders
**Then** they share the same rank — no tiebreaker of any kind is applied

**Given** any new result or prediction affecting scoring
**When** I open or refresh Leaderboard
**Then** it reflects the latest computation live — there is no separate "in-progress"/projected tier

*References: PRD FR-23; Architecture AD-15; UX-DR10; UX EXPERIENCE.md Component Patterns (Leaderboard table), Open Items (sample-data caveat — real scoring is this story's, not the click-dummy's placeholder values).*

## Epic 5: Compare Predictions

A player can select any prediction set and see every player's picks for it side by side, with division picks broken out and consistent value formatting.

### Story 5.1: Compare Selector and Side-by-Side Table

As a player,
I want to select which prediction set to compare and see all players' picks side by side,
So that I can focus on one set at a time and compare at a glance.

**Acceptance Criteria:**

**Given** I open Compare
**When** the screen renders
**Then** a chip selector is grouped into two labelled rows, "Before the season" and "Playoffs", with all options visible (no horizontal scrolling)

**Given** I select a set's chip
**When** the table renders
**Then** it shows that set's deadline, one column per player (bold names), with my own column visually distinguished, and one labelled row per prediction category

*References: PRD FR-25, FR-26; UX-DR11; UX EXPERIENCE.md Component Patterns (Compare selector, Compare table), Voice and Tone ("Everyone's picks.").*

### Story 5.2: Division Comparison Granularity and Consistent Value Formatting

As a player,
I want division picks broken out per division and every pick value shown consistently,
So that the table stays readable and easy to scan.

**Acceptance Criteria:**

**Given** the compared set includes Division playoff-team picks
**When** the table renders
**Then** there is one row per division for the playoff-team picks, with that division's winner shown directly beneath it

**Given** any cell's value
**When** the table renders
**Then** team-abbreviation values (playoff teams, division winners, series winners) render as a consistent tag; full team names (Cup champion, Presidents' Trophy) render as plain text; series rows show the tag plus "in N"; a value nobody has entered yet shows a faint em-dash, never blank space

**Given** an upcoming, still-gated playoff round is somehow selected
**When** the table would render
**Then** it shows a dashed "matchups not set" note instead (not reachable in practice while the round stays gated per Epic 3)

*References: PRD FR-27, FR-28; UX-DR11, UX-DR14 (empty-value and gated-round states); UX EXPERIENCE.md State Patterns (Compare: upcoming round selected).*

## Epic 6: Season Rollover & Multi-Season History

The pool can restart cleanly for a new NHL season without losing prior seasons' data.

### Story 6.1: Season Rollover With Retained History

As a player,
I want a new pool to start each season while prior seasons' data is kept,
So that the app fully replaces "one Excel file per season."

**Acceptance Criteria:**

**Given** a human has archived the current `fantasy-hockey.yml` (e.g. renamed to `fantasy-hockey-2026-27.yml`) and repointed `DATA_FILE`/`--data-file` at a fresh path
**When** the app starts
**Then** `internal/store` finds no file at the new path and creates it with an initial skeleton (current season + the fixed player list, sourced out-of-band per Story 1.1's bootstrap dependency) — nothing is inherited from the archived file

**Given** a prior season's archived file still exists on disk/volume
**When** the app runs against the new season's file
**Then** it never reads from or writes to the archived file — there is no in-app cross-season query, season-selector, or history view in v1

**Given** the current season's file
**When** any screen (Predict, Leaderboard, Compare) renders
**Then** it shows only that one season's data throughout

*References: PRD FR-34, §6.2; Architecture AD-26, AD-30; SPEC.md Open Questions (confirm the manual archive/repoint process is acceptable long-term).*
