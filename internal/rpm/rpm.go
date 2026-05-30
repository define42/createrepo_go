package rpm

import cr "github.com/rpm-software-management/createrepo_c/pkg/createrepo"

// ReadPackage parses one RPM into the public package model.
func ReadPackage(path string, checksum cr.ChecksumType) (cr.Package, error) {
	return cr.ParseRPMPackage(path, "", checksum)
}
