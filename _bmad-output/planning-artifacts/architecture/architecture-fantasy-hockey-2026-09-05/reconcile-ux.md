---
title: 'Reconciliation — UX Spines vs. Architecture Spine'
status: draft
created: '2026-09-05'
sources:
  - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/DESIGN.md
  - _bmad-output/planning-artifacts/ux-designs/ux-fantasy-hockey-2026-09-05/EXPERIENCE.md
  - _bmad-output/planning-artifacts/architecture/architecture-fantasy-hockey-2026-09-05/ARCHITECTURE-SPINE.md
---

# Reconciliation — UX Spines vs. Architecture Spine

Scope: this is a check for real architectural gaps or contradictions between the UX spines
(DESIGN.md, EXPERIENCE.md) and the technical architecture spine (ARCHITECTURE-SPINE.md). It
is not a re-audit of visual or copy detail — those correctly live only in the UX documents.

## 1. Single responsive layout vs. AD-11 (server-rendered, stdlib `html/template`)

**Finding: no contradiction. Correctly out of architecture's scope.**

EXPERIENCE.md's Foundation ("no UI system... build directly against DESIGN.md tokens") and
Responsive & Platform ("single responsive layout, no separate mobile design... standings
widget stays a full-width top bar at every viewport width... content below is single-column
at every width") describe a *CSS-only* responsive strategy — one HTML structure whose layout
adapts via CSS breakpoints/flow, not server-side device detection, separate templates per
viewport, or content negotiation.

AD-11 governs how HTML is generated (stdlib `html/template`, one route → one rendered page)
and is silent on responsive layout entirely. That silence is appropriate: a single
server-rendered template naturally satisfies "one HTML structure for every viewport" — there
is no server-side reason CSS media queries couldn't handle the rest. AD-11 does not assume or
require per-device templates, so there's no conflict to flag. This is a case where
architecture's silence is correct scoping, not a gap.

One adjacent, low-weight observation (see §3) is that the spine's source tree has no
`internal/web/static/` (or `go:embed`) entry for the CSS file that would carry DESIGN.md's
tokens — see the static-asset-delivery gap below, which is really about *serving* the CSS/JS,
not about the responsive strategy itself.

## 2. Autocomplete data delivery — real gap

**Finding: real gap. The spine does not say how the canonical Team / AwardFinalist list
reaches the browser for client-side filtering.**

EXPERIENCE.md (Component Patterns → Autocomplete input, Interaction Primitives) is explicit
that suggestions must filter live as the Participant types, keyboard- and pointer-navigable,
and that "selecting a suggestion is the only way to set the field" (PRD FR-5) — this is level
of behavioral, not decorative, importance: it's the core "no typos" mechanism the whole
Predictions-entry flow depends on.

AD-11 commits to: `internal/web` renders HTML server-side; the only client-side JS is
"vanilla JS driving the autocomplete widget (FR-5)." That statement covers *that* JS exists,
but not how it obtains the list of Teams (for Series Winner picks) or AwardFinalists (for
Award picks, entered via `internal/awards`/FR-13) to filter against as the user types. Two
architecturally distinct options are both live and neither is chosen:

- **Embed the full list in the page** at render time (e.g., a `<script type="application/json">`
  block populated by `html/template` from data `internal/predictions`/`internal/awards`
  already fetched from `internal/store` for that page) — zero new routes, consistent with
  AD-11's "no separate API+SPA split," but means every Enter Predictions / Award Data Entry
  page-load ships the full candidate list even though it's small (a handful of Teams/
  Finalists for 3 users).
- **A small JSON endpoint** in `internal/web` that the vanilla JS calls per keystroke or on
  page load — arguably in tension with AD-11's "prevents... a separate API+SPA split," since
  it introduces a second response shape (JSON) alongside server-rendered HTML, even if it's
  one endpoint, not a full API surface.

This matters architecturally, not just visually, because:

- It determines whether `internal/web` needs a JSON-returning route at all (a real deviation
  from "renders HTML server-side" as currently worded in AD-11), or whether the existing
  request/response model already covers it via template-embedded data.
- It determines what `internal/predictions` / `internal/awards` need to expose to `internal/web`
  (a full Team/AwardFinalist listing, not just per-Prediction data) — not currently reflected
  in the Capability → Architecture Map or Source Tree.
- Given the dataset is tiny (a fixed set of ~16 Teams and a handful of manually entered
  AwardFinalists per season, 3 users total), either option is cheap; the gap is that the
  spine doesn't pick one, and AD-11 as worded ("renders HTML server-side... prevents... a
  separate API+SPA split") reads as mildly contradicting the JSON-endpoint option without
  explicitly ruling it in or out.

**Recommendation:** Add a short rule (or amend AD-11) stating the chosen mechanism — most
likely: the candidate list is fetched from `internal/store` by `internal/predictions`/
`internal/awards`, passed to the template, and serialized inline as JSON for the vanilla JS to
read from the DOM at page load, with no dedicated JSON route. This keeps AD-11's "no
separate API+SPA split" intact and is sufficient for a list this size that changes rarely
(Teams are static per season; AwardFinalists are edited via a separate page, not while a
Participant is filling out predictions concurrently in the same session in a way that would
need live server round-trips).

## 3. Technology-need contradictions (real-time, websockets, asset pipeline) — no significant gap

**Finding: no contradiction; the UX spines actively reinforce the stdlib-only, no-JS-framework
stance.**

- EXPERIENCE.md's Interaction Primitives explicitly **bans** "any auto-refresh or polling UI
  (the Sync is invisible; the page reflects whatever the last request returned)" and its
  Component Patterns note the standings widget needs "no loading spinner variant... no 'live'
  indicator since there is no live tier." This directly matches AD-16 (standings/scores
  computed live on read, never cached) and AD-11 (plain server-rendered HTML, no framework) —
  there is no implied need for websockets, SSE, or any push mechanism. If anything, the UX
  spine forecloses that need explicitly, which is a helpful confirmation rather than a gap.
- Typography ("system font stack throughout — no webfont loading for a 3-person tool") and
  Elevation/Shapes (no shadows, no icons, no imagery) confirm no asset pipeline (no webfonts,
  no icon sprites/SVG library, no build step for images) is implied. This is consistent with
  AD-11's minimal-dependency stance and requires no build tooling beyond what's already
  scoped.
- The one small, low-weight loose end (noted in §1/§2) is that the spine's Source Tree has no
  entry at all for where the DESIGN.md-token CSS file or the autocomplete's vanilla JS file
  physically live and how they're served (a `go:embed`'d `static/` directory under
  `internal/web`, versus inlining `<style>`/`<script>` directly in the templates). This is a
  minor omission, not a contradiction — either approach is trivially compatible with AD-11 —
  but the Source Tree currently shows `internal/web/` with no static-asset subpath, so an
  implementer has no guidance on convention. Worth a one-line addition to the Source Tree
  (e.g., `internal/web/static/` for CSS/JS, embedded via `go:embed`) but does not rise to the
  level of a contradiction or a functional gap the way §2 does.

## Summary

| # | Item | Verdict |
|---|------|---------|
| 1 | Single responsive layout vs. AD-11 | No gap — correctly out of architecture's scope (CSS-only concern) |
| 2 | Autocomplete data delivery to the browser | **Real gap** — spine doesn't specify embedded-JSON-in-template vs. a JSON endpoint, and AD-11's wording mildly discourages the latter without ruling it in/out |
| 3 | Real-time/websockets/asset-pipeline implied by UX | No gap — UX spine explicitly reinforces the no-polling, no-webfont, stdlib-only stance |
| — | Static asset (CSS/JS) location in Source Tree | Minor omission, not a contradiction — worth a one-line Source Tree addition |
