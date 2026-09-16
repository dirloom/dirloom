package presentation

import (
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/render"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestDecoratorColorIconAndFallback(t *testing.T) {
	theme, _ := Lookup("midnight")
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	context := render.NodeContext{Path: "src/main.go", Name: "main.go", Display: "main.go", Type: tree.NodeFile}
	decorator := NewDecorator(compiled, true, IconsNerd, ProfileTrueColor)
	got := decorator.Node(context)
	if got != "\x1b[38;2;122;162;247m󰟓\x1b[0m \x1b[38;2;122;162;247mmain.go\x1b[0m" {
		t.Fatalf("decorated = %q", got)
	}
	edge := decorator.Edge("├── ")
	if !strings.Contains(edge, "\x1b[38;2;154;165;206;2m") || !strings.HasSuffix(edge, "\x1b[0m") {
		t.Fatalf("edge = %q", edge)
	}

	theme.Kinds["file"] = Binding{Icons: IconPair{Unicode: "·"}, unicodeIconSet: true, nerdIconSet: true}
	compiled, err = Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	got = NewDecorator(compiled, false, IconsNerd, ProfileANSI16).Node(render.NodeContext{Path: "plain.bin", Name: "plain.bin", Display: "plain.bin", Type: tree.NodeFile})
	if got != "· plain.bin" {
		t.Fatalf("Nerd fallback = %q", got)
	}
}

func TestVividDecoratorUsesIndependentKindAndRoleSegments(t *testing.T) {
	theme, _ := Lookup(ThemeVivid)
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	decorator := NewDecorator(compiled, true, IconsNerd, ProfileTrueColor)

	got := decorator.Node(render.NodeContext{Path: "main.go", Name: "main.go", Display: "main.go", Type: tree.NodeFile})
	want := "\x1b[38;2;0;255;209m󰟓\x1b[0m \x1b[38;2;102;240;192mmain.go\x1b[0m"
	if got != want {
		t.Fatalf("vivid source = %q, want %q", got, want)
	}

	got = decorator.Node(render.NodeContext{Path: "main_test.go", Name: "main_test.go", Display: "main_test.go", Type: tree.NodeFile})
	want = "\x1b[38;2;0;255;209m󰟓\x1b[0m \x1b[38;2;182;243;107;1mmain_test.go\x1b[0m"
	if got != want {
		t.Fatalf("vivid test = %q, want %q", got, want)
	}
}
func TestDecoratorEscapesTerminalControlsOnlyInPresentation(t *testing.T) {
	theme, _ := Lookup("default")
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}
	decorator := NewDecorator(compiled, false, IconsUnicode, ProfileANSI16)
	got := decorator.Node(render.NodeContext{Path: "bad", Name: "bad", Display: "bad\x1b[31m\n", Type: tree.NodeFile})
	if strings.ContainsRune(got, '\x1b') || strings.ContainsRune(got, '\n') || !strings.Contains(got, `\u{001B}`) || !strings.Contains(got, `\u{000A}`) {
		t.Fatalf("escaped = %q", got)
	}
}

func TestColorProfilesAreDeterministic(t *testing.T) {
	color, err := parseLiteralColor("#7AA2F7")
	if err != nil {
		t.Fatal(err)
	}
	if got := colorCode(color, ProfileTrueColor); got != "38;2;122;162;247" {
		t.Fatalf("truecolor = %s", got)
	}
	if got := colorCode(color, ProfileANSI256); !strings.HasPrefix(got, "38;5;") {
		t.Fatalf("256 = %s", got)
	}
	if got := colorCode(color, ProfileANSI16); !strings.HasPrefix(got, "9") && !strings.HasPrefix(got, "3") {
		t.Fatalf("16 = %s", got)
	}
}
