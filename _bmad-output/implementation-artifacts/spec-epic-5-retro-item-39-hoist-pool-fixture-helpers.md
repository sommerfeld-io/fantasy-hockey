---
title: 'Hoist Duplicated Pool-Player Fixture Helpers Into fixture_support_test.go'
type: 'refactor'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `compare_predictions_steps_test.go` and `leaderboard_steps_test.go` each independently define near-identical pool-player fixture helpers: `comparePlayerID`/`leaderboardPlayerID` (both `strings.ToLower(name)`), `requirePoolPlayer` (same logic, slightly different error text), `thePoolPlayersAre` (identical body), and `seedBody`'s player-emitting loop (identical, embedded inside each file's otherwise-different `seedBody`). This is the 4th epic in a row with this exact same duplication finding (epic-2 item 12, epic-3 item 22, epic-4 item 34), and nothing prevents a 5th (epic-5 retrospective, item 39).

**Approach:** Extract a shared, embeddable `poolFixture` type (`poolNames []string` plus `requirePoolPlayer`/`thePoolPlayersAre` methods) and a `poolPlayerID`/`writeSeedPoolPlayers` free-function pair into `fixture_support_test.go`, following the exact embeddable-mixin pattern `lazyFixture` already establishes in that same file. Both `compareScenarioState` and `leaderboardScenarioState` embed `poolFixture` alongside their existing `lazyFixture` embed, and their own duplicated definitions are deleted. Consider adding a standing "check fixture_support_test.go first" note to `src/CLAUDE.md`'s (`.github/instructions/go.instructions.md`'s) acceptance-test guidance, given the 4-epic recurrence.

</frozen-after-approval>

## Implementation Notes

Added `poolFixture` (an embeddable `poolNames []string` mixin plus `requirePoolPlayer`/`thePoolPlayersAre` methods) and `poolPlayerID`/`writeSeedPoolPlayers` free functions to `fixture_support_test.go`, mirroring `lazyFixture`'s own embeddable-mixin pattern in the same file rather than adding pool-player fields to `lazyFixture` itself (7 of the 9 scenario states that embed `lazyFixture` have no pool-player concept at all). Both `compareScenarioState` and `leaderboardScenarioState` now embed `poolFixture` alongside `lazyFixture`; their own duplicated `comparePlayerID`/`leaderboardPlayerID`, `requirePoolPlayer`, `thePoolPlayersAre`, and `poolNames` field are deleted, and `seedBody`'s inline player-emitting loop in both files is replaced with a call to `writeSeedPoolPlayers`. Method promotion via embedding means `ctx.Step(..., s.thePoolPlayersAre)` and every other call site needed no changes - confirmed via `go test -v` that both features' "pool players are" steps now show `*poolFixture` as their receiver.

Also added the standing note the retro item suggested to `.github/instructions/go.instructions.md` (`src/CLAUDE.md`'s symlink target)'s acceptance-test guidance section, directing future work to check `fixture_support_test.go` first - not in CLAUDE.md's protected-files list (only `Dockerfile`/`.github/workflows/**` are protected).

Scoped as a pure dedup, not new test coverage (item 40, "close three Compare story-boundary test-coverage gaps," is the separate backlog item for that): the hoisted logic (case-folding, slice membership, slice assignment, string building) was already exercised only indirectly through Gherkin scenarios before this change, in duplicated form, with the same coverage shape after.

**Process note:** mid-build, an empirical regression-proof step (temporarily breaking `poolPlayerID`, then attempting to restore via `git checkout -- fixture_support_test.go`) accidentally discarded the entire uncommitted `poolFixture` hoist in that file, since `git checkout` restores to the last commit, not to "before this specific edit," and the hoist had never been committed. Caught immediately via the file's changed-since-read warning and a `git status`/`grep` check; redid the `fixture_support_test.go` additions from scratch (identical content) and re-verified everything before continuing. Every later regression-proof in this build used a `cp`-based backup/restore instead of `git checkout`, specifically to avoid repeating this.

Verified the hoisted `thePoolPlayersAre` genuinely gates both features: temporarily broke it (no-op instead of setting `poolNames`), confirmed 27 of 150 acceptance scenarios fail across both Compare and Leaderboard features with `no pool player "..."; seeded players are []`, then restored from a `cp` backup and re-confirmed green.

Verified: `gofmt -l` clean, `go vet ./...` clean, `go build ./...` clean, `task go:lint` 0 issues, `task lint` exit 0 (for the markdown doc change), `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:test:acceptance` full pipeline green (83.8% coverage, no regressions - same percentage as before this change, confirming the refactor is coverage-neutral), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied four `patch`-routed blind-hunter findings — added a doc comment to `thePoolPlayersAre` (matching `requirePoolPlayer`'s sibling comment, since it's now promoted into two scenario states via embedding rather than living privately in one file) stating it always sets exactly three players and carries no ordering significance of its own; added four direct unit tests for the newly-shared helpers (`TestPoolPlayerIDShouldLowercaseTheName`, `TestPoolFixtureRequirePoolPlayerShouldAcceptASeededName`/`...ShouldRejectAnUnseededName` as a should/should-not pair, `TestWriteSeedPoolPlayersShouldSkipTheSignedInPlayer`), matching this same file's existing local precedent (`TestRequireSeededPlayerShouldAcceptTheExactName`/`...ShouldRejectADifferentCase` for `requireSeededPlayer`) and CLAUDE.md's TDD convention - these functions were previously exercised only indirectly through Gherkin scenarios in duplicated form; corrected the new `.github/instructions/go.instructions.md` bullet's unverified "single most recurring retrospective finding" superlative (the reviewer's own count shows the `DefaultSeason` staleness cluster spans 5 items in epic-6 alone, more than this pool-fixture duplication's 4 occurrences) to the specific, verifiable fact instead (epic-2 item 12, epic-3 item 22, epic-4 item 34, epic-5 item 39); and added a caveat to the same bullet noting a hoisted helper isn't guaranteed to already generalize to a new scenario's exact shape, naming `thePoolPlayersAre`'s fixed three-player count as the concrete example. One finding dispositioned `low, reject`: the unified `requirePoolPlayer` error message dropped Compare's feature-specific wording ("no compare pool player" → "no pool player") - not fixed, since Go test failures already report the failing file/test/scenario alongside the message (confirmed in this build's own empirical regression-proof output), so the unified wording doesn't actually impair triage in practice, and the retro item's own action text explicitly named `requirePoolPlayer` as one of the things to hoist, implying message unification was an expected, accepted consequence.

**Re-verified after patches**: all four new unit tests pass, full acceptance suite green (83.8% coverage, unchanged), `gofmt -l` clean, `go vet ./...` clean, `go build ./...` clean, `task go:lint` 0 issues, `task lint` exit 0, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:test:acceptance` full pipeline green, `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, reject** — hoisting `requirePoolPlayer` dropped Compare's feature-specific error wording ("no compare pool player" → generic "no pool player"), making a Compare failure read identically to a Leaderboard one. Verified real, but not fixed: Go test output already reports the failing file/test/scenario alongside the message (confirmed in this build's own regression-proof, which showed clear feature-file context despite the identical message text), and the retro item's own action text explicitly named `requirePoolPlayer` as one of the four things to hoist - message unification was an expected, accepted consequence, not an oversight.
- **low, patch** — the new CLAUDE.md guidance told future authors to check `fixture_support_test.go` first without noting that `thePoolPlayersAre` hardcodes exactly three pool players, so a scenario needing a different count could be misled into assuming the hoisted helper already generalizes. Verified real (both original per-file copies were also hardcoded to three, carried over unchanged). Fixed: added the caveat to both the method's own doc comment and the CLAUDE.md bullet.
- **low, patch** — the new CLAUDE.md bullet's "this repo's single most recurring retrospective finding" claim doesn't hold up against the repo's own retro history: the `DefaultSeason` staleness cluster spans 5 items in epic-6 alone, more than this pool-fixture duplication's 4 occurrences. Verified real by the reviewer's own count. Fixed: replaced the superlative with the specific, verifiable fact (the 4 exact epic/item references).
- **low, patch** — no unit tests were added for the newly-hoisted, now cross-file-shared helpers, despite this same file's own local precedent (`TestRequireSeededPlayerShouldAcceptTheExactName`/`...ShouldRejectADifferentCase`) and CLAUDE.md's "should not" counterpart convention. Verified real: `requirePoolPlayer`'s error path was previously exercised only indirectly, through ~27 Gherkin scenarios per this spec's own earlier notes. Fixed: added four direct unit tests.
- **low, patch** — `thePoolPlayersAre` had no doc comment, unlike its sibling `requirePoolPlayer` immediately above it, despite now being promoted into two different scenario states via embedding rather than living privately in one file. Verified real. Fixed: added a doc comment (combined with the three-player-count caveat above).

All four patched findings were independently re-verified after patching: the four new unit tests pass, the full acceptance suite remains green with unchanged coverage, and the full `gofmt`/`go vet`/`go build`/`task go:lint`/`task lint`/`task go:test`/`task go:test:acceptance`/`task go:run` pipeline is clean.
