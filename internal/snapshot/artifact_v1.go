package snapshot

import (
	"sort"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
	"golang.org/x/text/unicode/norm"
)

// NodeV1 is one persisted Snapshot Artifact Projection v1 node.
type NodeV1 struct {
	Path   string  `json:"path"`
	Kind   string  `json:"kind"`
	Target *string `json:"target,omitempty"`
}

// ArtifactV1 is the flat persisted projection of a Canonical Structural Artifact.
type ArtifactV1 struct {
	Nodes []NodeV1 `json:"nodes"`
}

// ProjectV1 flattens art into Snapshot Artifact Projection v1 without mutating
// the caller's children slices. Nodes are sorted by ascending path bytes.
func ProjectV1(art artifact.Artifact) (ArtifactV1, error) {
	if err := art.Validate(); err != nil {
		return ArtifactV1{}, invalidArtifact(err)
	}
	nodes := make([]NodeV1, 0, art.CountNodes())
	var walk func(node artifact.Node)
	walk = func(node artifact.Node) {
		record := NodeV1{Path: node.Path.String(), Kind: string(node.Kind)}
		if node.Kind.HasTarget() {
			target := node.Target
			record.Target = &target
		}
		nodes = append(nodes, record)
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(art.Root)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
	return ArtifactV1{Nodes: nodes}, nil
}

// Reconstruct builds a Canonical Structural Artifact from a flat projection.
// Input node order is ignored; the tree is normalized by path.
func Reconstruct(proj ArtifactV1) (artifact.Artifact, error) {
	if len(proj.Nodes) == 0 {
		return artifact.Artifact{}, invalidArtifact(invalidField("snapshot artifact.nodes must not be empty"))
	}
	byPath := make(map[string]NodeV1, len(proj.Nodes))
	for _, node := range proj.Nodes {
		if err := assertPersistedCanonicalPath(node.Path); err != nil {
			return artifact.Artifact{}, err
		}
		if _, exists := byPath[node.Path]; exists {
			return artifact.Artifact{}, invalidArtifact(invalidField("duplicate snapshot node path %q", node.Path))
		}
		kind := artifact.Kind(node.Kind)
		if !kind.Valid() {
			return artifact.Artifact{}, invalidArtifact(invalidField("invalid snapshot node kind %q at %q", node.Kind, node.Path))
		}
		if kind.HasTarget() {
			if node.Target == nil {
				return artifact.Artifact{}, invalidArtifact(invalidField("snapshot node %q of kind %q requires target", node.Path, node.Kind))
			}
			if !norm.NFC.IsNormalString(*node.Target) {
				return artifact.Artifact{}, invalidArtifact(invalidField("snapshot node %q target is not NFC", node.Path))
			}
		} else if node.Target != nil {
			return artifact.Artifact{}, invalidArtifact(invalidField("snapshot node %q of kind %q must not have target", node.Path, node.Kind))
		}
		byPath[node.Path] = node
	}
	rootRecord, ok := byPath[string(artifact.RootPath)]
	if !ok {
		return artifact.Artifact{}, invalidArtifact(invalidField("snapshot artifact missing root path %q", artifact.RootPath))
	}
	if artifact.Kind(rootRecord.Kind) != artifact.KindDirectory {
		return artifact.Artifact{}, invalidArtifact(invalidField("snapshot root kind must be directory"))
	}
	if rootRecord.Target != nil {
		return artifact.Artifact{}, invalidArtifact(invalidField("snapshot root must not have a target"))
	}

	childrenOf := make(map[string][]string)
	for path := range byPath {
		if path == string(artifact.RootPath) {
			continue
		}
		parent, ok := parentPath(path)
		if !ok {
			return artifact.Artifact{}, invalidArtifact(invalidField("snapshot node %q has no parent", path))
		}
		if _, exists := byPath[parent]; !exists {
			return artifact.Artifact{}, invalidArtifact(invalidField("snapshot node %q missing parent %q", path, parent))
		}
		parentKind := artifact.Kind(byPath[parent].Kind)
		if parentKind != artifact.KindDirectory {
			return artifact.Artifact{}, invalidArtifact(invalidField("snapshot parent %q of %q is not a directory", parent, path))
		}
		childrenOf[parent] = append(childrenOf[parent], path)
	}
	for parent, kids := range childrenOf {
		sort.Strings(kids)
		childrenOf[parent] = kids
	}

	var build func(path string) (artifact.Node, error)
	build = func(path string) (artifact.Node, error) {
		record := byPath[path]
		node := artifact.Node{
			Path: artifact.Path(path),
			Name: baseName(path),
			Kind: artifact.Kind(record.Kind),
		}
		if record.Target != nil {
			node.Target = *record.Target
		}
		kids := childrenOf[path]
		if len(kids) > 0 {
			node.Children = make([]artifact.Node, 0, len(kids))
			for _, childPath := range kids {
				child, err := build(childPath)
				if err != nil {
					return artifact.Node{}, err
				}
				node.Children = append(node.Children, child)
			}
		}
		return node, nil
	}
	root, err := build(string(artifact.RootPath))
	if err != nil {
		return artifact.Artifact{}, err
	}
	art := artifact.Artifact{Root: root}
	if err := art.Validate(); err != nil {
		return artifact.Artifact{}, invalidArtifact(err)
	}
	return art, nil
}

func assertPersistedCanonicalPath(path string) error {
	if path == "" {
		return invalidArtifact(invalidField("snapshot node path must not be empty"))
	}
	if !norm.NFC.IsNormalString(path) {
		return invalidArtifact(invalidField("snapshot node path %q is not NFC", path))
	}
	// '\' is a legal POSIX filename character. Canonicalize with the POSIX
	// separator rejects non-canonical forms without treating '\' as a separator.
	canonical, err := artifact.Canonicalize(path, artifact.POSIXSeparator)
	if err != nil {
		return invalidArtifact(err)
	}
	if string(canonical) != path {
		return invalidArtifact(invalidField("snapshot node path %q is not already canonical", path))
	}
	if path != string(artifact.RootPath) {
		for _, segment := range strings.Split(path, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return invalidArtifact(invalidField("snapshot node path %q is not already canonical", path))
			}
		}
	}
	return nil
}

func parentPath(path string) (string, bool) {
	if path == string(artifact.RootPath) || path == "" {
		return "", false
	}
	index := strings.LastIndex(path, "/")
	if index < 0 {
		return string(artifact.RootPath), true
	}
	return path[:index], true
}

func baseName(path string) string {
	if path == string(artifact.RootPath) || path == "" {
		return "."
	}
	index := strings.LastIndex(path, "/")
	if index < 0 {
		return path
	}
	return path[index+1:]
}
