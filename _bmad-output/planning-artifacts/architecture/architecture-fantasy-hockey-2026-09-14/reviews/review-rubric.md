---
title: Architecture Spine Review — Fantasy Hockey
reviews: '_bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md'
updated: '2026-09-14'
---

# Review: Fantasy Hockey Architecture Spine

**Gate verdict:** Conditional pass — the spine is structurally sound and covers all 34 FRs with no orphans, but ships with one undecided data-model dimension for an in-scope FR (multi-season history) and two brownfield-mismatch/enforcement claims marked `[ADOPTED]` that do not hold against the actual `src/` codebase; fix those before treating the spine as a safe build substrate.

## Critical

### C-1. FR-34 ("a new pool each season, with history kept") has no architecture decision

- **Location:** `ARCHITECTURE-SPINE.md` — Capability → Architecture Map, row "Persistence & season data (FR-32..FR-34)"; AD-9; AD-25; AD-26; "Illustrative data-file shape" (`season: "2026-27"` as a single top-level scalar).
- **Problem:** AD-9 mandates a single `fantasy-hockey.yml` file; AD-25 resolves to exactly one active `DATA_FILE` path; AD-26 bootstraps that one file with "the current season." Nowhere does the spine decide how a second season's predictions/players/teams/results coexist with a first season's retained history — the illustrative schema models exactly one season (`season: "2026-27"` as a scalar, not a list). Two plausible, mutually exclusive resolutions exist (rotate to a new file per season and repoint `DATA_FILE`, archiving the old one; or restructure the schema into a `seasons:` list inside one ever-growing file) and the spine picks neither, nor lists this under Deferred/Open Questions even though FR-34 is explicitly in MVP scope (§6.1 of the PRD) and explicitly cited in the Capability Map. This is exactly the kind of foundational divergence point the spine exists to close off — two story implementers would very plausibly build incompatible solutions.
- **Fix:** Add a new AD (or extend AD-9/AD-26) that states explicitly how season rollover works — e.g. "at season rollover, the human running the pool archives the current `fantasy-hockey.yml` (e.g. rename to `fantasy-hockey-2026-27.yml`) and repoints `DATA_FILE`/`--data-file` at a fresh file; AD-26's bootstrap then creates the new season's skeleton. Prior seasons' files are retained on disk/volume as read-only history; there is no in-app cross-season query." If a single growing file is preferred instead, say so and update the illustrative schema to a `seasons:` list.

## High

### H-1. AD-21 claims `depguard` is already enabled — it is not

- **Location:** `ARCHITECTURE-SPINE.md` AD-21 ("`.golangci.yml` enables `depguard` with rules encoding AD-8's forbidden import edges... [ADOPTED]"); actual file `/workspaces/fantasy-hockey/.golangci.yml`.
- **Problem:** Verified against the repo: `.golangci.yml`'s `linters.enable` list is `errcheck, gosimple, govet, ineffassign, staticcheck, unused, gofmt, goimports, revive, gocritic, errorlint, unconvert, misspell` — no `depguard` entry anywhere, and no `depguard` config exists elsewhere in the repo (`grep -r depguard` returns nothing). `task go:lint` does run `golangci-lint run`, so the tool itself is wired up, but the specific rule this AD is about does not exist. This means AD-8's dependency-direction rule (presentation → domain → store, and "internal/mailer depended on only by auth/predictions") currently has **zero automated enforcement** — exactly the failure mode AD-21 says it "Prevents." Marking this `[ADOPTED]` will lead implementers to assume a safety net exists that doesn't.
- **Fix:** Either add the `depguard` rule to `.golangci.yml` now (mechanical, not a protected file) and re-verify it fails on a deliberately-bad import, or change AD-21's status from `[ADOPTED]` to a call-out that this gate is still to be built, so the first feature PR doesn't silently ship without it.

### H-2. AD-16 cites a `internal/clock.Now()` function that does not exist

- **Location:** `ARCHITECTURE-SPINE.md` AD-16 ("every date/timestamp... is an RFC3339 string, produced via `internal/clock.Now()`. [ADOPTED — `src/internal/clock/` already exists.]"); actual file `/workspaces/fantasy-hockey/src/internal/clock/clock.go`.
- **Problem:** The existing package exports `NowTime() time.Time` (UTC `time.Time`), not `Now()`, and it does not return a string, RFC3339 or otherwise — callers must format it themselves (`internal/web` currently does `clock.NowTime().Format(time.RFC1123)`, not RFC3339). AD-16's whole purpose is to prevent "ambiguous or inconsistently formatted timestamps across packages" — but as written, it points implementers at a function signature that doesn't compile, and provides no canonical RFC3339-formatting helper, so the actual risk (auth formatting `issued_at` one way, predictions formatting a deadline another way) is still open.
- **Fix:** Correct AD-16 to reference the real `clock.NowTime()` signature, and either (a) add a `clock.Now() string` (or similarly named) helper that returns `NowTime().Format(time.RFC3339)` as part of adopting this architecture, or (b) explicitly spell out the formatting call site convention (`clock.NowTime().Format(time.RFC3339)`) so every package does it identically.

### H-3. AD-27 doesn't say reads are synchronized against writes — a real data-race risk

- **Location:** `ARCHITECTURE-SPINE.md` AD-27 ("Every write updates that in-memory structure under a mutex..."); cross-referenced by AD-15 ("`internal/scoring` and `internal/standings` compute points on every read directly from `internal/store`'s in-memory... data").
- **Problem:** `net/http` serves requests concurrently. AD-15 requires the Leaderboard to recompute live, in-memory, on every read — which will run concurrently with AD-27's mutex-guarded writes (a prediction save). AD-27 only says writes take "a mutex"; it never states reads must also take a lock (or that the mutex is a `sync.RWMutex` with read/write halves), nor that `internal/store`'s read-side API returns a safely-copied snapshot. Left this way, two implementers will resolve it differently — one adds proper `RWMutex` read-locking, another reads the shared map/slice directly from a goroutine with no synchronization at all, which is a genuine Go data race (undetectable without `-race`, but real).
- **Fix:** Extend AD-27 (or add a companion AD) stating explicitly that `internal/store`'s in-memory structure is guarded by a `sync.RWMutex`; every exported read method takes the read lock (or returns a defensively-copied value) and every write takes the write lock before mutating and serializing.

## Medium

### M-1. AD-11 never decides session-cookie security attributes (Secure/HttpOnly/SameSite)

- **Location:** `ARCHITECTURE-SPINE.md` AD-11; AD-14 (plain HTTP now, reverse proxy/TLS "added later"); Deferred ("Reverse proxy / public exposure / TLS").
- **Problem:** AD-11 specifies the cookie is HMAC-signed but says nothing about `HttpOnly`, `SameSite`, or `Secure`. Given AD-14 pins the app to plain HTTP today with TLS arriving later via a reverse proxy, whether `Secure` is set now is a real fork in the road (set it now and login silently breaks over plain HTTP; omit it and risk forgetting to add it once TLS lands) — precisely the "two units diverge" case the checklist asks about.
- **Fix:** Add one line to AD-11, e.g. "the cookie is `HttpOnly` and `SameSite=Lax` always; `Secure` is set only when the app is told it's behind a TLS-terminating proxy (env var or trusted-proxy header), revisited when AD-14's reverse-proxy work lands."

### M-2. Operational envelope: image delivery to the Raspberry Pi host is neither decided nor deferred

- **Location:** `ARCHITECTURE-SPINE.md` — Structural Seed → Deployment topology; Deferred (only "reverse proxy / public exposure / TLS" and "backup/restore" are listed).
- **Problem:** `.github/workflows/release.yml` already builds and pushes the container image to Docker Hub (`REGISTRY: docker.io`) — a real, existing part of the operational envelope — but the spine's deployment-topology diagram and Structural Seed never mention it, and nothing says how the Raspberry Pi host actually picks up a new image (manual `docker compose pull && up -d`? a scheduled puller/watchtower? none exists today). The checklist specifically calls out "operations" as a whole-system dimension this altitude owns; leaving image delivery completely unmentioned (not even in Deferred) is a silent gap, however low-stakes for a 3-person hobby deployment.
- **Fix:** Add a line ratifying the existing Docker Hub publish pipeline as the image-distribution mechanism, and either state the host's update mechanism (even "manual `docker compose pull` on the Pi, no auto-update") or explicitly add it to Deferred.

### M-3. AD-23's enumeration omits the FR-33 team/NHL-Player lists, and the FR-32..34 capability-map row doesn't cite AD-23

- **Location:** `ARCHITECTURE-SPINE.md` AD-23 ("award finalists/winners, team results, series results, playoff matchups, and each Prediction set's deadline are all written by a human..."); Capability Map row "Persistence & season data (FR-32..FR-34) | `internal/store` | AD-9, AD-17, AD-25, AD-26, AD-27" (no AD-23).
- **Problem:** FR-33 requires the canonical team list and NHL Player candidate list to be hand-maintained, app-read-only — exactly AD-23's territory — but AD-23's own rule text never names "teams" or the "NHL Player candidate list" among the entities it covers, relying instead on AD-9's looser closing clause ("everything else is human-maintained (AD-23)"). The capability map then compounds this by not citing AD-23 at all for FR-32..34, even though AD-23 is the AD that actually guarantees "no in-app write path" for two of the three FRs in that row.
- **Fix:** Add "teams, and the NHL Player candidate list (FR-33)" to AD-23's enumerated list, and add AD-23 to the FR-32..34 capability-map governance column.

## Low

### L-1. "latest"-pinned build-gate tools are a reproducibility risk, and currency can't be verified from the doc alone

- **Location:** `ARCHITECTURE-SPINE.md` Stack table — `golangci-lint`, `gocyclo`, `go-licenses`, `govulncheck` all listed as version `latest`; matches `src/taskfile.yml`'s `go install .../golangci-lint@latest`.
- **Note (for the dedicated web-verification reviewer):** Go 1.26.6, module path, and `cucumber/godog v0.16.0` are directly verifiable against `src/go.mod` and check out. `gopkg.in/yaml.v3`, `golangci-lint`, `gocyclo`, `go-licenses`, and `govulncheck` cannot be confirmed current from the document alone — flagging for a web-checked pass, particularly `gocyclo` (historically low commit activity) and the unpinned `@latest` installs, which make CI builds non-reproducible across time even if each tool individually is still fine today. `mailpit` is already self-flagged in the spine ("verify current tag before use") — no action needed there.

### L-2. Minor terminology casing nit

- **Location:** `ARCHITECTURE-SPINE.md` AD-19 — "**Binds:** FR-17 (player awards autocomplete)".
- **Problem:** The PRD/UX's canonical rename capitalizes the glossary term as "Player awards"; the spine's bind line uses lowercase "player awards." Purely cosmetic — doesn't contradict meaning or create ambiguity — but worth aligning for consistency with the rest of the spine, which otherwise gets the FR-22/FR-23 terminology renames (Standings→Leaderboard, Awards (individual)→Player awards, Playoffs Cup re-pick→Playoffs Cup pick) and FR-22's deferral correct throughout.

## What checks out cleanly (not findings, noted for completeness)

- Terminology: "Leaderboard" (with an explicit, well-reasoned carve-out that `internal/standings` is an unaffected Go identifier), "Player awards," and "Playoffs Cup pick" are used consistently; FR-22's deferral is correctly reflected everywhere it's mentioned (AD-12, Capability Map, Deferred) with no contradiction.
- Capability → Architecture Map covers FR-1 through FR-34 with no gaps or overlaps.
- AD-1 (Go 1.26.6, module path), AD-4 (existing `.feature` files), AD-6 (multi-stage non-root Dockerfile), AD-7 (root `taskfile.yml` → `src/taskfile.yml` via `go:` namespace), and AD-14 (arm64 cross-compile via `TARGETARCH`/`GOARCH` in `src/taskfile.yml`'s `build` task) all verify accurately against the current `src/` codebase and Dockerfile.
- The spine's AD-24 renumbering (source's AD-28 → spine AD-24) and the relocation of the source's reminder-send-mechanics AD-24 into the Deferred section are both done correctly and match the source content closely.
- No item under "Deferred" creates a divergence risk in practice — each is either a whole undeveloped feature (FR-22) or is cross-referenced to a single canonical decision point (AD-24 for shared structs) that will absorb the eventual schema choice.
