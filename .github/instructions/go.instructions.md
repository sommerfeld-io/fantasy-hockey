---
applyTo: "src/**/*.go"
---

# Go Code Style and Project Structure

Apply these instructions to all Go code in `src/`.

## Style priorities

- Prefer clarity, simplicity, maintainability, and consistency over clever code.
- Use standard language features and the standard library before introducing extra abstraction or dependencies.
- Keep local style consistent with nearby code unless that would worsen a deviation from the Google style guide.
- Format all Go code with `gofmt`; keep imports compatible with `goimports`.

## Project structure

- Keep all Go source code in `src/`.
- Keep `src/main.go` lightweight and focused on orchestration such as loading config, wiring dependencies, and starting the app.
- Put private application logic in `src/internal/**`.
- Put non-private packages in other `src/**` packages with short, descriptive package names.
- Keep acceptance tests in `src/acceptance-tests/`, with Gherkin feature files in `src/acceptance-tests/features/`. Acceptance tests are part of the main Go module - no separate `go.mod`.

## Naming

- Use short, descriptive, lowercase package names. Avoid underscores, dashes, mixed case, generic names like `util`, and unnecessary plurals.
- Use `PascalCase` for exported identifiers and `camelCase` for unexported identifiers.
- Prefer concise names that fit the local context and avoid stutter such as `user.Service` instead of `user.UserService`.
- Use short but meaningful local variable names.
- Use short, consistent receiver names.
- Keep interface names small and idiomatic; define interfaces where they are consumed.

## Magic values

- Never repeat the same literal string, number, or other fixed value across multiple call sites as a "magic value." Give it one home — a package-level `const` or `var` — and reference that everywhere instead, so a future rename or change only requires editing one place.
- Prefer a global variable or constant when the value itself is fixed and needs no computation (e.g. `store.DataFileName = "fantasy-hockey.yml"`, referenced by every Go file that needs that filename instead of hand-typing the string again).
- Prefer a small getter function over a bare variable when there is even the slightest need for an automated test around how the value is produced (e.g. it's derived, environment-dependent, or has a fallback/default to verify).
- Gherkin `.feature` files are a sanctioned exception: they describe behavior in prose, not Go code, so they may still reference a literal value like a filename directly.

## Functions and methods

- Keep functions small and focused on one responsibility.
- Prefer helper functions and better names over line-by-line comments that only restate the code.
- Use comments to explain why logic exists, important edge cases, or non-obvious trade-offs.
- Prefer the simplest implementation that keeps call sites easy to understand and hard to misuse.
- Long parameter lists. A method with eight parameters is doing too much, knows too much, or depends on too much. Get
nervous above four; refuse above six.

## Comments and documentation

- Use GoDoc-style `//` comments.
- Comment all exported packages, types, variables, constants, functions, and methods.
- Keep comments short, specific, and focused on behavior or rationale instead of narration.
- Avoid redundant comments that only describe what the code already says.
- Add a `README.md` file to document the purpose of each package.

## Line-level comments

Avoid commenting on *what* the code does. Use comments to explain the *why* when the logic is non-obvious or complex. Prefer refactoring complex logic into well-named helper functions over adding inline comments.

```go
// Bad: comment only restates what the code already says
// check if restaurant exists in the system
if restaurant == nil {
    return err
}

// Good: extract intent into a well-named function
if err := checkRestaurantExists(restaurant); err != nil {
    return err
}
```

## Imports

Split imports into three groups separated by blank lines:

1. Standard library
2. Third-party dependencies
3. Local packages from `github.com/sommerfeld-io/fantasy-hockey/src/...`

## Error handling

- Return `error` as the last return value from functions that can fail.
- Handle errors explicitly and early. Prefer the usual Go pattern:

```go
if err := someFunction(); err != nil {
    return err
}
```

- Avoid wrapping normal execution in `else` branches after an error check.
- Wrap lower-level errors with context by using `fmt.Errorf("context: %w", err)`.
- Define sentinel errors for expected failure cases that callers may need to detect.
- Lower-level packages should return or wrap errors, not log them.
- Boundary layers such as `main.go` or other entry points decide whether to log, convert, or terminate.
- Log errors to `stderr` and all non-error output to `stdout`, using structured logging whenever possible.
- Reserve `panic` for truly unrecoverable programmer or runtime faults, not normal control flow.

## Testing

- Follow TDD unconditionally. The test comes first - no exceptions.
- Keep unit tests next to the code they verify and use the `_test.go` suffix.
- Name tests with the `TestXxxShouldYyy` pattern.
- Cover happy paths, negative cases, and inversions so tests prove both what should and should not happen.
- Prefer table-driven tests when they improve clarity and reduce repetition.
- Use TDD as a Design Forcing Function: If a test is hard to write, do not power through, listen to the signal. Hard-to-write
tests almost always mean the code under test depends on too much. Refactor the design until the test is easy to write.
- Test Structure should follow the "Arrange -> Act -> Assert" pattern.
- As a validation, apply the Implementation-Swap Test: Ask, if I threw away this implementation and rewrote it from scratch, would my
tests still make sense? If yes, the tests are coupled to behavior. If not, they are coupled to implementation. But do not actually throw anything away! Just use this as a sort of validation.

- Prefer a global variable or constant when the value itself is fixed and needs no computation (e.g. `store.DataFileName = "fantasy-hockey.yml"`, referenced by every Go file that needs that filename instead of hand-typing the string again).

### Definition of Good Unit Tests / Good TDD

This is about using "tests as specifications", a way of using tests as self-validating programming for AI coding assistants. We need to make progress in small steps and make sure our tests are up to the job. Good tests are:

- **Understandable** - Our tests should describe what they are testing so that we can understand the goals of our software: Focused on the behavior of the system, not a specific implementation.
- **Maintainable** - We’d like our tests to act as a defense of our system, breaking when we want them to, but also remaining true as we change our system. So they need to be maintainable so that they are easy to change, without losing their intent.
- **Repeatable** - Our tests should be definitive, so should always pass or fail in the same way for a given version of the software that they are testing. We want our tests to be reliable: to give us the same result every time they run, wherever, or whenever, they are run.
- **Atomic** - Tests should be isolated and focus on a single outcome. We want tests with no side-effects and so able to achieve definitive results whatever else is going on. Independent of other tests and resources, we can run our tests in parallel, or in any order.
- **Necessary** - We don’t need to create “tests for test sake”. Our aim is to guide development from tests, so we should create tests that help us, are necessary, to guide our development choices.
- **Granular** - Tests should be small, simple and focused, and assert a single outcome of the code that they are testing. Giving us a clear Pass/Fail result, and a clear indication of what the problem is when they fail, that do not require interpretation.
- **Fast** - We will use tests as a tool to guide our development, so we will end up with lots of them. That means we need them to be efficient and give us results quickly - our tests need to be fast. Easiest way to do that is to keep them simple and focused.
- **First** - In TDD we write the test before writing code. This has a huge impact on our ability to achieve the properties that we value in good tests, compared to writing tests after we have written the code (Unit Testing).

### Acceptance tests and Gherkin feature files

- `.feature` files define GoDog acceptance tests. They are **not** used to derive unit tests.
- Step definitions live in `src/acceptance-tests/*_steps_test.go`.
- Wire the GoDog suite in `src/acceptance-tests/suite_test.go`.
- Before writing a new fixture helper (seeding pool players, writing a YAML section, embeddable scenario state), check `src/acceptance-tests/fixture_support_test.go` first - a near-identical pool-player fixture helper was independently duplicated across step files and flagged in 4 epics in a row before it was finally hoisted (epic-2 item 12, epic-3 item 22, epic-4 item 34, epic-5 item 39) - and a hoisted helper isn't guaranteed to already generalize to a new scenario's exact shape (e.g. `poolFixture.thePoolPlayersAre` only ever sets exactly three players).
- Acceptance test Go code follows the same TDD discipline as production code. Any non-trivial helper extracted into a non-test file must have its own `*_test.go` unit tests.
- Unit test coverage is measured with `go test ./internal/...` and written to `coverage.out`. Acceptance tests are run separately with `go test -coverpkg=./internal/... ./acceptance-tests/...`, write a second report to `acceptance-coverage.out`, and gate the `build` task. The two coverage files are intentionally separate signals: `coverage.out` measures branch-level unit test correctness; `acceptance-coverage.out` measures which internal lines are reachable end-to-end.
