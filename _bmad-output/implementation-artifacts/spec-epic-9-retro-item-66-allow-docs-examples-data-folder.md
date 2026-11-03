---
title: 'Allow docs/examples/data in .folderslintrc'
type: 'bugfix'
created: '2026-11-03'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `docs/operator-guide.md` and the header of `docs/examples/docker-compose.yml` tell the operator to `mkdir -p docs/examples/data`. That directory is git-ignored, but `folderslint` still sees it on disk, and `task lint` then fails with "Directory is not allowed by config" (Epic 9 retro action item 66).

**Approach:** Add `docs/examples/data` to the `rules` in `.folderslintrc`, next to `docs/examples`. Re-run the folder linter with the directory present and confirm it passes. Then mark item 66 `done` in `sprint-status.yaml`. Commit onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract` (user instruction: no new branch).

</frozen-after-approval>

## Implementation Notes

- Red: with `docs/examples/data/` present, `docker compose up lint-folders` failed (folderslint: 1 error, exit 1).
- `.folderslintrc`: added `docs/examples/data/**`. The `/**` form lets subdirectories such as an `archive/` of old season files also pass.
- `.yamllint.yml`: added `docs/examples/data/` to `ignore`, next to `src/fantasy-hockey.yml`. Without it, yamllint linted the operator's pool file (`document-start` warnings), which holds player emails and is not repo content.
- Green: with `data/fantasy-hockey.yml`, a second season file and `data/archive/…yml` present (copies of the dev state file), `lint-folders` and `lint-yaml` exit 0 and never mention `examples/data`. The full `task lint` also exits 0. Afterwards I deleted the throwaway `docs/examples/data/`, which is git-ignored.
- `sprint-status.yaml`: marked item 66 `done` and bumped `last_updated`.
- Committed onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract`, as the user instructed.
- No Go code changed, so `task go:run` does not apply.

## Review Triage Log

- Blind Hunter: yamllint would lint a real pool file in `docs/examples/data/`. Verdict: medium. Checked: it lints the file but reports only warnings, so it does not fail today, but it lints private pool data. Patched with a yamllint ignore. `.ls-lint.yml` (filenames) was checked with real files present and passes.
- Blind Hunter: the folder rule allowed only the exact path. Verdict: low, and the fix was simple. Patched to `docs/examples/data/**`.
- Blind Hunter: Implementation Notes were empty. Verdict: false. The notes are filled at finalization.
- Blind Hunter: the spec was `in-progress` while the item was `done`. Verdict: false. Finalizing sets `done`, in the same commit.
- Blind Hunter: the Approach said "folder linter" while the item says `task lint`. Verdict: low. The full `task lint` was run with realistic data, as recorded above.
- Blind Hunter: no acceptance-test decision was recorded. Verdict: real process gap. This is a lint-config bug fix with no app behavior. Raised with the user when this was presented.
- Blind Hunter: no `baseline_commit`. Verdict: low. Rejected because the template has no such field.
- Blind Hunter: the branch name covers a different item. Verdict: low. Rejected because the user explicitly asked for this branch; the PR description should list items 70, 73, 91 and 66.
