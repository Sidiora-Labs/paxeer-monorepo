// Package migrations embeds the API's goose SQL migrations.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

// Migrations returns the embedded migration set for the service and its tests.
func Migrations() embed.FS {
	return FS
}
