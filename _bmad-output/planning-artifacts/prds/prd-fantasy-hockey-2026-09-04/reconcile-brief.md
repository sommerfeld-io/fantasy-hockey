---
title: 'Reconciliation: brief.md vs prd.md'
created: '2026-09-04'
---

# Reconciliation: Product Brief vs. PRD

Source input: `_bmad-output/planning-artifacts/briefs/brief-fantasy-hockey-2026-09-04/brief.md`
Built artifact: `_bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md`

## Overall Assessment

The PRD is a faithful, well-traced elaboration of the brief. Every Must-have, Could-have, and Won't-have scope item, every success criterion, every "Who This Serves" rationale point, and all three Known Risks are addressed somewhere in the PRD — most with explicit cross-references back to the brief or `research.md`. No unexplained contradictions were found; the few places where the PRD is more specific than the brief (e.g. FR-10's "exactly two" reminder emails vs. the brief's unspecified count, FR-2/FR-3's 10-minute code expiry and 30-minute session timeout) read as normal PRD-stage elaboration, not silent deviation.

Two minor gaps are worth flagging.

## Gaps Found

1. **"Teams identified by a stable technical ID" has no functional requirement.** The brief lists this as one of its explicit Must-have (v1) bullets, on par with login, scoring, and standings. In the PRD it survives only as a Glossary definition (§3, "Team"): "identified internally by a stable technical ID independent of its display name, so relocations/renames... don't break historical data." It is not promoted to a numbered FR, and it is not listed among the FR ranges enumerated in §6.1 MVP Scope. Since the PRD's stated purpose (§0) is to "distill [brief, brainstorm, research] into features with nested, globally-numbered functional requirements... that downstream UX and architecture work can reference directly," this item is the one Must-have that has no FR number to reference and no testable consequence — architecture work could plausibly miss it. Recommend either adding a short FR under §4.6 (Automated Data Sync) or §4.2 (Enter Predictions), or explicitly noting in §6.1 that it's a data-modeling constraint rather than a user-facing FR.

2. **"Standings are always trustworthy" has no dedicated success metric.** The brief's Success Criteria list it as a top-level, independent bullet alongside the Carolina-bug fix and deadline enforcement. The PRD's §7 Success Metrics has SM-1 (zero scoring errors) and SM-2 (deadline completion) but nothing that specifically measures standings correctness/availability as its own criterion — it's only indirectly implied by SM-1 (since standings derive from the scoring engine). This is a minor omission; SM-1 arguably subsumes it, but the brief treated it as a distinct, separately-checkable criterion (a participant "can check... at any time and get a correct answer without asking anyone" — an availability/trust property, not just a correctness-of-computation property).

## Not Gaps (Checked and Confirmed Present)

For completeness, these brief items were verified present in the PRD and are not flagged:

- All 8 Must-have scope bullets except the technical-ID item above (login/logout, enter predictions, deadline lock + reminders, visibility rules, automated sync, scoring engine, standings, fixed 3-participant model).
- All 3 Could-have items (multi-season/season-selector, Season Hub, Hall of Fame) — deferred to v2 in §6.2, including the PM note about the user's enthusiasm for Hall of Fame.
- All 6 Won't-have items (no admin/scorekeeper, no registration, no real-time sync, no automated Hart/Norris/Vezina scoring, no deep per-category breakdown, no tie-break) — reflected in §4.5, §4.7 FR-20, and §5 Non-Goals.
- All 3 Known Risks (data source reliability, legal ambiguity, awards scoring gap) — reflected in Constraints and Guardrails, FR-14/15, §4.5's manual-entry resolution, and Open Question 3.
- "Who This Serves" rationale (no secondary user, deliberate design decision) — §2.2 and §5.
- The awards-scoring decision the brief flagged as needing resolution "before build starts" (manual entry vs. informational-only) was resolved as manual entry (FR-13) — a legitimate coaching-stage decision, not a silent contradiction.
