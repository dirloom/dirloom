package tree

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dirloom/dirloom/internal/filter"
)

// ScanOptions controls a single traversal. RootAbs selects the filesystem
// root; a non-nil Backend replaces the filesystem and starts at RootLocation.
type ScanOptions struct {
	RootAbs      string
	RootName     string
	MaxDepth     *int
	Directories  bool
	UseGitIgnore bool
	FilterPolicy *filter.Policy
	GitIgnore    *filter.GitIgnore
	Backend      Backend
	RootLocation string
}

// Scanner builds a complete tree before returning. It never follows links
// encountered below the explicitly selected root.
type Scanner struct {
	options ScanOptions
	backend Backend
}

// NewScanner returns a scanner configured for one inspection.
func NewScanner(options ScanOptions) *Scanner {
	backend := options.Backend
	if backend == nil {
		backend = filesystemBackend{}
		options.RootLocation = options.RootAbs
	}
	return &Scanner{options: options, backend: backend}
}

// Scan traverses the root and returns no partial tree on error.
func (s *Scanner) Scan(ctx context.Context) (*Node, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	root := &Node{
		Name:     s.options.RootName,
		Type:     NodeDirectory,
		Children: make([]*Node, 0),
	}
	if s.options.MaxDepth != nil && *s.options.MaxDepth == 0 {
		return root, nil
	}

	if err := s.scanDirectory(ctx, root, s.options.RootLocation, "", 0); err != nil {
		return nil, err
	}
	Sort(root)
	return root, nil
}

func (s *Scanner) scanDirectory(ctx context.Context, parent *Node, location, relativeDirectory string, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if s.options.UseGitIgnore {
		if err := s.loadGitIgnore(location, relativeDirectory); err != nil {
			return err
		}
	}

	entries, err := listDirectory(ctx, s.backend, location)
	if err != nil {
		return fmt.Errorf("read directory %q: %w", displayPath(relativeDirectory), classifyFilesystemError(err))
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		name := entry.Name()
		childLocation := s.backend.Child(location, entry)
		relativePath := name
		if relativeDirectory != "" {
			relativePath = relativeDirectory + "/" + name
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("read metadata for %q: %w", relativePath, classifyFilesystemError(err))
		}
		if s.options.FilterPolicy != nil && s.options.FilterPolicy.Excludes(childLocation, relativePath, entry, info) {
			continue
		}

		// A junction is a terminal link whatever mode Go reports for it.
		junction, err := s.backend.Junction(childLocation, info)
		if err != nil {
			return fmt.Errorf("read metadata for %q: %w", relativePath, classifyFilesystemError(err))
		}
		kind := classifyNode(info)
		if junction {
			kind = NodeSymlink
		}
		if s.options.Directories && kind != NodeDirectory {
			continue
		}

		node := &Node{Name: name, Path: relativePath, Type: kind, Junction: junction}
		if kind == NodeFile {
			node.Special = specialType(info.Mode())
		}
		switch kind {
		case NodeDirectory:
			node.Children = make([]*Node, 0)
			parent.Children = append(parent.Children, node)
			childDepth := depth + 1
			if s.options.MaxDepth == nil || childDepth < *s.options.MaxDepth {
				if err := s.scanDirectory(ctx, node, childLocation, relativePath, childDepth); err != nil {
					return err
				}
			}
		case NodeSymlink:
			target, err := s.backend.ReadLink(childLocation, relativePath)
			if err != nil {
				return err
			}
			node.Target = target
			parent.Children = append(parent.Children, node)
		case NodeFile:
			parent.Children = append(parent.Children, node)
		}
	}

	return nil
}

func (s *Scanner) loadGitIgnore(location, relativeDirectory string) error {
	data, ok, err := s.backend.ReadGitIgnore(location, relativeDirectory)
	if err != nil || !ok {
		return err
	}

	source := displayGitIgnorePath(relativeDirectory)
	if err := s.options.GitIgnore.AddPatterns(data, relativeDirectory, source); err != nil {
		return err
	}
	return nil
}

func displayGitIgnorePath(relativeDirectory string) string {
	if relativeDirectory == "" {
		return ".gitignore"
	}
	return relativeDirectory + "/.gitignore"
}

func classifyNode(info os.FileInfo) NodeType {
	if info.Mode()&os.ModeSymlink != 0 {
		return NodeSymlink
	}
	if info.IsDir() {
		return NodeDirectory
	}
	return NodeFile
}

// specialType names an entry that is a named pipe, a socket or a device, and
// returns "" otherwise. ModeIrregular is never special: Windows gives it to
// files behind reparse points (app execution aliases, cloud placeholders,
// compressed files), and Linux and macOS never set it.
func specialType(mode os.FileMode) string {
	switch {
	case mode&os.ModeNamedPipe != 0:
		return "named pipe"
	case mode&os.ModeSocket != 0:
		return "socket"
	case mode&os.ModeCharDevice != 0:
		return "character device"
	case mode&os.ModeDevice != 0:
		return "device"
	default:
		return ""
	}
}

func displayPath(relativePath string) string {
	if relativePath == "" {
		return "."
	}
	return relativePath
}

// contextLister is a backend whose listing honors cancellation while it
// waits, as the filesystem backend does when it retries on Windows.
type contextLister interface {
	ReadDirContext(ctx context.Context, location string) ([]fs.DirEntry, error)
}

func listDirectory(ctx context.Context, backend Backend, location string) ([]fs.DirEntry, error) {
	if lister, ok := backend.(contextLister); ok {
		return lister.ReadDirContext(ctx, location)
	}
	return backend.ReadDir(location)
}

func classifyFilesystemError(err error) error {
	switch {
	case errors.Is(err, os.ErrPermission):
		return errors.New("permission denied")
	case errors.Is(err, os.ErrNotExist):
		return errors.New("entry no longer exists")
	default:
		return err
	}
}

// RootLabel derives the stable label used by all renderers from an absolute,
// cleaned root path.
func RootLabel(absolutePath string) string {
	cleaned := filepath.Clean(absolutePath)
	volume := filepath.VolumeName(cleaned)
	remainder := strings.Trim(cleaned[len(volume):], string(filepath.Separator))
	if remainder == "" {
		return cleaned
	}
	return filepath.Base(cleaned)
}
