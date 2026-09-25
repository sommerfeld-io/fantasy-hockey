# Sprint Change Proposal: Reconcile Planning Documents with As-Built Epic 4, and Give the Results-File Deferrals a Home

- **Date:** 2026-09-25
- **Trigger:** Epic 4 retrospective items A2 and A3 (`_bmad-output/implementation-artifacts/epic-4-retro-2026-09-25.md`, findings R2, R3, R6, R7, S1–S3). A2 also absorbs Epic 3 retro item 24.
- **Mode:** batch
- **Scope classification:** Minor to moderate. It's mostly documentation, plus one new backlog story.

## 1. Issue summary

Decisions made with the user during the Epic 4 build and review changed how the app behaves in three ways the planning documents don't reflect:

- **Results take effect after a restart, not live.** `internal/store` reads the data file only at startup (AD-27). A hand-recorded result shows up on the Leaderboard only after the app restarts, while new picks show up immediately.
- **The app re-serializes hand-maintained sections on every save.** Values are unchanged, but formatting isn't: comments and unknown keys are dropped, flow style becomes block style, and `games: 5` becomes `"5"`. The documents say the app never writes those sections.
- **Every rank-1 player is gold, and nobody is while the top Total is 0.** Story 4.2's AC says "the leader".

Separately, four results-file problems were deferred "to story 7-3" in `deferred-work.md`, but Story 7.3's ACs cover only yamllint compliance:

- a misspelled key is silently ignored (and then dropped on the next save);
- a wrongly shaped entry makes the app refuse to start;
- saves rewrite the hand-maintained sections, dropping comments and unknown keys;
- `games` gets re-quoted.

**Evidence:**

- `src/internal/store/README.md` "Results (read-only)".
- `docs/recording-results-and-playoffs.md`.
- The retro's behavior check, which saw a save rewrite `games: 5` as `"5"` and strip comments.
- A probe: `playoffs: FLA` gives `yaml: unmarshal errors` and startup stops; `stanley_cup_winer: FLA` loads with no error or warning.
- `src/internal/standings/standings.go` (the leader rule).
- The spec-4-1 and spec-4-2 decision records.

## 2. Impact analysis

- **Epic impact:**
    - Epics 3 and 4 are done, and only their text changes.
    - Epic 7 gains Story 7.4.
    - Epics 5 and 6 are unaffected. Epic 6 is season rollover; its 6.1 AC never reads or writes the archived file.
- **Story impact:**
    - Wording changes to Story 3.2 AC2, and Story 4.2 AC1 and AC3.
    - New Story 7.4.
    - Story 7.3 gets a note linking it to 7.4, plus a reminder to remove the temporary `.yamllint.yml` ignore of `src/fantasy-hockey.yml` (commit `27f6868`).
- **Artifact conflicts:**
    - PRD: UJ-5, the Glossary's `fantasy-hockey.yml` entry, the §4.3 Notes and FR-23.
    - Architecture: AD-9, AD-23 and AD-27, the illustrative data-file shape (its header note and the `playoff_matchups` example), and AD-21's status note is untouched.
    - UX: no conflict. EXPERIENCE's Leaderboard row already says every rank-1 tie shares a rank, and it only says "live" for Division pick caps.
    - Implementation records: the frozen text in `spec-4-1`, `epic-4-context.md`, and `src/internal/standings/README.md`.
- **Technical impact:** none from A2, which is text only. Story 7.4 is future code work in `internal/store` and `main.go`.

## 3. Recommended approach

**Direct adjustment.** Reconcile every document with what was built, and add Story 7.4 to the Epic 7 backlog.

- There's no rollback: the as-built behavior was chosen deliberately with the user.
- There's no MVP change: FR-23 and FR-24 are still met.

Risk is low. The main value is that the next build, Epic 5, and the operator runbook stop working from contradictory sources.

On priority: Story 7.4 matters most before the playoffs start and results are recorded by hand in bulk. Consider doing it before Epic 6.

## 4. Detailed change proposals

### 4.1 Stories (`_bmad-output/planning-artifacts/epics.md`)

#### Story 3.2, AC2 (Epic 3 retro item 24)

OLD:

```text
**Given** a human directly adds that round's matchups to `fantasy-hockey.yml`
**When** I next load Predict
**Then** the round's set becomes Open — no in-app action, no admin screen, ever triggers this
```

NEW:

```text
**Given** a human directly adds that round's matchups to `fantasy-hockey.yml` (with the app stopped, then restarted — the store reads the file only at startup, AD-27)
**When** I next load Predict after the restart
**Then** the round's set becomes Open — no in-app action, no admin screen, ever triggers this
```

Rationale: this is what was built (AD-27). An edit made while the app runs is overwritten by the next save.

#### Story 4.2, AC1

OLD:

```text
**And** the leader's rank badge and Total render in `gold`, and no row carries a "(you)" marker
```

NEW:

```text
**And** every rank-1 player's rank badge and Total render in `gold` (tied leaders are treated alike, since singling one out would be a tiebreaker), nobody is gold while the top Total is 0, and no row carries a "(you)" marker
```

Rationale: this is the spec-4-2 user decision (2026-09-25), implemented in `standings.go`.

#### Story 4.2, AC3

OLD:

```text
**Given** any new result or prediction affecting scoring
**When** I open or refresh Leaderboard
**Then** it reflects the latest computation live — there is no separate "in-progress"/projected tier
```

NEW:

```text
**Given** a new prediction saved in the app, or a new result recorded by hand in `fantasy-hockey.yml` and picked up by restarting the app (AD-27)
**When** I open or refresh Leaderboard
**Then** it reflects the latest computation — recomputed on every request, never cached, with no separate "in-progress"/projected tier
```

Rationale: picks show up immediately, and results show up after a restart. "Live" meant "not cached", and that part holds.

#### Story 7.3, references paragraph (append one sentence)

NEW (added at the end of the existing references paragraph):

```text
Remove the temporary `src/fantasy-hockey.yml` entry from `.yamllint.yml`'s `ignore:` list (added in commit `27f6868` while the app's own writes weren't yet lint-clean) once this story's writes pass. Story 7.4 owns preserving the hand-maintained sections' own formatting; this story only guarantees the app's output is lint-clean.
```

#### New Story 7.4 (appended to Epic 7)

```markdown
### Story 7.4: Hand-Edited Results Are Safe to Edit

As the person running the pool,
I want the app to catch my mistakes in the hand-maintained sections of fantasy-hockey.yml and leave what I wrote exactly as I wrote it,
So that a typo never silently costs someone points, never takes the app down, and never gets rewritten away by the next player's save.

**Acceptance Criteria:**

**Given** a misspelled or unknown key inside `results:` or `award_finalists:` (e.g. `stanley_cup_winer`, `divison_winner`, `gmes`)
**When** the app starts
**Then** it logs one "malformed result" warning naming the key's path, alongside the existing value-level warnings — never silently ignoring it

**Given** a wrongly shaped entry in `results:` or `award_finalists:` (e.g. a scalar where a list belongs, `playoffs: FLA`)
**When** the app starts
**Then** it still starts, logs a warning naming the entry, and scores that entry as 0 — and the next save does not wipe or rewrite what the human wrote in that section

**Given** hand-maintained sections the app never changes (`results:`, `award_finalists:`, and the other human-maintained sections per AD-23)
**When** the app saves a prediction or a login code
**Then** those sections are written back exactly as the human wrote them — comments, flow/block style, quoting (`games: 5` stays unquoted), key order and unknown keys preserved

**Given** a startup with a readable data file
**When** loading completes
**Then** one summary line reports how many result problems were found (including zero), so a clean restart is distinguishable from an unchecked one

*References: Architecture AD-9, AD-23, AD-27; Epic 4 retro findings R3, R6, R7 (`epic-4-retro-2026-09-25.md`); `deferred-work.md` entries sourced from `spec-4-1-automatic-scoring-engine.md`; operator runbook `docs/recording-results-and-playoffs.md` (update its "not warned about" list when this ships). Out of scope: detecting a hand edit made while the app runs (the runbook's stop-edit-restart rule stands). Coordinate with Story 7.3, which owns lint-clean output. Ask the human whether a Gherkin acceptance test applies before implementation.*
```

Rationale: this gives all four deferred problems, plus the retro's missing summary line (R7), a real home.

### 4.2 PRD (`_bmad-output/planning-artifacts/prds/prd-fantasy-hockey-2026-09-14/prd.md`)

#### UJ-5 (line 47)

OLD: `Sadl, right after Round 1 results are entered, opens Leaderboard to see Regular, Playoff, and Total points update live, and where he now ranks.`

NEW: `Sadl, right after Round 1 results are entered (and the app restarted to pick them up), opens Leaderboard to see Regular, Playoff, and Total points recomputed, and where he now ranks.`

#### Glossary, `fantasy-hockey.yml` (line 68)

OLD: `... No in-app UI reads from or writes to it except as a read-only source.`

NEW: `... The app treats those sections as read-only: no in-app UI creates or changes them, and the app picks up a hand edit only after a restart.`

#### §4.3 Notes (line 214)

OLD: `... only *reads* it; nothing in the app ever writes to that part of the file.`

NEW: `... only *reads* it; nothing in the app ever creates or changes that part of the file. (The app rewrites the whole file when it saves a pick or login code, carrying the hand-maintained sections over with their values unchanged — preserving their exact formatting is Story 7.4.)`

#### FR-23 consequence (line 239)

OLD: `- Always reflects the latest scoring computation live — there is no separate "in-progress"/projected tier.`

NEW: `- Always reflects the latest scoring computation — recomputed on every view, never cached; a newly saved pick shows immediately, a hand-recorded result after the app is restarted. There is no separate "in-progress"/projected tier.`

### 4.3 Architecture (`_bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md`)

#### AD-9, Rule

OLD: `Only predictions and login codes are written by the app at runtime, in response to a player's own action — everything else is human-maintained (AD-23).`

NEW: `Only predictions and login codes are created or changed by the app at runtime, in response to a player's own action — everything else is human-maintained (AD-23). Because writes are whole-file (AD-27), every save re-serializes the hand-maintained sections too, with their values unchanged; keeping their exact formatting is Story 7.4.`

#### AD-23, Rule (last sentence)

OLD: `No package exposes a way to create/modify these sections; `internal/store`'s writers cover only predictions and login codes (AD-9).`

NEW: `No package exposes a way to create/modify these sections; `internal/store`'s writers change only predictions and login codes (AD-9) and carry every hand-maintained section over unchanged on each whole-file write. The store reads these sections once at startup, so a hand edit takes effect after a restart and must be made with the app stopped (AD-27; operator runbook `docs/recording-results-and-playoffs.md`).`

#### AD-27, Rule (append)

NEW (added at the end): `Consequence (as built, Epic 4): because the file is read only at startup, a hand edit made while the app runs is not seen and is overwritten by the next write. The stop-edit-restart rule is the accepted mitigation; detecting on-disk changes before a write is not planned.`

#### Illustrative data-file shape, header note

OLD: `Not a final schema — exact field names/nesting are an implementation-time decision (PRD Open Question 3) — but this is the kind of shape `fantasy-hockey.yml` takes, per AD-9/AD-17/AD-20/AD-23:`

NEW: `Illustrative only. The as-built schema for results, award finalists and playoff matchups is documented in `src/internal/store/README.md` and the operator runbook `docs/recording-results-and-playoffs.md`; this block follows it for those sections, per AD-9/AD-17/AD-20/AD-23:`

#### Illustrative data-file shape, `playoff_matchups` example

OLD:

```yaml
playoff_matchups:
    round2:
        - { a: "FLA", b: "TOR" }   # unlocks Round 2's Prediction set once present (AD-23, FR-20)
```

NEW:

```yaml
playoff_matchups:
    r2:                            # keyed by Prediction Set id: r1, r2, cf, scf
        - { key: "s1", a: "FLA", b: "TOR" }   # key: unique per round, never renamed; unlocks r2 once present (AD-23, FR-20)
```

### 4.4 Implementation records

#### `spec-4-1-automatic-scoring-engine.md`, frozen block (human renegotiation, 2026-09-25)

- Always: `` `internal/scoring` imports only `internal/store`, never `internal/web`, and nothing else imports it yet. `` becomes `` `internal/scoring` imports only `internal/store`, never `internal/web`. Its only production importer is `internal/standings` (Story 4.2); the acceptance tests also call it. ``
- Decision, malformed results: add the two classes added in review (a series winner not in its matchup, a `team_marks` team from another division). Replace "The app still starts" with "The app still starts for a bad value; a wrongly shaped entry currently stops startup (Story 7.4)".
- Never: `Write any score, or anything from the results section, to fantasy-hockey.yml.` becomes `Write any score to fantasy-hockey.yml, or create or change anything in the results section. (Whole-file saves carry it over with its values unchanged, per AD-27.)`
- I/O matrix row "Winner wrong, team made it": the input becomes `Picked TOR as winner and in the playoff list; TOR made playoffs but FLA won` (user decision in the code review: the 5 comes from the list).
- Outside the frozen block, AC1 becomes: `Given a seeded data file with picks and results, when scoring runs, the results are edited by hand and the app restarts, and scoring runs again, then the second call reflects the edit and the data file's bytes are unchanged by scoring.`

#### `epic-4-context.md`, Requirements

OLD: `... a new result or prediction shows up on the next open or refresh.`

NEW: `... a new prediction shows up on the next open or refresh; a hand-recorded result after the app restarts (AD-27).`

#### `src/internal/standings/README.md:16`

OLD: `... so a new pick or a reloaded result shows up on the next call.`

NEW: `... so a new pick shows up on the next call. Hand-recorded results are read only at startup (see internal/store), so a new result shows up after a restart.`

### 4.5 Sprint status

- Add `7-4-hand-edited-results-are-safe-to-edit: backlog` under `epic-7`.
- Once the edits are applied, mark three items `done`: `epic-4-retro-item-27` (A2), `epic-4-retro-item-28` (A3) and `epic-3-retro-item-24`, which is still open and is absorbed by the Story 3.2 edit.

## 5. Implementation handoff

- **Scope:** minor to moderate. These are document edits the developer applies directly, plus one backlog story. No replan is needed.
- **Applies it:** this correct-course run, after approval (developer role). The frozen spec-4-1 edits are covered by the human's approval of this proposal.
- **Success criteria:**
    - No planning document still claims results are live, that the app never writes hand-maintained sections, or that one leader is gold.
    - The architecture's examples load as written.
    - Story 7.4 is in `epics.md` and `sprint-status.yaml` (validator passes).
    - `task lint` passes.
- **Next:** `bmad-build 5-1-compare-selector-and-side-by-side-table`. Schedule Story 7.4 before the playoffs.
