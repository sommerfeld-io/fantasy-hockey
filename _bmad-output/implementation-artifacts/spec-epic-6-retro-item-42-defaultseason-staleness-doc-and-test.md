---
title: 'Document DefaultSeason Staleness Risk and Add a Format Regression Test'
type: 'docs'
created: '2026-09-29'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `store.DefaultSeason` (`internal/store/store.go`) is baked into the binary at build time. Repointing `DATA_FILE`/`--data-file` at a genuinely new season without first bumping `DefaultSeason` in Go source and rebuilding/redeploying silently bootstraps the wrong season label - `store.New`'s bootstrap branch has no way to know the constant is stale. `docs/operator-guide.md` already carries an operator-facing warning row for this (added 2026-09-28, after the epic-6 retrospective identified the gap), but `epic-6-context.md` - the dev/planning-facing context doc - still doesn't mention it, and every existing test referencing `DefaultSeason` is a value-equality tautology against the constant's own literal value, proving nothing about its format staying sane (epic-6 retrospective, item 42).

**Approach:** Add a note to `epic-6-context.md`'s Requirements & Constraints cross-referencing the existing operator-guide.md warning, so the risk is documented on both the dev-planning side and the operator side, not just one. Add a lightweight format-only regression test (`YYYY-YY` pattern) as the "partial backstop" the retro item itself suggested, explicitly not a value-equality check against the literal season string.

</frozen-after-approval>

## Implementation Notes

Discovered `docs/operator-guide.md` already carries an operator-facing warning row for this exact risk (added 2026-09-28, in a general docs commit that landed after the 2026-09-25 epic-6 retrospective identified the gap) - narrowed scope to what was actually still missing: an `epic-6-context.md` cross-reference (Requirements & Constraints, one bullet) and the format-only regression test.

Added `TestDefaultSeasonShouldMatchTheExpectedFormat` to `internal/store/store_test.go`: a package-level `seasonFormat` regexp (`^\d{4}-\d{2}$`) and a test asserting `DefaultSeason` matches it. Deliberately not a value-equality check against the literal `"2026-27"` string (which the retro item itself calls out as tautological) - this only catches a malformed constant, not a merely-stale-but-well-formed one (a stale value is indistinguishable from a current one by format alone; only a human bumping the constant closes that gap, which is what the operator-guide.md warning is for).

Verified the new test actually catches a real regression: temporarily changed `DefaultSeason` to `"2026-2027"` (malformed), confirmed the test fails, then restored via `git checkout`.

Verified: `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task lint` exit 0 (markdown/other project-wide checks, for the `epic-6-context.md` edit), `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

Nothing incomplete or risky.

**Review patches:** applied all four `patch`-routed blind-hunter findings — added a should-not counterpart test (`TestSeasonFormatShouldRejectMalformedValues`) proving `seasonFormat` itself rejects `"2026-2027"`/`"26-27"`/`"2026/27"`/a trailing-space variant/empty string, so an accidental future loosening of the pattern has permanent regression coverage, not just the one-off manual check recorded in these notes; narrowed `seasonFormat`'s doc comment to scope it strictly to `DefaultSeason` and explicitly cross-referenced spec-1-5's prior, still-standing decision *not* to add format/fuzz validation for the hand-edited `season:` field (the original comment's wording implied this test also covered that field, which it doesn't and shouldn't); added the staleness warning directly to `DefaultSeason`'s own doc comment in `store.go` (the single most direct place a future editor bumping the constant would actually look - the other two docs only pointed at the constant from elsewhere, not from it); and corrected the `epic-6-context.md` bullet's "before repointing `DATA_FILE`/`--data-file`" wording, which read as forbidding that ordering outright - the real constraint is the corrected binary must be running before the app *starts up* against the new path, not that the env var/flag can never be set first. Four findings dispositioned `false`/`reject`: spec/sprint-status tracking still showing in-progress/open mid-review is expected workflow state; `epic-6-context.md`'s own header explicitly says "Edit freely" (regeneration is an opt-in event tied to planning-doc changes, not an automatic overwrite risk) so hand-appending to it is sanctioned, matching precedent from the epic-6-item-44 build's own hand-edit to this same file; the BDD/acceptance-test policy question was already resolved as a standing decision (epic-8-item-63 build) for pure test/doc hardening; and the regex checking pattern-only rather than full YYYY-YY year-continuity (e.g. rejecting `"2026-11"`) matches the retro item's own explicitly scoped ask ("a lightweight format-only regression test... a YYYY-YY pattern check") - stricter validation would exceed "lightweight" and duplicate parsing logic nothing else in the codebase has.

**Re-verified after patches**: both `TestDefaultSeasonShouldMatchTheExpectedFormat` and the new `TestSeasonFormatShouldRejectMalformedValues` pass, `gofmt -l` clean, `go vet ./...` clean, `task go:lint` 0 issues, `task lint` exit 0, `task go:test` full pipeline green (95.4% total coverage, no regressions), `task go:run` builds and starts.

## Review Triage Log

*(blind-hunter, oneshot route)*

- **low, patch** — no should-not counterpart proving `seasonFormat` itself rejects a malformed value; only a one-off manual check (temporarily break `DefaultSeason`, watch the test fail, revert) was recorded, giving no permanent regression coverage. Verified real per this repo's own TDD convention (CLAUDE.md). Fixed: added `TestSeasonFormatShouldRejectMalformedValues`.
- **low, patch** — `seasonFormat`'s doc comment claimed to cover "a hand-edited season: field" too, in direct tension with spec-1-5's Design Notes, which explicitly rejected adding format/fuzz validation for that same hand-maintained operator config value. Verified real by reading spec-1-5's Review Triage Log (line 110: "`Season` is a hand-maintained operator config value... not user input; no other config value gets this kind of fuzz coverage"). Fixed: narrowed the comment's scope to `DefaultSeason` alone and cross-referenced the prior decision.
- **low, patch** — `DefaultSeason`'s own doc comment (the single most direct place a future editor bumping the constant would look) said nothing about the staleness risk; both new/updated docs pointed at the constant from elsewhere, not from it. Verified real. Fixed: added the warning directly to the constant's own doc comment.
- **low, patch** — the `epic-6-context.md` bullet's "before repointing `DATA_FILE`/`--data-file` at the fresh path" phrasing is stricter than the real constraint (repointing the env var/flag by itself triggers nothing; the corrected binary must be running before the app *starts up* against the new path). Verified real. Fixed: reworded to state the actual constraint.
- **false** — spec frontmatter and `sprint-status.yaml` still showed in-progress/open at review time. Not a defect: expected mid-review workflow state, resolved as the final step of this same build.
- **false** — hand-appending to `epic-6-context.md` risks being silently dropped on a future regeneration. Not a defect: the file's own header states "Edit freely. Regenerate... if planning docs change" - regeneration is an opt-in, human-triggered event tied to planning-doc changes, not an automatic overwrite; this also matches unchallenged precedent from the epic-6-item-44 build's own hand-edit to this exact file.
- **false** — reviewer flagged that CLAUDE.md's BDD policy requires asking whether an acceptance test is needed for non-feature work, with no record of that question being asked. Not a defect: already resolved as a standing decision in the epic-8-item-63 build - docs plus a unit test, zero observable behavior change, squarely fits that standing policy.
- **low, reject** — the regex checks digit-count shape only (`^\d{4}-\d{2}$`), not YYYY-YY year-continuity, so `"2026-11"`/`"9999-00"` would pass. Not fixed: the retro item's own wording explicitly scoped this to "a lightweight format-only regression test... a YYYY-YY pattern check" - a stricter continuity check exceeds "lightweight" and would require parsing/validation logic that doesn't exist anywhere else in the codebase for a hand-maintained value.

All four patched findings were independently re-verified after patching: both format tests pass, and the full `gofmt`/`go vet`/`task go:lint`/`task lint`/`task go:test`/`task go:run` pipeline is clean.
