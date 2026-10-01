# Package: `observe`

The one home of the app's observability. An `*Observer` owns a dedicated Prometheus registry (never the default registerer, so several Observers can coexist in one process) with the Go and process collectors plus two HTTP series, two action counters, and the audit-log emitter.

| Member                      | Purpose                                                                                                                                                                                                                                                           |
|-----------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `New()`                     | Builds an Observer and its registry.                                                                                                                                                                                                                              |
| `Handler()`                 | Serves the registry in the Prometheus text format (mounted at `GET /metrics` by `internal/web`, anonymous).                                                                                                                                                       |
| `Middleware(next)`          | Records `fantasy_hockey_http_requests_total` and `fantasy_hockey_http_request_duration_seconds`, labelled `route` and `status`.                                                                                                                                   |
| `Audit(event, playerID, …)` | The single audit emitter. Writes one `slog.Info("audit", "event", …, "player_id", …)` line (`player_id` omitted when empty) and increments the matching counter. Events: `login_code_requested`, `login_succeeded`, `login_failed`, `logout`, `prediction_saved`. |
| `PreRegisterKinds(kinds…)`  | Creates one `fantasy_hockey_prediction_saves_total{kind}` series per kind at zero (`internal/web` passes `store.Kinds`).                                                                                                                                          |

`fantasy_hockey_login_events_total{event}` (`login_code_requested`, `login_succeeded`, `login_failed`, `logout`) is pre-registered at zero in `New`. `fantasy_hockey_prediction_saves_total{kind}` is incremented by `Audit(EventPredictionSaved, …, "kind", k, …)`. Both are labelled by that one label only. `Audit` never takes an email or a login code.

The `route` label is the request's route pattern, or `unmatched` when no pattern matched, so a raw path never becomes a label. No Player, email or login code may appear in a label or metric.

Imports are limited by a `depguard` rule to the standard library, `internal/clock` and `github.com/prometheus/client_golang`.
