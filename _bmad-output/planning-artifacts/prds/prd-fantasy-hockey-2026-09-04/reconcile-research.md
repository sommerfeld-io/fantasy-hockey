# Reconciliation: research.md vs. prd.md

Source input: `_bmad-output/planning-artifacts/research/technical-free-nhl-data-webservices-2026-09-04/research.md`
Target: `_bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md`

## Summary

Most of the research's core findings *are* correctly carried into the PRD: the API choice (§4.6), the Art Ross/Rocket Richard-are-stat-leaders-not-voted-awards insight (FR-16), the Hart/Norris/Vezina manual-entry fallback (§4.5/FR-13), the cost/legal constraints section, and the Wikidata SPARQL lead (Open Question 2). However, three items from the research are either missing or meaningfully downgraded/oversimplified in the PRD.

## Gaps and Mismatches

1. **Recommendation 3 (structural breaking-change risk) is downgraded from a design constraint to a mere open question, and FR-15 doesn't actually cover it.**
   Research explicitly says: "Design the sync layer... assuming a breaking API change will happen again — it already has once — rather than as a low-probability edge case," and states this "should feed directly into the next architecture-planning step as a constraint, not a footnote" (high confidence). The PRD's only echo of this is Open Question 3 ("whether... breaking again... should have any user-visible handling beyond FR-15's silent retry"), which frames it as an optional UX question rather than a load-bearing architectural constraint. Worse, FR-15 ("Resilient to Source Failure") only covers the case where the source "is unreachable or returns an error" — the 2023 incident research cites was a full schema/endpoint replacement, which typically returns HTTP 200 with different or malformed JSON, not an "error." As written, FR-15 would not obviously catch a silent schema-breaking change (e.g. a sync that parses garbage into the DB, or silently syncs nothing because a field renamed), only outright unreachability. This is a real gap between what research recommended and what the PRD requires.

2. **Rate-limiting / throttling risk (Integration & Interoperability section) is omitted entirely.**
   Research notes `api-web.nhle.com` publishes no documented rate limit, that community guidance is informally "cache your requests, don't hammer it," and that at least one third-party client library implements defensive HTTP 429 handling, "implying throttling is encountered in practice." Nothing in the PRD's Automated Data Sync feature (FR-14/FR-15) requires backoff, caching, or any handling of throttling/429s. This isn't in the Open Questions list either — it's simply absent, despite being a distinct operational risk from the "source unreachable" case FR-15 does cover.

3. **The legal ToS ambiguity is stated as more settled in the PRD than the research supports.**
   Research treats whether NHL.com's ToS (which explicitly governs `nhl.com`) actually extends to the `api-web.nhle.com` / `api.nhle.com` subdomains as a genuinely open, unresolved legal question — "could not be resolved from available public sources — would likely require legal review, not further searching" — and Recommendation 4 says explicitly to "flag the ToS ambiguity to the human decision-maker explicitly; do not resolve it by assumption." The PRD's Constraints section instead states flatly: "The NHL data source's Terms of Service technically prohibit automated scraping... This is accepted knowingly for personal, non-commercial, 3-person use." This collapses an explicitly-flagged open legal question (does the ToS even apply to the API subdomain?) into an already-settled fact ("technically prohibit... accepted knowingly"), which slightly overstates certainty in one direction (assumes the ToS does apply) while understating that research asked for this to be an explicit, visible decision point rather than a background assumption baked into a constraints bullet.

## Minor / Low-Impact Omissions (not flagged as meaningful gaps)

- Research's third Open Question ("has NHL signaled any intent about the long-term status of the API, or is silence the permanent norm?") isn't mentioned in the PRD. Low impact — it's speculative and returned no findings either way, so there's nothing actionable to carry forward.
- The nuance that NHL.com's ToS "written approval" clause applies specifically to logos/marks, not general content redistribution, isn't mentioned in the PRD — but this doesn't change the PRD's conclusion and isn't load-bearing for product decisions.

## What the PRD Gets Right (accurately reflects research)

- Correctly identifies which 3 of 4 original data needs are automatable (standings, Art Ross/Rocket Richard, playoff bracket/series) vs. not (Hart/Norris/Vezina) — FR-13 vs. §4.6/FR-14.
- Correctly captures the "awards gap is narrower than it looks" insight — Art Ross/Rocket Richard use the same live stat-leaderboard endpoint, not a separate awards feed (FR-16).
- Correctly reflects the "confirmed infeasible for Hart/Norris/Vezina" framing in Constraints.
- Carries forward the Wikidata SPARQL lead as an explicit, unresolved Open Question (§8.2), matching research's Open Question 2 almost verbatim.
- FR-15's "no partial writes, retry next cycle" resilience pattern for outright unreachability matches research's description of the project's planned resilience pattern for the technical half of the risk.
