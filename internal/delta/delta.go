package delta

import (
	"context"

	cr "github.com/rpm-software-management/createrepo_c/pkg/createrepo"
)

// Generate creates one delta RPM.
func Generate(oldRPM, newRPM string) error {
	return cr.GenerateDeltaRPM(context.Background(), cr.DeltaOptions{
		OldRPM:     oldRPM,
		NewRPM:     newRPM,
		OutputPath: newRPM + ".drpm",
	})
}
