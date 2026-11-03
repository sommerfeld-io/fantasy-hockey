---
title: 'Reconcile the Epic 9 Contract to the As-Built'
type: 'docs'
created: '2026-11-03'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Epic 9 retrospective (item 70, `epic-9-retro-2026-10-01.md` "Spec-to-implementation reconciliation") found four places where the planning contract no longer matches what was built. (1) Story 9.6, AD-34 and `epic-9-context.md` say the example compose ships its own copy of the nginx config, and that a test checks the copy against the dev compose's. As built, `docs/examples/docker-compose.yml` mounts `../../configs/nginx/default.conf`, which was a human decision. There is no second copy and no drift test. (2) AD-33 names the emitter `observe.Audit`; as built it is `(*Observer).Audit` (`src/internal/observe/observe.go`). (3) AD-33 describes the mailer's recipient stripping as if it already existed; it was actually delivered in Story 9.4 (`b929488`). (4) AD-35 means a cleared division playoff-teams pick is saved but neither audited nor counted, and the contract never says that this is intentional.

**Approach:** Edit the planning documents (the `epics.md` Story 9.6 acceptance criterion, the AD-33, AD-34 and AD-35 text and the compose snippet comment in `ARCHITECTURE-SPINE.md`, and `epic-9-context.md`) so they describe the as-built system. No code, test or config changes. Then mark action item 70 `done` in `sprint-status.yaml`. As the user asked, also mark item 73 (the Raspberry Pi run of the example compose) `done`, because the user tested it and it behaved as expected.

</frozen-after-approval>

## Implementation Notes

- `ARCHITECTURE-SPINE.md`:
    - AD-33 now names `(*observe.Observer).Audit` and says the mailer recipient redaction came from Story 9.4 (`b929488`).
    - AD-34 now says both compose files mount the one repo file `configs/nginx/default.conf`, so there is no drift test, and that the example is not self-contained outside the repo. It is marked as amended on 2026-11-03.
    - AD-35 now says a cleared division playoff-teams row (empty team list) is persisted but not returned, and so is intentionally neither audited nor counted. This is checked against `SaveDivisionPicks` (`src/internal/store/store.go`, around line 833).
    - Fixed the compose snippet's proxy volume path and comment (`./nginx/` became `./configs/nginx/`, matching `docker-compose.yml:137`).
- `epics.md`: changed Story 9.6's "self-contained + drift test" acceptance criterion and its References line to describe the as-built mount. The criterion matches the example's header comment (`docs/examples/docker-compose.yml:30-32`).
- `epic-9-context.md`: changed the nginx-copy bullet and the `observe.Audit` name.
- `sprint-status.yaml`: marked action items 70 and 73 `done` (73 because the user reported testing it on the Pi), and bumped `last_updated`.
- Left alone on purpose: the closed review and memlog files under `architecture/.../reviews` and `.memlog.md`, which are historical records, and the spec for Story 9.6, which is done.
- Skipped `task go:run` because nothing changed in code or config. Ran `task lint` instead.

## Review Triage Log

- Blind Hunter: AD-34's Prevents line still listed "an example that only works inside this repo". Verdict: medium. Patched: it now names a second, drifting config copy.
- Blind Hunter: the nginx tags differ (dev `1.31.7`, example `1.31.6`) and the contract still says `stable-alpine`. Verdict: medium, but it existed before this change. Deferred to `deferred-work.md`.
- Blind Hunter: Story 9.6's Given/When no longer matched the new Then. Verdict: low, and the fix was simple. Patched: Given/When now read the volume and header comment, and the history moved to References.
- Blind Hunter: `epic-10-context.md:51` still said "its drift test". Verdict: medium. Patched.
- Blind Hunter: item 73 is closed without evidence in the YAML. Verdict: low. Rejected because the user reported the Pi run as tested and working. Action items have no evidence field, and the closure is recorded in this spec and in the commit.
- Blind Hunter: item 73 says "unmodified", but the example needs a repo clone. Verdict: low. Rejected because the user ran it successfully.
- Blind Hunter: only AD-34 had an amended marker. Verdict: low. Patched: markers added to AD-33 and AD-35.
- Blind Hunter: the cleared-pick statement went into AD-35 instead of AD-33. Verdict: low. Patched: AD-33 now points to AD-35, which is the AD that owns the returned-rows rule.
- Blind Hunter: AD-33 did not cite the commit. Verdict: low. Patched: `b929488` added and the redundant clause dropped.
- Blind Hunter: the spec frontmatter still said `in-progress`. Verdict: false, because finalizing sets `done`.
- Blind Hunter: no acceptance-test decision was recorded. Verdict: real process gap. These are planning documents only, with no behavior to test. Raised with the user when this was presented instead of being decided silently.
