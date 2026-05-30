package createrepo

func emptyPrimaryXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<metadata xmlns="http://linux.duke.edu/metadata/common" xmlns:rpm="http://linux.duke.edu/metadata/rpm" packages="0">
</metadata>
`)
}

func emptyFilelistsXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<filelists xmlns="http://linux.duke.edu/metadata/filelists" packages="0">
</filelists>
`)
}

func emptyOtherXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<otherdata xmlns="http://linux.duke.edu/metadata/other" packages="0">
</otherdata>
`)
}
