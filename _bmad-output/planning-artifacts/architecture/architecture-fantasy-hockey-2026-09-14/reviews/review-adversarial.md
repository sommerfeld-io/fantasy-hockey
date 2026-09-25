---
title: 'Adversarial Review — Architecture Spine'
type: review
target: architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md
verdict: FAIL — not tight enough for two independent builders to converge
created: '2026-09-14'
---

# Adversarial Review — Fantasy Hockey Architecture Spine

**Method:** for each finding below, I construct two units one level down (two developers, or
two AI coding agents, each independently implementing a different epic/story) who each read the
spine, follow every AD to the letter, and produce code that is individually spine-compliant —
yet incompatible with the other unit's output. Every finding states: the two units, what each
correctly does per the spine, and exactly where the divergence happens. None of these are
hypothetical stretch scenarios; every one is triggered by an ambiguity the spine's own text
leaves open, several are triggered by a contradiction inside the spine's own examples.

**Verdict: FAIL.** The spine is well-drafted prose and reads as internally consistent on a linear
read-through, but it does not survive parallel, independent implementation. Eight concrete
incompatibility classes are documented below, two of them (#1 scoring/standings, #4 NHL Player
identity) are severe enough that shipping without resolving them reproduces the exact scoring-
integrity bug this rebuild exists to fix.

**Finding count: 8** (plus one supporting fact: AD-21's depguard rule does not currently exist in
the repo, despite being marked `[ADOPTED]`).

---

## Finding 1 — `internal/scoring` and `internal/standings` cannot legally talk to each other, but FR-23 requires standings to consume scoring's output

**Severity: Critical.**

**The two units.** Agent A builds FR-24 (Automatic scoring) in `internal/scoring`, per the
Capability Map row `Scoring & Leaderboard (FR-23..FR-24) | internal/scoring, internal/standings`.
Agent B builds FR-23 (Leaderboard) in `internal/standings`, from the same row, same governing ADs.

**What each does, correctly, per the spine.** Agent A implements the FR-24 point table (award
finalist 5pts, division winner 15pts, series 15/25 replacing-not-adding, etc.) entirely inside
`internal/scoring`, exporting whatever functions/types it needs — this is exactly what AD-15
prescribes ("`internal/scoring` and `internal/standings` compute points on every read directly
from `internal/store`'s in-memory ... data"). Agent B implements FR-23's Regular/Playoff/Total
columns inside `internal/standings`, reading `internal/store` directly, per the same AD-15
sentence and per the dependency-direction rule stated twice in the spine: "Feature packages never
import `internal/web` or each other directly" (Design Paradigm prose) and AD-8's Rule ("feature/
domain packages depend on `internal/store`; ... `internal/store` depends on nothing above it").
The Mermaid dependency graph confirms this literally — it draws `standings --> store` and
`scoring --> store`, with **no edge between `scoring` and `standings`** anywhere in the diagram.

**Where it diverges.** FR-23's Total column is definitionally the sum of the exact point values
FR-24 computes ("the app computes results and scores automatically ... Leaderboard ... always
reflects the latest scoring computation live"). Agent B is contractually forbidden from importing
Agent A's package (feature packages never import each other), and there is no third channel for
scoring's output to reach standings — `internal/store` cannot carry it either, because AD-15
explicitly forbids persisting or caching a score value anywhere ("no score value is ever written
to `fantasy-hockey.yml`," and by extension nowhere in store's in-memory structure either, since
AD-27 says the in-memory structure mirrors the file). Agent B's only spine-compliant path is to
**re-derive the entire point-calculation logic independently**, straight from raw predictions/
results, inside `internal/standings`. Now the point table from FR-24 (award=5, division
winner=15, Round 1 series=15/25, etc.) exists in two independently-written implementations that
the architecture forbids from ever being unified or cross-checked at compile time. The moment one
of them is patched (say, the scoring calibration in FR-24 is revised after season 1, per the
PRD's own §9 Assumptions Index caveat that these values "may be revisited") and the other isn't,
the Leaderboard total silently stops matching what any downstream feature (a future per-set score
display, an audit tool, a unit test written against `internal/scoring`) reports — reintroducing,
inside the architecture itself, precisely the "stale/miscalculated-score failure mode this
rebuild exists to eliminate" that AD-15's own "Prevents" line names.

**What's missing.** Either an explicit sanctioned exception to "feature packages never import
each other" for the `standings → scoring` edge specifically (updating AD-8's Rule text and the
Mermaid diagram to draw it), or a redefinition of `internal/standings` as a thin summation layer
that receives already-scored `Prediction`/`Result` pairs from `internal/web` (which is allowed to
import both), with `internal/web` doing the glue — but that then needs its own AD, since AD-10
currently scopes `internal/web` to "HTTP handlers, `html/template` views," not score aggregation.
Either way, this is not resolvable by reading the current spine twice; two builders will diverge
exactly as described.

---

## Finding 2 — `internal/mailer`'s env-var contract has no stated default/required policy, and it collides with the mandatory `task go:run` verification workflow

**Severity: High.**

**The two units.** Agent A wires `internal/mailer`'s startup config to fail fast if `SMTP_HOST`/
`SMTP_PORT` are unset, mirroring AD-22's `SESSION_SECRET` pattern ("read from a required env var
at startup; the process fails to start if it's unset — never generated in-process"). Agent B
wires the same config with an in-code default (e.g. `localhost:1025`, mirroring the mailpit
compose values) when the env vars are absent.

**What each does, correctly, per the spine.** AD-12's Rule says only: "SMTP host, port, username,
and password are all read from env vars ... at startup — never hardcoded." It gives an explicit
adjacent example of the required-and-fail-fast pattern one AD number earlier (AD-22,
`SESSION_SECRET`) but does not say whether AD-12 follows that same discipline or not — nothing in
AD-12's text says "required" or "fails to start if unset," unlike AD-22's explicit "the process
fails to start if it's unset." Both readings are literally consistent with AD-12 as written.

**Where it diverges.** This is not a cosmetic difference. Agent A's build breaks local, non-
compose development outright: the project's own mandatory Verification Workflow (project
`CLAUDE.md`) requires running `task go:run` — plain binary, not `docker-compose up` — after every
change to confirm the app starts. `task go:run` does not go through the docker-compose file that
sets `SMTP_HOST=mailpit`; if Agent A's fail-fast policy ships, `task go:run` now fails for every
contributor who hasn't manually exported four SMTP env vars first, silently breaking the
project's own required verification step. Agent B's default-to-mailpit-address policy avoids
that, but creates the opposite risk in production: if the prod deploy ever omits `SMTP_HOST` (a
real, plausible ops mistake — nothing in AD-12 makes it a *required* var the way AD-22 makes
`SESSION_SECRET` required), the app silently falls back to a `localhost:1025` mailpit-shaped
default that doesn't exist in production, and login-code email (FR-1, a hard MVP requirement per
PRD §8 Open Question 1) fails quietly instead of refusing to start — exactly the "hardcoded ...
that can't be pointed at a local dev capture server" failure AD-12 was written to prevent, just
inverted: now dev works but prod's misconfiguration is silent instead of loud.

**Compounding sub-issue — auth vs. no-auth SMTP.** AD-12 also doesn't state the convention for
empty `SMTP_USERNAME`/`SMTP_APP_PASSWORD` (which the docker-compose shape deliberately leaves
blank for mailpit). Does `internal/mailer` skip `smtp.Auth` entirely when username is empty, or
always construct `smtp.PlainAuth(...)`? Two builders will guess differently; nothing in AD-12 or
AD-13 states the rule. AD-13 only pins the *production* App-Password-as-secret requirement, not
the local no-auth branch.

**What's missing.** AD-12 needs an explicit statement of which vars are required-and-fail-fast
(if any) vs. defaulted, what the default is, and the no-auth-when-empty convention — and if any
var is made required, the Verification Workflow / `task go:run` local dev story needs to be
reconciled with it (e.g., a documented local `.env` a contributor must source, or `task go:run`
itself gaining an implicit local SMTP default that AD-12 currently forbids by saying "never
hardcoded").

---

## Finding 3 — Prediction row granularity is never pinned: one row per atomic pick, or one row per whole set?

**Severity: Critical.**

**The two units.** Agent A implements FR-15/FR-16 (Division picks — playoff teams and division
winners) inside `internal/predictions`. Agent B implements FR-19 (per-round series predictions)
in the same package, in parallel, on a different story/branch.

**What each does, correctly, per the spine.** AD-24 pins only that `internal/store` owns and
exports the canonical `store.Prediction` struct, matching the YAML shape, with `yaml` tags — it
says nothing about cardinality. AD-17 pins only that runtime-created Prediction rows get a UUID.
FR-9's second consequence is the only real signal in either document, and it cuts toward
fine-grained rows: "Any subset of a set's fields can be saved independently — there is no
all-or-nothing submission, except a Series pick's winner and game count, which save together as
one unit." A builder reading this literally implements one `Prediction` row per independently-
saveable field (Agent A: one row for "Cup champion," one for "Presidents' Trophy," one row per
Division's playoff-team list, one row per Division winner — each individually addressable and
re-saveable, matching "any subset ... independently"). But nothing forbids the opposite reading:
Agent B, building series predictions where FR-19 explicitly says winner+games "save together as
one unit," reasonably generalizes that a whole *Prediction set* is naturally one row with an
internal blob of fields (a `Predictions[playerID][setKey]` shape), since AD-9's illustrative
schema shows exactly one flat `predictions:` list with a `kind` discriminator field per entry and
never shows more than one row per `(player, kind)` in its (admittedly-disclaimed) example.

**Where it diverges.** These two cardinality models produce genuinely different `store.Prediction`
struct shapes and different YAML row counts for the same submitted form. If Agent A's fine-
grained, per-field-row model ships for before-the-season predictions while Agent B's one-row-per-
set model ships for playoff predictions (both are "Prediction" rows sharing the *same* exported
struct per AD-24 — there's only one `store.Prediction` type, not two), the struct has to satisfy
both shapes at once or one of the two features has to be rewritten late. Concretely: does
`store.Prediction` need one `Value string` + `Values []string` pair of optional fields (fine-
grained model, cheap per-row, awkward "kind of field is this" typing) or a `Payload` blob keyed
by `Kind` with kind-specific sub-fields (coarse-grained model, harder to make "any subset ...
independently" true without extra bookkeeping of which sub-fields are "filled")? This is the PRD's
own Open Question 3 ("`fantasy-hockey.yml` schema ownership ... deliberately deferred") reaching
all the way into the architecture spine unresolved — the spine's illustrative schema explicitly
disclaims itself ("Not a final schema ... an implementation-time decision") in exactly the place
where AD-24 needed to actually pin this to prevent two builders from diverging.

**What's missing.** A row-granularity rule: either "one `Prediction` row per independently-
saveable pick" (with the Series winner+games exception explicitly called out as the one
multi-field row, matching FR-9's wording) stated as its own AD, or a canonical `Payload`
sub-struct-per-kind design shown in the illustrative schema instead of disclaimed away.

---

## Finding 4 — NHL Player has no ID strategy at all, and the spine's own two examples contradict each other on name-vs-slug identity

**Severity: Critical — this is the exact "name-typo scoring gap" the rebuild exists to fix.**

**The two units.** Agent A implements FR-17 (Player awards autocomplete + save) in
`internal/predictions` together with AD-19's embedded-JSON widget. Agent B implements FR-24's
award-scoring rule ("Award finalist, 5 per name found anywhere in the actual top-3") in
`internal/scoring`, reading the hand-maintained `award_finalists` section per AD-23.

**What each does, correctly, per the spine.** AD-17's entity list is explicit and exhaustive-
looking: "Player, Team, Deadline, Result, AwardFinalist, playoff matchup" get human-readable
keys; "Prediction, LoginCode" get UUIDs. **NHL Player is never listed as an entity type in AD-17
at all**, despite being its own Glossary term, explicitly distinct from "Player," and central to
FR-17/FR-33. Agent A, needing *some* stable value to put in the hidden autocomplete form field and
persist as the predicted finalist, reasonably extends AD-17's own slug convention by analogy
("Player" gets `basti`, "Team" gets `TOR`) and mints NHL Player slugs (`mcdavid-connor`),
consistent with AD-19's `{"id": "...", "label": "..."}` embedded-JSON shape requiring *some*
non-display `id`. Agent B, building the scoring read path against the illustrative schema exactly
as drawn in the spine's own Structural Seed section, sees `award_finalists: hart: ["...", "...",
"..."]` — a bare list of strings with no `id` field shown anywhere — and reasonably implements
exact-string equality against the human-maintained display name, because that's the only shape
the spine actually shows for this section.

**Where it diverges.** If Agent A's predictions are persisted keyed by slug id (`mcdavid-connor`)
and Agent B's scoring compares against human-typed display strings in `award_finalists`
(`"Connor McDavid"`, or a maintainer's slightly different casing/spacing), the two never match. A
correct pick scores zero — silently, with no error, no lint failure, nothing in `internal/store`
noticing the mismatch, because both sides are individually well-typed `string` values, just
drawn from different vocabularies. This is not a contrived edge case: it is the literal, named
failure mode from the PRD's own Vision section ("a real, uncorrected scoring error in a past
season") and SM-C1 ("name-typo scoring gaps," FR-17). AD-24's own parenthetical actually assumes a
struct exists here — "typed constants for every cross-cutting enumeration (e.g. `Prediction.Kind`,
`AwardFinalist.Award`)" reads `AwardFinalist.Award` as a struct field access — but the
illustrative schema right below it shows no such struct, just a bare `map[award][]string`. The
spine contradicts itself between its own AD-24 parenthetical and its own Structural Seed example
on whether `AwardFinalist` is a struct at all.

**What's missing.** AD-17 needs an explicit NHL Player ID rule (most consistent with the rest of
the AD: a human-readable slug, since the season's NHL Player list is itself hand-maintained per
FR-33/AD-23, same category as Player/Team), and the illustrative `award_finalists` shape needs to
show that id being used consistently in both the human-maintained result section and the
app-written `predictions` section — plus AD-24 needs to state definitively whether
`store.AwardFinalist` is a real struct or just an enum.

---

## Finding 5 — AD-19 mandates one shared autocomplete widget script but never specifies whether it submits `id` or `label`

**Severity: High — compounds Finding 4.**

**The two units.** Agent A wires the widget for FR-15/16 team-pick fields (playoff field, division
winners). Agent B wires the same shared widget for FR-17's NHL Player award-finalist fields.

**What each does, correctly, per the spine.** AD-19 requires "the same shared widget script" for
every embedding site and the identical `{"id": ..., "label": ...}` object shape for the embedded
candidate JSON — but it only specifies the *display* data shape, not what the widget writes into
the form field it drives on submit. Agent A, needing the persisted value to match `results.
team_marks`'s abbreviation-keyed convention (`TOR`, per AD-17), wires the widget to submit the
selected option's `id`. Agent B, working from the illustrative `award_finalists` bare-string
example (see Finding 4) and with no NHL Player `id` convention established anywhere (per Finding
4), wires the *same shared script* to submit the selected option's `label` (display text) instead,
since that's the only value his server-side validation ("rejects a name that doesn't match," FR-17)
has anything to compare against.

**Where it diverges.** AD-19 mandates exactly *one* shared script, not one-per-site — so whichever
convention lands first in the shared script silently breaks the other builder's already-written
server-side handler: if the script submits `id`, Agent B's name-matching validation logic has
nothing to match against (no `id` exists for NHL Players per Finding 4); if it submits `label`,
Agent A's team-pick persistence now stores full display names where the store's illustrative
schema and AD-17 both expect compact abbreviation keys, breaking every downstream reader
(`results.team_marks`, `internal/scoring`'s cross-referencing of a Prediction's team pick against
`results`) that assumes the abbreviation form.

**What's missing.** AD-19 needs one added sentence pinning what the shared widget submits (`id`,
always) — which is only actually resolvable once Finding 4 (NHL Player identity) is fixed, since
without an NHL Player `id`, "submit `id` always" has nothing to submit for award-finalist fields.

---

## Finding 6 — `internal/server` vs. `internal/web`: "no business logic" scope is ambiguous for cross-cutting session middleware

**Severity: Medium-High.**

**The two units.** Agent A implements AD-11's cookie-renewal requirement ("every authenticated
request re-issues the cookie") as middleware registered inside `internal/web`'s `NewServer()`,
wrapping only the authenticated routes. Agent B implements the same requirement as middleware
wrapping the `http.Handler` passed into `internal/server.Run(ctx, port, handler)`, i.e. inside
`internal/server`.

**What each does, correctly, per the spine.** AD-2's Rule text is scoped narrowly and literally to
one file: "all application logic lives in `internal/` packages; `main.go` only resolves config,
opens the data file, and wires dependencies — no business logic, no mode dispatch." Read strictly,
AD-2 restricts only `main.go`; it says nothing about `internal/server`. But the Design Paradigm
table's Orchestration row bundles them together: "`main.go` + `internal/server` (wiring, HTTP
bootstrap; no business logic)" — a broader claim that is *not* repeated as its own numbered AD and
is not enforced by AD-21's depguard (which encodes AD-8's import-edge rules, not a "business logic
must not appear in file X" rule — that's not an import-boundary concern at all, and depguard
cannot express it). Agent A takes AD-2's literal text as controlling (only `main.go` is
restricted) and reasons that `internal/server.Run()` already structurally wraps the handler
(`srv := &http.Server{Handler: handler, ...}`) — the natural place to add cross-cutting request
middleware — is fair game. Agent B takes the Design Paradigm table's broader framing as
controlling and keeps all session logic inside `internal/web`, which is allowed to import
`internal/auth` per the dependency graph (`web --> auth`), whereas `internal/server` importing
`internal/auth` would need to cross from the Orchestration layer into the feature/domain layer —
a layer the Mermaid diagram never draws an edge for from `server`.

**Where it diverges.** If Agent A's middleware ships in `internal/server` (global — wraps every
route unconditionally, including the unauthenticated FR-1 login-code request page) and, on a
later feature branch, Agent B's independently-planned middleware also ships in `internal/web`
(route-scoped — wraps only authenticated routes), a future integration lands with cookie-renewal
logic running twice on every authenticated request from two different layers, or — worse — the
`internal/server`-level middleware (which cannot see which routes are "authenticated" without
importing routing knowledge that only `internal/web`'s `ServeMux` actually owns) ends up trying to
renew a cookie that doesn't exist yet on the public login page, producing spurious `Set-Cookie`
headers on unauthenticated responses. Nothing in the spine's two conflicting framings (AD-2's
literal scope vs. the table's broader scope) resolves which layer legitimately owns this, and
depguard — the spine's one enforceable mechanism (AD-21) — has no way to catch either version,
since both are syntactically valid import graphs.

**What's missing.** Either promote the Design Paradigm table's "`internal/server`: no business
logic" claim into its own numbered AD (so it's unambiguous and citable), or explicitly carve out
session-cookie-renewal middleware's home in AD-11/AD-2 by name.

---

## Finding 7 — FR-15's "capped live" selection directly contradicts AD-10's "JS only drives autocomplete," and the Capability Map doesn't resolve it

**Severity: Medium.**

**The two units.** Agent A, a server-rendered purist following AD-10's Rule to the letter ("the
only client-side JS is vanilla JS driving the autocomplete widget"), implements FR-15's Division/
Conference team caps ("a Division won't accept a 6th team; a Conference won't accept a 9th team")
as pure server-side validation with no client JS — checkboxes are plain HTML; hitting Save past
the cap triggers a full-page round trip that re-renders with FR-15's required "clear message
explaining what's missing." Agent B, trying to literally satisfy FR-15's first testable
consequence — "Selection is capped live" (present tense, no round trip implied) — adds a second,
functionally distinct block of vanilla JS in `internal/web/static` that disables the 6th checkbox
in a Division client-side, the moment 5 are already checked.

**Where it diverges.** Agent A's build technically satisfies AD-10 but arguably fails FR-15's own
consequence as literally worded (nothing about "won't accept a 6th team" reads as "...after you
click Save and the page reloads"). Agent B's build satisfies FR-15's live-cap language but
directly violates AD-10's "only ... autocomplete widget" restriction, which names *one* JS
responsibility and implies no others exist. The Capability Map row for this FR cluster
("Prediction sets & deadlines (FR-6..FR-12) | `internal/predictions`, `internal/web`" and the
separate "Before-the-season & Playoff predictions (FR-13..FR-21) | ... | AD-8, AD-10, AD-17, AD-19,
AD-24") cites AD-10 as one of the governing ADs for exactly the FR that AD-10's own text seems to
conflict with, without resolving the tension.

**What's missing.** AD-10 needs an explicit exception (or FR-15 needs an explicit spine note)
stating whether "capped live" is satisfied by a full-page re-render per attempted 6th selection,
or whether the minimal-JS stance has a second sanctioned client-side behavior beyond autocomplete.

---

## Finding 8 — `internal/store`'s external API contract (encapsulated methods vs. exported fields + caller-managed locking) is never pinned

**Severity: Medium-High — directly threatens AD-27's atomicity guarantee.**

**The two units.** Agent A (building `internal/scoring`, a read-only consumer) calls a
`store.Snapshot()`-style method that returns a defensive copy of the whole in-memory structure,
assuming `internal/store` fully encapsulates its mutex and exposes only method calls. Agent B
(building `internal/predictions`, which needs a read-check-then-write flow — confirm a deadline
hasn't passed, then save a pick) reaches for what AD-27's own Rule text literally describes —
"`internal/store` loads the whole file into memory once at startup. Every write updates *that*
in-memory structure under a mutex" — and, finding no exported "read-modify-write in one call"
method for their specific case, accesses the shared in-memory struct's exported fields directly
and manages the mutex itself around a manual read-then-write sequence, because nothing in AD-24 or
AD-27 states that the in-memory structure's fields, or the mutex itself, are *unexported*.

**Where it diverges.** AD-24 pins struct *shape* ("matching the YAML shape, with `yaml` struct
tags") — struct tags are meaningless on an unexported field for external callers, which mildly
implies exported fields are expected — but never states the *access pattern*. If `internal/store`
ships with some operations encapsulated (safe) and, under time pressure, gains one exported-field-
plus-manual-lock escape hatch for whatever Agent B's story needed that the initial API surface
didn't anticipate, the package now has an inconsistent contract: some callers go through the
mutex correctly via methods, one caller manages the mutex by hand, and a future refactor of the
struct's internal shape (entirely legal, since nothing pins it as frozen) silently breaks whichever
caller was reaching in directly. That's exactly the "crash or concurrent write leaving
`fantasy-hockey.yml` half-written or corrupted" failure AD-27 exists to prevent, reintroduced at
the Go-API level instead of the file level.

**What's missing.** AD-27 (or a new AD) should state explicitly that `internal/store`'s in-memory
structure and its mutex are never exported — only method calls are — so no feature package can
ever legally reach for manual locking, regardless of how inconvenient a missing query method makes
their story.

---

## Supporting fact — AD-21 is marked `[ADOPTED]` but its depguard rule does not exist in the repo today

`.golangci.yml` (repo root) currently enables only `errcheck, gosimple, govet, ineffassign,
staticcheck, unused, gofmt, goimports, revive, gocritic, errorlint, unconvert, misspell` — there is
no `depguard` entry, and no rules encoding AD-8's forbidden import edges. AD-21's own "Prevents"
line states the reason this matters: "the dependency-direction rule existing only as prose a
build can silently violate." Right now, that's exactly the state of the repo. This doesn't
independently produce a two-builder divergence, but it directly undercuts the enforceability this
review was asked to check for Finding 6 and the file-write boundary discussed under AD-9/AD-23:
today, *nothing* in the build pipeline would catch either divergence described above, or a stray
`os.WriteFile` against `fantasy-hockey.yml` from outside `internal/store` (see AD-9 and AD-23's
"Prevents" lines, which are enforced by prose only — depguard governs import edges between
internal packages, not raw filesystem calls, so even a fully-configured depguard per AD-21 would
not catch a package that imports stdlib `os` directly and touches the data file path resolved via
AD-25 without ever importing `internal/store` at all).

---

## Summary Table

| # | Two units                                          | Divergence                                                  | Severity |
|---|-----------------------------------------------------|---------------------------------------------------------------|----------|
| 1 | `internal/scoring` builder vs. `internal/standings` builder | Feature packages forbidden from importing each other, but Leaderboard requires scoring's output — forces duplicate, drift-prone scoring logic | Critical |
| 2 | Fail-fast SMTP config vs. defaulted SMTP config      | Breaks `task go:run` locally, or silently swallows a missing prod SMTP_HOST | High     |
| 3 | Fine-grained per-field `Prediction` rows vs. one-row-per-set | Two incompatible `store.Prediction` shapes, both AD-24-compliant | Critical |
| 4 | Slug-id NHL Players vs. name-string NHL Players      | Correct picks silently score zero — the exact bug this rebuild fixes | Critical |
| 5 | Widget submits `id` vs. widget submits `label`       | One shared script (AD-19-mandated) can't satisfy both builders' server-side assumptions | High     |
| 6 | Session middleware in `internal/server` vs. `internal/web` | Double cookie-renewal, or renewal on unauthenticated routes | Medium-High |
| 7 | Server-side-only 6th-team cap vs. extra client JS for the cap | AD-10 vs. FR-15's own "live" wording contradict each other | Medium   |
| 8 | Encapsulated `store` methods vs. exported fields + manual locking | Inconsistent API contract threatens AD-27's atomicity guarantee | Medium-High |

**Recommendation:** do not hand epics to independent builders/agents off this spine as-is. At
minimum, resolve Findings 1, 3, and 4 before any parallel implementation starts — those three
alone can each independently reproduce the scoring-integrity bug the whole rebuild exists to fix.
