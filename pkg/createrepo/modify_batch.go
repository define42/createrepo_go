package createrepo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func parseModifyBatchFile(path string, defaults ModifyOptions) ([]ModifyOptions, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	type section struct {
		name   string
		values map[string]string
	}
	var sections []section
	var current *section
	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if name == "" {
				return nil, fmt.Errorf("%s:%d: empty batch section name", path, lineNo)
			}
			sections = append(sections, section{name: name, values: map[string]string{}})
			current = &sections[len(sections)-1]
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("%s:%d: key outside a batch section", path, lineNo)
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected key=value", path, lineNo)
		}
		current.values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	tasks := make([]ModifyOptions, 0, len(sections))
	for _, section := range sections {
		task := defaults
		task.BatchFile = ""
		task.MetadataPath = resolveBatchPath(path, firstNonEmpty(section.values["path"], section.name))
		task.MetadataType = section.values["type"]
		task.NewName = section.values["new-name"]
		task.Compress = parseBoolDefault(section.values["compress"], true)
		task.NoCompress = !task.Compress
		task.UniqueMDFilenames = parseBoolDefault(section.values["unique-md-filenames"], true)
		if checksum := section.values["checksum"]; checksum != "" {
			task.Checksum = ChecksumTypeFromName(checksum)
		}
		if compression := section.values["compress-type"]; compression != "" {
			task.Compression = CompressionTypeFromName(compression)
		}
		if parseBoolDefault(section.values["remove"], false) {
			task.RemoveType = firstNonEmpty(task.MetadataType, section.name)
			task.MetadataPath = ""
			task.MetadataType = ""
			task.NewName = ""
			task.Compress = false
			task.NoCompress = false
		} else {
			task.RemoveType = ""
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func resolveBatchPath(batchFile, value string) string {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "://") {
		return value
	}
	return filepath.Join(filepath.Dir(batchFile), value)
}

func parseBoolDefault(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
