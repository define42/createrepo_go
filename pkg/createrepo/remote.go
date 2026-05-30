package createrepo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func loadRemoteMetadata(ctx context.Context, rawURL string, opts LoadOptions) (*Repository, error) {
	repoURL, repomdURL, err := remoteRepositoryURLs(rawURL)
	if err != nil {
		return nil, err
	}
	cacheDir := opts.CacheDir
	if cacheDir == "" {
		cacheDir, err = os.MkdirTemp("", "createrepo_c-remote-*")
		if err != nil {
			return nil, err
		}
	}
	repomdPath := filepath.Join(cacheDir, "repodata", "repomd.xml")
	if err := downloadRemoteFile(ctx, repomdURL.String(), repomdPath); err != nil {
		return nil, err
	}

	repomd, err := ParseRepomdFile(repomdPath)
	if err != nil {
		return nil, err
	}
	for _, record := range repomd.Records {
		if record.LocationHref == "" {
			continue
		}
		dst, err := remoteCachePath(cacheDir, record.LocationHref)
		if err != nil {
			return nil, err
		}
		src, err := remoteRecordURL(repoURL, record.LocationHref)
		if err != nil {
			return nil, err
		}
		if err := downloadRemoteFile(ctx, src, dst); err != nil {
			return nil, err
		}
		if opts.VerifyChecksums {
			if err := verifyRemoteRecord(record, dst); err != nil {
				return nil, err
			}
		}
		record.LocationReal = dst
	}

	repo, err := loadLocalMetadata(ctx, cacheDir, opts)
	if err != nil {
		return nil, err
	}
	repo.Path = rawURL
	return repo, nil
}

func remoteRepositoryURLs(rawURL string) (*url.URL, *url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, nil, fmt.Errorf("unsupported remote scheme %q", parsed.Scheme)
	}
	repoURL := *parsed
	repomdURL := *parsed
	if strings.HasSuffix(parsed.Path, "/repodata/repomd.xml") {
		repoURL.Path = strings.TrimSuffix(parsed.Path, "/repodata/repomd.xml")
		if repoURL.Path == "" {
			repoURL.Path = "/"
		}
	} else {
		repoURL.Path = strings.TrimRight(repoURL.Path, "/") + "/"
		repomdURL = *repoURL.ResolveReference(&url.URL{Path: "repodata/repomd.xml"})
	}
	if !strings.HasSuffix(repoURL.Path, "/") {
		repoURL.Path += "/"
	}
	return &repoURL, &repomdURL, nil
}

func remoteRecordURL(repoURL *url.URL, href string) (string, error) {
	parsed, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		return parsed.String(), nil
	}
	cleanHref := path.Clean(strings.TrimPrefix(href, "/"))
	if cleanHref == "." || strings.HasPrefix(cleanHref, "../") {
		return "", fmt.Errorf("unsafe remote metadata href %q", href)
	}
	base := *repoURL
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	return base.ResolveReference(&url.URL{Path: path.Join(base.Path, cleanHref)}).String(), nil
}

func remoteCachePath(cacheDir, href string) (string, error) {
	parsed, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		href = parsed.Path
	}
	cleanHref := path.Clean(strings.TrimPrefix(href, "/"))
	if cleanHref == "." || strings.HasPrefix(cleanHref, "../") {
		return "", fmt.Errorf("unsafe metadata href %q", href)
	}
	return filepath.Join(cacheDir, filepath.FromSlash(cleanHref)), nil
}

func downloadRemoteFile(ctx context.Context, rawURL, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func verifyRemoteRecord(record *RepomdRecord, path string) error {
	if record.Checksum.Value == "" || record.Checksum.Type == "" {
		return nil
	}
	checksumType := ChecksumTypeFromName(record.Checksum.Type)
	if checksumType == ChecksumUnknown {
		return nil
	}
	sum, err := ChecksumFile(path, checksumType)
	if err != nil {
		return err
	}
	if sum != record.Checksum.Value {
		return fmt.Errorf("%s checksum mismatch: got %s, want %s", record.LocationHref, sum, record.Checksum.Value)
	}
	return nil
}
