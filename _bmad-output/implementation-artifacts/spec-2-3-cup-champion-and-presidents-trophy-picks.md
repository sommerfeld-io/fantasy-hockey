---
title: 'Cup Champion and Presidents'' Trophy Picks'
type: 'feature'
created: '2026-09-16'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '314fb2dc1fff21edbe714fc4e140e2d00168a365'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The "cup" and "presidents" Prediction Sets only render Story 2.1's static stub ("Not available yet.") — a player has no way to actually pick the Stanley Cup winner or the Presidents' Trophy winner, the app's first two before-season predictions.

**Approach:** Replace the stub with a real single-team-pick Prediction sheet (one dropdown grouped by division, sourced from Story 2.2's `Store.Teams()`) reused identically for both sets, backed by a new `Prediction` row per player+set persisted in `fantasy-hockey.yml`. Submitting saves/updates the pick; reopening pre-fills it in edit mode; a past-deadline set is read-only. The Predict list activates the already-scaffolded "Submitted" status (green pill/accent) once a pick exists.

## Boundaries & Constraints

**Always:** One `Prediction` row per (player, kind); `Kind` values are `store.KindCupChampion` = `"cup"` and `store.KindPresidentsTrophy` = `"presidents"`, matching the Prediction Set's own `id` (AD-17/AD-24/AD-28) — one identifier vocabulary, no separate mapping table. A resubmission updates the existing row in place (FR-9), never appends a duplicate. The server re-validates the submitted team id against `Store.Teams()` on every POST regardless of what the client sent (AD-10's server-revalidates stance) — an empty, missing, or unknown id is rejected with an inline error and nothing is saved. A Closed (past-deadline) set rejects POST outright, no override for anyone (FR-8); GET renders a read-only banner with the pick (if any) shown disabled, no action bar. The Predict list's status priority becomes Upcoming > Closed > Submitted > Open, reusing `statusSubmitted`/`statusPillSubmitted`/`.set-row--submitted`/`.status-pill--submitted` — all already scaffolded, unused since Story 2.1. Only the `"cup"` and `"presidents"` ids get this real form; every other Prediction Set id keeps rendering today's stub unchanged.

**Never:** No Division picks, Player awards, or any Epic 3 playoff-pick persistence — those stay stubbed. No client-side JS for this form — AD-10 sanctions JS only for the awards autocomplete (2.6) and Division's live-cap (2.4), neither of which is this story; the dropdown is a plain server-rendered `<select>`/`<optgroup>`, and the "picked" checkmark reflects saved (Submitted) state on render, not a live pre-submit toggle. No new `/api/...` endpoint or JSON-embed shape — the team list renders as native `<option>`s from `Store.Teams()`, not Story 2.2's `teamOption`/`newTeamOptions` (AD-19 embed shape, which stays unreached, reserved for 2.6's autocomplete).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Open set, no prior pick | `GET /predict/cup` | Form renders, no team preselected, button reads "Submit predictions" | N/A |
| Open set, prior pick exists | `GET /predict/cup` | Team preselected, button reads "Update predictions" | N/A |
| Submit a valid team id before deadline | `POST /predict/cup` `team_id=TOR` | `Prediction` saved/updated; 302 to `/predict`; Predict list shows Submitted (green) | N/A |
| Submit a missing or unknown team id | `POST /predict/cup` `team_id=""` or `"ZZZ"` | Re-renders the sheet (200) with an inline error caption; nothing saved | N/A |
| Submit after the deadline | `POST /predict/cup` (closed) | Rejected, no save, no override for anyone | `http.StatusForbidden` |
| Open a closed set | `GET /predict/cup` (closed) | Read-only banner; select shown disabled with any saved pick; no action bar | N/A |
| A Prediction Set id that isn't cup/presidents | `GET /predict/divisions` | Unchanged: today's "Not available yet." stub | N/A |
| Unknown Prediction Set id | `GET` or `POST /predict/does-not-exist` | Generic not-found response | `http.StatusNotFound` |

</frozen-after-approval>

## Code Map

- `src/internal/store/store.go:50-63,197-257` (`document`, `CreateLoginCode`, `ConsumeLoginCode`) — add a `Prediction` struct (`ID, PlayerID, Kind, TeamID, SubmittedAt string`, all `yaml`-tagged) + `Kind` consts (`KindCupChampion="cup"`, `KindPresidentsTrophy="presidents"`) + `document.Predictions []Prediction`. Add `Store.FindPrediction(playerID, kind string) (Prediction, bool)` (RLock, linear scan, return copy — mirrors `FindPlayerByID`). Add `Store.SavePrediction(playerID, kind, teamID string, now time.Time) error`: find an existing row by (playerID, kind); if found, remember its old `TeamID`/`SubmittedAt` and mutate in place; else append a new row (`uuid.NewString()`, mirrors `CreateLoginCode`'s append+rollback-by-truncation); call `writeLocked()`; on error roll back (restore old values, or truncate the appended row) and wrap the error.
- `src/internal/web/web.go:79-83` (`predictSheetPattern`) — add `predictSheetSubmitPattern = "POST /predict/{id}"`, registered on `authMux`/`mux` the same way (294-304) so the two can't drift (per Epic 1 retro action item 4's own fix).
- `src/internal/web/web.go:94-109` (`statusOpen`/`statusSubmitted`/pill consts) — already exist, unused; this story is what starts returning `statusSubmitted`.
- `src/internal/web/web.go:140-207` (`newPredictSetView`, `predictStatus`, `buildPredictPhases`) — thread a `submitted bool` through: `predictStatus(upcoming, submitted bool, deadline, now time.Time)` with precedence Upcoming > Closed > Submitted > Open; `newPredictSetView` gains a `submitted bool` param, sets `AccentCSS = "set-row--submitted"` when submitted and not upcoming (upcoming still wins); `buildPredictPhases` gains a `playerID string` param, looks up `st.FindPrediction(playerID, set.ID)` per row (kind == set.ID for this story's two sets; any other set's `FindPrediction` naturally misses). `handleShell` (361-386) passes the context's playerID through.
- `src/internal/web/web.go:388-430` (`sheetData`, `handleSheet`) — extend `sheetData` with `Closed bool` and an optional pick-view (new small struct: `SelectedTeamID string`, `Submitted bool`, `Error string`, `Divisions []teamDivisionGroup` where `teamDivisionGroup{Division string; Teams []store.Team}`, grouped from `st.Teams()` in the fixed order Atlantic/Metropolitan/Central/Pacific). `handleSheet` builds this pick-view only when `id` is `store.KindCupChampion` or `store.KindPresidentsTrophy`; every other id keeps today's plain stub path unchanged. Extract playerID via `auth.PlayerIDFromContext` (same pattern as `handleShell:364`).
- `src/internal/web/web.go` (new) — `handleSheetSubmit(st *store.Store) http.HandlerFunc` for `POST /predict/{id}`: 404 if id isn't cup/presidents or set unknown; reject (403) if closed; validate `r.FormValue("team_id")` against `st.Teams()`; on success call `st.SavePrediction`, redirect 302 `/predict`; on invalid input, re-render "sheet.html" (200) with the same pre-filled state plus an error caption.
- `src/internal/web/templates/sheet.html` — replace the stub body with: a `<select name="team_id" required {{if .Closed}}disabled{{end}}>` with one `<optgroup>` per division; an error caption (`.error-text`, reuse) when `.Error` is set; a submit `<button>` reading "Update predictions"/"Submit predictions" per `.Submitted`, hidden entirely when `.Closed`; a `.hint` caption "Editable until the deadline." when open; a read-only banner when `.Closed`.
- `src/internal/web/static/styles.css:113-123` (`button`, generic) — already primary-green-styled, reused as-is for Submit/Update with no new class. `.hint`/`.error-text` (125-135ish) already exist, reused for the caption/inline-error. `.status-pill--submitted`/`.set-row--submitted` (294, 334) already exist, now reachable. New rules needed: a `select`/`optgroup` style (DESIGN.md's Input/dropdown component: `--raised` fill, `--border`, `lg` radius) and a `.closed-banner` read-only notice.
- `src/internal/store/store_test.go:309-` (`CreateLoginCode`/`ConsumeLoginCode` tests) — structural pattern for `FindPrediction`/`SavePrediction`'s own tests (seeded lookup, not-found, update-in-place, rollback-on-write-failure).
- `src/internal/web/web_test.go:411-513` (`handleSheet` cluster) — structural pattern for the new pick-view branches and `handleSheetSubmit`'s tests (valid/invalid/closed/unknown-id).
- `src/acceptance-tests/browse_prediction_sets_steps_test.go` + `features/browse-prediction-sets.feature` — structural pattern (temp-dir store, `httptest.Server`, signed session cookie, relative-deadline helper) for this story's new feature file.

## Tasks & Acceptance

**Execution:**
- [x] `src/acceptance-tests/features/cup-and-presidents-picks.feature` + steps file -- scenarios for the AC below -- must fail before implementation (BDD red)
- [x] `src/internal/store/store.go` -- `Prediction` struct, `Kind` consts, `document.Predictions`, `FindPrediction`, `SavePrediction` -- unit tests: not-found, seeded-found, update-in-place on resubmit, rollback-on-write-failure
- [x] `src/internal/web/web.go` -- `predictSheetSubmitPattern` route; thread `submitted`/`playerID` through `predictStatus`/`newPredictSetView`/`buildPredictPhases`/`handleShell`; extend `sheetData` + `handleSheet` for cup/presidents; new `handleSheetSubmit` -- unit tests per the I/O matrix
- [x] `src/internal/web/templates/sheet.html` -- real form: select+optgroups, error caption, submit button text logic, closed read-only banner
- [x] `src/internal/web/static/styles.css` -- `select`/`optgroup` styling, `.closed-banner`

**Acceptance Criteria:**
- Given the "Cup champion" or "Presidents' Trophy" set is Open, when I tap it, then a full-screen Prediction sheet opens with a single team dropdown grouped by division; choosing a team and submitting saves my pick keyed by the team's id and returns me to Predict with the set now Submitted (green).
- Given the set is Submitted and still before its deadline, when I reopen it, then it opens with my current pick pre-filled and the action button reads "Update predictions"; re-saving updates my pick.
- Given the set's deadline has passed, when I open it, then it shows a read-only banner, every input disabled, no action bar — not even for the player who made the pick.
- Given I never made a pick for this set before its deadline, then this item scores zero later without having blocked anything else I predicted (no Prediction row is ever force-created for an empty pick).

## Implementation Notes

- Two pre-existing Story 2.1 stub tests (`TestGetPredictSheetShouldRenderTheStubPageForAKnownSet`, `...ForAClosedSet`) used Prediction Set id `"cup"` to exercise the generic stub path; repointed to a non-pickable id (`"divisions"`) since `"cup"` now renders the real pick form, matching this story's own Boundary that only cup/presidents change. `browse-prediction-sets.feature`'s equivalent stub-page scenario was updated the same way.
- The new acceptance feature's "the player opens the Prediction Set ..." step text already existed (near-identically) in `browse_prediction_sets_steps_test.go`; renamed to "the player opens the cup-picks Prediction Set ..." after godog silently routed to the wrong scenario state with the original wording — caught via a failing assertion, not a step-registration error.
- `renderRejectedPick`'s `newSheetData` error branch (a `deadline_utc` parse failure) is unreachable in practice, since `handleSheetSubmit` already parses the same deadline successfully earlier in the same request; kept as defensive code rather than assuming the invariant holds forever, and is the one intentionally-untested line in the new surface (`renderRejectedPick` at 57.1% coverage).
- No hand-edit to `fantasy-hockey.yml` was needed — the existing `cup`/`presidents` entries already had the right `id`s from Story 2.1's seed.
- Verified with `task go:test` (91.8% total, 100% on all new store/web methods except the one noted above), `task go:test:acceptance` (all scenarios pass, including 11 new cup/presidents ones), `task lint` and `task go:build` (full pipeline green: vet, golangci-lint, gocyclo, go-licenses, govulncheck — no vulnerabilities), and `task docker:build` (authoritative container pipeline, image built successfully).

## Spec Change Log

## Review Triage Log

- **false** — Verification Gap + Blind Hunter (x2) (`src/internal/web/web.go:152-158`, `newPredictSetView`'s `accentCSS` switch): a set that is both past its deadline and has a saved `Prediction` shows a "Closed" (red) status pill but a `set-row--submitted` (green) accent border — no `Closed` case in the accent switch, unlike `predictStatus`'s Upcoming>Closed>Submitted>Open precedence. Verified directly against the authoritative UX reference (`imports/faceoff-pool-source/fantasy-hockey/src/App.jsx:290-292`, `SetRow`'s `accent` computation: `set.upcoming ? c.border : data?.submitted ? c.green : c.ice` — no `closed` check at all) and `StatusPill` (`App.jsx:279-286`, whose *label* does check `closed` before `submitted`). The pill/accent split is a faithful, deliberate reproduction of the reference's own design, not a defect — matches the identical precedent Story 2.1's own review already established for the closed-alone case (`spec-2-1-...md`'s Review Triage Log: "the actual authoritative UX composition reference... never uses red for the accent stripe either... The implementation matches this reference exactly").
- **false** — Blind Hunter: the new bare `select`/`optgroup`/`option` CSS selectors aren't scoped under `.pick-form`, so a future `<select>` elsewhere would inherit this story's styling. Verified: this matches the codebase's own established convention (bare element selectors like `button` at `styles.css:113-123`, overridden by a specific class like `.logout-btn` only when a use site needs to differ) — not a deviation this diff introduced.
- **low, rejected** — Blind Hunter + Edge Case Hunter (`src/internal/web/web.go:434-447`, `groupTeamsByDivision`): a team whose `Division` doesn't match one of the four hardcoded division names is silently omitted from the dropdown, no `slog` diagnostic, unlike `buildPredictPhases`'s logged skip-malformed-row pattern. Reachable only via a hand-edit typo in `fantasy-hockey.yml`'s `teams:` section — same grounds Story 2.2's own review already rejected the identical class of finding (`spec-2-2-...md`'s Review Triage Log: "reachable only via a hand-edit mistake... matches the established no-validation convention for hand-maintained Player/PredictionSet data").
- **low, rejected** — Edge Case Hunter (`src/internal/store/store.go:284-294`, `FindPrediction`/`SavePrediction`): two `Prediction` rows sharing a (playerID, kind) pair would have only the first used, the second silently ignored. Unlike `Team`/`PredictionSet`/`Player`, `predictions:` is app-written, not hand-maintained (AD-9) — `SavePrediction`'s own tests already prove the app itself never creates a duplicate row; a duplicate could only arise from an unsupported manual edit to an app-owned file section, an even less likely path than the hand-edit-reachable findings already rejected elsewhere in this project. Guarding against that is speculative complexity beyond any demonstrated need.
- **low, rejected** — Blind Hunter: `renderRejectedPick`'s doc comment says a rejected submission's `teamID` is "retained" in the re-rendered form, but for a genuinely unknown id (not just an empty one) no `<option>` matches it, so the dropdown silently shows the placeholder instead of visibly echoing the bogus value. Only reachable via a request that bypasses the real `<select>` UI entirely (a browser's own dropdown can only submit one of its rendered `<option>` values) — the inline error caption is still shown regardless, so the rejection itself is communicated either way.
- **low, rejected** — Blind Hunter: if a previously-saved `Prediction.TeamID` no longer matches any current `Store.Teams()` entry (a team hand-removed/renamed after the pick was saved), the dropdown silently shows no selection while the button still reads "Update predictions." Reachable only via a hand-edit to `fantasy-hockey.yml`'s `teams:` section after real picks exist — same hand-edit-reachable grounds as the `groupTeamsByDivision` row above.
- **false** — Blind Hunter: `sprint-status.yaml` shows `2-3-...: in-progress` while the spec's own frontmatter is `status: 'in-review'` with every task checked. This is the expected mid-workflow state — `sprint-status.yaml` syncs to `review` at the *present* step, not the *review* step this diff is currently in.
- **medium** — Blind Hunter: `cup-and-presidents-picks.feature`'s Background is titled generically ("cup and presidents picks"), but every one of its 10 scenarios opens/submits only the `"cup"` id — confirmed via `grep -n "Scenario\|/predict/" acceptance-tests/features/cup-and-presidents-picks.feature`. Unit tests fare no better: the only handler-level test using `"presidents"` (`web_test.go:809`) exercises the negative 404-when-unseeded case, never a full save/reload cycle. `handleSheet`/`handleSheetSubmit`/`newSheetData`/`isKnownTeamID`/`SavePrediction` are all fully id-agnostic today, so the risk is currently latent rather than active — but nothing would catch a future edit that accidentally special-cased `"cup"` and broke `"presidents"`. Route: patch — add at least one scenario/test driving a full pick-submit-reload cycle for `"presidents"`. **Fixed 2026-09-16**: added a feature scenario submitting a valid pick for `"presidents"` (asserting redirect, saved pick, and Predict-list Submitted status) plus a unit test (`TestPostPredictSheetShouldSaveAValidPickForPresidentsAndShowSubmittedOnReload`) covering the same end-to-end for the handler layer.
- **low** — Blind Hunter (`src/internal/web/web.go:235-236`, `newTeamOptions`'s doc comment): still reads "Not called from any handler yet - 2.3/2.4 embed it once they add their own dropdowns/chips," but this story's own Design Notes state 2.3 deliberately does *not* use `teamOption`/`newTeamOptions` (native `<option>`s from `Store.Teams()` instead, reserved for 2.6's autocomplete). Route: patch — drop the stale "2.3/" reference. **Fixed 2026-09-16**: reworded to note this story renders native `<option>`s from `Store.Teams()` directly instead, with 2.4/2.6 as the still-future embedders.
- **low** — Blind Hunter (`src/internal/web/static/styles.css`): `select:disabled` dims `color`/`opacity` but `optgroup`/`option` get no corresponding rule. Reachable in ordinary use (every closed cup/presidents set hits this), so this doesn't qualify for low-rejection despite the cosmetic severity. Route: patch — add disabled-state color rules for `optgroup`/`option` matching the existing `--faint` token. **Fixed 2026-09-16**: added a `select:disabled optgroup, select:disabled option { color: var(--faint); }` rule.

## Design Notes

- `Kind` values reuse the Prediction Set's own `id` directly (`"cup"`/`"presidents"`) rather than a shared generic kind (e.g. `"team_mark"`) plus a separate set-reference field. The architecture spine's illustrative schema shows both phrasings (a generic `kind: "team_mark"` example next to AD-28's concrete `store.KindCupChampion` prose) — this spec takes the more specific reading: one identifier vocabulary, no second mapping to keep in sync.
- Submitting requires a real selection: the `<select required>` makes an empty pick unsubmittable without any JS, matching AD-10's server-revalidates-everything stance and AD-17's "never blank" id rule. This diverges from the UX click-dummy, which doesn't gate its Submit button for cup/presidents — but the dummy has no real persistence, so "submitted with nothing picked" was never a meaningful state there the way a saved-but-blank `Prediction` row would be here.
- EXPERIENCE.md's "label gets a check the moment a team is chosen" is satisfied by the native `<select>`'s own visible selection plus a real checkmark once truly Submitted (post-save) — not a live pre-submit JS toggle, since AD-10 sanctions JS only for the awards autocomplete and Division's live-cap, neither of which is this story.

## Verification

**Commands:**
- `task go:test` -- expected: new `store`/`web` unit tests pass
- `task go:test:acceptance` -- expected: the new feature file's scenarios pass end-to-end
- `task go:run` -- expected: app builds and starts

**Manual checks (if no CLI):**
- Pick a team for Cup champion and for Presidents' Trophy; confirm the Predict list shows both as Submitted (green); reopen one and confirm "Update predictions" with the pick pre-filled; edit a seeded deadline into the past and confirm the sheet becomes read-only.
