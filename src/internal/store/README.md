# Package: `store`

The data-access layer: owns all reads and writes to the single `fantasy-hockey.yml` data file (AD-9). No other package touches that file directly.

## Responsibilities

- `New(path)` loads `path` into memory, bootstrap-creating it with an empty `players` list and the current default season if it doesn't exist yet (AD-25, AD-26).
- `FindPlayerByEmail` looks up a hand-maintained `Player` by email.
- `CreateLoginCode` appends a new `LoginCode` row - one per issued code, hashed (`code_hash`), never plaintext - and persists it. Existing rows are never mutated or removed.

## Results (read-only)

The hand-maintained `results` and `award_finalists` sections record real-world outcomes for `internal/scoring`. No code path changes them: every save splices only `login_codes`/`predictions` into the originally-parsed raw node tree, leaving every other section's own nodes untouched, so comments, flow style, quoting and key order all survive a save unchanged (spec-7-4) - only a hand-maintained block-style sequence's own indentation isn't guaranteed to match what was typed (`CompactSeqIndent`, see Design notes below). A file without them loads fine and never gains them. The store reads them only at startup: stop the app before editing them by hand and start it again afterwards. An edit made while the app runs is not seen, and the next write still leaves the file holding whatever was parsed at startup, not the mid-run edit.

```yaml
results:
    team_marks:
        atlantic:                       # lowercase Divisions() name
            playoffs: [FLA, TOR, TBL, BOS]
            division_winner: FLA
    presidents_trophy: FLA
    stanley_cup_winner: FLA
    series:
        round1:                         # round1..round4 = r1, r2, cf, scf
            s1: {winner: FLA, games: 5} # key = a playoff_matchups key
award_finalists:
    hart:                               # more than 3 entries on a tie
        - {slug: mcdavid-connor, display_name: Connor McDavid}
```

Read methods, each returning copies under the read lock:

- `Players()` and `PredictionsForPlayer(playerID)`.
- `DivisionResult(division)` takes the capitalised `Divisions()` name and returns the recorded playoff teams and winner.
- `PresidentsTrophyWinner()` and `StanleyCupWinner()`.
- `SeriesResult(seriesKey)` takes the prediction-side key (`r1.s1`); `ok` is false until both winner and games are recorded.
- `RecordedAwardFinalists(award)` returns the finalist slugs.
- `ResultProblems()` lists each malformed entry: an unknown team abbreviation, finalist slug, division, award or round, a series key with no matching `playoff_matchups` entry, games outside 4-7, a series winner that is not one of its matchup's two teams, or a `team_marks` team from another division. The read methods ignore every such entry, so it scores 0. The store only reports them; `main.go` logs each one as a warning at startup.

## File layout

| File                | Contents                                                                                                                                                                             |
|---------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `store.go`          | `Store`, `New` and bootstrap, player/login-code reads and writes, the persisted series-key and round-set vocabulary.                                                                 |
| `store_results.go`  | The read-only `results`/`award_finalists` reads (`DivisionResult`, trophy winners, `SeriesResult`, `RecordedAwardFinalists`) and `ResultProblems`'s malformed/unknown-key detection. |
| `store_splice.go`   | The raw `*yaml.Node` splice/rollback mechanics (`spliceNamedValueLocked`) and the lenient-section line-range helpers a tolerated shape error is confined against.                    |

Each file has a matching `*_test.go`. General test fixtures shared package-wide (`newSeededStore`, `resultsFixtureBase`, `resultsFixtureResults`, `resultsFixtureMatchups`) live in `store_results_test.go`, even though `store_test.go` and `store_splice_test.go` build on them too. A test is grouped with whichever concern its outcome actually surfaces, not with the file whose method it happens to call first.

## Design notes

- `Store`'s in-memory document and mutex stay unexported; every access goes through an exported method (AD-29).
- Every write splices only `login_codes`/`predictions` into the originally-parsed raw node tree and atomically replaces the file on disk via write-to-temp-file-then-rename (AD-27) - every other hand-maintained section (`players`, `prediction_sets`, `teams`, `nhl_players`, `playoff_matchups`, `results`, `award_finalists`) round-trips through the exact node objects it was parsed into, never reconstructed from typed fields (spec-7-4, AD-23) - except a block-style sequence's own indentation, which the `CompactSeqIndent` bullet below re-derives from the encoder's own settings. A new app-writable field needs its own splice, not just a struct field - `document`'s own doc comment (`store.go`) spells out exactly what that involves.
- `Player`/`LoginCode` are the canonical structs for these entities; other packages import and use them as-is (AD-24).
- The store owns the persisted series-key format (`JoinSeriesKey`/`SplitSeriesKey`, `"<setID>.<matchupKey>"`), the playoff round-set ids (`Round1SetID`, `Round2SetID`, `ConferenceFinalsSetID`, `StanleyCupFinalSetID`) and the division vocabulary (`Divisions()`), so feature packages never redefine them (AD-24).
- Every write encodes with `yaml.NewEncoder(...).CompactSeqIndent()`, not plain `yaml.Marshal`, so a sequence nested inside a mapping-inside-a-list (e.g. `Prediction.TeamIDs`/`FinalistSlugs`) gets the same relative indent as a top-level sequence - a plain marshal's inconsistent indent otherwise fails the repo's yamllint gate. `yamllint_test.go` proves this against the real `yamllint` binary/config (skips without a reachable Docker daemon; CI always runs it for real).
