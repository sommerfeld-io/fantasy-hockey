# Adversarial review: Epic 9 spine update (AD-11/14/21 amended, AD-31..AD-34)

Reviewed: `ARCHITECTURE-SPINE.md` (AD-11, AD-14, AD-21, AD-31..AD-34, Consistency Conventions, compose snippet), Epic 9 stories 9.1-9.6 in `epics.md`, and the real code in `src/internal/web`, `src/internal/auth`, `src/internal/store`, `src/.golangci.yml`, `docker-compose.yml`, `src/acceptance-tests`.

## Verdict

Not ready to build stories 9.1-9.6 in parallel. The four new ADs fix the big structural choices (one package, one registry, public `/metrics` with network-only exposure, one nginx config). They leave the shared seams open: who owns the audit call, what "a row saved" means when the store only returns `error`, what the metric and label names are, and how the example compose mounts the data file. Each attack below is a pair of units that both obey every AD literally and still fail to fit together. Seven findings are High or Critical.

## Findings

| #   | Severity | Hole                                                                                          | Fix                     |
|-----|----------|-----------------------------------------------------------------------------------------------|-------------------------|
| F1  | Critical | Audit emitter has two owners (Story 9.3 counters, Story 9.4 log) and 9.4 text contradicts AD-33 | Tighten AD-33, edit 9.4 |
| F2  | Critical | "One event per `store.Prediction` row saved" is not observable from `internal/web`            | New AD-35               |
| F3  | High     | Example compose "mounted `fantasy-hockey.yml`" breaks the atomic rename write                 | Tighten AD-34           |
| F4  | High     | Consistency table says "no Player ... in any log field", AD-33 requires `player_id`           | Fix table wording       |
| F5  | High     | Kind label and log field disagree; series and playoffcup mapping                              | Tighten AD-33           |
| F6  | High     | Login event semantics: unknown email, SMTP failure, 401 empty-id branch, anonymous logout     | Tighten AD-33           |
| F7  | High     | Metric names, namespace, label values, histogram buckets and NewServer signature unspecified  | New AD-36               |
| F8  | High     | Middleware must read `r.Pattern` after `next`, on the original request, with a status recorder | Tighten AD-32           |
| F9  | Medium   | Nginx resolves `fantasy-hockey` once at start; recreated app gives 502 or nginx crash         | Tighten AD-34           |
| F10 | Medium   | Shared nginx config path cannot be `./nginx.conf` in both compose files                       | Tighten AD-34           |
| F11 | Medium   | Free-text error strings can leak email into logs despite AD-33                                | Tighten AD-33           |
| F12 | Low      | Zero-valued counter series absent until first increment                                        | Tighten AD-36           |
| F13 | Low      | `status` label granularity, HEAD/405, `/metrics` self-instrumentation                          | Tighten AD-32           |
| F14 | Low      | Logger injection: slog default vs injected logger in acceptance tests                         | Tighten AD-31           |

## F1 (Critical): two owners for the audit call, and Story 9.4 contradicts AD-33

AD-33 says "a single `internal/observe` call per event writes the log line and increments the counter". It binds Stories 9.3 and 9.4. But the epics split the two halves into separate stories:

- 9.3 is "counters", depends on 9.2, and has a Gherkin feature written first.
- 9.4 is "log lines", has no dependency on 9.2, and its test approach is "decided with the human at build time".

Attack pair, both literal to every AD:

- Unit A (9.3, built first): adds `observe.LoginFailed()` that only increments, called from `handleLoginCodeSubmit`. It has no log to write because the log is 9.4's job.
- Unit B (9.4, built independently, possible because it does not depend on 9.2): adds `slog.Info("audit", "event", "login_failed")` in the handler, because `observe` has no registry yet.

Merged, `login_failed` produces one counter increment and one log line from two call sites. The first refactor that routes either through the other double-counts. AD-33's single-call rule is unenforceable when the call has two stories as owner.

Story 9.4 also says "reuse what already fits" for `internal/auth`'s `send login code` and `validate login code` lines. AD-33 says those lines are superseded. A 9.4 implementer following the story keeps `validate login code` at `auth/validate.go:25` (which fires on every success) and adds `login_succeeded` in the handler: two lines for one login, violating the 9.4 AC "exactly one line records that event".

Fix:

- AD-33 names Story 9.2 as owner of the `observe.Audit` API (event vocabulary, emitter, counters registered), with 9.3 and 9.4 as callers only. Or merge 9.3 and 9.4 into one story.
- AD-33 says the superseded auth lines are deleted in the same change as the first audit call that replaces them, and a test asserts exactly one line per event.
- Edit Story 9.4's references and drop "reuse what already fits".

## F2 (Critical): "one `prediction_saved` per `store.Prediction` row" is not observable in `internal/web`

Every `Save*` method returns only `error`: `SavePrediction`, `SaveSeriesPick`, `SaveDivisionPicks`, `SaveAwardPicks` (`store.go:669, 733, 819, 921`). AD-33 says web emits after success, once per row saved (AD-28's unit). The web layer cannot see which rows were upserted, so each handler author reconstructs it:

- `SaveDivisionPicks` upserts one `division_playoff_teams` row for every division in the map, because `parseDivisionsSubmission` (`sheet_divisions.go:242`) always puts all eight divisions in the map, including empty lists. Then it upserts one `division_winner` row per non-empty winner. Handler author A emits 8 + n lines. Author B emits one per division with a non-empty selection. Author C emits one per submit. All read "one per row saved" honestly.
- `SaveAwardPicks` silently skips awards without all three slugs (`awardHasAllFinalistSlugs`), and web pre-filters through `awardFinalistSlugsToSave`. An all-blank awards submit writes the file, returns 302, and saves zero rows. Author A emits nothing. Author B emits one `prediction_saved` because a redirect happened.
- An unchanged resubmission rewrites `SubmittedAt`. Is it saved? AD-33 does not say.
- Series (`handleSeriesSubmit`, `sheet_series.go:288`): each valid series is saved immediately, invalid ones are collected, and the result is a 200 re-render with errors while earlier series stay saved. Author A emits per row as each `SaveSeriesPick` succeeds. Author B emits only on the final 302 and misses every row saved on a partially rejected submit. The Story 9.3 AC "a rejected save moves no counter" is ambiguous here: the submit was rejected and rows were saved.
- Per-row emission gives up to 16 lines per divisions submit plus a `store write` line each time, while the story says "one line names the Player id and the kind of action".

Fix (new AD-35): the `Save*` methods return the list of `(Kind, key)` rows they actually wrote (empty when none). `internal/web` emits exactly one `prediction_saved` per returned row, after the call returns nil, and never on a re-render or error path. Decide explicitly: unchanged resubmission counts; empty division team lists do not count as a saved row; partially saved series count per saved row. Or drop to one event per submit with a `rows` count and label by set kind. Either is fine; the spine must pick.

## F3 (High): the example compose "mounted `fantasy-hockey.yml`" cannot save

Story 9.6 says "a mounted `fantasy-hockey.yml`". The repo's own `docker-compose.yml` comment explains why a single-file bind mount fails: the store writes via temp file plus `rename(2)`, which cannot replace a bind-mount point. An implementer of 9.6 who reads the story literally mounts the file (`./fantasy-hockey.yml:/data/fantasy-hockey.yml`). Login codes then fail to persist, every login returns 500, and no prediction saves. The dev compose works, the example compose does not, and both match AD-34.

Also: the example must not copy `user: root` from the dev compose (its comment says root is dev-only), so the mounted directory needs to be writable by the image's non-root UID. Nothing in the spine says so.

Fix (tighten AD-34 / Docker-compose section): the data file is provided by mounting its directory, never the file. State the UID the directory must be owned by for the non-root image. Rewrite the 9.6 AC accordingly. Add an acceptance check that the example compose starts and a login-code write succeeds.

## F4 (High): the Consistency table contradicts AD-33

The Observability row: "no Player, email or code in any label or log field". AD-33 requires `player_id` in every audit log line. One reader (a 9.4 implementer or a reviewer using the table as a checklist) deletes `player_id` from the log; another asserts in a test that no log line contains a player id. Story 9.4 requires the Player id in the line.

Fix: reword to "no email, login code or code hash in any label or log field; `player_id` allowed in log fields only, never in a metric label".

## F5 (High): Kind label vs log field vs the set id

AD-33: "save counters by `store.Kind` (AD-24, a bounded enum)". The existing handlers log `"kind", id` where `id` is the Prediction Set id (`sheet.go:356`). For cup, presidents and playoffcup, set id equals Kind. For series, the four set ids (`round1`, `round2`, `conferencefinals`, `stanleycupfinal`; see `seriesSetIDs`) all store rows with `Kind == "series"`. Divisions and awards do not match either: the set id is `divisions`/`awards`, the Kinds are `division_playoff_teams`, `division_winner`, `award`.

- Implementer A labels the counter with `store.KindSeries`. The bounded enum is 8 values (cup, presidents, playoffcup, division_playoff_teams, division_winner, award, series).
- Implementer B logs `kind=id` following the existing pattern, giving `round1`.
- A mismatch between the log and metric is exactly what AD-33 tries to prevent.

Also, the AD-33 log line carries only `event` and `player_id`, while Story 9.4 requires "the kind of action" in the line. A strict AD-33 implementer leaves out the kind.

Fix: AD-33 fixes the attribute list for `prediction_saved` as `event`, `player_id`, `kind` (the `store.Kind` value, never the set id), and lists the allowed Kind values through an exported slice in `internal/store` so `observe` cannot invent values. Add a test that the counter's label values equal the log's `kind` values.

## F6 (High): login event semantics are open

`RequestLoginCode` returns only `error` and is a deliberate no-op for an unknown email (`auth.go:37`). AD-33 says auth "hands the Player id back". Open questions that yield incompatible builds:

- Unknown email: does web emit `login_code_requested` with `player_id` omitted? A: yes, counted (inflated by anyone hitting the public port 80 form). B: no, since no code was requested. The counter then differs by the amount of spam and the log gives no signal of enumeration attempts.
- SMTP failure: the send runs in a goroutine, after web returns. If web emits `login_code_requested` in the handler, it is emitted before the send outcome is known, and the only "sent" evidence (`slog.Info("send login code")`) is removed by the supersede rule. The audit line then says "requested" for mail that never left. AD-33's "only when that outcome succeeded" is unanswerable: the outcome is asynchronous.
- `handleLoginCodeSubmit` has three failure branches: rejected code (401), `ok && playerID == ""` (401 plus `slog.Error`), and persist error (500). Which are `login_failed`? A counts only the first; B counts the first two; a 500 is neither, and is nowhere. The Story 9.3 AC "wrong code moves login-failure by exactly one" hides the difference.
- `POST /logout` is registered outside `requireSession` on purpose (`handleLogout`). It has no player context. A: emit `logout` always (any anonymous POST from the public internet inflates the counter and writes log lines); B: emit only when the presented cookie validates, which needs `auth.ValidateSession` in the handler.
- Two identical outbound requests: `login_code_requested` for a Player who requests twice in a second: two lines, two rows, two counters. Fine, but it should be stated.

Fix: AD-33 defines each event's exact trigger:

- `login_code_requested`: only when a Player matched and the code row was persisted; the send result is a separate `send login code` error line, unchanged. `RequestLoginCode` returns `(playerID string, err error)` with `""` for no match.
- `login_failed`: every 401 from `POST /login/code`, including the empty-id branch. A 500 emits nothing.
- `logout`: only when a valid session cookie was presented; anonymous logout emits nothing and counts nothing.

## F7 (High): metric names, labels and NewServer signature are unspecified

Stories 9.2 and 9.3 are built independently, and the spec names no metric. The other repo's scraper, dashboards and any alert rules depend on names that this repo defines.

- A names the HTTP series `http_requests_total{route,status}`; B names it `fantasy_hockey_http_requests_total{path,code}`. AD-32 mandates label names `route` and `status` for HTTP but nothing else.
- Login counters "labeled by `event` only" and save counters "by `store.Kind`": one metric with `event=` covering all five events, or two metrics? A: `audit_events_total{event}` for everything. B: `login_total{event}` plus `predictions_saved_total{kind}`. The `event` label then differs in whether `prediction_saved` is a value.
- Histogram buckets unspecified: default buckets vs custom. `promhttp` adds its own `promhttp_metric_handler_requests_total` only if the handler is wrapped in the way A does and B does not.
- The Story 9.3 ACs are written in terms of "the matching action counter", which no AD names, so the Gherkin scenario cannot be written against a shared name.
- `web.NewServer(st, send, secret)` is called from at least 10 acceptance-test files (`fixture_support_test.go:371`, `app_shell_steps_test.go:66`, `load_canonical_nhl_player_list_steps_test.go:62`, ...). AD-31 says the registry is "injected (AD-3)" but not how. A: add a 4th parameter `reg *observe.Registry`. B: `NewServer(st, send, secret, opts ...Option)`. C: `NewServer` builds the registry internally and returns it via a second return value. Every step file breaks differently, and story 9.2 and 9.3 conflict on the signature.

Fix (new AD-36): fix metric names and labels in one table:

| Metric                                  | Type      | Labels           |
|-----------------------------------------|-----------|------------------|
| `fantasy_hockey_http_requests_total`    | counter   | `route`, `status`|
| `fantasy_hockey_http_request_duration_seconds` | histogram | `route`, `status` |
| `fantasy_hockey_login_events_total`     | counter   | `event`          |
| `fantasy_hockey_predictions_saved_total`| counter   | `kind`           |

State the NewServer signature (pass `*observe.Observer` as a required parameter and update every call site in the same change), and the bucket list.

## F8 (High): route label extraction is easy to get wrong

AD-32: `route` is `r.Pattern` "read after the mux has routed". The middleware wraps the outer mux. Facts from the code:

- `requireSession` calls `next.ServeHTTP(w, r.WithContext(...))`, a shallow copy. The inner `authMux` sets `Pattern` on the copy, not on the request the metrics middleware holds. The outer mux's own dispatch sets `Pattern` on the original request, in place (Go 1.22+ `ServeMux.ServeHTTP` assigns `r.Pattern`), so the value is only visible after `next.ServeHTTP` returns. Implementer A reads `r.Pattern` before calling `next` (always empty, so every request is `unmatched`); B reads it inside a handler under `requireSession` (empty on the copy's parent).
- The response status must come from a recorder wrapper; the handlers here use `w.WriteHeader` explicitly (`renderTemplateStatus`), `http.Redirect`, `http.Error`, and implicit 200 writes. A wrapper that only captures `WriteHeader` reports status 0 for implicit-200 responses. A wrapper struct that hides `http.Flusher` or `io.ReaderFrom` changes behavior for `http.FileServerFS`.
- 405 responses (right path, wrong method) leave `Pattern` empty: collapsed into `unmatched`, so a `GET /logout` scan hides. Fine, but state it.
- The double registration of every authenticated pattern (outer mux plus `authMux`) means the label is the outer pattern. They are the same strings today (`shellRoutes`), so labels do not diverge, but the rule should say the label comes from the outer mux.

Fix: AD-32 adds: "the middleware records `r.Pattern` from the original request after `next.ServeHTTP` returns, uses a status recorder that defaults to 200 and delegates `Flush`/`Unwrap`, and has a test that `POST /predict/{id}` labels as the pattern, not `/predict/cup`, for a request that passes through `requireSession`".

## F9 (Medium): nginx and a recreated app container

AD-34's `proxy_pass http://fantasy-hockey:8080` is resolved by nginx once at config load. `depends_on` orders start-up but does not wait for readiness. If the app container is recreated (the Pi update mechanism is "deferred" but will pull a new image and recreate), the proxy keeps the old IP and returns 502, or exits at start if the name does not resolve. Both compose files behave the same, so they agree, and both are broken.

Fix: AD-34 mandates Docker's embedded resolver (`resolver 127.0.0.11 valid=10s;` plus a variable in `proxy_pass`) or a restart policy on the proxy, and a Story 9.1 AC for "app recreated, proxy recovers without restart".

## F10 (Medium): the shared config path

The Spine snippet shows `./nginx.conf:/etc/nginx/conf.d/default.conf:ro` for the dev compose at the repo root. In `docs/examples/docker-compose.yml`, a relative `./nginx.conf` resolves to `docs/examples/nginx.conf`, which would be a second file (violating AD-34's one-definition rule) unless the path is `../../nginx.conf`. Story 9.1 settles "the config file's location". Story 9.6 reads the spine, not 9.1. The mount for a copy-and-adapt example (the operator copies the file out of the repo to the Pi) then dangles: the referenced `../../nginx.conf` does not exist next to the copied file.

Fix: AD-34 fixes the path (`docs/examples/nginx.conf`, mounted by both compose files, dev via `./docs/examples/nginx.conf`) and states that the example must be self-contained when copied, so the example folder holds the config and the operator docs say to copy the folder.

Also: ls-lint and folderslint run in this repo's lint compose service; a new `nginx.conf` name or a `docs/examples` folder needs to pass them, and `yamllint` is run over `.` including the new compose file. The `task lint` AC in 9.6 covers this, but not the config file's location.

## F11 (Medium): free-text errors leak email

AD-33 keeps email out of audit lines. Existing error lines are untouched: `slog.Error("send login code", "error", err)` wraps SMTP errors such as `550 5.1.1 <name@example.com> User unknown`, and `mailer: send mail: %w` includes the server's reply. Those lines land in the same log stream as the audit log. The Story 9.4 AC "any of these lines contains no email" is about audit lines, so a test passes while the log still leaks the address. The Observability convention says "no email ... in any log field", which the SMTP error violates.

Fix: AD-33 says the SMTP error string is logged only after redaction of the recipient (or the mailer returns a classified error), and adds a test with a mailer that returns an error containing the address.

## F12-F14 (Low)

- F12: Prometheus counters with labels do not exist until first use. The 9.3 AC "the counter rose by exactly one" reads a missing series as absent, not 0. Fix in AD-36: pre-register every `event` and `kind` label value at startup.
- F13: `status` as exact code vs class (`2xx`) is not stated. `/metrics` is instrumented under its own pattern (noted as assumption); with a 15s scrape this adds a constant series, harmless. HEAD requests match `GET` patterns and count as the GET route. State once.
- F14: `slog.Info("audit", ...)` writes to `slog.Default()` unless a logger is injected. `main.go` passes `slog.Default()` to `openStore`. In-process acceptance tests cannot capture the default logger without mutating global state, so 9.4's "test approach to be decided" may pick a design that races with parallel tests. Fix: `observe` takes a `*slog.Logger` at construction.

## Checked and found sound

- `/metrics` inside `requireSession`: AD-32 puts it on the outer mux; the cookie re-issue (`requireSession`) would otherwise set a session cookie on every scrape. Correct.
- Nginx `location = /metrics` exact match: nginx normalizes the URI (decodes percent-encoding, collapses slashes) before matching, so `//metrics` and `/%6detrics` are blocked. `/metrics/` is forwarded, and the app's `GET /metrics` pattern gives 404, so it is not a bypass. Method-agnostic.
- Cardinality via `{id}` wildcard: the pattern, not the path, is the label. Bounded.
- Login-code brute-force lines: `login_failed` carries no code, so log injection or code leakage through the audit line is not possible via slog's quoting.
- AD-21 depguard for `observe`: `internal/web` has deny-list rules and no allow-list, so importing `observe` and `prometheus` from web passes. The auth allow-list (`clock`, `mailer`, `store`) blocks `observe` there as AD-31 intends.
- Rolled-back write: `Save*` restores the snapshot when `writeLocked` fails, and web returns 500 before any redirect, so "emit after nil error" is safe for the store side.

## Recommended new or changed ADs

1. AD-33: single owner (Story 9.2 builds `observe.Audit`); exact trigger per event (F6); attribute list including `kind` (F5); delete superseded auth lines in the same change (F1); SMTP error redaction (F11).
2. AD-35 (new): `Save*` return the rows written; web emits per returned row (F2).
3. AD-36 (new): metric names, labels, buckets, pre-registered label values, NewServer signature (F7, F12).
4. AD-32: middleware mechanics (F8, F13).
5. AD-34: data mounted as a directory, non-root UID, resolver, config path (F3, F9, F10).
6. Consistency table: fix the Observability row (F4).
7. Epics: reword Story 9.4 references, Story 9.6 "mounted file" AC, add an ordering note that 9.2 owns the emitter API.
