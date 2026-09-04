---
title: 'Fantasy Hockey Web App — Product Brief'
status: draft
created: '2026-09-04'
updated: '2026-09-04'
---

# Product Brief: Fantasy Hockey Web App

## Executive Summary

Three friends — Basti, Sadl, and Tobbi — have run a private NHL prediction game once per season for years, tracked on a hand-maintained Excel sheet. It works, in the sense that the game gets played, but the sheet has failed them in small and large ways: a correct 20-point Stanley Cup pick was never credited last season because the scoring formula missed it, typo'd team and player names make automated checking impossible, and nothing stops a prediction from being edited after its deadline has passed.

This brief scopes a small, self-hosted web app — written in Go, shipped as a Docker image, run via docker-compose — that replaces the sheet with the same game, played the same way, minus the manual failure points. It automates scoring against a live NHL data source, enforces prediction deadlines, and gives the three of them a shared, always-correct view of standings. It is not trying to become a bigger product or attract more players; it exists to remove friction from something they already enjoy.

The timing is now because the group has already decided to rebuild, has already worked through the domain rules and scoring edge cases in detail, and has already validated (via technical research) that the core data dependency — a free NHL data source — is feasible for most of what the game needs — with the two remaining gaps already identified now, not left to surface mid-build.

## The Problem

The Excel sheet fails in four concrete ways, all rooted in being manually operated:

- **Scoring is hand-calculated and error-prone.** A confirmed real incident: Basti correctly picked Carolina to win the Stanley Cup (worth 20 points), and it was never credited because the formula didn't catch it. Nobody noticed until well after the fact — there was no mechanism to catch the error, only trust in a spreadsheet formula.
- **No lock enforcement.** Nothing prevents a participant from viewing or editing a prediction after its deadline has passed, undermining the fairness the whole game depends on.
- **No input validation.** The sheet is full of inconsistent player/team names ("Quinn Huges", "Winnnipeg", "Annaheim", "Kutcherov" vs "Kucherov") that make automated scoring impossible even if someone wanted to build it on top of the sheet.
- **No live standings, no per-category breakdown, no history.** Nobody can see where they stand mid-season without manually recalculating, and nothing from past seasons is preserved or comparable.

The cost of the status quo isn't business risk — it's a diminished experience for three people who enjoy a low-effort seasonal ritual and currently can't fully trust the tool that runs it.

## The Solution

A single web app with one user role (Participant), fixed to exactly three hardcoded accounts, logged in via a 6-digit emailed code — no passwords, no registration flow, nothing to administer. Participants enter their predictions once before the season (awards, team marks, Cup pick, Presidents' Trophy) and again before the playoffs (Late Cup pick, then each round's series as it becomes knowable). Once a deadline passes, that prediction is frozen and becomes visible to the other two players.

Scoring is fully automated: a separate, cron-scheduled sync process pulls standings, stat leaders, and playoff results from a free NHL data source and computes points against the game's rules — no scorekeeper, no manual result entry, no risk of a repeat of the Carolina bug. Standings (Total, split into Regular Season and Playoffs) are visible on every page at all times, since there are only three players to show. [ASSUMPTION: v1 ships as a single season with no season-selector — see Scope.]

## Who This Serves

**Basti, Sadl, and Tobbi** — three friends who already know and enjoy this game; the app is not trying to reach anyone else. What each of them needs from it is the same: enter picks in a couple of minutes twice a season, trust that the scoring is right without checking it themselves, and see a fair, live standings view without waiting on anyone to update a spreadsheet. Success for them looks like: forgetting the app exists between deadlines, and being nudged in time when a deadline approaches.

There is no secondary user. No admin, no scorekeeper, no visitor/spectator role — a design decision made deliberately during scoping, not an oversight.

## Success Criteria

- **No repeat of the Carolina bug** — every scoring rule (including tie-inclusive award scoring and playoff exact-result vs. winner-only scoring) computes correctly and automatically, with no hand calculation anywhere.
- **No missed or disputed deadlines** — every deadline is enforced server-side, and each participant reliably gets a reminder before their own deadline closes.
- **Standings are always trustworthy** — a participant can check Total, Regular Season, and Playoff points at any time and get a correct answer without asking anyone.
- **The three of them keep playing it low-effort, season after season** — the true measure of success given the passion-project stakes here is that nobody thinks about the app between deadlines, and nobody goes back to a spreadsheet. [ASSUMPTION: no numeric/analytics-based success metric is meaningful at this scale — success is qualitative and judged by the three players themselves.]

## Scope

**Must-have (v1):**

- Login (email → 6-digit code) / Logout
- Enter Predictions: all 5 awards (3 names each), regular-season team marks (playoffs + division winner), Early/Late Stanley Cup picks, Presidents' Trophy, playoff series (winner + game count) as each round unlocks
- Deadline lock enforcement (frozen after close, no grace period) + deadline-only reminder emails ("action required until TIMESTAMP" — no result summaries, no newsletter content)
- Own predictions always visible; other players' predictions visible only after their own deadline passes
- Automated result ingestion from the NHL's free data API via a cron-scheduled sync (interval set by env var), resilient to source downtime (writes nothing on failure, relies on the next run)
- Scoring engine implementing all resolved rules: tie-inclusive award matching, playoff round-based scoring (winner vs. exact result), shared rank on tied totals
- Standings — Total, Regular Season, and Playoffs buckets, shown persistently on every page
- Teams identified by a stable technical ID, not name, so relocations/renames don't corrupt data
- Fixed 3-participant model (hardcoded)

**Could-have (later, explicitly deferred):**

- Multi-season data model and a season-selector dropdown
- A dedicated Season Hub landing page
- Hall of Fame page (championship/runner-up/third-place counts across seasons)

**Explicitly won't (this version):**

- Any scorekeeper or admin manual-data-entry role or page
- Registration/join flow for new participants
- Real-time sync, or any "live/projected" standings tier distinct from the last confirmed sync
- Automated scoring for Hart, Norris, or Vezina — no free structured data source exists for these three voted awards (confirmed via technical research); predictions are still collected for all 5 awards, but only Art Ross and Rocket Richard can be scored automatically in v1
- Deep per-category point breakdown beyond the Regular Season / Playoffs split
- A tie-break mechanism — equal totals share the same rank, by design

## Known Risks

- **Data source reliability.** The NHL's free data API is undocumented and unofficial, publishes no ToS of its own, and was fully replaced without notice once already (2023), breaking third-party consumers. The sync design should assume this will happen again, not treat it as unlikely.
- **Legal ambiguity.** NHL.com's own Terms of Service technically prohibit automated scraping/compilation. This is a personal, non-commercial, 3-person use case — the most favorable fact pattern available — but the risk should be knowingly accepted, not resolved by assumption.
- **Awards scoring gap** (see Scope). Before build starts, this needs an explicit decision: manual entry once a year for these three trophies, or informational-only with no scoring at all.

_Full findings: `_bmad-output/planning-artifacts/research/technical-free-nhl-data-webservices-2026-09-04/research.md`_

## Vision

[ASSUMPTION: framed as continuity rather than growth, matching the passion-project stakes — confirm or correct.] If this works, it keeps running quietly every year with no maintenance burden between seasons, the Could-have Hall of Fame eventually gets built once there's more than one season of history to show off, and the three of them stop thinking about the tool entirely and just play the game — which was the point all along.
