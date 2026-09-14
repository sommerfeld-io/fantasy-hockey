# Click-dummy → DESIGN.md/EXPERIENCE.md reconciliation

Compares `imports/clickdummy-ui-ux.md` and `imports/clickdummy-requirements.md` (source) against `DESIGN.md` and `EXPERIENCE.md` (distilled spines), per the "preserve the click-dummy exactly" directive. Sources will be deleted once this reconciliation is resolved.

## Gaps found

1. **Header layout entirely undocumented.** `clickdummy-ui-ux.md` §3.1 (lines 62-65): left = current player's name (bold), right = season label "NHL 2026–27" (muted), no logo/icon, solid `surface` background with a bottom border. Neither `DESIGN.md` (no "Header" component) nor `EXPERIENCE.md` (no "Header" row in Component Patterns / IA) documents any of this — the header's content and visual treatment would have to be reinvented from scratch.

2. **Bottom-nav icons dropped.** `clickdummy-ui-ux.md` §3.2 (lines 67-70): Predict = checklist icon, Leaderboard = medal icon, Compare = people icon; active tab = icon+label in `ice`, inactive = `muted`; nav bar is solid `surface` with a top border. `EXPERIENCE.md`'s IA table names the three destinations but never mentions icons, active/inactive icon styling, or the nav bar's own surface/border treatment. `DESIGN.md` has no "Bottom navigation" component entry at all.

3. **Predict section-header icons dropped.** `clickdummy-ui-ux.md` §4 (line 76): "Before the season" uses a target icon, "Playoffs" uses a trophy icon, both small/muted. Not mentioned in `EXPERIENCE.md`'s IA or Component Patterns, nor in `DESIGN.md`.

4. **Deadline-countdown color state lost.** `clickdummy-ui-ux.md` §4 (line 81): the relative countdown text is rendered "in accent, or faint if upcoming." `EXPERIENCE.md`'s Set row pattern (line 61-ish, Component Patterns table) only says "a deadline line (clock icon + date/time + countdown)" with no color-state rule — the accent/faint distinction for the countdown text itself is gone.

5. **Division-picks progress indicators (n/5, n/8, check/dot) entirely absent.** `clickdummy-ui-ux.md` §6.2 (lines 109): "Each division shows an `n/5` counter; each conference shows an `n/8` indicator with a check (green) when valid or a dot (red) when not." Neither `DESIGN.md` nor `EXPERIENCE.md` mentions this counter/indicator UI anywhere — only the live-cap *behavior* (chips dim and stop being tappable) survived, not the visible progress indicator that shows players how close they are. This is a concrete, distinct piece of UI that's simply missing.

6. **Live-cap dim amount unspecified.** `clickdummy-ui-ux.md` §6.2 (line 110) specifies remaining chips dim to "~40%" opacity when a scope is capped. `EXPERIENCE.md`'s Division picks row just says chips "dim and stop being tappable" — the specific opacity value is dropped (minor, but it's an exact stated value the source gives).

7. **Playoff-round intro copy dropped.** `clickdummy-ui-ux.md` §6.4 (line 122): "For each series, tap the winner and how many games it takes." This literal instructional copy line doesn't appear anywhere in `EXPERIENCE.md`'s Voice and Tone or Component Patterns tables.

8. **Leaderboard captions dropped.** `clickdummy-ui-ux.md` §7 (lines 129, 134): section caption "Ranked by total points." and the footer caption clarifying Regular/Playoff feed Total. Neither literal string appears in `EXPERIENCE.md`; the underlying concept (sorted by Total, Total is the deciding column) survives structurally but the actual UI copy is gone.

9. **"No '(you)' marker on names" rule dropped from Leaderboard.** `clickdummy-ui-ux.md` §7 (line 133) is an explicit negative rule — the Leaderboard deliberately does *not* label the current player's row. `EXPERIENCE.md`'s Leaderboard table row (Component Patterns) never restates this exclusion, so a future implementer has no signal that adding a "(you)" tag there would be a deviation. (Contrast: the *Compare* table's "current player's column is visually distinguished" is preserved correctly — only the Leaderboard's specific non-marking is lost.)

10. **Compare section header copy dropped.** `clickdummy-ui-ux.md` §8 (line 138): "Everyone's picks." Not quoted anywhere in `EXPERIENCE.md`.

11. **Compare's "matchups not set" empty state dropped.** `clickdummy-ui-ux.md` §9 (line 156): "upcoming rounds in Compare would show a dashed 'matchups not set' note (not reachable while gated)." This defined-but-unreachable edge-case treatment isn't mentioned in `EXPERIENCE.md`'s State Patterns at all.

12. **Leaderboard "sample/placeholder data" nuance not preserved anywhere.** `clickdummy-ui-ux.md` §11 (line 167) and `clickdummy-requirements.md` US-4.1 (🟡, line 119: "Points are sample values in the prototype") both flag that leaderboard scoring is placeholder-only. Neither `DESIGN.md` nor `EXPERIENCE.md` mentions this anywhere — the Leaderboard table row and Flow 4 in `EXPERIENCE.md` describe Regular/Playoff/Total as if they're live, computed values, with no caveat that no real scoring exists yet. This is exactly the nuance the task asked to check for, and it's missing.

13. **Invented rule: "no tiebreaker of any kind" (Leaderboard).** `EXPERIENCE.md` Component Patterns, Leaderboard table row: "Players with an equal Total share the same rank — no tiebreaker of any kind." This is not stated anywhere in `clickdummy-ui-ux.md`. Worse, `clickdummy-requirements.md` US-4.2 (❓, line 125) explicitly lists "tie-breaker rules" among the still-open scoring questions. The spine asserts as settled fact something the source explicitly marks undecided.

14. **Invented rule: unfilled picks "score zero" (Flow 2).** `EXPERIENCE.md` Key Flows, Flow 2 (line 121): "whatever he left blank simply isn't blocking him — it'll score zero, not stop him from saving the rest." No scoring value is defined anywhere in the click-dummy sources; `clickdummy-ui-ux.md` §11 (line 167) states plainly "Scoring rules... are not yet defined." Asserting blanks "score zero" invents a specific scoring outcome the source explicitly leaves open.

15. **Invented rule: player-awards finalist "rejected, not silently accepted."** `EXPERIENCE.md` Component Patterns, Player awards row: "a name that doesn't match the list is rejected, not silently accepted." `clickdummy-ui-ux.md` §6.3 only says inputs "offer autocomplete suggestions via datalists" (native HTML datalists are non-restrictive by default — they suggest, they don't validate/reject). `clickdummy-requirements.md` US-2.5 (line 91) explicitly flags this as open: "❓ Should finalists be validated against a real player roster, or remain free text?" The spine states a definitive validation behavior that contradicts the source's own open question.

16. **Open decision dropped: before-season deadline strategy.** `clickdummy-requirements.md` US-1.6 (🔜❓, line 61) and `clickdummy-ui-ux.md` §11 (line 170) both flag "should all four before-season sets share one deadline (opening night) or be staggered?" as unresolved. `EXPERIENCE.md`'s "Open Items" section lists only three items (terminology, FR-22, session-timeout assumption) — this click-dummy-sourced open question isn't carried forward anywhere, so it risks being silently forgotten once `imports/` is deleted.

17. **Accessibility Floor wording conflicts with the (missing) dot indicator.** `EXPERIENCE.md` Accessibility Floor states: "validity shows a check or a red caption plus text, not a colored dot alone." But the source (`clickdummy-ui-ux.md` §6.2, line 109) explicitly defines a red *dot* as part of the conference-validity indicator (alongside the `n/8` text). Given gap #5 (the dot/counter UI is missing entirely), this accessibility line reads as though it's describing an indicator that doesn't exist in the spine, and it inaccurately implies dots are avoided when the source actually pairs a dot with text (which is accessible) rather than using it alone.

## Confirmed captured

- All 16 design tokens (exact hex values) and their stated usage/restrictions — `bg`, `surface`, `raised`, `border`/`border-soft`, `text`/`muted`/`faint`, `ice`/`ice-deep`, `sel`, `green`, `green-btn`/`green-btn-border`, `gold`, `goal` — match verbatim in `DESIGN.md` frontmatter and prose, including the "never a saturated fill for selection, grey+ice border only" rule and gold/goal's single-reserved-meaning restriction.
- Typography family, monospace/tabular-nums rule, full size scale, weights, and sentence-case-only rule.
- Spacing, radius (`lg`/`xl`/`full`/tag rounding), and elevation-via-surface-steps (no shadows/gradients) rules.
- Three-region fixed app shell (pinned header, scrolling content, pinned nav) and "no player switcher" principle.
- Set row anatomy: accent stripe color coding, status pill colors/labels, chevron vs. lock affordance, dimmed/non-tappable Upcoming rows.
- Prediction sheet: pinned header, scrolling body, Submit→Update button label swap, "Editable until the deadline" caption, Closed-set read-only banner + disabled inputs.
- Single-team pick (dropdown grouped by division + check on selection), Division picks chip visual spec and exact hint-text string, Division winners (4 scoped dropdowns), Player awards (5 trophies × 3 finalists, scoped autocomplete, check-on-complete).
- Playoff round series: identical `sel`+`ice` selected styling for winner and game-count buttons, explicit "no separate/no orange-red color for games buttons" rule preserved in both prose and the Do's/Don'ts table.
- Compare: two-row chip selector with no horizontal scroll, tag-vs-plain-text value formatting rules, division-picks-broken-out-per-row rule, "—" for empty values.
- Interaction primitives: immediate/local selection feedback (no page reload), input-time + submit-time validation split, closed-set full read-only state.
- Accessibility floor: tap-target sizes (≥32px, 36px number buttons), tabular numerals, color-plus-text for status.
