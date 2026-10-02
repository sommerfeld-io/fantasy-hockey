# fantasy-hockey

Entry point for the fantasy-hockey binary. `main` resolves the port to listen on via `internal/server` (`--port`/`-p`, default 8080) and serves `internal/web` on it. All application logic lives in the `internal/` packages; `main` only orchestrates.

## Data file persistence

`internal/store` never writes the data file in place. Every save marshals the in-memory document, writes it to a temp file in the same directory, then `os.Rename`s the temp file over the real one. Rename is atomic, so a crash mid-write can't leave a truncated file, and a concurrent reader never sees a torn write.

This only works if the data file's path is an ordinary file inside a mounted directory, not a bind-mount point itself - `rename(2)` can't replace a mount point. That's why `docker-compose.yml` bind-mounts the whole `src/` directory into the container instead of just `fantasy-hockey.yml`.
