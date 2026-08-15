package presentation

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestPublicThemeDocumentationUsesRealContracts(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "themes.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	themePattern := regexp.MustCompile(`(?s)<!-- dirloom-theme-example:([a-z-]+) -->\r?\n` + "```yaml" + `\r?\n(.*?)\r?\n` + "```")
	matches := themePattern.FindAllSubmatch(data, -1)
	if len(matches) != 1 || string(matches[0][1]) != "team" {
		t.Fatalf("theme examples = %d", len(matches))
	}
	theme, err := parseTheme(matches[0][2], path+"#team")
	if err != nil {
		t.Fatalf("documented theme is invalid: %v", err)
	}
	if theme.Name != "team" || theme.Appearance != "dark" || len(theme.Rules) < 3 || theme.Icons.Spacing != 1 {
		t.Fatalf("documented theme = %#v", theme)
	}
	if _, err := Compile(theme); err != nil {
		t.Fatalf("documented theme does not compile: %v", err)
	}

	commandPattern := regexp.MustCompile(`(?s)<!-- dirloom-theme-command:([a-z-]+) -->\r?\n` + "```bash" + `\r?\n(.*?)\r?\n` + "```")
	commands := commandPattern.FindAllSubmatch(data, -1)
	ids := make([]string, 0, len(commands))
	for _, command := range commands {
		ids = append(ids, string(command[1]))
		if !strings.Contains(string(command[2]), "dirloom") {
			t.Errorf("command %q does not invoke Dirloom", command[1])
		}
	}
	sort.Strings(ids)
	wantIDs := []string{"explain", "nerd", "no-color", "pipeline", "quick-start", "validate"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("command IDs = %#v, want %#v", ids, wantIDs)
	}

	for _, row := range []string{
		"| `default` | Universal | `default` | `ansi:blue` | `default` | `ansi:magenta` | `ansi:cyan` |",
		"| `midnight` | Dark (`#1A1B26` reference) | `#9AA5CE` | `#7AA2F7` | `#C0CAF5` | `#BB9AF7` | `#7DCFFF` |",
		"| `daylight` | Light (`#FFFFFF` reference) | `#4B5563` | `#1D4ED8` | `#111827` | `#6B21A8` | `#0369A1` |",
	} {
		if !strings.Contains(text, row) {
			t.Errorf("missing catalog row %q", row)
		}
	}
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "docs/themes.md") {
		t.Fatal("README does not link to docs/themes.md")
	}
}

func TestPublicThemeDocumentationRelativeLinksExist(t *testing.T) {
	paths := []string{
		filepath.Join("..", "..", "README.md"),
		filepath.Join("..", "..", "docs", "themes.md"),
		filepath.Join("..", "..", "docs", "configuration.md"),
		filepath.Join("..", "..", "docs", "presets.md"),
	}
	pattern := regexp.MustCompile(`\[[^\]]+\]\(([^):]+\.md)(?:#[^)]+)?\)`)
	for _, documentPath := range paths {
		data, err := os.ReadFile(documentPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range pattern.FindAllSubmatch(data, -1) {
			target := filepath.Clean(filepath.Join(filepath.Dir(documentPath), filepath.FromSlash(string(match[1]))))
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s links to missing %s: %v", documentPath, target, err)
			}
		}
	}
}
