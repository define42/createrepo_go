package createrepo

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const databaseVersion = 10

// GenerateSQLite creates yum sqlite metadata databases and records them in repomd.xml.
func GenerateSQLite(ctx context.Context, opts SQLiteOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.RepositoryPath == "" {
		return fmt.Errorf("repository path is required")
	}
	if opts.Checksum == ChecksumUnknown {
		opts.Checksum = ChecksumSHA256
	}
	if opts.Compression == CompressionAuto || opts.Compression == CompressionUnknown {
		opts.Compression = CompressionBzip2
	}

	repo, err := LoadMetadata(ctx, opts.RepositoryPath, LoadOptions{IgnoreSQLite: true})
	if err != nil {
		return err
	}
	repodataDir := filepath.Join(opts.RepositoryPath, "repodata")

	dbs := []struct {
		typ       string
		filename  string
		build     func(string, []Package, string) error
		xmlRecord string
	}{
		{"primary_db", "primary.sqlite", buildPrimarySQLite, "primary"},
		{"filelists_db", "filelists.sqlite", buildFilelistsSQLite, "filelists"},
		{"other_db", "other.sqlite", buildOtherSQLite, "other"},
	}
	for _, item := range dbs {
		xmlRecord := repo.Repomd.Record(item.xmlRecord)
		if xmlRecord == nil {
			return fmt.Errorf("missing %s metadata record", item.xmlRecord)
		}
		dbPath := filepath.Join(repodataDir, item.filename)
		if err := item.build(dbPath, repo.Packages, xmlRecord.Checksum.Value); err != nil {
			return err
		}
		finalPath := dbPath
		if opts.Compression != CompressionNone {
			data, err := os.ReadFile(dbPath)
			if err != nil {
				return err
			}
			finalPath = dbPath + opts.Compression.Suffix()
			if err := WriteCompressedFile(finalPath, data, opts.Compression); err != nil {
				return err
			}
			if !opts.KeepOld {
				_ = os.Remove(dbPath)
			}
		}
		record := NewRepomdRecord(item.typ, finalPath)
		record.DatabaseVersion = databaseVersion
		if err := record.Fill(opts.Checksum); err != nil {
			return err
		}
		if err := record.RenameFile(); err != nil {
			return err
		}
		upsertRepomdRecord(repo.Repomd, record)
	}

	return os.WriteFile(filepath.Join(repodataDir, "repomd.xml"), []byte(DumpRepomd(repo.Repomd)), 0o644)
}

func buildPrimarySQLite(path string, packages []Package, checksum string) error {
	_ = os.Remove(path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := execMany(db, primarySchema); err != nil {
		return err
	}
	for i := range packages {
		pkgKey := int64(i + 1)
		pkg := packages[i]
		_, err := db.Exec(`INSERT INTO packages (
pkgKey, pkgId, name, arch, version, epoch, release, summary, description, url,
time_file, time_build, rpm_license, rpm_vendor, rpm_group, rpm_buildhost,
rpm_sourcerpm, rpm_header_start, rpm_header_end, rpm_packager, size_package,
size_installed, size_archive, location_href, location_base, checksum_type)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pkgKey, pkg.PkgID, pkg.Name, pkg.Arch, pkg.Version, pkg.Epoch, pkg.Release, pkg.Summary,
			pkg.Description, pkg.URL, pkg.TimeFile, pkg.TimeBuild, pkg.RPMLicense, pkg.RPMVendor,
			pkg.RPMGroup, pkg.RPMBuildHost, pkg.RPMSourcerpm, pkg.RPMHeaderStart, pkg.RPMHeaderEnd,
			pkg.RPMPackager, pkg.SizePackage, pkg.SizeInstalled, pkg.SizeArchive,
			nullEmpty(pkg.LocationHref), nullEmpty(pkg.LocationBase), pkg.ChecksumType)
		if err != nil {
			return err
		}
		for _, file := range pkg.Files {
			if file.Type == "dir" {
				continue
			}
			fileType := file.Type
			if fileType == "" {
				fileType = "file"
			}
			if _, err := db.Exec(`INSERT INTO files (name, type, pkgKey) VALUES (?, ?, ?)`, fileFullPath(file), fileType, pkgKey); err != nil {
				return err
			}
		}
		for table, deps := range map[string][]Dependency{
			"requires":    pkg.Requires,
			"provides":    pkg.Provides,
			"conflicts":   pkg.Conflicts,
			"obsoletes":   pkg.Obsoletes,
			"suggests":    pkg.Suggests,
			"enhances":    pkg.Enhances,
			"recommends":  pkg.Recommends,
			"supplements": pkg.Supplements,
		} {
			for _, dep := range deps {
				if table == "requires" {
					_, err = db.Exec(`INSERT INTO requires (name, flags, epoch, version, release, pkgKey, pre) VALUES (?, ?, ?, ?, ?, ?, ?)`,
						dep.Name, nullEmpty(dep.Flags), nullEmpty(dep.Epoch), nullEmpty(dep.Version), nullEmpty(dep.Release), pkgKey, boolString(dep.Pre))
				} else {
					_, err = db.Exec(fmt.Sprintf(`INSERT INTO %s (name, flags, epoch, version, release, pkgKey) VALUES (?, ?, ?, ?, ?, ?)`, table),
						dep.Name, nullEmpty(dep.Flags), nullEmpty(dep.Epoch), nullEmpty(dep.Version), nullEmpty(dep.Release), pkgKey)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return dbInfoUpdate(db, checksum)
}

func buildFilelistsSQLite(path string, packages []Package, checksum string) error {
	_ = os.Remove(path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := execMany(db, filelistsSchema); err != nil {
		return err
	}
	for i, pkg := range packages {
		pkgKey := int64(i + 1)
		if _, err := db.Exec(`INSERT INTO packages (pkgKey, pkgId) VALUES (?, ?)`, pkgKey, pkg.PkgID); err != nil {
			return err
		}
		for dirname, files := range groupFilesForSQLite(pkg.Files) {
			names, types := encodeSQLiteFileGroup(files)
			if _, err := db.Exec(`INSERT INTO filelist (pkgKey, dirname, filenames, filetypes) VALUES (?, ?, ?, ?)`, pkgKey, dirname, names, types); err != nil {
				return err
			}
		}
	}
	return dbInfoUpdate(db, checksum)
}

func buildOtherSQLite(path string, packages []Package, checksum string) error {
	_ = os.Remove(path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := execMany(db, otherSchema); err != nil {
		return err
	}
	for i, pkg := range packages {
		pkgKey := int64(i + 1)
		if _, err := db.Exec(`INSERT INTO packages (pkgKey, pkgId) VALUES (?, ?)`, pkgKey, pkg.PkgID); err != nil {
			return err
		}
		for _, ch := range pkg.Changelogs {
			if _, err := db.Exec(`INSERT INTO changelog (pkgKey, author, date, changelog) VALUES (?, ?, ?, ?)`, pkgKey, ch.Author, ch.Date, ch.Changelog); err != nil {
				return err
			}
		}
	}
	return dbInfoUpdate(db, checksum)
}

func execMany(db *sql.DB, stmts []string) error {
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

func dbInfoUpdate(db *sql.DB, checksum string) error {
	_, err := db.Exec(`INSERT INTO db_info (dbversion, checksum) VALUES (?, ?)`, databaseVersion, checksum)
	return err
}

func upsertRepomdRecord(md *Repomd, record *RepomdRecord) {
	for i, existing := range md.Records {
		if existing.Type == record.Type {
			md.Records[i] = record
			return
		}
	}
	md.Records = append(md.Records, record)
}

func nullEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func boolString(value bool) string {
	if value {
		return "TRUE"
	}
	return "FALSE"
}

func groupFilesForSQLite(files []PackageFile) map[string][]PackageFile {
	grouped := map[string][]PackageFile{}
	for _, file := range files {
		dirname := file.Path
		if dirname == "" {
			dirname = "/"
		}
		grouped[dirname] = append(grouped[dirname], file)
	}
	return grouped
}

func encodeSQLiteFileGroup(files []PackageFile) (string, string) {
	names := make([]string, 0, len(files))
	types := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
		switch file.Type {
		case "dir":
			types = append(types, "d")
		case "ghost":
			types = append(types, "g")
		default:
			types = append(types, "f")
		}
	}
	return strings.Join(names, "/"), strings.Join(types, "")
}

var primarySchema = []string{
	`CREATE TABLE db_info (dbversion INTEGER, checksum TEXT)`,
	`CREATE TABLE packages (pkgKey INTEGER PRIMARY KEY, pkgId TEXT, name TEXT, arch TEXT, version TEXT, epoch TEXT, release TEXT, summary TEXT, description TEXT, url TEXT, time_file INTEGER, time_build INTEGER, rpm_license TEXT, rpm_vendor TEXT, rpm_group TEXT, rpm_buildhost TEXT, rpm_sourcerpm TEXT, rpm_header_start INTEGER, rpm_header_end INTEGER, rpm_packager TEXT, size_package INTEGER, size_installed INTEGER, size_archive INTEGER, location_href TEXT, location_base TEXT, checksum_type TEXT)`,
	`CREATE TABLE files (name TEXT, type TEXT, pkgKey INTEGER)`,
	`CREATE TABLE requires (name TEXT, flags TEXT, epoch TEXT, version TEXT, release TEXT, pkgKey INTEGER, pre BOOLEAN DEFAULT FALSE)`,
	`CREATE TABLE provides (name TEXT, flags TEXT, epoch TEXT, version TEXT, release TEXT, pkgKey INTEGER)`,
	`CREATE TABLE conflicts (name TEXT, flags TEXT, epoch TEXT, version TEXT, release TEXT, pkgKey INTEGER)`,
	`CREATE TABLE obsoletes (name TEXT, flags TEXT, epoch TEXT, version TEXT, release TEXT, pkgKey INTEGER)`,
	`CREATE TABLE suggests (name TEXT, flags TEXT, epoch TEXT, version TEXT, release TEXT, pkgKey INTEGER)`,
	`CREATE TABLE enhances (name TEXT, flags TEXT, epoch TEXT, version TEXT, release TEXT, pkgKey INTEGER)`,
	`CREATE TABLE recommends (name TEXT, flags TEXT, epoch TEXT, version TEXT, release TEXT, pkgKey INTEGER)`,
	`CREATE TABLE supplements (name TEXT, flags TEXT, epoch TEXT, version TEXT, release TEXT, pkgKey INTEGER)`,
	`CREATE TRIGGER removals AFTER DELETE ON packages BEGIN DELETE FROM files WHERE pkgKey = old.pkgKey; DELETE FROM requires WHERE pkgKey = old.pkgKey; DELETE FROM provides WHERE pkgKey = old.pkgKey; DELETE FROM conflicts WHERE pkgKey = old.pkgKey; DELETE FROM obsoletes WHERE pkgKey = old.pkgKey; DELETE FROM suggests WHERE pkgKey = old.pkgKey; DELETE FROM enhances WHERE pkgKey = old.pkgKey; DELETE FROM recommends WHERE pkgKey = old.pkgKey; DELETE FROM supplements WHERE pkgKey = old.pkgKey; END;`,
	`CREATE INDEX packagename ON packages (name)`,
	`CREATE INDEX packageId ON packages (pkgId)`,
	`CREATE INDEX filenames ON files (name)`,
	`CREATE INDEX pkgfiles ON files (pkgKey)`,
	`CREATE INDEX pkgrequires ON requires (pkgKey)`,
	`CREATE INDEX requiresname ON requires (name)`,
	`CREATE INDEX pkgprovides ON provides (pkgKey)`,
	`CREATE INDEX providesname ON provides (name)`,
	`CREATE INDEX pkgconflicts ON conflicts (pkgKey)`,
	`CREATE INDEX pkgobsoletes ON obsoletes (pkgKey)`,
	`CREATE INDEX pkgsuggests ON suggests (pkgKey)`,
	`CREATE INDEX pkgenhances ON enhances (pkgKey)`,
	`CREATE INDEX pkgrecommends ON recommends (pkgKey)`,
	`CREATE INDEX pkgsupplements ON supplements (pkgKey)`,
}

var filelistsSchema = []string{
	`CREATE TABLE db_info (dbversion INTEGER, checksum TEXT)`,
	`CREATE TABLE packages (pkgKey INTEGER PRIMARY KEY, pkgId TEXT)`,
	`CREATE TABLE filelist (pkgKey INTEGER, dirname TEXT, filenames TEXT, filetypes TEXT)`,
	`CREATE TRIGGER remove_filelist AFTER DELETE ON packages BEGIN DELETE FROM filelist WHERE pkgKey = old.pkgKey; END;`,
	`CREATE INDEX keyfile ON filelist (pkgKey)`,
	`CREATE INDEX pkgId ON packages (pkgId)`,
	`CREATE INDEX dirnames ON filelist (dirname)`,
}

var otherSchema = []string{
	`CREATE TABLE db_info (dbversion INTEGER, checksum TEXT)`,
	`CREATE TABLE packages (pkgKey INTEGER PRIMARY KEY, pkgId TEXT)`,
	`CREATE TABLE changelog (pkgKey INTEGER, author TEXT, date INTEGER, changelog TEXT)`,
	`CREATE TRIGGER remove_changelogs AFTER DELETE ON packages BEGIN DELETE FROM changelog WHERE pkgKey = old.pkgKey; END;`,
	`CREATE INDEX keychange ON changelog (pkgKey)`,
	`CREATE INDEX pkgId ON packages (pkgId)`,
}
