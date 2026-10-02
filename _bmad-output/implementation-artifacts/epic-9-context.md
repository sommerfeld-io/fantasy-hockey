# Epic 9 Context: Public Access & Observability

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Let the person running the pool put the app on a homelab host (Raspberry Pi, arm64) behind an nginx reverse proxy on port 80, using either the dev compose or a ready-to-adapt example production compose, and see how the app behaves: Prometheus metrics scrapable from the host only, an audit log of who logged in and who did what and when, and proxy access logs on stdout. Collection and visualization of telemetry (Alloy, Grafana) are out of scope and live in another repo.

## Stories

- Story 9.1: Reverse Proxy in the Dev Compose
- Story 9.2: Anonymous Metrics Endpoint With Runtime and HTTP Metrics
- Story 9.3: Login and Action Counters
- Story 9.4: Audit Log Lines for Logins and Prediction Saves
- Story 9.5: The Proxy Hides `/metrics`
- Story 9.6: Example Production Compose

## Requirements & Constraints

- Port 80 reaches the app's port 8080 with responses (login page, assets, session cookies) unchanged, except `/metrics`, which the proxy answers with 404.
- The proxy writes one access-log line per request to stdout in nginx's standard "combined" format.
- Plain HTTP only, no TLS/443, no `Secure` cookie flag, by decision.
- `/metrics` is anonymous on 8080; protection is network placement only (proxy 404 on 80, loopback-only publish in the example compose).
- No Player, email or login code in any metric label or log field; `player_id` slug is the only permitted identifier, in audit lines.
- The example production compose is an example, not a supported deployment: `latest` image tag, Mailpit as SMTP, dummy `SESSION_SECRET`, Mailpit ports not published.
- Dockerfile and `.github/workflows/**` are protected and stay untouched.

## Technical Decisions

- Proxy is the official `nginx` image (`stable-alpine`, arm64 available) on port 80.
- One proxy definition kept in step between dev and example compose: `location ~* ^/metrics/?$` returns 404; all else proxied to `http://fantasy-hockey:8080` with `resolver 127.0.0.11 valid=10s` and the upstream in a variable (re-resolves a recreated app container, avoids 502); `access_log /dev/stdout combined;` set explicitly.
- Dev compose mounts the repo's nginx config read-only; the example ships its own copy and a test asserts the two copies' proxy rules are identical.
- App data is mounted as a directory, never a single file (atomic rename cannot replace a bind-mount point).
- The app's direct 8080 publish stays in the dev compose; it is `127.0.0.1:8080` in the example.
- Observability lives only in `internal/observe` (dedicated Prometheus registry, HTTP middleware, `/metrics` handler, single `observe.Audit` emitter), consumed only by `internal/web` and `main.go`; enforced by golangci-lint `depguard` rules. `/metrics` is registered on the outer mux in `internal/web`, outside session auth.
- Metric names are `fantasy_hockey_` prefixed; `route` label is the route pattern from the original request, `unmatched` when empty.
- Audit vocabulary is fixed: `login_code_requested`, `login_succeeded`, `login_failed`, `logout`, `prediction_saved`; `store.Save*` methods return the persisted rows.
- New Prometheus client dependency must pass the `go-licenses` gate.

## Cross-Story Dependencies

- 9.3 and 9.4 depend on 9.2 (the `observe` API and `NewServer` signature change).
- 9.5 and 9.6 build on the proxy config introduced in 9.1.
- Infra stories (9.1, 9.5, 9.6): ask the human whether an acceptance test applies. 9.2 and 9.3 need a Gherkin feature written before implementation.
