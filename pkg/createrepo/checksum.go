package createrepo

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// ChecksumType identifies a checksum algorithm used in RPM repository
// metadata.
type ChecksumType int

const (
	// ChecksumUnknown represents an unset or unsupported checksum algorithm.
	ChecksumUnknown ChecksumType = iota
	// ChecksumMD5 represents the legacy MD5 metadata checksum.
	ChecksumMD5
	// ChecksumSHA represents the legacy SHA-1 compatibility checksum name.
	ChecksumSHA
	// ChecksumSHA1 represents SHA-1 metadata checksums.
	ChecksumSHA1
	// ChecksumSHA224 represents SHA-224 metadata checksums.
	ChecksumSHA224
	// ChecksumSHA256 represents SHA-256 metadata checksums.
	ChecksumSHA256
	// ChecksumSHA384 represents SHA-384 metadata checksums.
	ChecksumSHA384
	// ChecksumSHA512 represents SHA-512 metadata checksums.
	ChecksumSHA512
)

// ChecksumTypeFromName returns the checksum type for a metadata checksum name.
// "sha" is kept as a SHA-1 compatibility alias, matching createrepo_c.
func ChecksumTypeFromName(name string) ChecksumType {
	switch strings.ToLower(name) {
	case "md5":
		return ChecksumMD5
	case "sha":
		return ChecksumSHA
	case "sha1":
		return ChecksumSHA1
	case "sha224":
		return ChecksumSHA224
	case "sha256":
		return ChecksumSHA256
	case "sha384":
		return ChecksumSHA384
	case "sha512":
		return ChecksumSHA512
	default:
		return ChecksumUnknown
	}
}

// Name returns the canonical metadata name for a checksum type.
func (t ChecksumType) Name() (string, bool) {
	switch t {
	case ChecksumUnknown:
		return "", false
	case ChecksumMD5:
		return "md5", true
	case ChecksumSHA:
		return "sha", true
	case ChecksumSHA1:
		return "sha1", true
	case ChecksumSHA224:
		return "sha224", true
	case ChecksumSHA256:
		return "sha256", true
	case ChecksumSHA384:
		return "sha384", true
	case ChecksumSHA512:
		return "sha512", true
	default:
		return "", false
	}
}

func (t ChecksumType) String() string {
	name, ok := t.Name()
	if !ok {
		return "unknown"
	}
	return name
}

// NewHash creates a streaming hash.Hash for a checksum type.
func (t ChecksumType) NewHash() (hash.Hash, error) {
	switch t {
	case ChecksumUnknown:
		return nil, fmt.Errorf("unknown checksum type: %d", t)
	case ChecksumMD5:
		return md5.New(), nil
	case ChecksumSHA, ChecksumSHA1:
		return sha1.New(), nil
	case ChecksumSHA224:
		return sha256.New224(), nil
	case ChecksumSHA256:
		return sha256.New(), nil
	case ChecksumSHA384:
		return sha512.New384(), nil
	case ChecksumSHA512:
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("unknown checksum type: %d", t)
	}
}

// ChecksumReader consumes r and returns the hex checksum plus the number of
// bytes read.
func ChecksumReader(r io.Reader, checksumType ChecksumType) (string, int64, error) {
	h, err := checksumType.NewHash()
	if err != nil {
		return "", 0, err
	}

	n, err := io.Copy(h, r)
	if err != nil {
		return "", n, err
	}

	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// ChecksumFile calculates a file checksum.
func ChecksumFile(filename string, checksumType ChecksumType) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sum, _, err := ChecksumReader(f, checksumType)
	return sum, err
}
