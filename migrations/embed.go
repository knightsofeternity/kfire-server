// Package migrations embeds the SQL schema migrations so the server binary
// can apply them at startup without shipping loose files.
package migrations

import "embed"

// FS contains every *.sql migration, applied in lexical order.
//
//go:embed *.sql
var FS embed.FS
