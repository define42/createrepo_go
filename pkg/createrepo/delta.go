package createrepo

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	rpmfile "github.com/cavaliergopher/rpm"
)

const (
	defaultNumDeltas       = 1
	defaultMaxDeltaRPMSize = int64(100000000)
	rpmSigTagMD5           = 1004
)

type deltaGenerationOptions struct {
	OldPackageDirs  []string
	NumDeltas       int
	MaxDeltaRPMSize int64
	Checksum        ChecksumType
}

type deltaCandidate struct {
	Path string
	RPM  *rpmfile.Package
	Size int64
}

type prestoDelta struct {
	NewPackage Package
	OldEpoch   string
	OldVersion string
	OldRelease string
	OldNEVR    string
	Sequence   string
	Filename   string
	Size       int64
	Checksum   string
	ChecksumTy string
}

func generateDeltaMetadata(ctx context.Context, outputDir string, targets []parsedPackage, opts deltaGenerationOptions) ([]metadataItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(opts.OldPackageDirs) == 0 || len(targets) == 0 {
		return nil, nil
	}
	if opts.NumDeltas == 0 {
		opts.NumDeltas = defaultNumDeltas
	}
	if opts.NumDeltas < 0 {
		return nil, fmt.Errorf("num-deltas must be non-negative")
	}
	if opts.NumDeltas == 0 {
		return nil, nil
	}
	if opts.MaxDeltaRPMSize == 0 {
		opts.MaxDeltaRPMSize = defaultMaxDeltaRPMSize
	}
	if opts.MaxDeltaRPMSize < 0 {
		return nil, fmt.Errorf("max-delta-rpm-size must be non-negative")
	}
	if opts.Checksum == ChecksumUnknown {
		opts.Checksum = ChecksumSHA256
	}
	checksumName, ok := opts.Checksum.Name()
	if !ok {
		return nil, fmt.Errorf("unknown checksum type: %d", opts.Checksum)
	}

	oldCandidates, err := scanOldDeltaCandidates(opts.OldPackageDirs, opts.MaxDeltaRPMSize)
	if err != nil {
		return nil, err
	}
	if len(oldCandidates) == 0 {
		return nil, nil
	}

	drpmsDir := filepath.Join(outputDir, "drpms")
	if err := os.MkdirAll(drpmsDir, 0o755); err != nil {
		return nil, err
	}

	var deltas []prestoDelta
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if opts.MaxDeltaRPMSize > 0 && target.Package.SizeInstalled >= opts.MaxDeltaRPMSize {
			continue
		}
		targetRPM, err := rpmfile.Open(target.Path)
		if err != nil {
			return nil, err
		}
		key := deltaCandidateKey(targetRPM)
		candidates := oldCandidates[key]
		candidates = filterOlderDeltaCandidates(targetRPM, candidates)
		if len(candidates) == 0 {
			continue
		}
		sort.Slice(candidates, func(i, j int) bool {
			return rpmfile.Compare(candidates[i].RPM, candidates[j].RPM) > 0
		})
		if len(candidates) > opts.NumDeltas {
			candidates = candidates[:opts.NumDeltas]
		}
		for _, old := range candidates {
			name := deltaFilename(old.RPM, targetRPM)
			outPath := filepath.Join(drpmsDir, name)
			if err := writeRPMOnlyDelta(old.Path, target.Path, outPath); err != nil {
				return nil, err
			}
			info, err := os.Stat(outPath)
			if err != nil {
				return nil, err
			}
			sum, err := ChecksumFile(outPath, opts.Checksum)
			if err != nil {
				return nil, err
			}
			seq, err := rpmSignatureMD5(old.RPM)
			if err != nil {
				return nil, err
			}
			deltas = append(deltas, prestoDelta{
				NewPackage: target.Package,
				OldEpoch:   strconv.Itoa(old.RPM.Epoch()),
				OldVersion: old.RPM.Version(),
				OldRelease: old.RPM.Release(),
				OldNEVR:    rpmNEVR(old.RPM),
				Sequence:   hex.EncodeToString(seq),
				Filename:   filepath.ToSlash(filepath.Join("drpms", name)),
				Size:       info.Size(),
				Checksum:   sum,
				ChecksumTy: checksumName,
			})
		}
	}
	if len(deltas) == 0 {
		return nil, nil
	}
	sort.Slice(deltas, func(i, j int) bool {
		a, b := deltas[i], deltas[j]
		if a.NewPackage.Name != b.NewPackage.Name {
			return a.NewPackage.Name < b.NewPackage.Name
		}
		if a.NewPackage.Arch != b.NewPackage.Arch {
			return a.NewPackage.Arch < b.NewPackage.Arch
		}
		if a.NewPackage.Epoch != b.NewPackage.Epoch {
			return a.NewPackage.Epoch < b.NewPackage.Epoch
		}
		if a.NewPackage.Version != b.NewPackage.Version {
			return a.NewPackage.Version < b.NewPackage.Version
		}
		if a.NewPackage.Release != b.NewPackage.Release {
			return a.NewPackage.Release < b.NewPackage.Release
		}
		return a.OldNEVR < b.OldNEVR
	})
	return []metadataItem{{Type: "prestodelta", Name: "prestodelta.xml", Body: DumpPrestoDelta(deltas)}}, nil
}

func scanOldDeltaCandidates(dirs []string, maxSize int64) (map[string][]deltaCandidate, error) {
	out := map[string][]deltaCandidate{}
	for _, dir := range dirs {
		rpms, err := findRPMs(dir, false)
		if err != nil {
			return nil, err
		}
		for _, rpmPath := range rpms {
			info, err := os.Stat(rpmPath)
			if err != nil {
				continue
			}
			if maxSize > 0 && info.Size() > maxSize {
				continue
			}
			rp, err := rpmfile.Open(rpmPath)
			if err != nil {
				continue
			}
			key := deltaCandidateKey(rp)
			out[key] = append(out[key], deltaCandidate{Path: rpmPath, RPM: rp, Size: info.Size()})
		}
	}
	return out, nil
}

func filterOlderDeltaCandidates(target *rpmfile.Package, candidates []deltaCandidate) []deltaCandidate {
	filtered := make([]deltaCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if rpmfile.Compare(target, candidate.RPM) <= 0 {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func deltaCandidateKey(rp *rpmfile.Package) string {
	return rp.Name() + "\x00" + rp.Architecture()
}

func deltaFilename(oldRPM, newRPM *rpmfile.Package) string {
	parts := []string{
		oldRPM.Name(),
		oldRPM.Version(),
		oldRPM.Release() + "_" + newRPM.Version(),
		newRPM.Release() + "." + oldRPM.Architecture() + ".drpm",
	}
	return sanitizeDeltaFilename(strings.Join(parts, "-"))
}

func sanitizeDeltaFilename(name string) string {
	return strings.NewReplacer("/", "_", string(os.PathSeparator), "_").Replace(name)
}

// DumpPrestoDelta serializes delta RPM metadata as prestodelta.xml.
func DumpPrestoDelta(deltas []prestoDelta) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString("<prestodelta>\n")
	for i := 0; i < len(deltas); {
		pkg := deltas[i].NewPackage
		b.WriteString(`  <newpackage name="`)
		xml.EscapeText(&b, []byte(pkg.Name))
		b.WriteString(`" epoch="`)
		xml.EscapeText(&b, []byte(defaultString(pkg.Epoch, "0")))
		b.WriteString(`" version="`)
		xml.EscapeText(&b, []byte(pkg.Version))
		b.WriteString(`" release="`)
		xml.EscapeText(&b, []byte(pkg.Release))
		b.WriteString(`" arch="`)
		xml.EscapeText(&b, []byte(pkg.Arch))
		b.WriteString("\">\n")
		for ; i < len(deltas) && samePrestoNewPackage(pkg, deltas[i].NewPackage); i++ {
			writePrestoDelta(&b, deltas[i])
		}
		b.WriteString("  </newpackage>\n")
	}
	b.WriteString("</prestodelta>\n")
	return b.Bytes()
}

func samePrestoNewPackage(a, b Package) bool {
	return a.Name == b.Name && a.Arch == b.Arch && a.Epoch == b.Epoch && a.Version == b.Version && a.Release == b.Release
}

func writePrestoDelta(b *bytes.Buffer, delta prestoDelta) {
	b.WriteString(`    <delta oldepoch="`)
	xml.EscapeText(b, []byte(defaultString(delta.OldEpoch, "0")))
	b.WriteString(`" oldversion="`)
	xml.EscapeText(b, []byte(delta.OldVersion))
	b.WriteString(`" oldrelease="`)
	xml.EscapeText(b, []byte(delta.OldRelease))
	b.WriteString("\">\n")
	writeSimpleElement(b, 6, "filename", delta.Filename)
	writeSimpleElement(b, 6, "sequence", delta.OldNEVR+"-"+delta.Sequence)
	writeSimpleElement(b, 6, "size", strconv.FormatInt(delta.Size, 10))
	b.WriteString(`      <checksum type="`)
	xml.EscapeText(b, []byte(delta.ChecksumTy))
	b.WriteString(`">`)
	xml.EscapeText(b, []byte(delta.Checksum))
	b.WriteString("</checksum>\n")
	b.WriteString("    </delta>\n")
}

func writeRPMOnlyDelta(oldRPMPath, newRPMPath, outputPath string) error {
	oldRPM, err := rpmfile.Open(oldRPMPath)
	if err != nil {
		return err
	}
	newRPM, err := rpmfile.Open(newRPMPath)
	if err != nil {
		return err
	}
	oldSigMD5, err := rpmSignatureMD5(oldRPM)
	if err != nil {
		return err
	}
	newBytes, err := os.ReadFile(newRPMPath)
	if err != nil {
		return err
	}
	headerStart, headerEnd := newRPM.HeaderRange()
	if headerStart <= 0 || headerStart > len(newBytes) || headerEnd < headerStart || headerEnd > len(newBytes) {
		return fmt.Errorf("invalid RPM header range %d:%d for %s", headerStart, headerEnd, newRPMPath)
	}
	if len(newBytes) > int(^uint32(0)) {
		return fmt.Errorf("target RPM is too large for delta RPM v3: %s", newRPMPath)
	}
	targetBody := newBytes[headerStart:]
	if len(targetBody) > int(^uint32(0)) {
		return fmt.Errorf("target RPM body is too large for delta RPM v3: %s", newRPMPath)
	}
	targetMD5 := md5.Sum(newBytes)
	targetLeadSig := newBytes[:headerStart]

	var common bytes.Buffer
	common.WriteString("DLT3")
	writeBE32(&common, uint32(len(rpmNEVR(oldRPM))+1))
	common.WriteString(rpmNEVR(oldRPM))
	common.WriteByte(0)
	writeBE32(&common, uint32(len(oldSigMD5)))
	common.Write(oldSigMD5)
	common.Write(targetMD5[:])
	writeBE32(&common, uint32(len(newBytes)))
	writeBE32(&common, 0) // target compression: none; target body is stored byte-for-byte.
	writeBE32(&common, 0) // target compression parameter length
	writeBE32(&common, uint32(headerEnd-headerStart))
	writeBE32(&common, 0) // offset adjustment elements
	writeBE32(&common, uint32(len(targetLeadSig)))
	common.Write(targetLeadSig)
	writeBE32(&common, 0) // payload format offset; unused by rpm-only application.
	writeBE32(&common, 1) // one internal copy
	writeBE32(&common, 0) // no external copies
	writeBE32(&common, 0) // internal copy: external copies before this copy
	writeBE32(&common, uint32(len(targetBody)))
	writeBE64(&common, 1) // non-zero for readers that allocate by external-data block count.
	writeBE32(&common, 0) // add data length inside the common stream
	writeBE64(&common, uint64(len(targetBody)))
	common.Write(targetBody)

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	var out bytes.Buffer
	out.WriteString("drpm")
	out.WriteString("DLT3")
	writeBE32(&out, uint32(len(rpmNEVR(newRPM))+1))
	out.WriteString(rpmNEVR(newRPM))
	out.WriteByte(0)
	writeBE32(&out, 0) // rpm-only add data
	out.Write(common.Bytes())
	return os.WriteFile(outputPath, out.Bytes(), 0o644)
}

func rpmSignatureMD5(rp *rpmfile.Package) ([]byte, error) {
	tag := rp.Signature.GetTag(rpmSigTagMD5)
	if tag == nil {
		return nil, fmt.Errorf("RPM signature MD5 tag is missing")
	}
	value := tag.Bytes()
	if len(value) != md5.Size {
		return nil, fmt.Errorf("RPM signature MD5 has length %d, want %d", len(value), md5.Size)
	}
	return value, nil
}

func rpmNEVR(rp *rpmfile.Package) string {
	if rp.Epoch() > 0 {
		return fmt.Sprintf("%s-%d:%s-%s", rp.Name(), rp.Epoch(), rp.Version(), rp.Release())
	}
	return fmt.Sprintf("%s-%s-%s", rp.Name(), rp.Version(), rp.Release())
}

func writeBE32(b *bytes.Buffer, value uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], value)
	b.Write(buf[:])
}

func writeBE64(b *bytes.Buffer, value uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	b.Write(buf[:])
}
