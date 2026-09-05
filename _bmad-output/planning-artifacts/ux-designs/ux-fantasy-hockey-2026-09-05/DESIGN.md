---
title: 'Fantasy Hockey — Design'
status: final
created: '2026-09-05'
updated: '2026-09-05'
sources:
  - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md
  - _bmad-output/planning-artifacts/briefs/brief-fantasy-hockey-2026-09-04/brief.md
  - _bmad-output/brainstorming/brainstorm-fantasy-hockey-web-app-2026-09-04/brainstorm-intent.md
name: Fantasy Hockey
description: A private, 3-person NHL prediction game, dark-only, no gamification chrome — quiet utility for three friends who already know the rules.
colors:
  surface-base: '#151a1f'
  surface-raised: '#1c2229'
  border-hairline: '#2b333b'
  accent: '#6f9bb8'
  text-primary: '#e2e6ea'
  text-secondary: '#8b98a3'
  danger: '#c97b72'
typography:
  heading:
    fontFamily: 'system-ui, -apple-system, "Segoe UI", sans-serif'
    fontSize: '1.25rem'
    fontWeight: 600
  body:
    fontFamily: 'system-ui, -apple-system, "Segoe UI", sans-serif'
    fontSize: '1rem'
    fontWeight: 400
  meta:
    fontFamily: 'system-ui, -apple-system, "Segoe UI", sans-serif'
    fontSize: '0.8rem'
    fontWeight: 400
  data:
    fontFamily: 'system-ui, -apple-system, "Segoe UI", sans-serif'
    fontSize: '0.95rem'
    fontWeight: 500
    note: 'tabular-nums for point/score alignment in the standings widget and predictions tables'
rounded:
  sm: 4px
  md: 8px
  DEFAULT: 4px
spacing:
  '1': 4px
  '2': 8px
  '3': 12px
  '4': 16px
  '5': 24px
  '6': 32px
components:
  standings-widget:
    background: '{colors.surface-raised}'
    border: '{colors.border-hairline}'
    text: '{colors.text-primary}'
  button-primary:
    background: '{colors.accent}'
    text: '{colors.surface-base}'
    rounded: '{rounded.sm}'
  input-autocomplete:
    background: '{colors.surface-base}'
    border: '{colors.border-hairline}'
    rounded: '{rounded.sm}'
  badge-locked:
    text: '{colors.text-secondary}'
    background: 'transparent'
---

## Brand & Style

Fantasy Hockey is a private tool for three friends, not a product trying to win anyone's attention. It should feel like a well-kept scoreboard, not an app fighting for engagement — quiet, dense with the numbers that matter, and out of the way otherwise. Dark by default and dark only for v1: this is used at night after games, on a phone or a laptop, and there is no reason to make it fight the room's lighting.

The register is "Steel Ice" — muted, utilitarian steel-blue calm, not the moody near-navy or icy-teal drama other directions leaned into. No logos, no imagery, no hockey-arena visual cliché (no ice textures, no jersey stripes, no team-colored gradients). The only chromatic color is a single steel-blue accent; everything else is tone.

→ See the four key-screen mocks under `EXPERIENCE.md § Information Architecture` for this system applied in full. This document is the contract; the mocks illustrate and win no conflict against it.

## Colors

- **Surface Base (`#151a1f`)** — the page background. Deliberately not pure black — enough depth to feel like a dark room, not a void.
- **Surface Raised (`#1c2229`)** — cards, the standings widget, form panels. Distinguished from the base by tone alone, not shadow.
- **Border Hairline (`#2b333b`)** — the only separator. Used at the lowest contrast that still reads, between table rows, around inputs, under the nav bar.
- **Accent (`#6f9bb8`)** — the one chromatic color. Primary buttons, active nav state, links, and the "your row" highlight in the standings widget. Never used decoratively.
- **Text Primary (`#e2e6ea`)** / **Text Secondary (`#8b98a3`)** — primary for scores, names, and body copy; secondary for meta text (deadlines, timestamps, helper copy).
- **Danger (`#c97b72`)** — muted, not saturated. Marks a rejected autocomplete entry, an invalid login code, or another genuinely blocked action — never anything routine. A locked (post-deadline) Prediction is not an "error state," just muted text (see Components). There is deliberately no "success" color: the PRD doesn't call for marking individual predictions as correct/incorrect, and adding one would cut against the app's anti-gamification stance (see `EXPERIENCE.md § Inspiration and Anti-patterns`) — Standings numbers speak for themselves.

Avoid: saturated red/green scoreboard clichés, gradients, any team-branded color, and using accent for anything but the single primary action per screen.

## Typography

System font stack throughout — no webfont loading for a 3-person tool. Three text roles (`heading`, `body`, `meta`) plus a `data` role for anything tabular: standings points, series game counts, stat leaderboard numbers. `data` uses tabular figures so columns of numbers align without shifting as digits change.

Headings are rare — one per page section, not decorative. Most of the interface is `body` or `data`.

## Layout & Spacing

Scale: 4 / 8 / 12 / 16 / 24 / 32px. Single-column content everywhere; the only persistent two-zone layout is the standings widget (a full-width bar) sitting above a single scrollable content column — never a sidebar, which wastes width on mobile and adds a layout mode nothing else in the app needs.

Content padding tightens on narrow viewports but the structure never restructures — see `EXPERIENCE.md § Responsive & Platform` for the behavioral rule.

## Elevation & Depth

No shadows. Surface Raised vs. Surface Base tone-separation is the only depth cue, reinforced by hairline borders where two raised surfaces sit adjacent (e.g., stacked prediction rows). This matches the "scoreboard, not app-store hero" posture — nothing should look like it's floating for attention.

## Shapes

`rounded/sm` (4px) for inputs, buttons, and table cells — enough to soften edges, not enough to feel playful. Nothing fully rounded; no pill buttons, no circular avatars (there are no avatars — participants are named, not pictured).

## Components

- **Standings widget** — full-width bar, `surface-raised`, sitting above the nav on every page. Three rows (one per Participant), columns for Rank / Regular Season / Playoffs / Total, all in the `data` role. Participants sharing a Total show the identical Rank value (e.g., two Participants both showing "1") — no secondary tiebreaker rendering of any kind. The signed-in Participant's own row is the only one distinguished — a thin `accent` left-border, nothing louder.
- **Nav bar** — flat text links (Predictions, Award Data, Logout), `accent` underline on the active link, no icons.
- **Prediction row** — one Series or Award per row, `surface-raised` panel, hairline border between stacked rows. A locked row shows its saved value in `text-secondary` with no input controls at all — not a disabled-looking input, simply read-only text, since a locked Prediction isn't an error state. An Award row's 3 finalist names per Participant render on one line as three bordered chips (see Components → Pick chip), alphabetically ordered left-to-right — never a comma-joined string or a stacked list — since the three names carry no ranking between them. A Series row's Winner and Game Count each render as one Pick chip, side by side on one line, never blended into one string like "Team in N" — the Game Count chip shows the Glossary's own notation (`4-0`/`4-1`/`4-2`/`4-3`), never a raw "games played" number. Each Series row's title carries a visible seed label (e.g. "Eastern · Series A") in `meta` style, and rows within a round are ordered by that same NHL series seeding, never arbitrary/entry order.
- **Pick chip** — a bordered box (`border-hairline`, `rounded/sm`, standard input padding) holding one value: one Award finalist's name, a Series Winner, or a Game Count. Deliberately reuses the same border/radius/padding as the selectable controls in Enter Predictions (`winner-options`, `game-count-options`) so a read-only Prediction visually echoes the control it was entered with — a viewer recognizes "this looks like a team pick" or "this looks like a game count" on sight. No accent fill and no dot indicator in the read-only state (those are reserved for the interactive, editable version in Enter Predictions) — a hairline border only.
- **Login form** — a plain text field (email, then the 6-digit code), same visual treatment as the Autocomplete input below (`surface-base`, `border-hairline`, `rounded/sm`) minus the suggestions panel. An invalid code shows the same `danger`-bordered, inline-`meta`-message treatment as a rejected autocomplete entry — one consistent error pattern across the whole app.
- **Autocomplete input** — `surface-base` field, `border-hairline`, suggestions drop in a `surface-raised` panel below. A rejected (non-matching) entry shows the `danger` color on the border only, with an inline `meta`-sized message — no toast, no modal.
- **Button (primary)** — `accent` background, `surface-base` text (dark text on the light-relative accent for contrast), `rounded/sm`. One primary button per screen at most (Save, or the "My Predictions" entry point).
- **Locked badge / read-only marker** — text only ("Locked" in `text-secondary`, `meta` size), never an icon or colored pill — consistent with the "not an error" posture above.

## Do's and Don'ts

| Do | Don't |
|---|---|
| One accent color, used only for the primary action and active state | Color-code by team, or use accent decoratively |
| Text-only states ("Locked", "Saved") | Icon badges, checkmarks, colored pills for state |
| Hairline borders for all separation | Card shadows or elevation for hierarchy |
| Tabular-aligned numbers in standings/predictions | Numbers that jitter or misalign as digits change |
| Single accent per screen for the primary action | Multiple competing buttons of equal visual weight |
| Dark-only, tuned for low-light viewing | A light mode nobody asked for, or a toggle to maintain |
