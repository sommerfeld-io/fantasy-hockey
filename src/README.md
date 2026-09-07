# fantasy-hockey

Entry point for the fantasy-hockey binary. `main` resolves the database connection (`DATABASE_URL` env var or `--database-url` flag, flag wins, fail-fast if neither is set), opens the store and runs migrations, validates the required `SESSION_SECRET`/`SMTP_USERNAME`/`SMTP_APP_PASSWORD`/`PARTICIPANT_*` env vars, upserts the 3 fixed Participants, wires `internal/auth`, `internal/mailer`, and `internal/web` together, and serves HTTP on `HTTP_ADDR` (default `:8080`). All application logic lives in the `internal/` packages; `main` only orchestrates.
