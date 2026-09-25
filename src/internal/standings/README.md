# Package: `standings`

Turns `internal/scoring`'s points into the Leaderboard's ranked rows. `Rows(st *store.Store) []Row` returns one `Row{Rank, PlayerID, Name, Points, Leader}` for every player in `players:`, including players with no picks, who score 0.

## Rules

- Rows are sorted by Total (Regular + Playoff) descending.
- Equal Totals share a rank, with no tiebreaker of any kind. Ranking is competition ranking, so the next rank skips accordingly (60, 60, 45 ranks 1, 1, 3).
- Tied players are listed alphabetically by name, ignoring case. That order is for display only and never changes a rank.
- Every rank-1 player is a leader, but only while the top Total is above 0. Before anything scores, nobody is a leader.

## Design notes

- Every point value comes from `scoring.PlayerPoints`. This package never derives a point value itself.
- It imports only `internal/scoring` and `internal/store`, the one feature-to-feature import the architecture allows. `imports_test.go` guards this.
- Nothing is cached or persisted. Every call recomputes from the store, so a new pick or a reloaded result shows up on the next call.
