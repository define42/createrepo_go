package createrepo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
)

// DumpRepomd serializes repomd metadata in the canonical createrepo_c element
// order used by this Go port.
func DumpRepomd(md *Repomd) string {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<repomd xmlns="http://linux.duke.edu/metadata/repo" xmlns:rpm="http://linux.duke.edu/metadata/rpm">` + "\n")
	if md.Revision != "" {
		writeSimpleElement(&b, 2, "revision", md.Revision)
	}
	if md.RepoID.Value != "" {
		writeChecksumElement(&b, 2, "repoid", md.RepoID)
	}
	if md.ContentHash.Value != "" {
		writeChecksumElement(&b, 2, "contenthash", md.ContentHash)
	}
	if len(md.RepoTags) > 0 || len(md.ContentTags) > 0 || len(md.DistroTags) > 0 {
		b.WriteString("  <tags>\n")
		for _, tag := range md.RepoTags {
			writeSimpleElement(&b, 4, "repo", tag)
		}
		for _, tag := range md.ContentTags {
			writeSimpleElement(&b, 4, "content", tag)
		}
		for _, tag := range md.DistroTags {
			if tag.CPEID == "" {
				writeSimpleElement(&b, 4, "distro", tag.Value)
				continue
			}
			b.WriteString(`    <distro cpeid="`)
			xml.EscapeText(&b, []byte(tag.CPEID))
			b.WriteString(`">`)
			xml.EscapeText(&b, []byte(tag.Value))
			b.WriteString("</distro>\n")
		}
		b.WriteString("  </tags>\n")
	}
	for _, record := range md.Records {
		b.WriteString(`  <data type="`)
		xml.EscapeText(&b, []byte(record.Type))
		b.WriteString("\">\n")
		writeChecksumElement(&b, 4, "checksum", record.Checksum)
		writeChecksumElement(&b, 4, "open-checksum", record.OpenChecksum)
		writeChecksumElement(&b, 4, "header-checksum", record.HeaderChecksum)
		if record.LocationHref != "" {
			b.WriteString(`    <location href="`)
			xml.EscapeText(&b, []byte(record.LocationHref))
			if record.LocationBase != "" {
				b.WriteString(`" xml:base="`)
				xml.EscapeText(&b, []byte(record.LocationBase))
			}
			b.WriteString(`"/>` + "\n")
		}
		if record.Timestamp != 0 {
			writeSimpleElement(&b, 4, "timestamp", strconv.FormatInt(record.Timestamp, 10))
		}
		if record.Size != 0 {
			writeSimpleElement(&b, 4, "size", strconv.FormatInt(record.Size, 10))
		}
		if record.OpenSize >= 0 {
			writeSimpleElement(&b, 4, "open-size", strconv.FormatInt(record.OpenSize, 10))
		}
		if record.HeaderSize >= 0 {
			writeSimpleElement(&b, 4, "header-size", strconv.FormatInt(record.HeaderSize, 10))
		}
		if record.DatabaseVersion != 0 {
			writeSimpleElement(&b, 4, "database_version", strconv.Itoa(record.DatabaseVersion))
		}
		b.WriteString("  </data>\n")
	}
	b.WriteString("</repomd>\n")
	return b.String()
}

func writeSimpleElement(b *bytes.Buffer, indent int, name, value string) {
	fmt.Fprintf(b, "%*s<%s>", indent, "", name)
	xml.EscapeText(b, []byte(value))
	fmt.Fprintf(b, "</%s>\n", name)
}

func writeChecksumElement(b *bytes.Buffer, indent int, name string, checksum ChecksumValue) {
	if checksum.Value == "" {
		return
	}
	fmt.Fprintf(b, `%*s<%s`, indent, "", name)
	if checksum.Type != "" {
		b.WriteString(` type="`)
		xml.EscapeText(b, []byte(checksum.Type))
		b.WriteString(`"`)
	}
	b.WriteString(">")
	xml.EscapeText(b, []byte(checksum.Value))
	fmt.Fprintf(b, "</%s>\n", name)
}
