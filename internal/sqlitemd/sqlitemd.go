package sqlitemd

import (
	"context"

	cr "github.com/rpm-software-management/createrepo_c/pkg/createrepo"
)

// Generate writes yum sqlite metadata.
func Generate(ctx context.Context, opts cr.SQLiteOptions) error {
	return cr.GenerateSQLite(ctx, opts)
}
