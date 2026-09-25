# Package: `store`

The data-access layer: owns all reads and writes to the single `fantasy-hockey.yml` data file (AD-9). No other package touches that file directly.

## Responsibilities

- `New(path)` loads `path` into memory, bootstrap-creating it with an empty `players` list and the current default season if it doesn't exist yet (AD-25, AD-26).
- `FindPlayerByEmail` looks up a hand-maintained `Player` by email.
- `CreateLoginCode` appends a new `LoginCode` row - one per issued code, hashed (`code_hash`), never plaintext - and persists it. Existing rows are never mutated or removed.

## Results (read-only)

The hand-maintained `results` and `award_finalists` sections record real-world outcomes for `internal/scoring`. No code path changes them, but every save re-marshals the whole document: their values are preserved, while comments, flow style and quoting are not. A file without them loads fine and never gains them.

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

## Design notes

- `Store`'s in-memory document and mutex stay unexported; every access goes through an exported method (AD-29).
- Every write serializes the whole in-memory document and atomically replaces the file on disk via write-to-temp-file-then-rename (AD-27).
- `Player`/`LoginCode` are the canonical structs for these entities; other packages import and use them as-is (AD-24).
- The store owns the persisted series-key format (`JoinSeriesKey`/`SplitSeriesKey`, `"<setID>.<matchupKey>"`), the playoff round-set ids (`Round1SetID`, `Round2SetID`, `ConferenceFinalsSetID`, `StanleyCupFinalSetID`) and the division vocabulary (`Divisions()`), so feature packages never redefine them (AD-24).
