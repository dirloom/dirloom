package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// assertContractGolden compares stdout with a golden under
// testdata/identity/golden. Set DIRLOOM_UPDATE_GOLDEN=1 to rewrite it.
func assertContractGolden(t *testing.T, name, stdout string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "identity", "golden", name)
	if os.Getenv("DIRLOOM_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(stdout), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) // #nosec G304 -- fixed test golden path.
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(want) {
		t.Fatalf("%s drift\n got %q\nwant %q", name, stdout, want)
	}
}

// TestTreeJSONV1CLIGolden pins tree JSON v1 as the CLI writes it for a real
// directory: scan, sort and render together. The renderer goldens of
// internal/render pin the encoding of every node shape.
func TestTreeJSONV1CLIGolden(t *testing.T) {
	for _, fixture := range []string{"nested", "unicode-nfc"} {
		root := filepath.Join("..", "..", "testdata", "identity", fixture)
		stdout, stderr, code := executeForTest(t, root, "--format", "json", "--no-config", "--no-default-ignore", "--no-gitignore")
		if code != 0 || stderr != "" {
			t.Fatalf("%s: %q %q %d", fixture, stdout, stderr, code)
		}
		assertContractGolden(t, fixture+".tree.json", stdout)
	}
}

// TestFingerprintJSONV1Golden pins the bytes of `fingerprint --format json`
// (schemaVersion 1).
func TestFingerprintJSONV1Golden(t *testing.T) {
	for _, fixture := range []string{"basic", "nested"} {
		root := filepath.Join("..", "..", "testdata", "identity", fixture)
		stdout, stderr, code := executeForTest(t, "fingerprint", root, "--format", "json", "--no-config", "--no-default-ignore", "--no-gitignore")
		if code != 0 || stderr != "" {
			t.Fatalf("%s: %q %q %d", fixture, stdout, stderr, code)
		}
		assertContractGolden(t, fixture+".fingerprint.json", stdout)
	}
}
