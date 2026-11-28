package releaseartifacts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The [0.4.0-rc1] section of CHANGELOG.md is prepared before its release
// date is known: the date is the actual release date, written before the tag,
// and never anticipated (CTO review of 2026-11-28). Until then the release
// workflow refuses to extract it. This test checks the rest now: with a date,
// the section is extractable, and the file passes the freeze rules of
// changelog-policy.md §6. The freeze may still change wording; it must not
// have to fix the structure.
func TestChangelogRC1SectionIsExtractableOnceDated(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	changelog := strings.ReplaceAll(string(data), "\r\n", "\n")

	heading := regexp.MustCompile(`(?m)^## \[0\.4\.0-rc1\](?: - (\d{4}-\d{2}-\d{2}))?$`).FindStringSubmatch(changelog)
	if heading == nil {
		t.Fatal("CHANGELOG.md has no `## [0.4.0-rc1]` heading, dated or not")
	}
	undated := heading[1] == ""
	if undated {
		if _, err := ExtractVersionSection(changelog, "v0.4.0-rc1"); err == nil || !strings.Contains(err.Error(), "missing a release date") {
			t.Fatalf("an undated section must not be extractable, got %v", err)
		}
		// Check the rest with a placeholder date, as if it were written.
		changelog = strings.Replace(changelog, "## [0.4.0-rc1]\n", "## [0.4.0-rc1] - 2000-01-01\n", 1)
	}
	section, err := ExtractVersionSection(changelog, "v0.4.0-rc1")
	if err != nil {
		t.Fatalf("the [0.4.0-rc1] section is not extractable once dated: %v", err)
	}
	// changelog-policy.md §3: the summary of a release candidate keeps its
	// boundary sentence.
	summary := section
	if i := strings.Index(section, "\n### "); i >= 0 {
		summary = section[:i]
	}
	if !strings.Contains(summary, "This increment does not add") {
		t.Error("the [0.4.0-rc1] summary lacks the boundary sentence `This increment does not add …`")
	}
	if undated {
		// Until the date is written, every rc1 change belongs to [0.4.0-rc1],
		// and [Unreleased] stays empty: the freeze rules must already hold.
		if err := ValidateReleaseBranchFreeze("release/v0.4.0-rc1", changelog); err != nil {
			t.Errorf("with a date, CHANGELOG.md would fail the freeze rules: %v", err)
		}
	}
	links := parseCompareLinks(changelog)
	if got, want := links["0.4.0-rc1"], "https://github.com/dirloom/dirloom/compare/v0.4.0-a8...v0.4.0-rc1"; got != want {
		t.Errorf("[0.4.0-rc1] compare link = %q, want %q", got, want)
	}
}
