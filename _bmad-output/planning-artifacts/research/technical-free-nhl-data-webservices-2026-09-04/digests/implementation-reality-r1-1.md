# Digest: NHL & Free Sports Data API — Implementation Reality & Ecosystem Health

## Claims

1. Claim: The NHL retired its old public API (statsapi.web.nhl.com) around Sept–Nov 2023 and replaced it with a new, also-undocumented API at api-web.nhle.com, with no advance notice to third-party developers.
   Source: https://discuss.tidbyt.com/t/nhl-api-change/5962 | Publisher: Tidbyt developer forum | Date: Nov 2023 | Accessed: 2026-09-04 | Confidence: high | Class: stability, outage-history

2. Claim: The 2023 transition broke multiple production consumer apps (Tidbyt's "NHL Live," "Sports Scores," "NHL Next Game"); a community fix landed within ~days for one app, but edge cases (overtime/shootout status codes) needed iterative follow-up fixes over subsequent weeks, and some data fields (e.g., human-readable last-goal description) were permanently lost in the new API.
   Source: same Tidbyt thread | Confidence: high | Class: stability, schema-change

3. Claim: The original community documentation project (dword4/nhlapi, mirrored on GitLab) documented the pre-2023 statsapi.web.nhl.com endpoints; the NHL's history shows it has fully swapped API generations before, not just tweaked endpoints.
   Source: https://github.com/dword4/nhlapi (and https://gitlab.com/dword4/nhlapi) | Publisher: GitHub/GitLab, community project by Drew Hynes | Date: undated (long-running, since ~2018) | Accessed 2026-09-04 | Confidence: medium | Class: docs-quality, stability

4. Claim: A community-maintained reference for the current API, Zmalski/NHL-API-Reference, documents both api-web.nhle.com and api.nhle.com/stats/rest, has 596 stars/63 forks, 49 open issues, and shows real recent commit activity (contributions dated April 2025, October 2025, and November 2025).
   Source: https://github.com/Zmalski/NHL-API-Reference | Publisher: GitHub (community repo) | Date: commits through Nov 2025 | Accessed 2026-09-04 | Confidence: high | Class: docs-quality

5. Claim: Neither the old nor new NHL API has ever published an official rate-limit policy; developers rely on undocumented, trial-and-error throttling and treat IP-blocking as a real but unconfirmed risk.
   Source: https://github.com/dword4/nhlapi/issues/21 ("API Limits") | Publisher: GitHub issue, community discussion | Date: undated, issue never resolved with concrete numbers | Accessed 2026-09-04 | Confidence: medium | Class: shutdown-risk, rate-limiting (this is a question, not a confirmed incident — no one reported actually being blocked)

6. Claim: Client libraries built against the NHL API (e.g., R's nhlscraper, various npm/PyPI wrappers) implement HTTP 429 handling and retry/backoff logic as a defensive default, implying rate limiting is encountered in practice even though undocumented.
   Source: https://rdrr.io/cran/nhlscraper/man/nhl_api.html and general library docs surfaced in search | Publisher: CRAN package docs | Date: undated | Accessed 2026-09-04 | Confidence: low-medium (inferred from defensive code, not an incident report)

7. Claim: ESPN officially shut down its public developer API/portal in 2014; what developers use today (site.api.espn.com and similar) are unofficial/hidden JSON endpoints powering ESPN's own site and apps, undocumented, with no SLA, and "can change without notice."
   Source: https://gist.github.com/akeaswaran/b48b02f1c94f873c6655e7129910fc3b and https://publicapis.io/espn-sports-api | Publisher: developer gist + aggregator site | Date: undated/2026 guide | Accessed 2026-09-04 | Confidence: medium (aggregator content likely recycled from the same gist — treat as one source, not two independent confirmations)

8. Claim: MoneyPuck is not an API but a downloadable-CSV data site, single-maintainer (Peter Tanner), run on AWS; the about page discloses no uptime, SLA, or access-limit information at all.
   Source: https://moneypuck.com/about.htm | Publisher: MoneyPuck.com | Date: undated | Accessed 2026-09-04 | Confidence: high for "no SLA disclosed"/single-maintainer; not applicable as a polling-API candidate since it's bulk file downloads, not a queryable webservice.

## Leads worth chasing further

- pseudo-r/Public-NHL-API and Public-ESPN-API repos claim to package a Django REST wrapper and "complete" endpoint catalogs for both NHL and ESPN — worth a deeper look at maintenance recency and whether they document actual rate-limit behavior observed in production (not yet verified).
- jakubsvobodacz/nhl-api-client (TypeScript, npm) claims coverage of "NHL Edge analytics" and shift charts — could indicate how deep community reverse-engineering goes, but its own stability track record wasn't checked.
- The EA Forums mention (June 2025) of a user unable to reach NHL API endpoints ("NHL API Issues") is a dangling, unconfirmed report — worth checking if this was a real NHL-side outage or an EA/NHL-25-game-specific integration issue; could not disambiguate from the snippet.
- Natural Stat Trick was named in the brief but never surfaced in results — no evidence gathered on it at all.
- No source discussed what happens on the new api-web.nhle.com specifically around playoff Game 7s or awards-night traffic spikes — this remains an open gap.

## What could NOT be confirmed

- No documented outage windows tied to specific high-traffic moments (Stanley Cup Final Game 7s, awards announcements) for either the old or new NHL API — searches returned no incident reports, only general/old forum chatter unrelated to API status.
- No confirmed instance of a hobbyist being IP-blocked or rate-limited by the NHL for periodic (e.g., daily/nightly) polling — the only rate-limit discussion found is an unanswered question, not an incident.
- No official or semi-official statement from the NHL about the current API's intended audience, support policy, or future stability — it remains, by every source, entirely undocumented and unofficial.
- Natural Stat Trick — no information gathered (not returned by any query used).
- Could not verify whether the 2023 API migration is a one-time historical event or represents a recurring pattern (only one such full migration is documented, ~2018-era API to 2023 api-web.nhle.com; no second migration since found).
