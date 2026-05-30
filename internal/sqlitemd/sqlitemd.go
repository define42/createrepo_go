// Package sqlitemd exposes compatibility wrappers for SQLite metadata.
package sqlitemd

import (
	"context"

	cr "github.com/define42/createrepo_go/pkg/createrepo"
)

// Generate writes yum sqlite metadata.
func Generate(ctx context.Context, opts cr.SQLiteOptions) error {
	return cr.GenerateSQLite(ctx, opts)
}
