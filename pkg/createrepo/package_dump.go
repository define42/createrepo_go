package createrepo

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

// DumpPrimary serializes packages as primary XML.
func DumpPrimary(packages []Package) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&b, `<metadata xmlns="http://linux.duke.edu/metadata/common" xmlns:rpm="http://linux.duke.edu/metadata/rpm" packages="%d">`+"\n", len(packages))
	for _, pkg := range packages {
		b.WriteString(`<package type="rpm">` + "\n")
		writeSimpleElement(&b, 2, "name", pkg.Name)
		writeSimpleElement(&b, 2, "arch", pkg.Arch)
		writeVersionElement(&b, 2, pkg)
		b.WriteString(`  <checksum type="`)
		xml.EscapeText(&b, []byte(pkg.ChecksumType))
		b.WriteString(`" pkgid="YES">`)
		xml.EscapeText(&b, []byte(pkg.PkgID))
		b.WriteString("</checksum>\n")
		writeSimpleElement(&b, 2, "summary", pkg.Summary)
		writeSimpleElement(&b, 2, "description", pkg.Description)
		writeSimpleElement(&b, 2, "packager", pkg.RPMPackager)
		writeSimpleElement(&b, 2, "url", pkg.URL)
		fmt.Fprintf(&b, `  <time file="%d" build="%d"/>`+"\n", pkg.TimeFile, pkg.TimeBuild)
		fmt.Fprintf(&b, `  <size package="%d" installed="%d" archive="%d"/>`+"\n", pkg.SizePackage, pkg.SizeInstalled, pkg.SizeArchive)
		b.WriteString(`  <location href="`)
		xml.EscapeText(&b, []byte(pkg.LocationHref))
		if pkg.LocationBase != "" {
			b.WriteString(`" xml:base="`)
			xml.EscapeText(&b, []byte(pkg.LocationBase))
		}
		b.WriteString(`"/>` + "\n")
		b.WriteString("  <format>\n")
		writeSimpleElement(&b, 4, "rpm:license", pkg.RPMLicense)
		writeSimpleElement(&b, 4, "rpm:vendor", pkg.RPMVendor)
		writeSimpleElement(&b, 4, "rpm:group", pkg.RPMGroup)
		writeSimpleElement(&b, 4, "rpm:buildhost", pkg.RPMBuildHost)
		writeSimpleElement(&b, 4, "rpm:sourcerpm", pkg.RPMSourcerpm)
		fmt.Fprintf(&b, `    <rpm:header-range start="%d" end="%d"/>`+"\n", pkg.RPMHeaderStart, pkg.RPMHeaderEnd)
		writeDependencyList(&b, "rpm:provides", pkg.Provides)
		writeDependencyList(&b, "rpm:requires", pkg.Requires)
		writeDependencyList(&b, "rpm:conflicts", pkg.Conflicts)
		writeDependencyList(&b, "rpm:obsoletes", pkg.Obsoletes)
		writeDependencyList(&b, "rpm:suggests", pkg.Suggests)
		writeDependencyList(&b, "rpm:enhances", pkg.Enhances)
		writeDependencyList(&b, "rpm:recommends", pkg.Recommends)
		writeDependencyList(&b, "rpm:supplements", pkg.Supplements)
		for _, file := range pkg.Files {
			if file.Type == "dir" {
				continue
			}
			writeFileElement(&b, 4, file)
		}
		b.WriteString("  </format>\n")
		b.WriteString("</package>\n")
	}
	b.WriteString("</metadata>\n")
	return b.Bytes()
}

// DumpFilelists serializes packages as filelists XML.
func DumpFilelists(packages []Package) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&b, `<filelists xmlns="http://linux.duke.edu/metadata/filelists" packages="%d">`+"\n", len(packages))
	for _, pkg := range packages {
		fmt.Fprintf(&b, `<package pkgid="%s" name="`, pkg.PkgID)
		xml.EscapeText(&b, []byte(pkg.Name))
		b.WriteString(`" arch="`)
		xml.EscapeText(&b, []byte(pkg.Arch))
		b.WriteString(`">` + "\n")
		writeVersionElement(&b, 2, pkg)
		for _, file := range pkg.Files {
			writeFileElement(&b, 2, file)
		}
		b.WriteString("</package>\n")
	}
	b.WriteString("</filelists>\n")
	return b.Bytes()
}

// DumpFilelistsExt serializes packages as filelists-ext XML with per-file hashes.
func DumpFilelistsExt(packages []Package) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&b, `<filelists-ext xmlns="http://linux.duke.edu/metadata/filelists-ext" packages="%d">`+"\n", len(packages))
	for _, pkg := range packages {
		fmt.Fprintf(&b, `<package pkgid="%s" name="`, pkg.PkgID)
		xml.EscapeText(&b, []byte(pkg.Name))
		b.WriteString(`" arch="`)
		xml.EscapeText(&b, []byte(pkg.Arch))
		b.WriteString(`">` + "\n")
		writeVersionElement(&b, 2, pkg)
		for _, file := range pkg.Files {
			writeFileExtElement(&b, 2, file)
		}
		b.WriteString("</package>\n")
	}
	b.WriteString("</filelists-ext>\n")
	return b.Bytes()
}

// DumpOther serializes packages as other XML.
func DumpOther(packages []Package) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(&b, `<otherdata xmlns="http://linux.duke.edu/metadata/other" packages="%d">`+"\n", len(packages))
	for _, pkg := range packages {
		fmt.Fprintf(&b, `<package pkgid="%s" name="`, pkg.PkgID)
		xml.EscapeText(&b, []byte(pkg.Name))
		b.WriteString(`" arch="`)
		xml.EscapeText(&b, []byte(pkg.Arch))
		b.WriteString(`">` + "\n")
		writeVersionElement(&b, 2, pkg)
		for _, entry := range pkg.Changelogs {
			fmt.Fprintf(&b, `  <changelog author="`)
			xml.EscapeText(&b, []byte(entry.Author))
			fmt.Fprintf(&b, `" date="%d">`, entry.Date)
			xml.EscapeText(&b, []byte(entry.Changelog))
			b.WriteString("</changelog>\n")
		}
		b.WriteString("</package>\n")
	}
	b.WriteString("</otherdata>\n")
	return b.Bytes()
}

func writeVersionElement(b *bytes.Buffer, indent int, pkg Package) {
	fmt.Fprintf(b, `%*s<version epoch="`, indent, "")
	xml.EscapeText(b, []byte(defaultString(pkg.Epoch, "0")))
	b.WriteString(`" ver="`)
	xml.EscapeText(b, []byte(pkg.Version))
	b.WriteString(`" rel="`)
	xml.EscapeText(b, []byte(pkg.Release))
	b.WriteString(`"/>` + "\n")
}

func writeDependencyList(b *bytes.Buffer, name string, deps []Dependency) {
	if len(deps) == 0 {
		return
	}
	fmt.Fprintf(b, "    <%s>\n", name)
	for _, dep := range deps {
		b.WriteString(`      <rpm:entry name="`)
		xml.EscapeText(b, []byte(dep.Name))
		b.WriteString(`"`)
		if dep.Flags != "" {
			b.WriteString(` flags="`)
			xml.EscapeText(b, []byte(dep.Flags))
			b.WriteString(`"`)
		}
		if dep.Epoch != "" {
			b.WriteString(` epoch="`)
			xml.EscapeText(b, []byte(dep.Epoch))
			b.WriteString(`"`)
		}
		if dep.Version != "" {
			b.WriteString(` ver="`)
			xml.EscapeText(b, []byte(dep.Version))
			b.WriteString(`"`)
		}
		if dep.Release != "" {
			b.WriteString(` rel="`)
			xml.EscapeText(b, []byte(dep.Release))
			b.WriteString(`"`)
		}
		if dep.Pre {
			b.WriteString(` pre="1"`)
		}
		b.WriteString("/>\n")
	}
	fmt.Fprintf(b, "    </%s>\n", name)
}

func writeFileElement(b *bytes.Buffer, indent int, file PackageFile) {
	fmt.Fprintf(b, "%*s<file", indent, "")
	if file.Type != "" {
		b.WriteString(` type="`)
		xml.EscapeText(b, []byte(file.Type))
		b.WriteString(`"`)
	}
	b.WriteString(">")
	xml.EscapeText(b, []byte(fileFullPath(file)))
	b.WriteString("</file>\n")
}

func writeFileExtElement(b *bytes.Buffer, indent int, file PackageFile) {
	fmt.Fprintf(b, "%*s<file", indent, "")
	if file.Type != "" {
		b.WriteString(` type="`)
		xml.EscapeText(b, []byte(file.Type))
		b.WriteString(`"`)
	}
	if file.Digest != "" {
		b.WriteString(` hash="`)
		xml.EscapeText(b, []byte(file.Digest))
		b.WriteString(`"`)
	}
	b.WriteString(">")
	xml.EscapeText(b, []byte(fileFullPath(file)))
	b.WriteString("</file>\n")
}

func fileFullPath(file PackageFile) string {
	if file.Path == "" || file.Path == "/" {
		return "/" + file.Name
	}
	return file.Path + "/" + file.Name
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
