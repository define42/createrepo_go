package createrepo

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"

	dsbzip2 "github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// WriteCompressedFile writes data using a supported compression type.
func WriteCompressedFile(path string, data []byte, compression CompressionType) error {
	if compression == CompressionAuto || compression == CompressionUnknown {
		compression = CompressionZstd
	}

	var encoded bytes.Buffer
	switch compression {
	case CompressionNone:
		encoded.Write(data)
	case CompressionGzip:
		w := gzip.NewWriter(&encoded)
		if _, err := w.Write(data); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
	case CompressionXZ:
		w, err := xz.NewWriter(&encoded)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
	case CompressionZstd:
		w, err := zstd.NewWriter(&encoded)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			w.Close()
			return err
		}
		w.Close()
	case CompressionBzip2:
		w, err := dsbzip2.NewWriter(&encoded, &dsbzip2.WriterConfig{Level: 5})
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			w.Close()
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
	case CompressionZchunk:
		return writeZchunkFile(path, data)
	default:
		return fmt.Errorf("unknown compression type: %d", compression)
	}

	return os.WriteFile(path, encoded.Bytes(), 0o644)
}
