package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/tree"
)

// Filesystem builds an artifact from one existing-scanner traversal.
type Filesystem struct {
	Scan    func(context.Context) (*tree.Node, error)
	RootAbs string
}

func (f Filesystem) Kind() Kind { return KindFilesystem }

func (f Filesystem) Observe(ctx context.Context) (*artifact.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.Scan == nil {
		return nil, artifact.InternalError("filesystem source is missing a scanner")
	}
	model, err := f.Scan(ctx)
	if err != nil {
		return nil, artifact.SourceReadFailure(err)
	}
	return FromTree(model, FromTreeOptions{RootAbs: f.RootAbs})
}

// FromTreeOptions control observation-to-artifact adaptation.
type FromTreeOptions struct {
	RootAbs string
}

// FromTree converts the existing observation model into a Canonical Artifact.
// It does not walk the filesystem.
func FromTree(root *tree.Node, options FromTreeOptions) (*artifact.Artifact, error) {
	if root == nil {
		return nil, artifact.InternalError("observation root is nil")
	}
	if root.Type != tree.NodeDirectory {
		return nil, unsupportedObservation(root.Type, ".")
	}
	art := artifact.Artifact{
		DisplayRootName: root.Name,
		Root: artifact.Node{
			Path: artifact.RootPath,
			Name: ".",
			Kind: artifact.KindDirectory,
		},
	}
	collisions := artifact.NewCollisionSet()
	if err := collisions.Add(".", artifact.RootPath); err != nil {
		return nil, err
	}
	children, err := convertChildren(root.Children, artifact.RootPath, options, collisions)
	if err != nil {
		return nil, err
	}
	art.Root.Children = children
	if err := art.Validate(); err != nil {
		return nil, err
	}
	return &art, nil
}

func convertChildren(nodes []*tree.Node, parent artifact.Path, options FromTreeOptions, collisions *artifact.CollisionSet) ([]artifact.Node, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	out := make([]artifact.Node, 0, len(nodes))
	for _, node := range nodes {
		converted, err := convertNode(node, parent, options, collisions)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func convertNode(node *tree.Node, parent artifact.Path, options FromTreeOptions, collisions *artifact.CollisionSet) (artifact.Node, error) {
	if node == nil {
		return artifact.Node{}, artifact.InternalError("observation contains a nil node")
	}
	name, err := artifact.CanonicalizeName(node.Name)
	if err != nil {
		return artifact.Node{}, err
	}
	rawPath := node.Path
	if rawPath == "" {
		if parent == artifact.RootPath {
			rawPath = node.Name
		} else {
			rawPath = string(parent) + "/" + node.Name
		}
	}
	path, err := artifact.Canonicalize(rawPath, artifact.POSIXSeparator)
	if err != nil {
		return artifact.Node{}, err
	}
	if err := collisions.Add(rawPath, path); err != nil {
		return artifact.Node{}, err
	}
	kind, err := observationKind(node.Type, path)
	if err != nil {
		return artifact.Node{}, err
	}
	converted := artifact.Node{Path: path, Name: name, Kind: kind}
	if kind.HasTarget() {
		target, targetErr := canonicalizeTarget(node.Target, options.RootAbs)
		if targetErr != nil {
			return artifact.Node{}, targetErr
		}
		converted.Target = target
	}
	if kind == artifact.KindDirectory {
		children, childErr := convertChildren(node.Children, path, options, collisions)
		if childErr != nil {
			return artifact.Node{}, childErr
		}
		converted.Children = children
	} else if len(node.Children) != 0 {
		return artifact.Node{}, artifact.InternalError("observation node %q of type %q has children", path, node.Type)
	}
	return converted, nil
}

func observationKind(kind tree.NodeType, path artifact.Path) (artifact.Kind, error) {
	switch kind {
	case tree.NodeDirectory:
		return artifact.KindDirectory, nil
	case tree.NodeFile:
		return artifact.KindFile, nil
	case tree.NodeSymlink:
		return artifact.KindSymlink, nil
	default:
		return "", unsupportedObservation(kind, path.String())
	}
}

func unsupportedObservation(kind tree.NodeType, path string) error {
	return &artifact.Error{
		Code:    artifact.CodeUnsupportedKind,
		Message: fmt.Sprintf("unsupported node type %q at %q", kind, path),
		Paths:   []string{path},
	}
}

func canonicalizeTarget(raw, absRoot string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if !utf8.ValidString(raw) {
		return "", &artifact.Error{Code: artifact.CodeInvalidPath, Message: "symlink target is not valid UTF-8"}
	}
	nfc := norm.NFC.String(raw)
	converted := nfc
	if os.PathSeparator == '\\' {
		converted = strings.ReplaceAll(converted, `\`, "/")
	}
	if absRoot != "" && isAbsoluteTarget(nfc) {
		native := filepath.FromSlash(converted)
		rel, err := filepath.Rel(filepath.Clean(absRoot), filepath.Clean(native))
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel), nil
		}
	}
	return converted, nil
}

func isAbsoluteTarget(raw string) bool {
	if raw == "" {
		return false
	}
	return filepath.IsAbs(raw) || filepath.IsAbs(filepath.FromSlash(raw))
}
