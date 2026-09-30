package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
)

// inspectRequestFromCapture maps Capture Semantics v1 onto InspectRequest.
// Depth and Ignore are deep-copied. Current configuration is not consulted.
func inspectRequestFromCapture(capture snapshot.CaptureV1, root string) InspectRequest {
	cloned := capture.Clone()
	return InspectRequest{
		Root:              root,
		MaxDepth:          cloned.Depth,
		DirectoriesOnly:   cloned.DirsOnly,
		IncludeHidden:     cloned.Hidden,
		IgnorePatterns:    cloned.Ignore,
		UseDefaultIgnores: cloned.UseDefaultIgnores,
		UseGitIgnore:      cloned.UseGitignore,
	}
}

// referenceSelfExclusion returns the snapshot path for InspectRequest.OutputPath
// only when that file is inside the live root and absent from the expected artifact.
func referenceSelfExclusion(root, snapshotPath string, expected artifact.Artifact) (string, error) {
	if snapshotPath == "" {
		return "", nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", artifact.SourceReadFailure(fmt.Errorf("resolve directory %q: %w", root, err))
	}
	absRoot = filepath.Clean(absRoot)
	absSnap, err := filepath.Abs(snapshotPath)
	if err != nil {
		return "", artifact.SourceReadFailure(fmt.Errorf("resolve snapshot %q: %w", snapshotPath, err))
	}
	absSnap = filepath.Clean(absSnap)
	rel, err := filepath.Rel(absRoot, absSnap)
	if err != nil || !filepath.IsLocal(rel) {
		return "", nil
	}
	separator := artifact.POSIXSeparator
	if os.PathSeparator == '\\' {
		separator = artifact.WindowsSeparator
	}
	canonical, err := artifact.Canonicalize(rel, separator)
	if err != nil {
		return "", artifact.SourceReadFailure(fmt.Errorf("canonicalize snapshot path %q: %w", rel, err))
	}
	if canonical == artifact.RootPath || artifactContains(expected, canonical) {
		return "", nil
	}
	return absSnap, nil
}

func artifactContains(art artifact.Artifact, path artifact.Path) bool {
	return nodeContains(art.Root, path)
}

func nodeContains(node artifact.Node, path artifact.Path) bool {
	if node.Path == path {
		return true
	}
	for _, child := range node.Children {
		if nodeContains(child, path) {
			return true
		}
	}
	return false
}
