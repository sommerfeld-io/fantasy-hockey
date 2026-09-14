---
name: Face-Off Pool
description: Dark-by-default, mobile-first prediction-pool UI, distilled from the existing click-dummy prototype.
status: final
updated: 2026-09-14
colors:
  bg: '#0d1117'
  surface: '#161b22'
  raised: '#21262d'
  border: '#30363d'
  border-soft: '#21262d'
  text: '#e6edf3'
  muted: '#8b949e'
  faint: '#6e7681'
  ice: '#58a6ff'
  ice-deep: '#1f6feb'
  sel: '#30363d'
  green: '#3fb950'
  green-btn: '#238636'
  green-btn-border: '#2ea043'
  gold: '#d29922'
  goal: '#f85149'
typography:
  body:
    fontFamily: 'ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif'
  numeric:
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace'
    note: 'tabular-nums — leaderboard totals and any aligned numbers'
  scale:
    caption: 11px
    label: 12px
    body: 13-14px
    title: 15-16px
    total: 18-19px
  weight:
    label: semibold
    button: semibold
    title: bold
    total: extrabold
  case:
    note: 'Sentence case throughout — no all-caps labels'
rounded:
  lg: 10px
  xl: 12px
  full: 9999px
spacing:
  gutter: 12-16px
  card-padding: 12px
  tap-target-min: 32px
  tap-target-number-button: 36px
components:
  input-field:
    fill: '{colors.raised}'
    border: '{colors.border}'
    radius: '{rounded.lg}'
    text: '{colors.text}'
    placeholder: '{colors.faint}'
  header:
    background: '{colors.surface}'
    border: '{colors.border}'
    borderSide: bottom
  bottom-nav:
    background: '{colors.surface}'
    border: '{colors.border}'
    borderSide: top
    activeColor: '{colors.ice}'
    inactiveColor: '{colors.muted}'
  section-header-icon:
    color: '{colors.muted}'
    size: small
  division-progress:
    validColor: '{colors.green}'
    invalidColor: '{colors.goal}'
    capDimOpacity: 40%
  status-pill:
    open: { background: '{colors.ice}', opacity: 'tint' }
    submitted: { background: '{colors.green}', opacity: 'tint' }
    closed: { background: '{colors.goal}', opacity: 'tint' }
    upcoming: { background: '{colors.muted}', opacity: 'tint' }
  set-row:
    accentStripe: 3px
    open: '{colors.ice}'
    submitted: '{colors.green}'
    upcoming: '{colors.faint}'
  chip-selected:
    fill: '{colors.sel}'
    border: '{colors.ice}'
    text: '{colors.text}'
  chip-unselected:
    fill: '{colors.raised}'
    text: '{colors.muted}'
  button-primary:
    fill: '{colors.green-btn}'
    border: '{colors.green-btn-border}'
  rank-badge:
    shape: '{rounded.full}'
    leader: '{colors.gold}'
---

## Brand & Style

Face-Off Pool is a private, three-person NHL prediction pool, not a public product — the visual language is utilitarian and quiet, built for quick phone-width checks between periods, not a marketing surface. Mobile-first: every layout assumes a narrow viewport (~360–430px); other devices are not a design target for v1. Dark by default, and dark only — a single dark theme, no light mode. GitHub-dark-inspired greys, restrained accent use, no gradients, minimal decoration; information is carried by structure and typography, not ornament. One task per screen — each prediction set opens as its own focused form, with comparison and standings kept as separate destinations. The app always acts as the current, logged-in player; there is no player switcher, so nothing in the visual language should suggest one.

## Colors

The palette is a near-monochrome dark-grey scale (`{colors.bg}` → `{colors.surface}` → `{colors.raised}` → `{colors.border}`) carrying almost all of the interface, with two chromatic accents used sparingly and for one purpose each.

- **`bg` (`#0d1117`)** — the app canvas, the darkest surface.
- **`surface` (`#161b22`)** — cards, table bodies, header/nav bars — one step up from canvas.
- **`raised` (`#21262d`)** — inputs, chips, unselected buttons, the table-heading row — the "interactive/elevated" step.
- **`border` / `border-soft` (`#30363d` / `#21262d`)** — default borders/dividers, and a softer variant for inner row dividers.
- **`text` / `muted` / `faint`** — a three-step text hierarchy: primary content, secondary/tag/inactive-nav text, and hints/captions/placeholders/empty-value dashes.
- **`ice` (`#58a6ff`, accent)** — selected borders, active nav, the "you" column, small accents. Used sparingly and only as a border — selected states are a neutral grey fill (`sel`) with an `ice` border, never a saturated color fill.
- **`ice-deep` (`#1f6feb`)** — reserved deeper accent, not in active use on the primary surfaces.
- **`green` (`#3fb950`)** — valid/complete indicators (checks). **`green-btn`/`green-btn-border` (`#238636`/`#2ea043`)** — the one primary action color, reserved for the Submit/Update button.
- **`gold` (`#d29922`)** — the leaderboard leader marker and the leader's total only. Not a general-purpose highlight.
- **`goal` (`#f85149`)** — error/validation text and the "Closed" state only.

Avoid: saturated fills for any selected/active state (grey fill + blue border, always), gradients, shadows, and using `gold` or `goal` outside their one reserved meaning each.

## Typography

System sans for all UI text; monospace with tabular figures (`font-variant-numeric: tabular-nums`) for the leaderboard totals and any other aligned numbers, so digits don't jitter when they change. Scale runs from 11px (captions, tags, pills) up to 18–19px (leaderboard totals) — six tight steps, no large display sizes anywhere. Labels and buttons are semibold; titles and totals are bold-to-extrabold. Sentence case throughout, everywhere — no all-caps labels, ever.

## Layout & Spacing

Base gutters run 12–16px at the page level, 12px inside cards — a tight, consistent rhythm rather than a wide multi-step scale. The frame is a single centered column at max phone width; on wider viewports the column stays centered at that same max width against the dark canvas rather than stretching.

## Elevation & Depth

No shadows, no gradients, anywhere. Elevation is entirely a matter of surface-color steps (`bg` → `surface` → `raised`) plus 1px borders. A card reads as "above" the canvas because it's a lighter grey with a hairline border, not because it casts a shadow.

## Shapes

`rounded/lg` (~10px) for buttons, chips, and inputs. `rounded/xl` (~12px) for cards and containers. Full-circle (`rounded/full`) for the leaderboard rank badge only. Tags get a small, distinct rounding of their own (smaller than `lg`). Nothing else is fully rounded — nothing else reads as pill-shaped except the intentional tag/badge cases above.

## Components

- **Header** — pinned, non-scrolling. Player's name (bold) on the left, season label ("NHL 2026–27", muted) on the right. No logo or icon. Solid `surface` background with a bottom `border`.
- **Bottom navigation** — pinned, non-scrolling. Three equal tabs: Predict (checklist icon), Leaderboard (medal icon), Compare (people icon). Active tab: icon + label in `ice`. Inactive: `muted`. Solid `surface` background with a top `border`.
- **Section header icon** — small, `muted` icon preceding a Predict section label: a target icon for "Before the season," a trophy icon for "Playoffs."
- **Status pill** — the small colored label on each prediction-set row: Open (blue tint), Submitted (green tint), Closed (red tint), Upcoming (grey). Color is always paired with the status word itself, never color-only.
- **Set row** — a full-width tappable card with a 3px left accent stripe (blue = open, green = submitted, faint = upcoming), bold title + status pill, a faint one-line subtitle, a deadline line (clock icon + date/time + countdown, the countdown itself in `ice` when the set is Open/Submitted or `faint` when Upcoming), and a trailing chevron (actionable) or lock icon (upcoming/disabled).
- **Division-picks progress indicator** — a small `n/5` counter per division and an `n/8` indicator per conference, the latter paired with a check (`{components.division-progress.validColor}`) when valid or a dot (`{components.division-progress.invalidColor}`) when not — the dot is always shown together with the `n/8` text, never alone. Once a division reaches 5 or a conference reaches 8, remaining chips in that scope dim to `{components.division-progress.capDimOpacity}` and stop being tappable.
- **Chip** (team abbreviation, comparison selector) — selected: `sel` fill + `ice` border + primary text. Unselected: `raised` fill + muted text. Identical visual treatment wherever chips appear (Division picks, Compare selector).
- **Number button** (series game-count, 4/5/6/7) — same selected/unselected treatment as a chip: `sel` fill + `ice` border when chosen. No separate color for this versus the team-winner buttons next to it — winner and game-count picks must look equally important.
- **Input / dropdown field** (Login email/code, single-team dropdowns, division-winner dropdowns, award-finalist text inputs) — `raised` fill, 1px `border`, `lg` radius, `text` value color, `faint` placeholder color. No focus/error variant defined beyond the code field's `goal`-border error state (see `EXPERIENCE.md` State Patterns).
- **Card / container** — `surface` background, `xl` radius, 1px `border`, 12px internal padding.
- **Primary button (Submit / Update predictions)** — full-width, `green-btn` fill with `green-btn-border` border, semibold label. The only saturated-fill component in the system.
- **Rank badge** — full-circle, leader's badge in `gold`, others neutral.
- **Table (Leaderboard, Compare)** — `raised` heading row, `border-soft` row dividers, no shadows; the Total column is visually set apart with a left `border` and a `raised` background tint.

## Do's and Don'ts

| Do                                                                                    | Don't                                                            |
|---------------------------------------------------------------------------------------|------------------------------------------------------------------|
| One accent (`ice`) for selection/active state, always as a border over a neutral fill | Fill a selected state with a saturated color                     |
| Reserve `gold` for the leaderboard leader only                                        | Use `gold` as a general highlight/accent                         |
| Reserve `goal` (red) for errors and the Closed state only                             | Use red for anything else, including warnings that aren't errors |
| Pair every status color with its word                                                 | Rely on color alone to convey status                             |
| Give winner and game-count picks the identical selected style                         | Visually rank one pick above the other within a series card      |
| Sentence case everywhere                                                              | All-caps labels, headers, or buttons                             |
| Elevate via surface-color steps + hairline borders                                    | Shadows, gradients, or any decorative fill                       |
| Tabular numerals for anything that updates (totals, counts)                           | Proportional numerals on any live/aligned number                 |
