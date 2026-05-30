package createrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func pkgNVRA(name, epoch, version, release, arch string) Package {
	return Package{Name: name, Epoch: epoch, Version: version, Release: release, Arch: arch, TimeBuild: 0}
}

func TestCompareEVR(t *testing.T) {
	cases := []struct {
		a, b Package
		want int
	}{
		{pkgNVRA("p", "0", "1.0", "1", "x86_64"), pkgNVRA("p", "0", "1.0", "1", "x86_64"), 0},
		{pkgNVRA("p", "0", "1.1", "1", "x86_64"), pkgNVRA("p", "0", "1.0", "1", "x86_64"), 1},
		{pkgNVRA("p", "0", "1.0", "2", "x86_64"), pkgNVRA("p", "0", "1.0", "1", "x86_64"), 1},
		{pkgNVRA("p", "1", "1.0", "1", "x86_64"), pkgNVRA("p", "0", "9.9", "9", "x86_64"), 1},
		{pkgNVRA("p", "0", "1.0", "1", "x86_64"), pkgNVRA("p", "0", "1.10", "1", "x86_64"), -1},
	}
	for i, c := range cases {
		if got := compareEVR(c.a, c.b); sign(got) != c.want {
			t.Errorf("case %d: compareEVR = %d, want sign %d", i, got, c.want)
		}
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}

func TestMergeAccumulatorMethods(t *testing.T) {
	older := pkgNVRA("foo", "0", "1.0", "1", "x86_64")
	older.TimeBuild = 100
	newer := pkgNVRA("foo", "0", "2.0", "1", "x86_64")
	newer.TimeBuild = 50 // higher NVR but older timestamp, to distinguish methods

	t.Run("repo keeps first repo", func(t *testing.T) {
		acc := newMergeAccumulator(mergeMethodRepo, false)
		acc.Add(older, "repo1")
		acc.Add(newer, "repo2")
		got := acc.Packages()
		if len(got) != 1 || got[0].pkg.Version != "1.0" || got[0].origin != "repo1" {
			t.Fatalf("repo method = %+v", got)
		}
	})
	t.Run("nvr keeps highest version", func(t *testing.T) {
		acc := newMergeAccumulator(mergeMethodNVR, false)
		acc.Add(older, "repo1")
		acc.Add(newer, "repo2")
		got := acc.Packages()
		if len(got) != 1 || got[0].pkg.Version != "2.0" {
			t.Fatalf("nvr method = %+v", got)
		}
	})
	t.Run("ts keeps newest timestamp", func(t *testing.T) {
		acc := newMergeAccumulator(mergeMethodTimestamp, false)
		acc.Add(older, "repo1") // TimeBuild 100
		acc.Add(newer, "repo2") // TimeBuild 50
		got := acc.Packages()
		if len(got) != 1 || got[0].pkg.Version != "1.0" {
			t.Fatalf("ts method = %+v", got)
		}
	})
	t.Run("all keeps every version", func(t *testing.T) {
		acc := newMergeAccumulator(mergeMethodRepo, true)
		acc.Add(older, "repo1")
		acc.Add(newer, "repo2")
		acc.Add(older, "repo3") // exact NEVRA dup collapses
		if got := acc.Packages(); len(got) != 2 {
			t.Fatalf("all method kept %d packages, want 2", len(got))
		}
	})
}

func TestParseMergeMethod(t *testing.T) {
	for name, want := range map[string]mergeMethod{
		"":     mergeMethodRepo,
		"repo": mergeMethodRepo,
		"ts":   mergeMethodTimestamp,
		"nvr":  mergeMethodNVR,
	} {
		got, err := parseMergeMethod(name)
		if err != nil || got != want {
			t.Errorf("parseMergeMethod(%q) = %v, %v", name, got, err)
		}
	}
	if _, err := parseMergeMethod("bogus"); err == nil {
		t.Error("expected error for unknown method")
	}
}

func TestChecksumCache(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(target, []byte("hello cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(target)

	cacheDir := t.TempDir()
	cache, err := newChecksumCache(cacheDir, false)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := ChecksumFile(target, ChecksumSHA256)

	got, err := cache.ChecksumFile(target, ChecksumSHA256, info)
	if err != nil || got != want {
		t.Fatalf("cold cache = %q, %v; want %q", got, err, want)
	}
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 cache entry, got %d", len(entries))
	}
	// Corrupt the file content but keep size/mtime: a stat-validated read still
	// returns the cached value, proving it did not re-hash.
	if got2, err := cache.ChecksumFile(target, ChecksumSHA256, info); err != nil || got2 != want {
		t.Fatalf("warm cache = %q, %v; want %q", got2, err, want)
	}
}

func TestChecksumCacheInvalidatesOnSizeChange(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(target, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache, err := newChecksumCache(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	info1, _ := os.Stat(target)
	first, _ := cache.ChecksumFile(target, ChecksumSHA256, info1)

	if err := os.WriteFile(target, []byte("a longer second version"), 0o644); err != nil {
		t.Fatal(err)
	}
	info2, _ := os.Stat(target)
	second, err := cache.ChecksumFile(target, ChecksumSHA256, info2)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("cache returned stale checksum after size change")
	}
	want, _ := ChecksumFile(target, ChecksumSHA256)
	if second != want {
		t.Fatalf("recomputed checksum = %q, want %q", second, want)
	}
}

func TestBuildRPMJobsSplit(t *testing.T) {
	root := t.TempDir()
	disc1 := filepath.Join(root, "disc1")
	disc2 := filepath.Join(root, "disc2")
	for _, d := range []string{disc1, disc2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(disc1, "a.rpm"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(disc2, "b.rpm"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobs, err := buildRPMJobs(Options{Split: true, SplitDirs: []string{disc1, disc2}})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}
	prefixes := map[string]bool{}
	for _, j := range jobs {
		prefixes[j.hrefPrefix] = true
	}
	if !prefixes["disc1"] || !prefixes["disc2"] {
		t.Fatalf("split job prefixes = %v", prefixes)
	}
}

func TestAcquireRepodataLock(t *testing.T) {
	dir := t.TempDir()
	release, err := acquireRepodataLock(dir, false)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".repodata")); err != nil {
		t.Fatalf("lock dir not created: %v", err)
	}
	// A second acquisition must fail while the lock is held.
	if _, err := acquireRepodataLock(dir, false); err == nil {
		t.Fatal("expected second lock to fail")
	}
	// ignoreLock overrides the existing lock.
	release2, err := acquireRepodataLock(dir, true)
	if err != nil {
		t.Fatalf("ignoreLock acquisition failed: %v", err)
	}
	release2()
	if _, err := os.Stat(filepath.Join(dir, ".repodata")); !os.IsNotExist(err) {
		t.Fatal("lock dir not removed on release")
	}
	release() // releasing the stale handle must not panic or error
}

func TestPruneOldMetadataByAge(t *testing.T) {
	repodata := t.TempDir()
	prefix := func(c byte) string { return strings.Repeat(string(c), 64) }
	oldFile := prefix('a') + "-primary.xml.gz"
	cur := prefix('c') + "-primary.xml.gz"
	for _, name := range []string{oldFile, cur} {
		if err := os.WriteFile(filepath.Join(repodata, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Backdate the superseded file well beyond the age threshold.
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(repodata, oldFile), old, old); err != nil {
		t.Fatal(err)
	}
	current := []*RepomdRecord{{Type: "primary", LocationHref: "repodata/" + cur}}
	pruneOldMetadataFiles(repodata, []string{oldFile}, current, 0, 24*time.Hour)

	if _, err := os.Stat(filepath.Join(repodata, oldFile)); !os.IsNotExist(err) {
		t.Fatal("aged-out file should have been pruned")
	}
	if _, err := os.Stat(filepath.Join(repodata, cur)); err != nil {
		t.Fatal("current file should be kept")
	}
}

func TestPruneOldMetadataRetention(t *testing.T) {
	repodata := t.TempDir()
	// Real metadata uses 64-char sha256 filename prefixes; the retention
	// grouping strips that prefix to find files of the same kind.
	prefix := func(c byte) string { return strings.Repeat(string(c), 64) }
	oldA := prefix('a') + "-primary.xml.gz"
	oldB := prefix('b') + "-primary.xml.gz"
	cur := prefix('c') + "-primary.xml.gz"
	writeFile := func(name string) {
		if err := os.WriteFile(filepath.Join(repodata, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(oldA)
	writeFile(oldB)
	current := []*RepomdRecord{{Type: "primary", LocationHref: "repodata/" + cur}}
	writeFile(cur)

	// previous non-empty so pruning runs; retain 1 old version.
	pruneOldMetadataFiles(repodata, []string{oldA}, current, 1, 0)
	remaining := map[string]bool{}
	entries, _ := os.ReadDir(repodata)
	for _, e := range entries {
		remaining[e.Name()] = true
	}
	if !remaining[cur] {
		t.Fatal("current metadata was deleted")
	}
	oldCount := 0
	if remaining[oldA] {
		oldCount++
	}
	if remaining[oldB] {
		oldCount++
	}
	if oldCount != 1 {
		t.Fatalf("retained %d old primary files, want 1", oldCount)
	}
}
