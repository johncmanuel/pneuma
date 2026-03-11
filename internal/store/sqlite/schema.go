package sqlite

import "embed"

// Embedded migration directories, used by golang-migrate's iofs source driver.
// Each directory contains numbered migration files (e.g. 001_initial.up.sql).

//go:embed sql/server/migrations
var ServerMigrations embed.FS

//go:embed sql/desktop/migrations
var DesktopMigrations embed.FS
