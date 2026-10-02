# Web Verification Review — Architecture Spine (Fantasy Hockey, 2026-09-14)

**Reviewed file:** `_bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-14/ARCHITECTURE-SPINE.md`
**Reviewed against:** live web search / GitHub API / go.dev, run 2026-09-14.
**Scope:** every version-bearing decision in the Stack table plus version numbers named in Invariants & Rules and Structural Seed.

---

## Verdict

No hallucinated technology found. Every named tool/library exists and is spelled/pathed correctly (`axllent/mailpit`, `cucumber/godog`, `gopkg.in/yaml.v3`, `github.com/fzipp/gocyclo`, `google/go-licenses`, `golang.org/x/vuln`). One committed decision (`gopkg.in/yaml.v3`) points at a library that is deprecated in favor of a maintained fork — this should be corrected, not just flagged. A few other items are real-but-slightly-stale or carry a maintenance-risk signal worth the human's attention before this spine is treated as ADOPTED baseline.

---

## Findings by Severity

### High

**`gopkg.in/yaml.v3` (Stack table, row 2) — names a deprecated/archived library.**
`gopkg.in/yaml.v3` is now unmaintained; the YAML organization (which owns the YAML spec) forked it into `go.yaml.in/yaml/v3` as the maintained, drop-in-compatible successor that receives security updates — the old `gopkg.in/yaml.vX` branches are frozen to security-fix-only. Multiple real-world projects (Harbor, Forgejo, GitLab Runner, go-task) have open or merged migrations away from `gopkg.in/yaml.v3` for exactly this reason. `goccy/go-yaml` is a second, more actively-developed alternative some projects prefer. The spine's Stack table should name `go.yaml.in/yaml/v3` (drop-in, same v3 API surface — lowest-friction fix) rather than the deprecated path, or explicitly record the decision to stay on the frozen `gopkg.in/yaml.v3` with a rationale.

### Medium

**`google/go-licenses` (AD-5, Stack table) — real and current-ish, but the project itself signals it is not actively maintained.**
Confirmed to exist and be usable: latest tag `v2.0.1`, published 2025-09-08 (~1 year before this spine's date). The repository's own README/description states it is *not actively maintained* and *not an officially supported Google product*. It is not archived and still functions as a build-gate tool, so keeping it is defensible, but this is a real maintenance-risk signal the spine currently records as a flat "latest" with no caveat — worth a one-line acknowledgment (accepted risk) rather than silence.

**Alpine base (AD-6) — "Alpine" named unpinned.**
Alpine itself is still a legitimate, widely used minimal container base with no currency concern. The concern is specifically the *unpinned* usage: current (2026) container supply-chain guidance is to pin an exact Alpine version tag and ideally the image digest (e.g. `alpine:3.NN@sha256:...`) rather than a bare/untagged `alpine` reference, precisely to avoid silent drift or breakage on rebuild. AD-6 as written doesn't commit to a pinned tag. Since the Dockerfile itself is a protected file per project `CLAUDE.md`, this is advisory only — flag for the human, don't edit the Dockerfile from this finding.

### Low

**Go 1.26.6 (AD-1, Stack table) — real version, correct cadence, but two patch releases behind current as of today.**
Go's actual 2026 release history confirms the cadence and the version: `go1.26.0` shipped 2026-02-10 (6 months after 1.25, matching the Feb/Aug cadence), followed by monthly-ish patches — `1.26.1` (Mar 5), `1.26.2` (Apr 7), `1.26.3` (May 7), `1.26.4` (Jun 2), `1.26.5` (Jul 7), `1.26.6` (Aug 13). So `1.26.6` is a genuine, plausible, non-hallucinated release. However, `go.dev`'s official download index (checked 2026-09-14) currently lists `go1.26.8` and `go1.26.7` as newer stable patches released after `1.26.6`. Not a fabrication — just a couple of security/bugfix patches behind "current" at review time. Low severity since AD-1 says it's "confirmed against `src/go.mod`" (i.e., reality-checked against the actual repo, not asserted from training data) — recommend a bump to `1.26.8` (or later) before/at build time, or an explicit note that the pin is intentionally not tracking every patch.

**gocyclo (`fzipp/gocyclo`, AD-5/Stack) — alive but low-velocity.**
Confirmed to exist, not abandoned: latest tag `v0.6.0`, most recent commit 2025-12-27 (within the last year). An open GitHub issue literally asks "is this tool abandonware?", which reflects real community concern about response times to PRs/issues, but commit history shows it isn't dead. No action required; worth knowing golangci-lint also ships built-in `gocyclo`/`cyclop` linters, which is a possible future simplification (not a correctness issue with the current spine).

### Confirmed Current (no concern)

| Item                                    | Confirmation                                                                                                                                                                       |
|------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `cucumber/godog` v0.16.0                  | Verified via GitHub API: `v0.16.0` published 2026-07-31, is the actual latest release (supersedes `v0.15.1`, Jul 2025). This is the correct, current pin — well researched.       |
| golangci-lint                             | Very actively maintained; latest `v2.13.2` (2026-08-27). Note for awareness only: the tool is now on its v2 major line, whose `.golangci.yml` config schema differs from v1 — relevant context for AD-21's depguard config, not a defect in the spine. |
| govulncheck (`golang.org/x/vuln`)         | Actively maintained by the Go team; latest tag `v1.8.0`, recent activity confirmed (module published as recently as 2026-09-08). Still the standard Go vulnerability-gate tool.   |
| mailpit (`axllent/mailpit`)               | Real, current, and the standard MailHog successor: SMTP capture + web UI + API. `axllent/mailpit` is the correct official image name (confirmed via `mailpit.axllent.org/docs/install/docker/` and Docker Hub). Very active release cadence — latest `v1.31.1` (2026-09-05). No concerns; `:latest` for local-dev-only use is a reasonable, low-risk call (unlike the production Alpine base). |

---

## Summary Table

| Item                          | Verdict                  | Severity |
|--------------------------------|---------------------------|----------|
| Go 1.26.6                      | Real, cadence-correct, 2 patches behind | Low      |
| `gopkg.in/yaml.v3`              | Deprecated — successor exists | High     |
| `cucumber/godog` v0.16.0        | Confirmed current, exact match | None     |
| golangci-lint                  | Confirmed current, active | None (note: v2 config schema) |
| gocyclo                        | Alive, low-velocity       | Low      |
| go-licenses                    | Alive, self-flagged as not actively maintained | Medium   |
| govulncheck                    | Confirmed current, active | None     |
| `axllent/mailpit`               | Confirmed correct image, current, active | None     |
| Alpine (unpinned)               | Valid base, but pin is missing | Medium   |
