package releaseartifacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleChangelog = `# Changelog

## [Unreleased]

## [0.4.0-a2] - 2026-09-24

CHANGE alpha: persistent self-verifying structural snapshots on top of Fingerprint
v1. This increment does not add verify or diff.

### Added

- Add ` + "`dirloom snapshot`" + ` to persist Snapshot Schema v1 JSON.

### Changed

- Document latest published ` + "`v0.4.0-a1`" + `.

## [0.4.0-a1] - 2026-09-20

CHANGE alpha: fingerprint.

### Added

- Add fingerprint.

[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.4.0-a2...HEAD
[0.4.0-a2]: https://github.com/dirloom/dirloom/compare/v0.4.0-a1...v0.4.0-a2
[0.4.0-a1]: https://github.com/dirloom/dirloom/compare/v0.3.3...v0.4.0-a1
`

func TestExtractVersionSectionAcceptsLeadingV(t *testing.T) {
	section, err := ExtractVersionSection(sampleChangelog, "v0.4.0-a2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(section, "## [0.4.0-a2] - 2026-09-24\n") {
		head := section
		if len(head) > 60 {
			head = head[:60]
		}
		t.Fatalf("unexpected heading: %q", head)
	}
	if strings.Contains(section, "## [0.4.0-a1]") {
		t.Fatal("extraction leaked the next version")
	}
	if !strings.Contains(section, "### Added") || !strings.Contains(section, "### Changed") {
		t.Fatal("expected Added and Changed sections")
	}
}

func TestExtractVersionSectionOmitsTrailingLinks(t *testing.T) {
	changelog := `# Changelog

## [Unreleased]

## [0.4.0-a2] - 2026-09-24

CHANGE alpha: snapshots. This increment does not add verify or diff.

### Added

- Add snapshot.

[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.4.0-a2...HEAD
[0.4.0-a2]: https://github.com/dirloom/dirloom/compare/v0.4.0-a1...v0.4.0-a2
`
	section, err := ExtractVersionSection(changelog, "v0.4.0-a2")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(section, "[Unreleased]:") || strings.Contains(section, "[0.4.0-a2]:") {
		t.Fatalf("compare links leaked into notes:\n%s", section)
	}
}

func TestExtractVersionSectionRejectsMissingAndUnreleasedDate(t *testing.T) {
	if _, err := ExtractVersionSection(sampleChangelog, "9.9.9"); err == nil {
		t.Fatal("missing section must fail")
	}
	bad := strings.Replace(sampleChangelog, "## [0.4.0-a2] - 2026-09-24", "## [0.4.0-a2] - Unreleased", 1)
	if _, err := ExtractVersionSection(bad, "0.4.0-a2"); err == nil {
		t.Fatal("Unreleased date must fail")
	}
}

func TestValidateVersionSectionRejectsEmptyAndUnknown(t *testing.T) {
	emptySection := "## [1.0.0] - 2026-01-01\n\nSummary.\n\n### Added\n\n"
	if err := ValidateVersionSection(emptySection, "1.0.0"); err == nil {
		t.Fatal("empty Added must fail")
	}
	unknown := "## [1.0.0] - 2026-01-01\n\nSummary.\n\n### Notes\n\n- x\n"
	if err := ValidateVersionSection(unknown, "1.0.0"); err == nil {
		t.Fatal("unknown section must fail")
	}
	outOfOrder := "## [1.0.0] - 2026-01-01\n\nSummary.\n\n### Fixed\n\n- x\n\n### Added\n\n- y\n"
	if err := ValidateVersionSection(outOfOrder, "1.0.0"); err == nil {
		t.Fatal("out-of-order sections must fail")
	}
}

func TestWriteReleaseNotesCreatesParent(t *testing.T) {
	dir := t.TempDir()
	changelogPath := filepath.Join(dir, "CHANGELOG.md")
	if err := os.WriteFile(changelogPath, []byte(sampleChangelog), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "dist", "release-notes.md")
	if err := WriteReleaseNotes(changelogPath, "v0.4.0-a2", out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "## [0.4.0-a2] - 2026-09-24") {
		t.Fatalf("unexpected output: %s", data)
	}
}

func TestReleaseBranchVersion(t *testing.T) {
	version, ok := ReleaseBranchVersion("release/v0.4.0-a2")
	if !ok || version != "0.4.0-a2" {
		t.Fatalf("got %q %v", version, ok)
	}
	if _, ok := ReleaseBranchVersion("main"); ok {
		t.Fatal("main must not be a release branch")
	}
	if _, ok := ReleaseBranchVersion("refs/heads/release/v1.2.3"); !ok {
		t.Fatal("refs/heads prefix must be accepted")
	}
}

func TestValidateReleaseBranchFreeze(t *testing.T) {
	if err := ValidateReleaseBranchFreeze("main", sampleChangelog); err != nil {
		t.Fatalf("main must skip freeze gate: %v", err)
	}
	if err := ValidateReleaseBranchFreeze("release/v0.4.0-a2", sampleChangelog); err != nil {
		t.Fatal(err)
	}

	dirtyUnreleased := strings.Replace(sampleChangelog, "## [Unreleased]\n\n", "## [Unreleased]\n\n- pending\n\n", 1)
	if err := ValidateReleaseBranchFreeze("release/v0.4.0-a2", dirtyUnreleased); err == nil {
		t.Fatal("non-empty Unreleased must fail")
	}

	badLink := strings.Replace(sampleChangelog,
		"[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.4.0-a2...HEAD",
		"[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.4.0-a1...HEAD", 1)
	if err := ValidateReleaseBranchFreeze("release/v0.4.0-a2", badLink); err == nil {
		t.Fatal("wrong Unreleased link must fail")
	}
}

func TestValidateReleaseBranchFreezeRejectsMissingVersion(t *testing.T) {
	changelog := `# Changelog

## [Unreleased]

## [0.4.0-a1] - 2026-09-20

Summary.

### Added

- Add fingerprint.

[Unreleased]: https://github.com/dirloom/dirloom/compare/v0.4.0-a2...HEAD
[0.4.0-a1]: https://github.com/dirloom/dirloom/compare/v0.3.3...v0.4.0-a1
`
	if err := ValidateReleaseBranchFreeze("release/v0.4.0-a2", changelog); err == nil {
		t.Fatal("missing [0.4.0-a2] must fail on release/v0.4.0-a2")
	}
}
