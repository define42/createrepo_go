// Package compress exposes compatibility wrappers for metadata compression.
package compress

import (
	"io"

	cr "github.com/define42/createrepo_go/pkg/createrepo"
)

// Type identifies a metadata compression format.
type Type = cr.CompressionType

// Compression aliases exported by the compatibility package.
const (
	// Auto detects compression from file suffix or magic bytes.
	Auto    = cr.CompressionAuto
	Unknown = cr.CompressionUnknown
	None    = cr.CompressionNone
	Gzip    = cr.CompressionGzip
	Bzip2   = cr.CompressionBzip2
	XZ      = cr.CompressionXZ
	Zchunk  = cr.CompressionZchunk
	Zstd    = cr.CompressionZstd
)

// Detect returns the compression type for filename.
func Detect(filename string) (Type, error) {
	return cr.DetectCompression(filename)
}

// OpenReader opens filename for uncompressed reads.
func OpenReader(filename string, compression Type) (io.ReadCloser, error) {
	return cr.OpenReader(filename, compression)
}

// WriteFile writes data using the requested compression type.
func WriteFile(filename string, data []byte, compression Type) error {
	return cr.WriteCompressedFile(filename, data, compression)
}
