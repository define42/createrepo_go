package createrepo

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestGenerateSQLite(t *testing.T) {
	dir := t.TempDir()
	rpmName := "super_kernel-6.0.1-2.x86_64.rpm"
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", rpmName), filepath.Join(dir, rpmName)); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:   dir,
		Checksum:    ChecksumSHA256,
		Compression: CompressionGzip,
		Revision:    "1700000004",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := GenerateSQLite(context.Background(), SQLiteOptions{
		RepositoryPath: dir,
		Checksum:       ChecksumSHA256,
		Compression:    CompressionNone,
	}); err != nil {
		t.Fatalf("GenerateSQLite() error = %v", err)
	}

	md, err := ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, recordType := range []string{"primary_db", "filelists_db", "other_db"} {
		if md.Record(recordType) == nil {
			t.Fatalf("missing %s record", recordType)
		}
	}

	primaryDB := md.Record("primary_db")
	db, err := sql.Open("sqlite", filepath.Join(dir, filepath.FromSlash(primaryDB.LocationHref)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var packageCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM packages`).Scan(&packageCount); err != nil {
		t.Fatal(err)
	}
	if packageCount != 1 {
		t.Fatalf("packageCount = %d, want 1", packageCount)
	}
	var dbVersion int
	if err := db.QueryRow(`SELECT dbversion FROM db_info`).Scan(&dbVersion); err != nil {
		t.Fatal(err)
	}
	if dbVersion != databaseVersion {
		t.Fatalf("dbversion = %d, want %d", dbVersion, databaseVersion)
	}
}

func TestGenerateSQLiteEmptyRepository(t *testing.T) {
	dir := t.TempDir()
	if err := Create(context.Background(), Options{
		Directory:   dir,
		Checksum:    ChecksumSHA256,
		Compression: CompressionGzip,
		Revision:    "1700000012",
	}); err != nil {
		t.Fatalf("Create(empty) error = %v", err)
	}
	if err := GenerateSQLite(context.Background(), SQLiteOptions{
		RepositoryPath: dir,
		Checksum:       ChecksumSHA256,
		Compression:    CompressionNone,
	}); err != nil {
		t.Fatalf("GenerateSQLite(empty) error = %v", err)
	}

	md, err := ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	primaryDB := md.Record("primary_db")
	if primaryDB == nil {
		t.Fatal("missing primary_db record")
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, filepath.FromSlash(primaryDB.LocationHref)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var packageCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM packages`).Scan(&packageCount); err != nil {
		t.Fatal(err)
	}
	if packageCount != 0 {
		t.Fatalf("packageCount = %d, want 0", packageCount)
	}
}

func TestCreateWithDatabaseOption(t *testing.T) {
	dir := t.TempDir()
	rpmName := "super_kernel-6.0.1-2.x86_64.rpm"
	if err := copyFile(fixturePath(t, "tests", "testdata", "packages", rpmName), filepath.Join(dir, rpmName)); err != nil {
		t.Fatal(err)
	}
	if err := Create(context.Background(), Options{
		Directory:   dir,
		Checksum:    ChecksumSHA256,
		Compression: CompressionGzip,
		Revision:    "1700000007",
		Database:    true,
	}); err != nil {
		t.Fatalf("Create(database) error = %v", err)
	}
	md, err := ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, recordType := range []string{"primary_db", "filelists_db", "other_db"} {
		if md.Record(recordType) == nil {
			t.Fatalf("missing %s record", recordType)
		}
	}
}
