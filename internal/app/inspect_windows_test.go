//go:build windows

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

func TestInspectLongUnicodeWindowsPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "projet espace é")
	current := root
	for len(current) < 280 {
		current = filepath.Join(current, "répertoire-long")
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Skipf("long paths are unavailable in this Windows environment: %v", err)
	}
	if err := os.WriteFile(filepath.Join(current, "fichier-你好.txt"), nil, 0o644); err != nil {
		t.Skipf("long Unicode filenames are unavailable: %v", err)
	}
	model, err := Inspect(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if !containsNode(model, "fichier-你好.txt") {
		t.Fatal("long Unicode path was not scanned")
	}
}

func containsNode(node *tree.Node, name string) bool {
	if node.Name == name {
		return true
	}
	for _, child := range node.Children {
		if containsNode(child, name) {
			return true
		}
	}
	return false
}
