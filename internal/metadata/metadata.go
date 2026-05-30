package metadata

import (
	"io"

	cr "github.com/rpm-software-management/createrepo_c/pkg/createrepo"
)

type Package = cr.Package
type Repomd = cr.Repomd
type RepomdRecord = cr.RepomdRecord
type UpdateInfo = cr.UpdateInfo

func ParseRepomd(r io.Reader) (*Repomd, error) {
	return cr.ParseRepomd(r)
}

func ParseRepomdFile(path string) (*Repomd, error) {
	return cr.ParseRepomdFile(path)
}

func DumpRepomd(md *Repomd) string {
	return cr.DumpRepomd(md)
}

func ParsePrimaryFile(path string) ([]Package, error) {
	return cr.ParsePrimaryFile(path)
}

func ParseFilelistsFile(path string, packages []Package) ([]Package, error) {
	return cr.ParseFilelistsFile(path, packages)
}

func ParseOtherFile(path string, packages []Package) ([]Package, error) {
	return cr.ParseOtherFile(path, packages)
}
