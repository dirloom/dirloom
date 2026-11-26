package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1" // #nosec G505 -- Git object ids are SHA-1 in a sha1 repository.
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/buildinfo"
	"github.com/dirloom/dirloom/internal/cli"
	"github.com/dirloom/dirloom/internal/snapshot"
)

// Fixture sizes. "Nodes" always counts every node of the artifact: the
// root, the directories and the files.
const (
	nodes100k = 100_000
	nodes1m   = 1_000_000
	nodes2m   = snapshot.MaxNodes

	// filesPerDirectory is the shape of the snapshot limit measurements
	// (docs/benchmarks/v0.4-rc1-snapshot-limits.md): up to 1000 files per
	// directory, about 82 bytes per node in a snapshot document.
	filesPerDirectory = 1000

	// longPad makes the "long" document: 2,000,000 nodes of about 132 bytes
	// each, 252 MiB, the most expensive valid document within both the byte
	// and the node limit.
	longPad = "-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	// overBytes is the size of the document above the 256 MiB limit.
	overBytes = 271 << 20

	// gitDate is the fixed commit date of every generated repository, so
	// that the commit ids are reproducible.
	gitDate = 1767225600 // 2026-01-01T00:00:00Z
)

// fixture is one generated input. Its files live under root/Name.
type fixture struct {
	Name  string
	About string
	Make  func(root string) (map[string]any, error)
}

// fixtures lists every input of the scenarios, in generation order: a
// fixture may read another one generated before it.
func fixtures() []fixture {
	return []fixture{
		{"snap-wide-100k", "snapshots of 100,000 nodes: a, and b with every 10th directory relocated", func(r string) (map[string]any, error) {
			return writeSnapshots(r, "snap-wide-100k", nodes100k, "", map[string]int{"a": 0, "b10": 10})
		}},
		{"snap-wide-1m", "snapshots of 1,000,000 nodes: a, b10", func(r string) (map[string]any, error) {
			return writeSnapshots(r, "snap-wide-1m", nodes1m, "", map[string]int{"a": 0, "b10": 10})
		}},
		{"snap-wide-2m", "snapshots of 2,000,000 nodes, the limit: a, b10, and b50 with every 2nd directory relocated", func(r string) (map[string]any, error) {
			return writeSnapshots(r, "snap-wide-2m", nodes2m, "", map[string]int{"a": 0, "b10": 10, "b50": 2})
		}},
		{"snap-long-2m", "snapshots of 2,000,000 nodes with long names, about 252 MiB: a, b10", func(r string) (map[string]any, error) {
			return writeSnapshots(r, "snap-long-2m", nodes2m, longPad, map[string]int{"a": 0, "b10": 10})
		}},
		{"bad", "malformed documents close to the snapshot limits", writeBadSnapshots},
		{"live-empty", "an empty directory", func(r string) (map[string]any, error) {
			return map[string]any{"nodes": 1}, os.MkdirAll(filepath.Join(r, "live-empty"), 0o750)
		}},
		{"live-wide-100k", "a directory of 100,000 nodes on disk, wide shape", func(r string) (map[string]any, error) {
			return writeWideTree(filepath.Join(r, "live-wide-100k"), nodes100k)
		}},
		{"live-wide-1m", "a directory of 1,000,000 nodes on disk, wide shape", func(r string) (map[string]any, error) {
			return writeWideTree(filepath.Join(r, "live-wide-1m"), nodes1m)
		}},
		{"live-wide-2m", "a directory of 2,000,000 nodes on disk, wide shape", func(r string) (map[string]any, error) {
			return writeWideTree(filepath.Join(r, "live-wide-2m"), nodes2m)
		}},
		{"live-deep-100k", "a directory of 100,001 nodes on disk: 100 chains of 50 nested directories", func(r string) (map[string]any, error) {
			return writeDeepTree(filepath.Join(r, "live-deep-100k"))
		}},
		{"live-gi-short", "100,000 nodes and a root .gitignore of 1 MiB of short patterns", func(r string) (map[string]any, error) {
			return writeGitIgnoreTree(filepath.Join(r, "live-gi-short"), shortGitIgnore())
		}},
		{"live-gi-long", "100,000 nodes and a root .gitignore of 16 lines of 65,000 bytes", func(r string) (map[string]any, error) {
			return writeGitIgnoreTree(filepath.Join(r, "live-gi-long"), longGitIgnore('x'))
		}},
		{"live-gi-wild", "100,000 nodes and a root .gitignore of 16 lines of 65,000 bytes of wildcards", func(r string) (map[string]any, error) {
			return writeGitIgnoreTree(filepath.Join(r, "live-gi-wild"), longGitIgnore('*'))
		}},
		{"live-watch", "a directory of 100,000 nodes for watch, with an empty bursts directory", func(r string) (map[string]any, error) {
			dir := filepath.Join(r, "live-watch")
			info, err := writeWideTree(dir, nodes100k-1)
			if err != nil {
				return nil, err
			}
			return info, os.MkdirAll(filepath.Join(dir, "bursts"), 0o750)
		}},
		{"snaplive", "snapshots of the live trees, written by dirloom snapshot", writeLiveSnapshots},
		{"git-wide-1m", "a repository: HEAD~1 of 999,999 nodes, HEAD with every 10th directory relocated", func(r string) (map[string]any, error) {
			return writeWideRepository(filepath.Join(r, "git-wide-1m"), nodes1m)
		}},
		{"git-wide-2m", "a repository: HEAD~1 of 1,999,999 nodes, HEAD of 2,000,000 nodes with every 10th directory relocated", func(r string) (map[string]any, error) {
			return writeWideRepository(filepath.Join(r, "git-wide-2m"), nodes2m)
		}},
		{"git-bomb", "a repository whose tree shares subtrees: 2,306,353 nodes from a few kilobytes", func(r string) (map[string]any, error) {
			return writeBombRepository(filepath.Join(r, "git-bomb"))
		}},
		{"git-chain", "a repository of directory chains: main~1 and main just under 256 MiB of paths, over just above", func(r string) (map[string]any, error) {
			return writeChainRepository(filepath.Join(r, "git-chain"))
		}},
		{"git-history-10k", "a repository of 2,000 commits over a tree of about 10,000 entries", func(r string) (map[string]any, error) {
			return writeHistoryRepository(filepath.Join(r, "git-history-10k"), 10_000, 2_000)
		}},
		{"git-history-100k", "a repository of 200 commits over a tree of about 100,000 entries", func(r string) (map[string]any, error) {
			return writeHistoryRepository(filepath.Join(r, "git-history-100k"), 100_000, 200)
		}},
	}
}

func runGen(args []string) error {
	flags := flag.NewFlagSet("gen", flag.ContinueOnError)
	root := flags.String("fixtures", "", "directory of the fixtures (created)")
	only := flags.String("only", "", "comma-separated fixture names (default: all)")
	force := flags.Bool("force", false, "regenerate fixtures that already exist")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *root == "" {
		return errors.New("gen: -fixtures is required")
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return err
	}
	want := map[string]bool{}
	for _, name := range strings.Split(*only, ",") {
		if name = strings.TrimSpace(name); name != "" {
			want[name] = true
		}
	}
	manifest, err := readManifest(abs)
	if err != nil {
		return err
	}
	unknown := maps.Clone(want)
	for _, f := range fixtures() {
		if len(want) > 0 && !want[f.Name] {
			continue
		}
		delete(unknown, f.Name)
		if _, done := manifest[f.Name]; done && !*force {
			fmt.Fprintf(os.Stderr, "gen: %s exists, skipped\n", f.Name)
			continue
		}
		if err := os.RemoveAll(filepath.Join(abs, f.Name)); err != nil {
			return err
		}
		start := time.Now()
		fmt.Fprintf(os.Stderr, "gen: %s: %s\n", f.Name, f.About)
		info, err := f.Make(abs)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		if info == nil {
			info = map[string]any{}
		}
		info["about"] = f.About
		info["seconds"] = time.Since(start).Round(time.Millisecond).Seconds()
		manifest[f.Name] = info
		if err := writeManifest(abs, manifest); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "gen: %s done in %s\n", f.Name, time.Since(start).Round(time.Millisecond))
		runtime.GC()
	}
	if len(unknown) > 0 {
		return fmt.Errorf("gen: unknown fixtures: %v", sortedKeys(unknown))
	}
	return nil
}

const manifestName = "manifest.json"

func readManifest(root string) (map[string]map[string]any, error) {
	manifest := map[string]map[string]any{}
	data, err := os.ReadFile(filepath.Join(root, manifestName)) // #nosec G304 -- the fixture directory given on the command line.
	if errors.Is(err, os.ErrNotExist) {
		return manifest, nil
	}
	if err != nil {
		return nil, err
	}
	return manifest, json.Unmarshal(data, &manifest)
}

func writeManifest(root string, manifest map[string]map[string]any) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, manifestName), append(data, '\n'), 0o600)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// wideArtifact returns the shape of the snapshot limit measurements:
// directories d00000, d00001, ... of up to 1000 files named
// file-000000<pad>.txt, filled to exactly nodes nodes with the root and an
// empty zz-moved directory. With relocateEvery > 0, every relocateEvery-th
// directory (d00000 first) sits under zz-moved instead of the root: the
// node count is the same, and each relocated directory keeps a unique name
// and its contents, so Move Detection structural-v1 pairs it.
func wideArtifact(nodes, relocateEvery int, pad string) artifact.Artifact {
	root := artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}
	moved := artifact.Node{Path: "zz-moved", Name: "zz-moved", Kind: artifact.KindDirectory}
	count := 2
	for d := 0; count < nodes; d++ {
		name := fmt.Sprintf("d%05d", d)
		parent := ""
		if relocateEvery > 0 && d%relocateEvery == 0 {
			parent = "zz-moved/"
		}
		dir := artifact.Node{Path: artifact.Path(parent + name), Name: name, Kind: artifact.KindDirectory}
		count++
		for f := 0; f < filesPerDirectory && count < nodes; f++ {
			file := fmt.Sprintf("file-%06d%s.txt", f, pad)
			dir.Children = append(dir.Children, artifact.Node{Path: artifact.Path(parent + name + "/" + file), Name: file, Kind: artifact.KindFile})
			count++
		}
		if parent == "" {
			root.Children = append(root.Children, dir)
		} else {
			moved.Children = append(moved.Children, dir)
		}
	}
	root.Children = append(root.Children, moved)
	return artifact.Artifact{Root: root}
}

func countNodes(n artifact.Node) int {
	total := 1
	for _, c := range n.Children {
		total += countNodes(c)
	}
	return total
}

// defaultCapture is the Capture Semantics of a plain "dirloom snapshot".
func defaultCapture() snapshot.CaptureV1 {
	return snapshot.CaptureV1{UseDefaultIgnores: true, UseGitignore: true, Ignore: []string{}}
}

func writeSnapshotFile(path string, art artifact.Artifact, capture snapshot.CaptureV1) (map[string]any, error) {
	doc, err := snapshot.Build(art, capture)
	if err != nil {
		return nil, err
	}
	data, err := snapshot.Marshal(doc)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return map[string]any{"bytes": len(data), "nodes": countNodes(art.Root), "sha256": hex.EncodeToString(sum[:])}, nil
}

// writeEditedSnapshot writes the snapshot of art after edit changed its
// bytes.
func writeEditedSnapshot(path string, art artifact.Artifact, edit func([]byte) ([]byte, error)) (map[string]any, error) {
	doc, err := snapshot.Build(art, defaultCapture())
	if err != nil {
		return nil, err
	}
	data, err := snapshot.Marshal(doc)
	if err != nil {
		return nil, err
	}
	if data, err = edit(data); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return map[string]any{"bytes": len(data), "sha256": hex.EncodeToString(sum[:])}, nil
}

func writeSnapshots(root, name string, nodes int, pad string, variants map[string]int) (map[string]any, error) {
	info := map[string]any{}
	for _, variant := range sortedKeys(variants) {
		art := wideArtifact(nodes, variants[variant], pad)
		file, err := writeSnapshotFile(filepath.Join(root, name, variant+".json"), art, defaultCapture())
		if err != nil {
			return nil, err
		}
		if size, _ := file["bytes"].(int); size > snapshot.MaxBytes {
			return nil, fmt.Errorf("%s/%s.json is %d bytes, above the %d-byte limit", name, variant, size, snapshot.MaxBytes)
		}
		info[variant] = file
		runtime.GC()
	}
	return info, nil
}

// writeBadSnapshots writes the malformed and limit documents:
//   - keys.json: 250 MiB of one JSON object of unique keys, finding F6 of
//     the LOT 5 audit (the walk limits arrays, not keys);
//   - nodes-over.json: 2,000,001 nodes, one above the node limit;
//   - bytes-over.json: a valid 2,000,000-node document padded to 271 MiB;
//   - nesting.json: 3,000,000 nested arrays, 6 MB;
//   - ignore-4096.json: a valid snapshot of an empty directory whose
//     capture.ignore has 4,096 patterns, the limit, for verify against a
//     live tree.
func writeBadSnapshots(root string) (map[string]any, error) {
	dir := filepath.Join(root, "bad")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	info := map[string]any{}

	var keys bytes.Buffer
	const keysTarget = 250 << 20
	keys.Grow(keysTarget + 64)
	keys.WriteString(`{"schemaVersion":1`)
	for i := 0; keys.Len() < keysTarget-32; i++ {
		fmt.Fprintf(&keys, `,"k%09d":0`, i)
	}
	keys.WriteString("}\n")
	if err := os.WriteFile(filepath.Join(dir, "keys.json"), keys.Bytes(), 0o600); err != nil {
		return nil, err
	}
	info["keys"] = map[string]any{"bytes": keys.Len()}
	keys = bytes.Buffer{}

	// Dirloom never writes a snapshot it would refuse to read, so the two
	// documents above a limit are edited as text: one more node in the
	// array of a valid 2,000,000-node document, and the valid long
	// document followed by whitespace up to about 271 MiB.
	var err error
	if info["nodes-over"], err = writeEditedSnapshot(filepath.Join(dir, "nodes-over.json"), wideArtifact(nodes2m, 0, ""), func(doc []byte) ([]byte, error) {
		end := bytes.LastIndex(doc, []byte("\n    ]"))
		if end < 0 {
			return nil, errors.New("no end of the node array")
		}
		extra := []byte(",\n      {\n        \"path\": \"zz-extra.txt\",\n        \"kind\": \"file\"\n      }")
		return slices.Concat(doc[:end], extra, doc[end:]), nil
	}); err != nil {
		return nil, err
	}
	runtime.GC()
	if info["bytes-over"], err = writeEditedSnapshot(filepath.Join(dir, "bytes-over.json"), wideArtifact(nodes2m, 0, longPad), func(doc []byte) ([]byte, error) {
		return append(doc, bytes.Repeat([]byte(" "), overBytes-len(doc))...), nil
	}); err != nil {
		return nil, err
	}
	runtime.GC()

	const depth = 3_000_000
	nesting := append(bytes.Repeat([]byte("["), depth), bytes.Repeat([]byte("]"), depth)...)
	if err := os.WriteFile(filepath.Join(dir, "nesting.json"), nesting, 0o600); err != nil {
		return nil, err
	}
	info["nesting"] = map[string]any{"bytes": len(nesting)}

	capture := defaultCapture()
	capture.Ignore = ignorePatterns(snapshot.MaxIgnorePatterns)
	empty := artifact.Artifact{Root: artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}}
	if info["ignore-4096"], err = writeSnapshotFile(filepath.Join(dir, "ignore-4096.json"), empty, capture); err != nil {
		return nil, err
	}
	return info, nil
}

// ignorePatterns returns n patterns that match no entry of the generated
// trees, so every entry is tested against all of them. They mix the forms
// a real project uses: extensions, directories, anchored paths and **.
func ignorePatterns(n int) []string {
	patterns := make([]string, n)
	for i := range patterns {
		switch i % 4 {
		case 0:
			patterns[i] = fmt.Sprintf("*.ext%05d", i)
		case 1:
			patterns[i] = fmt.Sprintf("gen%05d", i)
		case 2:
			patterns[i] = fmt.Sprintf("**/tmp%05d/**", i)
		default:
			patterns[i] = fmt.Sprintf("build/**/out%05d-*.txt", i)
		}
	}
	return patterns
}

// shortGitIgnore returns about 1 MiB of short .gitignore patterns, below
// the 1 MiB limit, none of which matches an entry of the generated trees.
func shortGitIgnore() []byte {
	var b bytes.Buffer
	for i := 0; ; i++ {
		var line string
		switch i % 4 {
		case 0:
			line = fmt.Sprintf("*.ext%06d\n", i)
		case 1:
			line = fmt.Sprintf("gen%06d/\n", i)
		case 2:
			line = fmt.Sprintf("**/tmp%06d/**\n", i)
		default:
			line = fmt.Sprintf("/build%06d\n", i)
		}
		if b.Len()+len(line) > 1<<20 {
			return b.Bytes()
		}
		b.WriteString(line)
	}
}

// longGitIgnore returns 16 lines of 65,000 bytes, each under the 65,535
// byte line limit, 1,040,016 bytes in all. With fill 'x' a line is a long
// literal directory pattern; with fill '*' it alternates wildcards and
// letters, the costliest form for a wildcard matcher.
func longGitIgnore(fill byte) []byte {
	var b bytes.Buffer
	for i := 0; i < 16; i++ {
		line := make([]byte, 65_000)
		for j := range line {
			line[j] = 'x'
			if fill == '*' && j%2 == 0 {
				line[j] = '*'
			}
		}
		copy(line, fmt.Sprintf("l%02d", i))
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// writeWideTree creates the wide shape on disk: the files are empty.
func writeWideTree(dir string, nodes int) (map[string]any, error) {
	art := wideArtifact(nodes, 0, "")
	// The zz-moved directory of the snapshot shape is kept: it makes a live
	// tree of n nodes the same tree as the snapshot "a" of n nodes.
	return writeArtifactTree(dir, art)
}

// writeDeepTree creates 100 chains c000..c099 of 50 nested directories
// "a"; each directory of a chain holds 19 empty files f00..f18. With the
// root: 100 × 50 × 20 + 1 = 100,001 nodes, paths up to 106 bytes.
func writeDeepTree(dir string) (map[string]any, error) {
	root := artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}
	for c := 0; c < 100; c++ {
		name := fmt.Sprintf("c%03d", c)
		root.Children = append(root.Children, deepChain(name, name, 50))
	}
	return writeArtifactTree(dir, artifact.Artifact{Root: root})
}

func deepChain(path, name string, levels int) artifact.Node {
	node := artifact.Node{Path: artifact.Path(path), Name: name, Kind: artifact.KindDirectory}
	if levels > 1 {
		node.Children = append(node.Children, deepChain(path+"/a", "a", levels-1))
	}
	for f := 0; f < 19; f++ {
		file := fmt.Sprintf("f%02d", f)
		node.Children = append(node.Children, artifact.Node{Path: artifact.Path(path + "/" + file), Name: file, Kind: artifact.KindFile})
	}
	return node
}

func writeGitIgnoreTree(dir string, gitignore []byte) (map[string]any, error) {
	info, err := writeWideTree(dir, nodes100k-1)
	if err != nil {
		return nil, err
	}
	info["gitignoreBytes"] = len(gitignore)
	return info, os.WriteFile(filepath.Join(dir, ".gitignore"), gitignore, 0o600)
}

// writeArtifactTree creates the directories and empty files of art under
// dir, one directory per task on every core.
func writeArtifactTree(dir string, art artifact.Artifact) (map[string]any, error) {
	var dirs []artifact.Node
	var collect func(n artifact.Node)
	collect = func(n artifact.Node) {
		dirs = append(dirs, n)
		for _, c := range n.Children {
			if c.Kind == artifact.KindDirectory {
				collect(c)
			}
		}
	}
	collect(art.Root)
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(string(d.Path))), 0o750); err != nil {
			return nil, err
		}
	}
	work := make(chan artifact.Node)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range work {
				for _, c := range d.Children {
					if c.Kind != artifact.KindFile {
						continue
					}
					if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(string(c.Path))), nil, 0o600); err != nil {
						mu.Lock()
						firstErr = errors.Join(firstErr, err)
						mu.Unlock()
					}
				}
			}
		}()
	}
	for _, d := range dirs {
		work <- d
	}
	close(work)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return map[string]any{"nodes": countNodes(art.Root), "directories": len(dirs)}, nil
}

// writeLiveSnapshots writes, with the dirloom snapshot command itself, the
// snapshot of each live tree that exists, for verify.
func writeLiveSnapshots(root string) (map[string]any, error) {
	info := map[string]any{}
	if err := os.MkdirAll(filepath.Join(root, "snaplive"), 0o750); err != nil {
		return nil, err
	}
	for _, tree := range []string{"live-wide-100k", "live-wide-1m", "live-wide-2m", "live-deep-100k"} {
		dir := filepath.Join(root, tree)
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		out := filepath.Join(root, "snaplive", strings.TrimPrefix(tree, "live-")+".json")
		var stderr bytes.Buffer
		code := cli.Execute(context.Background(), []string{"snapshot", dir, "--output", out}, io.Discard, &stderr, buildinfo.ResolvedVersion())
		if code != 0 {
			return nil, fmt.Errorf("dirloom snapshot %s: exit %d: %s", dir, code, stderr.String())
		}
		data, err := os.ReadFile(out) // #nosec G304 -- written just above, under the fixture directory.
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		info[tree] = map[string]any{"bytes": len(data), "sha256": hex.EncodeToString(sum[:])}
		runtime.GC()
	}
	return info, nil
}

// git runs git in dir with a fixed identity and date and no user or system
// configuration, so the generated object ids are reproducible.
func git(dir string, stdin io.Reader, args ...string) (string, error) {
	cmd := exec.Command("git", args...) // #nosec G204 -- fixed git subcommands with generated arguments.
	cmd.Dir = dir
	cmd.Stdin = stdin
	date := fmt.Sprintf("%d +0000", gitDate)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_NAME=Dirloom Bench", "GIT_AUTHOR_EMAIL=bench@dirloom.invalid", "GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_NAME=Dirloom Bench", "GIT_COMMITTER_EMAIL=bench@dirloom.invalid", "GIT_COMMITTER_DATE="+date,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args[:min(len(args), 3)], " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func initRepository(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	_, err := git(dir, nil, "init", "--quiet", "--initial-branch=main", "--object-format=sha1")
	return err
}

func revParse(dir string, revisions ...string) (map[string]any, error) {
	info := map[string]any{}
	for _, rev := range revisions {
		id, err := git(dir, nil, "rev-parse", rev)
		if err != nil {
			return nil, err
		}
		info[rev] = id
	}
	return info, nil
}

// writeWideRepository commits the wide shape of nodes-1 nodes (Git holds no
// empty zz-moved directory), then relocates every 10th directory under
// zz-moved: nodes nodes.
func writeWideRepository(dir string, nodes int) (map[string]any, error) {
	if err := initRepository(dir); err != nil {
		return nil, err
	}
	art := wideArtifact(nodes, 0, "")
	var stream bytes.Buffer
	stream.WriteString("blob\nmark :1\ndata 0\n\n")
	fmt.Fprintf(&stream, "commit refs/heads/main\nmark :2\ncommitter Dirloom Bench <bench@dirloom.invalid> %d +0000\ndata 2\nA\n", gitDate)
	var relocated []string
	for i, d := range art.Root.Children {
		if d.Name == "zz-moved" {
			continue
		}
		for _, f := range d.Children {
			fmt.Fprintf(&stream, "M 100644 :1 %s\n", f.Path)
		}
		if i%10 == 0 {
			relocated = append(relocated, d.Name)
		}
	}
	fmt.Fprintf(&stream, "\ncommit refs/heads/main\nmark :3\ncommitter Dirloom Bench <bench@dirloom.invalid> %d +0000\ndata 2\nB\nfrom :2\n", gitDate+60)
	for _, name := range relocated {
		fmt.Fprintf(&stream, "R %s zz-moved/%s\n", name, name)
	}
	stream.WriteString("\n")
	if _, err := git(dir, &stream, "fast-import", "--quiet"); err != nil {
		return nil, err
	}
	info, err := revParse(dir, "HEAD~1", "HEAD")
	if err != nil {
		return nil, err
	}
	info["nodesHEAD~1"], info["nodesHEAD"], info["relocatedDirectories"] = nodes-1, nodes, len(relocated)
	return info, nil
}

// writeBombRepository commits the Git bomb of #91: a tree of 1000 files,
// then two levels of 48 entries that all point at the level below.
func writeBombRepository(dir string) (map[string]any, error) {
	if err := initRepository(dir); err != nil {
		return nil, err
	}
	blob, err := git(dir, strings.NewReader("x\n"), "hash-object", "-w", "--stdin")
	if err != nil {
		return nil, err
	}
	var entries strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&entries, "100644 blob %s\tf%d\n", blob, i)
	}
	id, err := git(dir, strings.NewReader(entries.String()), "mktree")
	if err != nil {
		return nil, err
	}
	for range 2 {
		entries.Reset()
		for i := 0; i < 48; i++ {
			fmt.Fprintf(&entries, "040000 tree %s\td%d\n", id, i)
		}
		if id, err = git(dir, strings.NewReader(entries.String()), "mktree"); err != nil {
			return nil, err
		}
	}
	commit, err := git(dir, nil, "commit-tree", id, "-m", "bomb")
	if err != nil {
		return nil, err
	}
	if _, err := git(dir, nil, "update-ref", "refs/heads/main", commit); err != nil {
		return nil, err
	}
	return map[string]any{"HEAD": commit, "nodes": 1 + 48 + 48*48 + 48*48*1000}, nil
}

// Chain depths: directory k of a chain of "d" directories has a path of
// 2k-1 bytes, so a chain of depth D holds D² bytes of directory paths, plus
// its leaf file. 16,382 is the deepest chain under the 256 MiB limit with
// its leaf; 16,385 is above it by its directories alone.
const (
	chainUnder = 16_382
	chainOver  = 16_385
)

// writeChainRepository commits main~1, a chain of chainUnder directories
// with a leaf file "leaf"; main, the same chain with the leaf renamed
// "leaf2"; and the branch over, a chain of chainOver directories. The trees
// are written as loose objects: building them with one git process per
// tree would take minutes.
func writeChainRepository(dir string) (map[string]any, error) {
	if err := initRepository(dir); err != nil {
		return nil, err
	}
	objects := filepath.Join(dir, ".git", "objects")
	blob, err := writeLooseObject(objects, "blob", []byte("x\n"))
	if err != nil {
		return nil, err
	}
	info := map[string]any{}
	var parent string
	for _, c := range []struct {
		ref, leaf, message string
		depth              int
	}{
		{"refs/heads/main", "leaf", "under", chainUnder},
		{"refs/heads/main", "leaf2", "under, leaf renamed", chainUnder},
		{"refs/heads/over", "leaf", "over", chainOver},
	} {
		id, err := writeLooseObject(objects, "tree", treeEntry("100644", c.leaf, blob))
		if err != nil {
			return nil, err
		}
		for range c.depth {
			if id, err = writeLooseObject(objects, "tree", treeEntry("40000", "d", id)); err != nil {
				return nil, err
			}
		}
		args := []string{"commit-tree", hex.EncodeToString(id), "-m", c.message}
		if c.ref == "refs/heads/main" && parent != "" {
			args = append(args, "-p", parent)
		}
		commit, err := git(dir, nil, args...)
		if err != nil {
			return nil, err
		}
		if _, err := git(dir, nil, "update-ref", c.ref, commit); err != nil {
			return nil, err
		}
		if c.ref == "refs/heads/main" {
			parent = commit
		}
		pathBytes := c.depth*c.depth + 2*c.depth + len(c.leaf)
		info[c.message] = map[string]any{"commit": commit, "depth": c.depth, "pathBytes": pathBytes, "nodes": c.depth + 2}
	}
	info["pathLimit"] = snapshot.MaxBytes
	return info, nil
}

func treeEntry(mode, name string, id []byte) []byte {
	entry := append([]byte(mode+" "+name+"\x00"), id...)
	return entry
}

// writeLooseObject stores a Git object as a loose object and returns its
// binary id.
func writeLooseObject(objects, kind string, body []byte) ([]byte, error) {
	header := fmt.Sprintf("%s %d\x00", kind, len(body))
	hash := sha1.New() // #nosec G401 -- Git object ids are SHA-1 in a sha1 repository.
	hash.Write([]byte(header))
	hash.Write(body)
	id := hash.Sum(nil)
	name := hex.EncodeToString(id)
	path := filepath.Join(objects, name[:2], name[2:])
	if _, err := os.Stat(path); err == nil {
		return id, nil
	}
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	if _, err := w.Write([]byte(header)); err != nil {
		return nil, err
	}
	if _, err := w.Write(body); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	return id, os.WriteFile(path, compressed.Bytes(), 0o600)
}

// writeHistoryRepository extends the history benchmark repository of
// v0.4.0-a7: a first commit of README.md and src/m<i>/f<j>.go (modules of
// nine files, about entries files), then commits that each add one file,
// delete one file in another module and, every fifth commit, rewrite
// README.md. Every 50th commit also relocates one module under moved/, so
// the history has moves to detect; a relocated module gets no deletion
// afterwards.
func writeHistoryRepository(dir string, entries, commits int) (map[string]any, error) {
	if err := initRepository(dir); err != nil {
		return nil, err
	}
	modules := entries / 10
	moved := map[int]bool{}
	var stream bytes.Buffer
	stream.WriteString("blob\nmark :1\ndata 13\npackage bench\n")
	stream.WriteString("blob\nmark :2\ndata 7\nreadme1\n")
	stream.WriteString("blob\nmark :3\ndata 7\nreadme2\n")
	for c := 0; c < commits; c++ {
		fmt.Fprintf(&stream, "commit refs/heads/main\ncommitter Dirloom Bench <bench@dirloom.invalid> %d +0000\ndata 5\nc%04d\n", gitDate+60*c, c)
		if c == 0 {
			stream.WriteString("M 100644 :2 README.md\n")
			for module := 0; module < modules; module++ {
				for file := 0; file < 9; file++ {
					fmt.Fprintf(&stream, "M 100644 :1 src/m%05d/f%d.go\n", module, file)
				}
			}
			continue
		}
		fmt.Fprintf(&stream, "M 100644 :1 src/m%05d/added%04d.go\n", c%modules, c)
		if module := (c + modules/2) % modules; !moved[module] {
			fmt.Fprintf(&stream, "D src/m%05d/f%d.go\n", module, (c/modules)%9)
		}
		if c%5 == 0 {
			fmt.Fprintf(&stream, "M 100644 :%d README.md\n", 2+(c/5)%2)
		}
		if c%50 == 0 {
			// 37 is prime to the module counts used: distinct modules.
			module := (c / 50 * 37) % modules
			moved[module] = true
			fmt.Fprintf(&stream, "R src/m%05d moved/m%05d-%04d\n", module, module, c)
		}
	}
	stream.WriteString("\n")
	if _, err := git(dir, &stream, "fast-import", "--quiet"); err != nil {
		return nil, err
	}
	info, err := revParse(dir, "HEAD")
	if err != nil {
		return nil, err
	}
	info["commits"], info["entries"] = commits, entries
	return info, nil
}
