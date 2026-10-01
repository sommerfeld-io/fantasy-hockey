# Package: `observe`

The one home of the app's observability. An `*Observer` owns a dedicated Prometheus registry (never the default registerer, so several Observers can coexist in one process) with the Go and process collectors plus two HTTP series, and the audit-log emitter.

| Member                      | Purpose                                                                                                                                                            |
|-----------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `New()`                     | Builds an Observer and its registry.                                                                                                                               |
| `Handler()`                 | Serves the registry in the Prometheus text format (mounted at `GET /metrics` by `internal/web`, anonymous).                                                        |
| `Middleware(next)`          | Records `fantasy_hockey_http_requests_total` and `fantasy_hockey_http_request_duration_seconds`, labelled `route` and `status`.                                    |
| `Audit(event, playerID, …)` | Writes one `slog.Info("audit", "event", …, "player_id", …)` line. Events: `login_code_requested`, `login_succeeded`, `login_failed`, `logout`, `prediction_saved`. |

The `route` label is the request's route pattern, or `unmatched` when no pattern matched, so a raw path never becomes a label. No Player, email or login code may appear in a label or metric.

Imports are limited by a `depguard` rule to the standard library, `internal/clock` and `github.com/prometheus/client_golang`.
