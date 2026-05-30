package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	cr "github.com/define42/createrepo_go/pkg/createrepo"
)

func RunCreate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var opts cr.Options
	var quiet, verbose, version, useXZ, useZchunk bool
	var checksumName, repomdChecksumName, compressionName, generalCompressionName string
	var oldPackageDirs, excludes, includePackages, packageListFiles stringSlice
	var updateMDPaths, repoTags, contentTags, distroTags stringSlice
	var localSQLite, compatibility bool
	compressionExplicit := hasLongFlag(args, "compress-type")
	fs := flag.NewFlagSet("createrepo_c", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&version, "version", false, "print version")
	fs.BoolVar(&version, "V", false, "print version")
	fs.BoolVar(&quiet, "quiet", false, "quiet output")
	fs.BoolVar(&quiet, "q", false, "quiet output")
	fs.BoolVar(&verbose, "verbose", false, "verbose output")
	fs.BoolVar(&verbose, "v", false, "verbose output")
	fs.StringVar(&opts.OutputDir, "outputdir", "", "output directory")
	fs.StringVar(&opts.OutputDir, "o", "", "output directory")
	fs.StringVar(&checksumName, "checksum", "sha256", "checksum type")
	fs.StringVar(&checksumName, "s", "sha256", "checksum type")
	fs.StringVar(&repomdChecksumName, "repomd-checksum", "", "repomd checksum type")
	fs.StringVar(&compressionName, "compress-type", "zstd", "metadata compression")
	fs.StringVar(&generalCompressionName, "general-compress-type", "", "general metadata compression")
	fs.StringVar(&opts.Revision, "revision", "", "repository revision")
	fs.BoolVar(&opts.SetTimestampToRevision, "set-timestamp-to-revision", false, "set timestamps to revision")
	fs.BoolVar(&opts.Database, "database", false, "generate sqlite metadata")
	fs.BoolVar(&opts.Database, "d", false, "generate sqlite metadata")
	fs.BoolVar(&opts.FilelistsExt, "filelists-ext", false, "generate filelists-ext metadata")
	fs.BoolVar(&opts.Pretty, "pretty", false, "pretty XML")
	fs.BoolVar(&opts.Deltas, "deltas", false, "generate delta RPM metadata")
	fs.Var(&oldPackageDirs, "oldpackagedirs", "old package directories for delta RPMs")
	fs.IntVar(&opts.NumDeltas, "num-deltas", 0, "maximum delta RPMs per package")
	fs.Int64Var(&opts.MaxDeltaRPMSize, "max-delta-rpm-size", 0, "maximum delta RPM size")
	fs.Var(&excludes, "excludes", "package path globs to exclude")
	fs.Var(&excludes, "x", "package path globs to exclude")
	fs.Var(&includePackages, "includepkg", "package path globs to include")
	fs.Var(&includePackages, "n", "package path globs to include")
	fs.StringVar(&opts.BaseURL, "baseurl", "", "base URL for package locations")
	fs.StringVar(&opts.BaseURL, "u", "", "base URL for package locations")
	fs.Var(&packageListFiles, "pkglist", "package list file")
	fs.Var(&packageListFiles, "i", "package list file")
	fs.Var(&packageListFiles, "read-pkgs-list", "package list file")
	fs.IntVar(&opts.ChangelogLimit, "changelog-limit", 0, "limit changelog entries per package")
	noDatabase := fs.Bool("no-database", false, "do not generate sqlite metadata")
	simpleNames := fs.Bool("simple-md-filenames", false, "do not checksum-prefix metadata filenames")
	uniqueNames := fs.Bool("unique-md-filenames", true, "checksum-prefix metadata filenames")
	fs.BoolVar(&useXZ, "xz", false, "use xz compression")
	fs.BoolVar(&useZchunk, "zck", false, "use zchunk compression")
	fs.Var(&updateMDPaths, "update-md-path", "existing repository metadata to preserve")
	fs.Var(&distroTags, "distro", "distro tag")
	fs.Var(&contentTags, "content", "content tag")
	fs.Var(&repoTags, "repo", "repo tag")
	fs.StringVar(&opts.BaseDir, "basedir", "", "base directory for package locations")
	fs.StringVar(&opts.GroupFile, "groupfile", "", "group metadata file")
	fs.StringVar(&opts.GroupFile, "g", "", "group metadata file")
	ignoredString(fs, "cachedir")
	ignoredString(fs, "c")
	ignoredString(fs, "retain-old-md-by-age")
	fs.StringVar(&opts.DuplicatedNEVRA, "duplicated-nevra", "keep-last", "duplicate NEVRA policy")
	ignoredInt(fs, "workers")
	ignoredInt(fs, "retain-old-md")
	fs.IntVar(&opts.CutDirs, "cut-dirs", 0, "location href path components to ignore")
	fs.BoolVar(&opts.Update, "update", false, "preserve additional metadata from an existing repository")
	ignoredBool(fs, "skip-stat")
	ignoredBool(fs, "split")
	fs.BoolVar(&opts.SkipSymlinks, "skip-symlinks", false, "skip symlinked RPMs")
	fs.BoolVar(&opts.SkipSymlinks, "S", false, "skip symlinked RPMs")
	fs.BoolVar(&compatibility, "compatibility", false, "compatibility mode")
	keepAllMetadata := fs.Bool("keep-all-metadata", true, "preserve additional metadata during update")
	fs.BoolVar(&opts.DiscardAdditionalMetadata, "discard-additional-metadata", false, "discard additional metadata during update")
	fs.BoolVar(&localSQLite, "local-sqlite", false, "generate sqlite metadata")
	ignoredBool(fs, "recycle-pkglist")
	ignoredBool(fs, "error-exit-val")
	ignoredBool(fs, "ignore-lock")
	fs.StringVar(&opts.LocationPrefix, "location-prefix", "", "package location href prefix")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if version {
		fmt.Fprintf(stdout, "createrepo_c %s\n", cr.Version)
		return 0
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Usage: createrepo_c [options] <directory>")
		return 2
	}
	opts.Directory = fs.Arg(0)
	opts.Checksum = cr.ChecksumTypeFromName(checksumName)
	opts.RepomdChecksum = checksumFromFlag(repomdChecksumName)
	if useXZ {
		compressionName = "xz"
	}
	if useZchunk {
		compressionName = "zck"
	}
	if compatibility && !compressionExplicit && compressionName == "zstd" && !useZchunk && !useXZ {
		compressionName = "gz"
	}
	opts.Compression = compressionFromFlag(compressionName)
	opts.GeneralCompression = compressionFromFlag(generalCompressionName)
	opts.UniqueMDFilenames = *uniqueNames && !*simpleNames
	opts.SimpleMDFilenames = *simpleNames
	opts.OldPackageDirs = oldPackageDirs
	opts.Excludes = excludes
	opts.IncludePackages = includePackages
	opts.PackageListFiles = packageListFiles
	opts.AdditionalMetadataPaths = updateMDPaths
	opts.RepoTags = repoTags
	opts.ContentTags = contentTags
	opts.DistroTags = parseDistroTags(distroTags)
	if !*keepAllMetadata {
		opts.DiscardAdditionalMetadata = true
	}
	if *noDatabase {
		opts.Database = false
	}
	if localSQLite {
		opts.Database = true
	}
	if err := cr.Create(ctx, opts); err != nil {
		return printError(stderr, quiet, verbose, err)
	}
	return 0
}

func RunModify(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var opts cr.ModifyOptions
	var version bool
	var checksumName, compressionName string
	var useZchunk bool
	fs := flag.NewFlagSet("modifyrepo_c", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&version, "version", false, "print version")
	fs.StringVar(&opts.MetadataType, "mdtype", "", "metadata type")
	fs.StringVar(&opts.RemoveType, "remove", "", "remove metadata type")
	fs.BoolVar(&opts.Compress, "compress", false, "compress metadata")
	fs.BoolVar(&opts.NoCompress, "no-compress", false, "do not compress metadata")
	fs.StringVar(&compressionName, "compress-type", "zstd", "metadata compression")
	fs.StringVar(&checksumName, "checksum", "sha256", "checksum type")
	fs.StringVar(&checksumName, "s", "sha256", "checksum type")
	fs.BoolVar(&opts.UniqueMDFilenames, "unique-md-filenames", false, "checksum-prefix metadata filenames")
	simpleNames := fs.Bool("simple-md-filenames", false, "do not checksum-prefix metadata filenames")
	fs.StringVar(&opts.NewName, "new-name", "", "new metadata filename")
	fs.StringVar(&opts.BatchFile, "batchfile", "", "batch file")
	fs.StringVar(&opts.BatchFile, "f", "", "batch file")
	ignoredString(fs, "zck-dict-dir")
	ignoredBool(fs, "verbose")
	fs.BoolVar(&useZchunk, "zck", false, "use zchunk compression")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if version {
		fmt.Fprintf(stdout, "modifyrepo_c %s\n", cr.Version)
		return 0
	}
	if *simpleNames {
		opts.UniqueMDFilenames = false
	}
	opts.Checksum = cr.ChecksumTypeFromName(checksumName)
	if useZchunk {
		compressionName = "zck"
	}
	opts.Compression = compressionFromFlag(compressionName)
	if opts.BatchFile != "" {
		if fs.NArg() != 1 {
			fmt.Fprintln(stderr, "Usage: modifyrepo_c --batchfile <batch file> <output repodata>")
			return 2
		}
		opts.RepodataDir = fs.Arg(0)
	} else if opts.RemoveType != "" {
		if fs.NArg() != 1 {
			fmt.Fprintln(stderr, "Usage: modifyrepo_c --remove <metadata type> <output repodata>")
			return 2
		}
		opts.RepodataDir = fs.Arg(0)
	} else {
		if fs.NArg() != 2 {
			fmt.Fprintln(stderr, "Usage: modifyrepo_c [options] <metadata> <output repodata>")
			return 2
		}
		opts.MetadataPath = fs.Arg(0)
		opts.RepodataDir = fs.Arg(1)
	}
	if err := cr.Modify(ctx, opts); err != nil {
		return printError(stderr, false, false, err)
	}
	return 0
}

func RunMerge(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var opts cr.MergeOptions
	var version, useZchunk bool
	var repos stringSlice
	var archList, compressionName string
	var noDatabase bool
	fs := flag.NewFlagSet("mergerepo_c", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&version, "version", false, "print version")
	fs.Var(&repos, "repo", "repository")
	fs.Var(&repos, "r", "repository")
	fs.StringVar(&opts.OutputDir, "outputdir", "", "output directory")
	fs.StringVar(&opts.OutputDir, "o", "", "output directory")
	fs.StringVar(&compressionName, "compress-type", "zstd", "metadata compression")
	fs.StringVar(&opts.RepoPrefixSearch, "repo-prefix-search", "", "repository prefix to replace")
	fs.StringVar(&opts.RepoPrefixReplace, "repo-prefix-replace", "", "replacement repository prefix")
	fs.StringVar(&archList, "archlist", "", "comma-separated architectures to include")
	fs.StringVar(&archList, "a", "", "comma-separated architectures to include")
	ignoredString(fs, "method")
	ignoredString(fs, "noarch-repo")
	fs.StringVar(&opts.GroupFile, "groupfile", "", "group metadata file")
	fs.StringVar(&opts.GroupFile, "g", "", "group metadata file")
	ignoredString(fs, "blocked")
	ignoredString(fs, "b")
	fs.BoolVar(&opts.Database, "database", false, "generate sqlite metadata")
	fs.BoolVar(&opts.Database, "d", false, "generate sqlite metadata")
	fs.BoolVar(&noDatabase, "no-database", false, "do not generate sqlite metadata")
	fs.BoolVar(&opts.FilelistsExt, "filelists-ext", false, "generate filelists-ext metadata")
	ignoredBool(fs, "verbose")
	ignoredBool(fs, "v")
	ignoredBool(fs, "nogroups")
	ignoredBool(fs, "noupdateinfo")
	fs.BoolVar(&useZchunk, "zck", false, "use zchunk compression")
	ignoredBool(fs, "all")
	fs.BoolVar(&opts.UniqueMDFilenames, "unique-md-filenames", true, "checksum-prefix metadata filenames")
	simpleNames := fs.Bool("simple-md-filenames", false, "do not checksum-prefix metadata filenames")
	fs.BoolVar(&opts.OmitBaseURL, "omit-baseurl", false, "omit package location base URLs")
	ignoredBool(fs, "koji")
	ignoredBool(fs, "k")
	ignoredBool(fs, "simple")
	ignoredBool(fs, "pkgorigins")
	ignoredBool(fs, "arch-expand")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if version {
		fmt.Fprintf(stdout, "mergerepo_c %s\n", cr.Version)
		return 0
	}
	opts.Repos = repos
	if useZchunk {
		compressionName = "zck"
	}
	opts.Compression = compressionFromFlag(compressionName)
	opts.SimpleMDFilenames = *simpleNames
	opts.ArchList = splitCommaList(archList)
	if noDatabase {
		opts.Database = false
	}
	if err := cr.Merge(ctx, opts); err != nil {
		return printError(stderr, false, false, err)
	}
	return 0
}

func RunSQLite(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var opts cr.SQLiteOptions
	var version, quiet, verbose, useXZ, useZchunk bool
	var checksumName, compressionName string
	fs := flag.NewFlagSet("sqliterepo_c", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&version, "version", false, "print version")
	fs.BoolVar(&version, "V", false, "print version")
	fs.BoolVar(&quiet, "quiet", false, "quiet output")
	fs.BoolVar(&quiet, "q", false, "quiet output")
	fs.BoolVar(&verbose, "verbose", false, "verbose output")
	fs.BoolVar(&verbose, "v", false, "verbose output")
	fs.BoolVar(&opts.Force, "force", false, "force generation")
	fs.BoolVar(&opts.Force, "f", false, "force generation")
	fs.BoolVar(&opts.KeepOld, "keep-old", false, "keep old sqlite metadata")
	fs.StringVar(&compressionName, "compress-type", "bz2", "sqlite compression")
	fs.StringVar(&checksumName, "checksum", "sha256", "checksum type")
	fs.BoolVar(&opts.LocalSQLite, "local-sqlite", false, "generate sqlite locally")
	fs.BoolVar(&useXZ, "xz", false, "use xz compression")
	fs.BoolVar(&useZchunk, "zck", false, "use zchunk compression")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if version {
		fmt.Fprintf(stdout, "sqliterepo_c %s\n", cr.Version)
		return 0
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Usage: sqliterepo_c [options] <repository>")
		return 2
	}
	opts.RepositoryPath = fs.Arg(0)
	opts.Checksum = cr.ChecksumTypeFromName(checksumName)
	if useXZ {
		compressionName = "xz"
	}
	if useZchunk {
		compressionName = "zck"
	}
	opts.Compression = compressionFromFlag(compressionName)
	if err := cr.GenerateSQLite(ctx, opts); err != nil {
		return printError(stderr, quiet, verbose, err)
	}
	return 0
}

func compressionFromFlag(name string) cr.CompressionType {
	if name == "" {
		return cr.CompressionUnknown
	}
	return cr.CompressionTypeFromName(name)
}

func checksumFromFlag(name string) cr.ChecksumType {
	if name == "" {
		return cr.ChecksumUnknown
	}
	return cr.ChecksumTypeFromName(name)
}

func splitCommaList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseDistroTags(values []string) []cr.DistroTag {
	out := make([]cr.DistroTag, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if cpeid, label, ok := strings.Cut(value, ","); ok {
			out = append(out, cr.DistroTag{CPEID: strings.TrimSpace(cpeid), Value: strings.TrimSpace(label)})
			continue
		}
		out = append(out, cr.DistroTag{Value: value})
	}
	return out
}

func hasLongFlag(args []string, name string) bool {
	prefix := "--" + name + "="
	exact := "--" + name
	for _, arg := range args {
		if arg == exact || strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func printError(stderr io.Writer, quiet, verbose bool, err error) int {
	if quiet {
		return 1
	}
	prefix := "Error"
	if verbose {
		fmt.Fprintf(stderr, "%s: %+v\n", prefix, err)
	} else {
		fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
	}
	return 1
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func ignoredString(fs *flag.FlagSet, name string) {
	var v string
	fs.StringVar(&v, name, "", "")
}

func ignoredStringArray(fs *flag.FlagSet, name string) {
	var v stringSlice
	fs.Var(&v, name, "")
}

func ignoredBool(fs *flag.FlagSet, name string) {
	var v bool
	fs.BoolVar(&v, name, false, "")
}

func ignoredInt(fs *flag.FlagSet, name string) {
	var v int
	fs.IntVar(&v, name, 0, "")
}

func Main(run func(context.Context, []string, io.Writer, io.Writer) int) {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
