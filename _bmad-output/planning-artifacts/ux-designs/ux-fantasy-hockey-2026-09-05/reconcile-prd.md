---
title: 'Fantasy Hockey — PRD ↔ UX Reconciliation'
created: '2026-09-05'
inputs:
  - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md
  - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/DESIGN.md
  - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/EXPERIENCE.md
---

# Reconciliation: PRD vs. UX (DESIGN.md + EXPERIENCE.md)

## Method

Walked every Feature/FR (§4.1–4.8), every Glossary term, both Key User Journeys, the qualitative
principles (transparency without friction, counter-metric SM-C1), and the Non-Goals/Cross-Cutting
NFRs against DESIGN.md's Components/Colors and EXPERIENCE.md's IA / Component Patterns / State
Patterns / Key Flows. Below: full coverage map, then gaps, then contradictions/tensions.

## Coverage — well reflected, no action needed

- **FR-1–FR-2 (login, generic response, exact wording)** — Login form pattern + Voice and Tone
  table quote PRD's exact strings ("Check the entered email address.", "Invalid code.").
- **FR-4 (logout)** — IA table + nav bar spec.
- **FR-5 (autocomplete, reject non-matching)** — Autocomplete input component + State Patterns
  "Autocomplete rejection" row, inline `danger` treatment.
- **FR-6 (incremental save, winner+count as one unit)** — Prediction row / Button(primary) pattern
  explicitly: "Per-row Save appears once the row's fields are valid (winner + game count both set
  for a Series, per FR-6)... no page-level Save button."
- **FR-7 (edit until deadline)** — implied by editable-row behavior + Key Flow UJ-1 edge case.
- **FR-8 (lock, no grace period)** — "Locked" state pattern + badge component, text-only per
  DESIGN.md's "not an error state" framing.
- **FR-9 (missing = 0, not blocking)** — "Missing at deadline" state pattern, explicit.
- **FR-10 (reminder wording, exactly two, no other content)** — Voice and Tone table quotes PRD's
  exact string ("Action required until {timestamp}.").
- **FR-11 (own predictions always visible)** — implied by "Locked (post-deadline, own)" state row
  still rendering the value.
- **FR-12 (others hidden pre-deadline, not masked)** — State Patterns "Hidden (others', pre-deadline)"
  row is explicit: "Not rendered at all — no blurred/masked placeholder."
- **FR-16 / no in-app alerting** — "There is no loading/sync-status state exposed anywhere..." directly
  mirrors FR-16's no-indicator rule.
- **FR-22 (no live/projected tier)** — Standings widget pattern: "no 'live' indicator since there is
  no live tier" is close to verbatim from the PRD consequence text.
- **"Transparency without friction"** — named explicitly in EXPERIENCE.md's IA section.
- **SM-C1 counter-metric** — named explicitly in Inspiration & Anti-patterns ("consistent with the
  PRD's counter-metric against optimizing for engagement").
- **UJ-1** — Key Flow beats match 1:1 (standings check → Enter Predictions → per-row Series saves →
  Late Pick → all-saved climax → mid-way-leave edge case with no all-or-nothing submission).
- **UJ-2** — Key Flow beats match (rank check → others' picks revealed → informal compare → no
  action taken), plus a PRD-consistent added edge case (checking before the deadline closes).
- **Non-Goals** (no admin/scorekeeper role, no native app, no multi-season UI) — correctly absent
  from both UX files; nothing invented that PRD excludes.

## Gaps — PRD requirements with no UX treatment

1. **FR-3, session timeout (30 min inactivity) has no UX treatment at all.** Neither
   DESIGN.md nor EXPERIENCE.md's State Patterns table mentions what a Participant sees when their
   session expires mid-task (e.g., mid-way through entering Round 1 picks) — no "session expired"
   message, no redirect behavior, nothing in the Voice and Tone table alongside the other error
   strings. This is genuinely user-facing (an interrupted task, unlike FR-2's expiry which only
   matters at code-submission time) and is silently missing.

2. **FR-21 (no tie-break, shared rank) is referenced but not specified.** EXPERIENCE.md's UJ-2 flow
   says "Standings widget shows his updated rank," but DESIGN.md's `standings-widget` component spec
   lists only three data columns (Regular Season / Playoffs / Total) — no rank/position element is
   defined anywhere. There is no rule for how a shared rank (two Participants tied on Total) is
   rendered (e.g., both shown as "1st", or rank simply not displayed and only row order/points imply
   it). The word "rank" is used in a flow before a corresponding UI element is designed.

3. **FR-13's last-write-wins conflict handling for Award Data Entry is unaddressed.** The State
   Patterns table itemizes autocomplete rejection and invalid-code errors but has no row for what
   (if anything) is shown when two Participants save the same trophy's finalists near-simultaneously.
   Likely intentional (nothing should be shown — it's invisible by design, symmetric with FR-16), but
   this is never stated, so it's ambiguous whether its absence is a decision or an oversight.

4. **Award Data Entry's "no Deadline of its own" property is not called out.** It is the one Prediction
   surface that is never locked. EXPERIENCE.md's State Patterns table (Not yet unlocked / Locked /
   Hidden / Missing) is written generically as if all surfaces follow the deadline lifecycle; nothing
   clarifies that this page is exempt. Low severity, but worth an explicit note given how central the
   lock/unlock lifecycle is to every other pattern in the table.

## Contradiction / internal tension (not from the PRD directly, but worth flagging)

5. **DESIGN.md's `success` color is defined but never operationalized in EXPERIENCE.md, and sits in
   tension with the "no gamification chrome" anti-pattern.** DESIGN.md states: "Success marks a
   correct/scored prediction after results land." No Component Pattern or State Pattern in
   EXPERIENCE.md defines when/where this appears (per Series row? per Award pick? only after Sync?).
   More importantly, EXPERIENCE.md's own Inspiration & Anti-patterns section rejects "gamification
   chrome (streaks, badges, celebratory animations on a correct pick)" and its State Patterns table
   insists a missing prediction is "not flagged or highlighted differently from a filled one; the
   score itself... tells the story." A dedicated green "correct" color for individual predictions
   pushes against that same "let the score speak for itself, don't decorate correctness" posture the
   rest of EXPERIENCE.md commits to. This isn't a PRD contradiction (the PRD is silent on whether
   individual picks get correctness styling) — it's a DESIGN.md addition that EXPERIENCE.md never
   picked up and that mildly conflicts with EXPERIENCE.md's own stated philosophy.

No other contradictions found — no case where a UX file states a rule that PRD directly forbids or
reverses the direction of a required behavior.
