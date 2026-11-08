package presentation

import (
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

// FuzzThemeDocument covers the theme file, a gap of the D9 targets found by
// the D7 audit (v0.4.0-rc1 plan): any document within the 1 MiB limit
// parses without panicking, the same way twice, and a refusal is always an
// invalid-theme error. An accepted theme compiles, and its rules resolve a
// few paths, without panicking.
func FuzzThemeDocument(f *testing.F) {
	for _, seed := range []string{
		validCustomTheme,
		"schemaVersion: 1\ncatalogVersion: 1\nname: bad\nappearance: dark\n",
		"schemaVersion: 1\ncatalogVersion: 1\nname: t\nappearance: dark\nrules:\n  - match: {glob: \"**/**/*.go\"}\n    style: bold\n",
		"schemaVersion: 1\ncatalogVersion: 1\nname: t\nappearance: light\nicons: {spacing: 9}\n",
		"schemaVersion: 1\npalette: {a: \"#zzz\"}\n",
		"a: &x 1\nb: *x\n",
		strings.Repeat("{a: ", 100) + "1" + strings.Repeat("}", 100),
		"\xff",
		"",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxThemeSize {
			return // the loader refuses it before parsing
		}
		theme, err := parseTheme(data, "fuzz.yaml")
		_, again := parseTheme(data, "fuzz.yaml")
		if (err == nil) != (again == nil) || (err != nil && err.Error() != again.Error()) {
			t.Fatalf("parsing %q twice differs: %v, %v", data, err, again)
		}
		if err != nil {
			if !IsInvalid(err) {
				t.Fatalf("parsing %q failed with %v, which is not an invalid theme", data, err)
			}
			return
		}
		compiled, err := Compile(theme)
		if err != nil {
			return
		}
		for _, path := range []string{"README.md", "src/main.go", "a/b/c/d/e.txt", ".hidden", "dir"} {
			name := path[strings.LastIndex(path, "/")+1:]
			compiled.resolve(path, name, tree.NodeFile)
			compiled.resolve(path, name, tree.NodeDirectory)
		}
	})
}
