package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cr "github.com/rpm-software-management/createrepo_c/pkg/createrepo"
)

func TestRunCreateVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCreate(context.Background(), []string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if stdout.String() != "createrepo_c "+cr.Version+"\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCreateEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunCreate(context.Background(), []string{"--revision", "1700000000", "--compress-type", "gz", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if _, err := cr.ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml")); err != nil {
		t.Fatalf("repomd was not created: %v", err)
	}
}

func TestRunCreateZchunkFlag(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunCreate(context.Background(), []string{"--revision", "1700000000", "--zck", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	md, err := cr.ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if primary := md.Record("primary"); primary == nil || !strings.HasSuffix(primary.LocationHref, ".zck") {
		t.Fatalf("primary record = %#v", primary)
	}
}

func TestRunCreateDatabaseFlag(t *testing.T) {
	dir := t.TempDir()
	rpmName := "super_kernel-6.0.1-2.x86_64.rpm"
	if err := copyTestFile(fixturePath(t, "tests", "testdata", "packages", rpmName), filepath.Join(dir, rpmName)); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunCreate(context.Background(), []string{"--revision", "1700000000", "--database", "--compress-type", "gz", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	md, err := cr.ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if md.Record("primary_db") == nil {
		t.Fatal("primary_db record missing")
	}
}

func TestRunCreateDeltasExplicit(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunCreate(context.Background(), []string{"--deltas", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if _, err := cr.ParseRepomdFile(filepath.Join(dir, "repodata", "repomd.xml")); err != nil {
		t.Fatalf("repomd was not created: %v", err)
	}
}

func TestRunModifyVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunModify(context.Background(), []string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if stdout.String() != "modifyrepo_c "+cr.Version+"\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunMergeInvalidRepos(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunMerge(context.Background(), []string{"--repo", "a", "--repo", "b"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
}

func TestRunSQLiteInvalidRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunSQLite(context.Background(), []string{t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
}

func copyTestFile(src, dst string) error {
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

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	if _, err := os.Stat(path); err == nil {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		return abs
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	t.Fatalf("fixture not found: %s", path)
	return ""
}
