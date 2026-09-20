package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkFingerprintFilesystem(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		root := b.TempDir()
		writeSyntheticTree(b, root, size)
		request := InspectRequest{Root: root, UseDefaultIgnores: true}
		b.Run(fmt.Sprintf("%d_scan", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Inspect(context.Background(), request); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("%d_full", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Fingerprint(context.Background(), request); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func writeSyntheticTree(tb testing.TB, root string, n int) {
	tb.Helper()
	dirs := 20
	if n < dirs {
		dirs = n
	}
	per := n / dirs
	if per < 1 {
		per = 1
	}
	created := 0
	for d := 0; d < dirs && created < n; d++ {
		dir := filepath.Join(root, fmt.Sprintf("pkg-%03d", d))
		if err := os.Mkdir(dir, 0o755); err != nil {
			tb.Fatal(err)
		}
		created++
		for f := 0; f < per && created < n; f++ {
			path := filepath.Join(dir, fmt.Sprintf("file-%03d.go", f))
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				tb.Fatal(err)
			}
			created++
		}
	}
}
