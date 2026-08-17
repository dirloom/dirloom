package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/dirloom/dirloom/internal/tree"
	"github.com/spf13/cobra"
)

func newThemeClassifyCommand(stdout io.Writer, sources *sourceOptions) *cobra.Command {
	var root, themeReference, as string
	command := &cobra.Command{
		Use: "classify <path>", Short: "Classify one real filesystem entry without scanning recursively",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{err: fmt.Errorf("expected exactly one filesystem path, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectSourceFlags(cmd, sources, "theme classification"); err != nil {
				return err
			}
			if as != "text" && as != "json" {
				return &usageError{err: fmt.Errorf("unsupported output format %q (expected text or json)", as)}
			}
			// Theme failures are intentionally resolved before touching the target.
			theme, err := presentation.LoadReference(themeReference, presentation.ReferenceContext{Kind: "cli"})
			if err != nil {
				return classifyPresentationError(err)
			}
			compiled, err := presentation.Compile(theme)
			if err != nil {
				return classifyPresentationError(err)
			}

			resolvedRoot, err := resolveClassifyRoot(root)
			if err != nil {
				return err
			}
			relativePath, info, err := resolveClassifyTarget(resolvedRoot, args[0])
			if err != nil {
				return err
			}
			nodeType, err := classifyNodeType(info)
			if err != nil {
				return err
			}
			document := presentation.NewClassifyDocument(relativePath, info.Name(), nodeType, theme, compiled)
			return writeThemeResult(stdout, as, document.WriteText, document.WriteJSON, "theme classification")
		},
	}
	command.Flags().StringVar(&root, "root", ".", "filesystem boundary for the inspected path")
	command.Flags().StringVar(&themeReference, "theme", presentation.ThemeDefault, "built-in theme or local YAML path")
	command.Flags().StringVar(&as, "as", "text", "output format: text or json")
	return command
}

func resolveClassifyRoot(raw string) (string, error) {
	if raw == "" {
		return "", &usageError{err: fmt.Errorf("--root requires a non-empty path")}
	}
	absolute, err := filepath.Abs(raw)
	if err != nil {
		return "", &usageError{err: fmt.Errorf("resolve classification root %q: %w", raw, err)}
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &usageError{err: fmt.Errorf("classification root %q does not exist", raw)}
		}
		return "", fmt.Errorf("resolve classification root %q: %w", raw, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &usageError{err: fmt.Errorf("classification root %q does not exist", raw)}
		}
		return "", fmt.Errorf("inspect classification root %q: %w", raw, err)
	}
	if !info.IsDir() {
		return "", &usageError{err: fmt.Errorf("classification root %q is not a directory", raw)}
	}
	return filepath.Clean(resolved), nil
}

func resolveClassifyTarget(root, raw string) (string, fs.FileInfo, error) {
	if raw == "" {
		return "", nil, &usageError{err: fmt.Errorf("classification path must not be empty")}
	}
	candidate := raw
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", nil, &usageError{err: fmt.Errorf("resolve classification path %q: %w", raw, err)}
	}
	absolute = filepath.Clean(absolute)

	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil, &usageError{err: fmt.Errorf("classification path %q does not exist", raw)}
		}
		return "", nil, fmt.Errorf("resolve classification path %q: %w", raw, err)
	}
	target := filepath.Join(resolvedParent, filepath.Base(absolute))
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", nil, &usageError{err: fmt.Errorf("classification path %q resolves outside root %q", raw, root)}
	}
	info, err := os.Lstat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil, &usageError{err: fmt.Errorf("classification path %q does not exist", raw)}
		}
		return "", nil, fmt.Errorf("inspect classification path %q: %w", raw, err)
	}
	return filepath.ToSlash(relative), info, nil
}
func classifyNodeType(info fs.FileInfo) (tree.NodeType, error) {
	mode := info.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		return tree.NodeSymlink, nil
	case mode.IsDir():
		return tree.NodeDirectory, nil
	case mode.IsRegular():
		return tree.NodeFile, nil
	default:
		return "", &usageError{err: fmt.Errorf("filesystem entry %q has unsupported type %s", info.Name(), mode.Type())}
	}
}
