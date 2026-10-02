---
id: SPEC-ui-and-container-testing
companions:
  - inspec-controls.md
  - pipeline-wiring.md
sources:
  - ../../brainstorming/brainstorm-ui-and-container-testing-2026-10-02/brainstorm-intent.md
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# UI and Container Regression Safety Nets

## Why

A vision to realize, driven by test-first discipline. fantasy-hockey is unreleased, and unit and GoDog acceptance tests cannot see browser-visible behaviour (for example `static/awards.js` reading embedded JSON option lists, or a template refactor renaming a form field) or what the container image actually ships. Dependency bumps arrive automatically, so a bump or refactor that breaks the UI or bloats the image must fail in the pipeline before an edge image is published. The owner is the sole maintainer and wants this at a sustainable, hobby-project cost.

## Capabilities

- **CAP-1** (Must)
    - **intent:** The maintainer can verify the award-picks journey in a real phone-sized browser: log in with a seeded login code, submit partial award picks, see them flagged, complete the rest, and see them saved.
    - **success:** A Playwright test in `tests/playwright` passes locally and in the pipeline, and fails when the awards form or `awards.js` is broken.
- **CAP-2** (Should)
    - **intent:** The maintainer can verify that main navigation and navigation to each prediction page work.
    - **success:** A Playwright test clicks each of the four tabs (Predict, Leaderboard, Compare, Rules) and asserts URL and `aria-current="page"`, opens each prediction set from Predict, and returns via the back link.
- **CAP-3** (Should)
    - **intent:** The maintainer can verify leaderboard standings computed from results written into the data file.
    - **success:** A Playwright test seeds data, submits picks through the UI, stops the app, writes results into the data file, restarts the app, and asserts one golden-path standings table.
- **CAP-4** (Must)
    - **intent:** The maintainer can verify that the shipped image is minimal and correctly built, by running InSpec against a running container of it.
    - **success:** The controls in `inspec-controls.md` pass against the current image and fail when one is violated (for example source left in the final stage or a root user).
- **CAP-5** (Should)
    - **intent:** The maintainer can keep the image from silently growing.
    - **success:** An InSpec allowlist control fails on any unexpected file, and a 6.5 MB size ceiling fails when the image exceeds it, so growth requires a deliberate change.
- **CAP-6** (Must)
    - **intent:** The maintainer can run both suites locally with one task each, and the same commands run in the pipeline.
    - **success:** Taskfiles in `tests/playwright` and `tests/inspec` are included from the root `taskfile.yml`, take an `IMAGE` variable, run sequentially with a data-file reset, and the project linters cover `tests/`.
- **CAP-7** (Must)
    - **intent:** The pipeline blocks publishing an edge image when the UI or image tests fail.
    - **success:** The job graph in `pipeline-wiring.md` is in place and a deliberately failing test stops `publish-edge`.
- **CAP-8** (Should)
    - **intent:** The release run confirms the released image is still the tested image.
    - **success:** `release.yml` runs InSpec after `publish-release` without breaking the run, and compares the digest of the newly tagged `latest` with `edge`.
- **CAP-9** (Should)
    - **intent:** Test dependency updates are watched like other dependencies.
    - **success:** `.github/dependabot.yml` has an `npm` entry for `tests/playwright`.

## Constraints

- Build the image once, test that exact artifact, and promote it by retagging. Never rebuild, because the released image must be the tested image.
- Tests must not be flaky. Seed deadlines relative to now with generous margins; add no production fake clock or test-only code path.
- The app loads the data file once at startup and has no reload. Results are written while the app is stopped, then the app restarts; no reload endpoint.
- Playwright runs a single phone-viewport project (Chromium). The app has one layout for all devices.
- InSpec runs in its own container with a `docker://` target and is never installed in the app image. The repo is not bind-mounted into the container under test. The runner needs docker socket access and `/bin/sh` in the container under test, so a future scratch or distroless final stage breaks this approach.
- The container under test runs the image's real `CMD` with `SESSION_SECRET` set (startup fails without it), so the app is live for the start-and-serve control. `curl` is banned from the image, so probes use busybox `wget`.
- The release digest compare uses `latest` against `edge`, never a `:sha` tag, because cleanup deletes `:sha` tags from Docker Hub.
- Data files are never shared between runs: sequential with a reset locally, separate workers in GitHub Actions.
- amd64 only for now.
- `Dockerfile` and `.github/workflows/**` are protected files; changing them requires an explicit user request.
- `tests/playwright` has its own `package.json` and lockfile; the root `package.json` stays semantic-release tooling.

## Non-goals

- Desktop viewport or any device matrix.
- arm64 testing; Raspberry Pi deployment, performance testing, and monitoring come later.
- A data-file reload endpoint or fake clock.
- Docker Scout as a gate, or CVE-free base images.
- InSpec inside the app image.
- Replacing the GoDog suite; Playwright covers only browser-visible behaviour.
- New GoDog acceptance tests or Gherkin features for this infrastructure work.
- Notifying anyone when the `release.yml` InSpec run fails.

## Success signal

A dependency bump or refactor that breaks navigation or the awards form, or adds source or bloat to the image, fails `pipeline.yml` before an edge image is pushed. Both suites run locally with one task each.

## Assumptions

- Eight InSpec controls are the Must set listed in `inspec-controls.md`: seven image controls plus start-and-serve. The allowlist and size ceiling are CAP-5.
- The container under test starts from the image's real `CMD` rather than the `sleep` entrypoint pattern used in another repo. Say if `docker exec -d` on a sleep container is preferred.
- The 5.33 MB size was measured on arm64, and the 6.5 MB ceiling is applied to the amd64 image under test.
- Playwright uses Chromium with phone emulation, per the accepted 60-minute plan.

## Open Questions

- Does the amd64 image also fit under 6.5 MB, given 5.33 MB was measured on arm64?
