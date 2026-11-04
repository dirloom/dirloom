package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestThemeClassifyWindowsJunctionIsSymlink checks that classification agrees
// with a tree scan, which shows a junction as a symlink, under both modes Go
// can report for it.
func TestThemeClassifyWindowsJunctionIsSymlink(t *testing.T) {
	for name, godebug := range map[string]string{"default": "", "winsymlink-0": "winsymlink=0"} {
		// No "=" in the name: it reaches t.TempDir and the mklink command line.
		t.Run(name, func(t *testing.T) {
			t.Setenv("GODEBUG", godebug)
			root := t.TempDir()
			target := filepath.Join(root, "target")
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "link"), target).CombinedOutput(); err != nil {
				t.Fatalf("mklink /J: %v\n%s", err, output)
			}
			stdout, stderr, code := executeForTest(t, "theme", "classify", "link", "--root", root)
			if code != 0 || stderr != "" || !strings.Contains(stdout, "Type: symlink") {
				t.Fatalf("junction classification=(%q,%q,%d)", stdout, stderr, code)
			}
		})
	}
}
