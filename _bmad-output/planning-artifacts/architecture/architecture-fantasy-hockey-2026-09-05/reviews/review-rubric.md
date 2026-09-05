---
title: 'Architecture Spine Review — Fantasy Hockey (rubric-based)'
target: _bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-05/ARCHITECTURE-SPINE.md
reviewed: '2026-09-05'
---

# Review: ARCHITECTURE-SPINE.md against the good-spine rubric

Context read in full: `ARCHITECTURE-SPINE.md`, `.memlog.md`, the driving PRD
(`prd-fantasy-hockey-2026-09-04/prd.md`), and the brownfield skeleton
(`src/main.go`, `src/go.mod`, `src/internal/clock/clock.go`, `src/Dockerfile`,
`src/taskfile.yml`, root `taskfile.yml`, `.golangci.yml`). Version claims were checked
against live web search rather than taken on faith.

## Overall verdict

This is a tight, well-reconciled spine — every [ADOPTED] claim checks out against the
actual repo, every named version checks out against current releases, and the memlog shows
real reconciliation work (PRD/UX/research gap-hunts that produced AD-21–AD-25). The findings
below are narrow and real, not padding: one enforcement gap on the paradigm's central rule,
and two silent dimensions (schema-migration ownership, Season-zero bootstrap, session-secret
lifecycle) that a downstream implementer would have to guess at.

## Checklist findings

### 1. Fixes real divergence points — mostly yes, three silent gaps

- **[MODERATE] Who runs `golang-migrate`, and when, is never decided.** `golang-migrate/migrate/v4`
  is pinned in the Stack table, and the ERD/source tree imply a schema exists, but no AD and no
  line in the docker-compose additions says whether migrations run automatically on `serve`
  startup, on `sync` startup, both (a race, even if migrate's locking makes it safe), or via a
  separate `migrate` mode/init step. Two independently-built services in the same compose file
  could each assume the other applies the schema, or both try to and rely on migrate's advisory
  lock to save them — undocumented, not decided here.
- **[MODERATE] Season-zero bootstrap is undecided.** FR-15 says "when a Season is created, the
  current NHL Team list is loaded," and MVP scope is explicitly single-Season, but nothing in the
  spine says *what* creates that one Season row for v1 — a migration seed, a one-time flag, first-run
  logic in `mode=sync`? Every feature package scopes by Season, so this is a real foundational gap,
  not a cosmetic one.
- **[MINOR] Session-signing secret's source/lifecycle is unstated.** AD-12 requires an HMAC-signed
  cookie but never says the signing key is a runtime secret (unlike AD-14's explicit treatment of the
  Gmail App Password). The Consistency Conventions "Config/secrets" row lists DB connection string,
  SMTP credentials, and `SYNC_INTERVAL` — not a session key. If an implementer generates the key
  in-process instead of reading it from an env var, every container restart silently logs out all
  three Participants — a real, user-visible bug this rebuild would otherwise never surface.

### 2. Every AD's Rule is enforceable — one real gap on the central rule

- **[NOTABLE] AD-9's layering rule and AD-23's "`sync` never imports `predictions`" rule have no
  automated enforcement.** `.golangci.yml` enables errcheck, gosimple, govet, ineffassign,
  staticcheck, unused, gofmt, goimports, revive, gocritic, errorlint, unconvert, misspell — no
  `depguard` or import-boundary linter. AD-6's build gates (golangci-lint, gocyclo, go-licenses,
  govulncheck) do not check package-import direction anywhere. So the paradigm's one dependency
  rule, and the one AD written specifically to prevent a named coupling bug, both rely entirely on
  code-review discipline rather than a gate that fails the build. This is the single highest-leverage
  fix available: a `depguard` rule per package would make AD-9/AD-23 actually self-enforcing.
- All other ADs are concrete and testable (AD-17's post-decode field-presence check, AD-25's
  server-side-filter-before-render, AD-22's per-code-row model) — no vagueness found there.

### 3. Deferred section — safe

Reviewed all six Deferred items; none leave a structural seam between two independently-built
units undefined. Each is either a tunable parameter (reminder timing, backoff bound,
`SYNC_INTERVAL`), a research lead, or an explicitly out-of-scope concern (reverse proxy, ToS) whose
interface contract (AD-15's plain-HTTP boundary) is already fixed. No finding here.

### 4. Named technology — verified current, no stale/guessed versions found

Checked live: PostgreSQL 18 (18.6 as of Aug 13, 2026 — matches memlog's own verification note),
`jackc/pgx` v5.10.0 (released June 3, 2026, still latest — also clears CVE-2026-33816, fixed in
5.9.0), `golang-migrate/migrate/v4` v4.19.1 (Nov 2025, still latest at time of review), `cucumber/godog`
v0.16.0 (matches `go.mod` exactly). Go 1.26.4 matches `go.mod` exactly — note Go 1.27 shipped Aug 19,
2026 (1.27.1 on Sept 1), so the pinned 1.26 line is no longer bleeding-edge, but it's an accurate
[ADOPTED] ratification of the existing brownfield toolchain, not a fabricated claim, and 1.26 remains
in Go's supported two-release window. Not a finding, just a freshness note for the next spine revision.

### 5. Ratifies brownfield — confirmed, no contradictions

Cross-checked every [ADOPTED] AD against the actual files: `go.mod` (Go 1.26.4, module path exact
match), `main.go` (wiring-only, `run(out io.Writer, now func() string)` DI pattern exactly as AD-4
describes), `internal/clock/clock.go` (RFC3339 via `time.Now().Format(time.RFC3339)`, matches AD-18
exactly), `Dockerfile` (`golang:1.26.4-alpine3.24` → `alpine:3.24.0`, non-root user, matches AD-7
exactly), `src/taskfile.yml` (golangci-lint, gocyclo -over 10, go-licenses, govulncheck all present,
matches AD-6 exactly), root `taskfile.yml` (`go:` namespace delegation, matches AD-8). Zero
divergence found between spine claims and repo reality.

### 6. PRD capability coverage — complete

Every §4 feature (4.1–4.8) and every FR (FR-1–FR-22) maps to at least one AD in the Capability →
Architecture Map, and the mapping is substantively correct on spot-check (e.g. AD-25 maps to FR-12's
"not shown as blank or masked — simply not present" with an AD rule that says exactly that: filter
server-side before template render). Cross-Cutting NFRs are all covered except one: "Observability
without in-app alerting" is covered on its *negative* half (no in-app alerting — stated in Consistency
Conventions) but not its positive half (what mechanism actually produces the "externally exported
metrics/logs"). Low severity: `src/CLAUDE.md` already establishes stdout/stderr + structured-logging
convention at the Go-style level, so this is a thin gap, not a silent dimension.

### 7. Dimension coverage at this altitude — operational envelope mostly covered, migrations/backup thin

- Deployment & environments: covered well (AD-15, deployment topology diagram, docker-compose
  additions) — appropriately sized for a single-host hobby deployment, correctly not over-built.
- Infra/provider strategy: covered (self-hosted Raspberry Pi + Docker Compose, no cloud provider
  needed) — correctly out of scope beyond that.
- Operations: **[MINOR] no mention of `pgdata` volume backup/restore anywhere, including Deferred.**
  For a season-long, single-copy game record, an unrecoverable disk failure means redoing a season by
  hand — the exact kind of trust-eroding failure this rebuild exists to prevent, just moved from
  "scoring" to "storage." Not enterprise-grade backup policy, just a one-line Deferred entry ("volume
  backup strategy") would close this without adding scope this project doesn't need.
- Migrations and Season bootstrap: see finding #1 above — the two clearest silent dimensions.

## Summary of findings by severity

| Severity | Finding                                                                                      |
|----------|-----------------------------------------------------------------------------------------------|
| Notable  | AD-9/AD-23 layering rules have no automated enforcement (no depguard/import-boundary lint gate) |
| Moderate | No AD decides who/when runs `golang-migrate` migrations                                        |
| Moderate | Season-zero bootstrap (who creates the single v1 Season row) is undecided                      |
| Minor    | Session-signing (HMAC cookie) secret's env-var source/lifecycle is never stated (AD-12 gap)      |
| Minor    | No mention of `pgdata` volume backup/restore, not even in Deferred                              |
| Minor    | Observability NFR's positive half (logging mechanism) left to `src/CLAUDE.md` convention only    |
