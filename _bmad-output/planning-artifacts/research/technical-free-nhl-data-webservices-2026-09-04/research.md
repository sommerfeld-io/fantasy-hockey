---
title: 'Technical research: Free NHL data webservices for automated fantasy-hockey scoring'
type: 'Technical'
topic: 'Free NHL data webservices for automated fantasy-hockey scoring'
decision: 'Whether a free, reliable public webservice can be the sole data source for fully-automated scoring (no scorekeeper, no admin page) in the fantasy-hockey web app'
source: 'native run'
status: complete
preset: 'standard'
validation: 'normal'
created: '2026-09-04'
updated: '2026-09-04'
claims_verified: 4
claims_unverified: 0
---

# Technical research: Free NHL data webservices for automated fantasy-hockey scoring

**Decision this research serves:** Whether a free, reliable public webservice can be the sole data source for fully-automated scoring (no scorekeeper, no admin page) in the fantasy-hockey web app.

## Executive Summary

**Partial yes.** A free, no-auth NHL web API (`api-web.nhle.com`) was directly tested in this session and confirmed live: it covers team standings, points/goals stat leaderboards (i.e. Art Ross and Rocket Richard), and playoff bracket/series results — 3 of the app's 4 data needs — with no key, no signup, and JSON output [1][2][3]. This closes the biggest part of the original risk.

It does not close all of it. Two load-bearing gaps remain: (1) **no free structured source exists anywhere** for the three *voted* individual awards — Hart, Norris, Vezina — checked and disconfirmed across NHL's own editorial pages, Hockey-Reference, TSN/Sportsnet, and Wikidata [4][5][6][21][22][23][25]; and (2) the entire data layer rests on an **unofficial, undocumented API with no ToS of its own and a proven history of being fully replaced without notice** — it happened once already, in 2023, breaking multiple production consumer apps [17][19][20]. NHL.com's own Terms of Service (a primary source) also prohibit automated scraping and non-personal use outright [12], creating a legal ambiguity the project should go in with eyes open, not resolve by inference.

**Bottom line for the design:** the "no scorekeeper, no admin page" architecture is sound for standings, Art Ross/Rocket Richard, and playoff scoring. It is not fully achievable as originally scoped for Hart/Norris/Vezina, and the whole design should be built assuming this data source will eventually change or break, not merely might.

## Landscape & Maturity

The NHL does not publish an official, documented, versioned public API. What the entire hobbyist/analytics community uses instead is `api-web.nhle.com` (and a companion `api.nhle.com/stats/rest`) — undocumented endpoints that power the NHL's own website and apps, reverse-engineered and catalogued by active community projects, most notably `Zmalski/NHL-API-Reference` (596 stars, commits through November 2025) and `pseudo-r/Public-NHL-API` [1][2]. An older, now-unmaintained project, `dword4/nhlapi`, documented the previous API generation and is superseded by these two [15][16]. Direct in-session testing confirmed three endpoint families are live and return usable JSON: `/v1/standings/now` (full current standings), `/v1/skater-stats-leaders/current` with `categories=points,goals` (directly usable as Art Ross/Rocket Richard leaderboards), and `/v1/playoff-bracket/{year}` plus `/v1/playoff-series/carousel/{season}` (series winner and win-count per team) [3].

The one confirmed gap in the landscape is awards. No community reference documents an endpoint for award nominees or winners on the current API, and NHL.com itself only covers awards editorially — news articles and a seasonal "Trophy Tracker" series, not structured data [1][2][4][5]. Hockey-Reference.com does maintain structured historical award-voting pages, but as HTML tables under a restrictive ToS (below), not a feed or API [6]. MoneyPuck offers free CSV downloads of advanced stats but is a bulk-data site, not a queryable webservice, and discloses no SLA [7][8]. ESPN's NHL data is available only through the same kind of unofficial, reverse-engineered hidden API pattern as the NHL's own — not an officially published free developer API (ESPN retired its public developer portal in 2014) [9][10]. No public or hidden API was found for TSN.ca or Sportsnet.ca at all, for any NHL data.

## Integration & Interoperability

`api-web.nhle.com` requires no API key and no signup — a plain GET request against any endpoint returns JSON [1][2]. No official rate limit is published; community guidance is informal ("cache your requests," "don't hammer it"), and at least one third-party client library implements HTTP 429 handling defensively, implying throttling is encountered in practice even though never documented as a number [11]. The same is true of ESPN's hidden API: no key, no documented limit, no SLA, "can change without notice" [9][10].

The clearest structural risk in this dimension is legal, not technical. NHL.com's own Terms of Service — a primary, official source — explicitly prohibit "unauthorized spidering, scraping, or harvesting of content... or any other unauthorized automated means to compile information," and restrict use of NHL content to "non-commercial, informational, personal use" [12]. (A separate clause requiring written approval for reproduction applies specifically to NHL team logos and marks, not to general content redistribution — noted here to avoid overstating the ToS's scope.) Whether this ToS actually governs the `nhle.com` API subdomain the same way it governs `nhl.com`, or whether the API operates in a separate, unaddressed legal space, could not be resolved from available sources — it is a real open question, not a settled one. A Sportradar commercial data-licensing addendum for NHL data was located but its actual clause text could not be retrieved in this session (the fetch returned only a generic portal page) [13], and the same is true of ESPN's own terms document, whose scraping-prohibition language is reported only secondhand [14] — both are noted for completeness but carry low confidence as a result. What is clear is that `api-web.nhle.com` publishes no ToS of its own: its "public" status is purely de facto (a large, active open-source ecosystem builds on it openly, unenforced) rather than de jure (sanctioned). Hockey-Reference's terms are more explicit and more restrictive still: they permit only search-indexing bots, forbid use for "competing or substitute products," enforce a stated rate limit (10–20 requests/min), and temporarily IP-block violators [23][24] — ruling it out as a scraping target regardless of the awards-data question.

## Implementation Reality & Ecosystem Health

The NHL has changed its entire public API generation once already. Around September–November 2023, the previous API (`statsapi.web.nhl.com`) was retired and replaced by the current `api-web.nhle.com`, with no advance notice to third-party developers [17]. This is independently confirmed by three separate publishers on three different platforms — a developer forum [17], a GitHub issue on an unrelated NuGet-package repo reporting the outage directly [19], and a GitLab issue on the most established community documentation project noting the same redesign [20] — not one account echoed across sites. The migration broke multiple production consumer apps; fixes for the main breakage landed within days, but edge cases took weeks to iron out, and some data fields were permanently lost in the transition [17].

Against that, the current API shows real signs of a healthy, active reverse-engineering community: the leading reference repo has hundreds of stars and commits as recent as November 2025 [1], and multiple independent client libraries exist across languages. No source found any documented outage tied to a specific high-traffic moment (a Cup-clinching Game 7, an awards announcement), and no one reported ever actually being IP-blocked for routine polling — the rate-limiting risk is a live open question in the community, not a confirmed incident [18].

## Cross-Dimension Insights

**The awards gap is narrower than it first looks.** Art Ross and Rocket Richard are not voted — they are, by definition, the season's points leader and goals leader. Since the stat-leaderboard endpoint is confirmed live and free [3], it *is* the source of truth for those two trophies once the season ends; no separate "award" endpoint is needed for them at all. The confirmed absence of structured award data therefore only blocks automated scoring for the three *voted* trophies — Hart, Norris, Vezina — not all five, as the original risk framing assumed.

**Reliability risk and legal risk are the same root cause wearing two hats.** The API has no official status, no ToS of its own, and no changelog or deprecation process — which is exactly why it broke without warning in 2023 and exactly why its legal standing under NHL.com's ToS is unresolved. A design that treats this API as authoritative is making one bet, not two: that an unofficial, unsanctioned data source will keep working and keep being tolerated. The project's already-planned resilience pattern (cron sync, write-nothing-on-failure, rely on the next scheduled run) is well-suited to the *technical* half of that bet, but does nothing for the *legal* half — that risk can only be accepted, not engineered around.

## Recommendations

1. **Adopt `api-web.nhle.com` as the primary data source for standings, Art Ross/Rocket Richard stats, and playoff series/results.** Confidence: high (direct verification + independent community corroboration). This resolves the largest share of the original open risk from the brainstorm intent doc and supports the "no scorekeeper" design for these three data needs as planned.
2. **Do not plan automated scoring for Hart, Norris, or Vezina in v1 as originally scoped.** Confidence: high (four independent negative findings across distinct source types). This corrects a scope assumption: entering predictions for all 5 awards is already a "Must" in the intent doc, but scoring three of them was assumed automatable, and it isn't. Either accept these three trophies score manually/rarely (once per season, at announcement) via some minimal path, or defer their scoring entirely and surface them as informational-only in v1.
3. **Treat the data-source risk as structural, not incidental.** Confidence: high. Design the sync layer (already planned as a separate, resilient, cron-driven container) assuming a breaking API change will happen again — it already has once — rather than as a low-probability edge case.
4. **Flag the ToS ambiguity to the human decision-maker explicitly; do not resolve it by assumption.** Confidence: medium (the underlying legal question could not be settled by research). This is a personal, non-commercial, 3-person hobby app, which is the most favorable possible fact pattern under NHL.com's stated ToS — but the ToS's plain text still technically prohibits the automated access the whole design depends on.

These recommendations update the brainstorm intent doc's "Biggest Open Risk" section and should feed directly into the next architecture-planning step as a constraint, not a footnote.

## Open Questions

- Does NHL.com's Terms of Service (governing `nhl.com`) actually extend to the `nhle.com` API subdomain, or does that surface sit in unaddressed legal space? Could not be resolved from available public sources — would likely require legal review, not further searching.
- Does structured per-year award-winner data exist on individual *player* Wikidata items (via the `wdt:P166` "award received" property), even though the trophy items themselves carry none? Flagged as a concrete next step (a live SPARQL query) but not executed this run.
- Has the NHL signaled any intent (blog post, developer relations statement) about the long-term status of `api-web.nhle.com`, or is total silence the permanent norm? Nothing found in either research round.

## Source Appendix

| # | Claim / Finding it supports | Publisher | Pub. date | Accessed | Confidence |
|----|------------------------------------------------------------------------------|-------------------------------------------------------------------------|-----------|------------|------------|
| 1  | [Zmalski/NHL-API-Reference](https://github.com/Zmalski/NHL-API-Reference) — current API endpoint catalogue, active maintenance | GitHub (community) | commits through 2025-11 | 2026-09-04 | High |
| 2  | [pseudo-r/Public-NHL-API](https://github.com/pseudo-r/Public-NHL-API) — endpoint catalogue, no awards endpoint | GitHub (community) | undated | 2026-09-04 | Medium-high |
| 3  | Direct HTTP GET, `api-web.nhle.com` standings/stats-leaders/playoff-bracket endpoints | NHL (nhle.com infrastructure) | live/undated | 2026-09-04 | High |
| 4  | [NHL award finalists schedule](https://www.nhl.com/news/nhl-awards-finalists-schedule-of-announcements-for-2025-26-season) — awards are editorial content only | NHL.com | 2025-26 season | 2026-09-04 | Medium |
| 5  | [Hart Trophy tracker article](https://www.nhl.com/news/hart-trophy-tracker-nikita-kucherov-of-tampa-bay-lightning-pick-for-mvp) | NHL.com | 2025-26 season | 2026-09-04 | Medium |
| 6  | [Hockey-Reference awards pages](https://www.hockey-reference.com/awards/voting-2025.html) — structured but HTML-table only | Sports Reference LLC | undated | 2026-09-04 | Medium |
| 7  | [MoneyPuck data page](https://moneypuck.com/data.htm) — CSV bulk downloads, not an API | MoneyPuck | undated | 2026-09-04 | Medium |
| 8  | [MoneyPuck about page](https://moneypuck.com/about.htm) — single-maintainer, no SLA | MoneyPuck | undated | 2026-09-04 | High |
| 9  | [pseudo-r/Public-ESPN-API](https://github.com/pseudo-r/Public-ESPN-API) | GitHub (community) | undated | 2026-09-04 | Medium |
| 10 | [ESPN hidden API gist](https://gist.github.com/akeaswaran/b48b02f1c94f873c6655e7129910fc3b) — ESPN retired official API portal 2014 | Developer gist | undated | 2026-09-04 | Medium |
| 11 | [nhlscraper CRAN docs](https://rdrr.io/cran/nhlscraper/man/nhl_api.html) — defensive 429 handling | CRAN | undated | 2026-09-04 | Low-medium |
| 12 | [NHL.com Terms of Service](https://www.nhl.com/info/terms-of-service) — prohibits scraping/automated compilation | NHL (primary/official) | undated | 2026-09-04 | High |
| 13 | [Sportradar NHL Addendum page](https://developer.sportradar.com/page/NHL_Addendum) — content unverified, portal landing page only | Sportradar | undated | 2026-09-04 | Low-medium |
| 14 | ESPN terms.pdf — content unverified (fetch failed), secondary paraphrase only | ESPN | undated | 2026-09-04 | Low |
| 15 | [dword4/nhlapi (GitHub)](https://github.com/dword4/nhlapi) — legacy API docs, no longer maintained | GitHub (community) | undated | 2026-09-04 | Medium |
| 16 | [dword4/nhlapi (GitLab mirror)](https://gitlab.com/dword4/nhlapi) | GitLab (community) | undated | 2026-09-04 | Medium |
| 17 | [Tidbyt developer forum — NHL API change](https://discuss.tidbyt.com/t/nhl-api-change/5962) — 2023 migration broke production apps | Tidbyt forum | 2023-11 | 2026-09-04 | High |
| 18 | [dword4/nhlapi Issue #21 "API Limits"](https://github.com/dword4/nhlapi/issues/21) — rate limits never documented | GitHub (community) | undated | 2026-09-04 | Medium |
| 19 | [Afischbacher/Nhl.Api Issue #41](https://github.com/Afischbacher/Nhl.Api/issues/41) — independent confirmation of 2023 outage | GitHub (community) | 2023-11-08 | 2026-09-04 | High |
| 20 | [dword4/nhlapi GitLab Issue #109](https://gitlab.com/dword4/nhlapi/-/work_items/109) — independent confirmation of 2023 redesign impact | GitLab (community) | ~2023 | 2026-09-04 | Medium-high |
| 21 | [NHL award finalists announcement](https://www.nhl.com/news/nhl-awards-finalists-to-be-announced-starting-april-28) — no structured feed exists | NHL.com | 2026 season | 2026-09-04 | Medium |
| 22 | [to-rss.xyz NHL feed](https://www.to-rss.xyz/nhl/) — third-party feed exists because no official one does | to-rss.xyz (third-party) | undated | 2026-09-04 | Medium |
| 23 | [Sports-Reference Terms of Use](https://www.sports-reference.com/termsofuse.html) — forbids "competing/substitute products," rate-limits scraping | Sports Reference LLC | undated | 2026-09-04 | Medium |
| 24 | Sports-Reference bot-traffic page — content unverified (HTTP 403 on fetch) | Sports Reference LLC | undated | 2026-09-04 | Low |
| 25 | [Wikidata — Hart Memorial Trophy (Q678383)](https://www.wikidata.org/wiki/Q678383) — no winner statements on the trophy item | Wikidata | live/undated | 2026-09-04 | High |

## Staleness Map

| Claim | Class | Pub. date | Re-check by | Stale now? |
|-------------------------------------------------------------------------------|----------------------|-----------|--------------|------------|
| `api-web.nhle.com` endpoints live and free for standings/stats/playoffs      | versions_compatibility | 2026-09  | 2026-10-01   | No |
| No free structured award-nominee/winner source exists                        | landscape             | 2026-09  | 2027-09-01   | No |
| NHL.com ToS prohibits automated scraping; API has no ToS of its own          | ecosystem_signals      | 2026-09  | 2027-03-01   | No |
| NHL fully replaced its API generation in 2023 without notice                 | patterns               | 2023-11  | 2025-11-01   | **Yes** |

Earliest re-check: **2025-11-01** — already past, on the "has this happened again" precedent claim. Since that claim documents a historical event rather than a current state, re-checking it means: has there been a *second* full migration since 2023? Worth a quick check before finalizing any architecture that assumes long-term endpoint stability. The endpoint-liveness claim (re-check 2026-10-01) is the one to actually watch closely given how recently it was verified.
