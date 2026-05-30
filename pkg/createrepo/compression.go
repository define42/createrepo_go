package createrepo

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// CompressionType identifies a metadata compression format.
type CompressionType int

const (
	CompressionAuto CompressionType = iota
	CompressionUnknown
	CompressionNone
	CompressionGzip
	CompressionBzip2
	CompressionXZ
	CompressionZchunk
	CompressionZstd
)

var ErrUnsupportedCompression = errors.New("unsupported compression")

func (t CompressionType) String() string {
	switch t {
	case CompressionAuto:
		return "auto"
	case CompressionNone:
		return "none"
	case CompressionGzip:
		return "gz"
	case CompressionBzip2:
		return "bz2"
	case CompressionXZ:
		return "xz"
	case CompressionZchunk:
		return "zck"
	case CompressionZstd:
		return "zstd"
	default:
		return "unknown"
	}
}

// CompressionTypeFromName returns a compression type for a common CLI/API name.
func CompressionTypeFromName(name string) CompressionType {
	switch strings.ToLower(name) {
	case "gz", "gzip":
		return CompressionGzip
	case "bz2", "bzip2":
		return CompressionBzip2
	case "xz":
		return CompressionXZ
	case "zck":
		return CompressionZchunk
	case "zstd", "zst":
		return CompressionZstd
	default:
		return CompressionUnknown
	}
}

// Suffix returns the conventional filename suffix for a compression type.
func (t CompressionType) Suffix() string {
	switch t {
	case CompressionGzip:
		return ".gz"
	case CompressionBzip2:
		return ".bz2"
	case CompressionXZ:
		return ".xz"
	case CompressionZchunk:
		return ".zck"
	case CompressionZstd:
		return ".zst"
	default:
		return ""
	}
}

// DetectCompression mirrors createrepo_c's suffix-first detection with magic
// byte fallback.
func DetectCompression(filename string) (CompressionType, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return CompressionUnknown, err
	}
	if !info.Mode().IsRegular() {
		return CompressionUnknown, fmt.Errorf("%s is not a regular file", filename)
	}

	switch {
	case strings.HasSuffix(filename, ".gz"),
		strings.HasSuffix(filename, ".gzip"),
		strings.HasSuffix(filename, ".gunzip"):
		return CompressionGzip, nil
	case strings.HasSuffix(filename, ".bz2"),
		strings.HasSuffix(filename, ".bzip2"):
		return CompressionBzip2, nil
	case strings.HasSuffix(filename, ".xz"):
		return CompressionXZ, nil
	case strings.HasSuffix(filename, ".zck"):
		return CompressionZchunk, nil
	case strings.HasSuffix(filename, ".zst"):
		return CompressionZstd, nil
	case strings.HasSuffix(filename, ".xml"),
		strings.HasSuffix(filename, ".tar"),
		strings.HasSuffix(filename, ".yaml"),
		strings.HasSuffix(filename, ".sqlite"),
		strings.HasSuffix(filename, ".txt"):
		return CompressionNone, nil
	}

	f, err := os.Open(filename)
	if err != nil {
		return CompressionUnknown, err
	}
	defer f.Close()

	magic := make([]byte, 5)
	n, err := io.ReadFull(f, magic)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return CompressionUnknown, err
	}
	if n < len(magic) {
		return CompressionNone, nil
	}

	switch {
	case string(magic[:2]) == "\x1f\x8b":
		return CompressionGzip, nil
	case string(magic[:4]) == "\x28\xb5\x2f\xfd":
		return CompressionZstd, nil
	case string(magic[:2]) == "\x42\x5a":
		return CompressionBzip2, nil
	case string(magic) == "\xfd\x37\x7a\x58\x5a":
		return CompressionXZ, nil
	case string(magic) == "\x00ZCK1":
		return CompressionZchunk, nil
	}

	if strings.Count(filepath.Base(filename), ".") >= 2 {
		return CompressionUnknown, nil
	}
	return CompressionNone, nil
}

func detectCompressionBytes(filename string, data []byte) CompressionType {
	switch {
	case strings.HasSuffix(filename, ".gz"),
		strings.HasSuffix(filename, ".gzip"),
		strings.HasSuffix(filename, ".gunzip"):
		return CompressionGzip
	case strings.HasSuffix(filename, ".bz2"),
		strings.HasSuffix(filename, ".bzip2"):
		return CompressionBzip2
	case strings.HasSuffix(filename, ".xz"):
		return CompressionXZ
	case strings.HasSuffix(filename, ".zck"):
		return CompressionZchunk
	case strings.HasSuffix(filename, ".zst"):
		return CompressionZstd
	}
	if len(data) >= 5 {
		switch {
		case string(data[:2]) == "\x1f\x8b":
			return CompressionGzip
		case string(data[:4]) == "\x28\xb5\x2f\xfd":
			return CompressionZstd
		case string(data[:2]) == "\x42\x5a":
			return CompressionBzip2
		case string(data[:5]) == "\xfd\x37\x7a\x58\x5a":
			return CompressionXZ
		case string(data[:5]) == "\x00ZCK1":
			return CompressionZchunk
		}
	}
	return CompressionNone
}

func openReaderBytes(filename string, data []byte) (io.ReadCloser, error) {
	compression := detectCompressionBytes(filename, data)
	r := bytes.NewReader(data)
	switch compression {
	case CompressionNone:
		return io.NopCloser(r), nil
	case CompressionGzip:
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		return gz, nil
	case CompressionBzip2:
		return io.NopCloser(bzip2.NewReader(r)), nil
	case CompressionXZ:
		xzr, err := xz.NewReader(r)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(xzr), nil
	case CompressionZstd:
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return zr.IOReadCloser(), nil
	case CompressionZchunk:
		return nil, fmt.Errorf("%w: nested zchunk", ErrUnsupportedCompression)
	default:
		return nil, fmt.Errorf("unknown compression type: %d", compression)
	}
}

// OpenReader opens filename for uncompressed reads.
func OpenReader(filename string, compression CompressionType) (io.ReadCloser, error) {
	if compression == CompressionAuto {
		detected, err := DetectCompression(filename)
		if err != nil {
			return nil, err
		}
		compression = detected
	}
	if compression == CompressionUnknown {
		return nil, fmt.Errorf("cannot detect compression type for %s", filename)
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	switch compression {
	case CompressionNone:
		return f, nil
	case CompressionGzip:
		r, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &compoundReadCloser{Reader: r, closers: []io.Closer{r, f}}, nil
	case CompressionBzip2:
		return &compoundReadCloser{Reader: bzip2.NewReader(f), closers: []io.Closer{f}}, nil
	case CompressionXZ:
		r, err := xz.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &compoundReadCloser{Reader: r, closers: []io.Closer{f}}, nil
	case CompressionZstd:
		r, err := zstd.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &compoundReadCloser{Reader: r, closers: []io.Closer{closeFunc(r.Close), f}}, nil
	case CompressionZchunk:
		f.Close()
		return openZchunkReader(filename)
	default:
		f.Close()
		return nil, fmt.Errorf("unknown compression type: %d", compression)
	}
}

type closeFunc func()

func (f closeFunc) Close() error {
	f()
	return nil
}

type compoundReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (r *compoundReadCloser) Close() error {
	var firstErr error
	for _, closer := range r.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ContentStat describes the uncompressed content of a metadata file.
type ContentStat struct {
	Size               int64
	ChecksumType       ChecksumType
	Checksum           string
	HeaderSize         int64
	HeaderChecksumType ChecksumType
	HeaderChecksum     string
}

// CompressedContentStat reads a compressed metadata file and calculates stats
// for its uncompressed payload.
func CompressedContentStat(filename string, checksumType ChecksumType) (*ContentStat, error) {
	compression, err := DetectCompression(filename)
	if err != nil {
		return nil, err
	}
	if compression == CompressionZchunk {
		return zchunkContentStat(filename, checksumType)
	}

	r, err := OpenReader(filename, CompressionAuto)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	sum, size, err := ChecksumReader(r, checksumType)
	if err != nil {
		return nil, err
	}

	return &ContentStat{
		Size:               size,
		ChecksumType:       checksumType,
		Checksum:           sum,
		HeaderSize:         -1,
		HeaderChecksumType: ChecksumUnknown,
	}, nil
}
