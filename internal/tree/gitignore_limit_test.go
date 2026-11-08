package tree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/filter"
)

// TestLiveGitIgnoreIsLimitedToOneMiB: a live .gitignore of 1 MiB applies;
// one byte more fails the scan, as a .gitignore blob of a Git source does
// (D7 of the v0.4.0-rc1 plan: patterns accumulate across directories, and
// every entry is tested against all of them).
func TestLiveGitIgnoreIsLimitedToOneMiB(t *testing.T) {
	for _, size := range []int{MaxGitIgnoreBytes, MaxGitIgnoreBytes + 1} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
			t.Fatal(err)
		}
		// Short comment lines, then one rule, padded to size bytes. (The
		// parser stops at a line longer than 64 KiB.)
		rule := "*.log\n"
		line := "# " + strings.Repeat("x", 61) + "\n" // 64 bytes
		body := size - len(rule)
		content := strings.Repeat(line, body/len(line))
		if rest := body % len(line); rest > 0 {
			content += strings.Repeat("#", rest-1) + "\n"
		}
		content += rule
		if len(content) != size {
			t.Fatalf("built %d bytes, want %d", len(content), size)
		}
		if err := os.WriteFile(filepath.Join(root, "nested", ".gitignore"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "nested", "a.log"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		git := filter.NewGitIgnore()
		scan := NewScanner(ScanOptions{RootAbs: root, RootName: "root", UseGitIgnore: true,
			GitIgnore: git, FilterPolicy: filter.NewPolicy("", true, nil, git, false)})
		model, err := scan.Scan(context.Background())
		if size == MaxGitIgnoreBytes {
			if err != nil {
				t.Fatalf("a 1 MiB .gitignore: %v", err)
			}
			if names := childNames(model.Children[0]); len(names) != 0 {
				t.Fatalf("the 1 MiB .gitignore did not apply: %q", names)
			}
			continue
		}
		if err == nil || err.Error() != `read gitignore "nested/.gitignore": file exceeds the 1 MiB limit` {
			t.Fatalf("a .gitignore of 1 MiB + 1 byte: %v", err)
		}
	}
}

// TestReadFileStopsAfterItsLimit: whatever Lstat said, a .gitignore is
// never read beyond its limit plus one byte, the byte that tells it is too
// large (a file can grow between Lstat and the read).
func TestReadFileStopsAfterItsLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	if err := os.WriteFile(path, make([]byte, 3*MaxGitIgnoreBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := readFile(path, MaxGitIgnoreBytes)
	if err != nil || len(data) != MaxGitIgnoreBytes+1 {
		t.Fatalf("read %d bytes, %v; want %d", len(data), err, MaxGitIgnoreBytes+1)
	}
}

// TestScanLimitsCountNodesAndPaths: the limits a Git source sets apply to
// the scanner whatever its backend, root included, and only when positive.
func TestScanLimitsCountNodesAndPaths(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"aa", "bb", "cc"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scan := func(nodes, pathBytes int) error {
		_, err := NewScanner(ScanOptions{RootAbs: root, RootName: "root", NodeLimit: nodes, PathBytesLimit: pathBytes}).Scan(context.Background())
		return err
	}
	for _, tc := range []struct {
		nodes, pathBytes int
		want             string
	}{
		{0, 0, ""},
		{4, 6, ""},
		{3, 0, "the tree holds more than 3 nodes"},
		{0, 5, "the paths of the tree exceed 5 bytes"},
	} {
		err := scan(tc.nodes, tc.pathBytes)
		if (err == nil) != (tc.want == "") || (err != nil && err.Error() != tc.want) {
			t.Fatalf("limits (%d nodes, %d bytes): %v, want %q", tc.nodes, tc.pathBytes, err, tc.want)
		}
	}
}
