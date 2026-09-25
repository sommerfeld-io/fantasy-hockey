# Recording Results and Playoffs

This runbook is for whoever runs the pool. It covers the hand edits to the data file (`fantasy-hockey.yml`) that turn on scoring and the playoff rounds:

- the real-world results the app scores picks against;
- the playoff matchups for each round;
- the `upcoming` flags that open the playoff Prediction Sets.

There is no admin screen. Every one of these edits is made by hand in the data file.

## Before you edit: stop the app

The app reads the data file once, at startup. Always follow these steps:

1. Stop the app (for example `docker stop fantasy-hockey`).
2. Copy the data file out and keep an untouched backup copy next to it (for example `fantasy-hockey.yml.bak`). A wrongly shaped entry stops the app from starting, and the backup gets it running again. With the named-volume setup from the [README](../README.md), copy the file out with `docker cp fantasy-hockey:/data/fantasy-hockey.yml .`. `docker cp` works on a stopped container.
3. Edit the file, then copy it back with `docker cp ./fantasy-hockey.yml fantasy-hockey:/data/fantasy-hockey.yml`.
4. Start the app again (for example `docker start fantasy-hockey`).
5. Check the startup log for warnings (see [Reading the startup warnings](#reading-the-startup-warnings)).

Don't edit the file while the app runs. The app doesn't see the change, and the next time any player saves a pick or requests a login code, the app writes the file back from the copy it loaded at startup. That silently overwrites your edit.

To correct a result you recorded wrongly, follow the same steps: fix the value and restart. The app stores no scores. The Leaderboard recomputes every player's points from the data file on each request, so the correction shows up straight after the restart.

## The app rewrites the whole file

Every save (a pick or a login code) rewrites the entire data file from memory. The values you recorded are kept, but the file's formatting isn't:

- comments are dropped, including any you add while following this runbook;
- `{ ... }` flow style becomes block style;
- quoting changes, for example `games: 5` becomes `games: "5"`;
- keys the app doesn't know (for example a misspelled one) are dropped.

So don't rely on comments in the data file to keep notes. Keep them somewhere else.

## Results and award finalists

Results live in a top-level `results:` section and award finalists in a top-level `award_finalists:` section. Neither exists in a fresh data file. Add them once, then fill them in over the season. Every part is optional: record each result when it's known, and leave the rest out. What each correct pick is worth is in the [point table](../_bmad-output/specs/spec-fantasy-hockey/scoring-rules.md).

The example's `series` entry only loads without a warning once `playoff_matchups.r1` has a matchup with key `s1`. Record the matchups first (see [Playoff matchups](#playoff-matchups)), or leave `series:` out until the round is played.

```yaml
results:
    team_marks:
        atlantic:                       # lowercase division: atlantic, metropolitan, central, pacific
            playoffs: [FLA, TOR, TBL, BOS]
            division_winner: FLA
    presidents_trophy: FLA
    stanley_cup_winner: FLA
    series:
        round1:                         # round1, round2, round3, round4 = r1, r2, cf, scf
            s1: {winner: FLA, games: 5} # s1 = the key of a playoff_matchups.r1 entry
award_finalists:
    hart:                               # hart, norris, vezina, art_ross, rocket_richard
        - {slug: mcdavid-connor, display_name: Connor McDavid}
        - {slug: mackinnon-nathan, display_name: Nathan MacKinnon}
        - {slug: kucherov-nikita, display_name: Nikita Kucherov}
```

The rules for each part:

| Entry                                   | What to write                                                                                                                  |
|-----------------------------------------|--------------------------------------------------------------------------------------------------------------------------------|
| `team_marks.<division>.playoffs`        | A list of the team abbreviations (the `teams:` `id` values) from that division that made the playoffs                          |
| `team_marks.<division>.division_winner` | The abbreviation of the team that won that division                                                                            |
| `presidents_trophy`                     | The abbreviation of the Presidents' Trophy team                                                                                |
| `stanley_cup_winner`                    | The abbreviation of the Stanley Cup champion. Both the season-opening Cup pick and the Playoffs Cup pick are scored against it |
| `series.<round>.<key>`                  | `winner` (one of the matchup's two teams) and `games` (4 to 7). A series only scores once both are recorded                    |
| `award_finalists.<award>`               | One `{slug, display_name}` entry per finalist. The `slug` must be in `nhl_players:`. List more than three when there is a tie  |

Keep these points in mind:

- The series section is nested: the round (`round1`), then the matchup key (`s1`). A flat `round1.s1:` key doesn't load and stops the app from starting.
- The round names under `results.series` (`round1` to `round4`) are not the Prediction Set ids used under `playoff_matchups` (`r1`, `r2`, `cf`, `scf`). `round1` is `r1`, `round2` is `r2`, `round3` is `cf` and `round4` is `scf`.
- A finalist's `slug` must already be listed under the top-level `nhl_players:` section. To add a player there, append an entry with a `slug` (lowercase `lastname-firstname`), the `display_name` and the `position` (`skater`, `defenseman` or `goalie`). Once a slug is added, never change it: saved award picks refer to it.

    ```yaml
    nhl_players:
        - slug: mcdavid-connor
          display_name: Connor McDavid
          position: skater
    ```

- Only the finalist `slug` is compared. Still write the `display_name`: an entry without one gains an empty `display_name: ""` on the next save.

## Playoff matchups

Each playoff round's series are listed under the top-level `playoff_matchups:` section, keyed by the round's Prediction Set id. A fresh data file has an empty `playoff_matchups: {}` line. Replace that line; don't add a second `playoff_matchups:` key, because a duplicate key stops the app from starting.

```yaml
playoff_matchups:
    r1:
        - {key: s1, a: FLA, b: TOR}
        - {key: s2, a: TBL, b: BOS}
    r2:
        - {key: s1, a: FLA, b: TBL}
```

- `a` and `b` are the two teams' abbreviations.
- `key` identifies the series. It must be unique within its round, and it must never change once players have picked, because each saved series pick and each `results.series` entry is matched to its matchup by this key. Reordering the list is safe; renaming a key isn't.
- The same key (for example `s1`) may be reused in different rounds, since a pick is stored as the round plus the key (`r1.s1`, `r2.s1`).

## Opening the playoff Prediction Sets

Each playoff Prediction Set is listed under `prediction_sets:` with an `upcoming` flag. An upcoming set shows as locked on the Predict screen. The sets behave differently:

| Set id       | What opens it                                                                                | When                                                           |
|--------------|----------------------------------------------------------------------------------------------|----------------------------------------------------------------|
| `playoffcup` | Set its `upcoming` to `false` by hand                                                        | When the playoff field is set                                  |
| `r1`         | Set its `upcoming` to `false` by hand. Recording `playoff_matchups.r1` alone doesn't open it | After you've recorded all `playoff_matchups.r1` entries        |
| `r2`         | Recording at least one `playoff_matchups.r2` entry. Its `upcoming` flag is ignored           | Once round 1 ends and the round 2 matchups are known           |
| `cf`         | Recording at least one `playoff_matchups.cf` entry. Its `upcoming` flag is ignored           | Once round 2 ends and the conference finals matchups are known |
| `scf`        | Recording at least one `playoff_matchups.scf` entry. Its `upcoming` flag is ignored          | Once the finalists are known                                   |

Record every matchup of a round in the same edit. `r2`, `cf` and `scf` open as soon as they have one matchup, and players can only pick the series that are listed.

Each set's `deadline_utc` still applies: an open set closes at its deadline. Check that the playoff deadlines match the real schedule when you open a set.

## Reading the startup warnings

At startup the app checks the `results:` and `award_finalists:` sections and logs one warning for each value it can't use:

```text
2026/09/25 12:17:24 WARN malformed result in data file file=/data/fantasy-hockey.yml problem="results.team_marks.atlantic.division_winner: unknown team \"XXX\""
```

The `problem` names the entry by its path in the file, then says what is wrong. The app still starts, and it ignores that one value, so it scores nothing. Fix it by hand (stop, edit, start) and check that the warning is gone. It warns about:

- an unknown team abbreviation, finalist slug, division, award or round;
- a division key that isn't lowercase (`Atlantic` instead of `atlantic`);
- a `team_marks` team that plays in another division;
- a series key with no matching `playoff_matchups` entry for that round;
- a series winner that isn't one of the matchup's two teams;
- `games` outside 4 to 7.

These mistakes are not warned about:

- A misspelled field name, for example `stanley_cup_winer:` or `divison_winner:`, is silently ignored, so that result scores nothing. The next save then drops it from the file.
- A wrongly shaped entry stops the app from starting. For example, a single team where a list belongs (`playoffs: FLA`), a flat `round1.s1:` series key or a duplicate top-level key. The app exits with a `yaml: unmarshal errors` message that names the line.
- A half-recorded series (a `winner` without `games`, or the reverse) scores nothing until both are recorded.
- Fewer than three finalists, or the same finalist listed twice, is accepted as written.

A clean startup logs no warnings at all. No warnings means none of the warned-about problems above was found. It doesn't rule out the mistakes that aren't warned about, so check those by eye after each edit.
