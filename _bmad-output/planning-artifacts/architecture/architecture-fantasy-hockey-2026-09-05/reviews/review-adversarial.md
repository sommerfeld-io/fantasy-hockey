# Adversarial Review — Architecture Spine (Fantasy Hockey)

**Target:** `ARCHITECTURE-SPINE.md` (2026-09-05)
**Method:** for each finding, construct two hypothetical builders (developers or AI coding agents) who each satisfy every applicable AD to the letter, yet produce artifacts that do not interoperate when merged. A finding is only reported if a concrete incompatible pair can be shown — clashing field names/types, two owners of one write path, or a Rule satisfiable two structurally different ways.

**Verdict up front:** the spine is strong on layering and on the "no cache/no session table/no envelope" negative rules, but it repeatedly states *what must never happen* without fixing *the one shape that must happen instead* at exactly the seams where two independently-built units meet (the store's public API, three newly-added entities' discriminator/PK fields, and the two sibling tickers in `mode=sync`). 13 genuine divergence pairs found below.

---

## 1. `internal/store`'s API shape is unspecified — repository vs. thin SQL-passthrough

**AD(s):** AD-9, AD-10, AD-16
**The gap:** "internal/store owns all reads/writes... is the only package that opens a database connection" fixes *who holds the connection*, not *what the public API looks like*.

**Builder A (predictions team):** designs `internal/store` as an encapsulated repository: `store.Predictions.Create(ctx, predictions.Prediction) (uuid.UUID, error)`, `store.Predictions.ByParticipant(ctx, id) ([]predictions.Prediction, error)`. All SQL text lives inside `internal/store`; feature packages never see a query string. Domain structs (`predictions.Prediction`) are defined in the feature package; store scans into caller-supplied types via reflection/generics.

**Builder B (scoring team):** reads AD-9's parenthetical ("owns all reads/writes for ... Predictions, Results...") as meaning store *defines the canonical entity type* the way an ORM model layer would, and expects `store.Prediction` (a struct exported from `internal/store`) to be the thing every feature package imports and passes around — store as data-model owner, not just a data-access facade.

**Collision:** there is exactly one `Prediction` entity in the ERD but two irreconcilable ownership models for its Go representation. Builder A's store never exports a `Prediction` type at all (that would violate AD-9's "feature packages must never import `internal/web`"-style layering instinct extended by analogy — no, more directly: it has no reason to, since callers supply their own types). Builder B's scoring code that does `var p store.Prediction; store.Scan(rows, &p)` will not compile against Builder A's store package, and vice versa. Both builders can point to AD-9's text as license for their choice; neither is wrong by the letter.

**Fix needed:** a new AD fixing whether domain entity structs are defined in `internal/store` (shared-kernel style, importable by every feature package) or in each feature package (with store staying generic/DTO-based) — and, if the former, that store's exported structs are the single canonical representation every feature package must consume as-is.

---

## 2. `Prediction.kind` has no canonical shared enum location — and AD-9's dependency direction makes one structurally impossible without a new rule

**AD(s):** AD-9, ERD note on `Prediction.kind`
**The gap:** the ERD note says `Prediction.kind` (award finalist pick, team mark, series pick, Early Cup pick, Late Cup pick, Presidents' Trophy pick) "is a data value, not a structural split" and defers the literal values to the PRD Glossary. It never says *which Go package defines the constant/enum type* for those values.

**Builder A (scoring, FR-17–21):** writes SQL/Go comparisons against `kind = "award_finalist"` (snake_case, matching the PRD glossary phrase loosely).

**Builder B (standings, FR-22):** independently encodes the same six values as `"AwardFinalist"`, `"TeamMark"`, etc. (PascalCase, because that's the Go idiom for exported constants), since AD-9 forbids `internal/standings` from importing `internal/scoring` or vice versa — there is no compliant place for either builder to get the *other's* string values from.

**Collision:** `internal/predictions` (which actually writes the `kind` column) picks one of the two casings when inserting. Whichever of scoring/standings guessed wrong now silently never matches any row for half its `WHERE kind = ...` branches — a runtime correctness bug, not a compile error, because both packages only touch `kind` as a bare string with no shared type. AD-9's fixed dependency direction (feature packages never depend on each other, only on `internal/store`) means the *only* AD-compliant location for a shared enum is `internal/store` — but nothing states that store exports domain enums/constants at all. As written, three features (predictions writes, scoring reads, standings reads) share one column with zero shared-vocabulary guarantee.

**Fix needed:** an AD stating that cross-feature enumerations (starting with `Prediction.kind`) are defined once in `internal/store` (or a dedicated shared package) as typed constants, and that no feature package re-derives its own string literals for a value it doesn't own.

---

## 3. No transaction-composition rule — breaks AD-17's "write-nothing-on-failure" depending on store API choice

**AD(s):** AD-17, AD-9, AD-10
**The gap:** AD-17 requires that a sync run that fails partway "writes nothing to `internal/store`." A single sync run touches multiple entities (Team via FR-15 bootstrap, StatLeader via FR-16). Nothing specifies whether store exposes a way to compose multiple writes into one atomic unit.

**Builder A (store implementer):** exposes one method per entity, each opening and committing its own transaction internally: `store.UpsertTeam(ctx, Team) error`, `store.InsertStatLeaders(ctx, []StatLeader) error`. This fully satisfies AD-10 ("the only package that opens a database connection") and AD-9.

**Builder B (sync implementer):** writes the sync-run loop assuming AD-17's atomicity applies to the *whole run*: it calls `store.UpsertTeam` successfully, then `store.InsertStatLeaders` fails validation (a field-presence check fails per AD-17). Team is now durably committed even though the run "failed" — a partial write has occurred, directly contradicting AD-17's stated guarantee, even though Builder A's store and Builder B's sync package are each individually correct against the ADs they were each handed.

**Collision:** neither builder violates the AD text they read; the *combination* violates AD-17. There is no `store.WithTx(ctx, func(tx) error)`-style contract mandated anywhere, so whether AD-17 is actually satisfiable depends entirely on which of two AD-9/AD-10-compliant store shapes gets built first.

**Fix needed:** AD-17 (or a new AD) must require that `internal/store` expose a transaction-composition primitive and that `internal/sync` uses it to wrap an entire run's writes atomically.

---

## 4. `StatLeader`'s discriminator field (Art Ross vs. Rocket Richard) is unnamed and untyped

**AD(s):** AD-24, ERD
**The gap:** the ERD collapses two different stat leaderboards (points leader, goals leader) into one `STATLEADER` entity "precisely because" write-path is automated for both — but nowhere does the spine name the column that distinguishes which leaderboard a row belongs to, nor its value domain.

**Builder A (sync/nhlclient + store writer, FR-16):** adds a column `category string` with values `"points"` / `"goals"` (matching the underlying NHL stat names it pulls from the API).

**Builder B (scoring reader, FR-17, computing tie-expanded top-3 per AD-24):** writes its query assuming a column `award string` with values `"art_ross"` / `"rocket_richard"` (matching the trophy names used everywhere else in the ERD/PRD, since that's the vocabulary the rest of the entity relationships use — e.g. `AWARDFINALIST` uses trophy names, not stat names).

**Collision:** Builder B's `WHERE award = 'art_ross'` matches zero rows against Builder A's schema (no `award` column exists; the data is keyed by `category = 'points'`). This is discovered only at integration/runtime, not at compile time, since both sides independently satisfy AD-24's text ("fetches and persists a top-N stat-leader list... as raw ranked data") — AD-24 never names the discriminator.

**Fix needed:** AD-24 (or a companion data-dictionary) must name the exact column and its exact value set (e.g., `award enum('art_ross','rocket_richard')`).

---

## 5. `AwardFinalist`'s discriminator field (Hart vs. Norris vs. Vezina) has the identical gap

**AD(s):** AD-9 (ERD), FR-13
**The gap:** same structure as #4 — one entity covering three trophies, no named/typed discriminator anywhere in the spine.

**Builder A (`internal/awards`, manual entry writer, FR-13):** models it as `AwardFinalist{award Award}` where `Award` is a Go `iota`-based integer enum (`HartAward = 0`, `NorrisAward = 1`, `VezinaAward = 2`), persisted as a Postgres `smallint`.

**Builder B (`internal/predictions`/`internal/scoring` reader, joining `AWARDFINALIST ||--o{ PREDICTION`):** expects a human-readable `varchar` column (`"Hart"`, `"Norris"`, `"Vezina"`) because that's what needs to render on the Predictions/Standings pages (AD-11: server-rendered HTML, no client-side lookup table to translate an integer code back to a trophy name) and because AD-21's embedded-JSON autocomplete candidate list would need the same string values.



**Collision:** Builder A's `smallint` column requires a translation table the spine never mandates to become the display string Builder B needs; if Builder B queries expecting a string column and gets an integer (or vice versa), the join/read fails or silently mis-renders (e.g., `0` rendered instead of "Hart"). Both are "raw ranked/manual data" per the letter of the ERD note; neither violates a stated Rule.

**Fix needed:** same as #4 — pin the discriminator column name, storage type, and value set explicitly.

---

## 6. StatLeader's top-N depth is a free-floating, unshared constant

**AD(s):** AD-24
**The gap:** AD-24 explicitly says N is "an implementation constant, not fixed here," but also explicitly requires N to be "large enough to safely capture ties past 3rd place" for a *different* package's (`scoring`) tie-expansion logic to work.

**Builder A (`internal/nhlclient`/sync, writes the constant):** picks `N = 5` (comfortably captures "one or two players tied for 3rd," which was the only case the builder considered plausible).

**Builder B (`internal/scoring`, reads and tie-expands):** writes its tie-detection algorithm assuming the persisted list is deep enough to reveal *any* tie configuration, including a pathological 8-way tie for 2nd (statistically rare but not impossible in a 82-game season with rounding of counting stats) — an assumption AD-24's own text invites ("large enough... to safely capture ties past 3rd place" implies no fixed ceiling).

**Collision:** with N=5, a tie that extends to 6th place is silently truncated before scoring ever sees it — scoring computes an incomplete/wrong top-3-with-ties set, with no error, no failed test until that exact game-state occurs live. There is no shared constant, config value, or interface contract forcing the two builders' assumptions about "large enough" to match.

**Fix needed:** fix N as a named, shared constant (or config value with a stated minimum) that both `internal/nhlclient`/`internal/store` and `internal/scoring` reference from one place.

---

## 7. AD-19's ID-strategy rule doesn't cover 4 of the 8 entities `internal/store` owns

**AD(s):** AD-19, AD-9 (parenthetical entity list)
**The gap:** AD-19 fixes ID type for exactly four entities: Participant, Season, Prediction (UUID) and Team (NHL's own ID). AD-9's list of what `internal/store` owns has eight: Participants, Seasons, Teams, Predictions, Results, **LoginCodes, AwardFinalists, StatLeaders**. The three newly-added ERD entities plus `Result` are simply not mentioned by AD-19.

**Builder A (auth team, building LoginCode per AD-22):** gives `LoginCode` a UUID primary key, consistent-by-analogy with Participant/Season/Prediction.

**Builder B (store schema/migration owner, building the actual `golang-migrate` migration files):** gives `LoginCode` a Postgres `bigserial` primary key, reasoning that a high-churn, short-lived, append-only table (a fresh row per requested code, per AD-22) doesn't need UUID's randomness/scale properties and a serial is cheaper and simpler — and nothing in AD-19 forbids it, since AD-19 never mentions LoginCode.

**Collision:** Builder A's Go code does `loginCodeID := uuid.New()` and passes it to an insert statement; Builder B's actual schema has an auto-generated `bigserial` column that the application must NOT supply a value for — inserting a client-generated UUID into a `bigserial` column fails outright (type mismatch) or, if the column is typed to accept it, silently defeats the auto-increment/ordering the schema owner intended. Same ambiguity applies independently to `Result`, `AwardFinalist`, and `StatLeader` — none is covered by AD-19.

**Fix needed:** extend AD-19 (or add a new AD) to state the ID strategy for every entity `internal/store` owns, not just four of eight.

---

## 8. `LoginCode.code`: plaintext vs. hashed storage

**AD(s):** AD-22, AD-25 (adjacent security posture)
**The gap:** AD-22 names the column `code` and describes its lifecycle (issued, used-at nullable, 10-minute window) but never states whether the stored value is the literal code the participant receives by email or a hash of it.

**Builder A (issuance path, FR-1):** stores the raw 6-digit code in the `code` column verbatim — the column name and AD-22's plain description ("a `LoginCode` row per issued code... `code`") read as literally storing the code.

**Builder B (validation path, FR-2, or a security-conscious reviewer building the same feature as a second story):** treats a login credential at rest the way a password-reset token would be treated in any competently-designed auth system, and stores `sha256(code)` in the `code` column, comparing hashes on validation — nothing in AD-22 forbids this, and AD-25 (no-enumeration-leak, timing-safety concerns) primes exactly this kind of security instinct.

**Collision:** whichever GoDog acceptance test (AD-5) is written against the *other* assumption breaks: a test written against Builder A's scheme that reads the DB row back and string-compares it against the code captured from the test mailbox fails against Builder B's implementation (the DB never contains the plaintext code), and Builder B's own validation logic would reject every code if some other component (e.g. a debug/admin tool, or a differently-built story reusing the same table) was written assuming plaintext lookup (`SELECT ... WHERE code = ?` with the raw code).

**Fix needed:** AD-22 should state explicitly whether `code` is stored plaintext or hashed, and if hashed, the algorithm.

---

## 9. No ERD entity exists for "reminder already sent" — forces either an unauthorized schema addition or duplicate-send spam

**AD(s):** AD-23, AD-9 (closed entity list)
**The gap:** AD-23 says the reminder ticker "calls `internal/predictions` (owns the 'has this Participant completed this Deadline' check...)". The ticker fires repeatedly at "its own interval" (undefined, but necessarily more than once between now and a Deadline, or it isn't a ticker). Completion status alone doesn't tell you whether a reminder for *this* window was already sent. The ERD/AD-9 entity list (Participants, Seasons, Teams, Predictions, Results, LoginCodes, AwardFinalists, StatLeaders) has no reminder-log entity.

**Builder A (`internal/predictions`, implementing FR-10 faithfully):** needs to persist "reminder X sent to participant Y for deadline Z at time T" to avoid re-emailing every tick until the participant completes their prediction, and calls a new `store.MarkReminderSent(...)` / `store.HasReminderBeenSent(...)` it assumes it can add.

**Builder B (`internal/store` implementer, working strictly from AD-9's enumerated entity list and the ERD as the closed contract for what store persists):** implements exactly the eight named entities and rejects/never builds `MarkReminderSent`, since it isn't in the architecture — from Builder B's perspective, inventing a ninth persisted entity unilaterally is exactly the kind of scope creep AD-9's explicit list exists to prevent.

**Collision:** Builder A's code doesn't compile/link against Builder B's store (the method doesn't exist), or — if Builder A instead "complies" by not persisting anything — the reminder ticker sends a duplicate reminder email on every tick until the participant completes their prediction, an observable behavior regression (spam) that FR-10 almost certainly doesn't intend, yet nothing in the letter of AD-23 forbids it.

**Fix needed:** either add a `ReminderLog`-style entity to the ERD/AD-9's list explicitly, or specify that dedupe is computed from existing fields only (and state exactly how, e.g. "send only in the single tick whose fire-time first crosses the FR-10 threshold before the deadline").

---

## 10. No env var is provisioned for the reminder ticker's own interval

**AD(s):** AD-23, docker-compose additions (Structural Seed)
**The gap:** AD-23 says the reminder ticker runs on "its own interval." The Consistency Conventions table names `SYNC_INTERVAL` as a config env var. The docker-compose snippet in the spine itself lists the *exact* env vars passed to the `fantasy-hockey-sync` container: `DATABASE_URL`, `SYNC_INTERVAL` — nothing else. Both tickers are started by the same `main.go` in the same process/container (AD-23: "as siblings... in `mode=sync`").

**Builder A (main.go wiring, following the compose file literally as the source of truth for what env vars exist):** reads only `DATABASE_URL` and `SYNC_INTERVAL` from the environment; the reminder ticker's interval is a hardcoded Go constant (e.g. `1 * time.Hour`), since no env var is wired for it anywhere in the given deployment topology.

**Builder B (`internal/predictions` reminder-check implementer, following AD-4's DI convention — "side effects... are injected as function parameters," and the general pattern that every other interval-like config in this spine is an env var):** writes `main.go`'s wiring code (or expects it) to require a `REMINDER_INTERVAL` (or similarly-named) env var and calls `os.Getenv` with no fallback, erroring out if unset.

**Collision:** deployed via the docker-compose file exactly as specified in the spine's own Structural Seed section, Builder B's binary crashes on startup in `mode=sync` (missing required env var), while Builder A's binary runs but has a reminder cadence no operator can tune without a code change — the two "sibling ticker" implementations are incompatible with each other and at most one is compatible with the given compose file.

**Fix needed:** either add the reminder-interval env var to the docker-compose snippet and Consistency Conventions table, or explicitly state it's a hardcoded constant (and its value/source).

---

## 11. `mode=sync`'s two sibling tickers: no shared shutdown/cancellation contract

**AD(s):** AD-23, AD-17, AD-3, AD-4
**The gap:** "main.go starts two independent `time.Ticker` loops as siblings" fixes that there are two loops, not how the *process* as a whole starts, blocks, and stops. `docker stop` (or a Pi power event) sends SIGTERM to the container's PID 1.

**Builder A (sync-loop implementer):** builds `main.go`'s `sync` branch around `ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM); defer cancel()`, passes `ctx` into both the sync ticker and the reminder ticker goroutines, and blocks on a `sync.WaitGroup` so that a SIGTERM lets an in-flight sync/reminder cycle finish before the process exits — treating this as necessary to actually deliver AD-17's write-nothing-on-failure guarantee cleanly (an abrupt kill mid-transaction is exactly the kind of partial-write risk AD-17 exists to prevent).

**Builder B (reminder-ticker implementer, working from AD-23's text alone, which says only "independent... siblings" and never mentions signals):** writes the reminder ticker as a bare `go func() { for range time.Tick(interval) { ... } }()` with no context parameter at all (also arguably more literally satisfying AD-4's "injected as function parameters" minimalism — nothing to inject if there's no shutdown concept), and has `main.go` block forever on `select {}`.

**Collision:** when both are wired into the same `main.go` (as AD-23 mandates — one file, two siblings), Builder A's sync loop is cancellable and drains cleanly on SIGTERM while Builder B's reminder loop is not — a SIGTERM could kill the process mid-email-send with no defined behavior, and `main.go`'s single blocking strategy (`WaitGroup.Wait()` vs `select {}`) can only be one or the other, so whichever ticker was built against the *other* assumption is either never awaited (goroutine leak / silently doesn't block process exit) or never receives the cancellation it was coded to expect.

**Fix needed:** an AD (or extension of AD-23) specifying that both tickers in `mode=sync` share one `context.Context` derived from OS signal handling, and that `main.go` blocks on both via a single coordinated mechanism (e.g. `errgroup` or `WaitGroup`).

---

## 12. AD-25's visibility filter, if placed in a shared store query, silently breaks AD-16's scoring completeness

**AD(s):** AD-25, AD-16, AD-9
**The gap:** AD-25 requires hidden Predictions to be "omitted from the rendered data entirely, server-side, before any template render." AD-16 requires scoring/standings to "compute points on every read directly from `internal/store`'s Predictions[...] data" — implying scoring needs *every* prediction (hidden-from-other-participants ones still count toward that participant's own score). Neither AD says *where in the call chain* the hidden-filter lives.

**Builder A (predictions/web team, building FR-11/12):** implements the filter as a default inside `internal/store`'s single shared `GetPredictions(seasonID)` query — e.g. `WHERE NOT (hidden AND now() < reveal_time)` baked into the one method every caller uses — reasoning this is the simplest way to guarantee AD-25's "before any template render, never client-side" requirement is met *everywhere*, by construction, with no risk of a web handler forgetting to filter.

**Builder B (scoring/standings team, building FR-17–22 against AD-16):** calls that same shared `store.GetPredictions(seasonID)` method (the only one available, per AD-9's single store-owns-everything model) expecting the complete, unfiltered dataset needed to score every participant's picks, including ones currently hidden from *other* participants' view.

**Collision:** with Builder A's filter baked into the shared query, Builder B's scoring engine silently omits any currently-hidden prediction from a participant's live score/standings entry — a correctness bug affecting real point totals, discovered only when a participant notices their score is missing points for a still-hidden pick, not via any test failure (both AD-25 and AD-16's literal text are individually satisfied by Builder A's design; the violation only exists in the intersection).

**Fix needed:** an explicit rule that visibility filtering (AD-25) is applied only in the presentation/predictions-render path, never inside a store method also used for scoring/standings reads — i.e., store must expose (at minimum) two distinct read paths, or a `includeHidden bool` parameter, not one filtered-by-default method.

---

## 13. AD-21's embedded autocomplete JSON has no fixed field-name schema, despite implying one shared, reusable widget

**AD(s):** AD-21, AD-11
**The gap:** AD-11 refers to "the autocomplete widget" (singular) as the one piece of client-side JS in the whole app; AD-21 says candidate lists for Team and Player are "embedded as JSON directly in the rendered page" for that widget to filter. Neither AD fixes the JSON object's field names/shape.

**Builder A (FR-5, predictions entry — team/player autocomplete):** embeds `[{"id": "<uuid-or-nhl-id>", "name": "..."}]` and writes the vanilla JS widget to read `.id`/`.name`.

**Builder B (FR-13, manual award-finalist entry — player autocomplete, built as a separate story/feature by `internal/awards` + `internal/web`):** independently embeds `[{"playerId": "...", "fullName": "..."}]` for its own instance of "the same" widget, since AD-21 only constrains *that* the data is embedded JSON, not its keys.

**Collision:** if the widget is meant to be the single reusable script AD-11's phrasing implies, it can only bind to one of the two field-name conventions — whichever feature was built second either breaks the shared widget or forks it into a second, near-duplicate vanilla-JS file, quietly contradicting AD-11's minimal-dependency/single-widget framing without ever violating its literal Rule text.

**Fix needed:** fix the embedded candidate-list JSON schema (field names, at minimum `id`/`label` or equivalent) once, shared by every embedding site.

---

## Summary Table

| # | Divergence pair | Severity |
|---|---|---|
| 1 | `internal/store` API shape: repository vs. raw-SQL-passthrough | High — root cause, affects every feature package |
| 2 | `Prediction.kind` enum has no canonical, importable location under AD-9's dependency direction | High — structural, silent runtime mismatch |
| 3 | No transaction-composition rule; breaks AD-17 atomicity depending on store shape | High — data-integrity guarantee not actually enforceable |
| 4 | `StatLeader` discriminator column unnamed/untyped (Art Ross vs. Rocket Richard) | High — silent empty-result queries |
| 5 | `AwardFinalist` discriminator column unnamed/untyped (Hart/Norris/Vezina) | High — silent join/render failures |
| 6 | StatLeader top-N not a shared constant; sync vs. scoring assumptions can diverge | Medium — silent incorrect tie detection |
| 7 | AD-19 ID-strategy rule omits LoginCode/AwardFinalist/StatLeader/Result | Medium — schema/insert type mismatches |
| 8 | `LoginCode.code` plaintext vs. hashed | Medium — security-relevant, breaks validation/tests |
| 9 | No "reminder already sent" entity; forces unauthorized schema add or spam | High — either build fails to link or ships a spam bug |
| 10 | No env var provisioned for reminder-ticker interval in the given compose file | Medium — one builder's binary crashes on the given deployment |
| 11 | No shared shutdown/cancellation contract for the two `mode=sync` tickers | Medium — inconsistent SIGTERM behavior, risk to AD-17's guarantee |
| 12 | AD-25 visibility filter placed in a shared store query silently breaks AD-16 scoring completeness | High — real point-total correctness bug |
| 13 | AD-21 embedded autocomplete JSON schema unfixed across FR-5/FR-13 | Medium — widget fork or breakage |

**Overall:** the spine's negative rules (no cache, no session table, no envelope, no client-side filtering) are well-enforced, but every *shared write/read seam* between two independently-buildable units — the store's API shape, the three new entities' discriminator/PK fields, and the two `mode=sync` tickers' coordination — is specified only at the level of "this must exist" or "this must never happen," not "this is the one shape it takes." That is exactly the gap two AD-compliant builders will fill differently.
