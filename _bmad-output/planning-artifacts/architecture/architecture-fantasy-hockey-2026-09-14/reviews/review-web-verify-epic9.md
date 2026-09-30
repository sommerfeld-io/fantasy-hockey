# Web/reality verification: Epic 9 spine additions (AD-31..AD-34)

Date: 2026-09-30. Scope: ARCHITECTURE-SPINE.md Stack rows (prometheus/client_golang, nginx) and AD-31..AD-34. Method: GitHub releases page, Docker Hub API, and executable experiments in a scratch module (Go 1.26.6, real nginx container).

## Verdict

PASS with minor notes. Every checked decision holds. No blocking findings.

## Checks

| # | Claim                                                                   | Result   | Evidence                                                                                                                                                                                       |
|---|-------------------------------------------------------------------------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1a | client_golang v1.24.1 is current                                       | Verified | Latest release, 2026-07-24 (promhttp nil-URL panic fix). `go get @latest` resolves to v1.24.1.                                                                                                 |
| 1b | Go minimum vs go.mod 1.26.6                                            | Verified | v1.24.x requires Go 1.25+ (supports 1.25 and 1.26). `go get` left the `go 1.26.6` directive unchanged.                                                                                         |
| 1c | License and transitive licenses vs go-licenses gate                    | Verified | Ran the repo's exact command (`--include_tests`, allowed MIT/Apache-2.0/BSD-2/BSD-3/ISC/Unlicense): exit 0. Deps: perks MIT, xxhash MIT, goautoneg BSD-3, gddo BSD-3, client_model/common/procfs Apache-2.0, x/sys BSD-3, protobuf BSD-3. |
| 1d | Other gates                                                            | Verified | `govulncheck` clean on a minimal handler; `CGO_ENABLED=0 GOARCH=arm64` build succeeds (xxhash has pure-Go fallback).                                                                            |
| 2  | `r.Pattern` visible to a wrapper around the outer mux after ServeHTTP  | Verified | Tested a nested outer mux -> clone via `r.WithContext` -> inner mux. Wrapper read `"GET /predict/{id}"`, `"GET /login"`, `"GET /static/"` (StripPrefix handler). The outer mux sets Pattern on the request pointer the wrapper holds, so inner-mux and requireSession clones do not matter. |
| 3a | nginx:stable-alpine exists for arm64                                   | Verified | Docker Hub: tag has arm64/v8 (also arm v6/v7, amd64); pushed 2026-09-23; resolves to nginx 1.30.5.                                                                                              |
| 3b | `location = /metrics { return 404; }`                                  | Verified | Ran the container: `/metrics` and `/metrics?x=1` -> 404, no upstream contact; also `/%6Detrics`, `//metrics`, `/x/../metrics` -> 404 (nginx normalizes before matching).                        |
| 3c | Default combined access log to stdout                                  | Verified | `docker logs` shows combined-format lines (the official image symlinks access.log to stdout).                                                                                                  |
| 4  | NewRegistry + GoCollector + ProcessCollector + promhttp.HandlerFor     | Verified | Two independent registries built in one process (no duplicate-registration panic); output contains `go_goroutines`, `go_gc_duration_seconds`, `process_cpu_seconds_total`. Confirms the AD-31 rationale for avoiding the default registry. |

## Findings

| Sev | Finding                                                                                                                                                                                                                                                                                          | Suggested action                                                                                                                                       |
|-----|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|
| Low | Stack row and compose snippet still say "verify tag before use" for nginx. It is now verified (arm64 present, 1.30.5). `stable-alpine` is a floating tag, so builds drift silently.                                                                                                              | Replace the caveat with "verified 2026-09-30, arm64 present"; optionally note pinning a minor (`1.30-alpine`) as a story decision.                      |
| Low | AD-32 says an empty `r.Pattern` becomes `unmatched`. Unmatched paths and method mismatches (e.g. `POST /login` -> 405) both give an empty pattern, so 404 and 405 collapse into one route label; `status` still distinguishes them.                                                              | None required; optionally state that `status` carries the distinction.                                                                                  |
| Low | `/metrics/`, `/METRICS` are not blocked by the proxy (exact match, case-sensitive) but the app has no such route, so they 404 there. Safe today; a future route with those spellings would be exposed.                                                                                           | Optionally add a note that the proxy block is an exact-path rule, not a prefix rule.                                                                    |
| Info | Middleware must wrap the ResponseWriter to capture `status`; `promhttp.InstrumentHandler*` labels by `code`/`method` and cannot use `r.Pattern`, so a small custom middleware is required (as AD-32 implies). Depguard prefix match on `github.com/prometheus/client_golang` covers its subpackages. | None.                                                                                                                                                  |

## Notes

- Adding the dependency pulls roughly 8 indirect modules into go.mod/go.sum (perks, xxhash, client_model, common, procfs, protobuf, x/sys, goautoneg); all pass the license gate.
- Scratch experiments lived in the session scratchpad, not the repo. Spine not edited.
