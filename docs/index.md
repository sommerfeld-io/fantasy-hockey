<!--
  This file is the single source of truth for the repo-root README.md.
  `task docs:readme` (part of `task lint`) copies it over README.md verbatim.
  Edit this file, not README.md - a direct edit there is overwritten on the
  next `task lint`. See docs/architecture.md#keeping-the-docs-and-readme-in-sync.

  Because this file is copied byte-for-byte to the repo root, a link to
  another docs/ page can't resolve correctly from both locations at once
  (root-relative works in README.md but not here; docs/-relative is the
  reverse). So this file names other docs/ pages in plain text/code spans
  instead of linking to them - see "Documentation" and "Recording results
  and playoffs" below.
-->

# Fantasy Hockey

NHL Fantasy Hockey Game - Web-based successor to the spreadsheet-based manual solution.

<!-- ===== START status badge ===== -->

[![Pipeline: Commit + Test](https://github.com/sommerfeld-io/fantasy-hockey/actions/workflows/pipeline.yml/badge.svg)](https://github.com/sommerfeld-io/fantasy-hockey/actions/workflows/pipeline.yml)
[![Pipeline: Release](https://github.com/sommerfeld-io/fantasy-hockey/actions/workflows/release.yml/badge.svg)](https://github.com/sommerfeld-io/fantasy-hockey/actions/workflows/release.yml)

<!-- ===== END status badge ===== -->

- [Github Repository](https://github.com/sommerfeld-io/fantasy-hockey)
- [Project on DockerHub](https://hub.docker.com/r/sommerfeldio/fantasy-hockey)
- [Sonarcloud Code Quality and Security Analysis](https://sonarcloud.io/project/overview?id=sommerfeld-io_fantasy-hockey)
- [Where to file issues](https://github.com/sommerfeld-io/fantasy-hockey/issues)

## About

Fantasy Hockey is a private, season-long NHL prediction pool for a small, fixed group of friends, delivered as a mobile-first, dark-themed web app. It replaces a manually maintained spreadsheet whose hand-calculated scoring and lack of deadline or name-typo enforcement had already caused a real, uncorrected scoring error in a past season.

Each player predicts outcomes before the season starts (Stanley Cup winner, Presidents' Trophy, playoff field, division winners, individual awards) and throughout the playoffs (a re-pick and per-series winner/length calls), all against fixed deadlines. The app scores every prediction automatically the moment results are recorded, and keeps a live leaderboard so nobody has to do the math — or gets it wrong — by hand again. Players can also compare their picks against everyone else's, set by set.

There is no registration, invite flow, or admin UI: the player list, NHL schedule data, deadlines, and results are all maintained directly in a single data file by whoever is already running the pool, out of band. The app's only job is to get the player-facing loop right — predict, wait, get scored, compare, gloat — reliably, every season.

## Documentation

Full documentation lives in the `docs/` folder of this repository: game rules (`docs/game-rules.md`, also viewable in-app under the Rules tab), the operator guide (`docs/operator-guide.md`), the results/playoffs runbook (`docs/recording-results-and-playoffs.md`), and architecture & development (`docs/architecture.md`).

## Usage

Fantasy Hockey ships as a single container image with no database — all state lives in one YAML data file.

```bash
docker run -d \
  --name fantasy-hockey \
  -p 8080:8080 \
  -e SESSION_SECRET=<a-long-random-secret> \
  -e DATA_FILE=/data/fantasy-hockey.yml \
  -v fantasy-hockey-data:/data \
  sommerfeldio/fantasy-hockey:latest
```

A player logs in with just their email address: the app emails a one-time 6-digit code, valid for 10 minutes, and a session then stays active while in use and expires after 30 minutes of inactivity. A player can log out at any time from the app shell's header, which ends their session immediately on that device.

If none of the `SMTP_*` variables below are set, the app still starts, but login-code emails silently fail to send (an error is logged server-side; the player never receives a code) — set at least `SMTP_HOST`/`SMTP_PORT` in any real deployment.

Once logged in, the bottom nav's **Rules** tab explains how to play and how scoring works — the same content also lives in `docs/game-rules.md`.

### Configuration

| Variable            | Required | Purpose                                                                                 |
| ------------------- | -------- | --------------------------------------------------------------------------------------- |
| `SESSION_SECRET`    | Yes      | Signing key for session cookies; the app refuses to start without it                    |
| `DATA_FILE`         | No       | Path to the data file (default `fantasy-hockey.yml`); a `--data-file` flag overrides it |
| `SMTP_HOST`         | No       | Outbound mail server for login-code emails                                              |
| `SMTP_PORT`         | No       | Outbound mail server port                                                               |
| `SMTP_USERNAME`     | No       | SMTP username; omit for an unauthenticated local mail catcher                           |
| `SMTP_APP_PASSWORD` | No       | SMTP password (e.g. a Gmail App Password)                                               |

The `--port`/`-p` flag (default `8080`) sets the listening port. On first run, if the data file doesn't exist yet, the app creates it with an empty player list — the pool's actual players are then added by hand-editing that file, since there is no in-app way to create an account. With the named-volume setup above, reach the file with `docker cp fantasy-hockey:/data/fantasy-hockey.yml .`, edit it, then copy it back with `docker cp ./fantasy-hockey.yml fantasy-hockey:/data/fantasy-hockey.yml`.

### Recording results and playoffs

Results, playoff matchups and the opening of each playoff round are also recorded by hand in the data file. Follow the operator guide (`docs/operator-guide.md`) for what to do and when, and the results and playoffs runbook (`docs/recording-results-and-playoffs.md`) for the exact YAML shape and how to read the startup warnings.

## Licensing

The application source code is released under the [MIT License](https://github.com/sommerfeld-io/fantasy-hockey/blob/main/LICENSE.md).

## Contact

Feel free to contact me via <sebastian@sommerfeld.io>.
