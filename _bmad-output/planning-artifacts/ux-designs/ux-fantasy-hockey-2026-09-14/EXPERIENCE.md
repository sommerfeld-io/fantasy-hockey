---
name: Face-Off Pool
status: final
sources:
  - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-14/prd.md
  - imports/clickdummy-ui-ux.md
  - imports/clickdummy-requirements.md
  - imports/faceoff-pool-source (executable prototype — fidelity source of truth)
updated: 2026-09-14
---

# Face-Off Pool — Experience Spine

> Distilled near-literally from an existing, prototype-validated click-dummy (`imports/faceoff-pool-source/fantasy-hockey/src/App.jsx`, the executable source of truth; `imports/faceoff-pool-dist`, the built runnable copy) per explicit user direction to preserve it unchanged. Two surfaces have no click-dummy precedent and were elicited fresh: Login (email + code) and session timeout. A third PRD requirement — the reminder-email trigger (FR-22) — is explicitly deferred; see "Deferred" below. Terminology follows the click-dummy's own UI text ("Leaderboard", "Player awards", "Playoffs Cup pick") rather than the PRD's ("Standings", "Awards (individual)", "Playoffs Cup re-pick") — a decision, not an oversight; see Open Items.

## Foundation

Single-surface mobile web, phone-width (~360–430px) by default; on wider viewports the column stays centered at that same max width rather than adapting — no tablet/desktop layout is a v1 target. No named UI system — a custom Tailwind build (see `imports/faceoff-pool-source/fantasy-hockey/package.json`). Dark theme only, always — there is no light-mode setting anywhere in the product. `DESIGN.md` is the visual identity reference; this spine is the behavior.

The frame is a fixed-height, three-region app shell: a pinned header, a single scrolling content region, and pinned bottom navigation. Header and navigation never move — only the content region scrolls (`overflow-y: auto`). The current player is always implicit from the session; there is no player switcher anywhere in the shell.

## Information Architecture

| Surface          | Reached from                                | Purpose                                                  |
|------------------|---------------------------------------------|----------------------------------------------------------|
| Login — email    | Cold app start with no valid session        | Request a login code                                     |
| Login — code     | After requesting a code                     | Enter the 6-digit code; establishes the session          |
| Predict          | Bottom nav (default surface once logged in) | Browse all Prediction sets, grouped by phase             |
| Prediction sheet | Tap an actionable set row on Predict        | Fill in / edit one set's picks                           |
| Leaderboard      | Bottom nav                                  | Ranked view of all players' points                       |
| Compare          | Bottom nav                                  | Side-by-side picks across all players for one chosen set |

Login is a full-screen flow that precedes the app shell entirely — it has no header, no bottom nav, and is not one of the three bottom-nav destinations. The Prediction sheet is a full-screen sheet layered over the Predict frame, not a fourth nav destination.

→ Composition reference: `imports/faceoff-pool-dist` (runnable build) and `imports/faceoff-pool-source` (source) for Predict, Prediction sheet, Leaderboard, and Compare exactly as built. Login has no click-dummy precedent — see `mockups/login-email.html` and `mockups/login-code.html`, rendered fresh for this spine. Spine wins on conflict with any of the above.

## Deferred (not designed in this pass)

- **Reminder-email trigger (PRD FR-22).** The PRD specifies a player can trigger a reminder email for a set that hasn't closed, but no control for this exists in the click-dummy and none is designed here — explicitly discarded from this UX pass per user direction. No surface, button, or affordance for it appears anywhere in the Information Architecture above. This narrows the finalized PRD's MVP scope (§6.1 lists FR-22 under MVP); flagged under Open Items for a PRD follow-up, not silently resolved.

## Voice and Tone

Microcopy only — brand voice and aesthetic posture live in `DESIGN.md.Brand & Style`. Sentence case throughout; short, direct captions; no exclamation marks anywhere in the click-dummy's copy, and none introduced here.

| Do                                                                                                                                     | Don't                                                   |
|----------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------|
| "Editable until the deadline."                                                                                                         | "You can still make changes!"                           |
| "8 teams per conference — either a 4/4 split, or 5 in one division and 3 in the other." (states the rule plainly when blocking submit) | A generic "Invalid selection" with no explanation       |
| "—" for any value nobody has entered yet                                                                                               | Blank space or "N/A" for an empty pick                  |
| [ASSUMPTION] "Email address" (Login) / "Send code" (button)                                                                            | Marketing-toned copy ("Welcome back!", "Let's go!")     |
| [ASSUMPTION] "That code didn't work — check it and try again." (wrong/expired/used, identically worded per PRD FR-2)                   | Copy that reveals *which* of wrong/expired/used applied |

## Component Patterns

Behavioral. Visual specs live in `DESIGN.md.Components`.

| Component | Use | Behavioral rules |
|---|---|---|
| Login — email field | Login (email) | Single field, single submit. Submitting always transitions to Login (code) with the same neutral confirmation, regardless of whether the email matched a player — no client-side way to tell. Realizes PRD FR-1 / UJ-2. → `mockups/login-email.html` |
| Login — code field | Login (code) | Single 6-digit input, single submit. Wrong/expired/already-used codes all show the identical error copy above; success transitions straight into the app shell on Predict. Realizes PRD FR-2 / UJ-2. → `mockups/login-code.html` (at-rest + error states) |
| Header | Every post-login surface | Static — no interaction. Always shows the logged-in player's name and the season label; never a player switcher. |
| Bottom navigation | Every post-login surface | Tap a tab to switch surfaces (Predict / Leaderboard / Compare). Current surface's tab is visually active; no other interaction. |
| Section header | Predict | Static label ("Before the season" / "Playoffs") with an icon; not interactive. |
| Set row | Predict | Tap opens the Prediction sheet if actionable (Open/Submitted); Closed rows still open (read-only); Upcoming rows are dimmed and non-tappable. |
| Prediction sheet (`DESIGN.md`: Card/container, Primary button) | Opened from a set row | Full-screen over the frame. Header pinned (back arrow, title, deadline+countdown); body scrolls; action bar pinned to the sheet bottom while the set is open. Button reads "Submit predictions" the first time, "Update predictions" once already submitted. Closed sets show a read-only banner instead of the action bar, and every input is disabled. |
| Single-team pick (Cup champion, Presidents' Trophy, Playoffs Cup pick) | Prediction sheet | One dropdown, teams grouped by division. Label gets a check the moment a team is chosen. |
| Division picks — playoff teams (`DESIGN.md`: Chip) | Prediction sheet | Tap a team chip to toggle it. Each division shows a live `n/5` counter; each conference shows a live `n/8` indicator (check when valid, dot when not — see `DESIGN.md.Components`). Live caps: once a division holds 5 or a conference holds 8, remaining chips in that scope dim (`DESIGN.md` capDimOpacity) and stop being tappable. Submit stays disabled, with a red caption explaining what's missing, until both conferences are exactly 8 in a 4/4 or 5/3 split. |
| Division picks — division winners | Prediction sheet | Four independent dropdowns, each scoped to its own division's teams. |
| Player awards | Prediction sheet | Five trophy groups, three finalist text inputs each. Autocomplete suggestions scoped by trophy (skaters for Hart/Art Ross/Rocket Richard, defensemen for Norris, goalies for Vezina); a name that doesn't match the list is rejected, not silently accepted — resolves the click-dummy's own open validation question; see "Resolved click-dummy open questions" below. Group label gets a check once all three finalists are filled. |
| Playoff round series (`DESIGN.md`: Chip, Number button) | Prediction sheet | Intro copy: "For each series, tap the winner and how many games it takes." Two full-width team buttons (pick winner) plus four number buttons 4/5/6/7 (pick game count), grouped under conference subheaders ("Eastern Conference" / "Western Conference"; final round under "Stanley Cup Final"). Winner and game-count selections use the identical selected styling — neither reads as more important. The winner + game-count pair saves as one atomic unit, unlike every other field in the app, which saves independently (PRD FR-9/FR-19). |
| Leaderboard table (`DESIGN.md`: Table, Rank badge) | Leaderboard | One row per player, sorted by Total descending. Players with an equal Total share the same rank — no tiebreaker of any kind; resolves the click-dummy's own open tie-breaker question; see "Resolved click-dummy open questions" below. Leader's rank badge and Total render in gold. No "(you)" marker on any name, including the current player's own row — deliberate, unlike Compare's highlighted own-column. Section caption: "Ranked by total points." Footer caption clarifies Regular + Playoff feed Total. [NOTE FOR UX] The click-dummy's own displayed point values are sample/placeholder only, not reflective of the real scoring formula — see PRD FR-24 for the actual rules; don't treat numbers in `imports/faceoff-pool-dist` as real. |
| Compare selector | Compare | Chips grouped into two labeled rows ("Before the season" / "Playoffs"); all options visible at once, no horizontal scrolling. Selecting a chip swaps the comparison table below and shows that set's deadline. Section header copy: "Everyone's picks." |
| Compare table | Compare | One column per player; the current player's column is visually distinguished. Division-pick rows are broken out one row per division, with that division's winner directly beneath it. Team-abbreviation values render as a consistent tag everywhere; full team names render as plain text; series rows show the tag plus "in N"; anything unfilled shows "—". |

## State Patterns

| State | Surface | Treatment |
|---|---|---|
| No valid session | Any cold start | Route straight to Login (email); the app shell never mounts first. |
| Session active | Any | Any request resets the 30-minute inactivity countdown (PRD FR-3). |
| [ASSUMPTION] Session timeout | Any | On the first request after 30 minutes idle, route back to Login (email). No special "you were logged out" messaging — the login screen simply reappears, consistent with the product's terse, unembellished copy elsewhere. |
| Set: Open | Predict | Blue accent stripe + blue status pill; actionable. |
| Set: Submitted | Predict | Green accent stripe + green status pill; still actionable (edit in place) until the deadline. |
| Set: Closed | Predict / Prediction sheet | Red status pill on Predict; sheet still opens but shows a read-only banner and disables every input — no action bar. |
| Set: Upcoming | Predict | Faint accent stripe, grey status pill, dimmed row, lock icon instead of chevron, not tappable. |
| Division picks: invalid | Prediction sheet | Submit button disabled; caption turns red and states exactly what's missing (e.g. short a division, over a conference cap). |
| Empty / unentered value at deadline | Prediction sheet, Compare | Renders as a faint "—", never blank space. Scores zero for that item, per PRD FR-11 — resolves an open click-dummy question; see "Resolved click-dummy open questions" below. |
| Compare: upcoming round selected | Compare | Not reachable in practice while the round stays gated (§ Predict gating), but if shown: a dashed "matchups not set" note in place of the comparison table. |
| [ASSUMPTION] Compare: default/initial selection | Compare | Not specified by the click-dummy sources. On first entry, the earliest "Before the season" set is pre-selected (rather than showing no table) — confirm before `bmad-architecture`. |
| [ASSUMPTION] Player awards: name rejected | Prediction sheet | Not specified by the click-dummy sources beyond "rejected, not silently accepted" (PRD FR-17). Assumed treatment: the offending input gets a `goal`-colored border plus an inline caption naming the problem, consistent with the Login (code) error pattern — confirm before `bmad-architecture`. |
| [ASSUMPTION] Login: malformed/empty email or in-flight submit | Login (email), Login (code) | Not specified anywhere. No click-side validation-error or loading state is designed in this pass — treat both submit actions as instant for v1 unless `bmad-architecture` says otherwise. |
| Login: wrong/expired/used code | Login (code) | Identical error copy in all three cases (see Voice and Tone); field stays on-screen for retry. |

## Interaction Primitives

- Tap to act; no drag, no swipe gestures anywhere in the click-dummy and none introduced here.
- Selection feedback is immediate and local (client-side state) — no page reloads, no spinner-then-refresh pattern for picking a chip, dropdown, or button.
- Validation is enforced at input time where it can be (live caps on Division picks) and at submit time otherwise (disabled button + explanatory caption).
- Every toggleable element (chip, number button, team button) uses the same selected-state language: neutral fill + accent border. There is no separate "correct/incorrect" coloring on any pick control — validity is communicated through captions and pill/banner states, not through recoloring the picks themselves.

## Accessibility Floor

- Tap targets are chip/button sized (≥ `{spacing.tap-target-min}`, 32px); the playoff-round number buttons are `{spacing.tap-target-number-button}` (36px) squares — no interactive element smaller than that.
- Color is never the only signal: status pills carry the status word, not just a color; the Division-picks conference indicator pairs its check/dot with the `n/8` count text, never a bare dot; submit-blocked validity additionally shows a red caption explaining what's missing.
- Numbers use tabular figures (`DESIGN.md.typography.numeric`) so totals and counts don't shift width as they update.
- [ASSUMPTION] Every form input (Login fields, dropdowns, chip groups, text finalist inputs) carries a programmatic label, and focus order follows visual reading order top-to-bottom on every surface — not verified against the click-dummy's markup, called out as a floor to implement against, not a confirmed existing behavior.

## Key Flows

### Flow 1 — Log in (Sadl, a few minutes after requesting a code)

→ `mockups/login-email.html`, `mockups/login-code.html`

1. Sadl opens the app with no active session; Login (email) appears immediately — no shell, no nav.
2. He enters his email and submits.
3. The screen advances to Login (code) with a neutral "check your email" confirmation, worded identically regardless of whether his email matched a player.
4. He copies the 6-digit code from the email he received and enters it.
5. **Climax:** the code is accepted and the app shell loads straight into Predict — no intermediate "welcome" screen, just the app, immediately useful.

Failure: a wrong, expired, or already-used code returns him to the same Login (code) screen with the identical generic error, and he can retype or request a fresh code.

### Flow 2 — Fill in picks before a deadline (Basti, phone, deadline morning)

1. Basti opens Predict; his still-open sets show blue (Open) or green (Submitted) accent stripes under "Before the season."
2. He taps an Open set; the Prediction sheet opens full-screen with his current (empty or partial) picks.
3. He fills in what he has time for — Division picks capping live as he taps chips — and leaves the rest blank.
4. He taps Submit predictions.
5. **Climax:** the sheet closes back to Predict, the set's stripe and pill turn green ("Submitted"), and whatever he left blank simply isn't blocking him — it'll score zero (PRD FR-11), not stop him from saving the rest.

### Flow 3 — Compare picks mid-playoffs (Tobbi, right after Round 1 results)

1. Tobbi opens Compare.
2. He taps "Playoffs" among the selector rows, then the "Playoff Round 1" chip.
3. The table swaps to show all three players' Round 1 series picks side by side, his own column visually distinguished.
4. **Climax:** he scans the row for the series he just watched and sees, at a glance, who called it and who didn't.

### Flow 4 — Check standings after a result lands (Sadl, right after a round is scored)

1. Sadl opens Leaderboard.
2. The table shows Regular, Playoff, and Total for all three players, sorted by Total.
3. **Climax:** his new rank and Total (or the leader's gold-highlighted row, if it isn't him) are immediately visible — no tap required to see where he stands.

### Flow 5 — A later round unlocks (Basti, once Round 2 matchups are known)

1. Basti opens Predict; "Playoff round 2" was previously shown dimmed with a lock icon under "Playoffs."
2. Once the round's matchups are recorded in `fantasy-hockey.yml` (out of band — no in-app action, no screen; PRD FR-20), the row loses its lock and dim state on his next visit.
3. **Climax:** he taps in and sees real matchups grouped under "Eastern Conference" / "Western Conference," ready to predict — not a placeholder.

## Resolved click-dummy open questions

`imports/clickdummy-requirements.md` and `imports/clickdummy-ui-ux.md` explicitly flag several behaviors as undecided (❓). Those questions were subsequently resolved by the finalized PRD (`_bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-14/prd.md`), which is why this spine states them as settled rather than open — not an invention, but a later resolution. Recorded here for traceability, since the source docs that raised the questions will be deleted:

- **Leaderboard tie-breaker** (`clickdummy-requirements.md` US-4.2, "❓ tie-breaker rules"). Resolved by PRD FR-23: no tiebreaker of any kind; equal Totals share the same rank.
- **Empty/unfilled pick scoring** (`clickdummy-ui-ux.md` §11, "Scoring rules... are not yet defined"). Resolved by PRD FR-11: an empty required pick scores zero at deadline.
- **Award-finalist name validation** (`clickdummy-requirements.md` US-2.5, "❓ validated against a real player roster, or remain free text?"). Resolved by PRD FR-17: validated entry, non-matching names rejected — a stricter behavior than the click-dummy's native (non-restrictive) datalist autocomplete.
- **Before-season deadline strategy** (`clickdummy-requirements.md` US-1.6, "❓ shared vs. staggered deadlines"). Resolved by PRD FR-12: each set's deadline is independent and data-driven — either pattern is valid per season, so the click-dummy's current shared-deadline sample data is not a rule to preserve.

## Open Items

- **Leaderboard scoring is sample data only in the click-dummy.** The actual numbers in `imports/faceoff-pool-dist`/`imports/faceoff-pool-source` are placeholders (`clickdummy-ui-ux.md` §11, `clickdummy-requirements.md` US-4.1 🟡) — real scoring follows PRD FR-24's point table, not anything currently displayed in the prototype build.
- ~~Terminology diverges from the PRD Glossary — three terms.~~ **Resolved.** The PRD (`prd-fantasy-hockey-2026-09-14`) was updated to match this spine: "Standings" → "Leaderboard," "Awards (individual)" → "Player awards," "Playoffs Cup re-pick" → "Playoffs Cup pick." Both documents now agree.
- ~~FR-22 (reminder-email trigger) has no v1 UI.~~ **Resolved.** The PRD was updated to carve FR-22/UJ-4 out of MVP scope (§6.2), matching this spine's "Deferred" section rather than contradicting it.
- **[ASSUMPTION] Session-timeout UX and accessibility floor** (see State Patterns, Accessibility Floor) are inferred defaults consistent with the product's existing terse style, not confirmed against user intent or the click-dummy's actual markup — worth a quick confirm before `bmad-architecture`.
