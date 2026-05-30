package createrepo

import (
	"context"
	"testing"
)

func TestParsePrimaryFile(t *testing.T) {
	packages, err := ParsePrimaryFile(fixturePath(t, "tests", "testdata", "repo_01", "repodata", "6c662d665c24de9a0f62c17d8fa50622307739d7376f0d19097ca96c6d7f5e3e-primary.xml.gz"))
	if err != nil {
		t.Fatalf("ParsePrimaryFile() error = %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("len(packages) = %d, want 1", len(packages))
	}
	pkg := packages[0]
	if pkg.Name != "super_kernel" || pkg.Arch != "x86_64" || pkg.Version != "6.0.1" || pkg.Release != "2" {
		t.Fatalf("unexpected package identity: %#v", pkg)
	}
	if pkg.PkgID != "152824bff2aa6d54f429d43e87a3ff3a0286505c6d93ec87692b5e3a9e3b97bf" {
		t.Fatalf("PkgID = %s", pkg.PkgID)
	}
	if len(pkg.Requires) != 4 || len(pkg.Provides) != 4 || len(pkg.Conflicts) != 3 || len(pkg.Obsoletes) != 2 {
		t.Fatalf("dependency counts = req:%d prov:%d conf:%d obs:%d", len(pkg.Requires), len(pkg.Provides), len(pkg.Conflicts), len(pkg.Obsoletes))
	}
}

func TestLoadMetadataLoadsPackages(t *testing.T) {
	repo, err := LoadMetadata(context.Background(), fixturePath(t, "tests", "testdata", "repo_01"), LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata() error = %v", err)
	}
	if len(repo.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(repo.Packages))
	}
	pkg := repo.Packages[0]
	if len(pkg.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(pkg.Files))
	}
	if len(pkg.Changelogs) != 2 {
		t.Fatalf("len(Changelogs) = %d, want 2", len(pkg.Changelogs))
	}
}

func TestLoadMetadataFilelistsExt(t *testing.T) {
	repo, err := LoadMetadata(context.Background(), fixturePath(t, "tests", "testdata", "repo_04"), LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata(repo_04) error = %v", err)
	}
	var digest string
	for _, pkg := range repo.Packages {
		if pkg.Name != "super_kernel" {
			continue
		}
		for _, file := range pkg.Files {
			if file.Name == "super_kernel" {
				digest = file.Digest
			}
		}
	}
	if digest != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("digest = %q", digest)
	}
}
