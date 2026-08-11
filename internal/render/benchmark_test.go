package render

import (
	"io"
	"testing"
)

func BenchmarkUnicodeRenderer(b *testing.B) {
	renderer, err := New(FormatText, StyleUnicode)
	if err != nil {
		b.Fatal(err)
	}
	model := sampleTree()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if err := renderer.Render(io.Discard, model); err != nil {
			b.Fatal(err)
		}
	}
}
