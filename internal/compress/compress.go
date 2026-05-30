package compress

import (
	"io"

	cr "github.com/rpm-software-management/createrepo_c/pkg/createrepo"
)

type Type = cr.CompressionType

const (
	Auto    = cr.CompressionAuto
	Unknown = cr.CompressionUnknown
	None    = cr.CompressionNone
	Gzip    = cr.CompressionGzip
	Bzip2   = cr.CompressionBzip2
	XZ      = cr.CompressionXZ
	Zchunk  = cr.CompressionZchunk
	Zstd    = cr.CompressionZstd
)

func Detect(filename string) (Type, error) {
	return cr.DetectCompression(filename)
}

func OpenReader(filename string, compression Type) (io.ReadCloser, error) {
	return cr.OpenReader(filename, compression)
}

func WriteFile(filename string, data []byte, compression Type) error {
	return cr.WriteCompressedFile(filename, data, compression)
}
