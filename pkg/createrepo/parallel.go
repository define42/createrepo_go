package createrepo

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// rpmJob describes a single RPM to parse together with the location settings
// that produce its repository href. In split media mode each job may carry a
// different volume prefix and location root.
type rpmJob struct {
	path       string
	root       string
	hrefPrefix string
}

// parsePackagesParallel parses the given RPM jobs using up to workers
// goroutines and returns the results in the same order as jobs. A workers
// value <= 0 selects a default based on the number of CPUs.
func parsePackagesParallel(ctx context.Context, jobs []rpmJob, opts Options, cache *checksumCache) ([]parsedPackage, error) {
	if len(jobs) == 0 {
		return nil, nil
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}

	results := make([]parsedPackage, len(jobs))
	errs := make([]error, len(jobs))
	skipped := make([]bool, len(jobs))
	indexes := make(chan int)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexes {
				if ctx.Err() != nil {
					errs[idx] = ctx.Err()
					continue
				}
				pkg, err := parseRPMJob(jobs[idx], opts, cache)
				if err != nil {
					// Context cancellation is always fatal; a per-package
					// failure is only skipped when SkipErrors is set.
					if opts.SkipErrors && ctx.Err() == nil {
						if opts.PackageErrorHandler != nil {
							opts.PackageErrorHandler(jobs[idx].path, err)
						}
						skipped[idx] = true
						continue
					}
					errs[idx] = err
					continue
				}
				results[idx] = parsedPackage{Package: pkg, Path: jobs[idx].path}
			}
		}()
	}

	for i := range jobs {
		if ctx.Err() != nil {
			break
		}
		indexes <- i
	}
	close(indexes)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	if !sliceHasTrue(skipped) {
		return results, nil
	}
	kept := results[:0]
	for i := range results {
		if !skipped[i] {
			kept = append(kept, results[i])
		}
	}
	return kept, nil
}

func sliceHasTrue(values []bool) bool {
	for _, v := range values {
		if v {
			return true
		}
	}
	return false
}

func parseRPMJob(job rpmJob, opts Options, cache *checksumCache) (Package, error) {
	pkg, err := parseRPMPackageCached(job.path, job.root, opts.Checksum, cache)
	if err != nil {
		return Package{}, err
	}
	href := applyLocationOptions(pkg.LocationHref, opts.CutDirs, opts.LocationPrefix)
	if job.hrefPrefix != "" {
		href = strings.TrimRight(job.hrefPrefix, "/") + "/" + strings.TrimLeft(href, "/")
	}
	pkg.LocationHref = href
	pkg.LocationBase = opts.BaseURL
	if opts.ChangelogLimit > 0 && len(pkg.Changelogs) > opts.ChangelogLimit {
		pkg.Changelogs = pkg.Changelogs[:opts.ChangelogLimit]
	}
	return pkg, nil
}

// buildRPMJobs enumerates the RPMs to include, honoring split media mode. In
// normal mode it scans opts.Directory; in split mode it scans each split
// directory and prefixes hrefs with the volume directory name.
func buildRPMJobs(opts Options) ([]rpmJob, error) {
	var jobs []rpmJob
	if opts.Split {
		for _, dir := range opts.SplitDirs {
			rpms, err := findRPMs(dir, opts.SkipSymlinks)
			if err != nil {
				return nil, err
			}
			rpms, err = filterRPMs(rpms, dir, opts.Excludes, opts.IncludePackages, opts.PackageListFiles)
			if err != nil {
				return nil, err
			}
			prefix := filepath.Base(filepath.Clean(dir))
			for _, rpmPath := range rpms {
				jobs = append(jobs, rpmJob{path: rpmPath, root: dir, hrefPrefix: prefix})
			}
		}
		return jobs, nil
	}

	rpms, err := findRPMs(opts.Directory, opts.SkipSymlinks)
	if err != nil {
		return nil, err
	}
	rpms, err = filterRPMs(rpms, opts.Directory, opts.Excludes, opts.IncludePackages, opts.PackageListFiles)
	if err != nil {
		return nil, err
	}
	locationRoot := opts.Directory
	if opts.BaseDir != "" {
		locationRoot = opts.BaseDir
	}
	for _, rpmPath := range rpms {
		jobs = append(jobs, rpmJob{path: rpmPath, root: locationRoot})
	}
	return jobs, nil
}

// checksumCache caches expensive package checksums on disk keyed by file path,
// size, and modification time, so repeated runs (typically with --update) do
// not re-hash unchanged packages. It is safe for concurrent use because each
// package maps to a distinct cache file.
type checksumCache struct {
	dir      string
	skipStat bool
	mu       sync.Mutex
}

func newChecksumCache(dir string, skipStat bool) (*checksumCache, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &checksumCache{dir: dir, skipStat: skipStat}, nil
}

func (c *checksumCache) entryPath(path string, checksumType ChecksumType) string {
	sum := sha1.Sum([]byte(strconv.Itoa(int(checksumType)) + "\x00" + path))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:]))
}

// ChecksumFile returns the cached checksum for path, computing and storing it
// on a cache miss. The cache entry is validated against the file size and
// modification time unless skip-stat is set.
func (c *checksumCache) ChecksumFile(path string, checksumType ChecksumType, info os.FileInfo) (string, error) {
	if c == nil {
		return ChecksumFile(path, checksumType)
	}
	entry := c.entryPath(path, checksumType)

	c.mu.Lock()
	cached, ok := c.read(entry, info)
	c.mu.Unlock()
	if ok {
		return cached, nil
	}

	sum, err := ChecksumFile(path, checksumType)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.write(entry, info, sum)
	c.mu.Unlock()
	return sum, nil
}

func (c *checksumCache) read(entry string, info os.FileInfo) (string, bool) {
	body, err := os.ReadFile(entry)
	if err != nil {
		return "", false
	}
	fields := strings.SplitN(strings.TrimSpace(string(body)), " ", 3)
	if len(fields) != 3 {
		return "", false
	}
	if !c.skipStat {
		if fields[0] != strconv.FormatInt(info.Size(), 10) || fields[1] != strconv.FormatInt(info.ModTime().Unix(), 10) {
			return "", false
		}
	}
	return fields[2], true
}

func (c *checksumCache) write(entry string, info os.FileInfo, sum string) {
	content := fmt.Sprintf("%d %d %s\n", info.Size(), info.ModTime().Unix(), sum)
	_ = os.WriteFile(entry, []byte(content), 0o644)
}
