# Package: `server`

Resolves the HTTP port to listen on and runs the HTTP server.

## Responsibilities

- `ResolvePort(args)` parses `--port`/`-p` from the given arguments, falling back to `DefaultPort` (8080) when neither is set.
- `Addr(port)` formats a port as a `net/http` listen address, e.g. `:8080`.
- `Run(ctx, port, handler)` starts `handler` on `port`, logs the port it is listening on via `slog` so it is clearly visible at startup, and blocks until `ctx` is done, shutting down gracefully.

## Design notes

- `ResolvePort` uses its own `flag.FlagSet` rather than the global `flag.CommandLine`, so it can be called with an explicit `args` slice and is unit-testable without touching `os.Args`.
- `Run` uses conservative HTTP server timeouts (read header, read, write, idle) to keep a slow or malicious client from holding a connection open indefinitely.
