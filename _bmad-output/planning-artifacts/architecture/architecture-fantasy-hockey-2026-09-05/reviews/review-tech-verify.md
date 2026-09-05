# Tech Verification Review — Architecture Spine (Fantasy Hockey, 2026-09-05)

**Reviewed:** `ARCHITECTURE-SPINE.md` Stack table and every version-specific claim, cross-checked against the spine's `.memlog.md` and live web search performed on 2026-09-05.

## Method

For each version/tool claim I checked:

1. Whether the `.memlog.md` recorded it as web-verified (vs. asserted).
2. Whether a live web search on 2026-09-05 confirms the claim is currently accurate.
3. Whether a newer/better alternative exists with real evidence, not speculation.

## Findings

### 1. Go 1.26.4 — real version, but two newer patch releases now exist (STALE)

`Go 1.26.4` is a real release (per go.dev), and the memlog correctly marks this as `[ADOPTED] Brownfield ... ratified from existing src/ code` rather than a fresh web-researched decision — it matches `src/go.mod` and `Dockerfile` (`golang:1.26.4-alpine3.24`) exactly, so the *spine* is not misrepresenting the codebase.

However, as of 2026-09-05, Go has released two newer patches: **Go 1.26.5** (2026-07-07) and **Go 1.26.6** (2026-08-13). Go 1.26.6 specifically ships security fixes to `crypto/tls`, `encoding/asn1`, `encoding/xml`, `html/template`, `net`, `net/http`, `net/url`, and the `go` command — packages this app uses directly (`net/http`, `html/template`, `net/smtp`-adjacent networking). Since AD-6 gates the build on `govulncheck`, staying on 1.26.4 risks a vuln-gate failure once the CVEs fixed in 1.26.5/1.26.6 are indexed.

**Flag:** not a hallucination, but a staleness gap the spine doesn't surface. Per repo rules, `Dockerfile` is a protected file the AI must not modify without explicit ask — recommend the human decide whether to bump `go.mod`/`Dockerfile` to 1.26.6 before/alongside implementation, rather than silently carrying 1.26.4 forward as "current."

### 2. Alpine 3.24.0 (Dockerfile runtime stage) — same staleness pattern

`alpine:3.24.0` matches the existing `Dockerfile` (line 20) exactly, correctly adopted from brownfield reality. But Alpine has since released **3.24.1** (2026-06-13), a patch release. Same caveat as Go: accurately describes current repo state, but is not "the current version" if read as a live recommendation. Same protected-file caveat applies.

### 3. PostgreSQL 18 / `postgres:18-alpine` — confirmed accurate, verified

PostgreSQL 18 is the current stable major version; latest patch is 18.6 (2026-08-13, per postgresql.org). The spine correctly uses the floating `postgres:18-alpine` tag (not a pinned patch), which auto-tracks patches — a reasonable choice, no issue. Memlog explicitly logs this as web-verified.

### 4. `jackc/pgx v5.10.0` — confirmed current and compatible, verified

v5.10.0 (tagged 2026-06-03) is the newest pgx v5 tag as of 2026-09-05 (checked GitHub tags directly — v5.9.2, v5.9.1, v5.9.0, v5.8.0 are all older). It requires Go 1.25+ and Postgres 14+, both satisfied by Go 1.26.4/Postgres 18. Memlog correctly logs this as web-verified.

### 5. `golang-migrate/migrate/v4 v4.19.1` — confirmed current, verified

v4.19.1 (2025-11-29) is the newest tag on the releases page as of 2026-09-05; no v4.20.x exists. Memlog explicitly flags this was corrected from an earlier unverified "latest" placeholder — good practice, confirmed accurate.

### 6. `cucumber/godog v0.16.0` — matches `go.mod`, and is genuinely still latest, but wasn't logged as web-verified

`go.mod` pins `github.com/cucumber/godog v0.16.0`, matching the spine exactly. Checked GitHub releases directly: v0.16.0 (2024-07-31) is still the newest godog release — no v0.16.1/v0.17.0 exists as of 2026-09-05, so the two-year-old pin is not actually stale, just a slow-moving upstream.

**Minor flag:** unlike the Postgres/pgx/migrate/Gmail lines, the memlog has no entry recording that this version claim was checked against upstream (it was presumably just copied from `go.mod`, which is a reasonable source, but the memlog doesn't say so). Turned out correct, but the verification trail is thinner than the others.

### 7. Gmail SMTP App Password + 2-Step Verification requirement — confirmed accurate, verified

Confirmed current for 2026: Less Secure Apps access was retired in 2022, App Passwords require 2-Step Verification to be enabled, and this remains Google's supported path (alongside full OAuth2) for SMTP auth. AD-14 and the memlog's `(version)` line are accurate and were logged as web-verified.

### 8. ~500 recipients/day limit — accurate, with a nuance the spine doesn't need but should know about

Confirmed: free consumer Gmail accounts cap at 500 recipients/day (To+Cc+Bcc combined). One nuance not mentioned in the spine: **connecting over raw SMTP (as `internal/mailer` does via `net/smtp`) drops the per-message recipient limit to 100**, separate from the 500/day cap. Irrelevant at this app's scale (3 participants, individual emails), so not a correctness bug — just noting the spine's "~500/day" framing is accurate but slightly incomplete for an SMTP-specific implementation.

## Supersession Check

No evidence found that any named technology (golang-migrate, pgx, godog, Gmail SMTP via `net/smtp`) is being meaningfully superseded by something a 2026-current developer would reach for instead — all are still the actively-maintained, most-current option in their category. Not flagging speculative alternatives (e.g., OAuth2/XOAUTH2 instead of App Password, or goose/Atlas instead of golang-migrate) since I found no real evidence the incumbent choice is falling out of favor, only that alternatives exist.

## Overall

Every Stack-table claim checked out as accurate at the time of writing, and the memlog shows genuine web-verification effort (explicit `(version) ... verified via web search` lines for Postgres, pgx, migrate, and Gmail) rather than blanket assertion. The material gap is currency, not correctness: Go and Alpine are pinned to versions one-to-two patches behind the current release, inherited silently from brownfield reality without a "here's what's newer" check — worth a human call before build, especially given AD-6's `govulncheck` gate.
