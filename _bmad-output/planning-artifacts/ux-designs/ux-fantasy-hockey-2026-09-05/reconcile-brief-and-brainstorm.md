---
title: 'Reconciliation: UX Files vs. Brief & Brainstorm Intent'
status: draft
created: '2026-09-05'
---

# Reconciliation: DESIGN.md / EXPERIENCE.md vs. Brief & Brainstorm Intent

## Inputs

- Brief: `_bmad-output/planning-artifacts/briefs/brief-fantasy-hockey-2026-09-04/brief.md`
- Brainstorm intent: `_bmad-output/brainstorming/brainstorm-fantasy-hockey-web-app-2026-09-04/brainstorm-intent.md`
- UX files under review: `DESIGN.md`, `EXPERIENCE.md` (both `_bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/`)

Note: this reconciliation covers only the brief and brainstorm doc against the UX files. The PRD-to-source reconciliation was done separately and is out of scope here, except where a UX detail traces back to a PRD decision that itself creates tension with the brief/brainstorm — those are called out explicitly below.

## 1. Brainstorm "UI Shape" section — faithfulness check

| Brainstorm element | Where it lands in EXPERIENCE.md | Verdict |
|---|---|---|
| Screens: Login, Enter Predictions, Predictions (view-all), Logout | IA table lists Login, Predictions (home), Enter Predictions, Logout | Faithful — "Predictions (view-all)" renamed "Predictions (home)" but same surface/purpose. |
| Season Hub / Hall of Fame deferred to Could | Absent from EXPERIENCE.md entirely | Correct — deferred items should not appear in v1 UX spec. |
| "No-tab single scrollable predictions page" | IA: "Flat IA on purpose: no tabs, no nested navigation, no modal stacks." Predictions (home) shows "Everyone's Predictions across all categories" on one surface. | Faithful in substance. Minor: EXPERIENCE.md never uses the word "scrollable" itself — the no-tab, single-surface behavior is present, but the specific brainstorm phrasing ("single scrollable") isn't echoed. Cosmetic, not a gap in behavior. |
| Standings as persistent widget "because there are only 3 players" | IA: "Standings is not a surface — it's a persistent widget... on every authenticated page." Responsive & Platform: "stays a full-width top bar at every viewport width... never moves to a sidebar, footer, or collapsible drawer." | Faithful in behavior. The stated *reason* ("because there are only 3 players") from the brainstorm/brief is not repeated as rationale anywhere in EXPERIENCE.md or DESIGN.md — the outcome matches but the justification is silently dropped rather than cited. Minor documentation gap, not a functional one. |
| "Transparency without friction" guiding philosophy | EXPERIENCE.md IA: "Matches the 'transparency without friction' principle carried from product discovery (PRD §4, intro)." | Faithful — phrase is carried through verbatim, though attributed to the PRD rather than to its origin in the brainstorm doc. Not a problem, just a provenance note. |

**Extra surface not in brainstorm's UI Shape list:** "Award Data Entry" (manual Hart/Norris/Vezina finalist entry). Not present in the brainstorm's screens enumeration or in the brief's Scope. See §3 (Contradictions) — this is the one item worth flagging.

## 2. Brief's Vision / Who This Serves tone — faithfulness check

| Brief tone element | Where it lands | Verdict |
|---|---|---|
| "Not trying to become a bigger product," not chasing engagement | DESIGN.md Brand & Style: "not a product trying to win anyone's attention... a well-kept scoreboard, not an app fighting for engagement." EXPERIENCE.md Inspiration & Anti-patterns: "Rejected — gamification chrome... consistent with the PRD's counter-metric against optimizing for engagement — a quiet scoreboard, not a habit loop." | Faithful, and stated explicitly in both files. |
| Low-effort, forgettable-between-deadlines | EXPERIENCE.md: reminder-email copy restricted to "Action required until {timestamp}" (no newsletter/result content); "Banned: ...any auto-refresh or polling UI"; no notification/badge counts; no loading/sync-status state anywhere. | Faithful by implication — nothing in the UX spec gives a participant a reason to open the app between deadlines, which is the behavioral expression of "forgettable between deadlines." The literal phrase "forgettable between deadlines" is never quoted, but the design choices that produce that outcome are all present. |
| Quiet passion project, plain/neutral voice among friends | Voice and Tone table: "Plain and neutral throughout — no banter, no exclamation marks, even though the users are friends." | Faithful and explicit — this is close to a direct restatement of the brief's framing. |
| "Steel Ice" register, dark-only, no hockey-arena visual cliché | DESIGN.md Brand & Style / Inspiration & Anti-patterns | This is original UX-designer creative work with no direct brief/brainstorm precedent (neither source doc specifies a visual register) — expected and appropriate, not a gap. |

No tone element from the brief's Vision or Who This Serves sections appears to have been silently dropped.

## 3. Contradictions

**One real tension, inherited via the PRD, worth flagging:**

- The brief's Scope explicitly lists as "Explicitly won't (this version)": *"Any scorekeeper or admin manual-data-entry role or page."*
- The brief's Known Risks section separately flags the awards-scoring gap as an **open decision** still needing resolution "before build starts": *"manual entry once a year for these three trophies, or informational-only with no scoring at all."* The brief itself does not resolve this — it names it as unresolved tension between the "no manual entry" principle and the Hart/Norris/Vezina data gap.
- EXPERIENCE.md's IA table includes a full surface, **Award Data Entry** — "Manual Hart/Norris/Vezina finalist entry (PRD §4.5)" — reachable via its own nav link, alongside Predictions and Logout.

This is not a contradiction the UX files introduced on their own — it resolves an open question the brief explicitly deferred, and the resolution (manual entry, not informational-only) presumably happened at the PRD stage. But taken at face value against the brief's own "Explicitly Won't" bullet, a literal "manual-data-entry... page" now exists in the product, and neither DESIGN.md nor EXPERIENCE.md acknowledges or cites this tension — they cite PRD §4.5 as if it were uncontested. Worth a second look: is "Award Data Entry" meaningfully different from the "admin manual-data-entry role or page" the brief ruled out (e.g., because it's open to all 3 participants symmetrically, not a distinct admin/scorekeeper role), or is this a case where the PRD quietly reopened a scope item the brief had closed? Recommend confirming this was a deliberate, informed reversal rather than an inherited oversight.

**No other contradictions found.** Everything else in the UX files that touches brief/brainstorm content either matches directly or extends it with UX-designer creative detail that neither source document rules out.

## 4. UX-relevant details unique to brief/brainstorm, checked against UX files

- Brief Success Criteria "each participant reliably gets a reminder before their own deadline closes" → present (Voice and Tone reminder copy).
- Brainstorm "Guiding design principle: removing friction — technical (manual entry, disputes) and social (tie-breaks, scorekeeper judgment calls)" → present via "transparency without friction," no tie-break UI, no dispute/admin surface.
- Brief "no admin, no scorekeeper, no visitor/spectator role" → no such role introduced anywhere in DESIGN.md/EXPERIENCE.md.
- Brief "Real-time sync... no live/projected tier" → EXPERIENCE.md explicitly: "no 'live' indicator since there is no live tier"; no loading/sync-status state exposed.
- Brainstorm's "Parked for Later" architecture items (multi-container compose, sync container, Grafana Cloud/otel, no in-app alerting) → correctly out of scope for a UX spec, not expected to appear and don't.
- Brainstorm/brief Could-have items (Season Hub, Hall of Fame, multi-season/season-selector) → correctly absent from the v1 UX files.

No unique UX-relevant idea from either source document appears to have been missed.

## Summary

Overall reconciliation is strong: the brainstorm's UI Shape (screens, no-tab single-page predictions view, standings-as-persistent-widget) and the brief's tone (low-effort, forgettable-between-deadlines, not chasing engagement) are both faithfully carried into EXPERIENCE.md and DESIGN.md, in several places nearly verbatim. The one item requiring a decision-owner's confirmation is the "Award Data Entry" page, which resolves a risk the brief left open but arguably lands on the opposite side of the brief's own "Explicitly Won't: manual-data-entry page" bullet — this should be confirmed as an intentional, informed resolution rather than silently inherited.
