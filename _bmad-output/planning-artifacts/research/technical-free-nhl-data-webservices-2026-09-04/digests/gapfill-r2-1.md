# Research Digest: Free NHL Data Webservices — Gaps 1 & 2 (Round 2)

## Claims

1. Claim: A GitHub Issue on the Afischbacher/Nhl.Api repo, titled "HttpRequestException: statsapi.web.nhl.com is not available," opened Nov 8, 2023, reports the legacy NHL stats API endpoint went dead, breaking a NuGet package that depended on it.
   Source: https://github.com/Afischbacher/Nhl.Api/issues/41 | Publisher: GitHub (independent of the Tidbyt forum) | Date: 2023-11-08 | Accessed: 2026-09-04 | Confidence: high | Class: primary

2. Claim: A GitLab issue on dword4/nhlapi ("NHL.com redesign - any impact on API endpoints?", #109) documents that following an NHL.com redesign, statsapi.web.nhl.com was no longer used by the site and play-by-play JSON disappeared from game-center pages.
   Source: https://gitlab.com/dword4/nhlapi/-/work_items/109 | Publisher: GitLab (dword4/nhlapi) | Date: undated in fetched content, consistent with 2023 migration window | Accessed: 2026-09-04 | Confidence: medium-high | Class: primary

3. Claim: No official RSS/syndication feed exists for NHL.com news, including award-finalist announcements; finalists are announced via editorial news articles only. A third-party site (to-rss.xyz) exists specifically because no official NHL feed exists.
   Source: https://www.nhl.com/news/nhl-awards-finalists-to-be-announced-starting-april-28, https://www.to-rss.xyz/nhl/ | Publisher: NHL.com / to-rss.xyz | Date: 2026 season articles | Accessed: 2026-09-04 | Confidence: medium | Class: secondary

4. Claim: Hockey-Reference.com (Sports-Reference network) ToS permits only search-engine indexing bots respecting robots.txt, explicitly disallows use for "competing or substitute products," and enforces rate limits (10-20 req/min), with violators temporarily IP-blocked.
   Source: https://www.sports-reference.com/termsofuse.html, https://www.sports-reference.com/bot-traffic.html (direct fetch returned HTTP 403) | Publisher: Sports Reference LLC | Date: undated current policy | Accessed: 2026-09-04 | Confidence: medium (search-summary, not raw fetched text) | Class: secondary/paraphrase of primary

5. Claim: No public or hidden JSON API for TSN.ca or Sportsnet.ca was found for any NHL data.
   Source: WebSearch, no usable results | Confidence: n/a (absence of evidence) | Class: negative finding

6. Claim: The Wikidata item for the Hart Memorial Trophy (Q678383) has no winner-type statements enumerating individual winners by year on the trophy item itself.
   Source: https://www.wikidata.org/wiki/Q678383 (fetched directly) | Publisher: Wikidata | Date: undated, live item | Accessed: 2026-09-04 | Confidence: high | Class: primary

## Gap Status

**GAP 1 — Award nominees/winners structured source: remains OPEN (largely disconfirmed, one small unresolved lead).**
Searched: NHL press-release RSS/JSON feeds (none exist at all), Hockey-Reference/Sports-Reference structured data products (none — only a restrictive ToS forbidding "competing or substitute products" and rate-limiting scraping), TSN.ca/Sportsnet.ca hidden APIs (none found, for any NHL data), Wikidata (trophy item itself has no winner statements). One unresolved lead not yet executed: a live SPARQL query for `wdt:P166` (award received) across player items might surface structured per-year winner data even though the trophy item itself doesn't list it.

**GAP 2 — Independent second-source confirmation of the 2023 API migration: CLOSED.**
Confirmed via two additional independently-published sources beyond the original Tidbyt forum thread: a GitHub issue (Afischbacher/Nhl.Api #41, 2023-11-08) and a GitLab issue (dword4/nhlapi #109) — different publishers/platforms, not republications of each other or of the original source.

## New Leads

- A live Wikidata SPARQL query for `wdt:P166` per trophy Q-id was flagged but not executed — worth a follow-up if awards automation becomes a priority later.
- Sports-Reference's own ToS/bot-traffic pages return HTTP 403 to automated fetchers, so only a search-engine paraphrase of their terms was obtained, not the raw text.
