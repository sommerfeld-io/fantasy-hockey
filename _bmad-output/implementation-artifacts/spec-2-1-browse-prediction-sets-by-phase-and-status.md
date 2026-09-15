---
title: 'Browse Prediction Sets by Phase and Status'
type: 'feature'
created: '2026-09-15'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'd78043b953b78ca12c2501670f886d4f4a19c843'
context: ['{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-14/DESIGN.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Predict tab only shows Story 1.5's static "Predictions are coming soon." placeholder; a player has no way to see which prediction sets exist, when each closes, or whether it's open, submitted, closed, or upcoming.

**Approach:** Replace that placeholder with two phase-grouped ("Before the season" / "Playoffs") read-only lists of Prediction Set summaries sourced from `fantasy-hockey.yml`, each actionable row linking to a new minimal per-set page. Building the actual pick forms (Stories 2.3/2.4/2.6), team/player data loading (2.2/2.5), and round-unlocking logic (Story 3.2) stay out of scope.

## Boundaries & Constraints

**Always:** Prediction Set data (deadlines, phase, upcoming flag) is human-maintained in `fantasy-hockey.yml`; this story only reads it. The status-pill CSS supports all four states (Open/blue, Submitted/green, Closed/red, Upcoming/grey) even though "Submitted" is unreachable until a later story can record a pick. Every stored timestamp is RFC3339 UTC, converted to Europe/Berlin only for display. Deadline/countdown formatting and status computation live in one reusable helper, not duplicated per template.

**Never:** No pick-entry forms, team/player list loading, or persistence of picks — the per-set page this story adds is a static stub ("not available yet"), not a real Prediction sheet. No computed round-unlocking — `upcoming` stays a static YAML flag this story only reads. No in-app settings page for editing deadlines.

**Decision:** The 9 seeded Prediction Sets use the UX click-dummy's illustrative placeholder dates (`imports/faceoff-pool-source/fantasy-hockey/src/App.jsx:110-118`, e.g. 2026-10-06 19:00 Europe/Berlin for all four before-season sets) — human decision, confirmed 2026-09-15. These are explicitly placeholders the human hand-edits to the real 2026-27 season deadlines before players rely on them; this story never invents or hardcodes that responsibility away.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Set within its window | `upcoming: false`, deadline 5 days out | Open pill (blue), chevron, countdown "in 5 days" | N/A |
| Set past its deadline | `upcoming: false`, deadline in the past | Closed pill (red), chevron, countdown "closed" | N/A |
| Deadline later today | deadline today | Countdown reads "today" | N/A |
| Deadline tomorrow | deadline tomorrow | Countdown reads "tomorrow" | N/A |
| Set flagged upcoming | `upcoming: true` | Upcoming pill (grey), dimmed row, lock icon, not a link | N/A |
| Tap an actionable row | Open/Closed set id | Navigates to `GET /predict/{id}` stub page (title, deadline, countdown, "not available yet") | N/A |
| Unknown set id in URL | `GET /predict/does-not-exist` | Generic not-found response | `http.StatusNotFound` |

</frozen-after-approval>

## Code Map

- `src/internal/web/web.go:106-150` (`shellRoutes`, `NewServer`) -- add a `GET /predict/{id}` route registered the same way (through `authMux`/`requireSession`), alongside the existing shell routes.
- `src/internal/web/web.go:67-88,199-218` (`tabMessages`, `shellData`, `handleShell`) -- Predict's placeholder message is replaced by real content; extend `shellData` (or add a Predict-specific data type) with the two phase-grouped, status-annotated lists.
- `src/internal/store/store.go:50-54` (`document`) -- add a `PredictionSet` struct (ID, Title, Subtitle, DeadlineUTC, Phase, Upcoming) and `PredictionSets []PredictionSet \`yaml:"prediction_sets"\``, plus a read-only `Store.PredictionSets()` accessor — no write method (mirrors `Season()`'s read-only pattern).
- `src/internal/clock/clock.go:8-9` (`NowTime`) -- add the Europe/Berlin conversion + deadline/countdown-text helper next to it (or as a small package-local helper in `web` if not needed elsewhere).
- `src/internal/web/templates/shell.html:27-30` -- Predict tab's content block renders the two sections (icon+label header, set rows: title/subtitle/pill/deadline+countdown/chevron-or-lock) instead of `.Message`.
- `src/internal/web/templates/sheet.html` (new) -- minimal stub for `GET /predict/{id}`: back arrow to `/predict`, title, formatted deadline+countdown, static "not available yet" body, no action bar.
- `src/internal/web/static/styles.css:8-23` (tokens) -- add status-pill/set-row/section-header classes reusing `--ice`, `--green`, `--goal`, `--faint`/`--raised`, per DESIGN.md's Status pill and Set row components.
- `src/fantasy-hockey.yml` -- add the `prediction_sets:` seed list per the Open Question's resolution.
- `src/acceptance-tests/features/app-shell.feature` + `app_shell_steps_test.go` -- structural pattern (`Background:` seeded session, scenario-state struct, GoDog step wiring) to follow for this story's feature file.

## Tasks & Acceptance

**Execution:**
- [x] `src/acceptance-tests/features/browse-prediction-sets.feature` -- write scenarios for the AC below -- must fail before implementation (BDD red)
- [x] `src/acceptance-tests/browse_prediction_sets_steps_test.go` -- step definitions following `app_shell_steps_test.go`'s pattern
- [x] `src/internal/store/store.go` -- add `PredictionSet`, `document.PredictionSets`, `Store.PredictionSets()` -- unit tests: populated list, empty list
- [x] deadline/countdown helper -- table-driven unit tests covering every I/O matrix row (today/tomorrow/in N days/closed)
- [x] `src/internal/web/web.go` -- populate phase-grouped lists with computed status/pill/link-ability; register `GET /predict/{id}` stub handler
- [x] `src/internal/web/templates/shell.html` -- render the two Predict sections
- [x] `src/internal/web/templates/sheet.html` -- new stub page
- [x] `src/internal/web/static/styles.css` -- pill/row/header styles
- [x] `src/fantasy-hockey.yml` -- seed `prediction_sets:`

**Acceptance Criteria:**
- Given a logged-in player opens Predict, when the screen renders, then it shows "Before the season" and "Playoffs" sections (target/trophy icons) and every set shows title, subtitle, Europe/Berlin deadline+countdown, and exactly one correct status pill.
- Given an Open, Submitted, or Closed set, when its row renders, then it shows a chevron and links to `GET /predict/{id}`; given an Upcoming set, when its row renders, then it is dimmed with a lock icon and is not a link.
- Given `fantasy-hockey.yml`'s `prediction_sets` list, when the app runs through this story's code, then nothing ever writes back to it.

## Implementation Notes

- `internal/clock`: added `FormatDeadline` (Europe/Berlin display) and `Countdown` (today/tomorrow/in N days/closed), with `_ "time/tzdata"` blank-imported so `time.LoadLocation("Europe/Berlin")` resolves in the alpine run image, which ships no zoneinfo files. Verified by running the built `docker:build` image directly (no panic, `/login` and `/predict` both respond) rather than trusting it at build time only.
- `internal/store`: added `PredictionSet` + `document.PredictionSets` + `Store.PredictionSets()` (read-only, returns a defensive copy), mirroring `Season()`.
- `internal/web`: `predictSetView`/`predictPhases` hold every precomputed presentation field (status, pill/accent CSS classes, formatted deadline, countdown); `shellData.Predict` is `nil` for Leaderboard/Compare and non-nil (even when empty) for Predict, which `shell.html` branches on instead of `.Message`. `GET /predict/{id}` is registered on `authMux`/outer `mux` alongside `shellRoutes` but outside that slice (it needs `handleSheet`, not `handleShell`). Each set row carries `id="predict-row-{id}"` as a test hook so both unit and acceptance tests can scope assertions to one row.
- Status is computed as Upcoming > Closed (`deadline <= now`) > Open; `Submitted` is wired into the CSS/status vocabulary but never produced by this story's code (no pick persistence exists yet).
- Seeded all 9 `prediction_sets` from the UX click-dummy's placeholder dates, converted from Europe/Berlin to UTC (e.g. `2026-10-06T19:00 Europe/Berlin` -> `2026-10-06T17:00:00Z`, confirmed with Python's `zoneinfo`).
- Updated `app-shell.feature`/`app_shell_steps_test.go`: Predict no longer has "coming soon" placeholder text to assert on (replaced by this story's real content), so the two Predict-destination scenarios there dropped that assertion; Predict content itself is covered by the new feature file instead.
- Existing `web_test.go` shell-route table and a few substring assertions were updated to match Predict's new rendered content and the `id="predict-row-*"` attribute.

## Spec Change Log

## Review Triage Log

- **false** — Closed-set rows reuse the Open row's blue left-accent stripe (`set-row--open`); no `set-row--closed` CSS class exists. Reviewers (Blind Hunter, Edge Case Hunter, Verification Gap) cited epic-2-context.md's "blue/green/red/faint by state" prose, but the actual authoritative UX composition reference (`imports/faceoff-pool-source/fantasy-hockey/src/App.jsx:280-289`, `SetRow`'s `accent` computation) never uses red for the accent stripe either — only the status pill goes red for Closed; the stripe distinguishes only Upcoming (grey) and Submitted (green). The implementation matches this reference exactly.
- **false** — `clock.Countdown` rounds raw elapsed hours into days rather than comparing calendar dates in Europe/Berlin, so a same-day/next-day label can mismatch local calendar intuition near a day boundary (Blind Hunter). `Countdown`'s own doc comment states it deliberately matches the click-dummy's identical algorithm (`Math.round(ms / 86400000)` in `App.jsx:190-197`), the same authoritative reference cited above; the boundary behavior is inherited from that reference, not introduced by this diff.
- **false** — No test covers Predict rendering with an empty `prediction_sets` list, so the resulting "headers with zero rows" page state is unverified (Blind Hunter). The row-rendering path is a plain `range`/`append` over the store's list with no special-casing that could plausibly render incorrectly for zero items, and neither the AC nor DESIGN.md requires empty-state messaging — no bad outcome to verify.
- **false** — Nothing in this diff enforces that `cup` (this epic's Cup champion pick) and `playoffcup` (Epic 3's future Playoffs Cup pick) stay distinct (Blind Hunter, re: epic-2-context.md's Cross-Story Dependencies note). This story writes no picks at all yet — there is nothing to overwrite — and the two rows are already distinguished by `id`, which is the mechanism any future write-path would key off of. A real guard only becomes meaningful once Epic 3 adds writes.
- **false** — The CSS/status vocabulary for "Submitted" (`statusSubmitted`, `.status-pill--submitted`, `.set-row--submitted`) is unreachable by any code path in this story (Blind Hunter). This is exactly what the frozen `<frozen-after-approval>` Boundaries & Constraints mandates: "supports all four states... even though 'Submitted' is unreachable until a later story can record a pick." Compliant by spec, not a defect.
- **low, rejected** — The I/O & Edge-Case Matrix has no row for a malformed `deadline_utc` or an unrecognized `phase`, even though the code handles both (Blind Hunter). The only fix is editing this build's frozen spec, which is out of scope for triage per this workflow's rules.
- **low, rejected** — Two `prediction_sets` rows sharing the same `id` aren't detected; the later one silently loses the `GET /predict/{id}` route resolution race, and duplicate DOM ids render (Edge Case Hunter). Real but reachable only via a hand-edit copy/paste mistake, and the smallest fix (track seen ids, skip and log a duplicate) adds a new guard/branch — meets both conditions for rejecting a low finding.
- **medium** — `buildPredictPhases` silently drops a row whose `deadline_utc` fails `time.Parse` or whose `phase` isn't recognized, logging but never crashing or surfacing anything player-visible; no unit or acceptance test exercises either path, so a future edit that turned "skip" into "panic" or "render blank" would go uncaught (Verification Gap, corroborated by Blind Hunter). Real and plausible given `fantasy-hockey.yml` is explicitly a human hand-edit target — a typo silently disappears a Prediction Set. Route: patch — add table-driven cases proving a malformed row is skipped while sibling valid rows still render.
- **low** — `sheet.html`'s `.back-link` wraps a bare 20×20 SVG with no padding, giving the stub page's back button a smaller-than-32px tap target, under the UX spec's stated "tap targets ≥32px" accessibility floor (Blind Hunter). This header shape is documented to persist into later stories' real Prediction sheet, so it's worth correcting now. Fix is a direct CSS correction, not new complexity — doesn't qualify for low-rejection. Route: patch.
- **medium** — `GET /predict/{id}` is hardcoded as a route-pattern string twice (once on `authMux`, once on the outer `mux`) instead of reusing `shellRoutes`' single-source-of-truth table, which exists specifically to prevent this class of drift (per Epic 1 retro action item 4) (Blind Hunter). A future rename/change only needs to miss one of the two occurrences to silently desync auth coverage from route registration. Route: patch — extract the pattern to one shared value referenced by both registrations.
- **low** — The acceptance step "the player \"Basti\" has signed in" (`thePlayerHasSignedIn`) performs no sign-in action; the session cookie is attached unconditionally later in `get()` regardless of this step (Blind Hunter). Misleading step name/test-code clarity issue, not a functional defect. Fix is a direct rename/clarification, not new complexity — doesn't qualify for low-rejection. Route: patch.
- **low** — The acceptance-test YAML fixture builder in `browse_prediction_sets_steps_test.go` interpolates `title`/`subtitle`/etc. unquoted (`%s`) into generated YAML, so a future scenario using a colon or quote in those fields would get a cryptic YAML-parse failure instead of a clear assertion failure (Edge Case Hunter). Developer-only, not reachable by current scenarios, but the fix (`%q` instead of `%s`) is a direct correction — doesn't qualify for low-rejection. Route: patch.

## Design Notes

Seeding all 9 sets (not just the 4 Epic-2 ones) follows the UX click-dummy exactly (`imports/faceoff-pool-source/.../App.jsx:110-118`), which EXPERIENCE.md names as the authoritative composition reference for Predict. The `PredictionSet` schema intentionally omits a `kind` discriminator — nothing in this story needs it; whichever story later builds a real form for a set can add it then. The stub `sheet.html` exists only so the AC's "chevron means tappable" holds literally, without building any real form logic early.

## Verification

**Commands:**
- `task go:test` -- expected: new store/helper unit tests pass
- `task go:test:acceptance` -- expected: `browse-prediction-sets.feature` scenarios pass end-to-end
- `task go:run` -- expected: app builds and starts

**Manual checks (if no CLI):**
- Open `/predict` in a browser; confirm pill colors/icons match DESIGN.md and Upcoming rows aren't clickable.
