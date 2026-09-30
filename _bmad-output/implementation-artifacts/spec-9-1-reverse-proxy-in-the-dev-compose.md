---
title: 'Reverse Proxy in the Dev Compose'
type: 'feature'
created: '2026-09-30'
status: 'done'
baseline_commit: '1b61be21976f868b4e3d19b91187635047b64187'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-9-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The dev compose exposes the app only directly on 8080, so the person running the pool cannot exercise the nginx proxy setup planned for the Raspberry Pi, nor see proxy access logs.

**Approach:** Add an official `nginx` service to `docker-compose.yml` that serves port 80 and forwards to the app by compose service name. Its config lives in `configs/nginx/`, mounted read-only, and `configs/nginx` is allowed in `.folderslintrc`.

## Boundaries & Constraints

**Always:** Proxy to `http://fantasy-hockey:8080` via `resolver 127.0.0.11 valid=10s` with the upstream held in a variable, so a recreated app container is re-resolved. Set `access_log /dev/stdout combined;` explicitly. Keep the app's `8080:8080` publish unchanged. All nginx config files live in `configs/nginx/`, named kebab-case per `.ls-lint.yml`. Use the pinned image `nginx:1.31.6`.

**Decisions (human):** No acceptance test, no static Go test, no automated curl check; verification is manual.

**Never:** Touch `Dockerfile` or `.github/workflows/**`. Do not add the `/metrics` 404 (Story 9.5) or the example production compose (Story 9.6). Do not add TLS/443.

## I/O & Edge-Case Matrix

| Scenario            | Input / State                        | Expected Output / Behavior                              | Error Handling         |
|---------------------|--------------------------------------|---------------------------------------------------------|------------------------|
| Proxied request     | `GET http://localhost/login`         | Login page; request reaches app over the compose network | N/A                    |
| Access log          | One proxied request                  | Exactly one "combined"-format line on proxy stdout      | N/A                    |
| Direct access       | `GET http://localhost:8080/login`    | Unchanged behavior                                      | N/A                    |
| App recreated       | App container recreated, proxy up    | `GET /login` on port 80 still returns login page        | Not a 502              |

</frozen-after-approval>

## Code Map

- `docker-compose.yml` -- dev compose; add `nginx` service (port `80:80`, `depends_on: fantasy-hockey`, `./configs/nginx/default.conf:/etc/nginx/conf.d/default.conf:ro`). Leave the `fantasy-hockey` service and its `8080:8080` publish as-is. `task compose:pull` pulls every service except `fantasy-hockey`, so the nginx image is covered.
- `configs/nginx/default.conf` -- new; replaces the image's default server block. `.conf` is kebab-case under `.ls-lint.yml`.
- `.folderslintrc` -- add `configs` and `configs/nginx` to `rules` (the file lists parent and child folders explicitly, e.g. `.github` and `.github/instructions`).
- `_bmad-output/implementation-artifacts/sprint-status.yaml` -- story status sync.

## Tasks & Acceptance

**Execution:**
- [x] `configs/nginx/default.conf` -- `server` listening on 80, `resolver 127.0.0.11 valid=10s;`, `set $app http://fantasy-hockey:8080;`, `access_log /dev/stdout combined;`, `location /` with `proxy_pass $app;` plus `Host`/`X-Forwarded-*` headers -- variable upstream forces re-resolution; explicit log format avoids the image's `main` default
- [x] `docker-compose.yml` -- add `nginx` service (`nginx:1.31.6`, `restart: unless-stopped`, port 80, read-only config mount, `depends_on`) -- serves port 80 in dev
- [x] `.folderslintrc` -- allow `configs` and `configs/nginx` -- required by the new folder

**Acceptance Criteria:**
- Given the dev compose is up, when I request `http://localhost/login`, then I get the login page via the app's compose service name, not the host-published port.
- Given one proxied request, when I read the proxy container's stdout, then there is exactly one combined-format access-log line for it.
- Given the proxy is added, when I request `http://localhost:8080/login`, then the app behaves as before.
- Given the app container is recreated while the proxy keeps running, when I request `http://localhost/login`, then I get the login page, not a 502.

## Implementation Notes

- Implemented directly (no subagent). Verified manually: `/login` 200 on port 80 and 8080; one combined-format line per proxied request; still 200 after `--force-recreate` of the app. `nginx -t`, `task lint` and `task go:run` pass.
- `proxy_set_header Host $http_host` keeps the client's Host (incl. port) for the app.
- Config mounted as a single file at `/etc/nginx/conf.d/default.conf:ro` (nginx never writes it, so the store's rename concern does not apply).

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| depends_on has no health condition; no healthcheck on nginx | low | App has no healthcheck; the variable upstream re-resolves per request, so start order only affects the first seconds. Fix adds complexity; rejected. |
| X-Forwarded-For / X-Real-IP spoofing reaches audit logs | false | Grep of `src` finds no read of `X-Forwarded*`, `X-Real-IP` or `RemoteAddr`; audit lines carry `player_id` only (AD-33). |
| No `client_max_body_size` | false | Every form handler caps its body with `http.MaxBytesReader` far below nginx's 1m default (`login.go`, `sheet.go`). |
| No WebSocket/SSE/upgrade support, read timeout | false | Grep finds no websocket, EventSource or `text/event-stream` use; the app is server-rendered forms. |
| `ipv6=off` on resolver | maybe-false | Would need to see Docker DNS return AAAA for a compose service; the recreate check returned 200. Not reproduced. |
| Empty `$http_host` on HTTP/1.0 | false | Browsers always send Host; the app does not build absolute URLs from it (no `Host` read found in the grep above). |
| Bare 502 when the app is down | low | Expected proxy behavior; the story only requires recovery after recreate, which was verified. |
| `${PROXY_PORT:-80}` override | low | Story requires port 80; extra knob is not asked for. |
| Bind `127.0.0.1:80` | false | Contradicts AC and the LAN/router use case (port 80 forwarded to the host). |
| No automated proxy check / regression guard | low | Human decided no acceptance, static or curl test for this story; manual checks passed. Not deferred, since the decision is explicit. |
| sprint-status.yaml change not in diff | false | Excluded from the review diff on purpose; it was changed (`9-1` and `epic-9` are `in-progress`). |
| Image pin upkeep, `error_log`, access-log fields, dev-only comment | low | Cosmetic or out of the story; the tag was pulled and ran, and the config already comments the `access_log` choice. |

## Verification

**Commands:**
- `docker compose config -q` -- expected: exit 0
- `task lint` -- expected: folders, filenames and YAML linters pass
- `task go:run` -- expected: app builds and starts

**Manual checks (if no CLI):**
- `docker compose up -d nginx fantasy-hockey`, open `http://localhost/login` -- expected: login page; `docker compose logs nginx` shows one combined-format line per request
- `docker compose up -d --force-recreate fantasy-hockey`, reload `http://localhost/login` -- expected: login page, not a 502
