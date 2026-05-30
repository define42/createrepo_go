package createrepo

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ChecksumValue is a checksum element from repomd.xml.
type ChecksumValue struct {
	Type  string
	Value string
}

// RepomdRecord is one <data> entry in repomd.xml.
type RepomdRecord struct {
	Type            string
	LocationReal    string
	LocationHref    string
	LocationBase    string
	Checksum        ChecksumValue
	OpenChecksum    ChecksumValue
	HeaderChecksum  ChecksumValue
	Timestamp       int64
	Size            int64
	OpenSize        int64
	HeaderSize      int64
	DatabaseVersion int
}

// DistroTag is a <distro> tag from repomd.xml.
type DistroTag struct {
	CPEID string
	Value string
}

// Repomd is the parsed representation of repomd.xml.
type Repomd struct {
	Revision    string
	RepoID      ChecksumValue
	ContentHash ChecksumValue
	RepoTags    []string
	ContentTags []string
	DistroTags  []DistroTag
	Records     []*RepomdRecord
}

// NewRepomdRecord creates a metadata record, initializing locations when path
// is supplied.
func NewRepomdRecord(recordType, path string) *RepomdRecord {
	record := &RepomdRecord{
		Type:       recordType,
		OpenSize:   -1,
		HeaderSize: -1,
	}
	if path != "" {
		record.LocationReal = path
		record.LocationHref = "repodata/" + filepath.Base(path)
	}
	return record
}

// Record returns the first metadata record with the requested type.
func (r *Repomd) Record(recordType string) *RepomdRecord {
	for _, record := range r.Records {
		if record.Type == recordType {
			return record
		}
	}
	return nil
}

// AddRecord appends a metadata record.
func (r *Repomd) AddRecord(record *RepomdRecord) {
	r.Records = append(r.Records, record)
}

// Fill calculates missing checksums and sizes for a metadata record.
func (r *RepomdRecord) Fill(checksumType ChecksumType) error {
	if r.LocationReal == "" {
		return fmt.Errorf("empty location in repomd record")
	}

	info, err := os.Stat(r.LocationReal)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", r.LocationReal)
	}

	checksumName, ok := checksumType.Name()
	if !ok {
		return fmt.Errorf("unknown checksum type: %d", checksumType)
	}

	if r.Checksum.Value == "" || r.Checksum.Type == "" {
		sum, err := ChecksumFile(r.LocationReal, checksumType)
		if err != nil {
			return err
		}
		r.Checksum = ChecksumValue{Type: checksumName, Value: sum}
	}

	compression, err := DetectCompression(r.LocationReal)
	if err != nil {
		return err
	}
	if compression != CompressionUnknown && compression != CompressionNone {
		stat, err := CompressedContentStat(r.LocationReal, checksumType)
		if err != nil {
			return err
		}
		r.OpenChecksum = ChecksumValue{Type: checksumName, Value: stat.Checksum}
		r.OpenSize = stat.Size
		if stat.HeaderChecksum != "" {
			headerName, ok := stat.HeaderChecksumType.Name()
			if ok {
				r.HeaderChecksum = ChecksumValue{Type: headerName, Value: stat.HeaderChecksum}
			}
			r.HeaderSize = stat.HeaderSize
		}
	}

	if r.Timestamp == 0 {
		r.Timestamp = info.ModTime().Unix()
	}
	if r.Size == 0 {
		r.Size = info.Size()
	}

	return nil
}

// RenameFile prefixes the backing file name with the record checksum and keeps
// record locations in sync.
func (r *RepomdRecord) RenameFile() error {
	if r.LocationReal == "" || r.LocationHref == "" {
		return fmt.Errorf("empty location in repomd record")
	}
	if r.Checksum.Value == "" {
		return fmt.Errorf("record does not contain a checksum")
	}

	dir := filepath.Dir(r.LocationReal)
	filename := filepath.Base(r.LocationReal)
	if strings.HasPrefix(filename, r.Checksum.Value) {
		return nil
	}

	filename = stripObsoleteChecksumPrefix(filename)
	newReal := filepath.Join(dir, r.Checksum.Value+"-"+filename)
	if _, err := os.Stat(newReal); err == nil {
		if err := os.Remove(newReal); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(r.LocationReal, newReal); err != nil {
		return err
	}
	r.LocationReal = newReal
	r.LocationHref = "repodata/" + filepath.Base(newReal)
	return nil
}

func stripObsoleteChecksumPrefix(filename string) string {
	dash := strings.IndexByte(filename, '-')
	if dash < 0 {
		return filename
	}
	switch dash {
	case 32, 40, 64, 128:
		return filename[dash+1:]
	default:
		return filename
	}
}

// ParseRepomdFile parses a repomd.xml file. The file can be plain or in a
// supported compressed format.
func ParseRepomdFile(path string) (*Repomd, error) {
	r, err := OpenReader(path, CompressionAuto)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ParseRepomd(r)
}

// ParseRepomd parses repomd.xml from r.
func ParseRepomd(r io.Reader) (*Repomd, error) {
	decoder := xml.NewDecoder(r)
	repomd := &Repomd{}

	var rootFound bool
	var current *RepomdRecord
	var collect string
	var text strings.Builder
	var distroCPEID string

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch token := tok.(type) {
		case xml.StartElement:
			local := token.Name.Local
			switch local {
			case "repomd":
				rootFound = true
			case "data":
				recordType := attrValue(token.Attr, "type")
				if recordType == "" {
					recordType = "unknown"
				}
				current = NewRepomdRecord(recordType, "")
				repomd.AddRecord(current)
			case "location":
				if current != nil {
					current.LocationHref = attrValue(token.Attr, "href")
					current.LocationBase = attrValue(token.Attr, "base")
				}
			case "repoid":
				repomd.RepoID.Type = attrValue(token.Attr, "type")
				collect = local
				text.Reset()
			case "contenthash":
				repomd.ContentHash.Type = attrValue(token.Attr, "type")
				collect = local
				text.Reset()
			case "checksum":
				if current != nil {
					current.Checksum.Type = attrValue(token.Attr, "type")
				}
				collect = local
				text.Reset()
			case "open-checksum":
				if current != nil {
					current.OpenChecksum.Type = attrValue(token.Attr, "type")
				}
				collect = local
				text.Reset()
			case "header-checksum":
				if current != nil {
					current.HeaderChecksum.Type = attrValue(token.Attr, "type")
				}
				collect = local
				text.Reset()
			case "distro":
				distroCPEID = attrValue(token.Attr, "cpeid")
				collect = local
				text.Reset()
			case "revision", "repo", "content", "timestamp", "size", "open-size", "header-size", "database_version":
				collect = local
				text.Reset()
			}
		case xml.CharData:
			if collect != "" {
				text.Write([]byte(token))
			}
		case xml.EndElement:
			local := token.Name.Local
			if collect == local {
				content := strings.TrimSpace(text.String())
				if err := applyRepomdContent(repomd, current, collect, distroCPEID, content); err != nil {
					return nil, err
				}
				collect = ""
				distroCPEID = ""
			}
			if local == "data" {
				current = nil
			}
		}
	}

	if !rootFound {
		return nil, fmt.Errorf("missing repomd root element")
	}
	return repomd, nil
}

func applyRepomdContent(repomd *Repomd, current *RepomdRecord, element, distroCPEID, content string) error {
	switch element {
	case "revision":
		repomd.Revision = content
	case "repoid":
		repomd.RepoID.Value = content
	case "contenthash":
		repomd.ContentHash.Value = content
	case "repo":
		repomd.RepoTags = append(repomd.RepoTags, content)
	case "content":
		repomd.ContentTags = append(repomd.ContentTags, content)
	case "distro":
		repomd.DistroTags = append(repomd.DistroTags, DistroTag{CPEID: distroCPEID, Value: content})
	case "checksum":
		if current != nil {
			current.Checksum.Value = content
		}
	case "open-checksum":
		if current != nil {
			current.OpenChecksum.Value = content
		}
	case "header-checksum":
		if current != nil {
			current.HeaderChecksum.Value = content
		}
	case "timestamp":
		value, err := parseInt64Content("timestamp", content)
		if err != nil {
			return err
		}
		if current != nil {
			current.Timestamp = value
		}
	case "size":
		value, err := parseInt64Content("size", content)
		if err != nil {
			return err
		}
		if current != nil {
			current.Size = value
		}
	case "open-size":
		value, err := parseInt64Content("open-size", content)
		if err != nil {
			return err
		}
		if current != nil {
			current.OpenSize = value
		}
	case "header-size":
		value, err := parseInt64Content("header-size", content)
		if err != nil {
			return err
		}
		if current != nil {
			current.HeaderSize = value
		}
	case "database_version":
		value, err := parseInt64Content("database_version", content)
		if err != nil {
			return err
		}
		if current != nil {
			current.DatabaseVersion = int(value)
		}
	}
	return nil
}

func parseInt64Content(name, content string) (int64, error) {
	value, err := strconv.ParseInt(content, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", name, content, err)
	}
	return value, nil
}

func attrValue(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}
