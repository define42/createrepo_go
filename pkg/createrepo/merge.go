package createrepo

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	rpmfile "github.com/cavaliergopher/rpm"
)

// mergeMethod selects how mergerepo_c resolves packages that share the same
// name and architecture across the source repositories.
type mergeMethod int

const (
	// mergeMethodRepo keeps the package from the first repository specified on
	// the command line (createrepo_c default).
	mergeMethodRepo mergeMethod = iota
	// mergeMethodTimestamp keeps the package with the newest build timestamp.
	mergeMethodTimestamp
	// mergeMethodNVR keeps the package with the highest epoch-version-release.
	mergeMethodNVR
)

func parseMergeMethod(name string) (mergeMethod, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "repo":
		return mergeMethodRepo, nil
	case "ts", "timestamp":
		return mergeMethodTimestamp, nil
	case "nvr":
		return mergeMethodNVR, nil
	default:
		return 0, fmt.Errorf("unknown merge method %q, use 'repo', 'ts', or 'nvr'", name)
	}
}

// mergedPackage couples a package with the source repository it came from so
// pkgorigins metadata can be emitted.
type mergedPackage struct {
	pkg    Package
	origin string
}

// mergeAccumulator deduplicates packages across repositories according to the
// configured merge method and the --all flag.
type mergeAccumulator struct {
	method mergeMethod
	all    bool
	keys   map[string]int
	items  []mergedPackage
}

func newMergeAccumulator(method mergeMethod, all bool) *mergeAccumulator {
	return &mergeAccumulator{method: method, all: all, keys: map[string]int{}}
}

func (a *mergeAccumulator) key(pkg Package) string {
	if a.all {
		// Keep every distinct version; collapse only exact NEVRA duplicates.
		return packageNEVRAKey(pkg)
	}
	return pkg.Name + "\x00" + pkg.Arch
}

// Add inserts a package, resolving conflicts with any package already held
// under the same key. origin records the source repository for pkgorigins.
func (a *mergeAccumulator) Add(pkg Package, origin string) {
	key := a.key(pkg)
	idx, ok := a.keys[key]
	if !ok {
		a.keys[key] = len(a.items)
		a.items = append(a.items, mergedPackage{pkg: pkg, origin: origin})
		return
	}
	if a.preferNew(a.items[idx].pkg, pkg) {
		a.items[idx] = mergedPackage{pkg: pkg, origin: origin}
	}
}

// preferNew reports whether candidate should replace existing under the merge
// method. With the --all key (exact NEVRA) duplicates are identical, so the
// first one is always kept.
func (a *mergeAccumulator) preferNew(existing, candidate Package) bool {
	if a.all {
		return false
	}
	switch a.method {
	case mergeMethodRepo:
		return false
	case mergeMethodTimestamp:
		return packageBuildTime(candidate) > packageBuildTime(existing)
	case mergeMethodNVR:
		return compareEVR(candidate, existing) > 0
	default:
		return false
	}
}

func (a *mergeAccumulator) Packages() []mergedPackage {
	out := make([]mergedPackage, len(a.items))
	copy(out, a.items)
	sort.Slice(out, func(i, j int) bool {
		return sortPackageLess(out[i].pkg, out[j].pkg)
	})
	return out
}

func packageBuildTime(pkg Package) int64 {
	if pkg.TimeBuild != 0 {
		return pkg.TimeBuild
	}
	return pkg.TimeFile
}

// compareEVR compares two packages by epoch, then version, then release using
// RPM version ordering. It returns a value > 0 when a is newer than b.
func compareEVR(a, b Package) int {
	if cmp := compareEpoch(a.Epoch, b.Epoch); cmp != 0 {
		return cmp
	}
	if cmp := rpmfile.CompareVersions(a.Version, b.Version); cmp != 0 {
		return cmp
	}
	return rpmfile.CompareVersions(a.Release, b.Release)
}

func compareEpoch(a, b string) int {
	ea := parseEpoch(a)
	eb := parseEpoch(b)
	switch {
	case ea > eb:
		return 1
	case ea < eb:
		return -1
	default:
		return 0
	}
}

func parseEpoch(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// pkgOriginsMetadata builds the koji-style pkgorigins file mapping each kept
// package's NEVRA to the repository it was merged from.
func pkgOriginsMetadata(packages []mergedPackage) metadataItem {
	var sb strings.Builder
	for _, item := range packages {
		sb.WriteString(packageNEVRA(item.pkg))
		sb.WriteByte('\t')
		sb.WriteString(item.origin)
		sb.WriteByte('\n')
	}
	return metadataItem{Type: "origin", Name: "pkgorigins", Body: []byte(sb.String())}
}

func packageNEVRA(pkg Package) string {
	epoch := parseEpoch(pkg.Epoch)
	if epoch > 0 {
		return fmt.Sprintf("%s-%d:%s-%s.%s", pkg.Name, epoch, pkg.Version, pkg.Release, pkg.Arch)
	}
	return fmt.Sprintf("%s-%s-%s.%s", pkg.Name, pkg.Version, pkg.Release, pkg.Arch)
}

// loadBlockedNames reads a koji blocked-package list (one package name per
// line, '#' comments allowed) into a set.
func loadBlockedNames(path string) (map[string]bool, error) {
	if path == "" {
		return nil, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	blocked := map[string]bool{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		blocked[line] = true
	}
	return blocked, nil
}
