package createrepo

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Dependency is an RPM dependency entry.
type Dependency struct {
	Name    string
	Flags   string
	Epoch   string
	Version string
	Release string
	Pre     bool
}

// PackageFile is one file entry in package metadata.
type PackageFile struct {
	Type   string
	Path   string
	Name   string
	Digest string
}

// ChangelogEntry is one package changelog entry.
type ChangelogEntry struct {
	Author    string
	Date      int64
	Changelog string
}

// Package is the public Go representation of package metadata.
type Package struct {
	PkgID             string
	Name              string
	Arch              string
	Version           string
	Epoch             string
	Release           string
	Summary           string
	Description       string
	URL               string
	TimeFile          int64
	TimeBuild         int64
	RPMLicense        string
	RPMVendor         string
	RPMGroup          string
	RPMBuildHost      string
	RPMSourcerpm      string
	RPMHeaderStart    int64
	RPMHeaderEnd      int64
	RPMPackager       string
	SizePackage       int64
	SizeInstalled     int64
	SizeArchive       int64
	LocationHref      string
	LocationBase      string
	ChecksumType      string
	FilesChecksumType string
	Requires          []Dependency
	Provides          []Dependency
	Conflicts         []Dependency
	Obsoletes         []Dependency
	Suggests          []Dependency
	Enhances          []Dependency
	Recommends        []Dependency
	Supplements       []Dependency
	Files             []PackageFile
	Changelogs        []ChangelogEntry
}

// Repository is a loaded RPM repository.
type Repository struct {
	Path       string
	Metadata   *MetadataLocation
	Packages   []Package
	Repomd     *Repomd
	UpdateInfo *UpdateInfo
	LoadedAt   time.Time
}

// Options controls repository creation.
type Options struct {
	Directory                 string
	OutputDir                 string
	Checksum                  ChecksumType
	RepomdChecksum            ChecksumType
	Compression               CompressionType
	GeneralCompression        CompressionType
	Revision                  string
	SetTimestampToRevision    bool
	UniqueMDFilenames         bool
	SimpleMDFilenames         bool
	Pretty                    bool
	Database                  bool
	FilelistsExt              bool
	BaseURL                   string
	LocationPrefix            string
	BaseDir                   string
	CutDirs                   int
	GroupFile                 string
	Excludes                  []string
	IncludePackages           []string
	PackageListFiles          []string
	AdditionalMetadataPaths   []string
	RepoTags                  []string
	ContentTags               []string
	DistroTags                []DistroTag
	ChangelogLimit            int
	Update                    bool
	SkipSymlinks              bool
	DuplicatedNEVRA           string
	DiscardAdditionalMetadata bool
	Deltas                    bool
	OldPackageDirs            []string
	NumDeltas                 int
	MaxDeltaRPMSize           int64
	Workers                   int
	CacheDir                  string
	SkipStat                  bool
	RetainOldMD               int
	Split                     bool
	SplitDirs                 []string
}

type parsedPackage struct {
	Package Package
	Path    string
}

// ModifyOptions controls modifyrepo_c behavior.
type ModifyOptions struct {
	MetadataPath      string
	RepodataDir       string
	BatchFile         string
	MetadataType      string
	RemoveType        string
	Checksum          ChecksumType
	Compression       CompressionType
	Compress          bool
	NoCompress        bool
	UniqueMDFilenames bool
	NewName           string
}

// MergeOptions controls mergerepo_c behavior.
type MergeOptions struct {
	Repos             []string
	OutputDir         string
	Checksum          ChecksumType
	Compression       CompressionType
	Revision          string
	UniqueMDFilenames bool
	SimpleMDFilenames bool
	Database          bool
	FilelistsExt      bool
	GroupFile         string
	ArchList          []string
	OmitBaseURL       bool
	RepoPrefixSearch  string
	RepoPrefixReplace string
	Method            string
	AllVersions       bool
	Koji              bool
	PkgOrigins        bool
	BlockedFile       string
	ArchExpand        bool
	NoGroups          bool
	NoUpdateInfo      bool
}

// SQLiteOptions controls sqlite metadata generation.
type SQLiteOptions struct {
	RepositoryPath string
	Checksum       ChecksumType
	Compression    CompressionType
	Force          bool
	KeepOld        bool
	LocalSQLite    bool
}

// DeltaOptions controls pure-Go delta RPM generation.
type DeltaOptions struct {
	OldRPM     string
	NewRPM     string
	OutputPath string
	Checksum   ChecksumType
}

// LoadOptions controls metadata loading.
type LoadOptions struct {
	IgnoreSQLite    bool
	CacheDir        string
	VerifyChecksums bool
}

// UpdateInfo is the public representation of updateinfo metadata.
type UpdateInfo struct {
	Updates []UpdateRecord
}

type UpdateRecord struct {
	From            string
	Status          string
	Type            string
	Version         string
	ID              string
	Title           string
	IssuedDate      string
	UpdatedDate     string
	Rights          string
	Release         string
	PushCount       string
	Severity        string
	Summary         string
	Description     string
	Solution        string
	RebootSuggested bool
	References      []UpdateReference
	Collections     []UpdateCollection
}

type UpdateReference struct {
	Href  string
	ID    string
	Type  string
	Title string
}

type UpdateCollection struct {
	ShortName string
	Name      string
	Module    *UpdateCollectionModule
	Packages  []UpdateCollectionPackage
}

type UpdateCollectionModule struct {
	Name    string
	Stream  string
	Version uint64
	Context string
	Arch    string
}

type UpdateCollectionPackage struct {
	Name             string
	Version          string
	Release          string
	Epoch            string
	Arch             string
	Src              string
	Filename         string
	Sum              string
	SumType          ChecksumType
	RebootSuggested  bool
	RestartSuggested bool
	ReloginSuggested bool
}

// Create generates repository metadata from RPMs in a directory.
func Create(ctx context.Context, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.Split {
		if len(opts.SplitDirs) == 0 {
			return fmt.Errorf("at least one directory is required in split mode")
		}
	} else if opts.Directory == "" {
		return fmt.Errorf("repository directory is required")
	}
	if opts.OutputDir == "" {
		if opts.Split {
			opts.OutputDir = opts.SplitDirs[0]
		} else {
			opts.OutputDir = opts.Directory
		}
	}
	if opts.Checksum == ChecksumUnknown {
		opts.Checksum = ChecksumSHA256
	}
	if opts.RepomdChecksum == ChecksumUnknown {
		opts.RepomdChecksum = opts.Checksum
	}
	if opts.Compression == CompressionAuto || opts.Compression == CompressionUnknown {
		opts.Compression = CompressionZstd
	}
	if opts.GeneralCompression == CompressionAuto || opts.GeneralCompression == CompressionUnknown {
		opts.GeneralCompression = opts.Compression
	}
	duplicatePolicy, err := duplicateNEVRAPolicy(opts.DuplicatedNEVRA)
	if err != nil {
		return err
	}
	if opts.SimpleMDFilenames {
		opts.UniqueMDFilenames = false
	} else if !opts.UniqueMDFilenames {
		opts.UniqueMDFilenames = true
	}
	jobs, err := buildRPMJobs(opts)
	if err != nil {
		return err
	}
	cache, err := newChecksumCache(opts.CacheDir, opts.SkipStat)
	if err != nil {
		return err
	}
	parsedPackages, err := parsePackagesParallel(ctx, jobs, opts, cache)
	if err != nil {
		return err
	}
	parsedPackages = applyParsedDuplicateNEVRAPolicy(parsedPackages, duplicatePolicy)
	packages := make([]Package, 0, len(parsedPackages))
	for _, parsed := range parsedPackages {
		packages = append(packages, parsed.Package)
	}
	sortPackages(packages)

	revision := opts.Revision
	if revision == "" {
		revision = fmt.Sprintf("%d", time.Now().Unix())
	}
	var extraMetadata []metadataItem
	metadataSources := append([]string{}, opts.AdditionalMetadataPaths...)
	if opts.Update && !opts.DiscardAdditionalMetadata {
		metadataSources = append(metadataSources, opts.OutputDir)
	}
	for _, metadataSource := range metadataSources {
		items, err := additionalMetadataItemsFromRepo(metadataSource)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, item := range items {
			extraMetadata = upsertMetadataItem(extraMetadata, item)
		}
	}
	if opts.GroupFile != "" {
		item, err := groupMetadataItem(opts.GroupFile)
		if err != nil {
			return err
		}
		extraMetadata = upsertMetadataItem(extraMetadata, item)
	}
	if opts.Deltas {
		deltaMetadata, err := generateDeltaMetadata(ctx, opts.OutputDir, parsedPackages, deltaGenerationOptions{
			OldPackageDirs:  opts.OldPackageDirs,
			NumDeltas:       opts.NumDeltas,
			MaxDeltaRPMSize: opts.MaxDeltaRPMSize,
			Checksum:        opts.Checksum,
		})
		if err != nil {
			return err
		}
		for _, item := range deltaMetadata {
			extraMetadata = upsertMetadataItem(extraMetadata, item)
		}
	}
	if err := writeRepositoryMetadata(opts.OutputDir, packages, repositoryWriteOptions{
		Revision:               revision,
		Checksum:               opts.RepomdChecksum,
		Compression:            opts.Compression,
		UniqueMDFilenames:      opts.UniqueMDFilenames,
		SetTimestampToRevision: opts.SetTimestampToRevision,
		FilelistsExt:           opts.FilelistsExt,
		ExtraMetadata:          extraMetadata,
		RepoTags:               opts.RepoTags,
		ContentTags:            opts.ContentTags,
		DistroTags:             opts.DistroTags,
		RetainOldMD:            opts.RetainOldMD,
	}); err != nil {
		return err
	}
	if opts.Database {
		return GenerateSQLite(ctx, SQLiteOptions{
			RepositoryPath: opts.OutputDir,
			Checksum:       opts.RepomdChecksum,
			Compression:    opts.Compression,
			LocalSQLite:    true,
		})
	}
	return nil
}

type repositoryWriteOptions struct {
	Revision               string
	Checksum               ChecksumType
	Compression            CompressionType
	UniqueMDFilenames      bool
	SetTimestampToRevision bool
	FilelistsExt           bool
	ExtraMetadata          []metadataItem
	RepoTags               []string
	ContentTags            []string
	DistroTags             []DistroTag
	RetainOldMD            int
}

type metadataItem struct {
	Type string
	Name string
	Body []byte
}

func writeRepositoryMetadata(outputDir string, packages []Package, opts repositoryWriteOptions) error {
	repodataDir := filepath.Join(outputDir, "repodata")
	previousFiles := previousMetadataFiles(repodataDir)
	if err := os.MkdirAll(repodataDir, 0o755); err != nil {
		return err
	}
	items := []metadataItem{
		{Type: "primary", Name: "primary.xml", Body: DumpPrimary(packages)},
		{Type: "filelists", Name: "filelists.xml", Body: DumpFilelists(packages)},
		{Type: "other", Name: "other.xml", Body: DumpOther(packages)},
	}
	if opts.FilelistsExt {
		items = append(items, metadataItem{Type: "filelists-ext", Name: "filelists-ext.xml", Body: DumpFilelistsExt(packages)})
	}
	items = append(items, opts.ExtraMetadata...)
	records := []*RepomdRecord{}
	for _, item := range items {
		path := filepath.Join(repodataDir, item.Name+opts.Compression.Suffix())
		if err := WriteCompressedFile(path, item.Body, opts.Compression); err != nil {
			return err
		}
		record := NewRepomdRecord(item.Type, path)
		if err := record.Fill(opts.Checksum); err != nil {
			return err
		}
		if opts.UniqueMDFilenames {
			if err := record.RenameFile(); err != nil {
				return err
			}
		}
		records = append(records, record)
	}

	repomd := &Repomd{
		Revision:    opts.Revision,
		RepoTags:    append([]string{}, opts.RepoTags...),
		ContentTags: append([]string{}, opts.ContentTags...),
		DistroTags:  append([]DistroTag{}, opts.DistroTags...),
		Records:     records,
	}
	if opts.SetTimestampToRevision {
		ts, err := parseRevisionTimestamp(opts.Revision)
		if err != nil {
			return err
		}
		for _, record := range records {
			record.Timestamp = ts
			_ = os.Chtimes(record.LocationReal, time.Unix(ts, 0), time.Unix(ts, 0))
		}
	}

	if err := os.WriteFile(filepath.Join(repodataDir, "repomd.xml"), []byte(DumpRepomd(repomd)), 0o644); err != nil {
		return err
	}
	pruneOldMetadataFiles(repodataDir, previousFiles, records, opts.RetainOldMD)
	return nil
}

// previousMetadataFiles returns the repodata-relative file names referenced by
// an existing repomd.xml, if one is present. These are the files that may be
// superseded once new metadata is written.
func previousMetadataFiles(repodataDir string) []string {
	repomd, err := ParseRepomdFile(filepath.Join(repodataDir, "repomd.xml"))
	if err != nil {
		return nil
	}
	var files []string
	for _, record := range repomd.Records {
		if name := filepath.Base(record.LocationHref); name != "" && name != "." {
			files = append(files, name)
		}
	}
	return files
}

// pruneOldMetadataFiles removes superseded metadata files from repodataDir,
// retaining the newest retain files (by modification time). Pruning only runs
// when a previous repomd.xml existed (previous is non-empty), so a fresh
// repository is never touched. Files referenced by the freshly written repomd
// and any repomd.xml* files (such as detached signatures) are always kept.
func pruneOldMetadataFiles(repodataDir string, previous []string, current []*RepomdRecord, retain int) {
	if len(previous) == 0 {
		return
	}
	keep := map[string]bool{}
	for _, record := range current {
		keep[filepath.Base(record.LocationHref)] = true
	}

	entries, err := os.ReadDir(repodataDir)
	if err != nil {
		return
	}
	type candidate struct {
		name    string
		modTime int64
	}
	// Group superseded files by metadata kind (the name with its checksum
	// prefix stripped) so --retain-old-md keeps N old versions per kind.
	groups := map[string][]candidate{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || keep[name] || strings.HasPrefix(name, "repomd.xml") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		kind := stripObsoleteChecksumPrefix(name)
		groups[kind] = append(groups[kind], candidate{name: name, modTime: info.ModTime().Unix()})
	}
	for _, stale := range groups {
		if retain > 0 {
			sort.Slice(stale, func(i, j int) bool { return stale[i].modTime > stale[j].modTime })
			if retain < len(stale) {
				stale = stale[retain:]
			} else {
				continue
			}
		}
		for _, c := range stale {
			_ = os.Remove(filepath.Join(repodataDir, c.name))
		}
	}
}

// Modify adds or removes metadata records in an existing repodata directory.
func Modify(ctx context.Context, opts ModifyOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.RepodataDir == "" {
		return fmt.Errorf("repodata directory is required")
	}
	if opts.BatchFile != "" {
		tasks, err := parseModifyBatchFile(opts.BatchFile, opts)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			if err := Modify(ctx, task); err != nil {
				return err
			}
		}
		return nil
	}
	if opts.Checksum == ChecksumUnknown {
		opts.Checksum = ChecksumSHA256
	}
	repomdPath := filepath.Join(opts.RepodataDir, "repomd.xml")
	repomd, err := ParseRepomdFile(repomdPath)
	if err != nil {
		return err
	}

	if opts.RemoveType != "" {
		filtered := repomd.Records[:0]
		for _, record := range repomd.Records {
			if record.Type != opts.RemoveType {
				filtered = append(filtered, record)
			}
		}
		repomd.Records = filtered
		return os.WriteFile(repomdPath, []byte(DumpRepomd(repomd)), 0o644)
	}

	if opts.MetadataPath == "" {
		return fmt.Errorf("metadata path is required")
	}
	if opts.MetadataType == "" {
		return fmt.Errorf("metadata type is required")
	}
	name := opts.NewName
	if name == "" {
		name = filepath.Base(opts.MetadataPath)
	}
	if opts.Compression == CompressionAuto || opts.Compression == CompressionUnknown {
		opts.Compression = CompressionZstd
	}
	if opts.Compress && !opts.NoCompress {
		if suffix := opts.Compression.Suffix(); suffix != "" && !strings.HasSuffix(name, suffix) {
			name += suffix
		}
	}
	dst := filepath.Join(opts.RepodataDir, name)
	if opts.Compress && !opts.NoCompress {
		body, err := os.ReadFile(opts.MetadataPath)
		if err != nil {
			return err
		}
		if err := WriteCompressedFile(dst, body, opts.Compression); err != nil {
			return err
		}
	} else {
		if err := copyFile(opts.MetadataPath, dst); err != nil {
			return err
		}
	}
	record := NewRepomdRecord(opts.MetadataType, dst)
	if err := record.Fill(opts.Checksum); err != nil {
		return err
	}
	if opts.UniqueMDFilenames {
		if err := record.RenameFile(); err != nil {
			return err
		}
	}

	replaced := false
	for i, existing := range repomd.Records {
		if existing.Type == record.Type {
			repomd.Records[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		repomd.Records = append(repomd.Records, record)
	}
	return os.WriteFile(repomdPath, []byte(DumpRepomd(repomd)), 0o644)
}

// Merge merges repositories by loading package XML metadata from each source
// repository and writing a combined repository.
func Merge(ctx context.Context, opts MergeOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(opts.Repos) < 2 {
		return fmt.Errorf("at least two repositories are required")
	}
	if opts.OutputDir == "" {
		opts.OutputDir = "merged_repo"
	}
	if opts.Checksum == ChecksumUnknown {
		opts.Checksum = ChecksumSHA256
	}
	if opts.Compression == CompressionAuto || opts.Compression == CompressionUnknown {
		opts.Compression = CompressionZstd
	}
	if opts.SimpleMDFilenames {
		opts.UniqueMDFilenames = false
	} else if !opts.UniqueMDFilenames {
		opts.UniqueMDFilenames = true
	}
	method, err := parseMergeMethod(opts.Method)
	if err != nil {
		return err
	}
	// Koji mode keeps every package version and records their origins.
	if opts.Koji {
		opts.AllVersions = true
		opts.PkgOrigins = true
	}
	blocked, err := loadBlockedNames(opts.BlockedFile)
	if err != nil {
		return err
	}
	accumulator := newMergeAccumulator(method, opts.AllVersions)
	var updates []UpdateRecord
	var groupItem *metadataItem
	for _, repoPath := range opts.Repos {
		repo, err := LoadMetadata(ctx, repoPath, LoadOptions{IgnoreSQLite: true})
		if err != nil {
			return err
		}
		repoBaseURL := applyPrefixReplacement(repoPath, opts.RepoPrefixSearch, opts.RepoPrefixReplace)
		for _, pkg := range repo.Packages {
			if blocked[pkg.Name] {
				continue
			}
			if len(opts.ArchList) > 0 && !stringInSlice(pkg.Arch, opts.ArchList) {
				if !(opts.ArchExpand && pkg.Arch == "noarch") {
					continue
				}
			}
			if opts.OmitBaseURL {
				pkg.LocationBase = ""
			} else if pkg.LocationBase == "" {
				pkg.LocationBase = repoBaseURL
			} else {
				pkg.LocationBase = applyPrefixReplacement(pkg.LocationBase, opts.RepoPrefixSearch, opts.RepoPrefixReplace)
			}
			accumulator.Add(pkg, repoBaseURL)
		}
		if !opts.NoUpdateInfo && repo.UpdateInfo != nil {
			updates = append(updates, repo.UpdateInfo.Updates...)
		}
		if groupItem == nil && opts.GroupFile == "" && !opts.NoGroups {
			if item, ok := firstGroupMetadata(repoPath); ok {
				groupItem = &item
			}
		}
	}
	merged := accumulator.Packages()
	packages := make([]Package, 0, len(merged))
	for _, item := range merged {
		packages = append(packages, item.pkg)
	}
	revision := opts.Revision
	if revision == "" {
		revision = fmt.Sprintf("%d", time.Now().Unix())
	}
	var extraMetadata []metadataItem
	if opts.GroupFile != "" {
		item, err := groupMetadataItem(opts.GroupFile)
		if err != nil {
			return err
		}
		extraMetadata = append(extraMetadata, item)
	} else if groupItem != nil {
		extraMetadata = append(extraMetadata, *groupItem)
	}
	if !opts.NoUpdateInfo && len(updates) > 0 {
		body := DumpUpdateInfo(&UpdateInfo{Updates: updates})
		extraMetadata = append(extraMetadata, metadataItem{Type: "updateinfo", Name: "updateinfo.xml", Body: body})
	}
	if opts.PkgOrigins {
		extraMetadata = append(extraMetadata, pkgOriginsMetadata(merged))
	}
	if err := writeRepositoryMetadata(opts.OutputDir, packages, repositoryWriteOptions{
		Revision:          revision,
		Checksum:          opts.Checksum,
		Compression:       opts.Compression,
		UniqueMDFilenames: opts.UniqueMDFilenames,
		FilelistsExt:      opts.FilelistsExt,
		ExtraMetadata:     extraMetadata,
	}); err != nil {
		return err
	}
	if opts.Database {
		return GenerateSQLite(ctx, SQLiteOptions{
			RepositoryPath: opts.OutputDir,
			Checksum:       opts.Checksum,
			Compression:    opts.Compression,
			LocalSQLite:    true,
		})
	}
	return nil
}

// GenerateDeltaRPM creates one RPM-only delta RPM.
func GenerateDeltaRPM(ctx context.Context, opts DeltaOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.OldRPM == "" || opts.NewRPM == "" || opts.OutputPath == "" {
		return fmt.Errorf("old RPM, new RPM, and output path are required")
	}
	return writeRPMOnlyDelta(opts.OldRPM, opts.NewRPM, opts.OutputPath)
}

// LoadMetadata loads repository metadata from a local repository path.
func LoadMetadata(ctx context.Context, pathOrURL string, opts LoadOptions) (*Repository, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pathOrURL == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	if u, err := url.Parse(pathOrURL); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return loadRemoteMetadata(ctx, pathOrURL, opts)
	}
	if strings.HasPrefix(pathOrURL, "file://") {
		u, err := url.Parse(pathOrURL)
		if err != nil {
			return nil, err
		}
		pathOrURL = u.Path
	}
	return loadLocalMetadata(ctx, pathOrURL, opts)
}

func loadLocalMetadata(ctx context.Context, pathOrURL string, opts LoadOptions) (*Repository, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	location, err := LocalMetadata(pathOrURL, opts.IgnoreSQLite)
	if err != nil {
		return nil, err
	}
	var packages []Package
	if location.PrimaryXMLHref != "" {
		if err := mustExistingFile(location.PrimaryXMLHref); err == nil {
			packages, err = ParsePrimaryFile(location.PrimaryXMLHref)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(packages) > 0 && location.FilelistsXMLHref != "" {
		if err := mustExistingFile(location.FilelistsXMLHref); err == nil {
			packages, err = ParseFilelistsFile(location.FilelistsXMLHref, packages)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(packages) > 0 && location.FilelistsExtHref != "" {
		if err := mustExistingFile(location.FilelistsExtHref); err == nil {
			packages, err = ParseFilelistsExtFile(location.FilelistsExtHref, packages)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(packages) > 0 && location.OtherXMLHref != "" {
		if err := mustExistingFile(location.OtherXMLHref); err == nil {
			packages, err = ParseOtherFile(location.OtherXMLHref, packages)
			if err != nil {
				return nil, err
			}
		}
	}
	updateInfo, err := loadRepositoryUpdateInfo(location)
	if err != nil {
		return nil, err
	}
	return &Repository{
		Path:       pathOrURL,
		Metadata:   location,
		Repomd:     location.Repomd,
		Packages:   packages,
		UpdateInfo: updateInfo,
		LoadedAt:   time.Now(),
	}, nil
}

func loadRepositoryUpdateInfo(location *MetadataLocation) (*UpdateInfo, error) {
	var fallback string
	for _, item := range location.AdditionalMetadata {
		switch item.Type {
		case "updateinfo":
			return ParseUpdateInfoFile(item.Name)
		case "updateinfo_zck":
			if fallback == "" {
				fallback = item.Name
			}
		}
	}
	if fallback != "" {
		return ParseUpdateInfoFile(fallback)
	}
	return nil, nil
}

func applyLocationOptions(href string, cutDirs int, prefix string) string {
	href = strings.TrimLeft(filepath.ToSlash(href), "/")
	if cutDirs > 0 && href != "" {
		parts := strings.Split(href, "/")
		if cutDirs < len(parts) {
			href = strings.Join(parts[cutDirs:], "/")
		} else {
			href = parts[len(parts)-1]
		}
	}
	prefix = strings.TrimRight(filepath.ToSlash(strings.TrimSpace(prefix)), "/")
	if prefix == "" {
		return href
	}
	if href == "" {
		return prefix
	}
	return prefix + "/" + href
}

func applyPrefixReplacement(value, search, replace string) string {
	if value == "" || search == "" || !strings.HasPrefix(value, search) {
		return value
	}
	return replace + strings.TrimPrefix(value, search)
}

func findRPMs(root string, skipSymlinks bool) ([]string, error) {
	var rpms []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skipSymlinks && d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if filepath.Base(path) == "repodata" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".rpm") {
			rpms = append(rpms, path)
		}
		return nil
	})
	return rpms, err
}

func upsertMetadataItem(items []metadataItem, item metadataItem) []metadataItem {
	for i := range items {
		if items[i].Type == item.Type {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func additionalMetadataItemsFromRepo(repoPath string) ([]metadataItem, error) {
	if repoPath == "" {
		return nil, nil
	}
	repomdPath, metadataRoot := repomdPathAndRoot(repoPath)
	location, err := ParseRepomdLocation(repomdPath, metadataRoot, true)
	if err != nil {
		return nil, err
	}
	var items []metadataItem
	for _, item := range location.AdditionalMetadata {
		if !copyableAdditionalMetadataType(item.Type) {
			continue
		}
		r, err := OpenReader(item.Name, CompressionAuto)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		items = upsertMetadataItem(items, metadataItem{
			Type: item.Type,
			Name: metadataItemBaseName(item.Name),
			Body: body,
		})
	}
	return items, nil
}

// firstGroupMetadata returns the comps/group metadata item from a repository,
// if present, so mergerepo_c can carry groups over from the source repos.
func firstGroupMetadata(repoPath string) (metadataItem, bool) {
	items, err := additionalMetadataItemsFromRepo(repoPath)
	if err != nil {
		return metadataItem{}, false
	}
	for _, item := range items {
		if item.Type == "group" {
			return item, true
		}
	}
	return metadataItem{}, false
}

func repomdPathAndRoot(path string) (string, string) {
	clean := filepath.Clean(path)
	if filepath.Base(clean) == "repomd.xml" {
		repodataDir := filepath.Dir(clean)
		if filepath.Base(repodataDir) == "repodata" {
			return clean, filepath.Dir(repodataDir)
		}
		return clean, repodataDir
	}
	if filepath.Base(clean) == "repodata" {
		return filepath.Join(clean, "repomd.xml"), filepath.Dir(clean)
	}
	return filepath.Join(clean, "repodata", "repomd.xml"), clean
}

func metadataItemBaseName(path string) string {
	return stripObsoleteChecksumPrefix(stripCompressionSuffix(filepath.Base(path)))
}

func copyableAdditionalMetadataType(recordType string) bool {
	return !strings.HasSuffix(recordType, "_zck") && !strings.HasSuffix(recordType, "_gz")
}

func filterRPMs(rpms []string, root string, excludes, includes, packageListFiles []string) ([]string, error) {
	packageList, err := loadPackageListFiles(packageListFiles)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(rpms))
	for _, rpmPath := range rpms {
		if len(packageList) > 0 && !matchesAnyPackagePattern(rpmPath, root, packageList) {
			continue
		}
		if len(includes) > 0 && !matchesAnyPackagePattern(rpmPath, root, includes) {
			continue
		}
		if matchesAnyPackagePattern(rpmPath, root, excludes) {
			continue
		}
		filtered = append(filtered, rpmPath)
	}
	return filtered, nil
}

func loadPackageListFiles(files []string) ([]string, error) {
	var patterns []string
	for _, filename := range files {
		if filename == "" {
			continue
		}
		body, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(body), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			patterns = append(patterns, line)
		}
	}
	return patterns, nil
}

func matchesAnyPackagePattern(rpmPath, root string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPackagePattern(rpmPath, root, pattern) {
			return true
		}
	}
	return false
}

func matchPackagePattern(rpmPath, root, pattern string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	base := filepath.Base(rpmPath)
	rel := filepath.ToSlash(base)
	if root != "" {
		if r, err := filepath.Rel(root, rpmPath); err == nil && !strings.HasPrefix(r, "..") {
			rel = filepath.ToSlash(r)
		}
	}
	full := filepath.ToSlash(rpmPath)
	for _, candidate := range []string{base, rel, full} {
		if pattern == candidate {
			return true
		}
		if ok, err := filepath.Match(pattern, candidate); err == nil && ok {
			return true
		}
	}
	return false
}

func duplicateNEVRAPolicy(policy string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "", "keep-last":
		return "keep-last", nil
	case "keep":
		return "keep", nil
	default:
		return "", fmt.Errorf("bad duplicated-nevra argument %q, use 'keep' or 'keep-last'", policy)
	}
}

func applyParsedDuplicateNEVRAPolicy(packages []parsedPackage, policy string) []parsedPackage {
	if policy == "keep" || len(packages) < 2 {
		return packages
	}
	seen := map[string]int{}
	filtered := make([]parsedPackage, 0, len(packages))
	for _, pkg := range packages {
		key := packageNEVRAKey(pkg.Package)
		if index, ok := seen[key]; ok {
			filtered[index] = pkg
			continue
		}
		seen[key] = len(filtered)
		filtered = append(filtered, pkg)
	}
	return filtered
}

func packageNEVRAKey(pkg Package) string {
	return strings.Join([]string{pkg.Name, pkg.Epoch, pkg.Version, pkg.Release, pkg.Arch}, "\x00")
}

func stringInSlice(value string, values []string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func groupMetadataItem(filename string) (metadataItem, error) {
	r, err := OpenReader(filename, CompressionAuto)
	if err != nil {
		return metadataItem{}, err
	}
	defer r.Close()
	body, err := io.ReadAll(r)
	if err != nil {
		return metadataItem{}, err
	}
	return metadataItem{
		Type: "group",
		Name: stripCompressionSuffix(filepath.Base(filename)),
		Body: body,
	}, nil
}

func stripCompressionSuffix(name string) string {
	for {
		stripped := false
		for _, suffix := range []string{".zck", ".zst", ".xz", ".bz2", ".gz", ".gzip", ".bzip2"} {
			if strings.HasSuffix(name, suffix) {
				name = strings.TrimSuffix(name, suffix)
				stripped = true
				break
			}
		}
		if !stripped {
			return name
		}
	}
}

func parseRevisionTimestamp(revision string) (int64, error) {
	var ts int64
	_, err := fmt.Sscanf(revision, "%d", &ts)
	if err != nil {
		return 0, fmt.Errorf("revision must be a unix timestamp: %w", err)
	}
	return ts, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := out.ReadFrom(in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
