// Package migrations embeds shop_analytics's schema migration files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
