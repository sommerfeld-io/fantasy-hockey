# Reconciliation: brainstorm-intent.md vs prd.md

Source: `_bmad-output/brainstorming/brainstorm-fantasy-hockey-web-app-2026-09-04/brainstorm-intent.md`
Target: `_bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md`

The PRD faithfully carries forward almost all of the brainstorm's resolved domain rules (award scoring with tie-expansion, no tie-break, prediction visibility rules, deadline immutability, automated sync, manual entry for the 3 non-automatable awards, cost/legal constraints, Could/Won't scope deferrals). No outright reversal of a brainstorm decision was found. The gaps below are omissions and one large untraceable addition, not contradictions.

## Gaps / items not reflected in the PRD

1. **Playoff Series predictions (FR-19) have no counterpart anywhere in the brainstorm.** The brainstorm's only playoff-team scope item is "Playoff team marks" (playoff berth + one division-winner mark per division). It never mentions round-by-round Series predictions (winner + exact game count, scored per round: R1/R2/Conference Finals/Stanley Cup Final) or per-round Deadlines. This is a substantial addition to both the Enter Predictions feature and the Deadline model (up to 5 deadlines across a season: pre-season, before R1, before R2, before CF, before SCF) that isn't sourced from this document — likely pulled from `brief.md`, but the PRD gives no provenance note. It's also in mild tension with the brainstorm's explicit framing of "only a handful of touchpoints per season (pre-season predictions, pre-playoffs predictions) — not weekly engagement."

2. **Presidents' Trophy prediction (Glossary, FR-18) is entirely new.** It appears nowhere in the brainstorm's Domain & Scoring Rules table or MoSCoW list. Same concern as #1 — not traceable to this source, no rationale given in the PRD for the addition.

3. **The brainstorm's named UI/design philosophy is absorbed into FRs but never restated as a principle.** The brainstorm explicitly names a guiding philosophy — "transparency without friction," a "no-tab single scrollable predictions page," and standings as a persistent sidebar/header widget rather than its own page (because there are only 3 players). The PRD captures the underlying mechanics as individual FRs (FR-11/12 visibility, §4.8 standings-everywhere) but drops the "single scrollable page, no tabs" UI constraint and never states the unifying design principle anywhere for downstream UX work to inherit intentionally rather than by accident.

4. **No explicit requirement for populating the per-season Team list from a webservice on season creation.** The brainstorm lists this as a Must ("Technical (stable) team ID data model, with per-season team list persisted in the data store... loaded from a webservice when a new season is created"). The PRD's Glossary and FR-5 reference the canonical per-season Team list but never state the mechanism or trigger that populates it. This may be a reasonable implicit deferral since MVP is single-season (§6.2), but the PRD doesn't say so explicitly.

5. **Minor: NHL playoff-qualification structure (8 teams/conference across 2 divisions, 4/4 or 5/3 split) is dropped.** Low severity — it's realized automatically via the Sync's external data rather than being a product decision — but the brainstorm listed it as a resolved domain rule and the PRD's Glossary/FR-17 don't restate it.

6. **Minor: the "continuously scorable all season" nuance for Art Ross/Rocket Richard vs. "only after nominees published" for Hart/Norris/Vezina is implicit, not stated.** The brainstorm frames this as a resolved rule with explicit data-availability rationale; the PRD's FR-16/§4.5/§4.6 split accomplishes the same effect structurally but never states the rule or its rationale directly.

## Not flagged as gaps (intentional/explained evolution)

- FR-13's manual entry page for Hart/Norris/Vezina looks at first glance like it contradicts the brainstorm's "no manual confirmation step, no admin data-entry page" result-ingestion rule — but the brainstorm's own "Biggest Open Risk — RESOLVED" section anticipates exactly this, stating these 3 trophies "need a different plan for v1 (manual/rare entry once per season...)." Correctly reconciled, not a silent contradiction.
- Specific numeric/timing refinements (10-minute code expiry, 30-minute session timeout, exactly two reminder emails) are elaborations beyond the brainstorm's coarser rules, not contradictions.
- The "Carolina incident" origin story in the Vision/JTBD/FR-16 isn't in this source document — presumably from `brief.md`. Not a contradiction, just worth noting it didn't originate here.
