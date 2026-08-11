// Package app exposes Dirloom's reusable application services.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/tree"
)

// InspectRequest contains resolved CLI and default settings for an inspection.
type InspectRequest struct {
	Root              string
	MaxDepth          *int
	DirectoriesOnly   bool
	IncludeHidden     bool
	IgnorePatterns    []string
	UseDefaultIgnores bool
	UseGitIgnore      bool
	OutputPath        string
}

// Inspect resolves the selected root and builds a deterministic tree.
func Inspect(ctx context.Context, request InspectRequest) (*tree.Node, error) {
	if request.MaxDepth != nil && *request.MaxDepth < 0 {
		return nil, fmt.Errorf("depth must be a non-negative integer")
	}
	root := request.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve directory %q: %w", root, err)
	}
	absRoot = filepath.Clean(absRoot)

	info, err := os.Stat(absRoot)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil, fmt.Errorf("directory %q does not exist", root)
		case errors.Is(err, os.ErrPermission):
			return nil, fmt.Errorf("permission denied while opening directory %q", root)
		default:
			return nil, fmt.Errorf("open directory %q: %w", root, err)
		}
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %q is not a directory", root)
	}

	ignoreMatcher, err := filter.NewIgnoreMatcher(request.IgnorePatterns)
	if err != nil {
		return nil, err
	}

	outputPath := ""
	if request.OutputPath != "" {
		outputPath, err = filepath.Abs(request.OutputPath)
		if err != nil {
			return nil, fmt.Errorf("resolve output path %q: %w", request.OutputPath, err)
		}
		outputPath = filepath.Clean(outputPath)
	}

	var gitMatcher *filter.GitIgnore
	if request.UseGitIgnore {
		gitMatcher = filter.NewGitIgnore()
	}
	policy := filter.NewPolicy(outputPath, request.UseDefaultIgnores, ignoreMatcher, gitMatcher, request.IncludeHidden)
	scanner := tree.NewScanner(tree.ScanOptions{
		RootAbs:      absRoot,
		RootName:     tree.RootLabel(absRoot),
		MaxDepth:     request.MaxDepth,
		Directories:  request.DirectoriesOnly,
		UseGitIgnore: request.UseGitIgnore,
		FilterPolicy: policy,
		GitIgnore:    gitMatcher,
	})

	return scanner.Scan(ctx)
}
