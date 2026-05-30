package createrepo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseUpdateInfoFile parses updateinfo XML metadata from a plain or
// compressed file.
func ParseUpdateInfoFile(path string) (*UpdateInfo, error) {
	r, err := OpenReader(path, CompressionAuto)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ParseUpdateInfo(r)
}

// ParseUpdateInfo parses updateinfo XML metadata.
func ParseUpdateInfo(r io.Reader) (*UpdateInfo, error) {
	var raw updateInfoXML
	if err := xml.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}
	out := &UpdateInfo{Updates: make([]UpdateRecord, 0, len(raw.Updates))}
	for _, update := range raw.Updates {
		record := UpdateRecord{
			From:            update.From,
			Status:          update.Status,
			Type:            update.Type,
			Version:         update.Version,
			ID:              update.ID,
			Title:           update.Title,
			IssuedDate:      update.Issued.Date,
			UpdatedDate:     update.Updated.Date,
			Rights:          update.Rights,
			Release:         update.Release,
			PushCount:       update.PushCount,
			Severity:        update.Severity,
			Summary:         update.Summary,
			Description:     update.Description,
			Solution:        update.Solution,
			RebootSuggested: xmlBool(update.RebootSuggested),
			References:      make([]UpdateReference, 0, len(update.References)),
			Collections:     make([]UpdateCollection, 0, len(update.Collections)),
		}
		for _, ref := range update.References {
			record.References = append(record.References, UpdateReference{
				Href:  ref.Href,
				ID:    ref.ID,
				Type:  ref.Type,
				Title: ref.Title,
			})
		}
		for _, collection := range update.Collections {
			outCollection := UpdateCollection{
				ShortName: collection.ShortName,
				Name:      collection.Name,
				Packages:  make([]UpdateCollectionPackage, 0, len(collection.Packages)),
			}
			if collection.Module != nil {
				outCollection.Module = &UpdateCollectionModule{
					Name:    collection.Module.Name,
					Stream:  collection.Module.Stream,
					Version: collection.Module.Version,
					Context: collection.Module.Context,
					Arch:    collection.Module.Arch,
				}
			}
			for _, pkg := range collection.Packages {
				outCollection.Packages = append(outCollection.Packages, UpdateCollectionPackage{
					Name:             pkg.Name,
					Version:          pkg.Version,
					Release:          pkg.Release,
					Epoch:            pkg.Epoch,
					Arch:             pkg.Arch,
					Src:              pkg.Src,
					Filename:         pkg.Filename,
					Sum:              pkg.Sum.Value,
					SumType:          ChecksumTypeFromName(pkg.Sum.Type),
					RebootSuggested:  xmlBool(pkg.RebootSuggested),
					RestartSuggested: xmlBool(pkg.RestartSuggested),
					ReloginSuggested: xmlBool(pkg.ReloginSuggested),
				})
			}
			record.Collections = append(record.Collections, outCollection)
		}
		out.Updates = append(out.Updates, record)
	}
	return out, nil
}

// DumpUpdateInfo serializes updateinfo metadata.
func DumpUpdateInfo(info *UpdateInfo) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString("<updates>\n")
	if info != nil {
		for _, update := range info.Updates {
			fmt.Fprintf(&b, `  <update from="`)
			xml.EscapeText(&b, []byte(update.From))
			b.WriteString(`" status="`)
			xml.EscapeText(&b, []byte(update.Status))
			b.WriteString(`" type="`)
			xml.EscapeText(&b, []byte(update.Type))
			b.WriteString(`" version="`)
			xml.EscapeText(&b, []byte(update.Version))
			b.WriteString("\">\n")
			writeSimpleElement(&b, 4, "id", update.ID)
			writeSimpleElement(&b, 4, "title", update.Title)
			writeDateElement(&b, "issued", update.IssuedDate)
			writeDateElement(&b, "updated", update.UpdatedDate)
			writeSimpleElement(&b, 4, "rights", update.Rights)
			writeSimpleElement(&b, 4, "release", update.Release)
			writeSimpleElement(&b, 4, "pushcount", update.PushCount)
			writeSimpleElement(&b, 4, "severity", update.Severity)
			writeSimpleElement(&b, 4, "summary", update.Summary)
			writeSimpleElement(&b, 4, "description", update.Description)
			writeSimpleElement(&b, 4, "solution", update.Solution)
			if update.RebootSuggested {
				writeSimpleElement(&b, 4, "reboot_suggested", "True")
			}
			if len(update.References) > 0 {
				b.WriteString("    <references>\n")
				for _, ref := range update.References {
					b.WriteString(`      <reference href="`)
					xml.EscapeText(&b, []byte(ref.Href))
					b.WriteString(`" id="`)
					xml.EscapeText(&b, []byte(ref.ID))
					b.WriteString(`" type="`)
					xml.EscapeText(&b, []byte(ref.Type))
					b.WriteString(`" title="`)
					xml.EscapeText(&b, []byte(ref.Title))
					b.WriteString(`"/>` + "\n")
				}
				b.WriteString("    </references>\n")
			}
			if len(update.Collections) > 0 {
				b.WriteString("    <pkglist>\n")
				for _, collection := range update.Collections {
					b.WriteString(`      <collection short="`)
					xml.EscapeText(&b, []byte(collection.ShortName))
					b.WriteString("\">\n")
					writeSimpleElement(&b, 8, "name", collection.Name)
					if collection.Module != nil {
						fmt.Fprintf(&b, `        <module name="`)
						xml.EscapeText(&b, []byte(collection.Module.Name))
						b.WriteString(`" stream="`)
						xml.EscapeText(&b, []byte(collection.Module.Stream))
						b.WriteString(`" version="`)
						xml.EscapeText(&b, []byte(strconv.FormatUint(collection.Module.Version, 10)))
						b.WriteString(`" context="`)
						xml.EscapeText(&b, []byte(collection.Module.Context))
						b.WriteString(`" arch="`)
						xml.EscapeText(&b, []byte(collection.Module.Arch))
						b.WriteString(`"/>` + "\n")
					}
					for _, pkg := range collection.Packages {
						writeUpdatePackage(&b, pkg)
					}
					b.WriteString("      </collection>\n")
				}
				b.WriteString("    </pkglist>\n")
			}
			b.WriteString("  </update>\n")
		}
	}
	b.WriteString("</updates>\n")
	return b.Bytes()
}

type updateInfoXML struct {
	Updates []updateXML `xml:"update"`
}

type updateXML struct {
	From            string          `xml:"from,attr"`
	Status          string          `xml:"status,attr"`
	Type            string          `xml:"type,attr"`
	Version         string          `xml:"version,attr"`
	ID              string          `xml:"id"`
	Title           string          `xml:"title"`
	Issued          dateXML         `xml:"issued"`
	Updated         dateXML         `xml:"updated"`
	Rights          string          `xml:"rights"`
	Release         string          `xml:"release"`
	PushCount       string          `xml:"pushcount"`
	Severity        string          `xml:"severity"`
	Summary         string          `xml:"summary"`
	Description     string          `xml:"description"`
	Solution        string          `xml:"solution"`
	RebootSuggested *string         `xml:"reboot_suggested"`
	References      []referenceXML  `xml:"references>reference"`
	Collections     []collectionXML `xml:"pkglist>collection"`
}

type dateXML struct {
	Date string `xml:"date,attr"`
}

type referenceXML struct {
	Href  string `xml:"href,attr"`
	ID    string `xml:"id,attr"`
	Type  string `xml:"type,attr"`
	Title string `xml:"title,attr"`
}

type collectionXML struct {
	ShortName string       `xml:"short,attr"`
	Name      string       `xml:"name"`
	Module    *moduleXML   `xml:"module"`
	Packages  []packageXML `xml:"package"`
}

type moduleXML struct {
	Name    string `xml:"name,attr"`
	Stream  string `xml:"stream,attr"`
	Version uint64 `xml:"version,attr"`
	Context string `xml:"context,attr"`
	Arch    string `xml:"arch,attr"`
}

type packageXML struct {
	Name             string  `xml:"name,attr"`
	Version          string  `xml:"version,attr"`
	Release          string  `xml:"release,attr"`
	Epoch            string  `xml:"epoch,attr"`
	Arch             string  `xml:"arch,attr"`
	Src              string  `xml:"src,attr"`
	Filename         string  `xml:"filename"`
	Sum              sumXML  `xml:"sum"`
	RebootSuggested  *string `xml:"reboot_suggested"`
	RestartSuggested *string `xml:"restart_suggested"`
	ReloginSuggested *string `xml:"relogin_suggested"`
}

type sumXML struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

func xmlBool(value *string) bool {
	if value == nil {
		return false
	}
	text := strings.TrimSpace(*value)
	return text == "" || strings.EqualFold(text, "true") || text == "1" || strings.EqualFold(text, "yes")
}

func writeDateElement(b *bytes.Buffer, name, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, `    <%s date="`, name)
	xml.EscapeText(b, []byte(value))
	fmt.Fprintf(b, `"/>`+"\n")
}

func writeUpdatePackage(b *bytes.Buffer, pkg UpdateCollectionPackage) {
	b.WriteString(`        <package name="`)
	xml.EscapeText(b, []byte(pkg.Name))
	b.WriteString(`" version="`)
	xml.EscapeText(b, []byte(pkg.Version))
	b.WriteString(`" release="`)
	xml.EscapeText(b, []byte(pkg.Release))
	if pkg.Epoch != "" {
		b.WriteString(`" epoch="`)
		xml.EscapeText(b, []byte(pkg.Epoch))
	}
	b.WriteString(`" arch="`)
	xml.EscapeText(b, []byte(pkg.Arch))
	if pkg.Src != "" {
		b.WriteString(`" src="`)
		xml.EscapeText(b, []byte(pkg.Src))
	}
	b.WriteString("\">\n")
	writeSimpleElement(b, 10, "filename", pkg.Filename)
	if pkg.Sum != "" {
		b.WriteString(`          <sum type="`)
		xml.EscapeText(b, []byte(pkg.SumType.String()))
		b.WriteString(`">`)
		xml.EscapeText(b, []byte(pkg.Sum))
		b.WriteString("</sum>\n")
	}
	if pkg.RebootSuggested {
		b.WriteString("          <reboot_suggested/>\n")
	}
	if pkg.RestartSuggested {
		b.WriteString("          <restart_suggested/>\n")
	}
	if pkg.ReloginSuggested {
		b.WriteString("          <relogin_suggested/>\n")
	}
	b.WriteString("        </package>\n")
}
