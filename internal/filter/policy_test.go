package filter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPolicyLayerPriority(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{".git", "other"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "important.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	git := NewGitIgnore()
	if err := git.AddPatterns([]byte("!.git/\n!important.log\n"), "", ".gitignore"); err != nil {
		t.Fatal(err)
	}
	explicit, err := NewIgnoreMatcher([]string{"*.log"})
	if err != nil {
		t.Fatal(err)
	}
	policy := NewPolicy("", true, explicit, git, true)

	gitEntry, gitInfo := readEntry(t, root, ".git")
	if !policy.Excludes(filepath.Join(root, ".git"), ".git", gitEntry, gitInfo) {
		t.Fatal("default exclusion must not be reversed by gitignore or --hidden")
	}
	logEntry, logInfo := readEntry(t, root, "important.log")
	if !policy.Excludes(filepath.Join(root, "important.log"), "important.log", logEntry, logInfo) {
		t.Fatal("explicit exclusion must not be reversed by gitignore")
	}
}

func TestPolicyHiddenAndOutputLayers(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".hidden", "output.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(root, "output.txt")
	policy := NewPolicy(output, false, nil, nil, false)

	hiddenEntry, hiddenInfo := readEntry(t, root, ".hidden")
	if !policy.Excludes(filepath.Join(root, ".hidden"), ".hidden", hiddenEntry, hiddenInfo) {
		t.Fatal("hidden entry should be excluded")
	}
	outputEntry, outputInfo := readEntry(t, root, "output.txt")
	if !policy.Excludes(output, "output.txt", outputEntry, outputInfo) {
		t.Fatal("output destination should be excluded")
	}
}

func readEntry(t *testing.T, root, name string) (os.DirEntry, os.FileInfo) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == name {
			info, infoErr := entry.Info()
			if infoErr != nil {
				t.Fatal(infoErr)
			}
			return entry, info
		}
	}
	t.Fatalf("entry %q not found", name)
	return nil, nil
}
