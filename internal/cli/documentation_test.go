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
