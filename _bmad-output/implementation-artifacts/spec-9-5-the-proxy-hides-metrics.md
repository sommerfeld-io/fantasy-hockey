---
title: 'The Proxy Hides /metrics'
type: 'feature'
created: '2026-10-01'
baseline_commit: '2365e58ef2222564c04931393d17785cc53bd3b1'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-9-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The port-80 proxy forwards `/metrics` to the app, so anyone who can reach the proxy can read the app's metrics.

**Approach:** Add `location ~* ^/metrics/?$` to `configs/nginx/default.conf` returning 404 without forwarding (AD-34). All other paths still proxy to the app; `/metrics` on 8080 is unchanged. Decision (human): no automated test; verification is manual with curl against the dev compose.

</frozen-after-approval>

## Implementation Notes

- Added `location ~* ^/metrics/?$ { return 404; }` to `configs/nginx/default.conf` before `location /` (regex locations outrank the prefix location either way). Added a note to `src/internal/observe/README.md`.
- Verified manually on 2026-10-01 against `docker compose up nginx fantasy-hockey` (containers removed afterwards): `nginx -t` OK and `docker compose config -q` OK. Port 80: `/login` 200; 404 for `/metrics`, `/metrics/`, `/METRICS`, `/Metrics/`, `/%6Detrics`, `/metrics?x=1` and `//metrics`. `/metricsx` and `/metrics/foo` still reached the app (its own 404). Port 8080: `/metrics` 200. The app's `fantasy_hockey_http_requests_total{route="GET /metrics"}` rose only for the direct 8080 scrapes (1 then 3), so none of the nine port-80 requests reached it.

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| No verification evidence recorded | false | Recorded in Implementation Notes above. |
| `nginx -t` not run | false | Run via the nginx image; output "test is successful". |
| No regression guard / automated test | false | Human decision in this build: manual checks only. |
| Port 80 404 is nginx's page, distinguishable from the app's 404 | low | The story requires a 404, not concealment; matching the body would add config. Rejected. |
| Match narrower than "hides /metrics" (subpaths, case) | false | AD-34 and the story fix the pattern at `^/metrics/?$`; case is covered by `~*`; `/metrics/foo` is not an app route. |
| Port 8080 exposure unchecked | false | AD-34: 8080 stays published in the dev compose; the example compose (Story 9.6) binds it to loopback. |
| Docs do not say `/metrics` is hidden on port 80 | low | Direct fix applied: a paragraph in `src/internal/observe/README.md`. Scrape guidance belongs to Story 9.6. |
| Spec and status bookkeeping incomplete | false | Finalized in this step: spec `done`, story `review`. |

### Code review 2026-10-01

Blind Hunter, Edge Case Hunter, Verification Gap and Acceptance Auditor: 0 patches, 0 deferred, rest rejected. The Acceptance Auditor found all four ACs and the AD-34 pattern met.

- `false` -- `/metrics/foo` or `/metrics;x` stay reachable through port 80 -- the app registers only `GET /metrics` (`src/internal/web/web.go`), so those paths are the app's own 404; AD-34 fixes the pattern.
- `false` -- a path-prefixing ingress could bypass the regex -- no base path or ingress exists in this stack.
- `false` -- no automated test -- human decision for this story.
- `low` -- README says `/metrics` is "reachable only on the app's own port 8080" without naming the publish setting -- accurate for the dev compose; the example compose (9.6) binds 8080 to loopback and says so.
- `low` -- nginx 404 page differs from the app's; `~*` rationale; access-log noise -- cosmetic; the story requires a 404, not concealment.
