# Intent: UI and Container Regression Safety Nets

## Intent

Add two test-first regression safety nets for the fantasy-hockey app, which is unreleased and has no past failures:

- Playwright guards UI and web behaviour that GoDog and unit tests cannot exercise (for example `static/awards.js` and `divisions.js` reading embedded JSON option lists, template refactors breaking form fields, navigation).
- Chef InSpec guards that the shipped image holds only what is needed (size, download time, attack surface).

Both gate the pipeline, so a Dependabot bump or refactor that breaks the UI or bloats the image fails before the edge image is published.

Keystone principle: build the image once, test that exact artifact, and promote it by retagging. Never rebuild.

## Playwright track

- Runner: Node `@playwright/test` in `tests/playwright`, with its own `package.json` and lockfile (the root `package.json` is semantic-release tooling).
- Viewport: phone only, as the single Playwright project. The app is mobile-first with one layout for all devices, so there is no desktop or device matrix.
- Journey 1 (Must): log in with a login code seeded in the data file, fill some player award picks, assert the partial-picks warning (story 2.7), complete the rest, assert they are saved. This exercises the `awards.js` surface GoDog cannot see.
- Journey 2 (Should): navigation walk over the 4 tabs (Predict, Leaderboard, Compare, Rules). Click each and assert URL and `aria-current=page`. From Predict, open each prediction set and return via the back link.
- Journey 3 (Should): phased leaderboard test.
    - Phases: seed the data file, start the app, submit picks through the UI, stop the app, write results into the data file, restart the app, assert the standings.
    - The app loads the data file once at startup and has no reload, so results must be written while it is stopped. Stopping and restarting between phases is accepted.
    - Keep it to one thin golden-path standings table. GoDog `leaderboard.feature` already covers ranking, ties and gold.
- Determinism: the harness generates the seed data file with deadlines relative to now, using generous margins instead of tight offsets such as +5s. Flaky tests are not acceptable. No production fake clock.
- Complements GoDog: GoDog stays the HTTP-level behaviour suite, and Playwright covers only browser-visible behaviour.

## Container track

- Tool: Chef InSpec, run in its own container against a running container of the app image via an `--target docker://<container>` target. Never install it inside the app image.
- Pattern: `docker run -d --entrypoint sleep`, then dockerized `inspec exec --no-distinct-exit`, then stop and remove. Do not bind-mount the repo into the container under test, because that would hide leaks.
- Constraints: the runner needs docker socket access (pinned image tag, ephemeral). The docker transport execs `/bin/sh` in the container under test, so a future scratch or distroless image would break the approach.
- Controls:
    - Runs as a non-root user.
    - App binary exists and is executable.
    - No source or build artifacts: `/workspaces/fantasy-hockey/src` is gone, and there are no `*.go`, `go.mod`, `go.sum`, `.git`, `taskfile.yml`, `Dockerfile` or Go toolchain.
    - Base image is alpine and matches the pinned release.
    - `curl` and `git` are absent.
    - Maintainer label is set (value `sebastian@sommerfeld.io`), checked through the `docker_image` resource because labels are image metadata.
    - Allowlist of expected files (only `/opt/fantasy-hockey` and a few known paths) plus an image size ceiling via `docker_image` (Should).
- A start-and-serve smoke (app starts, `/metrics` answers) is worth adding, because the missed-COPY bug `ee51d80` shows build-stage gaps.

## Pipeline and local wiring

- Local: taskfiles in `tests/playwright` and `tests/inspec`, included from the root `taskfile.yml`. Taskfiles take an `IMAGE` variable (local build locally, registry `:sha` in CI). Linters are extended to cover the new `tests/` folders.
- Execution: locally, Playwright and InSpec run sequentially with a data file reset. In GitHub Actions they run in parallel on separate workers, so there is no shared data file. The pipeline calls the same taskfile commands, so a CI failure reproduces locally.
- `pipeline.yml`:
    - Playwright and InSpec run between `build-image` and `publish-edge`, pull the `:sha` image, and gate `publish-edge`. An InSpec failure blocks pushing edge to Docker Hub.
    - `docker-scout` runs in parallel after `build-image`, reports only, and never gates. `publish-edge` does not need it.
    - `cleanup-dockerhub` must also wait for `docker-scout` (keeping `always()`), so it does not delete `:sha` while scout still pulls it.
    - `release-code` needs only `publish-edge`.
- `release.yml`:
    - InSpec runs after `publish-release`, parallel to `upload-sbom` and `docker-scout`.
    - It does not break the run. It only verifies the image is still the one tested.
    - Digest compare of the newly tagged `latest` against `edge` (never a `sha` tag, since cleanup removes those).
- Platform: amd64 only. Raspberry Pi, performance testing and monitoring come later.
- Dependabot: add an npm entry for `tests/playwright`.

## Scope

| Priority  | Items                                                                                                                                                                      |
|-----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Must      | InSpec controls (7), Playwright scaffold with login and award-picks journey, taskfiles and root includes, lint coverage, pipeline.yml gating with cleanup waiting on scout |
| Should    | Navigation walk, phased leaderboard test, allowlist and size ceiling, release.yml InSpec with latest-vs-edge digest, Dependabot npm entry                                  |
| Could     | Trace and screenshot on failure, nightly run                                                                                                                               |
| Won't now | Desktop viewport, arm64, reload endpoint, fake clock, scout gating, InSpec inside the image                                                                                |

Suggested build order: InSpec profile and taskfile, Playwright scaffold and award-picks journey, navigation walk, leaderboard phases, root taskfile and lint, `pipeline.yml`, `release.yml`, Dependabot.

## Open questions and risks

- Size ceiling value: set from the current image size plus headroom, so growth is a conscious decision.
- Single source of truth for the base image version: it appears in both the `Dockerfile` and a workflow label, and the pinned-base control should read one source.
- The root `Dockerfile` is not in the Dependabot docker entry, which covers only `.devcontainer` (unverified). Without it, base image bumps are not gated.
- Protected files: `Dockerfile` and `.github/workflows/**` may only change on an explicit user request, so the `pipeline.yml` and `release.yml` work needs one.
- Repo rules require the human to decide whether GoDog acceptance tests apply to this infrastructure-type work. Do not skip or add them silently.
- The `release.yml` InSpec failure has no consequence unless it is notified.
- The `docker-scout` job is skipped for the Dependabot actor, which is why `publish-edge` must not depend on it.
