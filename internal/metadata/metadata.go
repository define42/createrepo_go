// Package metadata exposes compatibility wrappers for repository metadata.
package metadata

import (
	"io"

	cr "github.com/define42/createrepo_go/pkg/createrepo"
)

// Package is the public RPM package metadata model.
type Package = cr.Package

// Repomd is the parsed representation of repomd.xml.
type Repomd = cr.Repomd

// RepomdRecord is one metadata record from repomd.xml.
type RepomdRecord = cr.RepomdRecord

// UpdateInfo is the public representation of updateinfo metadata.
type UpdateInfo = cr.UpdateInfo

// ParseRepomd parses repomd.xml from r.
func ParseRepomd(r io.Reader) (*Repomd, error) {
	return cr.ParseRepomd(r)
}

// ParseRepomdFile parses a repomd.xml file.
func ParseRepomdFile(path string) (*Repomd, error) {
	return cr.ParseRepomdFile(path)
}

// DumpRepomd serializes repository metadata.
func DumpRepomd(md *Repomd) string {
	return cr.DumpRepomd(md)
}

// ParsePrimaryFile parses primary XML metadata into packages.
func ParsePrimaryFile(path string) ([]Package, error) {
	return cr.ParsePrimaryFile(path)
}

// ParseFilelistsFile parses filelists XML metadata into packages.
func ParseFilelistsFile(path string, packages []Package) ([]Package, error) {
	return cr.ParseFilelistsFile(path, packages)
}

// ParseOtherFile parses other XML metadata into packages.
func ParseOtherFile(path string, packages []Package) ([]Package, error) {
	return cr.ParseOtherFile(path, packages)
}
