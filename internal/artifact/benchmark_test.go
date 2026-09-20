package artifact

import (
	"strings"
	"testing"
)

func BenchmarkCanonicalize(b *testing.B) {
	path := strings.Repeat("dir/", 20) + "file.txt"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Canonicalize(path, POSIXSeparator); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidate10k(b *testing.B) {
	art := benchTree(10000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := art.Validate(); err != nil {
			b.Fatal(err)
		}
	}
}

func benchTree(n int) Artifact {
	root := Node{Path: RootPath, Name: ".", Kind: KindDirectory, Children: make([]Node, 0, n-1)}
	for i := 0; i < n-1; i++ {
		name := "f" + itoa(i)
		root.Children = append(root.Children, Node{Path: Path(name), Name: name, Kind: KindFile})
	}
	return Artifact{Root: root}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
