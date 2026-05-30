package createrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateEmptyRepository(t *testing.T) {
	dir := t.TempDir()
	if err := Create(context.Background(), Options{
		Directory:              dir,
		Checksum:               ChecksumSHA256,
		Compression:            CompressionGzip,
		Revision:               "1700000000",
		SetTimestampToRevision: true,
		UniqueMDFilenames:      true,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	md, err := ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatalf("ParseRepomdFile() error = %v", err)
	}
	if md.Revision != "1700000000" {
		t.Fatalf("Revision = %q", md.Revision)
	}
	for _, typ := range []string{"primary", "filelists", "other"} {
		record := md.Record(typ)
		if record == nil {
			t.Fatalf("missing %s record", typ)
		}
		if !strings.HasPrefix(filepath.Base(record.LocationHref), record.Checksum.Value+"-") {
			t.Fatalf("%s href is not checksum-prefixed: %s", typ, record.LocationHref)
		}
	}
}

func TestCreateNonEmptyRepositoryFromRPM(t *testing.T) {
	dir := t.TempDir()
	for _, rpmName := range []string{"super_kernel-6.0.1-2.x86_64.rpm"} {
		src := fixturePath(t, "tests", "testdata", "packages", rpmName)
		dst := filepath.Join(dir, rpmName)
		if err := copyFile(src, dst); err != nil {
			t.Fatal(err)
		}
	}
	if err := Create(context.Background(), Options{
		Directory:   dir,
		Checksum:    ChecksumSHA256,
		Compression: CompressionGzip,
		Revision:    "1700000003",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	repo, err := LoadMetadata(context.Background(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata() error = %v", err)
	}
	if len(repo.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(repo.Packages))
	}
	if repo.Packages[0].Name != "super_kernel" {
		t.Fatalf("package name = %q", repo.Packages[0].Name)
	}
}

func TestCreateFiltersAndBaseURL(t *testing.T) {
	dir := t.TempDir()
	for _, rpmName := range []string{"super_kernel-6.0.1-2.x86_64.rpm", "Archer-3.4.5-6.x86_64.rpm"} {
		src := fixturePath(t, "tests", "testdata", "packages", rpmName)
		dst := filepath.Join(dir, rpmName)
		if err := copyFile(src, dst); err != nil {
			t.Fatal(err)
		}
	}
	if err := Create(context.Background(), Options{
		Directory:       dir,
		Checksum:        ChecksumSHA256,
		Compression:     CompressionGzip,
		Revision:        "1700000010",
		Excludes:        []string{"Archer*"},
		BaseURL:         "https://example.test/repo",
		GroupFile:       fixturePath(t, "tests", "testdata", "comps_files", "comps_00.xml"),
		ChangelogLimit:  1,
		IncludePackages: []string{"*.rpm"},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	repo, err := LoadMetadata(context.Background(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata() error = %v", err)
	}
	if len(repo.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(repo.Packages))
	}
	if repo.Packages[0].Name != "super_kernel" {
		t.Fatalf("package name = %q", repo.Packages[0].Name)
	}
	if repo.Packages[0].LocationBase != "https://example.test/repo" {
		t.Fatalf("LocationBase = %q", repo.Packages[0].LocationBase)
	}
	if len(repo.Packages[0].Changelogs) > 1 {
		t.Fatalf("changelogs = %d, want at most 1", len(repo.Packages[0].Changelogs))
	}
	if repo.Repomd.Record("group") == nil {
		t.Fatal("missing group metadata record")
	}
}

func TestCreateRepomdTagsAndLocationOptions(t *testing.T) {
	work := t.TempDir()
	base := filepath.Join(work, "base")
	dir := filepath.Join(base, "repo", "Packages")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	rpmName := "super_kernel-6.0.1-2.x86_64.rpm"
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", rpmName), filepath.Join(dir, rpmName)); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:      filepath.Join(base, "repo"),
		Checksum:       ChecksumSHA256,
		Compression:    CompressionGzip,
		Revision:       "1700000013",
		BaseDir:        base,
		CutDirs:        1,
		LocationPrefix: "mirror",
		RepoTags:       []string{"repo-tag"},
		ContentTags:    []string{"content-tag"},
		DistroTags:     []DistroTag{{CPEID: "cpe:/o:test", Value: "Test Distro"}},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	repo, err := LoadMetadata(context.Background(), filepath.Join(base, "repo"), LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata() error = %v", err)
	}
	if len(repo.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(repo.Packages))
	}
	if repo.Packages[0].LocationHref != "mirror/Packages/"+rpmName {
		t.Fatalf("LocationHref = %q", repo.Packages[0].LocationHref)
	}
	if got := repo.Repomd.RepoTags; len(got) != 1 || got[0] != "repo-tag" {
		t.Fatalf("RepoTags = %#v", got)
	}
	if got := repo.Repomd.ContentTags; len(got) != 1 || got[0] != "content-tag" {
		t.Fatalf("ContentTags = %#v", got)
	}
	if got := repo.Repomd.DistroTags; len(got) != 1 || got[0].CPEID != "cpe:/o:test" || got[0].Value != "Test Distro" {
		t.Fatalf("DistroTags = %#v", got)
	}
}

func TestCreateSkipSymlinks(t *testing.T) {
	dir := t.TempDir()
	rpmName := "super_kernel-6.0.1-2.x86_64.rpm"
	realRPM := filepath.Join(dir, rpmName)
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", rpmName), realRPM); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realRPM, filepath.Join(dir, "linked.rpm")); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:    dir,
		Checksum:     ChecksumSHA256,
		Compression:  CompressionGzip,
		Revision:     "1700000014",
		SkipSymlinks: true,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	repo, err := LoadMetadata(context.Background(), dir, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(repo.Packages))
	}
}

func TestCreateDuplicateNEVRAPolicy(t *testing.T) {
	work := t.TempDir()
	src := fixturePath(t, "tests", "testdata", "packages", "super_kernel-6.0.1-2.x86_64.rpm")
	for _, tc := range []struct {
		name   string
		policy string
		want   int
	}{
		{name: "default", policy: "", want: 1},
		{name: "keep", policy: "keep", want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(work, tc.name)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := copyFile(src, filepath.Join(dir, "one.rpm")); err != nil {
				t.Fatal(err)
			}
			if err := copyFile(src, filepath.Join(dir, "two.rpm")); err != nil {
				t.Fatal(err)
			}
			if err := Create(context.Background(), Options{
				Directory:       dir,
				Checksum:        ChecksumSHA256,
				Compression:     CompressionGzip,
				Revision:        "1700000016",
				DuplicatedNEVRA: tc.policy,
			}); err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			repo, err := LoadMetadata(context.Background(), dir, LoadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(repo.Packages) != tc.want {
				t.Fatalf("len(Packages) = %d, want %d", len(repo.Packages), tc.want)
			}
		})
	}
}

func TestCreateSeparateRepomdChecksum(t *testing.T) {
	dir := t.TempDir()
	rpmName := "super_kernel-6.0.1-2.x86_64.rpm"
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", rpmName), filepath.Join(dir, rpmName)); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:      dir,
		Checksum:       ChecksumSHA256,
		RepomdChecksum: ChecksumSHA1,
		Compression:    CompressionGzip,
		Revision:       "1700000017",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	repo, err := LoadMetadata(context.Background(), dir, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.Repomd.Record("primary").Checksum.Type != "sha1" {
		t.Fatalf("repomd primary checksum type = %q", repo.Repomd.Record("primary").Checksum.Type)
	}
	if repo.Packages[0].ChecksumType != "sha256" {
		t.Fatalf("package checksum type = %q", repo.Packages[0].ChecksumType)
	}
}

func TestCreateUpdateMDPathPreservesAdditionalMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := Create(context.Background(), Options{
		Directory:               dir,
		Checksum:                ChecksumSHA256,
		Compression:             CompressionGzip,
		Revision:                "1700000015",
		AdditionalMetadataPaths: []string{fixturePath(t, "tests", "testdata", "repo_with_additional_metadata")},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	repo, err := LoadMetadata(context.Background(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata() error = %v", err)
	}
	if repo.UpdateInfo == nil || len(repo.UpdateInfo.Updates) != 1 {
		t.Fatalf("UpdateInfo = %#v", repo.UpdateInfo)
	}
	if repo.Repomd.Record("modules") == nil {
		t.Fatal("missing modules metadata record")
	}
}

func TestCreateZchunkRepository(t *testing.T) {
	dir := t.TempDir()
	rpmName := "super_kernel-6.0.1-2.x86_64.rpm"
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", rpmName), filepath.Join(dir, rpmName)); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:   dir,
		Checksum:    ChecksumSHA256,
		Compression: CompressionZchunk,
		Revision:    "1700000006",
	}); err != nil {
		t.Fatalf("Create(zchunk) error = %v", err)
	}
	md, err := ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	primary := md.Record("primary")
	if primary == nil {
		t.Fatal("missing primary record")
	}
	if !strings.HasSuffix(primary.LocationHref, ".xml.zck") {
		t.Fatalf("primary href = %q, want .xml.zck", primary.LocationHref)
	}
	if primary.HeaderChecksum.Value == "" || primary.HeaderSize <= 0 {
		t.Fatalf("missing zchunk header stats: %#v size %d", primary.HeaderChecksum, primary.HeaderSize)
	}
	repo, err := LoadMetadata(context.Background(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata(zchunk) error = %v", err)
	}
	if len(repo.Packages) != 1 || repo.Packages[0].Name != "super_kernel" {
		t.Fatalf("loaded packages = %#v", repo.Packages)
	}
}

func TestCreateFilelistsExtRepository(t *testing.T) {
	dir := t.TempDir()
	rpmName := "super_kernel-6.0.1-2.x86_64.rpm"
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", rpmName), filepath.Join(dir, rpmName)); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:    dir,
		Checksum:     ChecksumSHA256,
		Compression:  CompressionGzip,
		Revision:     "1700000009",
		FilelistsExt: true,
	}); err != nil {
		t.Fatalf("Create(filelists-ext) error = %v", err)
	}
	md, err := ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if md.Record("filelists-ext") == nil {
		t.Fatal("missing filelists-ext record")
	}
}

func TestModifyRemoveRecord(t *testing.T) {
	dir := t.TempDir()
	if err := Create(context.Background(), Options{
		Directory:   dir,
		Checksum:    ChecksumSHA256,
		Compression: CompressionGzip,
		Revision:    "1700000001",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := Modify(context.Background(), ModifyOptions{
		RepodataDir: filepath.Join(dir, "repodata"),
		RemoveType:  "other",
	}); err != nil {
		t.Fatalf("Modify(remove) error = %v", err)
	}

	md, err := ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if md.Record("other") != nil {
		t.Fatal("other record was not removed")
	}
}

func TestModifyAddRecord(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "modules.yaml")
	if err := os.WriteFile(src, []byte("document: modulemd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:   dir,
		OutputDir:   filepath.Join(dir, "repo"),
		Checksum:    ChecksumSHA256,
		Compression: CompressionGzip,
		Revision:    "1700000002",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := Modify(context.Background(), ModifyOptions{
		RepodataDir:       filepath.Join(dir, "repo", "repodata"),
		MetadataPath:      src,
		MetadataType:      "modules",
		Checksum:          ChecksumSHA256,
		UniqueMDFilenames: true,
	}); err != nil {
		t.Fatalf("Modify(add) error = %v", err)
	}
	md, err := ParseRepomdFile(filepath.Join(dir, "repo", "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if md.Record("modules") == nil {
		t.Fatal("modules record was not added")
	}
}

func TestModifyBatchFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "modules.yaml")
	if err := os.WriteFile(src, []byte("document: modulemd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:   dir,
		OutputDir:   filepath.Join(dir, "repo"),
		Checksum:    ChecksumSHA256,
		Compression: CompressionGzip,
		Revision:    "1700000011",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	batch := filepath.Join(dir, "batch.ini")
	if err := os.WriteFile(batch, []byte("[modules]\npath=modules.yaml\ntype=modules\ncompress=false\nunique-md-filenames=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Modify(context.Background(), ModifyOptions{
		RepodataDir: filepath.Join(dir, "repo", "repodata"),
		BatchFile:   batch,
		Checksum:    ChecksumSHA256,
	}); err != nil {
		t.Fatalf("Modify(batch) error = %v", err)
	}
	md, err := ParseRepomdFile(filepath.Join(dir, "repo", "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if md.Record("modules") == nil {
		t.Fatal("modules record was not added")
	}
}

func TestModifyAddCompressedZchunkRecord(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "modules.yaml")
	if err := os.WriteFile(src, []byte("document: modulemd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:   dir,
		OutputDir:   filepath.Join(dir, "repo"),
		Checksum:    ChecksumSHA256,
		Compression: CompressionGzip,
		Revision:    "1700000008",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := Modify(context.Background(), ModifyOptions{
		RepodataDir:       filepath.Join(dir, "repo", "repodata"),
		MetadataPath:      src,
		MetadataType:      "modules_zck",
		Checksum:          ChecksumSHA256,
		Compression:       CompressionZchunk,
		Compress:          true,
		UniqueMDFilenames: true,
	}); err != nil {
		t.Fatalf("Modify(add zchunk) error = %v", err)
	}
	md, err := ParseRepomdFile(filepath.Join(dir, "repo", "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	record := md.Record("modules_zck")
	if record == nil {
		t.Fatal("modules_zck record was not added")
	}
	if !strings.HasSuffix(record.LocationHref, ".zck") {
		t.Fatalf("modules_zck href = %q", record.LocationHref)
	}
	if record.HeaderChecksum.Value == "" || record.HeaderSize <= 0 {
		t.Fatalf("missing zchunk header stats: %#v size %d", record.HeaderChecksum, record.HeaderSize)
	}
}
