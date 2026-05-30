package createrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeRepositories(t *testing.T) {
	work := t.TempDir()
	repoA := filepath.Join(work, "a")
	repoB := filepath.Join(work, "b")
	out := filepath.Join(work, "merged")
	if err := osMkdirAll(repoA); err != nil {
		t.Fatal(err)
	}
	if err := osMkdirAll(repoB); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", "super_kernel-6.0.1-2.x86_64.rpm"), filepath.Join(repoA, "super_kernel-6.0.1-2.x86_64.rpm")); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", "Archer-3.4.5-6.x86_64.rpm"), filepath.Join(repoB, "Archer-3.4.5-6.x86_64.rpm")); err != nil {
		t.Fatal(err)
	}
	for _, repoPath := range []string{repoA, repoB} {
		if err := Create(context.Background(), Options{
			Directory:   repoPath,
			Checksum:    ChecksumSHA256,
			Compression: CompressionGzip,
			Revision:    "1700000005",
		}); err != nil {
			t.Fatalf("Create(%s) error = %v", repoPath, err)
		}
	}
	if err := Merge(context.Background(), MergeOptions{
		Repos:             []string{repoA, repoB},
		OutputDir:         out,
		RepoPrefixSearch:  work,
		RepoPrefixReplace: "file:///merged-source",
	}); err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	repo, err := LoadMetadata(context.Background(), out, LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata(merged) error = %v", err)
	}
	if len(repo.Packages) != 2 {
		t.Fatalf("len(Packages) = %d, want 2", len(repo.Packages))
	}
	for _, pkg := range repo.Packages {
		if pkg.LocationBase == "" {
			t.Fatalf("empty LocationBase for %s", pkg.Name)
		}
		if !strings.HasPrefix(pkg.LocationBase, "file:///merged-source") {
			t.Fatalf("LocationBase was not prefix-replaced: %q", pkg.LocationBase)
		}
	}
}

func osMkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}
