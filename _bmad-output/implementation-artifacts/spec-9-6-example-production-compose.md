---
title: 'Example Production Compose'
type: 'feature'
created: '2026-10-01'
baseline_commit: '0f6448bcc5e660dac2767920cf402fd5a1aeb2ab'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-9-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The person running the pool has no ready-to-adapt compose file for the Raspberry Pi, only the dev compose.

**Approach:** Add `docs/examples/docker-compose.yml` with the app (`sommerfeldio/fantasy-hockey:latest`, a mounted data directory, a dummy `SESSION_SECRET`, 8080 published on `127.0.0.1` only), Mailpit as the SMTP target (no ports published) and the nginx proxy on port 80 (no `/metrics`, 404 as in Story 9.5). Comments state it is an example to adapt, that the data directory must be writable by UID 1000, and that the file is not a supported deployment. Add a pointer in `docs/operator-guide.md` and allow `docs/examples` in `.folderslintrc`.

**Decisions (human):** No test of any kind, and no drift test. The example re-uses the repo's nginx config by mounting `configs/nginx/default.conf` instead of shipping a copy, so there is no second copy to drift. Consequence: the example is not self-contained when copied out alone; the comments say to copy the nginx config next to it and fix the mount path.

**Never:** Touch `Dockerfile` or `.github/workflows/**`. Add TLS/443. Publish Mailpit ports. Mount the data file as a single file.

</frozen-after-approval>

## Implementation Notes

- Added `docs/examples/docker-compose.yml` (app at `sommerfeldio/fantasy-hockey:latest`, `./data:/data` directory mount, dummy `SESSION_SECRET`, `127.0.0.1:8080:8080`, Mailpit with no published ports, nginx on port 80 mounting `../../configs/nginx/default.conf` read-only), a pointer and run steps in `docs/operator-guide.md`, and `docs/examples` in `.folderslintrc`.
- The image runs as UID 1000 (`Dockerfile`: `USER_ID=1000`), stated in the compose comments with the `chown` command.
- Verified manually on 2026-10-01: `docker compose -f docs/examples/docker-compose.yml config -q` OK. Ran the stack with a temporary override outside the repo (local dev image because the published `latest` predates `/metrics`, and `user: root` because this sandbox cannot run non-root containers, even stock alpine as UID 1000). Port 80: `/login` 200; `/metrics`, `/metrics/` and `/METRICS` 404. `127.0.0.1:8080/metrics` 200 with 8080 bound to `127.0.0.1` only. Mailpit published no ports. The app created `fantasy-hockey.yml` inside the mounted directory. Stack and scratch data directory removed afterwards.
- Not verified: running as the image's real non-root UID 1000 (sandbox limit), and the published `latest` image (does not yet include `/metrics`).

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| Mailpit UI not published, so login codes cannot be read | medium | Real for someone trying the example. Fixed: comment says how to publish 8025 on loopback temporarily. |
| `SMTP_USERNAME` / `SMTP_APP_PASSWORD` not shown | low | Direct fix: commented template lines in the environment block. |
| `latest` on Mailpit contradicts "pin a tag" comment | low | Direct fix: comment now covers both images. |
| Guide has no run steps, chown command or check | medium | Fixed: run, chown and curl check added to `docs/operator-guide.md`, plus a backup note. |
| No healthchecks | low | nginx re-resolves the app per request; start order only matters for the first seconds. Rejected. |
| No arm64 platform pin | false | AD-14: the image targets arm64; nginx and Mailpit publish arm64 images. |
| No hardening, resource limits or log rotation | low | Out of scope for a documented example. Rejected. |
| Unquoted port mappings parse as sexagesimal | false | Compose parsed and published them correctly in the real run; `80:80` is also used unquoted in the dev compose. |
| No automated check or smoke test for the example | false | Human decision in this build: no test of any kind. |
| Example can drift from root compose, nginx config, env vars | false | The nginx config is mounted, not copied (human decision); env vars verified against `main.go` by the real run. |
| Relative nginx mount path breaks if copied out | low | Documented in the compose header comment. |
