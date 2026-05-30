package createrepo

import (
	"os"
	"path/filepath"
	"strings"
)

// Metadatum is an additional metadata record discovered in repomd.xml.
type Metadatum struct {
	Name string
	Type string
}

// MetadataLocation groups important repository metadata paths.
type MetadataLocation struct {
	RepomdPath             string
	LocalPath              string
	Repomd                 *Repomd
	PrimaryXMLHref         string
	FilelistsXMLHref       string
	FilelistsExtHref       string
	OtherXMLHref           string
	PrimarySQLiteHref      string
	FilelistsSQLiteHref    string
	FilelistsExtSQLiteHref string
	OtherSQLiteHref        string
	AdditionalMetadata     []Metadatum
}

// InsertAdditionalMetadatum inserts or replaces an additional metadata entry by
// type.
func InsertAdditionalMetadatum(metadata []Metadatum, path, recordType string) []Metadatum {
	for i := range metadata {
		if metadata[i].Type == recordType {
			metadata[i].Name = path
			return metadata
		}
	}
	return append([]Metadatum{{Name: path, Type: recordType}}, metadata...)
}

// ParseRepomdLocation parses repomd.xml and resolves record hrefs relative to
// repoPath.
func ParseRepomdLocation(repomdPath, repoPath string, ignoreSQLite bool) (*MetadataLocation, error) {
	repomd, err := ParseRepomdFile(repomdPath)
	if err != nil {
		return nil, err
	}

	location := &MetadataLocation{
		RepomdPath: repomdPath,
		LocalPath:  repoPath,
		Repomd:     repomd,
	}

	for _, record := range repomd.Records {
		fullHref := filepath.Join(repoPath, filepath.FromSlash(record.LocationHref))
		switch record.Type {
		case "primary":
			location.PrimaryXMLHref = fullHref
		case "primary_db":
			if !ignoreSQLite {
				location.PrimarySQLiteHref = fullHref
			}
		case "filelists":
			location.FilelistsXMLHref = fullHref
		case "filelists_db":
			if !ignoreSQLite {
				location.FilelistsSQLiteHref = fullHref
			}
		case "filelists-ext":
			location.FilelistsExtHref = fullHref
		case "filelists-ext_db":
			if !ignoreSQLite {
				location.FilelistsExtSQLiteHref = fullHref
			}
		case "other":
			location.OtherXMLHref = fullHref
		case "other_db":
			if !ignoreSQLite {
				location.OtherSQLiteHref = fullHref
			}
		default:
			if isAdditionalMetadataType(record.Type) {
				location.AdditionalMetadata = InsertAdditionalMetadatum(location.AdditionalMetadata, fullHref, record.Type)
			}
		}
	}

	return location, nil
}

// LocalMetadata locates and parses a local repository's repodata/repomd.xml.
func LocalMetadata(repoPath string, ignoreSQLite bool) (*MetadataLocation, error) {
	info, err := os.Stat(repoPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "stat", Path: repoPath, Err: os.ErrInvalid}
	}

	repomdPath := filepath.Join(repoPath, "repodata", "repomd.xml")
	return ParseRepomdLocation(repomdPath, repoPath, ignoreSQLite)
}

func isAdditionalMetadataType(recordType string) bool {
	return !strings.HasPrefix(recordType, "primary_") &&
		!strings.HasPrefix(recordType, "filelists_") &&
		!strings.HasPrefix(recordType, "filelists-ext_") &&
		!strings.HasPrefix(recordType, "other_")
}
