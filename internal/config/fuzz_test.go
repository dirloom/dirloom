package config

import (
	"reflect"
	"strings"
	"testing"
)

// FuzzConfigDocument is the configuration target of D9 (v0.4.0-rc1 plan):
// any document within the 1 MiB limit parses without panicking, the same
// way twice, and a refusal is always a configuration error (exit 2), never
// a runtime failure.
func FuzzConfigDocument(f *testing.F) {
	for _, seed := range []string{
		"schemaVersion: 1\n",
		"schemaVersion: 1\npreset: docs\ndefaults:\n  depth: 6\n  dirsOnly: false\n  hidden: false\n  format: markdown\n  style: ascii\nfilters:\n  useDefaultIgnores: true\n  useGitignore: true\nignore:\n  - generated/**\n  - \"*.log\"\npresentation:\n  color: auto\n  icons: unicode\n  theme: midnight\n",
		"schemaVersion: 1\nignore: [\"**/**/x\", \"a/**/b\"]\n",
		"schemaVersion: 1\ndefaults: {depth: unlimited}\n",
		"schemaVersion: 2\n",
		"schemaVersion: 1\nunknown: 1\n",
		"schemaVersion: 1\nschemaVersion: 1\n",
		"schemaVersion: 1\n---\nschemaVersion: 1\n",
		"a: &x 1\nb: *x\n",
		"<<: {schemaVersion: 1}\n",
		"schemaVersion: !!str 1\n",
		"schemaVersion: 1\npresentation: {theme: ./theme.yaml}\n",
		strings.Repeat("[", 200) + strings.Repeat("]", 200),
		"\xff\xfe",
		"",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxConfigSize {
			return // the loader refuses it before parsing
		}
		first, err := parseDocument(data, "fuzz.yaml")
		second, again := parseDocument(data, "fuzz.yaml")
		if (err == nil) != (again == nil) || (err != nil && err.Error() != again.Error()) || !reflect.DeepEqual(first, second) {
			t.Fatalf("parsing %q twice differs: %v, %v", data, err, again)
		}
		if err != nil && !IsInvalid(err) {
			t.Fatalf("parsing %q failed with %v, which is not a configuration error", data, err)
		}
	})
}
