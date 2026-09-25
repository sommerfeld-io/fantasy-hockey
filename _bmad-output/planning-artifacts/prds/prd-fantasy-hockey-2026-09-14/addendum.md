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
