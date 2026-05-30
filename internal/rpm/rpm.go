// Package rpm exposes compatibility wrappers for reading RPM package metadata.
package rpm

import cr "github.com/define42/createrepo_go/pkg/createrepo"

// ReadPackage parses one RPM into the public package model.
func ReadPackage(path string, checksum cr.ChecksumType) (cr.Package, error) {
	return cr.ParseRPMPackage(path, "", checksum)
}
