package migrations

import "embed"

// FS embeds all versioned SQL migration files (*.up.sql and *.down.sql) from the root migrations/ directory.
//
//go:embed *.sql
var FS embed.FS
