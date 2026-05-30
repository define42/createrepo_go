package createrepo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRepomdFile(t *testing.T) {
	md, err := ParseRepomdFile(fixturePath(t, "tests", "testdata", "repo_00", "repodata", "repomd.xml"))
	if err != nil {
		t.Fatalf("ParseRepomdFile() error = %v", err)
	}
	if md.Revision != "1533242352" {
		t.Fatalf("Revision = %q", md.Revision)
	}
	if len(md.Records) != 6 {
		t.Fatalf("len(Records) = %d, want 6", len(md.Records))
	}

	primary := md.Record("primary")
	if primary == nil {
		t.Fatal("missing primary record")
	}
	if primary.LocationHref != "repodata/1cb61ea996355add02b1426ed4c1780ea75ce0c04c5d1107c025c3fbd7d8bcae-primary.xml.gz" {
		t.Fatalf("primary href = %q", primary.LocationHref)
	}
	if primary.Checksum.Type != "sha256" || primary.Checksum.Value != "1cb61ea996355add02b1426ed4c1780ea75ce0c04c5d1107c025c3fbd7d8bcae" {
		t.Fatalf("primary checksum = %#v", primary.Checksum)
	}
	if primary.OpenSize != 167 || primary.Size != 134 || primary.Timestamp != 1533242352 {
		t.Fatalf("primary stats = size:%d open:%d ts:%d", primary.Size, primary.OpenSize, primary.Timestamp)
	}

	primaryZchunk := md.Record("primary_zck")
	if primaryZchunk == nil {
		t.Fatal("missing primary_zck record")
	}
	if primaryZchunk.HeaderChecksum.Value != "243baf7c02f5241d46f2e8c237ebc7ea7e257ca993d9cfe1304254c7ba7f6546" {
		t.Fatalf("primary_zck header checksum = %#v", primaryZchunk.HeaderChecksum)
	}
	if primaryZchunk.HeaderSize != 132 {
		t.Fatalf("primary_zck header size = %d", primaryZchunk.HeaderSize)
	}
}

func TestRepomdRecordFill(t *testing.T) {
	record := NewRepomdRecord("primary", fixturePath(t, "tests", "testdata", "repo_00", "repodata", "1cb61ea996355add02b1426ed4c1780ea75ce0c04c5d1107c025c3fbd7d8bcae-primary.xml.gz"))
	if err := record.Fill(ChecksumSHA256); err != nil {
		t.Fatalf("Fill() error = %v", err)
	}
	if record.Checksum.Value != "1cb61ea996355add02b1426ed4c1780ea75ce0c04c5d1107c025c3fbd7d8bcae" {
		t.Fatalf("Checksum = %s", record.Checksum.Value)
	}
	if record.OpenChecksum.Value != "e1e2ffd2fb1ee76f87b70750d00ca5677a252b397ab6c2389137a0c33e7b359f" {
		t.Fatalf("OpenChecksum = %s", record.OpenChecksum.Value)
	}
	if record.Size != 134 || record.OpenSize != 167 {
		t.Fatalf("Size/OpenSize = %d/%d", record.Size, record.OpenSize)
	}
}

func TestRepomdRecordRenameFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "primary.xml")
	if err := os.WriteFile(path, []byte("<metadata/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	record := NewRepomdRecord("primary", path)
	if err := record.Fill(ChecksumSHA256); err != nil {
		t.Fatalf("Fill() error = %v", err)
	}
	if err := record.RenameFile(); err != nil {
		t.Fatalf("RenameFile() error = %v", err)
	}

	if _, err := os.Stat(record.LocationReal); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if filepath.Base(record.LocationReal) != record.Checksum.Value+"-primary.xml" {
		t.Fatalf("renamed base = %q", filepath.Base(record.LocationReal))
	}
	if record.LocationHref != "repodata/"+record.Checksum.Value+"-primary.xml" {
		t.Fatalf("LocationHref = %q", record.LocationHref)
	}
}
