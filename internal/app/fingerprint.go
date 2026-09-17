package app

import (
	"context"
	"path/filepath"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
	"github.com/dirloom/dirloom/internal/tree"
)

// FingerprintResult is the application-level fingerprint outcome.
type FingerprintResult struct {
	Fingerprint identity.Fingerprint
	NodeCount   int
	SourceKind  source.Kind
}

// FingerprintFromSource observes src once and hashes Identity Projection v1.
func FingerprintFromSource(ctx context.Context, src source.Source) (FingerprintResult, error) {
	if err := ctx.Err(); err != nil {
		return FingerprintResult{}, err
	}
	if src == nil {
		return FingerprintResult{}, artifact.InternalError("fingerprint source is nil")
	}
	art, err := src.Observe(ctx)
	if err != nil {
		return FingerprintResult{}, err
	}
	fp, err := identity.Compute(*art)
	if err != nil {
		return FingerprintResult{}, err
	}
	return FingerprintResult{
		Fingerprint: fp,
		NodeCount:   art.CountNodes(),
		SourceKind:  src.Kind(),
	}, nil
}

// Fingerprint inspects the filesystem with the existing scanner exactly once.
func Fingerprint(ctx context.Context, request InspectRequest) (FingerprintResult, error) {
	return FingerprintFromSource(ctx, NewFilesystemSource(request))
}

// NewFilesystemSource reuses Inspect as the single structural traversal.
func NewFilesystemSource(request InspectRequest) source.Source {
	root := request.Root
	if root == "" {
		root = "."
	}
	return source.Filesystem{
		RootAbs: absRoot(root),
		Scan: func(ctx context.Context) (*tree.Node, error) {
			return Inspect(ctx, request)
		},
	}
}

func absRoot(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return filepath.Clean(abs)
}
