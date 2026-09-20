package artifact

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Validate checks Canonical Structural Artifact v1 invariants without touching
// a filesystem.
func (a Artifact) Validate() error {
	if a.Root.Path != RootPath {
		return invariant("root path must be %q, got %q", RootPath, a.Root.Path)
	}
	if a.Root.Name != "." {
		return invariant("root name must be %q, got %q", ".", a.Root.Name)
	}
	if a.Root.Kind != KindDirectory {
		return invariant("root kind must be directory, got %q", a.Root.Kind)
	}
	if a.Root.Target != "" {
		return invariant("root must not have a target")
	}
	seen := make(map[Path]struct{})
	return validateNode(a.Root, Path(""), true, seen)
}

func validateNode(node Node, parent Path, isRoot bool, seen map[Path]struct{}) error {
	if !node.Kind.Valid() {
		return unsupportedKind(string(node.Kind), node.Path.String())
	}
	if !utf8.ValidString(string(node.Path)) || !utf8.ValidString(node.Name) || !utf8.ValidString(node.Target) {
		return invariant("node %q is not valid UTF-8", node.Path)
	}
	if strings.Contains(string(node.Path), "//") {
		return invariant("canonical path %q contains an empty segment", node.Path)
	}
	if isAbsoluteNative(string(node.Path), POSIXSeparator) && node.Path != RootPath {
		return invariant("canonical path %q must not be absolute", node.Path)
	}
	if !node.Path.IsNFC() {
		return invariant("canonical path %q is not NFC", node.Path)
	}
	if !norm.NFC.IsNormalString(node.Name) {
		return invariant("node name %q is not NFC", node.Name)
	}
	if node.Kind.HasTarget() {
		if !norm.NFC.IsNormalString(node.Target) {
			return invariant("target of %q is not NFC", node.Path)
		}
	} else if node.Target != "" {
		return invariant("node %q of kind %q must not have a target", node.Path, node.Kind)
	}
	if node.Kind != KindDirectory && len(node.Children) != 0 {
		return invariant("node %q of kind %q must not have children", node.Path, node.Kind)
	}

	if isRoot {
		if node.Path != RootPath {
			return invariant("root path must be %q", RootPath)
		}
	} else {
		if node.Path == RootPath {
			return invariant("non-root node must not use the root path")
		}
		expectedParent, ok := node.Path.Parent()
		if !ok {
			return invariant("node %q has no parent", node.Path)
		}
		if expectedParent != parent {
			return invariant("node %q is not a child of %q", node.Path, parent)
		}
		joined, err := Join(parent, node.Name)
		if err != nil {
			return err
		}
		if joined != node.Path {
			return invariant("name %q is incompatible with path %q", node.Name, node.Path)
		}
		if node.Path.Base() != node.Name {
			return invariant("name %q is incompatible with path %q", node.Name, node.Path)
		}
	}

	if _, exists := seen[node.Path]; exists {
		return invariant("duplicate canonical path %q", node.Path)
	}
	seen[node.Path] = struct{}{}

	childNames := make(map[string]struct{}, len(node.Children))
	for _, child := range node.Children {
		if _, exists := childNames[child.Name]; exists {
			return collision(string(node.Path)+"/"+child.Name, string(node.Path)+"/"+child.Name, child.Path.String())
		}
		childNames[child.Name] = struct{}{}
		nfcName := norm.NFC.String(child.Name)
		if nfcName != child.Name {
			return invariant("child name %q of %q is not NFC", child.Name, node.Path)
		}
		if err := validateNode(child, node.Path, false, seen); err != nil {
			return err
		}
	}
	return nil
}
