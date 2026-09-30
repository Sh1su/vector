// Package migrations bettet die Schema-Migrationen ein (ADR-005).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
