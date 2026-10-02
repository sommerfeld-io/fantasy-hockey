# InSpec controls

InSpec runs in its own container against a running container of the app image (`docker run -d` with the image's real `CMD` and `SESSION_SECRET` set, wait for startup, then dockerized `inspec exec --target docker://<container> --no-distinct-exit`, then stop and remove). Profile location: `tests/inspec`.

| Control          | Asserts                                                                                                                        | Priority |
|------------------|--------------------------------------------------------------------------------------------------------------------------------|----------|
| Non-root user    | The container runs as a non-root user (`fantasy-hockey`, uid 1000)                                                             | Must     |
| Binary           | `/opt/fantasy-hockey/fantasy-hockey` exists and is executable                                                                  | Must     |
| No source        | `/workspaces/fantasy-hockey/src` is gone; no `*.go`, `go.mod`, `go.sum`, `.git`, `taskfile.yml`, `Dockerfile`, or Go toolchain | Must     |
| Pinned base      | The alpine release matches the version in the `Dockerfile` `FROM` line                                                         | Must     |
| No setup tools   | `curl` and `git` are absent                                                                                                    | Must     |
| Alpine           | The image is alpine-based (`os.name`)                                                                                          | Must     |
| Serves metrics   | The app answers `/metrics` inside the container (busybox `wget`; `curl` is banned)                                             | Must     |
| Maintainer label | The `maintainer` label equals `sebastian@sommerfeld.io`, read through the `docker_image` resource                              | Must     |
| Allowlist        | Only `/opt/fantasy-hockey` and a few known paths hold files beyond the base OS                                                 | Should   |
| Size ceiling     | Image size via `docker_image` stays under 6.5 MB (5.33 MB today on arm64)                                                      | Should   |
