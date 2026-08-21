package releaseartifacts

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var requiredArchiveFiles = []string{
	"LICENSE",
	"README.md",
	"CHANGELOG.md",
	"CONTRIBUTING.md",
	"SECURITY.md",
	"THIRD_PARTY_NOTICES.md",
}

// VerifyArchivePayloads checks licences, docs and the platform binary in each archive.
func VerifyArchivePayloads(distDir string) error {
	for _, archive := range ArchiveNames() {
		path := filepath.Join(distDir, archive)
		names, err := listArchive(path)
		if err != nil {
			return fmt.Errorf("list %s: %w", archive, err)
		}
		for _, required := range requiredArchiveFiles {
			if !containsFile(names, required) {
				return fmt.Errorf("%s is missing %s", archive, required)
			}
		}
		binary := "dirloom"
		if strings.Contains(archive, "Windows") {
			binary = "dirloom.exe"
		}
		if !containsFile(names, binary) {
			return fmt.Errorf("%s is missing %s", archive, binary)
		}
	}
	return nil
}

func listArchive(path string) ([]string, error) {
	if strings.HasSuffix(path, ".zip") {
		reader, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		names := make([]string, 0, len(reader.File))
		for _, file := range reader.File {
			names = append(names, filepath.ToSlash(file.Name))
		}
		return names, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var names []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return names, nil
		}
		if err != nil {
			return nil, err
		}
		names = append(names, filepath.ToSlash(header.Name))
		_, _ = io.Copy(io.Discard, tarReader)
	}
}

func containsFile(names []string, want string) bool {
	for _, name := range names {
		base := name
		if index := strings.LastIndex(name, "/"); index >= 0 {
			base = name[index+1:]
		}
		if base == want {
			return true
		}
	}
	return false
}
