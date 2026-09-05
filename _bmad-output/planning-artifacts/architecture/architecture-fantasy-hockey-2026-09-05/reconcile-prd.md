---
title: 'PRD ↔ Architecture Spine Reconciliation'
status: draft
created: '2026-09-05'
sources:
    - _bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-04/prd.md
    - _bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-05/ARCHITECTURE-SPINE.md
---

# Reconciliation: PRD vs. Architecture Spine

Scope note: this review only flags gaps with real architectural weight (would affect a technical
decision, a data shape, a boundary, or an NFR). Missing product narrative, FR-wording restatement,
or UX copy in the spine is expected and not flagged.

## 1. Feature/FR coverage in the Capability → Architecture Map

All eight features (§4.1–4.8) and all 22 FRs are accounted for in the Capability → Architecture Map.
No feature or FR is entirely absent from the spine. The mapping is generally sound. Two FR↔AD
pairings, however, don't actually hold up technically on inspection:

### Gap 1 — No persistence design for login codes (FR-1, FR-2) [Real gap]

FR-2 requires: a code is valid until used once or 10 minutes elapse; requesting a new code does
**not** invalidate a prior one, so **multiple codes can be simultaneously valid** for the same
Participant, each independently tracked for use/expiry.

AD-12 ("Stateless signed-cookie sessions") is the only AD bound to §4.1, and it explicitly states
"no session table exists in `internal/store`." That rule is correct and sufficient for the
*session* (post-authentication) half of Authentication — but it says nothing about where the
*pre-authentication* login codes themselves live. Storing/validating a login code requires, at
minimum, a record per issued code (participant, code hash, issued-at, used flag) so that: (a)
multiple codes can be independently valid at once, (b) a code can be marked used after first
success, and (c) expiry can be checked per-code rather than globally.

This is missing everywhere in the spine's data layer:
- `internal/store`'s stated ownership list is "Participants, Seasons, Teams, Predictions, Results,
  AwardFinalists" — no login-code entity.
- The Core-entity ERD has no LoginCode node or relation.
- AD-19 (ID strategy) doesn't cover a login-code identifier/PK.

Without this, AD-12 reads as if it covers all of Authentication's persistence, but it only covers
sessions. This is a real data-shape gap, not a narrative one — the store's package contract and the
ERD need one more entity (or an explicit note that codes are ephemeral/in-memory, if that's the
intended design, which would itself have implications for the single-binary/mode topology since
`serve` may run as more than one instance behind a reverse proxy — see AD-15).

**Recommendation:** add a `LoginCode` (or similar) entity to the ERD and to `internal/store`'s
ownership list, and either fold it under AD-12 or add a new AD stating where/how codes are
persisted and invalidated.

### Gap 2 — No scheduling/process-topology mechanism for reminder emails (FR-10) [Real gap]

FR-10 requires a periodic check per Deadline ("has this Participant completed everything under
this Deadline yet") that fires two reminder emails at some point before closure. This needs an
active, periodically-running process — structurally the same kind of thing FR-14's Sync needs.

But the spine's process topology (AD-1) defines exactly two modes: `serve` and `sync`. AD-17 gives
`sync` its own `time.Ticker` loop, driven by `SYNC_INTERVAL`, but that loop is scoped by AD-17
explicitly to NHL data fetching via `internal/nhlclient` — nothing says reminder-checking runs
there too, and it would be an odd fit since it needs no NHL API access. §4.3 in the Capability map
is governed by AD-9 (layering), AD-13 (SMTP transport), AD-14 (App Password secret) — none of which
say *what triggers* a reminder check, or in which of the two modes (or a third mode) it runs.

This is a real boundary gap: it affects AD-1's mode-dispatch design (does a third mode become
necessary, or does `serve` also need a ticker goroutine?) and isn't just an implementation detail,
since AD-1 currently commits to exactly two modes as an explicit "prevents" rule ("publishing and
maintaining two separate Docker images ... one Go binary with a mode argument").

**Recommendation:** add an AD (or extend AD-1/AD-17) stating which mode owns reminder scheduling
and what drives it (e.g., a ticker inside `serve`, or folded into `sync`'s existing ticker even
though it doesn't touch `nhlclient`).

## 2. FR-technical-requirement vs. AD support checks

### AD-16 (live computation) vs. FR-9 (missing prediction scores 0) — supported, no contradiction

Since scoring is computed live by reading whatever Predictions/Results rows exist, an absent
Prediction naturally yields "no contribution" as long as the scoring engine treats absence as zero
rather than erroring on a missing row. No contradiction; this is a correct, if implicit, design
consequence of AD-16. No gap.

### AD-16 (live computation) vs. FR-17 tie-inclusive Award scoring — partially unsupported [Real gap]

(Note: the tie-inclusive matching rule is FR-17, "Award Scoring (Tie-Inclusive)," not FR-19, which
is Cup Picks/Presidents' Trophy — flagging this in case the cross-reference in the brief was
intended to point at FR-17.)

FR-17 requires: a tie at any position within the top 3 expands the counted set to *every* tied
name — e.g., a 3-way tie for 3rd extends the scoring set to 5 names total. Detecting this requires
knowing the actual stat value (points, goals, etc.) for players beyond the nominal top-3 cutoff,
since a name tied for 3rd necessarily has teammates in the data at the same value who may sit at
ranks 4, 5, etc. in a naive "top 3" fetch/list.

The spine's Core-entity ERD models `RESULT` generically ("PREDICTION ||--o| RESULT: scored
against") with no field-level detail, and AD-17 (sync) only commits to "fetch each needed resource
once per scheduled run" — it doesn't say the Art Ross/Rocket Richard leaderboard fetch must include
enough depth (or raw stat values, not just a rank-3 cutoff) to detect ties extending past position
3. This is understandably left at a lower level of detail than most other ADs, but it's the one
place where under-specifying the data shape could silently produce an implementation that can't
satisfy FR-17 at all (e.g., if `nhlclient` only ever pulls exactly 3 leaderboard rows). Worth at
least a one-line rule or note, since it affects both the external gateway's fetch contract and the
Result data shape.

### AD-12 vs. FR-3/FR-4 (session duration, logout) — supported, no gap

Cookie re-issuance on every authenticated request correctly implements the sliding 30-minute
timeout, and "no longer re-issued/cleared" correctly implements logout. Consistent.

## 3. Cross-Cutting NFRs

| PRD NFR                          | Reflected in spine?                                                                 |
|-----------------------------------|--------------------------------------------------------------------------------------|
| No partial writes                 | Yes — AD-17 (sync writes nothing on failure); trivially satisfied for scoring by AD-16 (nothing is ever written). |
| Reproducibility                   | Yes — AD-16 directly implements this (live compute, no hidden state, no persisted score). |
| No enumeration leaks              | **Not reflected anywhere** — see Gap 3 below. |
| Observability without in-app alerting | Yes — stated verbatim in the Consistency Conventions table and echoed in the Deferred section. |

### Gap 3 — "No enumeration leaks" NFR has no architectural counterpart [Real gap, moderate weight]

The PRD's cross-cutting NFR requires that no user-facing response (login, elsewhere) reveal
information about the fixed Participant list or other Participants' state beyond what §4.4
explicitly allows — notably, FR-1 requires the same generic response regardless of email match,
and FR-12 requires hidden predictions to be *absent*, not blank/masked (a distinction with response-
shape implications).

Nothing in the spine's ADs, Consistency Conventions, or Deferred section addresses this. It isn't
purely a UX/copy concern: satisfying it constrains the *technical* design of the auth endpoint
(e.g., constant-time / indistinguishable-latency handling regardless of whether an email matches,
since a timing side-channel would itself be an enumeration leak) and the predictions-visibility
query design (must not synthesize placeholder rows for hidden predictions). Given AD-12 is the only
AD touching the auth boundary, and it doesn't mention this, it's a legitimate small gap — likely a
single added bullet under AD-12 or the Consistency Conventions table would close it.

## 4. Constraints and Guardrails

| PRD constraint | Reflected in spine? |
|---|---|
| Cost (free data source only) | Implicitly satisfied by construction (AD-17/`nhlclient` targets the free API per `research.md`), not called out explicitly as a constraint anywhere in the spine. Low weight — no architectural decision hinges on it beyond what's already implied by using the free source; not flagged as a gap. |
| Deployment (Docker + docker-compose only) | Yes — AD-1, AD-7, the docker-compose section, and the deployment-topology diagram all reflect this thoroughly. |
| Legal (NHL ToS ambiguity) | Yes — carried into Deferred verbatim. |

## 5. Open Questions and Assumptions Index

| PRD item | Carried into spine? |
|---|---|
| Open Question 1 — reminder timing | Yes — Deferred item 1. |
| Open Question 2 — Wikidata SPARQL lead | Yes — Deferred item 2. |
| Open Question 3 — user-visible handling of broken NHL API | Yes — Deferred item 3. |
| Open Question 4 — backoff/retry bound | Yes — Deferred item 4. |
| Assumption 1 (FR-15) — Season's Team list never changes mid-Season | **Not explicitly carried** — see note below. |
| Assumption 2 (FR-13) — last-write-wins, no conflict handling on simultaneous award-finalist edits | Not explicitly carried, but not a gap (see note below). |

**Assumption 1 note (minor, not scored as a top gap):** the spine's ERD and AD-19 already
structurally encode a design consistent with this assumption — the Team list is bootstrapped once
per Season (`SEASON ||--o{ TEAM: "bootstraps (FR-15)"`) and Team's ID is stable/NHL-native
regardless of renames — so the architecture doesn't contradict or ignore the assumption, it just
doesn't restate it in the Deferred list the way the four Open Questions are restated. Worth a
one-line addition to Deferred for traceability, but there's no technical inconsistency.

**Assumption 2 note:** not a gap — FR-13's last-write-wins behavior is the natural behavior of an
unguarded `UPDATE` with no added optimistic-concurrency mechanism, which is exactly what AD-9/store
already implies by omission (no locking/versioning AD exists anywhere in the spine). No design
decision is needed to achieve this; it falls out of doing nothing extra.

## Summary of Real Gaps

1. **No login-code persistence design** (FR-1/FR-2) — AD-12 covers sessions only; no LoginCode
   entity in `internal/store`'s ownership list or the ERD.
2. **No process/trigger mechanism defined for FR-10 reminder emails** — AD-1's two-mode topology
   and AD-17's sync-scoped ticker don't account for a periodic reminder-check process.
3. **Tie-inclusive Award scoring (FR-17) needs leaderboard depth beyond "top 3"** in the Result
   data shape / `nhlclient` fetch contract — not specified, risks an implementation that can't
   detect ties past position 3.
4. **"No enumeration leaks" cross-cutting NFR has no architectural counterpart** — no AD addresses
   timing/response-shape at the auth or visibility boundary.
5. (Minor) PRD Assumption 1 (Team list frozen mid-Season) isn't explicitly carried into the
   Deferred list, though the ERD/AD-19 design is already consistent with it.
