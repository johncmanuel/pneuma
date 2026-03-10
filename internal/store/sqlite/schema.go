package sqlite

import _ "embed"

// Embedded canonical schemas, usable by any package that imports this one.
// Both are idempotent (CREATE … IF NOT EXISTS) and safe to Exec on every startup.

//go:embed sql/server/schema/main.sql
var ServerSchema string

//go:embed sql/desktop/schema/main.sql
var DesktopSchema string
