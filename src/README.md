# fantasy-hockey

Entry point for the fantasy-hockey binary. `main` resolves the port to listen on via `internal/server` (`--port`/`-p`, default 8080) and serves `internal/web` on it. All application logic lives in the `internal/` packages; `main` only orchestrates.
