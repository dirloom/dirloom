package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/presentation"
)

func TestASCIIIconsStayStrictAndOrthogonalToStyle(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	asciiTree, stderr, code := executeForTest(t, root, "--no-config", "--color", "never", "--style", "ascii", "--icons", "ascii")
	if code != 0 || stderr != "" {
		t.Fatalf("ascii style+icons=(%q,%q,%d)", asciiTree, stderr, code)
	}
	for _, want := range []string{"[DR]", "[SC]", "[DC]", "|--", "`--"} {
		if !strings.Contains(asciiTree, want) {
			t.Errorf("ascii tree missing %q\n%s", want, asciiTree)
		}
	}
	if strings.Contains(asciiTree, "├") || strings.Contains(asciiTree, "•") {
		t.Fatalf("ascii style leaked unicode geometry or glyphs:\n%s", asciiTree)
	}
	for index := 0; index < len(asciiTree); index++ {
		if asciiTree[index] >= 0x80 {
			t.Fatalf("ascii icons produced byte 0x%02X at %d in %q", asciiTree[index], index, asciiTree)
		}
	}

	unicodeTree, stderr, code := executeForTest(t, root, "--no-config", "--color", "never", "--style", "unicode", "--icons", "ascii")
	if code != 0 || stderr != "" {
		t.Fatalf("unicode style + ascii icons=(%q,%q,%d)", unicodeTree, stderr, code)
	}
	if !strings.Contains(unicodeTree, "[SC]") || !strings.Contains(unicodeTree, "├") || strings.Contains(unicodeTree, "|--") {
		t.Fatalf("style/icons orthogonality failed:\n%s", unicodeTree)
	}
	if strings.Contains(unicodeTree, "•") {
		t.Fatalf("ascii icons leaked unicode glyphs:\n%s", unicodeTree)
	}
}

func TestAutoIconsHonorDeclaredNerdFontOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("disabled")
	}))
	withoutCapability := presentation.NewEvaluator(
		presentation.WithEnvironment(func(string) (string, bool) { return "", false }),
		presentation.WithTerminalDetection(func(io.Writer) bool { return true }),
		presentation.WithANSIPreparation(func(io.Writer) (func() error, error) { return func() error { return nil }, nil }),
		presentation.WithWindowsTerminalCompatibility(false),
	)
	autoUnicode, stderr, code := executeForTestWithDependencies(t, loader, withoutCapability, root, "--no-config", "--color", "never", "--icons", "auto")
	if code != 0 || stderr != "" || !strings.Contains(autoUnicode, "•") || strings.Contains(autoUnicode, "󰟓") {
		t.Fatalf("auto without capability=(%q,%q,%d)", autoUnicode, stderr, code)
	}

	withCapability := presentation.NewEvaluator(
		presentation.WithEnvironment(func(name string) (string, bool) {
			if name == presentation.NerdFontEnvironmentVariable {
				return "1", true
			}
			return "", false
		}),
		presentation.WithTerminalDetection(func(io.Writer) bool { return true }),
		presentation.WithANSIPreparation(func(io.Writer) (func() error, error) { return func() error { return nil }, nil }),
		presentation.WithWindowsTerminalCompatibility(false),
	)
	autoNerd, stderr, code := executeForTestWithDependencies(t, loader, withCapability, root, "--no-config", "--color", "never", "--icons", "auto")
	if code != 0 || stderr != "" || !strings.Contains(autoNerd, "󰟓") {
		t.Fatalf("auto with capability=(%q,%q,%d)", autoNerd, stderr, code)
	}
	explicitUnicode, stderr, code := executeForTestWithDependencies(t, loader, withCapability, root, "--no-config", "--color", "never", "--icons", "unicode")
	if code != 0 || stderr != "" || !strings.Contains(explicitUnicode, "•") || strings.Contains(explicitUnicode, "󰟓") {
		t.Fatalf("explicit unicode must ignore capability=(%q,%q,%d)", explicitUnicode, stderr, code)
	}

	userBase := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userBase, "dirloom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userBase, "dirloom", "config.yaml"), []byte("schemaVersion: 1\nterminal:\n  capabilities:\n    nerdFont: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	userLoader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) { return userBase, nil }))
	fromUser, stderr, code := executeForTestWithDependencies(t, userLoader, withoutCapability, root, "--color", "never", "--icons", "auto")
	if code != 0 || stderr != "" || !strings.Contains(fromUser, "󰟓") {
		t.Fatalf("user config capability=(%q,%q,%d)", fromUser, stderr, code)
	}
}

func TestHelpDocumentsASCIIIconMode(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "--help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "never, ascii, unicode, nerd, or auto") {
		t.Fatalf("root help=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "help", "icons")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "ascii") || !strings.Contains(stdout, "DIRLOOM_NERD_FONT") {
		t.Fatalf("help icons=(%q,%q,%d)", stdout, stderr, code)
	}
}
