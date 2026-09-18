package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestPublicClipboardDocumentationMatchesCLI(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "clipboard-and-completions.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"dirloom --format markdown --copy",
		"dirloom --format json --copy",
		"dirloom completion bash",
		"dirloom completion powershell",
		"mutually exclusive",
		"no ANSI",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("clipboard guide missing %q", want)
		}
	}
	pattern := regexp.MustCompile(`(?s)<!-- dirloom-clipboard-command:([a-z-]+) -->\r?\n` + "```(?:bash)?\r?\n(.*?)\r?\n" + "```")
	matches := pattern.FindAllSubmatch(data, -1)
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, string(match[1]))
		if !strings.Contains(string(match[2]), "dirloom") {
			t.Errorf("clipboard command %q does not invoke Dirloom", match[1])
		}
	}
	wantIDs := []string{"completion-bash", "json", "markdown", "text"}
	sort.Strings(ids)
	if strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("clipboard command IDs = %#v, want %#v", ids, wantIDs)
	}
}

func TestPublicDistributionDocumentationMatchesReleaseContract(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "distribution.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"winget install Dirloom.Dirloom",
		"brew install --cask dirloom/tap/dirloom",
		"scoop install dirloom",
		"13 release artifacts",
		"exactly 12 hash",
		"Release Done",
		"Distribution Verified",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("distribution guide missing %q", want)
		}
	}
}

func TestREADMEAndUseCasesExposeCopyAndCompletions(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readmeText := string(readme)
	for _, want := range []string{
		"--copy",
		"dirloom completion bash",
		"docs/clipboard-and-completions.md",
		"docs/distribution.md",
		"winget install Dirloom.Dirloom",
		"brew install --cask dirloom/tap/dirloom",
	} {
		if !strings.Contains(readmeText, want) {
			t.Errorf("README missing %q", want)
		}
	}
	useCases, err := os.ReadFile(filepath.Join("..", "..", "docs", "use-cases.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(useCases), "| `--copy` natif |") {
		t.Fatal("use cases still list native --copy as unavailable")
	}
	if !strings.Contains(string(useCases), "dirloom --format markdown --copy") {
		t.Fatal("use cases missing native Markdown copy")
	}
}

func TestPublicHelpDocumentationMatchesCLIContracts(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile(filepath.Join("..", "..", "docs", "contextual-help.md"))
	if err != nil {
		t.Fatal(err)
	}
	themes, err := os.ReadFile(filepath.Join("..", "..", "docs", "themes.md"))
	if err != nil {
		t.Fatal(err)
	}
	completions, err := os.ReadFile(filepath.Join("..", "..", "docs", "clipboard-and-completions.md"))
	if err != nil {
		t.Fatal(err)
	}
	readmeText := string(readme)
	guideText := string(guide)
	themeText := string(themes)
	completionText := string(completions)
	for _, want := range []string{
		"dirloom help icons",
		"dirloom help topics",
		"docs/contextual-help.md",
		"dirloom --icons",
	} {
		if !strings.Contains(readmeText, want) {
			t.Errorf("README missing %q", want)
		}
	}
	for _, want := range []string{
		"dirloom --help",
		"dirloom help <topic>",
		"--icons",
		"--color",
		"never",
		"ascii",
		"unicode",
		"nerd",
		"auto",
		"always",
		"help vs explain",
	} {
		if !strings.Contains(guideText, want) {
			t.Errorf("contextual help guide missing %q", want)
		}
	}
	for _, mode := range []string{"never", "ascii", "unicode", "nerd", "auto"} {
		if !strings.Contains(themeText, mode) {
			t.Errorf("themes guide missing icon mode %q", mode)
		}
	}
	for _, mode := range []string{"never", "always", "auto"} {
		if !strings.Contains(themeText, mode) {
			t.Errorf("themes guide missing color mode %q", mode)
		}
	}
	if !strings.Contains(themeText, "dirloom help icons") || !strings.Contains(themeText, "without a value is equivalent") {
		t.Fatal("themes guide missing implicit --icons documentation")
	}
	if !strings.Contains(completionText, "dirloom help") || !strings.Contains(completionText, "help topics") {
		t.Fatal("completion guide missing contextual help completions")
	}
	for _, name := range helpTopicNames() {
		if !strings.Contains(guideText, name) {
			t.Errorf("contextual help guide missing topic %q", name)
		}
	}
}

func TestReleaseWorkflowDocumentsPre1Versioning(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "release-workflow.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"Pre-1.0 versioning policy",
		"Release branch integration",
		"0.Y.0",
		"0.Y.Z",
		"v0.3.0",
		"v0.3.1",
		"v0.3.2",
		"v0.4.0",
		"PRESENTATION",
		"CHANGE",
		"release/vX.Y.Z",
		"PR → release/vX.Y.Z",
		"PR finale → main",
		"Do not open versioned feature or fix pull requests against `main`.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("release workflow missing %q", want)
		}
	}
	roadmap, err := os.ReadFile(filepath.Join("..", "..", "docs", "product", "roadmap.md"))
	if err != nil {
		t.Fatal(err)
	}
	roadmapText := string(roadmap)
	if !strings.Contains(roadmapText, "v0.3.1") || !strings.Contains(roadmapText, "CLI GUIDANCE") {
		t.Fatal("roadmap missing v0.3.1 CLI guidance refinement")
	}
	if strings.Contains(roadmapText, "v0.4.0-beta") || strings.Contains(roadmapText, "v0.4.0-b") {
		t.Fatal("roadmap must not present contextual help as a v0.4.0 beta")
	}
}
