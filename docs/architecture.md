# Architecture & Development

Technical reference for anyone building or deploying Fantasy Hockey. For game rules see [game-rules.md](game-rules.md); for season operations see the [operator guide](operator-guide.md).

## Stack

| Component           | Choice                                                                                         |
| ------------------- | ---------------------------------------------------------------------------------------------- |
| Language            | Go 1.26.6                                                                                      |
| HTTP & routing      | stdlib `net/http` (`ServeMux`) — no third-party router                                         |
| Views               | stdlib `html/template`, server-rendered — no JS framework, no SPA                              |
| Data storage        | One hand-editable YAML file (`fantasy-hockey.yml`), `go.yaml.in/yaml/v3` — no database, no ORM |
| Sessions            | Stateless, HMAC-signed cookies — no server-side session store                                  |
| Outbound email      | stdlib `net/smtp`, config via env vars — no transactional-email SaaS                           |
| Acceptance tests    | `cucumber/godog`, Gherkin `.feature` files                                                     |
| Build orchestration | [Task](https://taskfile.dev)                                                                   |
| Container image     | Multi-stage Alpine build, arm64 target (Raspberry Pi)                                          |

No database driver, migration tool, or client-side JS framework — the app is intentionally as boring and dependency-light as a 3-person hobby project can be.

## Layered architecture

One dependency direction: presentation (`internal/web`) depends on domain packages (`internal/auth`, `internal/scoring`, `internal/standings`), and any layer may depend directly on the infrastructure packages (`internal/store`, `internal/mailer`, `internal/clock`), which depend on nothing above them. The only import between two domain packages is `internal/standings` → `internal/scoring` (Leaderboard reuses the scoring engine's point math instead of re-deriving it) — every other cross-package dependency is either presentation-to-domain or a layer reaching straight to an infrastructure package (e.g. both `internal/web` and `internal/auth` use `internal/mailer` directly).

![Layered architecture](https://kroki.io/mermaid/svg/eNp9kcFugzAMhu99iohTK43yAlVPO-ywSZXGDfVggkuihRgl6SrevgaXjQ5pHJD84--zE9oAvVHl60bx04H1VTa-9y0d6lActzcbrG8VeTfssrPK86OKGL4xVJn1CYMHV0gg_W9leVI1UYqJzYxMYumY6BvWC5SrBWfANw5DfFEmda5I2PUOEs4Wbp4UcE1m4RhLkThqrVeaGmRFxBgt-fiXjprGIy33l0QcPXGuNDh95dksWPGJt-T2uDTMmTjeERoMNUFoZnpc8oFTwCeUa8Eu4BPEITekv3DYD51b0fxv3NPdSyD850d5-rlxOdPvSEnnPf_PhX184Z4pvfCgaputl8x2580dZuy-tw==)

| Package              | Responsibility                                                                                                                     |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `internal/web`       | HTTP handlers, `html/template` views, the two sanctioned vanilla-JS widgets (award-finalist autocomplete, live division-pick caps) |
| `internal/auth`      | Login-code issuance/validation, session cookies                                                                                    |
| `internal/scoring`   | The point table and every point calculation — the only home of scoring logic                                                       |
| `internal/standings` | Live Leaderboard computation (Regular/Playoff/Total), reads `internal/scoring`                                                     |
| `internal/store`     | The only package that reads or writes `fantasy-hockey.yml`                                                                         |
| `internal/mailer`    | SMTP delivery, used by `internal/auth` and `internal/web`                                                                          |
| `internal/clock`     | Current time, RFC3339 formatting                                                                                                   |
| `internal/server`    | HTTP bootstrap: port resolution, graceful shutdown — no business logic                                                             |
| `main.go`            | Wiring only: config, opens the data file, starts the server                                                                        |

## Data model

All persisted state lives in one YAML file, `fantasy-hockey.yml` (path via `DATA_FILE` env var or `--data-file` flag). The store loads it once at startup and holds it in memory behind a mutex; every write re-serializes the whole file via write-to-temp-file-then-rename, so a crash mid-write never corrupts it. Only `predictions:` and `login_codes:` are ever written by the app itself — every other section (`players:`, `teams:`, `prediction_sets:`, `nhl_players:`, `playoff_matchups:`, `results:`, `award_finalists:`) is maintained entirely by hand and carried over byte-faithfully (comments, formatting, key order) on every save.

IDs follow one rule: anything the app creates at runtime (a `Prediction`, a `LoginCode`) gets an application-generated UUID; anything a human types gets a short, human-readable key instead — a team abbreviation (`TOR`), a Prediction Set id (`r1`), an NHL Player slug (`mcdavid-connor`). Nothing a human hand-edits is ever a UUID, and once assigned a slug is never regenerated.

See [`src/internal/store/README.md`](../src/internal/store/README.md) for the exact field-level shape, and the [operator guide](operator-guide.md) / [recording results and playoffs](recording-results-and-playoffs.md) runbook for how to edit it safely.

**Why no database, no SPA, no ORM:** the pool is a handful of fixed players for one friend group — the whole point of this rebuild was replacing a spreadsheet's manual scoring, not building infrastructure. A single YAML file that's easy to read, diff and hand-edit is a feature here, not a limitation, and server-rendered HTML keeps the app inspectable end-to-end in one language with no separate frontend build step.

## Deployment

One container, no database container, no separate storage container. The data file lives on a bind mount or named volume outside the container's writable layer.

![Deployment topology](https://kroki.io/mermaid/svg/eNpVUMFqwzAMve8rhE8JLNll7DDGYLDRHjoobW6hBztRG1PbCrbTNn8_xYZ1E_gg6b2n93zychxgs3sArk63YqXjelLw0UVNLrwp__Tu0aAMCFfy56OhqzhAVYHoqTujh3EKg-DBOwyTagvxmccskshFhtWaSlEe0hnGQVWD0FaekPnGCKiZf0WV9qOn29yKHV7Q89nUZi1HEWaMEPhNY7kYYZ1102wfwRE0m_1dKUmFSZ1SwoFCZEkZRoXez7DVUEhvX57LtBLZ2VLMbcVRuijDXA2L-Rk64lY79H9wFzKc9j-wnq3JTpV2fWVpchF7IA_kQC6UyeLvP6Drs0s-mT6Q9_eeg-2_my0UnL9_hZWV2pQ5XrBx5Ntft4jeSQMLbhH9AdaXjE4=)

The app itself serves plain HTTP; TLS termination is a reverse proxy's job, set up outside the app. No in-app alerting exists anywhere — failures are visible only through the app's own structured logs.

## Development process

- Everything is run through [Task](https://taskfile.dev). The root `taskfile.yml` delegates Go-specific tasks to `src/taskfile.yml` under the `go:` namespace.
- Follow **TDD** for all Go code: red (failing test), green (minimum implementation), refactor. Test observable behavior, not internal state.
- Follow **BDD** for every user-visible feature: write or update a Gherkin `.feature` file under `src/acceptance-tests/features/` before writing implementation code. It must fail (red) before the feature exists and pass (green) once it's done.
- Local dev runs in a devcontainer (`.devcontainer/`), with `mailpit` as a disposable local SMTP sink (web UI at `http://localhost:8025`) so login-code emails never reach a real inbox during development.

Common commands (see the [README's Usage section](../README.md#usage) for `docker run` details):

| Command                   | What it does                                                                              |
| ------------------------- | ----------------------------------------------------------------------------------------- |
| `task go:run`             | Build and run the binary locally against a local data file                                |
| `task go:test`            | Unit tests with a coverage report (`coverage.out`)                                        |
| `task go:test:acceptance` | GoDog acceptance tests with end-to-end internal-package coverage                          |
| `task go:build`           | Full pipeline: lint, vet, test, acceptance test, complexity, licenses, vulncheck, compile |
| `task lint`               | Project-wide linters: YAML, GitHub workflows, filenames, folders, Gherkin, Markdown links |
| `task docker:build`       | Lint, test and build the actual container image (the authoritative pre-merge check)       |
| `task docker:run`         | Build and run the app in Docker, alongside `mailpit`                                      |

A non-zero exit from `task go:run` caused solely by a `govulncheck` finding is acceptable (with a fix attempted first when one exists) — everything else must be green before a change is considered done.

Feature work is planned and tracked as epics and stories under `_bmad-output/`: `_bmad-output/planning-artifacts/` holds the PRD, architecture spine, UX design contract and epic/story breakdown; `_bmad-output/implementation-artifacts/` holds `sprint-status.yaml` (what's done/in-progress/backlog) and per-epic context/retrospective notes. This trail is the living record of *why* something was built a certain way — check it before assuming a design choice was arbitrary.

## CI/CD and release process

Every push and pull request against `main` runs the **Pipeline: Commit + Test** workflow: ShellCheck, the project-wide linters, a Dockerfile lint, a SonarQube scan, then a build of the container image (linux/amd64 + linux/arm64) with an SBOM and provenance attached, a Docker Scout vulnerability compare, and — on `main` only — a push of the image tagged `:edge` to Docker Hub.

Releases use [semantic-release](https://semantic-release.gitbook.io/), driven entirely by [Conventional Commits](https://www.conventionalcommits.org/) messages (`fix`, `feat`, `BREAKING CHANGE` footer — see the project's `CLAUDE.md`). A qualifying commit reaching `main` creates a GitHub Release automatically; that triggers the **Pipeline: Release** workflow, which re-tags the already-built `:edge` image as the new version and `:latest` on Docker Hub, generates and attaches an SPDX SBOM to the release, and runs a final CVE scan.

![CI and release flow](https://kroki.io/mermaid/svg/eNp9UctOwzAQvPMVKx8QiD7uPSCVUrUVUEqKEJD0sHE2iUXiVLFT-HzWjlsQB3JwtI-ZnZ3Nq-ZTlthauI_OgL99Z8pYbPiFMWwisA3UqLTYwXB4DbKpa2W5rvZUKU0TmPkMXMEzGZvoC87aAXTa51BK2lvUksBy2QwSvW00tk9dSgNIO1VlcA5GogZVY0GXYudF9GP8RMoKisXKVcFiUVAGE5dLtFPKEQu8beQHtbDs0oB3DR5tqG6pigX_UVslhxwRGofG1pBh-fpAXGFVVdjFBJIeyzQgckI7ztXX-CaaT-9W6wXMltP1Yp7ovOl0Bka5FSs01mkUfnRRhlmxWCjL2iDqY5At01HGZh1eR2-jd4c5bn5CeY4Tw4_fgSS0_27u3fnrVT8BWeSkQutv9I9xAeWtS5s6Ftubxwc-5Oxl7s-UaLQWZYDbko4KGP8NNQzAkQ==)

The image is published to [DockerHub](https://hub.docker.com/r/sommerfeldio/fantasy-hockey), and this repository's `README.md` becomes that image's DockerHub description automatically on every release — which is also why `README.md` is generated from [`docs/index.md`](index.md) rather than edited by hand (see below).

## Keeping the docs and README in sync

[`docs/index.md`](index.md) is the single source of truth for the repo-root `README.md`. `task docs:readme` (part of `task lint`) copies it over `README.md` verbatim, so edit `docs/index.md`, never `README.md` directly — a direct edit is overwritten on the next `task lint`.

The in-app Rules page works the same way: [`game-rules.md`](game-rules.md) here is the canonical file. `task docs:embed-game-rules` (also part of `task lint`) copies it into [`src/internal/web/rules/game-rules.md`](../src/internal/web/rules/game-rules.md), since `go:embed` can only reach files inside its own package's directory tree, not `docs/`. Edit the copy in `docs/`, not the one under `src/`. That embedded copy is committed to git, so neither the Dockerfile nor `task go:build` needs `docs/` copied into the container or build context — the binary embeds whatever's already checked in under `src/`.
