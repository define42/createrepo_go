package createrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	rpmfile "github.com/cavaliergopher/rpm"
)

// ParseRPMPackage reads one RPM header into the public package model.
func ParseRPMPackage(path, repoRoot string, checksumType ChecksumType) (Package, error) {
	rp, err := rpmfile.Open(path)
	if err != nil {
		return Package{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Package{}, err
	}
	sum, err := ChecksumFile(path, checksumType)
	if err != nil {
		return Package{}, err
	}
	checksumName, ok := checksumType.Name()
	if !ok {
		return Package{}, fmt.Errorf("unknown checksum type: %d", checksumType)
	}
	hstart, hend := rp.HeaderRange()
	href := filepath.Base(path)
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, path); err == nil && !strings.HasPrefix(rel, "..") {
			href = filepath.ToSlash(rel)
		}
	}

	pkg := Package{
		PkgID:             sum,
		Name:              rp.Name(),
		Arch:              rp.Architecture(),
		Version:           rp.Version(),
		Epoch:             strconv.Itoa(rp.Epoch()),
		Release:           rp.Release(),
		Summary:           rp.Summary(),
		Description:       rp.Description(),
		URL:               rp.URL(),
		TimeFile:          info.ModTime().Unix(),
		TimeBuild:         rp.BuildTime().Unix(),
		RPMLicense:        rp.License(),
		RPMVendor:         rp.Vendor(),
		RPMGroup:          firstString(rp.Groups()),
		RPMBuildHost:      rp.BuildHost(),
		RPMSourcerpm:      rp.SourceRPM(),
		RPMHeaderStart:    int64(hstart),
		RPMHeaderEnd:      int64(hend),
		RPMPackager:       rp.Packager(),
		SizePackage:       info.Size(),
		SizeInstalled:     int64(rp.Size()),
		SizeArchive:       int64(rp.ArchiveSize()),
		LocationHref:      href,
		ChecksumType:      checksumName,
		FilesChecksumType: fileChecksumName(rp),
		Requires:          convertDependencies(rp.Requires()),
		Provides:          convertDependencies(rp.Provides()),
		Conflicts:         convertDependencies(rp.Conflicts()),
		Obsoletes:         convertDependencies(rp.Obsoletes()),
		Suggests:          convertDependencies(rp.Suggests()),
		Enhances:          convertDependencies(rp.Enhances()),
		Recommends:        convertDependencies(rp.Recommends()),
		Supplements:       convertDependencies(rp.Supplements()),
		Files:             convertFiles(rp.Files()),
		Changelogs:        convertChangelogs(rp),
	}
	sortPackageFiles(pkg.Files)
	return pkg, nil
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func fileChecksumName(rp *rpmfile.Package) string {
	algo := rp.Header.GetTag(5011).Int64()
	switch algo {
	case 1:
		return "md5"
	case 2:
		return "sha1"
	case 8:
		return "sha256"
	case 9:
		return "sha384"
	case 10:
		return "sha512"
	case 11:
		return "sha224"
	default:
		return ""
	}
}

func convertDependencies(deps []rpmfile.Dependency) []Dependency {
	out := make([]Dependency, 0, len(deps))
	for _, dep := range deps {
		epoch := ""
		if dep.Epoch() != 0 || dep.Version() != "" {
			epoch = strconv.Itoa(dep.Epoch())
		}
		out = append(out, Dependency{
			Name:    dep.Name(),
			Flags:   dependencyFlags(dep.Flags()),
			Epoch:   epoch,
			Version: dep.Version(),
			Release: dep.Release(),
			Pre:     dep.Flags()&rpmfile.DepFlagPrereq != 0,
		})
	}
	return out
}

func dependencyFlags(flags int) string {
	hasLess := flags&rpmfile.DepFlagLesser != 0
	hasGreater := flags&rpmfile.DepFlagGreater != 0
	hasEqual := flags&rpmfile.DepFlagEqual != 0
	switch {
	case hasLess && hasEqual:
		return "LE"
	case hasGreater && hasEqual:
		return "GE"
	case hasLess:
		return "LT"
	case hasGreater:
		return "GT"
	case hasEqual:
		return "EQ"
	default:
		return ""
	}
}

func convertFiles(files []rpmfile.FileInfo) []PackageFile {
	out := make([]PackageFile, 0, len(files))
	for _, f := range files {
		fileType := ""
		if f.IsDir() {
			fileType = "dir"
		} else if f.Flags()&rpmfile.FileFlagGhost != 0 {
			fileType = "ghost"
		}
		pf := splitPackageFile(fileType, f.Name())
		pf.Digest = f.Digest()
		out = append(out, pf)
	}
	return out
}

func convertChangelogs(rp *rpmfile.Package) []ChangelogEntry {
	names := rp.Header.GetTag(1081).StringSlice()
	times := rp.Header.GetTag(1080).Int64Slice()
	texts := rp.Header.GetTag(1082).StringSlice()
	n := min(len(names), len(times), len(texts))
	out := make([]ChangelogEntry, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ChangelogEntry{
			Author:    names[i],
			Date:      times[i],
			Changelog: texts[i],
		})
	}
	return out
}

func sortPackages(packages []Package) {
	sort.Slice(packages, func(i, j int) bool {
		a, b := packages[i], packages[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Arch != b.Arch {
			return a.Arch < b.Arch
		}
		if a.Epoch != b.Epoch {
			return a.Epoch < b.Epoch
		}
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		return a.Release < b.Release
	})
}

func sortPackageFiles(files []PackageFile) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path+"/"+files[i].Name < files[j].Path+"/"+files[j].Name
	})
}
