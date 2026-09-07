package store

import "embed"

// migrationsFS embeds the golang-migrate migration files so they ship inside
// the compiled binary instead of being read from disk at runtime.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS
