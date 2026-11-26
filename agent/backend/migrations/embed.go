// Package migrations embeds the SQL migrations of the conversation store, applied in file name order when
// DATABASE_URL is set. Never edit a file that was released: add a new pair instead.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
