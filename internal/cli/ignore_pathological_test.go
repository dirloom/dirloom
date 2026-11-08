package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPathologicalIgnorePatternEndsQuickly runs the F4 case of the D7 audit
// of the v0.4.0-rc1 plan through the real commands: 2,000 "**" before a
// segment that never matches, on a tree three levels deep. The former
// recursive matcher needed about 10^12 calls per entry; snapshot keeps the
// pattern in its capture, and verify applies it again.
func TestPathologicalIgnorePatternEndsQuickly(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{{"a/b/c/file.txt", "x"}, {"top.txt", "y"}})
	pattern := strings.Repeat("**/", 2000) + "nothing"
	snap := filepath.Join(t.TempDir(), "ref.dlm.json")
	start := time.Now()
	if _, stderr, code := executeForTest(t, "snapshot", root, "--no-config", "--ignore", pattern, "--output", snap); code != 0 {
		t.Fatalf("snapshot = %d: %s", code, stderr)
	}
	if stdout, stderr, code := executeForTest(t, "verify", snap, root); code != 0 || stdout != "Structure matches snapshot.\n" {
		t.Fatalf("verify = (%q, %q, %d), want a match", stdout, stderr, code)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("snapshot and verify took %s", elapsed)
	}
}
