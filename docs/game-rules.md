<!--
  This file is the single source of truth for the in-app Rules page.
  `task docs:embed-game-rules` (part of `task lint`) copies it into
  src/internal/web/rules/game-rules.md, since go:embed can't reach outside
  its own package's directory tree. Edit this file, not the copy under
  src/ - a direct edit there is overwritten on the next `task lint`.
  See docs/architecture.md#keeping-the-docs-and-readme-in-sync.

  Avoid links in this file: it's rendered standalone inside the app (the
  Rules tab), where a link to another docs/ page has nothing to resolve
  against.
-->

# Game Rules

Fantasy Hockey is a season-long NHL prediction pool. You predict outcomes before the season and throughout the Playoffs, the app scores every pick automatically the moment a real-world result is recorded, and the Leaderboard always shows the live standings.

There's no way to lose points for something you didn't predict: an empty pick simply scores zero for that one item and never blocks anything else.

## The two phases

Every prediction belongs to exactly one **Prediction set**, and every Prediction set belongs to one of two phases, shown as two sections on the Predict tab:

- **Before the season** — four sets, all due before the regular season starts.
- **Playoffs** — five sets, opened one at a time as the real playoffs unfold.

A set's status is always exactly one of:

| Status        | Meaning                                                             |
| ------------- | ------------------------------------------------------------------- |
| **Open**      | Deadline hasn't passed, nothing saved yet                           |
| **Submitted** | Picks saved, deadline hasn't passed — still editable                |
| **Closed**    | Deadline passed — read-only, no exceptions, not even for you        |
| **Upcoming**  | Not available yet — a later Playoff round whose matchups aren't set |

Until a set's deadline passes, save and resave as often as you like — only the last save before the deadline counts. Every field in a set (except a Playoff series pick, see below) saves independently, so you can fill in what you know now and come back for the rest later.

## Before the season

Made once, before the regular season starts:

- **Cup champion** — your Stanley Cup winner pick, from all 32 teams.
- **Presidents' Trophy** — your pick for the best regular-season record, from all 32 teams.
- **Division picks** — for each of the 4 divisions (Atlantic, Metropolitan, Central, Pacific): which teams make the Playoffs from that division, and who wins it. Each conference (8 teams across its 2 divisions) must add up to exactly 8 across a 4/4 or 5/3 split.
- **Player awards** — 3 finalists each for Hart (MVP), Norris (best defenseman), Vezina (best goalie), Art Ross (points leader) and Rocket Richard (goals leader), picked from the season's known NHL player list. You can skip an award entirely, but if you pick any finalist for an award you must pick all 3 for it.

## Playoffs

Opened as the real playoffs unfold — see the operator guide for exactly when each one unlocks:

- **Playoffs Cup pick** — re-pick the Stanley Cup winner once the Playoff field is known, separate from (and scored separately from) your season-opening Cup champion pick.
- **Round 1, Round 2, Conference Finals, Stanley Cup Final** — for each series in an unlocked round, pick the winning team and the exact game count (4, 5, 6 or 7). Winner and game count save together as one pick — you can't save one without the other.

A round stays **Upcoming** until its real matchups are recorded, which only happens once the previous round is actually decided.

## Scoring

Every pick scores automatically the moment its result is recorded — nothing to calculate by hand, nothing to get wrong. Points split into two buckets, **Regular** (before-the-season picks) and **Playoff** (Playoffs picks); the Leaderboard's **Total** is Regular + Playoff, and that's what you're ranked by.

| Prediction                         | Points                                                            | Bucket  |
| ---------------------------------- | ----------------------------------------------------------------- | ------- |
| Award finalist (each of 3 names)   | 5 per name found anywhere in the real top 3 (ties expand the set) | Regular |
| Team makes the Playoffs            | 5                                                                 | Regular |
| Division winner                    | 15 (replaces, doesn't add to, the 5-point playoff-team mark)      | Regular |
| Presidents' Trophy pick            | 20                                                                | Regular |
| Cup champion (season-opening pick) | 20                                                                | Regular |
| Playoffs Cup pick                  | 20                                                                | Playoff |
| Round 1 series                     | 15 correct winner / 25 exact result (replaces, not additive)      | Playoff |
| Round 2 series                     | 25 / 35                                                           | Playoff |
| Conference Finals series           | 30 / 45                                                           | Playoff |
| Stanley Cup Final series           | 30 / 50                                                           | Playoff |

A few things worth knowing:

- All 5 awards score identically — no award is worth more than another.
- A correct division-winner pick replaces the 5-point playoff-team mark for that same team; you never get both for the same team.
- An exact series result (winner + right game count) replaces the winner-only value — the two never add together.
- Players with an equal Total share the same rank. There is no tiebreaker of any kind.

Worked examples:

- You pick FLA as division winner and FLA also makes the Playoffs (as it must, if it wins the division): you get **15**, not 15 + 5. Every other correctly picked playoff team in that division still earns its own 5.
- You pick the Round 1 winner correctly but the wrong game count: **15**. You pick both correctly: **25**, not 15 + 25.
- Your three Hart finalists are McDavid, MacKinnon and Kucherov. The real top 3 turns out to be McDavid, MacKinnon and someone else: you score **10** (2 of 3 matched) — nothing for the third.

## Compare

The Compare tab shows every player's picks for one chosen Prediction set, side by side, with your own column visually marked. It's always visible — there's no deadline-based hiding of anyone's picks, before or after a deadline.

## Behind the scenes

Results, deadlines and Playoff matchups are all maintained by hand by whoever runs the pool — there's no admin screen anywhere in the app. See the operator guide (`docs/operator-guide.md` in the repository) if that's you.
