package createrepo

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectCompression(t *testing.T) {
	tests := []struct {
		path string
		want CompressionType
	}{
		{fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt"), CompressionNone},
		{fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.gz"), CompressionGzip},
		{fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.bz2"), CompressionBzip2},
		{fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.xz"), CompressionXZ},
		{fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.zst"), CompressionZstd},
		{fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.zck"), CompressionZchunk},
	}

	for _, tt := range tests {
		got, err := DetectCompression(tt.path)
		if err != nil {
			t.Fatalf("DetectCompression(%s) error = %v", tt.path, err)
		}
		if got != tt.want {
			t.Fatalf("DetectCompression(%s) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestOpenReader(t *testing.T) {
	plainPath := fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt")
	want, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		plainPath,
		fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.gz"),
		fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.bz2"),
		fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.xz"),
		fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.zst"),
		fixturePath(t, "tests", "testdata", "compressed_files", "01_plain.txt.zck"),
	} {
		t.Run(path, func(t *testing.T) {
			r, err := OpenReader(path, CompressionAuto)
			if err != nil {
				t.Fatalf("OpenReader() error = %v", err)
			}
			defer r.Close()

			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if string(got) != string(want) {
				t.Fatalf("OpenReader() read %q, want %q", got, want)
			}
		})
	}
}

func TestOpenReaderEmptyZchunk(t *testing.T) {
	r, err := OpenReader(fixturePath(t, "tests", "testdata", "compressed_files", "00_plain.txt.zck"), CompressionAuto)
	if err != nil {
		t.Fatalf("OpenReader(empty zchunk) error = %v", err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty zchunk read %q", got)
	}
}

func TestOpenReaderAllZchunkFixtures(t *testing.T) {
	root := fixturePath(t, "tests", "testdata")
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".zck") {
			return nil
		}
		r, err := OpenReader(path, CompressionAuto)
		if err != nil {
			t.Fatalf("OpenReader(%s) error = %v", path, err)
		}
		got, err := io.ReadAll(r)
		closeErr := r.Close()
		if err != nil {
			t.Fatalf("ReadAll(%s) error = %v", path, err)
		}
		if closeErr != nil {
			t.Fatalf("Close(%s) error = %v", path, closeErr)
		}
		if len(got) == 0 && filepath.Base(path) != "00_plain.txt.zck" {
			t.Fatalf("OpenReader(%s) returned empty payload", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCompressedContentStat(t *testing.T) {
	stat, err := CompressedContentStat(fixturePath(t, "tests", "testdata", "repo_00", "repodata", "1cb61ea996355add02b1426ed4c1780ea75ce0c04c5d1107c025c3fbd7d8bcae-primary.xml.gz"), ChecksumSHA256)
	if err != nil {
		t.Fatalf("CompressedContentStat() error = %v", err)
	}
	if stat.Size != 167 {
		t.Fatalf("CompressedContentStat().Size = %d, want 167", stat.Size)
	}
	if stat.Checksum != "e1e2ffd2fb1ee76f87b70750d00ca5677a252b397ab6c2389137a0c33e7b359f" {
		t.Fatalf("CompressedContentStat().Checksum = %s", stat.Checksum)
	}
}

func TestCompressedContentStatZchunk(t *testing.T) {
	stat, err := CompressedContentStat(fixturePath(t, "tests", "testdata", "repo_00", "repodata", "e0ac03cd77e95e724dbf90ded0dba664e233315a8940051dd8882c56b9878595-primary.xml.zck"), ChecksumSHA256)
	if err != nil {
		t.Fatalf("CompressedContentStat(zchunk) error = %v", err)
	}
	if stat.Size != 167 {
		t.Fatalf("Size = %d, want 167", stat.Size)
	}
	if stat.Checksum != "e1e2ffd2fb1ee76f87b70750d00ca5677a252b397ab6c2389137a0c33e7b359f" {
		t.Fatalf("Checksum = %s", stat.Checksum)
	}
	if stat.HeaderSize != 132 {
		t.Fatalf("HeaderSize = %d, want 132", stat.HeaderSize)
	}
	if stat.HeaderChecksum != "243baf7c02f5241d46f2e8c237ebc7ea7e257ca993d9cfe1304254c7ba7f6546" {
		t.Fatalf("HeaderChecksum = %s", stat.HeaderChecksum)
	}
}

func TestWriteCompressedFileZchunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "primary.xml.zck")
	want := []byte("<metadata packages=\"0\"/>\n")
	if err := WriteCompressedFile(path, want, CompressionZchunk); err != nil {
		t.Fatalf("WriteCompressedFile(zchunk) error = %v", err)
	}
	r, err := OpenReader(path, CompressionAuto)
	if err != nil {
		t.Fatalf("OpenReader(zchunk) error = %v", err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("round trip = %q, want %q", got, want)
	}

	record := NewRepomdRecord("primary_zck", path)
	if err := record.Fill(ChecksumSHA256); err != nil {
		t.Fatalf("Fill(zchunk) error = %v", err)
	}
	if record.HeaderChecksum.Value == "" || record.HeaderSize <= 0 {
		t.Fatalf("missing zchunk header stats: %#v size %d", record.HeaderChecksum, record.HeaderSize)
	}
}
