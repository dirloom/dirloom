package presentation

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestBuiltInCatalogIsCompleteStableAndDefensive(t *testing.T) {
	if got, want := ThemeNames(), []string{"daylight", "default", "midnight"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %#v, want %#v", got, want)
	}
	wantPalettes := map[string]map[string]string{
		"default":  {"edge": "default", "directory": "ansi:blue", "file": "default", "symlink": "ansi:magenta", "accent": "ansi:cyan"},
		"midnight": {"edge": "#9AA5CE", "directory": "#7AA2F7", "file": "#C0CAF5", "symlink": "#BB9AF7", "accent": "#7DCFFF"},
		"daylight": {"edge": "#4B5563", "directory": "#1D4ED8", "file": "#111827", "symlink": "#6B21A8", "accent": "#0369A1"},
	}
	for name, palette := range wantPalettes {
		theme, ok := Lookup(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if theme.SchemaVersion != 1 || theme.Description == "" || !reflect.DeepEqual(theme.Palette, palette) {
			t.Errorf("theme %s = %#v", name, theme)
		}
		if len(theme.Tokens) != 4 || len(theme.Rules) == 0 || theme.Icons.Spacing != 1 {
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
	copyTheme.Rules[0].Icons.Unicode = "X"
	again, _ := Lookup("default")
	if again.Palette["edge"] != "default" || again.Tokens["node.file"].Color != "file" || again.Rules[0].Icons.Unicode == "X" {
		t.Fatal("catalog was mutated through returned values")
	}
}

func TestReferencePalettesMaintainReadableContrast(t *testing.T) {
	tests := []struct{ name, background string }{{"midnight", "#1A1B26"}, {"daylight", "#FFFFFF"}}
	for _, test := range tests {
		theme, _ := Lookup(test.name)
		for role, value := range theme.Palette {
			if ratio := contrastRatio(value, test.background); ratio < 4.5 {
				t.Errorf("%s %s contrast %.2f is below 4.5:1", test.name, role, ratio)
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
