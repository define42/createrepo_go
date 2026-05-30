package createrepo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseUpdateInfoFile(t *testing.T) {
	info, err := ParseUpdateInfoFile(fixturePath(t, "tests", "testdata", "updateinfo_files", "updateinfo_01.xml"))
	if err != nil {
		t.Fatalf("ParseUpdateInfoFile() error = %v", err)
	}
	if len(info.Updates) != 1 {
		t.Fatalf("len(Updates) = %d, want 1", len(info.Updates))
	}
	update := info.Updates[0]
	if update.ID != "foobarupdate_1" || !update.RebootSuggested {
		t.Fatalf("unexpected update = %#v", update)
	}
	if len(update.References) != 1 || update.References[0].Href != "https://foobar/foobarupdate_1" {
		t.Fatalf("references = %#v", update.References)
	}
	if len(update.Collections) != 1 || len(update.Collections[0].Packages) != 1 {
		t.Fatalf("collections = %#v", update.Collections)
	}
	pkg := update.Collections[0].Packages[0]
	if pkg.Name != "bar" || pkg.SumType != ChecksumSHA256 || !pkg.RestartSuggested || !pkg.ReloginSuggested {
		t.Fatalf("package = %#v", pkg)
	}
}

func TestParseUpdateInfoNestedZchunk(t *testing.T) {
	info, err := ParseUpdateInfoFile(fixturePath(t, "tests", "testdata", "repo_with_additional_metadata", "repodata", "0219a2f1f9f32af6b7873905269ac1bc27b03e0caf3968c929a49e5a939e8935-updateinfo_01.xml.gz.zck"))
	if err != nil {
		t.Fatalf("ParseUpdateInfoFile(.gz.zck) error = %v", err)
	}
	if len(info.Updates) != 1 || info.Updates[0].ID != "foobarupdate_1" {
		t.Fatalf("updates = %#v", info.Updates)
	}
}

func TestLoadMetadataLoadsUpdateInfo(t *testing.T) {
	repo, err := LoadMetadata(context.Background(), fixturePath(t, "tests", "testdata", "repo_with_additional_metadata"), LoadOptions{})
	if err != nil {
		t.Fatalf("LoadMetadata() error = %v", err)
	}
	if repo.UpdateInfo == nil || len(repo.UpdateInfo.Updates) != 1 {
		t.Fatalf("UpdateInfo = %#v", repo.UpdateInfo)
	}
}

func TestLoadMetadataRemote(t *testing.T) {
	server := httptest.NewServer(http.FileServer(http.Dir(fixturePath(t, "tests", "testdata"))))
	defer server.Close()

	repo, err := LoadMetadata(context.Background(), server.URL+"/repo_01", LoadOptions{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("LoadMetadata(remote) error = %v", err)
	}
	if len(repo.Packages) != 1 || repo.Packages[0].Name != "super_kernel" {
		t.Fatalf("packages = %#v", repo.Packages)
	}
	if repo.Path != server.URL+"/repo_01" {
		t.Fatalf("Path = %q", repo.Path)
	}
}
