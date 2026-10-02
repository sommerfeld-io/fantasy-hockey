---
name: 'Reconcile architecture-requirements.md → ARCHITECTURE-SPINE.md'
type: reconciliation-note
purpose: verify-no-silent-drop
created: '2026-09-14'
sources:
  - 'assets/architecture-requirements.md'
  - '_bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md'
---

# Reconciliation: `assets/architecture-requirements.md` → `ARCHITECTURE-SPINE.md`

Exhaustive line-by-line comparison of the prior planning document (source, to be deleted) against
the current spine (post adversarial + rubric review round). Verified relevant claims against the
actual repo (`src/go.mod`, `src/internal/clock/clock.go`) where the spine's own citations made a
factual claim about the codebase.

## Gaps found

1. **Stack table: YAML library renamed without a citation.** Source's Stack table (§4) lists
   `gopkg.in/yaml.v3` as the YAML dependency; the spine's Stack table lists `go.yaml.in/yaml/v3`
   instead. Unlike essentially every other substantive change in the spine, this rename carries no
   `[ADOPTED]`/`[RECONCILED]` tag or any inline note explaining the switch. Checked against
   `src/go.mod`/`go.sum`: only `gopkg.in/yaml.v3 v3.0.1` appears there today (as an indirect dep of
   `godog`; no direct YAML dependency exists yet since `internal/store` isn't built). The rename may
   be intentional (the go-yaml project's newer canonical home), but as written it's a silent,
   untraceable change to a named package the task specifically flagged for scrutiny.

2. **Consistency Conventions table contradicts its own AD-16.** The spine's Consistency Conventions
   table (Data & formats row) still reads `RFC3339 dates via internal/clock.Now() (AD-16)`. But
   AD-16 itself was explicitly corrected at spine review to say the real call site is
   `internal/clock.NowTime().UTC().Format(time.RFC3339)`, adding "no RFC3339-string helper exists
   yet, so this AD pins the formatting call site instead of inventing an unbuilt helper function."
   Confirmed against `src/internal/clock/clock.go`: it exports `NowTime() time.Time`; there is no
   `Now()` function. The correction landed in AD-16's prose but was never propagated to the
   Consistency Conventions table, which now states the disproven fact as fact.

3. **AD-12's explicit email-content ceiling isn't restated anywhere.** Source AD-12 ends with "No
   email content beyond login codes and deadline reminders is ever sent" — an explicit scope fence
   on what `internal/mailer` may ever be used for. The spine's AD-12 (reconciled for configurable
   SMTP, correctly cited as `[RECONCILED ... Finding 2]`) drops this sentence; it doesn't appear
   elsewhere in the spine either. Low-severity (the only two email types the spine actually defines
   are still login codes and — once un-deferred — reminders), but the rule itself, as a rule, is not
   carried forward.

4. **Minor prose drop, AD-2.** Source AD-2: "...no business logic, no mode dispatch." Spine AD-2
   drops "no mode dispatch" (kept "no business logic"). Unlikely to matter in practice; flagging for
   completeness since the task asked for exhaustive comparison.

5. **Stack table lost its Purpose column.** Source's Stack table has three columns (Name | Version |
   Purpose); the spine's has two (Name | Version). Each dependency's purpose is still recoverable
   from the ADs that reference it (e.g. `net/smtp` ↔ AD-12), so nothing is actually lost, but the
   table itself is less self-contained than the source's was.

## Confirmed captured

- **AD-1 through AD-27 (source numbering) all present and accounted for**, either adopted verbatim
  or amended with an accurate, checkable citation:
  - AD-1 (Go 1.26.6, module path) — `[ADOPTED — confirmed against src/go.mod]`; verified `go.mod`
    says `go 1.26.6` and the module path matches.
  - AD-2 (`main.go` wiring-only) — extended to name `internal/server`, cited
    `[ADOPTED — internal/server named explicitly at spine review to close Finding 6]`.
  - AD-3, AD-4, AD-5, AD-6, AD-7, AD-14, AD-18, AD-22, AD-25, AD-26, AD-27 — adopted essentially
    verbatim (AD-4's citation additionally notes two `.feature` files already exist; AD-27's citation
    correctly cross-references the PRD addendum's write-queuing deferral).
  - AD-8 (layered architecture) — amended with the `standings → scoring` sanctioned import
    exception, cited `[ADOPTED, exception added at spine review]`; consistently reflected in the
    Design Paradigm mermaid diagram and in AD-15.
  - AD-9 (single YAML file, no DB) — condensed but all substantive content preserved, including the
    "what the app actually writes" split and the accepted single-writer constraint.
  - AD-10 (server-rendered, minimal JS) — amended with the live-cap-checkbox JS exception, cited
    `[ADOPTED, live-cap exception added at spine review to close Finding 7]`.
  - AD-11 (stateless cookie sessions) — amended with `HttpOnly`/`SameSite=Lax`/deferred-`Secure`
    cookie attribute policy, cited `[ADOPTED ... to close Medium finding M-1]`.
  - AD-12 (SMTP) — reconciled from hardcoded Gmail (`smtp.gmail.com:587`) to fully configurable
    `SMTP_HOST`/`SMTP_PORT`/`SMTP_USERNAME`/`SMTP_APP_PASSWORD`, cited
    `[RECONCILED ... to close Finding 2]`; the change is consistently threaded through the
    Consistency Conventions table, the Stack table (mailpit addition), and the docker-compose shape
    (local dev now depends on a `mailpit` service — the source's "no `depends_on`" line no longer
    applies, but that's a direct, traceable consequence of this same AD-12 change, not a silent
    contradiction).
  - AD-13 (App Password secret) — retitled "SMTP App Password" and scoped "in production," matching
    AD-12's change; rule content otherwise unchanged.
  - AD-15 (live-computed scores) — retitled "Leaderboard," amended to route through AD-8's
    `scoring`/`standings` exception, cited and internally consistent.
  - AD-16 (RFC3339 timestamps) — corrected call site, cited and verified accurate against
    `src/internal/clock/clock.go` (see Gap #2 above for the one place this correction didn't
    propagate).
  - AD-17 (ID strategy) — extended with the NHL Player slug rule (FR-33), cited
    `[ADOPTED, NHL Player identity gap closed at spine review]`.
  - AD-19 (embedded autocomplete data) — extended with the "submit `id` never `label`" rule, cited
    `[ADOPTED, submitted-value rule added at spine review to close Finding 5]`.
  - AD-20 (LoginCode hashed) — enriched with the 10-minute window from PRD FR-2 (source left this
    unquantified); not a contradiction, an enrichment.
  - AD-21 (depguard) — honestly downgraded to `[NOT YET ADOPTED]` after verifying `.golangci.yml`
    doesn't currently enable `depguard`; flagged as a near-term build task rather than silently
    presented as done.
  - AD-23 (human-maintained data) — extended with the FR-33 team/NHL-Player-list read paths, cited
    `[ADOPTED, FR-33 lists added at spine review]`.
  - AD-24 in the source (reminder-send mechanics, synchronous/no-ticker) — correctly moved to the
    spine's Deferred section (FR-22 itself is deferred), with its "no ticker, no dedupe log, no
    goroutine/queue" content preserved verbatim there.
  - AD-28 in the source (`internal/store` owns entity structs) — correctly renumbered to the spine's
    AD-24, cited `[ADOPTED — renumbered from the source's AD-28 ...]`; also resolves a real
    self-contradiction in the source (source's illustrative YAML showed `award_finalists` as plain
    `["...", "...", "..."]` strings while source AD-28 implied a `store.AwardFinalist` struct) by
    pinning the struct shape `{Slug, DisplayName}` and updating the illustrative YAML to match.
- **New ADs (AD-28, AD-29, AD-30) are genuinely new** — row granularity, store method-only
  synchronization, and season rollover — not reconciliations of anything in the source, so nothing
  to check them against; they don't contradict any surviving source content.
- **Consistency Conventions table**: Naming row and State & secrets row both correctly updated
  (added `server` to the package list; SMTP env vars expanded to four, both citing AD-12/AD-13).
  Only the Data & formats row's `internal/clock.Now()` reference is stale (Gap #2).
- **Stack table**: Go version, godog version (`v0.16.0`, matches `go.mod` exactly), golangci-lint /
  gocyclo (complexity limit 10) / go-licenses / govulncheck all carried forward unchanged; `flag`
  stdlib and `mailpit` added as legitimate consequences of AD-25 and AD-12 respectively. Only the
  YAML package identity is uncited (Gap #1).
- **Docker Hub / CI details**: source doesn't mention Docker Hub, `docker.io`, or any CI/release
  pipeline detail at all — confirmed by re-reading the source in full. The spine's Deployment
  topology section adds this (Docker Hub push via `.github/workflows/release.yml`, Pi update
  mechanism deferred) as genuinely new information, not a reconciliation of anything in the source,
  and it doesn't contradict anything the source said.
- **Numeric thresholds**: gocyclo complexity limit 10 (AD-5), 30-minute sliding session timeout
  (AD-11) both carried forward unchanged. LoginCode window went from unquantified in the source to
  an explicit 10 minutes in the spine (PRD FR-2) — an enrichment, not a contradiction.
- **Source tree**: the full source package list (`web/`, `auth/`, `predictions/`, `scoring/`,
  `standings/`, `mailer/`, `store/`, `clock/`, `acceptance-tests/features/`) survives unchanged; only
  addition is `internal/server/`, explicitly justified immediately below the tree as an existing
  package now ratified into the Orchestration layer, matching AD-2's change.
- **Illustrative data-file shape**: timestamps changed from `Z`/UTC to `+02:00`/Europe/Berlin,
  annotated inline "per PRD FR-6" — traceable. `award_finalists` shape fixed to match the corrected
  AD-24/AD-28 struct (see above). All `AD-28` comment references in the YAML correctly renumbered to
  `AD-24`.
- **Deferred section**: all four source items (scheduled reminders, backup/restore, multiple writer
  processes, reverse proxy/TLS) survive with equivalent or expanded content; two new items (Pi image
  update mechanism, exact data-file schema) added, both tied to genuinely new spine content, not
  reconciliations.
- **Capability → architecture map**: restructured around PRD FR ranges rather than the source's
  feature names, but every source row's content is traceable into the new rows (including the
  AD-28→AD-24 renumbering carried through consistently in every cell that cites it).
