# PRD Quality Review — Fantasy Hockey Web App (2026-09-04)

## Overall verdict

This is a strong, well-calibrated hobby-tier PRD: it has a specific thesis (eliminate the three concrete manual failure modes that already broke trust in the Excel sheet), features that trace cleanly back to that thesis, testable FR consequences throughout, and honest scope/assumption tagging. It is not over-formalized for a 3-person tool and not under-formalized for a multi-user game with real UX (visibility rules, deadlines). The few gaps are narrow and fixable: a couple of soft-language NFRs, one under-specified concurrent-edit case (FR-13), a small glossary drift ("Late Season Cup pick" vs. the defined term "Late Pick"), and an unresolved working-title decision that never made it into Open Questions.

## Decision-readiness — strong

Trade-offs are named with what's given up, not just what's chosen. Examples: the Legal constraint (§ Constraints and Guardrails) states plainly that NHL.com's ToS "prohibit automated scraping in general" and that whether it covers this API subdomain is "a genuinely open legal question, not a settled one," and the PRD accepts that risk rather than resolving or hiding it. The Cost constraint states what's "confirmed feasible" vs. "confirmed infeasible" (Hart/Norris/Vezina), and FR-13 exists precisely because of that gap rather than smoothing over it. Open Questions (§8) are genuinely open — e.g. Q2 (possible Wikidata SPARQL source for award finalists) is an unresolved lead, not a rhetorical question answered in the next sentence. `[NOTE FOR PM]` at § 6.2 (Hall of Fame deferral) sits at a real tension — the user's most visible enthusiasm vs. a deliberate v2 deferral — not a safe checkpoint.

### Findings
- **low** Working title left unresolved outside the Open Questions list (§ Title, line 9: "*Working title — confirm.*") — Note: this is a live open decision but isn't tracked in §8 Open Questions, so it could be lost. *Fix:* either resolve the title now or add it as an Open Question.

## Substance over theater — strong

No persona theater: exactly two named participants drive the two UJs (Basti in UJ-1, Sadl in UJ-2), and Tobbi appears only as a fixed participant, not as inflated persona material. No innovation/differentiation section — appropriately absent, since this isn't a market product. The Vision (§1) is specific to this project, not swappable: it names the actual failure ("a confirmed real error (a correct 20-point Stanley Cup pick that was never credited)") rather than generic aspirational language. Cross-Cutting NFRs (§ Cross-Cutting NFRs) are not boilerplate — "No partial writes," "Reproducibility," and "No enumeration leaks" are each tied to a specific mechanism or incident (the Carolina-pick bug), not copy-pasted "must be scalable/secure" filler.

No findings — this dimension is clean.

## Strategic coherence — strong

The thesis is explicit and singular: replace manual scoring/deadline/typo failure points while keeping the game identical (§1). Feature order and MVP scope follow from it — Auth is minimal by design (no registration, three hardcoded emails), and every FR ties back either to eliminating a specific failure mode or preserving existing gameplay. Success Metrics validate the thesis directly rather than measuring activity: SM-1 ("Zero manually-corrected scoring errors... displayed Standings always match a hand-check") and SM-2 (deadline completion) are the literal antidotes to the two failure modes named in §1. SM-C1 is a genuine counter-metric — it explicitly warns against optimizing session frequency/time-in-app, which is the DAU/MAU trap the rubric flags, and ties it back to SM-2 ("don't chase completion by nagging harder than the two-reminder rule"). MVP scope logic is coherent: everything needed to run one Season end-to-end is in scope; only cross-season features (multi-season model, Season Hub, Hall of Fame) are deferred to v2, and the reasoning for each deferral is stated, not just asserted.

No findings — this dimension is clean.

## Done-ness clarity — strong, with isolated gaps

Nearly every FR carries at least one testable consequence with concrete numbers (FR-2's 10-minute expiry, FR-3's 30-minute session window, FR-17–20's exact point values, FR-10's "exactly two reminder emails"). Hedge words like "reasonable performance" or "handles X gracefully" do not appear.

### Findings
- **medium** No conflict-resolution rule for concurrent edits to shared data (FR-13) — FR-13 lets "any authenticated Participant" edit the same Hart/Norris/Vezina finalist fields, but unlike FR-7 (which governs a Participant's *own* Predictions and explicitly says "no edit history... required"), there is no stated behavior for what happens when two Participants edit the same shared field around the same time (e.g., last-write-wins, and whether that's acceptable under the "trust" rationale given in §4.5). *Fix:* either state last-write-wins explicitly for FR-13 or add a one-line NFR extending FR-7's "no history required" rule to shared-entry data.
- **low** Un-quantified backoff behavior — the §4.6 Feature-specific NFR says the Sync "must back off rather than retry aggressively" and "should avoid redundant requests," but gives no bound (max retry count, backoff multiplier, or ceiling). Given the hobby-tier stakes this is a minor gap, not a blocker. *Fix:* either leave as an implementation parameter (say so explicitly, as FR-10's reminder timing already does) or add a rough bound.
- **low** FR-6's "any subset" of Predictions doesn't specify field-level granularity for a single Series (e.g., can a winner be saved before its game count, leaving the Series pick partially filled?). Likely fine given FR-9's "empty scores zero" fallback, but not explicitly stated at the field level. *Fix:* one sentence clarifying whether partial-Series saves are allowed or whether a Series pick is atomic (winner + game count together).

## Scope honesty — strong

Non-Goals (§5) do real work — they foreclose specific things a reader might otherwise assume are just deferred (no admin role ever, no mobile app, not monetized). Two `[ASSUMPTION]` tags (§3 on "Standings" as one glossary term; §4.6 FR-15 on mid-season relocations) are both indexed in §9, and both entries in §9 round-trip back to their inline locations — no drift. One `[NOTE FOR PM]` (§6.2) flags a genuinely deferred, emotionally-loaded decision (Hall of Fame) rather than a safe one. Open-items density (3 Open Questions + 2 Assumptions + 1 NOTE FOR PM = 6) is appropriately low for a private 3-person build and does not read as a blocker.

No findings — this dimension is clean.

## Downstream usability — strong

This PRD explicitly feeds UX and architecture work (§0: "downstream UX and architecture work can reference directly"), so this dimension carries real weight. The Glossary (§3) is thorough and terms are used consistently in almost every case (Participant/Player distinction is maintained throughout; Deadline, Series, Prediction, Result, Standings, Sync all read identically wherever they appear). FR IDs (FR-1 through FR-22) are contiguous with no gaps or duplicates, cleanly partitioned across features 4.1–4.8. UJs (UJ-1, UJ-2) each have a named protagonist with inline context (Basti, Sadl) — no floating UJs. Success Metrics cite their validating FR ranges (e.g., "Validates FR-17–22"), and those ranges resolve correctly.

### Findings
- **low** Glossary drift: "Late Season Cup pick" (UJ-1, § Key User Journeys) vs. the defined term "Late Pick" (§3 Glossary, §4.7 FR-19). Same concept, different phrasing — minor but worth normalizing since UJs are meant to be pulled out and read standalone against the Glossary. *Fix:* change UJ-1's wording to "Late Pick" to match §3 exactly.

## Shape fit — strong

This is a hobby/solo-adjacent, small multi-user (3-person) tool, and the PRD's shape matches: light UJ use (exactly two, both load-bearing — UJ-1 covers the entry path, UJ-2 covers the visibility/standings path), no admin role invented where none exists, no market/competitive framing, and NFRs scoped to what a self-hosted 3-person Docker deployment actually needs (no scalability/SLA sections). The Constraints and Guardrails section (Cost, Deployment, Legal) is exactly the kind of concrete, product-specific constraint list a hobby build needs instead of generic enterprise NFRs. Nothing here reads as over-formalized (no persona inflation, no fake stakeholder matrix) or under-formalized (the two UJs that matter are present and detailed).

No findings — this dimension is clean.

## Mechanical notes

- **Glossary drift:** "Late Season Cup pick" (UJ-1) vs. "Late Pick" (§3, §4.7) — see Downstream usability finding above.
- **ID continuity:** FR-1–FR-22 contiguous, no gaps/duplicates across §4.1–4.8. UJ-1/UJ-2, SM-1/2/3 + SM-C1 all contiguous and referenced correctly (e.g., §7 SM-1 "Validates FR-17–22," SM-2 "Validates FR-8–10").
- **Assumptions Index roundtrip:** Both inline `[ASSUMPTION]` tags (§3 Glossary; §4.6 FR-15) are indexed in §9, and both §9 entries trace back to real inline tags — clean roundtrip, nothing orphaned in either direction.
- **UJ protagonist naming:** UJ-1 (Basti) and UJ-2 (Sadl) both carry a named protagonist with inline context per the rubric's expectation; no floating UJs.
- **Required sections for stakes/type:** All sections expected for a hobby-tier, small-multi-user capability spec are present (Vision, Target User/JTBD, Glossary, Features/FRs, Cross-Cutting NFRs, Constraints, Non-Goals, MVP Scope, Success Metrics, Open Questions, Assumptions Index). No missing section flagged.
