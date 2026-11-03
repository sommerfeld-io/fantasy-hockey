---
title: 'Operator Observability Section in docs/operator-guide.md'
type: 'docs'
created: '2026-11-03'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Epic 9 added `/metrics` and audit log lines, but `docs/operator-guide.md` never tells the person running the pool what to scrape, which series and audit events exist, or why `/metrics` must stay off the network (Epic 9 retro action item 69, findings F4 and F5). The example paragraph also does not point at the checklist row saying a freshly bootstrapped data file has no players.

**Approach:** Add an observability section to `docs/operator-guide.md` covering:

- the scrape target `127.0.0.1:8080/metrics`
- the four `fantasy_hockey_*` series and their labels
- the five audit events and the shape of their log line
- that `login_code_requested` fires only for an email that matches a Player, so it reveals which emails matched and `/metrics` must never be published on all interfaces
- the proxy's access log on stdout

Link the example paragraph to the checklist's first-deploy rows. The item's "published `latest` image predates `/metrics`" warning is out of date: every release since v0.1.0 serves `/metrics` (v0.3.1 is the latest), and item 73 confirmed it on the Pi. The section therefore states the current fact instead of that warning. Mark item 69 `done`. Commit onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract` (user instruction: no new branch).

</frozen-after-approval>

## Implementation Notes

- `docs/operator-guide.md`: added `## Observability` (scrape target, Metrics table, Audit log table and example line, the matched-email signal, the send-failure line, log retention, proxy access log) before `## Checklist`. The example paragraph now says the bootstrapped file has only `season` and an empty `players:` list, and links to the checklist and to Observability.
- Checked against the code:
    - The series names, the Go runtime and process collectors, and the `Audit` behavior are in `src/internal/observe/observe.go`.
    - The audit call sites, with their `kind`/`set` attributes and empty `player_id` on `login_failed`, are in `src/internal/web/login.go` and `sheet.go:394`.
    - Route patterns, including the method, are in `src/internal/web/web.go:103-108`.
    - The `send login code` error with `player_id` is in `src/internal/auth/auth.go:59`.
    - Nothing calls `slog.SetDefault`, so audit lines go to stderr.
    - The `kind` values come from `store.Kinds`.
- Deviation from the item text, as stated in the Intent: the item asked to warn that the published `latest` image predates `/metrics`. That is out of date, because `src/internal/observe` is in every tag from v0.1.0 on and item 73 confirmed it on the Pi. The guide states the current fact instead.
- `sprint-status.yaml`: marked item 69 `done`.
- Committed onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract`, as the user instructed.
- No code changed, so `task go:run` does not apply. `task lint` passes, including the markdown link and anchor checks.

## Review Triage Log

- Blind Hunter: the doc said audit lines go to stdout, but they go to stderr. Verdict: medium. Confirmed: no `slog.SetDefault` call. Patched, and an example line added.
- Blind Hunter: the first-deploy rows don't mention `players:`. Verdict: medium. Patched: the link sentence names the players and the "A player joins" row.
- Blind Hunter: the link went to the whole checklist, not the row. Verdict: low. Patched together with the above, since table rows have no anchors.
- Blind Hunter: the bootstrap sentence was imprecise and in the wrong place. Verdict: low. Patched: it now sits after the curl check and names `season` and the empty `players:` list.
- Blind Hunter: "unknown email logs and counts nothing" was wrong, because the HTTP counter still records the request. Verdict: medium. Patched, and the HTTP-counter gap is named as a second signal.
- Blind Hunter: "This is why" referred to the wrong sentence. Verdict: low. Patched by reordering.
- Blind Hunter: the dev compose publishes `8080:8080`. Verdict: low. Patched with a one-line note that it is for local use only.
- Blind Hunter: a containerised scraper can't reach the host's loopback. Verdict: medium. Patched: it should scrape `fantasy-hockey:8080` on the compose network.
- Blind Hunter: `route` label values were not shown. Verdict: low. Patched with examples.
- Blind Hunter: the `set` attribute was not explained, and partial saves were not mentioned. Verdict: low. Patched.
- Blind Hunter: where failed sends show up was not said. Verdict: low. Patched with the `send login code` ERROR line.
- Blind Hunter: the audit trail is lost when the container is recreated, and logs never rotate. Verdict: low. Patched with a one-paragraph retention note.
- Blind Hunter: the sprint-status entry was closed without a deviation note. Verdict: low. Rejected because action items have no resolution field. The deviation is recorded in this spec and the commit.
- Blind Hunter: the spec was unfinished. Verdict: false. Finalizing sets `done` and fills the notes.
- Blind Hunter: the v0.1.0 claim gives operators no way to check it, and the example pulls `:latest`. Verdict: low. Rejected: the claim is verified (`git tag --contains a69e5f3`), and the example header already says to pin release tags.
