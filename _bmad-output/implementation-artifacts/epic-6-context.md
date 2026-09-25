# Epic 6 Context: Season Rollover & Multi-Season History

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

The pool must be able to restart cleanly for a new NHL season without losing any prior season's data. A human running the pool archives the current data file and repoints the app at a fresh path; the app then bootstraps a brand-new season from scratch there, while old seasons' files remain untouched on disk as read-only history. This is the mechanism that finally lets the app fully replace the old "one Excel file per season" workflow, since a season boundary was never something a single, ever-growing data file could represent on its own.

## Stories

- Story 6.1: Season Rollover With Retained History

## Requirements & Constraints

- Each NHL season must get its own pool: fresh season, fresh set of predictions, results, and standings, not layered on top of the previous season's data.
- Prior seasons' data must be retained, never deleted or overwritten by a rollover.
- There is no in-app season-selector, cross-season query, or history/Hall-of-Fame view in v1 — rollover is a filesystem-level, out-of-band operation performed by a human, not an in-app action or admin screen.
- On startup against a data file path that does not exist yet, the app must bootstrap a fresh skeleton (current season + the fixed player list) rather than failing or trying to inherit anything from an old file.
- Once a season's file exists, the running app must only ever read and write that one file — no code path reads from or writes to an archived/previous season's file.
- Every screen (Predict, Leaderboard, Compare) must show only the current season's data throughout; nothing carries over visibly or silently from a prior season.
- Whether the manual archive-and-repoint process is an acceptable long-term operator workflow is an open question the team should keep validating, not something to over-engineer around in this epic.

## Technical Decisions

- One `fantasy-hockey.yml`-shaped file per season (e.g. `fantasy-hockey-2026-27.yml`); multi-season-ness lives at the filesystem/deployment level, not inside the YAML schema — the `season:` field inside a given file stays a single scalar, never a list.
- Rollover procedure: a human archives the current file, then repoints `DATA_FILE` (env var) or `--data-file` (CLI flag, wins over env) at a fresh path; the existing first-run bootstrap path in `internal/store` (used whenever the resolved path doesn't exist) creates the new season's skeleton there — no new bootstrap mechanism is needed for this epic.
- `internal/store` remains the sole reader/writer of the data file, exactly one process at a time, via the existing atomic write-and-rename; this holds unchanged across a rollover.
- No new code path should ever open, read, or reference more than one season's file at a time — reinforces the existing constraint that hand-maintained sections and runtime-written sections (predictions, login codes) are scoped to a single file/season.
- This is filesystem/ops-level behavior riding on already-adopted persistence architecture (data-file resolution, first-run bootstrap, single-writer discipline) — Epic 6 does not introduce new persistence machinery of its own, it exercises the existing one across a rollover.

## Cross-Story Dependencies

- Depends on Epic 1's data-file resolution and first-run bootstrap behavior (data file location via `DATA_FILE`/`--data-file`, auto-creation of a missing file with the initial season + player skeleton) — Epic 6 reuses this path unchanged for the new season's file.
- Independent of Epics 2–5's screens: Predict, Leaderboard, and Compare simply operate against whatever single file the app currently points at; no changes to those features are expected from this epic.
- Sequencing note from later planning: Story 7.4 (making hand-edited `results:`/`award_finalists:` sections resilient to malformed entries) is recommended to land before this epic, since it hardens the same hand-maintained-file safety net that a season rollover exercises.
