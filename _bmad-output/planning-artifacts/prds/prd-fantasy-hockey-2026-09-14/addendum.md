# Addendum: Fantasy Hockey PRD

Technical/implementation depth volunteered during the PRD conversation that belongs to a downstream document (`bmad-architecture`, `bmad-build`) rather than the PRD itself, plus conflict-resolution traceability with no home elsewhere in the PRD. Not audit/override information — see `.memlog.md` for that.

## Future consideration: write-queuing for `fantasy-hockey.yml`

Raised during PRD review, deliberately deferred for MVP.

`fantasy-hockey.yml` is the single source of truth for the app's state (results, deadlines, matchups, and — per the project's architecture baseline — participants/predictions as well). With only 3 trusted participants, the risk of colliding concurrent writes is accepted as small for now; no queuing, locking, or concurrency-control mechanism is being built for MVP.

**Revisit later:** if the file ever needs to support genuinely concurrent multi-writer scenarios (more participants, automated writers, or observed real collisions), a write-queuing mechanism for updates to this file is worth designing. Feed this into `bmad-architecture` as a noted-but-deferred concern, not a requirement.

## Local development: email capture

Raised during PRD review as a build/dev-tooling requirement, not a product FR (FR-1 request-login-code and FR-22 send-reminder-email are the product-facing requirements; this covers how to develop against them locally).

- Add a local email-capture server to the project's `docker-compose` setup for local development — one that accepts outbound SMTP traffic and exposes a web UI to view captured emails (e.g. a Mailpit/MailHog-style tool), so login codes and reminder emails can be inspected without a real mailbox.
- Email/SMTP settings (host, port, credentials, from-address, etc.) must be externally configurable (environment variables/config, not hardcoded), so the same code path can be pointed at the local capture server in development and at a real provider (Gmail, or another SMTP provider) in production, without a code change.

Feed this into `bmad-architecture` (email-delivery mechanism design) and the eventual `bmad-build` pass for Epic 1 (Authentication) and Epic 5 (Deadline Reminders).

## Conflict resolutions (traceability)

`assets/requirements.md` is the source document this PRD was built from; the user intends to delete it once `_bmad-output/` fully captures what's relevant. It was itself a reconciliation of two prior sources: the click-dummy prototype (`assets/clickdummy/`, the authoritative feature/UI-UX definition) and an earlier, now-retired BMad planning pass (`_bmad-output/`, the version before the "reset to a minimal foundation" baseline commit). Its standing rule: **where the two sources conflicted, the click-dummy wins.** The PRD's Features (§4) and Non-Goals (§5) carry forward only the *outcomes* of that reconciliation; the rationale below is preserved here purely for traceability, since it has no natural home in the PRD's FR-driven structure and would otherwise be lost when the source file is deleted.

- **Compare visibility (→ PRD §4.7, FR-25–FR-28).** The retired planning pass specified that another player's picks under a set stay hidden until that set's deadline closes. The click-dummy's Compare tab instead shows every player's picks for any chosen set at any time — its own sample data even shows "submitted" picks for sets whose deadline hasn't passed yet. Resolved in favor of the click-dummy: Compare is always fully visible, for every player, at any time.
- **Cup-pick scoring bucket (→ PRD §4.6, FR-24's point table).** The retired planning pass put both the season-opening Cup pick and the playoffs Cup re-pick in the Playoff points bucket. The click-dummy explicitly states the season-opening pick feeds the Regular total. Resolved in favor of the click-dummy: season-opening pick is Regular, playoffs re-pick is Playoff.
- **Series length notation (→ PRD FR-19/FR-20, Glossary "Series").** The retired planning pass specified series results as win-loss notation (e.g. "4-2"). The click-dummy records and displays a series purely as a game count (4–7) with a separately identified winner. Resolved in favor of the click-dummy: game-count notation, used for both predictions and results.
- **Admin/commissioner role (→ PRD §5 Non-Goals).** The click-dummy's own domain glossary calls this "implied, not yet a real role" and leaves it open. The requirements-baseline document resolved it further than either source: no in-app role or screen for it at all — results, deadlines, and matchups are maintained by directly editing `fantasy-hockey.yml` outside the app.
- **Finalist-name validation (→ PRD FR-17).** The click-dummy's player-award inputs offer free-text suggestions without enforcing a match, and its own text flags this as an open question. The retired planning pass's answer — validated entry, reject non-matching names — didn't contradict anything the click-dummy asserts as final, so it was carried forward as the resolution.

## Deployment & reverse proxy (added 2026-09-30 → PRD §4.10, FR-35–FR-36, §5)

For `bmad-architecture` and build; not requirements in themselves.

- **Motivation.** Run the app on a Raspberry Pi in a homelab; forward router port 80 to it so the other Players can reach it. The image is already published for `linux/amd64` and `linux/arm64`.
- **Proxy.** nginx, exposing port 80 and forwarding to the app container on 8080 over the compose network. Added to the root `docker-compose.yml` (dev; the app's direct `8080` publish stays for dev unless architecture decides otherwise) and to `docs/examples/docker-compose.yml`.
- **Example production compose.** Image `sommerfeldio/fantasy-hockey:latest`, mounted data YAML file, Mailpit for SMTP, dummy `SESSION_SECRET`. The user confirmed each is intentional because the file is only an example; production rollout happens in a separate repo with Ansible and an Ansible Vault for secrets.
- **Known consequences accepted by the user:** `latest` floats (a `pull` can change the running version, including across a season rollover — see the operator guide's rollover check); with Mailpit as SMTP, login codes are only visible in Mailpit's UI, so real mail delivery needs real SMTP in an actual deployment; a dummy secret must never be used on a public host.
- **No HTTPS/443.** Plain HTTP only, by decision. Login codes and session cookies are sent in cleartext and the app sets no `Secure` flag (`internal/auth/session.go`). Adding TLS later means terminating it at the proxy and setting the cookie's `Secure` flag.
- **Build notes.** Infra/config work: per the repo's `CLAUDE.md`, ask the human whether an acceptance test applies before skipping one. The Dockerfile and `.github/workflows/**` are protected and untouched.

## Observability (added 2026-09-30 → PRD §4.11, FR-37–FR-42, §5)

For `bmad-architecture` and build; not requirements in themselves.

- **Logs vs metrics (AI opinion, accepted).** "Who did what when" is an event record, so it goes to structured logs (Loki in Grafana Cloud can query and count them per Player over time); metrics carry rates and health only. With three Players a per-Player label would not blow up cardinality, but identity in metrics still leaks who is active to anyone who can scrape and adds nothing the log line lacks. Counters are therefore unlabeled by Player.
- **Reuse.** The app already emits `slog` lines for every data-changing write (epic 7, story 7-1) carrying ids and hashes only, never emails or raw login codes; audit events extend that convention. Player id, not email.
- **Metrics.** The standard Prometheus Go client's Go-runtime and process collectors, plus HTTP request count/duration by route pattern.
- **Exposure model.** `/metrics` is anonymous on the app's own port 8080. Alloy runs on the same host but *not* on the compose network, so it scrapes `localhost:8080/metrics`. Port 80 (public, via router forward) must never serve it: nginx returns 404 for `/metrics`. In the example compose, 8080 is published on loopback only.
- **Proxy access logs.** nginx default "combined" format to stdout (the official image symlinks `access.log` to `/dev/stdout` by default).
- **Out of scope, separate repo:** Alloy config to Grafana Cloud, the metrics dashboard, a proxy-access-log panel. **Future:** OpenTelemetry.
- **Build notes.** Behavior change in the app (endpoint, counters, log lines): per the repo's BDD rule this needs a Gherkin feature (e.g. `/metrics` returns 200 anonymously; a wrong code increments only the failure counter). The nginx/compose parts are infra: ask the human whether an acceptance test applies. Dockerfile and `.github/workflows/**` stay untouched.
