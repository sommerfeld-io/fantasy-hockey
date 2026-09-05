---
title: 'Reconciliation: technical-free-nhl-data-webservices research vs. ARCHITECTURE-SPINE'
type: 'Reconciliation note'
created: '2026-09-05'
inputs:
    - _bmad-output/planning-artifacts/research/technical-free-nhl-data-webservices-2026-09-04/research.md
    - _bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-05/ARCHITECTURE-SPINE.md
---

# Reconciliation: NHL data research vs. Architecture Spine

## Method

Checked the research's four Recommendations, three Open Questions, and four
key findings (no published rate limit, 2023 full-API-migration precedent,
ToS ambiguity, confirmed absence of a free Hart/Norris/Vezina source)
against the spine's `internal/nhlclient` design, AD-17, and Deferred section.

## Recommendation-by-recommendation

| # | Research recommendation                                             | Spine handling                                                                                                  | Verdict |
|---|-----------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|---------|
| 1 | Adopt `api-web.nhle.com` for standings, Art Ross/Rocket Richard, playoffs | `internal/nhlclient` + `internal/sync` (§4.6, AD-1, AD-9, AD-17) built exactly for this                          | Addressed |
| 2 | Do NOT automate Hart/Norris/Vezina; keep manual                       | `internal/awards` source-tree comment explicitly names "manual Hart/Norris/Vezina finalist entry" (FR-13, §4.5) | Addressed, precisely scoped |
| 3 | Treat data-source risk as structural, not incidental                  | AD-17's "schema-validation failure → write nothing → retry next run" — see gap below on mechanism depth          | Partially addressed (mechanism gap, see Gap 1) |
| 4 | Flag ToS ambiguity to the human; do not resolve by assumption         | Deferred: "NHL.com Terms of Service ambiguity" — explicitly framed as "knowingly accepted... rather than resolved" | Addressed, wording matches research's "accept, don't engineer around" framing |

## Open-question-by-open-question

| # | Research open question | Spine handling | Verdict |
|---|---|---|---|
| 1 | Does NHL.com ToS extend to the `nhle.com` API subdomain? | Carried into Deferred verbatim in substance | Correctly carried |
| 2 | Does per-player Wikidata `wdt:P166` data exist for awards? | Carried into Deferred as "Wikidata SPARQL lead for Hart/Norris/Vezina automation" | Correctly carried |
| 3 | Has NHL signaled any intent about the API's long-term status? (nothing found) | Not mentioned in Deferred | Dropped, but immaterial — nothing was found, so there is nothing actionable to defer |

## Gaps found

### Gap 1 — AD-17's "schema-validation failure" doesn't specify a validation mechanism strict enough to catch the 2023-style failure mode

The research is explicit that the 2023 migration didn't just produce clean
errors: "some data fields were **permanently lost** in the transition"
(research.md line 42) — i.e., partial, silent schema drift, not only
hard failures. AD-17's rule is:

> "on any failure, transport error, or **schema-validation failure**, the run
> writes nothing to `internal/store`"

This presumes schema drift reliably surfaces *as* a validation failure. But
nothing in AD-17, the Stack table, or the Structural Seed specifies *how*
`internal/nhlclient` validates response shape. Go's default
`encoding/json` unmarshaling into a loosely-typed struct is tolerant by
default: renamed fields silently zero out, removed fields are silently
ignored, and unexpected new shapes don't error unless the code explicitly
uses `DisallowUnknownFields`, checks for required-field presence, or
validates types/ranges post-decode. Without that explicit strictness being
named as a rule (not just left as a §4.6/FR-16 implementation detail), a
future implementer could satisfy the letter of AD-17 with lenient decoding
that never trips the "schema-validation failure" branch at all — reproducing
exactly the silent-data-loss failure mode the research documented, rather
than the loud, safe one AD-17 intends. This is the sharpest place where the
2023 precedent (Recommendation 3) is invoked by name in the research but
under-specified in the architecture: the *policy* (write-nothing-on-failure)
is there; the *detection mechanism* it depends on is not.

**Suggested fix:** either add a short clause to AD-17 (or a new AD) requiring
`internal/nhlclient` to decode defensively — reject unknown/missing
required fields, validate types — so that a shape change is guaranteed to
surface as a "schema-validation failure" rather than silently degrading; or,
if this is intentionally left to implementation, add it to Deferred
alongside the existing FR-16 backoff-parameters item so it isn't lost.

### Gap 2 — ERD's "manually entered" label on AWARDFINALIST doesn't scope to only Hart/Norris/Vezina, leaving Art Ross/Rocket Richard's data path unstated

The research's sharpest positive finding is that Art Ross and Rocket
Richard are *not* voted and therefore need no manual step at all — they are
mechanically the season's points/goals leader, fully derivable from the
already-confirmed-live stats-leaders endpoint (research.md lines 48, 54).
The spine's source tree correctly scopes manual entry to only three named
trophies ("manual Hart/Norris/Vezina finalist entry", line 288). But the
Core-entity ERD's edge label is unscoped:

> `SEASON ||--o{ AWARDFINALIST : "has manually entered finalists for"`

If Art Ross/Rocket Richard predictions are also of `Prediction.kind =
"award finalist pick"` (the only listed kind that plausibly fits an
individual-player award), this ERD line reads as if *all* AwardFinalist
rows — including Art Ross/Rocket Richard — are manually entered, which
would mean the architecture never actually uses the one endpoint the
research verified live and free for those two trophies. No entity in the
Core-entity ERD (no `PlayerStats`/`StatsLeader`) shows an alternate,
sync-populated path for Art Ross/Rocket Richard's "finalist" (i.e., the
current points/goals leader). This may simply be inherited PRD terminology
this spine review didn't have in scope (the PRD itself wasn't re-read here),
but as written, the spine document alone doesn't make clear how/whether
Art Ross and Rocket Richard AwardFinalist rows get populated automatically
from `internal/sync`, rather than falling into the same manual-entry path
as the three voted awards.

**Suggested fix:** either scope the ERD edge label explicitly ("has
manually entered Hart/Norris/Vezina finalists for") and add an explicit
sync-populated path/entity for Art Ross/Rocket Richard, or confirm against
the PRD that Art Ross/Rocket Richard are out of scope for `AWARDFINALIST`
entirely and modeled some other way — and reflect whichever is true in the
ERD.

### Gap 3 (minor) — no rate-limit-informed default surfaces at the architecture level

The research found no published rate limit and only informal community
guidance ("cache your requests," defensive 429 handling in at least one
client). The spine correctly defers "exact backoff/retry parameters for
FR-16" — but that Deferred item is scoped to retry behavior *within one
sync run*, not to the choice of `SYNC_INTERVAL` itself (the polling
cadence). Nothing in the spine connects the research's "no official rate
limit, don't hammer it" finding to guidance on a conservative default
polling interval. Likely a genuinely minor, implementation-level point
(and `SYNC_INTERVAL` may already be fixed by the PRD's FR-14), but worth a
one-line note in Deferred for completeness since it's the most direct
architectural lever against the rate-limit risk the research flagged.

## What is correctly and solidly addressed

- The core "eyes open, not resolved" framing for both the ToS ambiguity and
  the 2023-precedent risk is present and worded consistently with the
  research's own framing (accept the legal risk; engineer for the technical
  risk via write-nothing-on-failure).
- Manual-entry scope for the three voted awards (Hart/Norris/Vezina) is
  named precisely, matching Recommendation 2 exactly — no over- or
  under-scoping in the source tree/capability map.
- Both actionable Open Questions (ToS subdomain scope, Wikidata SPARQL lead)
  are carried into Deferred with enough fidelity to act on later.
- AD-17's "write nothing, retry independently next run" pattern is a sound,
  correctly-motivated response to an unofficial, ToS-less, no-SLA data
  source — the *policy* layer of Recommendation 3 is well done; only its
  *detection* layer (Gap 1) is under-specified.

## Bottom line

No blocking contradictions between the research and the spine. The spine's
design is directionally correct and, in the case of Hart/Norris/Vezina
scoping, precisely matches the research's conclusion. The two real gaps
worth closing before implementation are (1) AD-17 needs to name the
decoding-strictness mechanism that makes "schema-validation failure" a
reliable signal rather than an aspirational label, and (2) the ERD's
AwardFinalist provenance needs to be scoped or clarified so Art Ross/Rocket
Richard don't silently fall back to a manual path the research explicitly
found unnecessary for them.
