---
title: 'Fantasy Hockey — Experience'
status: final
created: '2026-09-05'
updated: '2026-09-05'
sources:
  - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md
  - _bmad-output/planning-artifacts/briefs/brief-fantasy-hockey-2026-09-04/brief.md
  - _bmad-output/brainstorming/brainstorm-fantasy-hockey-web-app-2026-09-04/brainstorm-intent.md
---

# Fantasy Hockey — Experience Spine

## Foundation

Responsive web, single codebase, targeting desktop/tablet/mobile browsers — no native apps, no separate mobile experience. No UI system named; the app is small enough to build directly against `DESIGN.md` tokens without inheriting a component library. `DESIGN.md` is the visual identity reference; this spine owns behavior. Dark-only (no light mode, no toggle).

## Information Architecture

| Surface | Reached from | Purpose | Mock |
|---|---|---|---|
| Login | App open, unauthenticated | Email → 6-digit code → in | [`mockups/key-login.html`](mockups/key-login.html) |
| Predictions (home) | Post-login default landing | Everyone's Predictions across all categories, respecting the visibility rule (PRD §4.4); includes a "My Predictions" entry point | [`mockups/key-predictions-home.html`](mockups/key-predictions-home.html) |
| Enter Predictions | "My Predictions" on the Predictions page | Entry/edit forms for all 6 Prediction types | [`mockups/key-enter-predictions.html`](mockups/key-enter-predictions.html) |
| Award Data Entry | Nav link | Manual Hart/Norris/Vezina finalist entry (PRD §4.5) — low-traffic, not prime nav real estate. Not an admin page: any Participant can use it under the same single Participant identity as everywhere else, no separate role or account (see `DESIGN.md` — no admin visual treatment exists because none is needed). | [`mockups/key-award-data-entry.html`](mockups/key-award-data-entry.html) |
| Logout | Header action, every authenticated page | Ends session immediately | — (action, not a screen) |

Standings is not a surface — it's a persistent widget (full-width bar) rendered above the nav on every authenticated page, per `DESIGN.md.Components.standings-widget` — see it in any authenticated mock above.

Flat IA on purpose: no tabs, no nested navigation, no modal stacks. Matches the "transparency without friction" principle carried from product discovery (PRD §4, intro).

→ Layout wireframe: [`wireframes/flow-uj1-2026-09-05.excalidraw`](wireframes/flow-uj1-2026-09-05.excalidraw) (UJ-1, low-fidelity, standings-widget placement). Spine tables above win over any mock or wireframe on conflict — the mocks illustrate, they don't govern.

## Voice and Tone

Plain and neutral throughout — no banter, no exclamation marks, even though the users are friends. Aesthetic posture lives in `DESIGN.md`; this is just the words.

| Do | Don't |
|---|---|
| "Check the entered email address." (PRD FR-1) | "Hmm, that doesn't look right!" |
| "Invalid code." | "Oops! Wrong code." |
| "Action required until {timestamp}." (PRD FR-10) | "Don't forget to make your picks! 🏒" |
| "Locked." | "🔒 This prediction is locked and can no longer be edited." |
| "Session expired." | "You've been logged out due to inactivity!" |
| Short, direct sentences. | Encouragement, emoji, exclamation marks. |

## Component Patterns

Behavioral. Visual specs live in `DESIGN.md.Components`.

| Component | Use | Behavioral rules |
|---|---|---|
| Login form | Login | One field visible at a time. Email field, submit → the email field is replaced by a code field in that same slot (same page, no navigation, no second field added alongside it) → submit → redirect to Predictions (home). Generic confirmation message regardless of whitelist match (PRD FR-1). |
| Standings widget | Every authenticated page | Always renders from the latest Scoring Engine state (PRD FR-22) — no loading spinner variant needed since it's server-rendered with the page; no "live" indicator since there is no live tier. Shows Rank alongside Regular Season/Playoffs/Total; Participants tied on Total render the same Rank value, side by side, with no visual distinction between them (PRD FR-21). |
| Prediction row | Predictions (home), Enter Predictions | One Series/Award/pick per row. Editable rows show input controls; locked rows show plain text only (no disabled-looking inputs). Each row saves independently (PRD FR-6) — no page-level Save button. An Award row shows each Participant's 3 finalists on one line as bordered Pick chips (`DESIGN.md` → Pick chip), alphabetically ordered left-to-right — never one comma-joined string or a stacked list. A Series row shows Winner and Game Count as two Pick chips on one line — never a blended "Team in N" string. Series rows within a round are ordered by NHL series seeding, not arbitrarily. |
| Autocomplete input | Enter Predictions, Award Data Entry | Suggestions filter as the Participant types; selecting a suggestion is the only way to set the field (PRD FR-5). A non-matching free-text value is rejected inline on blur/submit, not as a separate error page. |
| Button (primary) | Prediction row (Save), Predictions home ("My Predictions" entry point) | Per-row Save appears once the row's fields are valid (winner + game count both set for a Series, per FR-6). Saving is immediate — no separate "confirm" step, since edits remain changeable until the Deadline anyway. At most one primary button per screen. |
| Locked badge / read-only marker | Prediction row (locked state) | Renders in place of input controls once a Prediction's Deadline has closed — plain "Locked" text, not a disabled-looking control (PRD FR-8). Never paired with an icon or color beyond `text-secondary`. |
| Nav bar | Every authenticated page | Flat links: Predictions, Award Data, Logout. No icons, no collapse-to-hamburger — three links fit any viewport this app targets. |

## State Patterns

| State | Surface | Treatment |
|---|---|---|
| Not yet unlocked | Predictions (home), Enter Predictions | A round/category whose Deadline hasn't opened yet (e.g., Round 2 before Round 1 finishes) simply isn't shown — not a placeholder or "coming soon," per PRD's progressive-unlock rule. |
| Locked (post-deadline, own) | Predictions (home), Enter Predictions | Read-only text, `text-secondary`, "Locked" marker — never styled as an error or a disabled control (PRD FR-8). |
| Hidden (others', pre-deadline) | Predictions (home) | Not rendered at all — no blurred/masked placeholder implying "something is there" (PRD FR-12). |
| Missing at deadline | Predictions (home), post-lock | Shown as an empty/blank Prediction, scored 0 — not flagged or highlighted differently from a filled one; the score itself (once visible) tells the story (PRD FR-9). |
| Autocomplete rejection | Enter Predictions, Award Data Entry | Inline `danger`-bordered field + one-line `meta` message. No modal, no toast. |
| Invalid login code | Login | Generic "Invalid code." inline, code field cleared, no lockout. |
| Session expired | Any authenticated page | The next request after 30 minutes of inactivity (PRD FR-3) redirects to Login with a plain inline message: "Session expired." No auto-retry, no countdown warning beforehand — consistent with no loading/status chrome elsewhere. |
| Concurrent Award Data Entry edit | Award Data Entry | No visible conflict state at all — per PRD FR-13, the later save silently wins. Nothing in the UI ever indicates a conflict occurred. |

Award Data Entry (PRD FR-13) is exempt from the Deadline lifecycle entirely — it has no Deadline, so none of the Locked/Hidden/Missing states above ever apply to it; its fields are always editable.

There is no loading/sync-status state exposed anywhere in the UI — the Sync (PRD §4.6) is invisible by design; a page always simply shows the latest computed state, per FR-16/FR-22's no-in-app-indicator rule.

## Interaction Primitives

- Click/tap to act — no swipe gestures, no long-press, no drag-and-drop anywhere.
- Per-row save (see Component Patterns) is the only "commit" interaction in the app — there is no page-level submit for a batch of Predictions.
- Autocomplete is keyboard-navigable (arrow keys + enter) and pointer-selectable — this is the one interaction the whole "no typos" premise (PRD, brief) rests on, so it must work equally well with a mouse (desktop) and touch (tablet/phone).
- **Banned:** carousels, hero animations, badge/notification counts on nav items, any auto-refresh or polling UI (the Sync is invisible; the page reflects whatever the last request returned).

## Accessibility Floor

Sensible baseline only — no formal compliance target, per explicit decision (3 known users, no accessibility requirements stated).

- Contrast: body text and data figures meet at least WCAG AA contrast against their surface, even though full AA compliance isn't a stated goal — this is a floor, not a target to chase further.
- Full keyboard operability: every interactive element (nav links, autocomplete, save buttons, logout) reachable and operable via keyboard, since the autocomplete pattern must work this way regardless.
- Tap targets sized comfortably for touch (tablet/phone) — no precision-dependent small controls.
- Focus order follows visual/reading order on every surface.

## Responsive & Platform

Single responsive layout, no separate mobile design — confirmed via the UJ-1 wireframe ([`wireframes/flow-uj1-2026-09-05.excalidraw`](wireframes/flow-uj1-2026-09-05.excalidraw)).

- The standings widget stays a full-width top bar at every viewport width; it narrows/wraps its three rows but never moves to a sidebar, footer, or collapsible drawer.
- Content below the widget is single-column at every width — there is no multi-column desktop layout to maintain or keep in sync with mobile.
- Nav bar stays flat links at every width (three items never needs a hamburger).
- No platform-specific gestures or affordances (no pull-to-refresh, no native share sheets) — this is a plain responsive web app, not a PWA or hybrid shell.

## Inspiration & Anti-patterns

- **Rejected — sports-scoreboard visual clichés** (team-color gradients, ice/rink textures, trophy iconography): the product is about the friends' competition, not a broadcast-style presentation of the NHL itself.
- **Rejected — gamification chrome** (streaks, badges, celebratory animations on a correct pick): consistent with the PRD's counter-metric against optimizing for engagement — a quiet scoreboard, not a habit loop.
- **Lifted from plain data-table tools** (spreadsheets, admin dashboards): dense, tabular-aligned numbers over card-based/visual scorecards — the audience already reads a spreadsheet today and is comfortable with that density.

## Key Flows

### UJ-1 — Basti enters his Round 1 playoff picks

1. Basti logs in, lands on Predictions (home).
2. Checks the standings widget (top bar) — regular season points, freshly computed.
3. Opens "My Predictions" → Enter Predictions.
4. Works through each Round 1 Series row: picks a winner and game count, saves — each row independently (Component Patterns → Prediction row).
5. Turns to the Late Pick row, enters it.
6. **Climax:** every Round 1 Series row plus the Late Pick shows as saved (plain saved-state text, no confetti/animation per Inspiration & Anti-patterns).

Edge case: leaves mid-way through → returns later, unsaved rows are simply still editable (not "abandoned" or flagged) until the Round 1 Deadline actually closes.

### UJ-2 — Sadl checks how everyone stacks up after Round 1 locks

1. Sadl logs in, lands on Predictions (home).
2. Standings widget shows his updated rank.
3. Scrolls the Predictions page — Basti's and Tobbi's Round 1 picks are now rendered (no longer hidden, per State Patterns → Hidden/Locked).
4. Compares informally; takes no action in-app.

Edge case: if Sadl checks before the Deadline closes, the others' rows for that Deadline simply aren't in the page at all (not shown as locked-but-blank) — nothing to imply information exists that he can't see.

### Login — component pattern, not a named journey

Per explicit decision, the email → code → in flow is simple enough to spec directly in Component Patterns above rather than narrate as a UJ.
