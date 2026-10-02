---
title: 'Anonymous Metrics Endpoint With Runtime and HTTP Metrics'
type: 'feature'
created: '2026-10-01'
baseline_commit: 'ddfe8143c49b84b7e104a58d95d8cfb8b89555de'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-9-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The person running the pool cannot see how the app behaves: there is no `/metrics` endpoint, and the audit-event API that Stories 9.3 and 9.4 need does not exist.

**Approach:** Add `internal/observe` (dedicated Prometheus registry with Go and process collectors, HTTP middleware, `/metrics` handler, `Audit` emitter behind one `*observe.Observer`). `internal/web.NewServer` takes the Observer, registers `GET /metrics` on the outer mux and wraps it with the middleware. Remove `internal/auth`'s two superseded info lines.

## Boundaries & Constraints

**Always:** Follow AD-31, AD-32, AD-33, AD-36. Series are `fantasy_hockey_http_requests_total{route,status}` and `fantasy_hockey_http_request_duration_seconds{route,status}` (`prometheus.DefBuckets`). `route` is `r.Pattern` of the original request read after `next` returns, `unmatched` when empty; `status` is the response code string (default 200). `/metrics` is outside `requireSession` and instrumented under its own pattern. The Observer builds its own `prometheus.NewRegistry()`, never the default registerer. `observe.Audit(event, playerID, attrs…)` writes one `slog.Info("audit", "event", …, "player_id", …)` line; its event constants are `login_code_requested`, `login_succeeded`, `login_failed`, `logout`, `prediction_saved`. `observe` may import only `$gostd`, `internal/clock` and `github.com/prometheus/client_golang`. Add `github.com/prometheus/client_golang` v1.24.1 and keep `task go:licenses` green. Update every `NewServer` call site, including acceptance tests.

**Decisions (human):** Pending approval.

**Never:** Call `Audit` from any handler (Stories 9.3, 9.4). Add login or save counters (9.3). Touch the proxy config or compose files (9.5, 9.6). Touch `Dockerfile` or `.github/workflows/**`. Put a Player, email or login code in any label or metric.

## I/O & Edge-Case Matrix

| Scenario        | Input / State                          | Expected Output / Behavior                                                     | Error Handling |
|-----------------|----------------------------------------|--------------------------------------------------------------------------------|----------------|
| Anonymous scrape| `GET /metrics`, no session cookie      | 200, Prometheus text format, includes `go_goroutines` and `process_*` series   | N/A            |
| Matched route   | `GET /login` then `GET /metrics`       | `http_requests_total{route="GET /login",status="200"}` and duration series exist | N/A          |
| Unmatched path  | `GET /no/such/path/123`                | Counted under `route="unmatched"`, `status="404"`; raw path appears nowhere    | N/A            |
| Wrong method    | `PUT /login`                           | Counted under `route="unmatched"`, `status="405"`                              | N/A            |
| Protected route | `GET /` without session                | Counted under the route pattern, `status="302"`                                | N/A            |
| Two servers     | Two `NewServer` calls in one process   | Both build; no duplicate-registration panic                                    | N/A            |

</frozen-after-approval>

## Code Map

- `src/internal/observe/` -- new package; `Observer`, `New`, `Middleware`, `Handler`, `Audit` plus event constants.
- `src/internal/web/web.go:79` -- `NewServer(st, send, secret)` gains `*observe.Observer`; register `mux.Handle("GET /metrics", ob.Handler())` and `return ob.Middleware(mux)`. Inner `requireSession` uses `WithContext`, so read the pattern from the original request.
- `src/main.go:~105` -- build the Observer in `run()` and pass it to `NewServer`.
- `src/internal/auth/auth.go:59` -- delete `slog.Info("send login code", …)`; keep the `slog.Error` at L56.
- `src/internal/auth/validate.go:15-25` -- delete the `if ok { slog.Info("validate login code" … ) }` block and the doc sentence about server-side logging; keep web's `slog.Error` lines.
- `src/internal/auth/auth_test.go` (`TestRequestLoginCodeShouldLogWhenACodeIsSent`, `waitForLogContaining` if unused) and `validate_test.go` (~L65-80) -- drop the tests asserting the removed lines; keep the no-log tests.
- `src/.golangci.yml` -- add `observe` depguard rule (files `**/internal/observe/*.go` excluding tests; allow `$gostd`, `internal/clock`, `github.com/prometheus/client_golang`); deny `github.com/prometheus/client_golang` in the `web` rule.
- `src/go.mod`, `src/go.sum` -- add client_golang v1.24.1.
- `src/acceptance-tests/features/metrics-endpoint.feature` + `metrics_endpoint_steps_test.go` -- new; register `InitializeMetricsEndpointScenario` in `suite_test.go`. Call sites to update: `suite_test.go:74`, `fixture_support_test.go:371`, and `login_`, `enter_login_code_`, `hand_edited_results_`, `log_out_`, `stay_logged_in_`, `load_canonical_team_list_`, `load_canonical_nhl_player_list_`, `app_shell_steps_test.go`.
- `src/internal/web/*_test.go` (~190 calls) -- route through one shared test helper that builds an Observer.
- `docs/architecture.md` -- sync layer table if it lists packages.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` -- story status sync.

## Tasks & Acceptance

**Execution:**
- [x] `src/acceptance-tests/features/metrics-endpoint.feature` + steps + suite wiring -- Gherkin first, red before implementation -- BDD per project rules
- [x] `src/internal/observe/*` (tests first) -- registry, middleware, handler, Audit -- AD-31/32/33/36
- [x] `src/.golangci.yml`, `src/go.mod`, `src/go.sum` -- depguard rules and dependency
- [x] `src/internal/web/web.go`, `src/main.go`, every test call site -- new `NewServer` parameter and `/metrics` route
- [x] `src/internal/auth/*` -- remove the two info lines and their tests
- [x] `docs/architecture.md`, `sprint-status.yaml` -- sync

**Acceptance Criteria:**
- Given the app is running, when an unauthenticated client requests `/metrics`, then it gets 200 in the Prometheus text format without a login session.
- Given the `/metrics` output, then it includes goroutine, memory, GC, CPU and open file descriptor series.
- Given the app served a request, then request-count and duration series exist for that route and status, and no label holds a raw path.
- Given any `/metrics` response, then it contains no Player email address and no login code.
- Given `task go:lint`, then `internal/web` importing `github.com/prometheus/client_golang` fails and `observe` passes its own rule.
- Given `internal/auth`, then the `send login code` and `validate login code` info lines are gone, failure `slog.Error` lines stay, and `observe.Audit` exists with the five event constants.

### Review Findings

Code review 2026-10-01 (Blind Hunter, Edge Case Hunter, Verification Gap, Acceptance Auditor): 0 `decision-needed`, 0 `patch`, 0 `defer`, 35 rejected. Acceptance Auditor found all six ACs, the Never list and the I/O matrix covered.

#### Rejected

- `false` -- Audit has no call sites; logins unlogged in the interim -- spec Never section forbids handler call sites here, 9.3/9.4 add them.
- `false` -- `/metrics` publicly reachable, no restriction -- AD-32 makes exposure network placement only; proxy 404 is Story 9.5, loopback publish is 9.6.
- `false` -- scrape counted under `GET /metrics`, no in-flight gauge, `DefBuckets` -- AD-32 instruments `/metrics`; AD-36 fixes series and buckets.
- `false` -- `observe` allow-list contains `internal/clock` though unused -- AD-31 specifies that allow-list.
- `false` -- client_golang denied only in `web` and `compare` -- every other package has an allow-list that rejects it.
- `false` -- `NewServer` doc comment run-on paragraph -- matches the existing style of the paragraphs above it.
- `false` -- spec says "Pending approval" and tables unpadded -- edits the frozen spec; fix is not in scope of the code.
- `low` -- panicking handler not counted (raised by 3 layers) -- no handler panics; fix needs a defer plus re-panic.
- `low` -- double `WriteHeader`, 1xx, no Flusher/Hijacker on `statusRecorder` -- no handler does either; `Unwrap` covers ResponseController.
- `low` -- nil Observer, `Audit` odd attrs or empty player id, global slog in `Audit` -- only internal callers; guards add complexity.
- `low` -- process series absent off Linux -- dev container and CI run Linux.
- `low` -- 6-digit code could match digits in a metric value; `PostForm` default client without timeout, follows redirects -- roughly 1e-4 chance, local httptest server, extra counts harmless.
- `low` -- no HEAD/OPTIONS or failed-login scenario -- label is the pattern or `unmatched`, so cardinality stays bounded.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| Finding | Verdict | Evidence |
|---------|---------|----------|
| Audit never called; no `audit` line is emitted | false | Spec Never section forbids handler call sites in this story; 9.3 and 9.4 add them. |
| Removed auth info lines leave logins unlogged | false | Removal is a spec requirement (AD-33); 9.4 restores the lines as audit events. |
| Audit/event vocabulary beyond metrics scope | false | Spec and AD-33 make 9.2 own the `observe` audit API. |
| Audit odd attrs / empty playerID / unvalidated attrs | low | Only internal callers; fix adds guards for a state not shown reachable. Rejected. |
| statusRecorder: double WriteHeader, 1xx | low | No handler writes twice or sends 1xx; fix adds branches. Rejected. |
| statusRecorder lacks Flusher/Hijacker | low | Grep finds no flush/hijack use; `Unwrap` covers ResponseController. Rejected. |
| Panicking handler not counted | low | net/http recovers; no handler panics on a known path; fix needs defer plus re-panic. Rejected. |
| `/metrics` scrape counted under its own pattern | false | AD-32 assumption states `/metrics` is instrumented under its own pattern. |
| Proxy exposure of `/metrics` undocumented | false | Story 9.5 and 9.6 own exposure. |
| Cardinality: HEAD/OPTIONS on matched path | false | Route is the pattern or `unmatched`; status values are a bounded set. |
| Tautological constants test, no-assertion build-twice test | low | Cosmetic; build-twice fails on a duplicate-registration panic, which is its purpose. |
| Series step could match HELP/TYPE lines | false | Lines start with `# HELP`/`# TYPE`; the check matches `\nname` at line start. |
| Empty emailed code gives misleading failure | low | Direct correction; patched with a guard in the step. |
| 6-digit code could match digits in a metric value | low | Chance per run roughly 1e-4; fix is a larger step redesign. Rejected. |
| Shared scenario state across scenarios | false | Godog calls the initializer per scenario; same pattern as `log_out_steps_test.go`. |
| Non-Linux hosts lack process series | low | Dev container and CI run Linux. Rejected. |
| `http.PostForm` default client, no timeout | low | Local httptest server; sibling steps use the same call. Rejected. |
| nil Observer panics in NewServer | low | Every caller passes one; fail-fast is acceptable. Rejected. |
| govulncheck / depguard prefix coverage | false | Gate runs in `task go:build`; other packages have allow-lists that reject client_golang. |
| depguard denies client_golang only in web and compare | false | Store, mailer, clock, scoring, standings and auth rules are allow-lists that reject it. |

## Design Notes

`Audit` in this story only writes the log line. Stories 9.3 and 9.4 add call sites; 9.3 registers the login and save counters inside the same Observer, so `Audit` keeps one signature and stays the single emitter (AD-33).

## Verification

**Commands:**
- `task go:test` -- expected: unit tests pass
- `task go:test:acceptance` -- expected: acceptance tests pass, including the new feature
- `task lint` -- expected: all linters pass, including depguard and gherkin
- `task go:licenses` -- expected: license gate passes with client_golang
- `task go:run` -- expected: app builds and starts
- `task docker:build` -- expected: image builds and lints (feature complete)
