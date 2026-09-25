---
title: 'Pre-Epic-3 hardening: Upcoming gate, roster helper, web.go split, shared acceptance fixtures'
type: 'refactor'
created: '2026-09-24'
status: 'done'
route: 'dispatch'
baseline_commit: 'd9be297d0a3004425ae7b7399e731b788d53bcb7'
review_loop_iteration: 0
context:
    - '{project-root}/src/CLAUDE.md'
    - '{project-root}/_bmad-output/implementation-artifacts/epic-2-retro-2026-09-18.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Epic 2 retro left four open action items that block Epic 3. #8 (F1): `Upcoming` is only enforced where the Predict list renders links, so anyone can open and submit an upcoming set by direct URL. #9 (F2): four roster-membership helpers do the same job in three different shapes. #10 (F3): `internal/web/web.go` has grown to 1698 lines and mixes routing with every sheet kind. #17 (F12): the acceptance-test fixture builders and the lazy `ensureReady` pattern are copied across step files.

**Approach:** Refactors come first and change no behavior. Split `web.go` into same-package files by concern (#10), then put roster membership behind one shared shape (#9), then pull the duplicated acceptance fixture code into one shared test helper (#17). Last, add a server-side Upcoming gate to `handleSheet` and `handleSheetSubmit` (#8), written TDD/BDD first. Story 3.2 will build its round unlocking on this gate (epics.md Story 3.2 reuses Story 2.1's Upcoming treatment).

## Boundaries & Constraints

**Always:**

- Refactor commits (#10, #9, #17) are pure moves or reshapes. The full unit and acceptance suites stay green without changing any assertion.
- Everything stays in `package web` / `package acceptance_test`. No new packages.
- The Upcoming check runs after the set is found and before any deadline or form handling, and applies to every set kind.
- Deciding by human (2026-09-24): the refactors (#9, #10, #17) need no new acceptance tests. #8 gets new Gherkin scenarios.
- The `.task` / `.vscode` submodule pointer changes stay uncommitted.
- Human decision (2026-09-24): The upcoming response is **404 Not Found** for both GET and POST, with the generic not-found body, the same as an unknown id.
- Human decision (2026-09-24): keep the full four-item spec despite exceeding the 1600-token guideline.

**Never:**

- Don't touch `Dockerfile` or `.github/workflows/**`.
- Don't work on the other open retro items (#11–16, #18), including the duplicated `setRowFragment`.
- Don't change how the Predict list renders Upcoming, and don't change the Closed or deadline behavior.
- Don't add a new unlock mechanism. `PredictionSet.Upcoming` is the gate.

## I/O & Edge-Case Matrix

| Scenario                     | Input / State                                   | Expected Output / Behavior          | Error Handling                  |
|------------------------------|-------------------------------------------------|-------------------------------------|---------------------------------|
| Open upcoming set            | `GET /predict/{id}`, set has `upcoming: true`   | 404, no sheet rendered              | Generic body, no set details    |
| Submit upcoming set          | `POST /predict/{id}`, pickable set, upcoming    | 404, nothing saved                  | Checked before parsing the form |
| Upcoming and past deadline   | upcoming set whose deadline has passed          | 404 (Upcoming wins over 403)        | Matches `predictStatus` order   |
| Non-upcoming set (unchanged) | open, closed, or unknown id                     | Current behavior (200 / 403 / 404)  | N/A                             |

</frozen-after-approval>

## Code Map

- `src/internal/web/web.go` (1698 lines). It holds these areas:
    - embeds and templates (lines 6–31)
    - consts (36–112)
    - Predict list (119–229)
    - embed options (231–268)
    - shell and routing (270–429)
    - sheet-kind registry and award tables (431–512)
    - divisions view (517–776)
    - awards view (779–1015)
    - generic sheet, cup and presidents (1017–1279)
    - divisions submit (1285–1486)
    - awards submit (1489–1593)
    - login and logout (1595–1680)
    - render helpers (1682–)
- `handleSheet` (web.go:1132) checks found, then runs `newSheetData`, then renders. It has no Upcoming or pickable check. `handleSheetSubmit` (web.go:1204) checks the pickable kind, found, the deadline parse, `!deadline.After(now)` (403), then `ParseForm`, then dispatches. `findPredictionSetByID` is at web.go:1157.
- Roster helpers:
    - `isKnownTeamID` (web.go:1169), called at 1249
    - `divisionRoster` (web.go:1315), called at 1340 and 1405
    - `allBelongToRoster` (web.go:1326)
    - `nhlPlayerBySlug` (web.go:830), called at 847 and 856
    - `isValidAwardFinalist` (web.go:846), called at 881, 954 and 1532
- Store accessors: `store.Team{ID,Name,Conference,Division}`, `store.AwardFinalist{Slug,DisplayName,Position}`, `st.Teams()`, `st.NHLPlayers()`. `PredictionSet.Upcoming` has the yaml tag `upcoming` (store.go:72).
- `src/internal/web/web_test.go` (2949 lines). Tests to mirror for #8: `TestGetPredictSheetShouldReturn404ForAnUnknownSetID` (757) and `TestPostPredictSheetShouldRejectAfterTheDeadlineWithNoOverride` (1034). The helpers to use are `newTestStoreWithPredictionSetsAndTeams`, `cupPredictionSetSeed` and `postSheet`. `TestPredictShouldDimAndLockAnUpcomingSetInsteadOfLinkingIt` (411) has an upcoming seed.
- `src/internal/web/README.md` is stale. It describes only `/` and login.
- `src/acceptance-tests/suite_test.go` already has these shared helpers: `newTestServer`, `newTempStore`, `noopSender`, `signedSessionCookieForTest` and `testSessionSecret`.
- The eager builders are byte-identical apart from their prefix and seed. They are `newLoadCanonicalTeamListStore` (load_canonical_team_list_steps_test.go:75) and `newLoadCanonicalNHLPlayerListStore` (load_canonical_nhl_player_list_steps_test.go:67).
- The lazy `ensureReady` + `do`/`get` skeleton is in four files:
    - browse_prediction_sets (:147)
    - cup_and_presidents_picks (:152)
    - division_picks (:167)
    - award_finalists (:190)

    What they share: the temp dir and dataFile, the seed header (the `season` line plus the basti player), writing the file and calling `store.New`, `httptest.NewServer(web.NewServer(...))`, a nil-safe close with `RemoveAll`, and a no-redirect request helper that records status, location and body.

    What stays in each file: the fixture structs and Given steps, the YAML body sections, browse's `fileBeforeRequests` snapshot, the post-store seeding, the `set == nil` guards, and the per-file secret and player constants.
- `parseRelativeDeadline` lives in browse_prediction_sets_steps_test.go:42 but all four files use it. Move it to the shared helper.
- Lint (`.golangci.yml`) runs `unused`, `revive`, `goimports` and `gocritic`. Orphaned helpers will fail the build.

## Tasks & Acceptance

**Execution:**

- [x] `src/internal/web/` -- Split `web.go` into these files: `web.go` (embeds, shared consts, `NewServer`, routing, `requireSession`, static files, render helpers), `shell.go` (shell data and handler), `predict.go` (Predict list view model), `options.go` (embed options), `login.go` (login and logout handlers), `sheet.go` (sheet registry, `sheetData`, `handleSheet`, `handleSheetSubmit`, cup and presidents), `sheet_divisions.go` (divisions view and submit) and `sheet_awards.go` (award tables, view and submit). Split `web_test.go` the same way into matching `*_test.go` files, keeping the shared test helpers in `web_test.go`. Move code only. -- #10
- [x] `src/internal/web/roster.go` -- Replace the four helpers with one shared shape: an id-keyed lookup built from a store slice with an optional filter, exposing `has` and `get`. Rewire every call site and delete the old helpers. Add a table-driven unit test (`roster_test.go`) covering has, get, a missing id, an empty id and the filter. -- #9
- [x] `src/acceptance-tests/fixture_support_test.go` -- Add a shared seeded-store builder that replaces both `newLoadCanonical*Store`. Add an embeddable lazy-fixture type that owns the temp dir, the seed header, write plus `store.New` with an optional post-store hook, the server start with a given secret, close, and the no-redirect `do(method, path, body)` recorder. Move `parseRelativeDeadline` here. Adapt the six step files to use it. -- #17
- [x] `src/acceptance-tests/features/cup-and-presidents-picks.feature` and its steps -- RED first. Add two scenarios: "An Upcoming set cannot be opened by direct URL" and "An Upcoming set cannot be submitted by direct POST, nothing is saved". Extend the cup fixture so a set can be marked upcoming. -- #8 BDD
- [x] `src/internal/web/sheet_test.go` -- RED first. Add unit tests for every row of the I/O matrix: GET upcoming, POST upcoming with nothing saved, upcoming past the deadline, and a table across the cup, divisions and awards ids. Add the "should not" counterpart: a non-upcoming open set still returns 200 and still saves. -- #8 TDD
- [x] `src/internal/web/sheet.go` -- In both handlers, right after the set is found, return 404 (`http.NotFound`) when `set.Upcoming` is true. Update the doc comments. -- #8
- [x] `src/internal/web/README.md` -- Update it to describe the current routes and the new file layout. -- Go rule: each package keeps a current README.
- [x] `_bmad-output/implementation-artifacts/sprint-status.yaml` -- Set action items epic-2-retro-item-8, -9, -10 and -17 to `done`. -- tracking

**Acceptance Criteria:**

- Given the refactor tasks are complete, when `task go:test` and `task go:test:acceptance` run, then every test that existed before passes unchanged.
- Given the split, when `web.go` is inspected, then it contains no sheet-kind-specific view, validation or submit logic.
- Given the roster refactor, when the package is grepped, then `isKnownTeamID`, `divisionRoster`, `allBelongToRoster` and `nhlPlayerBySlug` no longer exist, and all membership checks go through the shared shape.
- Given the fixture refactor, when the acceptance step files are inspected, then no step file defines its own `ensureReady` write/`store.New`/`httptest` block or its own no-redirect request helper.

## Implementation Notes

- The Upcoming gate lives in a new `findOpenablePredictionSet` helper instead of an inline `|| set.Upcoming`. The inline version pushed `handleSheetSubmit`'s cyclomatic complexity to 11, over gocyclo's limit of 10.
- The browse fixture's `fileBeforeRequests` snapshot is now taken right after `store.New` instead of right before it. `store.New` only reads an existing file, so the snapshot content is identical.
- `selectedDivisionTeams`' own ad-hoc membership map was also moved onto `newRoster`.
- `isValidAwardFinalist` remains, rebuilt on `playerRoster`.

## Spec Change Log

## Review Triage Log

Pass 1 (2026-09-24). The reviewers were Blind Hunter (BH), Edge Case Hunter (EC) and Verification Gap (VG).

| #  | Source      | Finding                                                                 | Verdict | Route  | Evidence                                                                                                                                                 |
|----|-------------|-------------------------------------------------------------------------|---------|--------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1  | BH, EC, VG  | `newRoster` keeps the last duplicate id; the old scans returned the first | low     | patch  | Map overwrite at roster.go. The store has no duplicate-id or duplicate-slug validation. Fix: skip ids already seen, and add a duplicate-key test.        |
| 2  | EC          | An empty team id is no longer a roster member                           | false   | reject | Deliberate per Design Notes ("an empty id is never a member"). Rejecting a blank `team_id` is the correct server-side validation.                     |
| 3  | BH, EC, VG  | Rosters are rebuilt as maps on every lookup                             | low     | reject | The result is correct. The cost is a map of a few hundred rows per slot. Fixing it adds parameters to several functions, which is more than a direct correction. |
| 4  | BH          | Key funcs `teamID`/`finalistSlug` are shadowed by local `teamID` variables | low     | patch  | Confirmed at sheet.go:313 and sheet_divisions.go:338. Rename to `teamKey`/`finalistKey`.                                                                |
| 5  | BH          | No test proves the Upcoming gate runs before form parsing               | low     | patch  | Matrix row 2 claims "checked before parsing the form", but every POST test sends a valid small form. Add an oversized-body POST to an upcoming set and expect 404. |
| 6  | BH          | No acceptance scenario for upcoming past its deadline, or for non-cup kinds | low     | reject | Unit tests cover all four kinds and the past-deadline row. The spec asked for two scenarios and they exist.                                              |
| 7  | BH          | "shows no team dropdown" step is vacuous after the exact-body check     | low     | patch  | The body is asserted to be exactly the not-found text, so the dropdown step can't fail. Delete the step.                                                 |
| 8  | BH          | README lists the POST check order wrong                                 | low     | patch  | `handleSheetSubmit` checks `pickableSheetKinds` first, but the README puts it third.                                                                     |
| 9  | EC          | README says the shared test helpers live in web_test.go                 | low     | patch  | `newTestStoreWithAwardsRoster` is in sheet_awards_test.go and `postSheet`/`markUpcoming` are in sheet_test.go.                                           |
| 10 | BH          | Stale godoc comments (package comment, `NewServer`, `predictSheetSubmitPattern`, `maxSheetFormBytes`) | low     | defer  | The comments predate this change and were moved verbatim. They fit with retro item #11.                                                                 |
| 11 | BH, EC      | #17 is incomplete: Epic 1 step files keep their own clients and fixtures | medium  | defer  | Pre-existing duplication in app_shell, login, enter_login_code, log_out and stay_logged_in. Retro item #17 named only the load_canonical pair plus four lazy files. |
| 12 | BH          | `seedHeader` hardcodes the email and season                             | low     | reject | Every current fixture uses the same basti player and season. This is speculative future need.                                                            |
| 13 | BH          | `markUpcoming` silently does nothing when the flag text is absent       | low     | reject | The test would still fail loudly (200 vs 404). A guard adds complexity for a failure mode no one has hit.                                               |
| 14 | BH          | Unlock path for Story 3.2 is undocumented                               | low     | reject | This belongs to Story 3.2. The retro item's primary option ("enforce Upcoming server-side") is done, and the README names the gate as what round unlocking builds on. |
| 15 | BH          | The spec's Code Map cites stale web.go line numbers                     | low     | reject | The fix would edit this build's spec. The Code Map describes the baseline commit.                                                                         |
| 16 | BH          | Store-roster test naming and missing negative checks                    | low     | reject | The generic roster table already covers a missing id. The position filter is covered by the existing award submit tests, moved unchanged.               |

## Design Notes

Suggested roster shape (the names are illustrative):

```go
type roster[T any] map[string]T

func newRoster[T any](items []T, key func(T) string, keep func(T) bool) roster[T]
func (r roster[T]) has(id string) bool     { _, ok := r[id]; return ok }
func (r roster[T]) get(id string) (T, bool) { v, ok := r[id]; return v, ok }
```

`teamRoster(st)`, `divisionTeamRoster(st, division)` and `playerRoster(st)` wrap it. An empty id is never a member, which keeps `nhlPlayerBySlug`'s empty-slug guard.

The upcoming gate goes before the deadline check. That makes an upcoming set reject the same way whatever its deadline, matching `predictStatus`, where Upcoming takes precedence.

## Verification

**Commands:**

- `cd src && task test` -- expected: all unit tests pass, including the new roster and upcoming tests
- `cd src && task test:acceptance` -- expected: all scenarios pass, and the two new ones were seen failing first
- `task go:run` -- expected: builds and starts. Only `govulncheck` findings are acceptable as a failure.
- `task docker:build` -- expected: success. This is the authoritative check.
