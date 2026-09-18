---
title: 'Load Season''s Canonical NHL Player List'
type: 'feature'
created: '2026-09-17'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'db1767c76fb1233616cc2bec7d3729e253f0a7e3'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** There is no season-wide, position-scoped NHL Player list anywhere in the app yet — Story 2.6's award-finalist autocomplete (Hart/Art Ross/Rocket Richard need skaters, Norris needs defensemen, Vezina needs goalies) has nothing to validate names against or source suggestions from.

**Approach:** Add a hand-maintained `nhl_players:` section to `fantasy-hockey.yml` (mirroring `teams:`'s own out-of-band-maintained pattern), expose it from `internal/store` as `[]AwardFinalist{Slug, DisplayName, Position}` via a `Teams()`-shaped accessor plus a position-filtered one, and generalize the existing (still-unused) `teamOption`/`newTeamOptions` embed-shape helpers so both teams and NHL Players convert into the identical `{"id","label"}` JSON object AD-19 requires. No handler, template, or route changes — this story only loads and exposes the data; wiring it into the awards form is Story 2.6.

## Boundaries & Constraints

**Always:** `nhl_players:` is hand-maintained directly in `fantasy-hockey.yml` by whoever runs the pool; `internal/store` only ever reads it, never writes it (AD-23, matching `Team`/`PredictionSet`/`Player`). Each entry has a `slug` (e.g. `mcdavid-connor`, lastname-firstname, hand-picked once by the maintainer and never regenerated — AD-17) and a `display_name`; `slug` is the only value any downstream reader (a saved `Prediction`'s finalist pick, the embed's submitted `id`) ever compares on. `store.AwardFinalist` is a real struct, `{Slug, DisplayName, Position string}` (AD-24's "at minimum" baseline plus the `Position` field this story's own scoping requirement needs) — never a bare `[]string` of names. `Position` is one of exactly three plain string values matching the PRD's own wording: `"skater"`, `"defenseman"`, `"goalie"` (constants `store.PositionSkater`/`PositionDefenseman`/`PositionGoalie`). `Store.NHLPlayers()` mirrors `Teams()`'s exact read pattern (RLock, defensive copy, no write method); `Store.NHLPlayersByPosition(position string)` filters the same data by `Position` — this filtering method is what satisfies "scoped by position... never a bare list," not a trio of duplicated per-position methods. The existing `teamOption`/`newTeamOptions` (`web.go:229-245`, still uncalled from any handler) are generalized: `teamOption` renamed to `embedOption` (the shared `{"id","label"}` shape AD-19 names), and a new `newAwardFinalistOptions(finalists []store.AwardFinalist) []embedOption` sits alongside `newTeamOptions`, both producing `embedOption` - reserved for Story 2.6 to call, exactly like `newTeamOptions` itself has sat unused since Story 2.2.

**Never:** No in-app UI to add, edit, or remove NHL Players (FR-33: maintained only by hand-editing the file). No automatic sync with any external NHL data source. No new HTTP route, handler, or template change - the "page needing this data renders" clause in the AC is satisfied by proving a well-formed `nhl_players:` section doesn't break app startup or the existing Predict screen (mirrors `load-canonical-team-list.feature`'s own minimal smoke-test shape), not by building 2.6's autocomplete UI now. No validation of a hand-edited `Position` value against the three known constants - matches this codebase's own established convention of never validating hand-maintained data (Story 2.2/2.3's reviews both rejected suggestions to add such validation on identical grounds).

</frozen-after-approval>

## Code Map

- `src/internal/store/store.go:120-126` (`Team` struct + doc comment) — mirror its exact style for the new `AwardFinalist` struct and its own doc comment; place it near `Team`.
- `src/internal/store/store.go:128-135` (`document` struct) — add `NHLPlayers []AwardFinalist \`yaml:"nhl_players"\`` alongside the existing `Teams []Team \`yaml:"teams"\`` field.
- `src/internal/store/store.go:236-243` (`Store.Teams()`) — structural precedent (RLock, `make`+`copy`, return) for the new `Store.NHLPlayers()` and `Store.NHLPlayersByPosition(position string) []AwardFinalist`.
- `src/internal/web/web.go:229-245` (`teamOption`, `newTeamOptions`) — rename `teamOption` → `embedOption` (update its doc comment to describe both producers); add `newAwardFinalistOptions(finalists []store.AwardFinalist) []embedOption` right after `newTeamOptions`, identical shape/order-preserving pattern (`Slug`→`ID`, `DisplayName`→`Label`).
- `src/internal/web/web_test.go:578-606` (`TestNewTeamOptionsShouldConvertEveryTeamIntoTheSharedEmbedShape`, `TestNewTeamOptionsShouldReturnAnEmptySliceForAnEmptyInput`) — update the JSON-string assertions for the `teamOption`→`embedOption` rename (behavior unchanged); add the mirrored `TestNewAwardFinalistOptionsShould...` pair.
- `src/internal/store/store_test.go:276-345` (`TestTeamsShouldReturnTheSeededList`, `TestTeamsShouldReturnAnEmptyListWhenNoneAreSeeded`, `TestTeamsShouldReturnACopyThatCannotMutateTheStore`) — structural precedent for the new `NHLPlayers`/`NHLPlayersByPosition` tests (seeded list, empty list, defensive-copy, plus a by-position filter test with a mix of positions).
- `src/fantasy-hockey.yml` — add a `nhl_players:` section (new top-level key, after `teams:`) seeded with a small, real, recognizable starter sample across all three positions (the maintainer grows this over the season per FR-33; this story's job is the mechanism, not an exhaustive roster). **Never** name this key `players:` - that key already means the fantasy pool's own human players (`store.Player`), a completely different concept.
- `src/acceptance-tests/features/load-canonical-team-list.feature` + its steps file - structural precedent (temp-dir store, signed session cookie, "doesn't break startup/Predict" smoke assertion) for this story's own `load-canonical-nhl-player-list.feature` + steps.

## Tasks & Acceptance

**Execution:**
- [x] `src/acceptance-tests/features/load-canonical-nhl-player-list.feature` + steps -- smoke scenario mirroring `load-canonical-team-list.feature` -- must fail before implementation (BDD red)
- [x] `src/internal/store/store.go` -- `AwardFinalist` struct, `Position` consts, `document.NHLPlayers`, `Store.NHLPlayers()`, `Store.NHLPlayersByPosition` -- unit tests: seeded list, empty list, defensive copy, position filter (including a position with no matches)
- [x] `src/internal/web/web.go` -- rename `teamOption`→`embedOption`, add `newAwardFinalistOptions` -- unit tests: shared-shape JSON assertion, empty input
- [x] `src/fantasy-hockey.yml` -- add the seeded `nhl_players:` section

**Acceptance Criteria:**
- Given `fantasy-hockey.yml`'s `nhl_players:` section is well-formed, when the app starts and a player requests the Predict destination, then the request still succeeds (200) - loading the list never breaks startup or an unrelated page.
- Given the seeded list, when `Store.NHLPlayers()` is called, then every entry comes back as an `AwardFinalist{Slug, DisplayName, Position}`, never a bare name string.
- Given the seeded list spans all three positions, when `Store.NHLPlayersByPosition("goalie")` (or skater/defenseman) is called, then only that position's entries come back.
- Given a `Team` and an `AwardFinalist`, when each is converted via `newTeamOptions`/`newAwardFinalistOptions`, then both produce the identical `embedOption{"id","label"}` JSON shape.

### Review Findings

- [x] [Review][Patch] Seeded `nhl_players:` has only 2 defensemen and 2 goalies, but Story 2.6 (already shipped) requires 3 *distinct* finalists per award - Norris (defensemen) and Vezina (goalies) are currently impossible to complete in the real, running app [src/fantasy-hockey.yml]

**Rejected:**
- `false` — The Given step verifying the seed "holds a sample of skaters, defensemen, and goalies" is a no-op that never checks this [src/acceptance-tests/load_canonical_nhl_player_list_steps_test.go] — refuted: already litigated during this story's own build review — the seed's well-formedness is enforced by construction (`store.New` panics on a malformed seed before any step runs), matching `load-canonical-team-list.feature`'s own already-accepted pattern.
- `false` — No malformed/invalid `nhl_players:` entry scenario proves startup tolerates it [src/acceptance-tests/features/load-canonical-nhl-player-list.feature] — refuted: already litigated during this story's own build review — matches Story 2.2's established scope (a single well-formed-doesn't-break-startup smoke scenario only), and a missing YAML field wouldn't even error at parse time (`yaml.Unmarshal` just zero-values it).
- `false` — `Position` is compared as a raw string with no validation against the three constants [src/internal/store/store.go] — refuted: already litigated during this story's own build review — matches the established, already-accepted convention of never validating hand-maintained enum-like data (`Team.Conference`/`Division` have identical exposure).
- `false` — No uniqueness check exists for `Slug` [src/internal/store/store.go] — refuted: same established convention - a duplicate slug is a hand-edit mistake in hand-maintained data, not a code defect, matching the same class of already-accepted risk as `Team`/`PredictionSet` field typos.
- `false` — No `NHLPlayerBySlug`-style lookup accessor was added for Story 2.6 to use [src/internal/store/store.go] — refuted: Story 2.6 (already implemented and shipped on this branch) added its own equivalent (`nhlPlayerBySlug` in `internal/web/web.go`) - nothing is left unaddressed.
- `false` — `store.New()` given a malformed `nhl_players:` entry has no test proving a clear error [src/internal/store/store_test.go] — refuted: same grounds as the malformed-entry finding above.

## Implementation Notes

- Implemented via a fresh subagent; verified independently against the diff (read every file changed rather than trusting the report). All four Code Map items match exactly: `AwardFinalist{Slug, DisplayName, Position}` + `Position` consts, `Store.NHLPlayers()`/`NHLPlayersByPosition` mirroring `Teams()`'s pattern, `teamOption`→`embedOption` rename with `newAwardFinalistOptions` alongside `newTeamOptions`, and a small real-player seed (McDavid, MacKinnon, Kucherov, Hughes, Makar, Hellebuyck, Shesterkin) spanning all three positions.
- The implementer caught and fixed a real GoDog step-collision bug proactively: a first-draft step phrase collided globally with an identically-worded step already registered by `login_steps_test.go` (GoDog resolves ambiguous step text suite-wide, not per feature file, first-registered-wins) - reworded all three new step phrases to be unique, verified via grep against every other `ctx.Step` registration before finalizing.
- Verified: `task go:test` (new store/web lines at 100% coverage), `task go:test:acceptance`, `task go:run`, `task docker:build` (authoritative) - all green.
- No handler/template/route changes, no in-app UI, no `Position` validation - all per the spec's own "Never" boundaries.
- **Review (single loop):** three parallel review layers found two real, low-severity findings (a nil-vs-non-nil-empty-slice inconsistency between `NHLPlayers()`/`NHLPlayersByPosition()`; an overstated acceptance step/scenario name) and seven findings that all turned out to match this codebase's own already-established, sometimes previously-litigated conventions once checked directly against baseline code and prior stories' Review Triage Logs. Re-engaged the same implementation subagent with both fixes; verified independently (read the patched `store.go` directly, then ran `task go:test`/`task go:test:acceptance`/`task docker:build` myself) - all green.

## Spec Change Log

## Review Triage Log

- **low** — Blind Hunter + Edge Case Hunter (independently): `Store.NHLPlayers()` always returns a non-nil (possibly empty, via `make`) slice, while `Store.NHLPlayersByPosition()` returns `nil` when nothing matches (`var players []AwardFinalist`, never appended to). Verified real by reading both implementations directly - two sibling read methods on the same type now have inconsistent nil-vs-empty-slice semantics; a future caller that JSON-marshals the result directly (nil serializes as `null`, `[]` as empty) or does a nil-check would see the two methods behave differently. `newAwardFinalistOptions` happens to normalize this today (its own `make([]embedOption, len(finalists))` turns a nil input into a non-nil empty output), so no current caller is actually harmed, but the inconsistency itself is real and the fix is a one-line change. Route: patch.
- **low** — Blind Hunter: the acceptance step/scenario name "the NHL player list request succeeds with status 200" overstates what's tested - there is no NHL-player-list-specific endpoint; the step just re-requests the pre-existing generic `/predict` destination (mirroring `load-canonical-team-list.feature`'s own identical shape). Real naming-clarity gap, trivial rename. Route: patch.
- **false** — Edge Case Hunter: `NHLPlayersByPosition` doesn't validate `position` against the three known constants, silently returning an empty list for a typo'd/unknown value. Refuted: this matches the codebase's own already-established convention for a `kind`/`position`-style string parameter - `FindPrediction`/`SavePrediction`'s own `kind string` parameter is never validated against `KindCupChampion`/`KindPresidentsTrophy` either; callers are trusted to use the exported constants, exactly as here.
- **false** — Blind Hunter: nothing verifies the 7 real `nhl_players:` entries just added to `fantasy-hockey.yml` actually match the exact `skater`/`defenseman`/`goalie` strings, so a future hand-edit typo would silently and permanently exclude that player from position filtering with no signal. Refuted: matches this codebase's own established, already-litigated convention of never validating hand-maintained enum-like data (Story 2.2/2.3's Review Triage Logs both reject the identical class of finding on identical grounds) - `Team.Conference`/`Division` have exactly the same unvalidated-typo exposure today.
- **false** — Blind Hunter: the acceptance scenario's `Given` step is a no-op that never inspects the seed to confirm it truly "holds a sample of skaters, defensemen, and goalies." Refuted: the seed's well-formedness is enforced by construction, not by this step - `newLoadCanonicalNHLPlayerListStore` calls `store.New` (and panics on failure) when the scenario state is built, before any step runs, exactly mirroring `load-canonical-team-list.feature`'s own already-accepted no-op `Given` pattern.
- **false** — Blind Hunter: only a happy-path scenario exists; no scenario exercises a malformed/missing-field `nhl_players` entry to prove startup degrades gracefully. Refuted: matches Story 2.2's own established scope (a single well-formed-doesn't-break-startup smoke scenario, nothing more) and this spec's own frozen AC never asked for malformed-data handling; a merely-missing YAML field (e.g. no `position:` key) wouldn't even error at parse time - `yaml.Unmarshal` just zero-values the absent field - so there's no distinct "startup breaks" behavior to demonstrate here anyway.
- **false** — Blind Hunter: five new `store_test.go` tests each inline a near-identical multi-line seed literal instead of sharing one, unlike the acceptance-test file added in the same diff (which factors its seed into one const). Refuted: matches `store_test.go`'s own pre-existing, established local-seed-literal convention - the baseline `TestTeamsShouldReturnTheSeededList` (unchanged by this diff) inlines its own local seed exactly the same way; the acceptance-test file's shared-const style is that file's own separate, unrelated convention.
- **false** — Blind Hunter: `newAwardFinalistOptions` ships with no production caller, growing maintained surface ahead of the not-yet-built Story 2.6. Refuted: exactly mirrors `newTeamOptions`' own precedent from Story 2.2 (added unused, first consumed stories later) - the frozen spec's own Approach explicitly calls for this "generalize now, 2.6 consumes it later" design, matching an already-established, unchallenged pattern in this codebase.
- **false** — Blind Hunter: the new `AwardFinalist`/`Position*` doc comments cite forward-looking, unverifiable identifiers ("Story 2.6", "FR-33/AD-24") that could go stale if that story's shape changes. Refuted: matches this codebase's own pervasive, already-established comment convention of cross-referencing future story numbers and FR/AD ids everywhere (e.g. the pre-existing `newTeamOptions` comment itself says "2.4/2.6 embed it once they add their own dropdowns/chips") - not a defect this diff introduced.

## Design Notes

- `Position` stays a plain string constant (matching `Prediction.Kind`'s own convention), not a dedicated Go type with a `Validate()` method - this codebase never validates hand-maintained enum-like fields (Boundaries & Constraints), so a typed enum would only add ceremony with no enforcement benefit.
- The click-dummy `App.jsx`'s own `SKATERS`/`DEFENSEMEN`/`GOALIES` reference arrays are plain display-name strings with no slug and are not position-exclusive (a couple of names appear in more than one list) - not authoritative here; PRD FR-33/AD-17/AD-24 and epics.md's own AC override it with the real `{slug, display_name, position}` shape and position-exclusivity this story implements.
- `Store.NHLPlayersByPosition` takes the position as a plain `string` parameter rather than three separate `Skaters()`/`Defensemen()`/`Goalies()` methods - one parameterized accessor mirrors how the codebase already filters `Team` by a field (`groupTeamsByDivision`) rather than one method per division, and avoids tripling the surface for what is one shared filter operation.

## Verification

**Commands:**
- `task go:test` -- expected: new store/web unit tests pass
- `task go:test:acceptance` -- expected: the new feature file's scenario passes end-to-end
- `task go:run` -- expected: app builds and starts
- `task docker:build` -- authoritative: lint + test + container build all green

**Manual checks (if no CLI):**
- Confirm the Predict screen still renders normally after the `nhl_players:` section is added (nothing observable changes yet - this story is store-layer only).
