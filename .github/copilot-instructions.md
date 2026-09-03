# AI Instructions

This file defines the rules and conventions that AI coding assistants should follow when working in this repository. It is tool-agnostic and serves as the single source of truth for all AI-generated code and commit messages.

Go-specific coding rules live in `src/CLAUDE.md`, a symlink to `.github/instructions/go.instructions.md`. Run `task symlinks` from the repo root to (re)create it if needed.

## Commit Messages: Conventional Commits

Always use Conventional Commits: `<type>(optional scope): <description>`.

| Type                                                                 | Effect        |
|----------------------------------------------------------------------|---------------|
| `fix`                                                                | PATCH release |
| `feat`                                                               | MINOR release |
| `BREAKING CHANGE` footer                                             | MAJOR release |
| `build`, `chore`, `ci`, `docs`, `style`, `refactor`, `perf`, `test`  | No release    |

A scope may not contain a slash. Document breaking changes with a `BREAKING CHANGE:` footer, not the `!` notation. Commit titles must match `^(fix|feat|build|chore|ci|docs|style|refactor|perf|test)/[a-z0-9._-]+$`.

## Protected Files

**AI agents MUST NOT modify the following unless the user explicitly and unambiguously asks for a change to these specific files:**

- `Dockerfile` - container image definition
- `.github/workflows/**` - all GitHub Actions workflow files

If a lint check, test, or CI step fails, fix the application code or configuration - never the pipeline itself to work around a failure.

## Development Commands

All Go work runs from `src/`. The root `taskfile.yml` delegates to `src/taskfile.yml` via the `go:` namespace.

```bash
task go:build            # lint + vet + test + compile binary (full pipeline)
task go:test             # run unit tests with coverage report
task go:test:acceptance  # run acceptance tests with end-to-end coverage
task go:run               # build then run the binary locally
task lint                # project-wide linters (YAML, Markdown, filenames, Gherkin, Go, etc.)
task docker:build         # lint, test, and build the container image
```

## Test-Driven Development

Always follow TDD - the test comes before the implementation. Red (failing test) - Green (minimum implementation) - Refactor. Test observable behavior, not internal state. For every "should do X" test, add a "should not do Y" counterpart where it increases confidence.

## Behavior-Driven Development (BDD) and Acceptance Tests

Every feature development follows BDD, not just unit-level TDD:

- Before writing any implementation code for a feature, write (or update) an executable acceptance test spec as a Gherkin `.feature` file in `src/acceptance-tests/features/`, together with its step definitions. See `src/CLAUDE.md` for the Go-specific mechanics (GoDog, step definition location, suite wiring, coverage reporting).
- The acceptance test must fail (red) before the feature is implemented, then pass (green) once the feature is done. Acceptance tests describe observable, user-facing behavior - not internal implementation - and complement, not replace, unit-level TDD.
- "Feature development" means any user-visible or externally observable behavior change (new functionality, changed behavior, bug fixes that change observable output).

For infrastructure-related or other non-feature development (tooling, CI/CD, build config, dependency bumps, refactors with no observable behavior change, documentation), an acceptance test might not be the right tool. In these cases, ask the human in the loop whether an acceptance test is needed or can be skipped - do not silently decide either way.

This applies with equal force when using BMAD (`bmad-*` skills): when creating stories, specs, or driving implementation (e.g. `bmad-spec`, `bmad-create-epics-and-stories`, `bmad-build`), treat acceptance criteria as executable Gherkin scenarios in `src/acceptance-tests/features/`, not just prose bullet points. If a BMAD-driven change is infrastructure-related or otherwise non-feature work, ask the human in the loop whether acceptance tests apply before skipping them.
