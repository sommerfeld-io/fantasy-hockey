# Package: `scoring`

The only home of point calculation. `PlayerPoints(st, playerID)` turns one player's saved picks into `Points{Regular, Playoff}` (with `Total()`) by comparing them against the hand-recorded results that `internal/store` loads from `fantasy-hockey.yml`.

## Rules

The point values live in one block at the top of `scoring.go` (the constants plus `seriesPoints`), so a season's recalibration means editing only that block.

| Prediction                         | Points                                                    | Bucket  |
|------------------------------------|-----------------------------------------------------------|---------|
| Award finalist (each of 3 names)   | 5 per name found anywhere in the recorded finalists       | Regular |
| Team makes the playoffs            | 5                                                         | Regular |
| Division winner                    | 15 (replaces, does not add to, the 5-point mark)          | Regular |
| Presidents' Trophy pick            | 20                                                        | Regular |
| Cup champion (season-opening pick) | 20                                                        | Regular |
| Playoffs Cup pick                  | 20                                                        | Playoff |
| Round 1 series                     | 15 correct winner / 25 exact result (replaces, not added) | Playoff |
| Round 2 series                     | 25 / 35                                                   | Playoff |
| Conference Finals series           | 30 / 45                                                   | Playoff |
| Stanley Cup Final series           | 30 / 50                                                   | Playoff |

- Picks are matched by team abbreviation or NHL player slug only, never by display name.
- A recorded award finalist list can hold more than 3 slugs when there is a tie; every pick found in it scores.
- A recorded division winner counts as having made the playoffs even if the hand-maintained `playoffs` list leaves it out.
- An empty pick, a result not recorded yet, a half-recorded series, a malformed result or a player not in the players list all score 0. Scoring never returns an error.

## Design notes

- Every call recomputes from the store's read methods. Nothing is cached and nothing is ever written to the data file.
- `scoring` imports `internal/store` and no other `internal/` package (enforced by `imports_test.go`).
- Ranking and the Leaderboard are not part of this package.
