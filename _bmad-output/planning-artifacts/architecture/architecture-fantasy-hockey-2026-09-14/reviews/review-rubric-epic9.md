# Rubric review — ARCHITECTURE-SPINE.md after the Epic 9 update

Scope: AD-14, AD-11, AD-21 (amended); AD-31..AD-34 (new); Design Paradigm, diagrams, Stack, Structural Seed, Capability map, Deferred. Checked against `src/internal/web/web.go`, `src/internal/web/login.go`, `src/internal/auth/*`, `src/.golangci.yml`, `src/main.go`, `docker-compose.yml`, PRD FR-35..FR-42, addendum "Observability", and Epic 9 stories 9.1-9.6.

## Verdict

PASS WITH CONDITIONS. The new ADs are well aimed, mostly enforceable and do not weaken AD-2, AD-3, AD-8 or AD-21 in substance. Three real divergence points are missed (one operational, one security, one contract), and the amended text leaves several stale or inconsistent statements. No Critical findings.

## Findings

| # | Severity | Finding                                                                                                          |
|---|----------|------------------------------------------------------------------------------------------------------------------|
| 1 | High     | No decision on abuse limits now that the app is internet-reachable (login-code brute force / email flooding)     |
| 2 | Medium   | AD-34: one nginx config file "mounted into both" composes does not survive the example being copied elsewhere    |
| 3 | Medium   | AD-34: nginx caches the upstream IP at startup; recreating only the app container yields 502                      |
| 4 | Medium   | AD-34: "default combined format" is not what the official image's http block uses; the directive is not pinned    |
| 5 | Medium   | AD-33 vs the code: `login_code_requested`, `logout` and `login_failed` have undefined player/emit semantics       |
| 6 | Medium   | AD-33 vs Story 9.4 vs `store write`: three sources of "one line per event", contradicting each other              |
| 7 | Low      | Who constructs the `observe` registry (main vs `web.NewServer`) is not fixed; AD-2/AD-8 text not updated          |
| 8 | Low      | Stale text: Design Paradigm "two support roles", Deferred TLS wording, AD-2 package list, AD-8 mailer/observe    |
| 9 | Low      | Enforceability gaps: no depguard deny for `client_golang` in `web`; no rule for future `predictions`; proxy config untested |

## Detail

### 1. High — abuse limits on an internet-reachable login (missed divergence / operational envelope)

FR-35 and the deployment diagram put the app on the public internet via router port-forward. The code has no attempt limiting: `auth.ValidateLoginCode(st, code)` looks the code up by hash across all players (no player binding), codes are 6 digits (`generateCode`, 1M space), valid 10 minutes, and multiple codes can be live at once (AD-20). `POST /login` also mails a code to a known player on demand, unthrottled. None of AD-11, AD-14, AD-20, AD-34 or Deferred addresses this. The Deferred TLS bullet says "revisit before the app is reachable beyond the three Players", which this epic arguably already does. Two builders could each pick a different place (nginx `limit_req`, an in-app counter, nothing). Fix: either decide (e.g. AD-34: `limit_req` on `POST /login` and `POST /login/code` in the shared config; login attempts are visible via the `login_failed` counter) or list it explicitly under Deferred as an accepted risk with a trigger.

### 2. Medium — shared nginx config vs the copyable example (AD-34, Structural Seed)

AD-34 requires "one committed config file mounted read-only into both" compose files. The dev shape shows `./nginx.conf:/etc/nginx/conf.d/default.conf:ro`, and `docs/examples/docker-compose.yml` (which does not exist yet) would need `../../nginx.conf`. But PRD FR-36 says real deployments copy and adapt the example in another repo (Ansible), where that relative path is meaningless, and the example uses the published `latest` image, which does not contain the config. The AD does not say whether the example is self-contained (inline `configs:` content or a sibling file under `docs/examples/`). That is a real fork for Story 9.6. Decide: sibling file `docs/examples/nginx.conf` plus a test/lint that it is byte-identical to the dev one, or inline `configs`. Also, the config file location is left to Story 9.1 while the Capability map and Source tree do not list it.

### 3. Medium — upstream IP caching (AD-34)

`proxy_pass http://fantasy-hockey:8080` resolves the name once at nginx start. The Deferred item names `docker compose pull && up -d` as the update path; that recreates only the app container, typically with a new IP, and nginx then returns 502 until restarted. AD-34 says nothing (resolver + variable, `restart` on both, or accept and document). Since the Pi update mechanism is the stated operating procedure, this should be decided or added to the runbook obligation.

### 4. Medium — "combined" is not the image default (AD-34 / FR-42)

The official nginx image's `nginx.conf` sets `access_log /var/log/nginx/access.log main;` where `main` is combined plus `"$http_x_forwarded_for"`. Mounting only `conf.d/default.conf` inherits `main`, not `combined`. FR-42 and Story 9.1 require exactly combined. The AD should pin `access_log /dev/stdout combined;` in the server block (and say `proxy_set_header Host`/`X-Forwarded-*` are or are not set; today unspecified). Not verified against a live image in this review; based on the documented image layout.

### 5. Medium — AD-33 semantics not matched to code

- `auth.RequestLoginCode(st, send, email) error` returns nothing about the player and returns `nil` silently for unknown emails (response must be identical, FR-1). AD-33 says auth "hands the Player id back", but does not say whether an unknown-email request emits `login_code_requested` (it must not distinguish in metrics/logs either, or the audit log becomes an enumeration oracle to log readers; and counters would count non-events). Decide: emit only when a player matched, or emit with no `player_id` always.
- `auth.ValidateLoginCode` returns the player id only on success; `login_failed` therefore never has one (AD-33 says so: fine). The AD says "omitted when no Player is known (`login_failed`)" as the only omission case; `logout` is on the outer mux without a validated session (`handleLogout`), so its player is derived from a possibly invalid cookie or absent. State: use `auth.ValidateSession` and omit id (or skip the event) when invalid.
- The existing `send login code` line is emitted after asynchronous SMTP success, a different moment than "requested". AD-33 says the audit line supersedes it; the semantic change (requested vs delivered) should be stated.

### 6. Medium — conflicting "reuse" guidance

AD-33 supersedes the `send login code` and `validate login code` lines. Story 9.4 says "reuse what already fits and fill only the gaps". `internal/store` already emits `store write` (store.go:1110) per data-changing write, so a saved prediction produces both `store write` and `prediction_saved`. AD-33 does not mention it; the PRD FR-40 "exactly one line" is per audit event, but a log consumer counting lines will see two. Reconcile the story with the AD and state that `store write` stays and is not an audit line (or is the audit line).

### 7. Low — registry ownership; AD-2/AD-3

AD-31 says the registry is "constructed once and injected (AD-3)" but not by whom. `web.NewServer(st, send, secret)` is called from `main.go`; tests build several servers in-process. Two builders could put construction in `main.go` (changing the signature) or inside `NewServer` (hidden, one per server). Only the latter needs no signature churn and is what the "several servers" rationale requires; say so. AD-33 also uses global `slog.Info`, which matches existing practice (`slog.SetDefault` in acceptance steps), but is not "injected" per AD-3; note it as a ratified exception. AD-2's rule text ("all application logic lives in ...") does not name `internal/observe`; add it so a reader does not think it forbidden or logic-bearing.

### 8. Low — stale or inconsistent amended text

- Design Paradigm: "three domain layers plus two support roles" now has three (gateway, observability, orchestration). AD-31 calls observe "infrastructure-layer, same tier as store/mailer/clock", while the table lists it as its own row.
- AD-8 (dependency direction, mailer "depended on only by...") was not updated for `web -> observe`; AD-8 is the rule AD-21 says the lint encodes.
- Deferred TLS bullet: "before reachable beyond the three Players" is now misleading (see 1).
- The mermaid layered diagram shows no `clock` node although AD-31 permits `observe -> clock`; harmless, but `observe` needs no clock (slog supplies the time), so the permission is unused surface.
- Stack: nginx `stable-alpine` is a moving tag with "verify tag before use"; no pinning policy stated (mailpit has the same). Acceptable for a homelab, but say it is deliberate.

### 9. Low — enforceability

- AD-31 "only `internal/web` imports observe": auth, scoring, standings and infrastructure rules are allow-lists and will reject it (verified in `.golangci.yml`). `internal/predictions` does not exist yet and has no rule, and `main.go`/`internal/server` have no rule at all (AD-2 is not lint-enforced today, pre-existing).
- `web` is deny-list based, so it can import `prometheus/client_golang` directly, bypassing `observe`; add a deny entry for it in the `web` rule (and `compare`).
- AD-31's proposed `observe` rule must be added in the same change as the package (the amended AD-21 says so; good). Note the existing `infrastructure` rule allows no internal package at all, so `observe -> clock` needs its own rule, as AD-31 states.

## Rubric walk

| Check                                              | Result                                                                                                             |
|----------------------------------------------------|--------------------------------------------------------------------------------------------------------------------|
| Fixes real divergence points, misses none          | Mostly; misses items 1-6 above                                                                                     |
| Each Rule enforceable and prevents stated harm     | AD-31 partly (item 9); AD-32 yes (testable: route label, 404 at proxy); AD-33 partly (items 5-6); AD-34 partly (2-4) |
| Nothing under Deferred lets units diverge          | TLS wording stale; abuse limits absent (item 1). Others (backups, multi-writer, telemetry collection) are safe     |
| Named tech verified current                        | `client_golang` v1.24.1 exists (checked with `go list -m -versions`), Apache-2.0, fits `go-licenses`; Go 1.26.6 matches `go.mod`; nginx/mailpit unpinned tags |
| Ratifies brownfield code                           | Yes: `NewServer` outer mux + `requireSession` on authMux match AD-2/AD-32; `SameSite=Lax`/`HttpOnly` match `session.go`; depguard claims match `.golangci.yml`. `r.Pattern` after routing works because `ServeMux.ServeHTTP` sets it on the same `*Request` the wrapper holds (Go 1.23+); `requireSession`'s `WithContext` copy does not disturb the outer request. Unit-test this |
| FR-35..FR-42 covered                               | All bound and mapped; FR-35 "unchanged responses" lacks the header/Host/cookie forwarding decision (item 4)        |
| New ADs do not weaken AD-2/3/8/21                  | AD-2 intact (`/metrics` in web, server untouched); AD-3 has the global-slog caveat (item 7); AD-8 text not updated (8); AD-21 strengthened |
| Operational envelope                               | Ports, exposure and access logs decided; abuse limits, upstream re-resolution, log handler format (text vs JSON for Alloy; `main` uses default text handler) undecided or unlisted |
| Internal consistency and mermaid validity          | Both diagrams are valid mermaid syntax (edge labels quoted, `-. "x" .->` form valid, nodes declared in the subgraph after first use resolve to it). Text inconsistencies listed in item 8 |

## Additional note

FR-40 asks for a "structured line". The app uses the default `slog` text handler (`slog.Default()` in `main.go`); AD-33 fixes attribute names but not the handler. Text `key=value` is parseable by Alloy/Loki, so this is fine, but the AD should say the handler is intentionally unchanged so nobody switches to JSON in one unit.
