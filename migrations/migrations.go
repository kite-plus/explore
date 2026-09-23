// Package migrations embeds the SQL migrations that explore migrate applies.
package migrations

import "embed"

// FS holds every migration, applied in file name order.
//
//go:embed *.sql
var FS embed.FS
