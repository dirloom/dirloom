package identity

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
)

func BenchmarkProjectEncodeHash(b *testing.B) {
	for _, size := range []int{1000, 10000, 100000} {
		art := syntheticArtifact(size, "balanced")
		b.Run(label(size, "pipeline"), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Compute(art); err != nil {
					b.Fatal(err)
				}
			}
		})
		records, err := ProjectV1(art)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(label(size, "project"), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ProjectV1(art); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(label(size, "encode"), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := EncodeV1(discard{}, records); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	for _, profile := range []string{"deep", "wide"} {
		size := 10000
		if profile == "deep" {
			size = 256
		}
		art := syntheticArtifact(size, profile)
		b.Run(profile, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Compute(art); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSHA256(b *testing.B) {
	art := syntheticArtifact(10000, "balanced")
	records, err := ProjectV1(art)
	if err != nil {
		b.Fatal(err)
	}
	var buf bytes.Buffer
	if err := EncodeV1(&buf, records); err != nil {
		b.Fatal(err)
	}
	payload := buf.Bytes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sha256.Sum256(payload)
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func label(size int, name string) string {
	switch size {
	case 1000:
		return "1k_" + name
	case 10000:
		return "10k_" + name
	case 100000:
		return "100k_" + name
	default:
		return name
	}
}

func syntheticArtifact(n int, profile string) artifact.Artifact {
	root := artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}
	switch profile {
	case "deep":
		current := &root
		path := ""
		for i := 0; i < n; i++ {
			name := "d"
			childPath := name
			if path != "" {
				childPath = path + "/" + name
			}
			kind := artifact.KindDirectory
			if i == n-1 {
				kind = artifact.KindFile
				name = "leaf.txt"
				if path == "" {
					childPath = name
				} else {
					childPath = path + "/" + name
				}
			}
			child := artifact.Node{Path: artifact.Path(childPath), Name: name, Kind: kind}
			current.Children = []artifact.Node{child}
			current = &current.Children[0]
			path = childPath
			if kind != artifact.KindDirectory {
				break
			}
		}
	case "wide":
		root.Children = make([]artifact.Node, 0, n-1)
		for i := 0; i < n-1; i++ {
			name := wideName(i)
			root.Children = append(root.Children, artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindFile})
		}
	default:
		remaining := n - 1
		dirs := 32
		if remaining < dirs {
			dirs = remaining
		}
		if dirs == 0 {
			return artifact.Artifact{Root: root}
		}
		perDir := remaining / dirs
		if perDir == 0 {
			perDir = 1
		}
		root.Children = make([]artifact.Node, 0, dirs)
		created := 0
		for d := 0; d < dirs && created < remaining; d++ {
			dirName := wideName(d)
			dir := artifact.Node{Path: artifact.Path(dirName), Name: dirName, Kind: artifact.KindDirectory}
			budget := perDir
			if d == dirs-1 {
				budget = remaining - created - 1
			}
			if budget < 0 {
				budget = 0
			}
			for f := 0; f < budget && created+1 < remaining; f++ {
				fileName := wideName(f)
				dir.Children = append(dir.Children, artifact.Node{
					Path: artifact.Path(dirName + "/" + fileName),
					Name: fileName,
					Kind: artifact.KindFile,
				})
				created++
			}
			root.Children = append(root.Children, dir)
			created++
		}
	}
	return artifact.Artifact{Root: root}
}

func wideName(i int) string {
	return "n" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[n:])
}

func TestSyntheticArtifactSizes(t *testing.T) {
	if got := syntheticArtifact(1000, "balanced").CountNodes(); got < 900 {
		t.Fatalf("balanced 1k = %d", got)
	}
	if got := syntheticArtifact(1000, "wide").CountNodes(); got != 1000 {
		t.Fatalf("wide = %d", got)
	}
}
