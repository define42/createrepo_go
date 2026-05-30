package createrepo

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ParsePrimaryFile parses primary XML metadata into packages.
func ParsePrimaryFile(path string) ([]Package, error) {
	r, err := OpenReader(path, CompressionAuto)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ParsePrimary(r)
}

// ParseFilelistsFile parses filelists XML metadata and merges file entries into packages.
func ParseFilelistsFile(path string, packages []Package) ([]Package, error) {
	r, err := OpenReader(path, CompressionAuto)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return parseFilelistsLike(r, packages, false)
}

// ParseFilelistsExtFile parses filelists-ext XML metadata and merges file digests.
func ParseFilelistsExtFile(path string, packages []Package) ([]Package, error) {
	r, err := OpenReader(path, CompressionAuto)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return parseFilelistsLike(r, packages, true)
}

// ParseOtherFile parses other XML metadata and merges changelogs into packages.
func ParseOtherFile(path string, packages []Package) ([]Package, error) {
	r, err := OpenReader(path, CompressionAuto)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ParseOther(r, packages)
}

// ParsePrimary parses primary XML metadata into packages.
func ParsePrimary(r io.Reader) ([]Package, error) {
	decoder := xml.NewDecoder(r)
	var packages []Package
	var current *Package
	var depKind string
	var fileType string
	var collect string
	var text strings.Builder

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch token := tok.(type) {
		case xml.StartElement:
			local := token.Name.Local
			switch local {
			case "package":
				current = &Package{}
			case "version":
				if current != nil {
					current.Epoch = attrValue(token.Attr, "epoch")
					current.Version = attrValue(token.Attr, "ver")
					current.Release = attrValue(token.Attr, "rel")
				}
			case "checksum":
				if current != nil {
					current.ChecksumType = attrValue(token.Attr, "type")
				}
				startCollect(local, &collect, &text)
			case "time":
				if current != nil {
					current.TimeFile = attrInt64(token.Attr, "file")
					current.TimeBuild = attrInt64(token.Attr, "build")
				}
			case "size":
				if current != nil {
					current.SizePackage = attrInt64(token.Attr, "package")
					current.SizeInstalled = attrInt64(token.Attr, "installed")
					current.SizeArchive = attrInt64(token.Attr, "archive")
				}
			case "location":
				if current != nil {
					current.LocationHref = attrValue(token.Attr, "href")
					current.LocationBase = attrValue(token.Attr, "base")
				}
			case "header-range":
				if current != nil {
					current.RPMHeaderStart = attrInt64(token.Attr, "start")
					current.RPMHeaderEnd = attrInt64(token.Attr, "end")
				}
			case "provides", "requires", "conflicts", "obsoletes", "suggests", "enhances", "recommends", "supplements":
				depKind = local
			case "entry":
				if current != nil && depKind != "" {
					appendDependency(current, depKind, dependencyFromAttrs(token.Attr))
				}
			case "file":
				fileType = attrValue(token.Attr, "type")
				startCollect(local, &collect, &text)
			case "name", "arch", "summary", "description", "packager", "url", "license", "vendor", "group", "buildhost", "sourcerpm":
				startCollect(local, &collect, &text)
			}
		case xml.CharData:
			if collect != "" {
				text.Write([]byte(token))
			}
		case xml.EndElement:
			local := token.Name.Local
			if collect == local {
				value := strings.TrimSpace(text.String())
				if current != nil {
					applyPrimaryContent(current, local, value, fileType)
				}
				collect = ""
				fileType = ""
			}
			if local == depKind {
				depKind = ""
			}
			if local == "package" && current != nil {
				packages = append(packages, *current)
				current = nil
			}
		}
	}
	return packages, nil
}

// ParseFilelists parses filelists XML metadata and merges file entries into packages.
func ParseFilelists(r io.Reader, packages []Package) ([]Package, error) {
	return parseFilelistsLike(r, packages, false)
}

func parseFilelistsLike(r io.Reader, packages []Package, withDigest bool) ([]Package, error) {
	decoder := xml.NewDecoder(r)
	index := packageIndex(packages)
	var current *Package
	var fileType string
	var fileDigest string
	var collect string
	var text strings.Builder

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch token := tok.(type) {
		case xml.StartElement:
			switch token.Name.Local {
			case "package":
				key := attrValue(token.Attr, "pkgid")
				current = index[key]
				if current == nil {
					packages = append(packages, Package{
						PkgID: attrValue(token.Attr, "pkgid"),
						Name:  attrValue(token.Attr, "name"),
						Arch:  attrValue(token.Attr, "arch"),
					})
					current = &packages[len(packages)-1]
					index[current.PkgID] = current
				}
			case "version":
				if current != nil {
					current.Epoch = attrValue(token.Attr, "epoch")
					current.Version = attrValue(token.Attr, "ver")
					current.Release = attrValue(token.Attr, "rel")
				}
			case "file":
				fileType = attrValue(token.Attr, "type")
				fileDigest = attrValue(token.Attr, "hash")
				startCollect("file", &collect, &text)
			}
		case xml.CharData:
			if collect != "" {
				text.Write([]byte(token))
			}
		case xml.EndElement:
			if token.Name.Local == collect && current != nil {
				file := splitPackageFile(fileType, strings.TrimSpace(text.String()))
				file.Digest = fileDigest
				mergePackageFile(file, current, withDigest)
				collect = ""
				fileType = ""
				fileDigest = ""
			}
			if token.Name.Local == "package" {
				current = nil
			}
		}
	}
	return packages, nil
}

func mergePackageFile(file PackageFile, pkg *Package, withDigest bool) {
	fullPath := fileFullPath(file)
	for i := range pkg.Files {
		if fileFullPath(pkg.Files[i]) == fullPath {
			if pkg.Files[i].Type == "" && file.Type != "" {
				pkg.Files[i].Type = file.Type
			}
			if file.Digest != "" {
				pkg.Files[i].Digest = file.Digest
			}
			return
		}
	}
	if !withDigest {
		file.Digest = ""
	}
	pkg.Files = append(pkg.Files, file)
}

// ParseOther parses other XML metadata and merges changelogs into packages.
func ParseOther(r io.Reader, packages []Package) ([]Package, error) {
	decoder := xml.NewDecoder(r)
	index := packageIndex(packages)
	var current *Package
	var changelog ChangelogEntry
	var collect string
	var text strings.Builder

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch token := tok.(type) {
		case xml.StartElement:
			switch token.Name.Local {
			case "package":
				key := attrValue(token.Attr, "pkgid")
				current = index[key]
				if current == nil {
					packages = append(packages, Package{
						PkgID: attrValue(token.Attr, "pkgid"),
						Name:  attrValue(token.Attr, "name"),
						Arch:  attrValue(token.Attr, "arch"),
					})
					current = &packages[len(packages)-1]
					index[current.PkgID] = current
				}
			case "version":
				if current != nil {
					current.Epoch = attrValue(token.Attr, "epoch")
					current.Version = attrValue(token.Attr, "ver")
					current.Release = attrValue(token.Attr, "rel")
				}
			case "changelog":
				changelog = ChangelogEntry{
					Author: attrValue(token.Attr, "author"),
					Date:   attrInt64(token.Attr, "date"),
				}
				startCollect("changelog", &collect, &text)
			}
		case xml.CharData:
			if collect != "" {
				text.Write([]byte(token))
			}
		case xml.EndElement:
			if token.Name.Local == collect && current != nil {
				changelog.Changelog = strings.TrimSpace(text.String())
				current.Changelogs = append(current.Changelogs, changelog)
				collect = ""
			}
			if token.Name.Local == "package" {
				current = nil
			}
		}
	}
	return packages, nil
}

func startCollect(local string, collect *string, text *strings.Builder) {
	*collect = local
	text.Reset()
}

func applyPrimaryContent(pkg *Package, local, value, fileType string) {
	switch local {
	case "name":
		pkg.Name = value
	case "arch":
		pkg.Arch = value
	case "checksum":
		pkg.PkgID = value
	case "summary":
		pkg.Summary = value
	case "description":
		pkg.Description = value
	case "packager":
		pkg.RPMPackager = value
	case "url":
		pkg.URL = value
	case "license":
		pkg.RPMLicense = value
	case "vendor":
		pkg.RPMVendor = value
	case "group":
		pkg.RPMGroup = value
	case "buildhost":
		pkg.RPMBuildHost = value
	case "sourcerpm":
		pkg.RPMSourcerpm = value
	case "file":
		pkg.Files = append(pkg.Files, splitPackageFile(fileType, value))
	}
}

func dependencyFromAttrs(attrs []xml.Attr) Dependency {
	return Dependency{
		Name:    attrValue(attrs, "name"),
		Flags:   attrValue(attrs, "flags"),
		Epoch:   attrValue(attrs, "epoch"),
		Version: attrValue(attrs, "ver"),
		Release: attrValue(attrs, "rel"),
		Pre:     attrValue(attrs, "pre") == "1" || strings.EqualFold(attrValue(attrs, "pre"), "true"),
	}
}

func appendDependency(pkg *Package, kind string, dep Dependency) {
	switch kind {
	case "requires":
		pkg.Requires = append(pkg.Requires, dep)
	case "provides":
		pkg.Provides = append(pkg.Provides, dep)
	case "conflicts":
		pkg.Conflicts = append(pkg.Conflicts, dep)
	case "obsoletes":
		pkg.Obsoletes = append(pkg.Obsoletes, dep)
	case "suggests":
		pkg.Suggests = append(pkg.Suggests, dep)
	case "enhances":
		pkg.Enhances = append(pkg.Enhances, dep)
	case "recommends":
		pkg.Recommends = append(pkg.Recommends, dep)
	case "supplements":
		pkg.Supplements = append(pkg.Supplements, dep)
	}
}

func splitPackageFile(fileType, value string) PackageFile {
	value = strings.TrimSpace(value)
	idx := strings.LastIndexByte(value, '/')
	if idx < 0 {
		return PackageFile{Type: fileType, Path: "", Name: value}
	}
	path := value[:idx]
	if path == "" {
		path = "/"
	}
	return PackageFile{Type: fileType, Path: path, Name: value[idx+1:]}
}

func packageIndex(packages []Package) map[string]*Package {
	index := make(map[string]*Package, len(packages))
	for i := range packages {
		index[packages[i].PkgID] = &packages[i]
	}
	return index
}

func attrInt64(attrs []xml.Attr, local string) int64 {
	value := attrValue(attrs, local)
	if value == "" {
		return 0
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func mustExistingFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}
