# Scoring Rules

Point values for CAP-5 (Scoring & Leaderboard). Season-1 calibration — the PM's own initial values, not an externally validated rule set; may be revisited after a season's real results are in (see SPEC.md Assumptions). Source: PRD FR-24.

| Prediction | Points | Bucket |
|---|---|---|
| Award finalist (each of 3 names) | 5 per name found anywhere in the actual top-3 (ties expand the set) | Regular |
| Team makes the playoffs | 5 | Regular |
| Division winner | 15 (replaces, not adds to, the 5-point mark) | Regular |
| Presidents' Trophy pick | 20 | Regular |
| Cup champion (season-opening pick) | 20 | Regular |
| Playoffs Cup pick | 20 | Playoff |
| Round 1 series | 15 correct winner / 25 exact result (replaces, not additive) | Playoff |
| Round 2 series | 25 / 35 | Playoff |
| Conference Finals series | 30 / 45 | Playoff |
| Stanley Cup Final series | 30 / 50 | Playoff |

All 5 awards (Hart, Norris, Vezina, Art Ross, Rocket Richard) score identically, against whatever is recorded in `fantasy-hockey.yml` for that award — no award is exempt. A required pick left empty at its deadline scores zero for that item (PRD FR-11).

Players with an equal Total share the same rank — no tiebreaker of any kind (PRD FR-23).
