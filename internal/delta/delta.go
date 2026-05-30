// Package delta exposes compatibility wrappers for delta RPM generation.
package delta

import (
	"context"

	cr "github.com/define42/createrepo_go/pkg/createrepo"
)

// Generate creates one delta RPM.
func Generate(oldRPM, newRPM string) error {
	return cr.GenerateDeltaRPM(context.Background(), cr.DeltaOptions{
		OldRPM:     oldRPM,
		NewRPM:     newRPM,
		OutputPath: newRPM + ".drpm",
	})
}
