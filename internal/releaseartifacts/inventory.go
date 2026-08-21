// Package releaseartifacts verifies the immutable GitHub Release inventory.
package releaseartifacts

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ChecksumsName = "checksums.txt"
	ExpectedCount = 13
	ChecksumLines = 12
)

// ArchiveNames is the stable GoReleaser archive set.
func ArchiveNames() []string {
	return []string{
		"dirloom_Windows_x86_64.zip",
		"dirloom_Windows_arm64.zip",
		"dirloom_Linux_x86_64.tar.gz",
		"dirloom_Linux_arm64.tar.gz",
		"dirloom_Darwin_x86_64.tar.gz",
		"dirloom_Darwin_arm64.tar.gz",
	}
}

// SBOMName returns the SPDX sidecar for an archive.
func SBOMName(archive string) string {
	return archive + ".spdx.json"
}

// Prepare generates one SPDX SBOM per archive and rewrites checksums.txt.
func Prepare(distDir, syftPath string) error {
	if syftPath == "" {
		syftPath = "syft"
	}
	for _, archive := range ArchiveNames() {
		archivePath := filepath.Join(distDir, archive)
		if _, err := os.Stat(archivePath); err != nil {
			return fmt.Errorf("archive %s: %w", archive, err)
		}
		document := SBOMName(archive)
		command := exec.Command(syftPath, archive, "-o", "spdx-json="+document) //nolint:gosec // Syft path is pinned in CI and release workflows.
		command.Dir = distDir
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("syft %s: %w\n%s", archive, err, output)
		}
	}
	return WriteChecksums(distDir)
}

// WriteChecksums writes checksums.txt covering the 6 archives and 6 SBOMs.
func WriteChecksums(distDir string) error {
	entries, err := hashedEntries(distDir)
	if err != nil {
		return err
	}
	var builder strings.Builder
	for _, entry := range entries {
		_, _ = fmt.Fprintf(&builder, "%s  %s\n", entry.hash, entry.name)
	}
	path := filepath.Join(distDir, ChecksumsName)
	return os.WriteFile(path, []byte(builder.String()), 0o644) //nolint:gosec // Published checksums are intentionally world-readable.
}

// Verify asserts the 13-artifact inventory and independent checksums.
func Verify(distDir string) error {
	for _, archive := range ArchiveNames() {
		if err := mustFile(filepath.Join(distDir, archive)); err != nil {
			return err
		}
		if err := mustFile(filepath.Join(distDir, SBOMName(archive))); err != nil {
			return err
		}
	}
	checksumsPath := filepath.Join(distDir, ChecksumsName)
	if err := mustFile(checksumsPath); err != nil {
		return err
	}
	file, err := os.Open(checksumsPath) //nolint:gosec // Release verification reads files from the GoReleaser dist directory.
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	seen := map[string]string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return fmt.Errorf("checksums.txt has an invalid line %q", line)
		}
		if fields[1] == ChecksumsName {
			return fmt.Errorf("checksums.txt must not hash itself")
		}
		seen[fields[1]] = fields[0]
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(seen) != ChecksumLines {
		return fmt.Errorf("checksums.txt has %d hashes, want %d", len(seen), ChecksumLines)
	}
	want, err := hashedEntries(distDir)
	if err != nil {
		return err
	}
	if len(want)+1 != ExpectedCount {
		return fmt.Errorf("inventory mismatch: hashed=%d checksums file=1", len(want))
	}
	for _, entry := range want {
		got, ok := seen[entry.name]
		if !ok {
			return fmt.Errorf("checksums.txt missing %s", entry.name)
		}
		if got != entry.hash {
			return fmt.Errorf("hash mismatch for %s: checksums=%s actual=%s", entry.name, got, entry.hash)
		}
	}
	return nil
}

type hashed struct {
	name string
	hash string
}

func hashedEntries(distDir string) ([]hashed, error) {
	names := make([]string, 0, ChecksumLines)
	names = append(names, ArchiveNames()...)
	for _, archive := range ArchiveNames() {
		names = append(names, SBOMName(archive))
	}
	sort.Strings(names)
	entries := make([]hashed, 0, len(names))
	for _, name := range names {
		sum, err := fileSHA256(filepath.Join(distDir, name))
		if err != nil {
			return nil, err
		}
		entries = append(entries, hashed{name: name, hash: sum})
	}
	return entries, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // Release verification reads files from the GoReleaser dist directory.
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func mustFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing release artifact %s: %w", filepath.Base(path), err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("release artifact %s is empty or not a file", filepath.Base(path))
	}
	return nil
}
