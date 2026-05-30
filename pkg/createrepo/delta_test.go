package createrepo

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateDeltaRPMCreatesRPMOnlyDeltaFromDifferentRPMs(t *testing.T) {
	newRPM := fixturePath(t, "tests", "testdata", "packages", "super_kernel-6.0.1-2.x86_64.rpm")
	oldRPM := fixturePath(t, "tests", "testdata", "packages", "Archer-3.4.5-6.x86_64.rpm")
	out := filepath.Join(t.TempDir(), "super_kernel.drpm")

	err := GenerateDeltaRPM(context.Background(), DeltaOptions{
		OldRPM:     oldRPM,
		NewRPM:     newRPM,
		OutputPath: out,
	})
	if err != nil {
		t.Fatalf("GenerateDeltaRPM() error = %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte("drpmDLT3")) {
		t.Fatalf("delta magic = %x", raw[:min(len(raw), 8)])
	}
	reconstructed := applyRPMOnlyDeltaForTest(t, raw)
	want, err := os.ReadFile(newRPM)
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(oldRPM)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(old, want) {
		t.Fatal("old and new RPM fixtures unexpectedly match")
	}
	if !bytes.Equal(reconstructed, want) {
		t.Fatalf("reconstructed RPM differs from target: got %d bytes want %d", len(reconstructed), len(want))
	}
}

func applyRPMOnlyDeltaForTest(t *testing.T, raw []byte) []byte {
	t.Helper()
	r := bytes.NewReader(raw)
	magic := readBytesForTest(t, r, 4)
	if string(magic) != "drpm" {
		t.Fatalf("magic = %q", magic)
	}
	if version := string(readBytesForTest(t, r, 4)); version != "DLT3" {
		t.Fatalf("rpm-only version = %q", version)
	}
	_ = readCStringForTest(t, r)
	addLen := readBE32ForTest(t, r)
	if addLen != 0 {
		t.Fatalf("rpm-only add data length = %d", addLen)
	}
	if version := string(readBytesForTest(t, r, 4)); version != "DLT3" {
		t.Fatalf("common version = %q", version)
	}
	_ = readCStringForTest(t, r)
	sequenceLen := readBE32ForTest(t, r)
	if sequenceLen != md5.Size {
		t.Fatalf("sequence length = %d", sequenceLen)
	}
	_ = readBytesForTest(t, r, int(sequenceLen))
	targetMD5 := readBytesForTest(t, r, md5.Size)
	targetSize := readBE32ForTest(t, r)
	targetComp := readBE32ForTest(t, r)
	if targetComp != 0 {
		t.Fatalf("target compression = %d", targetComp)
	}
	compParamLen := readBE32ForTest(t, r)
	if compParamLen != 0 {
		t.Fatalf("target compression parameter length = %d", compParamLen)
	}
	headerLen := readBE32ForTest(t, r)
	if headerLen == 0 {
		t.Fatal("target header length is zero")
	}
	offAdjCount := readBE32ForTest(t, r)
	if offAdjCount != 0 {
		t.Fatalf("offset adjustment count = %d", offAdjCount)
	}
	leadSigLen := readBE32ForTest(t, r)
	leadSig := readBytesForTest(t, r, int(leadSigLen))
	_ = readBE32ForTest(t, r) // payload format offset
	intCopies := readBE32ForTest(t, r)
	extCopies := readBE32ForTest(t, r)
	if intCopies != 1 || extCopies != 0 {
		t.Fatalf("copy counts = internal %d external %d", intCopies, extCopies)
	}
	extBefore := readBE32ForTest(t, r)
	intCopyLen := readBE32ForTest(t, r)
	if extBefore != 0 {
		t.Fatalf("external copies before internal copy = %d", extBefore)
	}
	_ = readBE64ForTest(t, r) // external data length
	commonAddLen := readBE32ForTest(t, r)
	if commonAddLen != 0 {
		t.Fatalf("common add data length = %d", commonAddLen)
	}
	intDataLen := readBE64ForTest(t, r)
	if intDataLen != uint64(intCopyLen) {
		t.Fatalf("internal data length = %d, copy length = %d", intDataLen, intCopyLen)
	}
	intData := readBytesForTest(t, r, int(intDataLen))
	if r.Len() != 0 {
		t.Fatalf("trailing bytes = %d", r.Len())
	}
	out := append(append([]byte{}, leadSig...), intData...)
	if uint32(len(out)) != targetSize {
		t.Fatalf("target size = %d, reconstructed = %d", targetSize, len(out))
	}
	sum := md5.Sum(out)
	if !bytes.Equal(sum[:], targetMD5) {
		t.Fatal("target MD5 does not match reconstructed RPM")
	}
	return out
}

func readCStringForTest(t *testing.T, r *bytes.Reader) string {
	t.Helper()
	n := readBE32ForTest(t, r)
	value := readBytesForTest(t, r, int(n))
	if len(value) == 0 || value[len(value)-1] != 0 {
		t.Fatalf("string is not NUL-terminated: %q", value)
	}
	return string(value[:len(value)-1])
}

func readBytesForTest(t *testing.T, r *bytes.Reader, n int) []byte {
	t.Helper()
	out := make([]byte, n)
	if _, err := io.ReadFull(r, out); err != nil {
		t.Fatal(err)
	}
	return out
}

func readBE32ForTest(t *testing.T, r *bytes.Reader) uint32 {
	t.Helper()
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		t.Fatal(err)
	}
	return binary.BigEndian.Uint32(buf[:])
}

func readBE64ForTest(t *testing.T, r *bytes.Reader) uint64 {
	t.Helper()
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		t.Fatal(err)
	}
	return binary.BigEndian.Uint64(buf[:])
}
