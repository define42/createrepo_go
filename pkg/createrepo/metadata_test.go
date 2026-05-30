package createrepo

import (
	"path/filepath"
	"testing"
)

func TestParseRepomdLocation(t *testing.T) {
	repoPath := fixturePath(t, "tests", "testdata", "repo_00")
	location, err := ParseRepomdLocation(filepath.Join(repoPath, "repodata", "repomd.xml"), repoPath, true)
	if err != nil {
		t.Fatalf("ParseRepomdLocation() error = %v", err)
	}

	if location.PrimaryXMLHref != filepath.Join(repoPath, "repodata", "1cb61ea996355add02b1426ed4c1780ea75ce0c04c5d1107c025c3fbd7d8bcae-primary.xml.gz") {
		t.Fatalf("PrimaryXMLHref = %q", location.PrimaryXMLHref)
	}
	if location.FilelistsXMLHref == "" || location.OtherXMLHref == "" {
		t.Fatalf("core metadata paths not populated: %#v", location)
	}
	if len(location.AdditionalMetadata) != 0 {
		t.Fatalf("AdditionalMetadata len = %d, want 0", len(location.AdditionalMetadata))
	}
}

func TestParseRepomdLocationAdditionalMetadata(t *testing.T) {
	repoPath := fixturePath(t, "tests", "testdata", "repo_with_additional_metadata")
	location, err := ParseRepomdLocation(filepath.Join(repoPath, "repodata", "repomd.xml"), repoPath, false)
	if err != nil {
		t.Fatalf("ParseRepomdLocation() error = %v", err)
	}

	if location.PrimarySQLiteHref == "" || location.FilelistsSQLiteHref == "" || location.OtherSQLiteHref == "" {
		t.Fatalf("sqlite metadata paths not populated: %#v", location)
	}
	if len(location.AdditionalMetadata) != 8 {
		t.Fatalf("AdditionalMetadata len = %d, want 8", len(location.AdditionalMetadata))
	}

	seen := map[string]bool{}
	for _, item := range location.AdditionalMetadata {
		seen[item.Type] = true
		if item.Name == "" {
			t.Fatalf("empty metadata name for %s", item.Type)
		}
	}
	for _, recordType := range []string{"group", "group_zck", "group_gz", "group_gz_zck", "modules", "modules_zck", "updateinfo", "updateinfo_zck"} {
		if !seen[recordType] {
			t.Fatalf("missing additional metadata type %s", recordType)
		}
	}
}

func TestInsertAdditionalMetadatum(t *testing.T) {
	metadata := InsertAdditionalMetadatum(nil, "first.xml", "group")
	metadata = InsertAdditionalMetadatum(metadata, "second.xml", "group")
	metadata = InsertAdditionalMetadatum(metadata, "modules.yaml", "modules")

	if len(metadata) != 2 {
		t.Fatalf("len(metadata) = %d, want 2", len(metadata))
	}
	for _, item := range metadata {
		if item.Type == "group" && item.Name != "second.xml" {
			t.Fatalf("group item = %#v", item)
		}
	}
}
