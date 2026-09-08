# Face-Off Pool — UI & UX Specification

Reference for the look, layout, and interaction of the app. Scope is the mobile-first web prototype ("click dummy"); it defines the visual system and every screen's behaviour so the UI can be rebuilt or extended consistently.

---

## 1. Design principles

- **Mobile-first.** Designed for a phone-width column; other devices come later. All layouts assume a narrow viewport (~360–430px).
- **Dark by default.** A single dark theme, no light mode.
- **Clean and flat.** GitHub-dark-inspired greys, restrained accent use, no gradients, minimal decoration. Information is carried by structure and typography, not ornament.
- **One task per screen.** Each prediction set opens as its own focused form; comparison and standings are separate destinations.
- **Only your picks are editable.** The app acts as the current player; there is no player switcher.

## 2. Design tokens

### 2.1 Colour palette

| Token | Hex | Usage |
|---|---|---|
| `bg` | `#0d1117` | App background / canvas |
| `surface` | `#161b22` | Cards, table bodies, header/nav bars |
| `raised` | `#21262d` | Inputs, chips, unselected buttons, table-heading row |
| `border` | `#30363d` | Default borders, dividers under headers |
| `borderSoft` | `#21262d` | Subtle inner row dividers |
| `text` | `#e6edf3` | Primary text |
| `muted` | `#8b949e` | Secondary text, tag text, inactive nav |
| `faint` | `#6e7681` | Hints, captions, placeholders, "—" |
| `ice` (accent) | `#58a6ff` | Selected borders, active nav, "you" column, small accents |
| `iceDeep` | `#1f6feb` | Reserved deeper accent |
| `sel` | `#30363d` | Fill of selected toggles/chips |
| `green` | `#3fb950` | Valid / complete indicators (checks) |
| `greenBtn` / border | `#238636` / `#2ea043` | Primary action (Submit) |
| `gold` | `#d29922` | Leaderboard leader marker & total |
| `goal` | `#f85149` | Error/validation text, "Closed" state |

Accent (blue) is used sparingly. Selected states are a neutral grey fill (`sel`) with a blue (`ice`) border — never a saturated fill. The primary Submit button is green.

### 2.2 Typography

- **Family:** system sans (`ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, …`).
- **Numeric / scoreboard:** monospace (`ui-monospace, SFMono-Regular, Menlo, …`) with tabular figures for leaderboard totals and any aligned numbers (`font-variant-numeric: tabular-nums`).
- **Scale (px):** 11 (captions, tags, pills), 12 (labels, chips), 13–14 (body, values, buttons), 15–16 (titles/headers), 18–19 (leaderboard totals).
- **Weight:** semibold for labels and buttons; bold/extrabold for titles and totals.
- **Case:** sentence case throughout — no all-caps labels.

### 2.3 Spacing, radius, elevation

- Base padding: 12–16px page gutters; 12px card padding.
- Radii: `lg` (~10px) for buttons/chips/inputs; `xl` (~12px) for cards/containers; full circle for the rank badge; small rounding for tags.
- Elevation via surface colour steps (`bg` → `surface` → `raised`) and 1px borders — no shadows, no gradients.

## 3. Layout & app shell

- **Frame:** a single centred column, max phone width, filling the full viewport height. The whole app is a fixed-height shell.
- **Three regions, top to bottom:**
  1. **Header** — pinned, non-scrolling.
  2. **Content** — the only scrolling region (`overflow-y: auto`).
  3. **Bottom navigation** — pinned, non-scrolling.
- The header and nav never move; only the middle scrolls. (This replaced an earlier sticky approach that let the footer drift.)

### 3.1 Header

- Left: current player's name (bold). Right: season label "NHL 2026–27" (muted).
- No logo/icon. Solid `surface` background with a bottom border. No player switcher.

### 3.2 Bottom navigation

- Three equal tabs: **Predict** (checklist icon), **Leaderboard** (medal icon), **Compare** (people icon).
- Active tab: icon + label in accent (`ice`); inactive: `muted`. Solid `surface` with a top border.

## 4. Screen: Predict

The default screen. A scrollable list of prediction sets grouped into two labelled sections.

- **Section headers:** "Before the season" (target icon) and "Playoffs" (trophy icon), small and muted.
- **Set row anatomy** (a full-width tappable card):
  - A 3px left accent stripe encoding state: blue = open, green = submitted, faint = upcoming.
  - Title (bold) + **status pill**.
  - Subtitle (faint, one line).
  - Deadline line: clock icon + absolute date/time + relative countdown (countdown in accent, or faint if upcoming).
  - Right affordance: chevron (actionable) or lock icon (upcoming/disabled).
- **Status pill** colours: Open = blue tint; Submitted = green tint; Closed = red tint; Upcoming = grey.
- Upcoming rows are dimmed and non-tappable.

**Before-the-season sets:** Cup champion · Presidents' Trophy · Division picks · Player awards.
**Playoff sets:** Playoffs Cup pick · Playoff round 1 · Playoff round 2 · Conference finals · Stanley Cup final (last three upcoming).

## 5. Screen: Prediction sheet

Tapping an actionable set opens a full-screen sheet over the frame.

- **Sheet header (pinned):** back arrow, set title, deadline + countdown.
- **Body (scrolls):** the set's form (see §6). If the set is closed, a red read-only banner appears and inputs are disabled.
- **Action bar (pinned to sheet bottom, when open):** a full-width green **Submit predictions** button (label becomes **Update predictions** if already submitted). A caption reads "Editable until the deadline".
- When a form is invalid (see Division picks), the button is disabled and the caption turns red explaining what's missing.

## 6. Components: forms per set kind

### 6.1 Single-team pick (Cup champion, Presidents' Trophy, Playoffs Cup pick)

- One labelled field with a grouped dropdown of all 32 teams (grouped by division via `optgroup`).
- A green check appears on the label once a team is chosen.

### 6.2 Division picks

- **Playoff teams** — two conference blocks (Eastern, Western), each with its two divisions. Each division shows its 8 teams as **abbreviation chips** (e.g. TOR, BOS). Tapping toggles selection.
  - Selected chip: `sel` fill + `ice` border + primary text. Unselected: `raised` fill + muted text.
  - Each division shows an `n/5` counter; each conference shows an `n/8` indicator with a check (green) when valid or a dot (red) when not.
  - **Live caps:** once a division holds 5, or a conference holds 8, remaining chips in that scope dim to ~40% and become non-tappable.
  - Hint text states the rule: "8 teams per conference — either a 4/4 split, or 5 in one division and 3 in the other."
- **Division winners** — four dropdowns, one per division, each limited to that division's teams.

### 6.3 Player awards

- Five trophy groups (Hart, Norris, Vezina, Art Ross, Rocket Richard). Each has three text inputs for finalists.
- Inputs offer autocomplete suggestions via datalists: skaters for Hart/Art Ross/Rocket, defensemen for Norris, goalies for Vezina.
- The group's label gets a green check when all three finalists are filled.

### 6.4 Playoff round

- Intro line: "For each series, tap the winner and how many games it takes."
- Series are grouped under conference subheaders ("Eastern Conference" / "Western Conference"); the final round is grouped under "Stanley Cup Final".
- **Series card:** two full-width team buttons (choose winner), then a row: "in" + four number buttons (4/5/6/7) + "games".
  - **Winner and games use the identical selected style:** `sel` fill + `ice` border. (No orange/red on the games buttons — the two selections are marked the same way.)

## 7. Screen: Leaderboard

- Section header "Leaderboard" with a caption "Ranked by total points."
- A single table inside a bordered card:
  - Heading row: **Player · Regular · Playoff · Total**. The **Total** header/column is tinted and divided off with a left border to read as the deciding column.
  - One row per player, sorted by Total descending. Rank badge on the left (leader badge is gold). Regular/Playoff are muted; **Total** is large, bold, monospace (leader's total in gold).
  - No "(you)" marker on names.
- Footer caption clarifies that Regular and Playoff feed Total, which decides standings (sample scoring noted).

## 8. Screen: Compare

- Section header "Everyone's picks".
- **Selector:** chips to choose the set, in **two labelled rows** — "Before the season" and "Playoffs". Chips wrap within their row (no horizontal scrolling); all options are visible at once. Selected chip: `sel` fill + `ice` border.
- **Deadline line** for the chosen set.
- **Comparison table** (one bordered container):
  - **Heading row:** one column per player (bold). The current player's name is in accent; the row sits on a `raised` background with a divider beneath.
  - **Category rows:** each category is a labelled row (faint label) with a 3-column value grid aligned under the player headings, separated by soft dividers.
  - **Value formatting:**
    - Team-abbreviation picks (playoff teams, division winners, series winners) render as a single consistent **tag** (`raised` chip, muted text).
    - Full team names (Cup champion, Presidents' Trophy) render as plain text.
    - Series rows show `TAG in N` (tag + muted "in N"); the matchup title (e.g. "Eastern · FLA vs OTT") is plain text.
  - **Division picks specifically:** one row per division for the playoff teams, with that division's winner in a dedicated row directly below it (8 rows total).
  - Empty values render as a faint "—".

## 9. Interaction & state rules

- **Selection feedback** is immediate and local (React state); no page reloads.
- **Validation** is enforced at input time (caps) and at submit time (disabled button + message) for Division picks; other sets can be saved partially.
- **Closed sets** are fully read-only (inputs disabled, no action bar).
- **Empty states:** a set with no picks shows placeholders/"—"; upcoming rounds in Compare would show a dashed "matchups not set" note (not reachable while gated).

## 10. Responsiveness & accessibility notes

- Layout targets phone widths; on wider screens the column stays centred at max phone width against the dark canvas.
- Tap targets are chip/button sized (≥ ~32px height); number buttons are 36px squares.
- Colour is supported by text/labels (status pills carry words, not colour alone; validity shows a check/dot plus text).
- Numbers use tabular figures for stable alignment.

## 11. Known gaps / to define

- **Scoring rules** (point values per prediction type) are not yet defined; leaderboard values are samples.
- **Persistence, auth, and shared visibility** are not implemented — state is in-memory only.
- **Admin UI** for deadlines, real matchups, and results entry is out of scope in the prototype.
- **Deadline strategy** for the four before-season sets (shared vs. staggered) is an open decision.
