package presentation

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

func TestBuiltInCatalogIsCompleteStableAndDefensive(t *testing.T) {
	if got, want := ThemeNames(), []string{"daylight", "default", "midnight", "vivid"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %#v, want %#v", got, want)
	}
	wantPalettes := map[string]map[string]string{
		"default":  {"edge": "default", "directory": "ansi:blue", "file": "default", "symlink": "ansi:magenta", "accent": "ansi:cyan"},
		"midnight": {"edge": "#9AA5CE", "directory": "#7AA2F7", "file": "#C0CAF5", "symlink": "#BB9AF7", "accent": "#7DCFFF"},
		"daylight": {"edge": "#4B5563", "directory": "#1D4ED8", "file": "#111827", "symlink": "#6B21A8", "accent": "#0369A1"},
		"vivid":    {"edge": "#7A869E", "directory": "#44D7FF", "file": "#F1F5F9", "symlink": "#F38BFF", "accent": "#8B7CFF"},
	}
	for name, palette := range wantPalettes {
		theme, ok := Lookup(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if theme.SchemaVersion != 1 || theme.CatalogVersion != 1 || theme.Description == "" {
			t.Errorf("theme %s = %#v", name, theme)
		}
		for key, want := range palette {
			if got := theme.Palette[key]; got != want {
				t.Errorf("theme %s palette %s = %q, want %q", name, key, got, want)
			}
		}
		if len(theme.Tokens) != 4 || len(theme.Rules) != 0 || theme.Icons.Spacing != 1 {
			t.Errorf("incomplete theme %s: %#v", name, theme)
		}
		for token, value := range theme.Tokens {
			if _, err := resolveColor(value.Color, theme.Palette); err != nil {
				t.Errorf("%s %s: %v", name, token, err)
			}
			if value.Icons.Unicode != "" {
				if err := validateGlyph(value.Icons.Unicode); err != nil {
					t.Errorf("%s unicode: %v", token, err)
				}
			}
			if value.Icons.Nerd != "" {
				if err := validateGlyph(value.Icons.Nerd); err != nil {
					t.Errorf("%s nerd: %v", token, err)
				}
			}
		}
	}

	copyTheme, _ := Lookup("default")
	copyTheme.Palette["edge"] = "ansi:red"
	copyTheme.Tokens["node.file"] = Token{Color: "ansi:red"}
	copyTheme.Kinds["source"] = Binding{Color: "ansi:red"}
	again, _ := Lookup("default")
	if again.Palette["edge"] != "default" || again.Tokens["node.file"].Color != "file" || again.Kinds["source"].Color == "ansi:red" {
		t.Fatal("catalog was mutated through returned values")
	}
}

func TestVividThemeHasIndependentTwoToneIdentity(t *testing.T) {
	vivid, ok := Lookup(ThemeVivid)
	if !ok {
		t.Fatal("vivid theme is missing")
	}
	midnight, _ := Lookup(ThemeMidnight)
	for _, key := range []string{
		"edge", "file", "directory", "symlink", "accent", "security", "generated", "vendor", "test",
		"contract", "lock", "infra", "config", "executable", "archive", "media", "data", "source", "document", "tooling", "generic",
	} {
		if vivid.Palette[key] == midnight.Palette[key] {
			t.Errorf("vivid palette %s still aliases midnight", key)
		}
	}

	wantIconColors := map[string]string{
		"source": "icon-source", "manifest": "icon-manifest", "data": "icon-data", "document": "icon-document",
		"media": "icon-media", "archive": "icon-archive", "binary": "icon-binary", "directory": "icon-directory", "symlink": "icon-symlink",
	}
	for kind, want := range wantIconColors {
		if got := vivid.Kinds[kind].IconColor; got != want {
			t.Errorf("vivid kind %s iconColor = %q, want %q", kind, got, want)
		}
	}
	wantRoleStyles := map[string][]string{
		"security": {"bold", "underline"}, "generated": {"dim"}, "vendor": {"dim"},
		"test": {"bold"}, "contract": {"bold", "underline"}, "infra": {"bold"}, "executable": {"bold"},
	}
	for role, want := range wantRoleStyles {
		if got := vivid.Roles[role].Styles; !reflect.DeepEqual(got, want) {
			t.Errorf("vivid role %s styles = %#v, want %#v", role, got, want)
		}
	}

	compiled, err := Compile(vivid)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path, name string
		nodeType   tree.NodeType
		text, icon string
		styles     []string
	}{
		{path: "main.go", name: "main.go", nodeType: tree.NodeFile, text: "#66F0C0", icon: "#00FFD1"},
		{path: "main_test.go", name: "main_test.go", nodeType: tree.NodeFile, text: "#B6F36B", icon: "#00FFD1", styles: []string{"bold"}},
		{path: "README.md", name: "README.md", nodeType: tree.NodeFile, text: "#FFE066", icon: "#A78BFA", styles: []string{"bold", "underline"}},
		{path: "src", name: "src", nodeType: tree.NodeDirectory, text: "#66F0C0", icon: "#00D7FF", styles: []string{"bold"}},
	} {
		inspection := compiled.Inspect(test.path, test.name, test.nodeType)
		if inspection.TextColor != test.text || inspection.IconColor != test.icon || !reflect.DeepEqual(inspection.Styles, test.styles) {
			t.Errorf("vivid %s = %#v", test.path, inspection)
		}
	}
}

func TestReferencePalettesMaintainReadableContrast(t *testing.T) {
	tests := []struct{ name, background string }{{"midnight", "#1A1B26"}, {"daylight", "#FFFFFF"}, {"vivid", "#10131A"}}
	for _, test := range tests {
		theme, _ := Lookup(test.name)
		for key, value := range theme.Palette {
			if ratio := contrastRatio(value, test.background); ratio < 4.5 {
				t.Errorf("%s palette %s contrast %.2f is below 4.5:1", test.name, key, ratio)
			}
		}
	}
}

func contrastRatio(foreground, background string) float64 {
	left, right := luminance(foreground), luminance(background)
	if left < right {
		left, right = right, left
	}
	return (left + 0.05) / (right + 0.05)
}

func luminance(value string) float64 {
	raw := strings.TrimPrefix(value, "#")
	channels := make([]float64, 3)
	for index := range channels {
		number, _ := strconv.ParseUint(raw[index*2:index*2+2], 16, 8)
		channel := float64(number) / 255
		if channel <= 0.04045 {
			channels[index] = channel / 12.92
		} else {
			channels[index] = math.Pow((channel+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*channels[0] + 0.7152*channels[1] + 0.0722*channels[2]
}

func TestIconCatalogContainsDocumentedFallbacks(t *testing.T) {
	theme, _ := Lookup("default")
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ path, name, unicode, nerd string }{
		{"README.md", "README.md", "¶", "󰍔"},
		{"main.go", "main.go", "•", "󰟓"},
		{"app.ts", "app.ts", "•", "󰛦"},
		{"app.js", "app.js", "•", "󰌞"},
		{"data.json", "data.json", "◇", "󰘦"},
		{"config.yaml", "config.yaml", "◇", "󰈙"},
		{"Dockerfile", "Dockerfile", "▣", "󰡨"},
	}
	for _, test := range tests {
		style := compiled.resolve(test.path, test.name, "file")
		if style.icons.Unicode != test.unicode || style.icons.Nerd != test.nerd {
			t.Errorf("%s icons = %#v", test.name, style.icons)
		}
	}
}
