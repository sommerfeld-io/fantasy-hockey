# Pipeline and local wiring

## Local

- Taskfiles in `tests/playwright` and `tests/inspec`, included from the root `taskfile.yml`.
- Each takes an `IMAGE` variable: a locally built image locally, the registry `:sha` image in the pipeline.
- Playwright and InSpec run sequentially and reset the data file between runs.
- The pipeline calls the same taskfile commands, so a CI failure reproduces locally.
- The project linters (YAML, Markdown, filenames, Gherkin, Go, and so on) cover the new `tests/` folders.

## pipeline.yml job graph

| Job               | Needs                             | Behaviour                                                                   |
|-------------------|-----------------------------------|-----------------------------------------------------------------------------|
| build-image       | lint, lint-dockerfile, shellcheck | Unchanged: builds once, pushes the `:sha` image                             |
| playwright        | build-image                       | Pulls `:sha`; runs on its own worker; failure blocks `publish-edge`         |
| inspec            | build-image                       | Pulls `:sha`; runs on its own worker; failure blocks `publish-edge`         |
| docker-scout      | build-image                       | Runs in parallel with the tests; reports only; never gates                  |
| publish-edge      | playwright, inspec                | Re-tags the tested `:sha` image as edge; does not need `docker-scout`       |
| release-code      | publish-edge                      | Needs only `publish-edge`                                                   |
| cleanup-dockerhub | publish-edge, docker-scout        | Keeps `if: always()`; waits for scout so `:sha` is not deleted while pulled |

## release.yml additions

| Job                      | Needs           | Behaviour                                                                        |
|--------------------------|-----------------|----------------------------------------------------------------------------------|
| inspec                   | publish-release | Parallel to `upload-sbom` and `docker-scout`; does not break the run             |
| digest compare (with it) | publish-release | Compares the digest of the newly tagged `latest` with `edge`; never a `:sha` tag |
