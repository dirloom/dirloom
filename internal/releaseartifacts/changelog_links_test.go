package releaseartifacts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// rc1LinkPrefix is where the links of the [0.4.0-rc1] section point: the
// files of the tag itself. The release workflow copies the section into the
// body of the GitHub Release, where a relative link would resolve against the
// release page, and a link to a branch would drift after the release (CTO
// review of 2026-11-28).
const rc1LinkPrefix = "https://github.com/dirloom/dirloom/blob/v0.4.0-rc1/"

var markdownLinkRE = regexp.MustCompile(`\]\(([^)\s]+)\)`)

func TestChangelogRC1LinksArePinnedToTheTag(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	changelog := strings.ReplaceAll(string(data), "\r\n", "\n")
	start := strings.Index(changelog, "\n## [0.4.0-rc1]")
	if start < 0 {
		t.Fatal("CHANGELOG.md has no [0.4.0-rc1] section")
	}
	section := changelog[start+1:]
	if end := strings.Index(section, "\n## ["); end >= 0 {
		section = section[:end]
	}

	links := markdownLinkRE.FindAllStringSubmatch(section, -1)
	if len(links) == 0 {
		t.Fatal("the [0.4.0-rc1] section has no link: the test would prove nothing")
	}
	for _, link := range links {
		target := link[1]
		if !strings.HasPrefix(target, "https://") {
			t.Errorf("link %q is not absolute; write %s<path>", target, rc1LinkPrefix)
			continue
		}
		// Files of this repository must be read at the tag; its issues,
		// pull requests, releases and comparisons need no pin.
		if !strings.HasPrefix(target, "https://github.com/dirloom/dirloom/blob/") &&
			!strings.HasPrefix(target, "https://github.com/dirloom/dirloom/tree/") {
			continue
		}
		if !strings.HasPrefix(target, rc1LinkPrefix) {
			t.Errorf("link %q is not pinned to the tag; write %s<path>", target, rc1LinkPrefix)
			continue
		}
		// Until the tag exists, this tree is the one it will hold: the file,
		// and its anchor, must exist. Once v0.4.0-rc1 is tagged, its links
		// read the tag, and a later rename does not break them: this check
		// can then go.
		path, anchor, _ := strings.Cut(strings.TrimPrefix(target, rc1LinkPrefix), "#")
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("link %q: %v (once v0.4.0-rc1 is tagged, check the link against the tag instead)", target, err)
			continue
		}
		if anchor != "" && !hasHeadingAnchor(string(body), anchor) {
			t.Errorf("link %q: %s has no heading with the anchor %q (once v0.4.0-rc1 is tagged, check the link against the tag instead)", target, path, anchor)
		}
	}
}

// hasHeadingAnchor reports whether a Markdown document has a heading whose
// GitHub anchor is anchor: the heading text in lower case, without the
// characters other than letters, digits, spaces, hyphens and underscores,
// with each space turned into a hyphen.
func hasHeadingAnchor(markdown, anchor string) bool {
	inFence := false
	for _, line := range strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimSpace(strings.TrimLeft(line, "#"))
		var slug strings.Builder
		for _, r := range strings.ToLower(text) {
			switch {
			case r == ' ':
				slug.WriteRune('-')
			case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
				slug.WriteRune(r)
			}
		}
		if slug.String() == anchor {
			return true
		}
	}
	return false
}
