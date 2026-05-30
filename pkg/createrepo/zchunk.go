package createrepo

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

const (
	zchunkChecksumSHA1      = 0
	zchunkChecksumSHA256    = 1
	zchunkChecksumSHA512    = 2
	zchunkChecksumSHA512128 = 3

	zchunkCompressionNone = 0
	zchunkCompressionZstd = 2

	zchunkFlagDataStreams        = 1 << 0
	zchunkFlagOptionalElements   = 1 << 1
	zchunkFlagUncompressedSource = 1 << 2
)

var zchunkMagic = []byte{0x00, 'Z', 'C', 'K', '1'}

type zchunkFile struct {
	payload            []byte
	headerSize         int64
	headerChecksumType ChecksumType
	headerChecksum     string
}

type zchunkChunk struct {
	stream             uint64
	checksum           []byte
	compressedLength   uint64
	uncompressedLength uint64
}

func openZchunkReader(filename string) (io.ReadCloser, error) {
	zck, err := readZchunkFile(filename)
	if err != nil {
		return nil, err
	}
	return openReaderBytes("", zck.payload)
}

func zchunkContentStat(filename string, checksumType ChecksumType) (*ContentStat, error) {
	zck, err := readZchunkFile(filename)
	if err != nil {
		return nil, err
	}
	sum, size, err := ChecksumReader(bytes.NewReader(zck.payload), checksumType)
	if err != nil {
		return nil, err
	}
	return &ContentStat{
		Size:               size,
		ChecksumType:       checksumType,
		Checksum:           sum,
		HeaderSize:         zck.headerSize,
		HeaderChecksumType: zck.headerChecksumType,
		HeaderChecksum:     zck.headerChecksum,
	}, nil
}

func readZchunkFile(filename string) (*zchunkFile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if len(data) < len(zchunkMagic) || !bytes.Equal(data[:len(zchunkMagic)], zchunkMagic) {
		return nil, fmt.Errorf("invalid zchunk magic in %s", filename)
	}

	pos := len(zchunkMagic)
	overallChecksumID, err := readZchunkInt(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("zchunk checksum type: %w", err)
	}
	overallChecksumType, err := zchunkOverallChecksumType(overallChecksumID)
	if err != nil {
		return nil, err
	}
	overallChecksumLen, err := zchunkChecksumLen(overallChecksumID)
	if err != nil {
		return nil, err
	}
	headerBodySize, err := readZchunkInt(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("zchunk header size: %w", err)
	}
	headerChecksumStart := pos
	headerChecksumEnd := pos + overallChecksumLen
	if headerChecksumEnd > len(data) {
		return nil, fmt.Errorf("truncated zchunk header checksum")
	}
	headerBodyStart := headerChecksumEnd
	if headerBodySize > uint64(len(data)-headerBodyStart) {
		return nil, fmt.Errorf("truncated zchunk header")
	}
	headerEnd := headerBodyStart + int(headerBodySize)
	storedHeaderChecksum := data[headerChecksumStart:headerChecksumEnd]
	calculatedHeaderChecksum, err := zchunkHashBytes(overallChecksumID, append(append([]byte{}, data[:headerChecksumStart]...), data[headerChecksumEnd:headerEnd]...))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(storedHeaderChecksum, calculatedHeaderChecksum) {
		return nil, fmt.Errorf("zchunk header checksum mismatch")
	}

	pos = headerBodyStart
	if pos+overallChecksumLen > headerEnd {
		return nil, fmt.Errorf("truncated zchunk data checksum")
	}
	storedDataChecksum := data[pos : pos+overallChecksumLen]
	pos += overallChecksumLen
	flags, err := readZchunkInt(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("zchunk flags: %w", err)
	}
	if flags&^(zchunkFlagDataStreams|zchunkFlagOptionalElements|zchunkFlagUncompressedSource) != 0 {
		return nil, fmt.Errorf("%w: zchunk flags 0x%x", ErrUnsupportedCompression, flags)
	}
	compressionID, err := readZchunkInt(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("zchunk compression type: %w", err)
	}
	if compressionID != zchunkCompressionNone && compressionID != zchunkCompressionZstd {
		return nil, fmt.Errorf("%w: zchunk compression type %d", ErrUnsupportedCompression, compressionID)
	}
	if flags&zchunkFlagOptionalElements != 0 {
		count, err := readZchunkInt(data, &pos)
		if err != nil {
			return nil, fmt.Errorf("zchunk optional count: %w", err)
		}
		for i := uint64(0); i < count; i++ {
			if _, err := readZchunkInt(data, &pos); err != nil {
				return nil, fmt.Errorf("zchunk optional id: %w", err)
			}
			size, err := readZchunkInt(data, &pos)
			if err != nil {
				return nil, fmt.Errorf("zchunk optional size: %w", err)
			}
			if size > uint64(headerEnd-pos) {
				return nil, fmt.Errorf("truncated zchunk optional data")
			}
			pos += int(size)
		}
	}

	indexSize, err := readZchunkInt(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("zchunk index size: %w", err)
	}
	indexStart := pos
	chunkChecksumID, err := readZchunkInt(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("zchunk chunk checksum type: %w", err)
	}
	chunkChecksumLen, err := zchunkChecksumLen(chunkChecksumID)
	if err != nil {
		return nil, err
	}
	chunkCount, err := readZchunkInt(data, &pos)
	if err != nil {
		return nil, fmt.Errorf("zchunk chunk count: %w", err)
	}
	if chunkCount == 0 {
		return nil, fmt.Errorf("zchunk index has no dictionary entry")
	}

	dict, err := readZchunkChunkIndex(data, &pos, headerEnd, flags, chunkChecksumLen)
	if err != nil {
		return nil, fmt.Errorf("zchunk dictionary index: %w", err)
	}
	chunks := []zchunkChunk{}
	for i := uint64(1); i < chunkCount; i++ {
		chunk, err := readZchunkChunkIndex(data, &pos, headerEnd, flags, chunkChecksumLen)
		if err != nil {
			return nil, fmt.Errorf("zchunk chunk index %d: %w", i, err)
		}
		chunks = append(chunks, chunk)
	}
	if uint64(pos-indexStart) != indexSize {
		return nil, fmt.Errorf("zchunk index size mismatch")
	}
	if err := skipZchunkSignatures(data, &pos, headerEnd); err != nil {
		return nil, err
	}
	if pos != headerEnd {
		return nil, fmt.Errorf("zchunk header trailing data")
	}

	body := data[headerEnd:]
	calculatedDataChecksum, err := zchunkHashBytes(overallChecksumID, body)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(storedDataChecksum, calculatedDataChecksum) {
		return nil, fmt.Errorf("zchunk data checksum mismatch")
	}

	payload, err := decodeZchunkBody(body, dict, chunks, compressionID, chunkChecksumID, flags)
	if err != nil {
		return nil, err
	}
	return &zchunkFile{
		payload:            payload,
		headerSize:         int64(headerEnd),
		headerChecksumType: overallChecksumType,
		headerChecksum:     hex.EncodeToString(storedHeaderChecksum),
	}, nil
}

func readZchunkChunkIndex(data []byte, pos *int, limit int, flags uint64, checksumLen int) (zchunkChunk, error) {
	var chunk zchunkChunk
	var err error
	if flags&zchunkFlagDataStreams != 0 {
		chunk.stream, err = readZchunkInt(data, pos)
		if err != nil {
			return chunk, err
		}
	}
	if *pos+checksumLen > limit {
		return chunk, fmt.Errorf("truncated checksum")
	}
	chunk.checksum = append([]byte(nil), data[*pos:*pos+checksumLen]...)
	*pos += checksumLen
	if flags&zchunkFlagUncompressedSource != 0 {
		if *pos+checksumLen > limit {
			return chunk, fmt.Errorf("truncated uncompressed checksum")
		}
		*pos += checksumLen
	}
	chunk.compressedLength, err = readZchunkInt(data, pos)
	if err != nil {
		return chunk, err
	}
	chunk.uncompressedLength, err = readZchunkInt(data, pos)
	return chunk, err
}

func skipZchunkSignatures(data []byte, pos *int, limit int) error {
	count, err := readZchunkInt(data, pos)
	if err != nil {
		return fmt.Errorf("zchunk signature count: %w", err)
	}
	for i := uint64(0); i < count; i++ {
		if _, err := readZchunkInt(data, pos); err != nil {
			return fmt.Errorf("zchunk signature type: %w", err)
		}
		size, err := readZchunkInt(data, pos)
		if err != nil {
			return fmt.Errorf("zchunk signature size: %w", err)
		}
		if size > uint64(limit-*pos) {
			return fmt.Errorf("truncated zchunk signature")
		}
		*pos += int(size)
	}
	return nil
}

func decodeZchunkBody(body []byte, dict zchunkChunk, chunks []zchunkChunk, compressionID, chunkChecksumID, flags uint64) ([]byte, error) {
	pos := 0
	dictBytes, err := readZchunkBodySlice(body, &pos, dict.compressedLength)
	if err != nil {
		return nil, fmt.Errorf("zchunk dictionary body: %w", err)
	}
	if len(dictBytes) > 0 {
		if err := verifyZchunkChunkChecksum(chunkChecksumID, dictBytes, dict.checksum); err != nil {
			return nil, fmt.Errorf("zchunk dictionary checksum: %w", err)
		}
	}
	var decoderOptions []zstd.DOption
	if dict.compressedLength > 0 {
		decodedDict, err := decodeZstdFrame(dictBytes, nil)
		if err != nil {
			return nil, fmt.Errorf("zchunk dictionary decode: %w", err)
		}
		if uint64(len(decodedDict)) != dict.uncompressedLength {
			return nil, fmt.Errorf("zchunk dictionary length mismatch")
		}
		decoderOptions = append(decoderOptions, zstd.WithDecoderDicts(decodedDict))
	}

	var payload bytes.Buffer
	for i, chunk := range chunks {
		chunkBytes, err := readZchunkBodySlice(body, &pos, chunk.compressedLength)
		if err != nil {
			return nil, fmt.Errorf("zchunk chunk %d body: %w", i, err)
		}
		if err := verifyZchunkChunkChecksum(chunkChecksumID, chunkBytes, chunk.checksum); err != nil {
			return nil, fmt.Errorf("zchunk chunk %d checksum: %w", i, err)
		}
		var decoded []byte
		switch {
		case compressionID == zchunkCompressionNone:
			decoded = chunkBytes
		case flags&zchunkFlagUncompressedSource != 0 && zchunkZeroChecksum(chunk.checksum):
			decoded = chunkBytes
		default:
			decoded, err = decodeZstdFrame(chunkBytes, decoderOptions)
			if err != nil {
				return nil, fmt.Errorf("zchunk chunk %d decode: %w", i, err)
			}
		}
		if uint64(len(decoded)) != chunk.uncompressedLength {
			return nil, fmt.Errorf("zchunk chunk %d length mismatch", i)
		}
		payload.Write(decoded)
	}
	if pos != len(body) {
		return nil, fmt.Errorf("zchunk body trailing data")
	}
	return payload.Bytes(), nil
}

func readZchunkBodySlice(body []byte, pos *int, length uint64) ([]byte, error) {
	if length > uint64(len(body)-*pos) {
		return nil, fmt.Errorf("truncated body")
	}
	start := *pos
	*pos += int(length)
	return body[start:*pos], nil
}

func decodeZstdFrame(input []byte, options []zstd.DOption) ([]byte, error) {
	decoder, err := zstd.NewReader(bytes.NewReader(input), options...)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	return io.ReadAll(decoder)
}

func writeZchunkFile(path string, payload []byte) error {
	var compressed bytes.Buffer
	encoder, err := zstd.NewWriter(&compressed)
	if err != nil {
		return err
	}
	if _, err := encoder.Write(payload); err != nil {
		encoder.Close()
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}

	body := compressed.Bytes()
	chunkChecksum, err := zchunkHashBytes(zchunkChecksumSHA256, body)
	if err != nil {
		return err
	}
	dataChecksum, err := zchunkHashBytes(zchunkChecksumSHA256, body)
	if err != nil {
		return err
	}

	var index bytes.Buffer
	index.Write(encodeZchunkInt(zchunkChecksumSHA256))
	index.Write(encodeZchunkInt(2))
	index.Write(make([]byte, sha256.Size))
	index.Write(encodeZchunkInt(0))
	index.Write(encodeZchunkInt(0))
	index.Write(chunkChecksum)
	index.Write(encodeZchunkInt(uint64(len(body))))
	index.Write(encodeZchunkInt(uint64(len(payload))))

	var headerBody bytes.Buffer
	headerBody.Write(dataChecksum)
	headerBody.Write(encodeZchunkInt(0))
	headerBody.Write(encodeZchunkInt(zchunkCompressionZstd))
	headerBody.Write(encodeZchunkInt(uint64(index.Len())))
	headerBody.Write(index.Bytes())
	headerBody.Write(encodeZchunkInt(0))

	prefix := append(append([]byte{}, zchunkMagic...), encodeZchunkInt(zchunkChecksumSHA256)...)
	prefix = append(prefix, encodeZchunkInt(uint64(headerBody.Len()))...)
	headerChecksumInput := append(append([]byte{}, prefix...), headerBody.Bytes()...)
	headerChecksum, err := zchunkHashBytes(zchunkChecksumSHA256, headerChecksumInput)
	if err != nil {
		return err
	}

	var out bytes.Buffer
	out.Write(prefix)
	out.Write(headerChecksum)
	out.Write(headerBody.Bytes())
	out.Write(body)
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func readZchunkInt(data []byte, pos *int) (uint64, error) {
	var value uint64
	var shift uint
	for {
		if *pos >= len(data) {
			return 0, io.ErrUnexpectedEOF
		}
		b := data[*pos]
		*pos = *pos + 1
		if shift >= 64 {
			return 0, fmt.Errorf("zchunk integer overflows uint64")
		}
		value |= uint64(b&0x7f) << shift
		if b&0x80 != 0 {
			return value, nil
		}
		shift += 7
	}
}

func encodeZchunkInt(value uint64) []byte {
	var out []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value == 0 {
			out = append(out, b|0x80)
			return out
		}
		out = append(out, b)
	}
}

func zchunkOverallChecksumType(id uint64) (ChecksumType, error) {
	switch id {
	case zchunkChecksumSHA1:
		return ChecksumSHA1, nil
	case zchunkChecksumSHA256:
		return ChecksumSHA256, nil
	default:
		return ChecksumUnknown, fmt.Errorf("%w: zchunk checksum type %d", ErrUnsupportedCompression, id)
	}
}

func zchunkChecksumLen(id uint64) (int, error) {
	switch id {
	case zchunkChecksumSHA1:
		return sha1.Size, nil
	case zchunkChecksumSHA256:
		return sha256.Size, nil
	case zchunkChecksumSHA512:
		return sha512.Size, nil
	case zchunkChecksumSHA512128:
		return sha512.Size / 4, nil
	default:
		return 0, fmt.Errorf("%w: zchunk checksum type %d", ErrUnsupportedCompression, id)
	}
}

func zchunkHashBytes(id uint64, data []byte) ([]byte, error) {
	switch id {
	case zchunkChecksumSHA1:
		sum := sha1.Sum(data)
		return sum[:], nil
	case zchunkChecksumSHA256:
		sum := sha256.Sum256(data)
		return sum[:], nil
	case zchunkChecksumSHA512:
		sum := sha512.Sum512(data)
		return sum[:], nil
	case zchunkChecksumSHA512128:
		sum := sha512.Sum512(data)
		return sum[:sha512.Size/4], nil
	default:
		return nil, fmt.Errorf("%w: zchunk checksum type %d", ErrUnsupportedCompression, id)
	}
}

func verifyZchunkChunkChecksum(id uint64, data, expected []byte) error {
	if zchunkZeroChecksum(expected) {
		return nil
	}
	got, err := zchunkHashBytes(id, data)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, expected) {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}

func zchunkZeroChecksum(sum []byte) bool {
	for _, b := range sum {
		if b != 0 {
			return false
		}
	}
	return true
}
