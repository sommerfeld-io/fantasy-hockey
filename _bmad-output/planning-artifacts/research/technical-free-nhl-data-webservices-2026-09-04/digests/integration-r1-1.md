# Research Digest: Free NHL Data Webservices — Integration & Interoperability

## 1. Claims

**Auth**

- **Claim:** `api-web.nhle.com` (undocumented NHL web API) requires no API key; all endpoints are publicly accessible with a plain GET request.
  Source: https://github.com/pseudo-r/Public-NHL-API | Publisher: community repo (pseudo-r) | Undated | Accessed: 2026-09-04 | Confidence: high | Class: auth

- **Claim:** ESPN's hidden API (`site.api.espn.com/apis/site/v2/sports/hockey/nhl/...`) requires no API key; endpoints were reverse-engineered from ESPN's own web/mobile apps, not officially published.
  Source: WebSearch aggregate incl. https://gist.github.com/akeaswaran/b48b02f1c94f873c6655e7129910fc3b, https://github.com/pseudo-r/Public-ESPN-API | Publisher: mixed community sources | Undated | Accessed: 2026-09-04 | Confidence: medium (not independently fetched/verified against a primary ESPN statement — ESPN has no official docs at all)

**Rate limits**

- **Claim:** No documented/published rate limits exist for either `api-web.nhle.com` or ESPN's hidden API. Community guidance is informal ("avoid excessive/abusive querying," "cache responses") rather than a stated cap.
  Sources: https://github.com/dword4/nhlapi, https://github.com/pseudo-r/Public-NHL-API (neither mentions limits), plus WebSearch summary of scrapecreators.com/zuplo.com re: ESPN | Undated | Accessed 2026-09-04 | Confidence: medium (absence-of-evidence, from several independent community docs — consistent pattern, not one recycled source)

**Data formats / coverage**

- **Claim:** NHL web API returns JSON and covers standings (`/v1/standings/now`, `/v1/standings/{date}`, `/v1/standings-season`), skater/goalie stat leaders (`/v1/skater-stats-leaders/current`, `/v1/skater-stats-leaders/{season}/{game-type}`, with `categories`/`limit` params — i.e., usable for Art Ross/Rocket Richard-style leaderboards), and playoff bracket/series (`/v1/playoff-series/carousel/{season}`, `/v1/playoff-bracket/{year}`, `/v1/schedule/playoff-series/{season}/{series_letter}`).
  Source: https://github.com/Zmalski/NHL-API-Reference (GitHub, community reference) | Undated | Accessed 2026-09-04 | Confidence: high (specific, corroborated by second independent repo below)

- **Claim:** A second, independently-maintained reference confirms JSON format, no-API-key access, and a playoff-bracket endpoint, but does **not** list any awards/trophy-nominee endpoint.
  Source: https://github.com/pseudo-r/Public-NHL-API | Undated | Accessed 2026-09-04 | Confidence: medium-high

- **Claim:** The older `dword4/nhlapi` community doc says it does cover an "awards" endpoint category alongside standings/team stats, but explicitly does **not** document stats-leader or playoff-specific endpoints, and notes the maintainer has moved off GitHub (doc may be stale/unmaintained).
  Source: https://github.com/dword4/nhlapi | Undated | Accessed 2026-09-04 | Confidence: low-medium (contradicts Zmalski/pseudo-r on playoff coverage; likely reflects an older/legacy NHL API generation, not current api-web.nhle.com — flagged as a lead, not resolved)

- **Claim:** No source found among those checked documents a working, current `api-web.nhle.com` endpoint specifically for award nominees/winners (Hart, Norris, Vezina, Calder, etc.).
  Confidence: this is a gap, not a positive claim — see section 3.

**Licensing / ToS**

- **Claim:** NHL.com's Terms of Service (Section 2) explicitly prohibit "unauthorized spidering, scraping, or harvesting of content or information, or use [of] any other unauthorized automated means to compile information," and (Section 7) restrict use of NHL content/Services to "non-commercial, informational, personal use, without modification or alteration," with redistribution and commercial reuse (including ad-revenue aggregation sites) requiring express written NHL approval.
  Source: https://www.nhl.com/info/terms-of-service | Publisher: NHL (primary/official) | Undated | Accessed 2026-09-04 | Confidence: high | Class: licensing

- **Claim:** A Sportradar/NHL data-licensing addendum reportedly prohibits customers from redistributing NHL data or using it for anything other than "consumer" use, and bars modifying, combining, reverse-engineering, or reselling NHL data except as expressly permitted.
  Source: WebSearch summary citing https://developer.sportradar.com/page/NHL_Addendum | Publisher: Sportradar (official commercial data provider) | Undated | Accessed 2026-09-04 | Confidence: low-medium — direct WebFetch of this URL returned only a generic developer-portal landing page with no addendum text, so this claim rests on the search engine's own summary/snippet, not content verified directly. Needs re-fetch of the actual addendum PDF/page.

- **Claim:** ESPN's terms and conditions prohibit "spiders, robots or other automated data mining techniques to catalog, download, store or otherwise reproduce or distribute content," and commercial/heavy automated use risks ToS violation and IP blocking.
  Source: WebSearch summary referencing https://www.espn.com/mobile/aware/terms.pdf (direct fetch of this PDF failed — binary/unparseable) and secondary sites (sportsapis.dev, espnapi.com) | Confidence: low — this is a secondary-source paraphrase; the primary PDF could not be verified in this session. Multiple secondary sites here look like recycled content from one another (marketing-register "2026 guide" pages), so treat as one weak signal, not several corroborating ones.

## 2. Leads worth chasing further

- **Direct contradiction on api-web.nhle.com legal status vs. common practice**: the current NHL.com ToS (a primary, official source) bans scraping/automated compilation and non-personal use outright, yet a large, active open-source ecosystem (Zmalski, pseudo-r, dword4, plus NuGet/PyPI packages) publicly builds on `api-web.nhle.com` with no evident enforcement action. This gap between stated ToS and observed tolerance is the single most decision-relevant tension for the "sole data source" question and deserves explicit flagging, not resolution by inference.
- Whether `api-web.nhle.com` is legally/contractually the *same* surface the nhl.com ToS governs (subdomain of nhle.com vs nhl.com) is unconfirmed — worth a targeted check of NHL's actual domain/entity structure and whether a separate or no ToS attaches to `api-web.nhle.com`/`api.nhle.com` specifically.
- The Sportradar NHL Addendum looked like the strongest primary evidence of explicit anti-redistribution terms but the fetch didn't retrieve the actual clause text — worth a follow-up fetch (possibly the addendum is a PDF or gated page, similar to the ESPN terms PDF failure).
- `dword4/nhlapi` vs `Zmalski/NHL-API-Reference` disagree on which endpoint categories exist (awards vs. playoff/stat-leaders) — likely reflects different API generations (legacy `statsapi.web.nhl.com`, now deprecated/shut down, vs. current `api-web.nhle.com`). Worth confirming which legacy API dword4's doc actually describes and whether it's still live at all — if the legacy API is dead, that's itself a reliability data point (NHL has shut down a public API surface before, which bears directly on "reliable ... for a season-long engine").
- TSN was named in scope but no search specifically surfaced a TSN public/hidden API — worth one more targeted pass if TSN data access is still needed.
- No source addressed uptime/reliability history or precedent of the API changing/breaking mid-season, which matters for the "fully automated, no human fallback" framing.

## 3. What could not be found or confirmed

- No current, working `api-web.nhle.com` endpoint for **award nominees/winners** (Hart, Vezina, Norris, Calder, etc.) was confirmed in any of the three community references checked; one older doc claims an awards category exists but with no endpoint detail, and the two more current/detailed references (Zmalski, pseudo-r) don't list one at all.
- No documented, numeric rate limit for either the NHL or ESPN hidden APIs — only informal community norms ("don't hammer it," "cache your requests").
- Could not retrieve the actual text of the Sportradar NHL data addendum (fetch returned a generic portal page, not the addendum content) — the redistribution-prohibition claim about it is unverified beyond a search-engine summary.
- Could not read ESPN's own terms.pdf directly (binary/unparseable via fetch tool) — the ESPN scraping-prohibition claim rests on secondary paraphrase sources only, which also show signs of mutual recycling (near-identical "2026 guide" language across sportsapis.dev, espnapi.com, publicapis.io).
- No TSN public or hidden API was identified in the searches run.
- No official NHL statement (blog, dev-relations page, press comment) acknowledging or sanctioning third-party use of `api-web.nhle.com` was found — its "public" status appears to be purely de facto (unenforced), not de jure.
