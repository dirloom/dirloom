package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkInspectSyntheticRepository(b *testing.B) {
	root := b.TempDir()
	for directory := 0; directory < 100; directory++ {
		dir := filepath.Join(root, fmt.Sprintf("package-%03d", directory))
		if err := os.Mkdir(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		for file := 0; file < 20; file++ {
			path := filepath.Join(dir, fmt.Sprintf("file-%03d.go", file))
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				b.Fatal(err)
			}
		}
	}

	request := InspectRequest{Root: root, UseDefaultIgnores: true, UseGitIgnore: true}
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := Inspect(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}
