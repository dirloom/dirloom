package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestTreeJSONRefusesInvalidUTF8JunctionTarget covers a real NTFS junction
// whose target directory name holds an unpaired UTF-16 surrogate. The target
// sits outside the root, so every name in the tree is valid and only the
// recorded target is refused. A junction needs no symlink privilege.
func TestTreeJSONRefusesInvalidUTF8JunctionTarget(t *testing.T) {
	outside := t.TempDir()
	target := createInvalidUTF8Entry(t, outside, "dir"+invalidUTF8Unit, true)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "link"), target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v\n%s", err, output)
	}
	if recorded, err := os.Readlink(filepath.Join(root, "link")); err != nil || recorded != target {
		t.Skipf("the junction target was rewritten: %q, %v", recorded, err)
	}
	assertTreeJSONRefused(t, `entry "link": target "`, root, "--no-config", "--format", "json")
	assertTreeJSONRefused(t, `dir`+escapedInvalidUTF8Unit+`" is not valid UTF-8`, root, "--no-config", "--format", "json")
}
