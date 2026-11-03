---
title: 'Acceptance Steps Assert the Real Mail Startup Warning Lines'
type: 'refactor'
created: '2026-11-03'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The two mail startup warnings ("ignoring mail settings of the unselected transport" and "mail settings of the selected transport are missing") are logged by `newSender` in `src/main.go`. The acceptance steps in `mail_transport_steps_test.go` cannot call `main`, so they assert on `mailer.IgnoredVars` and `mailer.MissingVars` instead of the log lines a person running the pool actually sees. A regression in the wording, the level, or the names-never-values guarantee of the real line would pass the acceptance suite (Epic 10 retro action item 99).

**Approach:**

- Move the `newSender` logic into `internal/mailer` as `Setup(settings Settings, logger *slog.Logger) (Sender, error)`. It builds the selected transport with `New` and logs each warning once at WARN through the injected logger. The two messages become exported constants so there is one home for the wording.
- `main.go` calls `mailer.Setup(mailer.SettingsFromEnv(os.Getenv), slog.Default())`. Behavior stays the same: same messages, same `variables` attribute, same error for an invalid `MAIL_TYPE`.
- The `newSender` unit tests move to `internal/mailer` as `Setup` tests, written first. `TestRunShouldStopStartupOnAnInvalidMailType` stays in `main_test.go` as the wiring proof.
- The acceptance step `theAppStarts` calls `mailer.Setup` with a capturing logger. The warning steps assert the real WARN lines: message, names present, values absent, or no such line.
- The feature file is unchanged. Mark item 99 `done`. Commit onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract` (user instruction: no new branch).

</frozen-after-approval>

## Implementation Notes

- `src/internal/mailer/setup.go` (new):
    - `Setup(Settings, *slog.Logger) (Sender, error)` and the constants `WarnIgnoredSettings` and `WarnMissingSettings`, with the wording unchanged from `main.go`.
    - Note that the Intent's lowercase quote of the second message is a paraphrase. The real text starts with "Mail…" and was kept as it was.
- `src/internal/mailer/setup_test.go` (new, written first, red because `Setup` was undefined):
    - the eight `newSender` tests moved from `main_test.go`;
    - a wording pin for both constants;
    - a test that logs both warnings in one startup.
- `src/main.go`: `newSender` removed; `run()` calls `mailer.Setup(mailer.SettingsFromEnv(os.Getenv), slog.Default())`. `src/main_test.go`: the moved tests were removed, and `TestRunShouldStopStartupOnAnInvalidMailType` stays as the wiring proof.
- `src/acceptance-tests/mail_transport_steps_test.go`:
    - `theAppStarts` calls `mailer.Setup` with a logger that writes into the scenario's captured log, and the `ignored`/`missing` fields are gone.
    - The warning steps assert the real WARN line: `warning` requires exactly one, `noWarning` requires zero, and `warnedAboutExactly` checks the `variables` attribute.
    - The value-leak steps check the whole log.
    - One `logLines` scanner is shared with `findLogLine`.
- `src/internal/mailer/README.md`: documents `Setup`.
- Mutation checks on `setup.go`, each restored from a saved copy in the same command (`cmp` showed it identical afterwards). Each one fails the acceptance suite; the old steps would have passed all four:
    - dropping the ignored warning
    - adding the SMTP host value to it
    - logging the missing warning at INFO
    - adding the Gmail secret to it
- `go test -race` passes for the acceptance suite, and `task lint` passes (gocyclo too). `task go:run` passes golangci-lint and all unit and acceptance tests, then exits non-zero only on govulncheck `GO-2026-6809` (Go 1.27.2 pinned by the protected `Dockerfile`). That question is already with the user from item 87.
- Acceptance tests: the feature file is unchanged, and its steps now assert stronger, real behavior. No new scenario is needed for a refactor with no observable change.
- Committed onto the existing branch `docs/epic-9-retro-item-70-reconcile-contract`, as the user instructed.

## Review Triage Log

- Blind Hunter: the steps reuse the message constants, so a rewording goes unnoticed. Verdict: medium. Patched with a unit test pinning both literal texts.
- Blind Hunter: `noWarning` passed when the warning was logged twice. Verdict: medium. Patched to require zero matching lines.
- Blind Hunter: the value-leak check looked at one line only. Verdict: medium. Patched to check the whole captured log.
- Blind Hunter: `warning` duplicated the `findLogLine` scan. Verdict: low. Patched with a shared `logLines`.
- Blind Hunter: no test covered both warnings in one startup. Verdict: low. Patched with a new unit test.
- Blind Hunter: nothing proves `run()` passes the real logger. Verdict: medium. Deferred to `deferred-work.md`, because the fix needs a refactor of `run()`.
- Blind Hunter: no nil-logger guard. Verdict: low. Patched by documenting that the logger must not be nil (doc comment and README), with no fallback.
- Blind Hunter: the README overstated when the warnings are logged. Verdict: low. Patched.
- Blind Hunter: inconsistent constant style, and the spec quote does not match. Verdict: low. Style rejected, because the wording is user-visible behavior this refactor must keep. The quote is noted above (frozen block).
- Blind Hunter: the spec was unfinished. Verdict: false. Finalizing sets `done`.
- Blind Hunter: no acceptance-test decision recorded. Verdict: low. Recorded above.
- Blind Hunter: the `ref` still points to the old spec. Verdict: low. Rejected because `ref` records where the item came from.
- Blind Hunter: `IgnoredVars`/`MissingVars` could be unexported. Verdict: low. Rejected as out of scope; it is an API change not asked for.
