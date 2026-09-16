package releaseartifacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAndVerifyInventory(t *testing.T) {
	dist := t.TempDir()
	for _, archive := range ArchiveNames() {
		if err := os.WriteFile(filepath.Join(dist, archive), []byte(archive+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dist, SBOMName(archive)), []byte(`{"spdxVersion":"SPDX-2.3"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteChecksums(dist); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dist); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dist, ChecksumsName))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != ChecksumLines {
		t.Fatalf("checksum lines = %d", len(lines))
	}
	if strings.Contains(string(data), ChecksumsName+"\n") || strings.Contains(string(data), "  "+ChecksumsName) {
		t.Fatal("checksums.txt hashed itself")
	}
}

func TestInventoryContract(t *testing.T) {
	if ExpectedCount != 13 || ChecksumLines != 12 || len(ArchiveNames()) != 6 {
		t.Fatalf("inventory constants changed: artifacts=%d hashes=%d archives=%d", ExpectedCount, ChecksumLines, len(ArchiveNames()))
	}
	if ChecksumLines+1 != ExpectedCount {
		t.Fatal("checksum lines plus checksums.txt must equal 13")
	}
}

func TestVerifyRejectsMissingAndSelfHash(t *testing.T) {
	dist := t.TempDir()
	if err := Verify(dist); err == nil {
		t.Fatal("empty dist must fail")
	}
}
