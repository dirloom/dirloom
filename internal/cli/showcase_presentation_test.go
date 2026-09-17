package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestShowcaseVisualProjectionsAndCanonicalIsolation(t *testing.T) {
	goService := filepath.Join("..", "..", "testdata", "showcase", "go-service")
	nextApp := filepath.Join("..", "..", "testdata", "showcase", "typescript-next")
	for _, path := range []string{goService, nextApp} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("showcase corpus is missing %s: %v", path, err)
		}
	}

	neutral, stderr, code := executeForTest(t, goService, "--no-config", "--hidden", "--color", "never", "--icons", "never", "--theme", "default")
	if code != 0 || stderr != "" || strings.ContainsRune(neutral, '\x1b') || containsCatalogGlyph(neutral) {
		t.Fatalf("default + no icons=(%q,%q,%d)", neutral, stderr, code)
	}
	if !strings.Contains(neutral, "go.mod") || !strings.Contains(neutral, "main.go") {
		t.Fatalf("neutral tree is missing expected entries: %q", neutral)
	}

	midnight, stderr, code := executeForTest(t, goService, "--no-config", "--hidden", "--color", "always", "--icons", "unicode", "--theme", "midnight")
	if code != 0 || stderr != "" || !strings.ContainsRune(midnight, '\x1b') || !containsCatalogGlyph(midnight) {
		t.Fatalf("midnight + unicode=(%q,%q,%d)", midnight, stderr, code)
	}

	vivid, stderr, code := executeForTest(t, goService, "--no-config", "--hidden", "--color", "always", "--icons", "nerd", "--theme", "vivid")
	if code != 0 || stderr != "" || !strings.ContainsRune(vivid, '\x1b') || vivid == midnight || !containsPrivateUseGlyph(vivid) {
		t.Fatalf("vivid + nerd=(%q,%q,%d)", vivid, stderr, code)
	}

	nextTree, stderr, code := executeForTest(t, nextApp, "--no-config", "--no-default-ignore", "--hidden", "--color", "never", "--icons", "never")
	if code != 0 || stderr != "" || !strings.Contains(nextTree, ".next") || !strings.Contains(nextTree, "next.config.ts") || !strings.Contains(nextTree, "playwright.config.ts") {
		t.Fatalf("typescript-next=(%q,%q,%d)", nextTree, stderr, code)
	}

	for _, format := range []string{"markdown", "markdown-tree", "json", "mermaid", "graphviz", "d2"} {
		stdout, stderr, code := executeForTest(t, goService, "--no-config", "--hidden", "--format", format)
		if code != 0 || stderr != "" || strings.ContainsRune(stdout, '\x1b') || containsCatalogGlyph(stdout) || containsPrivateUseGlyph(stdout) {
			t.Errorf("canonical %s leaked presentation=(%q,%q,%d)", format, stdout, stderr, code)
		}
		if !strings.Contains(stdout, "go.mod") {
			t.Errorf("canonical %s is missing go.mod: %q", format, stdout)
		}
	}
}

func containsCatalogGlyph(value string) bool {
	for _, glyph := range []string{"¶", "•", "◇", "▸", "↗", "▪", "◆", "▣"} {
		if strings.Contains(value, glyph) {
			return true
		}
	}
	return false
}

func containsPrivateUseGlyph(value string) bool {
	for _, r := range value {
		if unicode.In(r, unicode.Co) {
			return true
		}
	}
	return false
}
