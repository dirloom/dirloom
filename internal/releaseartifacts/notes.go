package releaseartifacts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	versionHeadingRE = regexp.MustCompile(`(?m)^## \[([^\]]+)\](?: - (.+))?\s*$`)
	sectionHeadingRE = regexp.MustCompile(`(?m)^### (.+)\s*$`)
	compareLinkRE    = regexp.MustCompile(`(?m)^\[([^\]]+)\]:\s+(\S+)\s*$`)
	dateRE           = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	releaseBranchRE  = regexp.MustCompile(`^release/v(.+)$`)
)

var allowedSections = []string{"Added", "Changed", "Fixed", "Security"}

// NormalizeVersion strips an optional leading "v" from a SemVer tag or version.
func NormalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// ExtractVersionSection returns the markdown for ## [version] through the next
// version heading (exclusive). The returned text includes the heading.
func ExtractVersionSection(changelog, version string) (string, error) {
	version = NormalizeVersion(version)
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	if strings.EqualFold(version, "Unreleased") {
		return "", fmt.Errorf("version must not be Unreleased")
	}
	matches := versionHeadingRE.FindAllStringSubmatchIndex(changelog, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("no version headings in changelog")
	}
	for i, match := range matches {
		name := changelog[match[2]:match[3]]
		if name != version {
			continue
		}
		start := match[0]
		end := len(changelog)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := trimCompareLinkFooter(changelog[start:end])
		section = strings.TrimRight(section, " \t\r\n") + "\n"
		if err := ValidateVersionSection(section, version); err != nil {
			return "", err
		}
		return section, nil
	}
	return "", fmt.Errorf("changelog section [%s] not found", version)
}

// ValidateVersionSection checks title, date, summary, and Allowed sections.
func ValidateVersionSection(section, version string) error {
	version = NormalizeVersion(version)
	lines := splitLines(section)
	if len(lines) == 0 {
		return fmt.Errorf("empty version section")
	}
	titleMatch := versionHeadingRE.FindStringSubmatch(lines[0])
	if titleMatch == nil {
		return fmt.Errorf("version section must start with ## [X.Y.Z] - YYYY-MM-DD")
	}
	if titleMatch[1] != version {
		return fmt.Errorf("section version %q does not match requested %q", titleMatch[1], version)
	}
	if strings.EqualFold(titleMatch[1], "Unreleased") {
		return fmt.Errorf("version title must not be Unreleased")
	}
	if titleMatch[2] == "" {
		return fmt.Errorf("version %s is missing a release date", version)
	}
	if strings.EqualFold(titleMatch[2], "Unreleased") {
		return fmt.Errorf("version %s date must not be Unreleased", version)
	}
	if !dateRE.MatchString(titleMatch[2]) {
		return fmt.Errorf("version %s has invalid date %q", version, titleMatch[2])
	}

	body := strings.TrimSpace(strings.TrimPrefix(section, lines[0]))
	if body == "" {
		return fmt.Errorf("version %s is missing a summary", version)
	}

	// Summary must appear before the first ### section (or be the whole body).
	firstSection := sectionHeadingRE.FindStringIndex(body)
	summary := body
	if firstSection != nil {
		summary = strings.TrimSpace(body[:firstSection[0]])
	}
	if summary == "" {
		return fmt.Errorf("version %s is missing a summary before sections", version)
	}

	seen := map[string]bool{}
	orderIndex := -1
	sectionMatches := sectionHeadingRE.FindAllStringSubmatchIndex(body, -1)
	for i, match := range sectionMatches {
		title := body[match[2]:match[3]]
		allowedIdx := indexOf(allowedSections, title)
		if allowedIdx < 0 {
			return fmt.Errorf("version %s has unknown section %q", version, title)
		}
		if seen[title] {
			return fmt.Errorf("version %s repeats section %q", version, title)
		}
		if allowedIdx < orderIndex {
			return fmt.Errorf("version %s sections out of order near %q", version, title)
		}
		orderIndex = allowedIdx
		seen[title] = true

		start := match[1]
		end := len(body)
		if i+1 < len(sectionMatches) {
			end = sectionMatches[i+1][0]
		}
		content := strings.TrimSpace(body[start:end])
		if content == "" || isEmptySectionContent(content) {
			return fmt.Errorf("version %s has empty section %q", version, title)
		}
	}
	return nil
}

// WriteReleaseNotes extracts and writes the version section to outputPath.
func WriteReleaseNotes(changelogPath, version, outputPath string) error {
	data, err := os.ReadFile(changelogPath) //nolint:gosec // Caller supplies the changelog path.
	if err != nil {
		return err
	}
	section, err := ExtractVersionSection(string(data), version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(section), 0o644) //nolint:gosec // Release notes are intentionally world-readable.
}

// ReleaseBranchVersion returns the SemVer (without leading v) encoded in a
// release/vX.Y.Z branch name.
func ReleaseBranchVersion(branch string) (string, bool) {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	match := releaseBranchRE.FindStringSubmatch(branch)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// ValidateReleaseBranchFreeze checks freeze rules for a release branch against
// a changelog document. Non-release branches return nil without checking.
func ValidateReleaseBranchFreeze(branch, changelog string) error {
	version, ok := ReleaseBranchVersion(branch)
	if !ok {
		return nil
	}
	if err := validateUnreleasedEmpty(changelog); err != nil {
		return err
	}
	section, err := ExtractVersionSection(changelog, version)
	if err != nil {
		return err
	}
	_ = section
	links := parseCompareLinks(changelog)
	unreleased, ok := links["Unreleased"]
	if !ok {
		return fmt.Errorf("missing [Unreleased] compare link")
	}
	wantUnreleased := fmt.Sprintf("https://github.com/dirloom/dirloom/compare/v%s...HEAD", version)
	if unreleased != wantUnreleased {
		return fmt.Errorf("[Unreleased] link = %s, want %s", unreleased, wantUnreleased)
	}
	versionLink, ok := links[version]
	if !ok {
		return fmt.Errorf("missing [%s] compare link", version)
	}
	wantSuffix := fmt.Sprintf("...v%s", version)
	if !strings.HasSuffix(versionLink, wantSuffix) {
		return fmt.Errorf("[%s] link = %s, want …%s", version, versionLink, wantSuffix)
	}
	if !strings.Contains(versionLink, "https://github.com/dirloom/dirloom/compare/") {
		return fmt.Errorf("[%s] link must be a GitHub compare URL", version)
	}
	return nil
}

func validateUnreleasedEmpty(changelog string) error {
	matches := versionHeadingRE.FindAllStringSubmatchIndex(changelog, -1)
	for i, match := range matches {
		name := changelog[match[2]:match[3]]
		if name != "Unreleased" {
			continue
		}
		start := match[1]
		end := len(changelog)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		body := strings.TrimSpace(changelog[start:end])
		// Strip trailing compare-link block if Unreleased is somehow last.
		if idx := strings.Index(body, "\n["); idx >= 0 {
			body = strings.TrimSpace(body[:idx])
		}
		if body != "" {
			return fmt.Errorf("[Unreleased] must be empty at freeze")
		}
		return nil
	}
	return fmt.Errorf("missing ## [Unreleased] heading")
}

func parseCompareLinks(changelog string) map[string]string {
	out := map[string]string{}
	for _, match := range compareLinkRE.FindAllStringSubmatch(changelog, -1) {
		out[match[1]] = match[2]
	}
	return out
}

// trimCompareLinkFooter drops the Keep a Changelog link reference block that
// follows the last version entry when no later ## heading exists.
func trimCompareLinkFooter(section string) string {
	lines := splitLines(section)
	cut := len(lines)
	for i := 0; i < len(lines); i++ {
		if compareLinkRE.MatchString(lines[i]) {
			cut = i
			break
		}
	}
	if cut == len(lines) {
		return section
	}
	return strings.Join(lines[:cut], "\n")
}

func isEmptySectionContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	if lower == "none." || lower == "none" || lower == "n/a" || lower == "- none." || lower == "- none" {
		return true
	}
	hasBullet := false
	for _, line := range splitLines(trimmed) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			hasBullet = true
			item := strings.TrimSpace(line[2:])
			if item == "" || strings.EqualFold(item, "None.") || strings.EqualFold(item, "None") {
				return true
			}
		}
	}
	return !hasBullet
}

func indexOf(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Split(text, "\n")
}
