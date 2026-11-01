package source

import (
	"context"
	"errors"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/gitsource"
	"github.com/dirloom/dirloom/internal/tree"
)

// Git observes one committed tree through the scanner and its Capture
// Semantics, reading Git objects only (Git Tree Source v1). Observe records
// the observed tree and commit, so a Git source is used through a pointer.
type Git struct {
	// Revision is the text after "git:". It must already satisfy
	// gitsource.ValidateRevision.
	Revision string
	// Dir is where Git discovers the repository; empty means the process
	// working directory.
	Dir string
	// Scan carries Capture Semantics and the root label. Observe sets its
	// Backend and RootLocation.
	Scan tree.ScanOptions

	tree   string
	commit string
}

func (g *Git) Kind() Kind { return KindGit }

// Tree returns the observed tree id after a successful Observe.
func (g *Git) Tree() string { return g.tree }

// Commit returns the commit the revision resolved through after a
// successful Observe, or "" when it named a tree.
func (g *Git) Commit() string { return g.commit }

func (g *Git) Observe(ctx context.Context) (*artifact.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g.Scan.FilterPolicy != nil && g.Scan.FilterPolicy.OutputPath != "" {
		return nil, artifact.InternalError("git source must not exclude a filesystem output path")
	}
	session, err := gitsource.OpenSession(ctx, g.Dir)
	if err != nil {
		return nil, gitObservationError(err)
	}
	resolved, err := session.Resolve(ctx, g.Revision)
	if err != nil {
		_ = session.Close()
		return nil, gitObservationError(err)
	}
	model, scanErr := scanGitTree(ctx, session, resolved.Tree, g.Scan)
	closeErr := session.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, gitObservationError(closeErr)
	}
	art, err := FromTree(model, FromTreeOptions{PortableTargets: true})
	if err != nil {
		return nil, err
	}
	g.tree, g.commit = resolved.Tree, resolved.Commit
	return art, nil
}

// ObserveGitTree builds the artifact of one tree through an open session,
// under the Capture Semantics of scan; it sets the scan's Backend and
// RootLocation. A history observes many trees through one session this way.
func ObserveGitTree(ctx context.Context, session *gitsource.Session, treeID string, scan tree.ScanOptions) (*artifact.Artifact, error) {
	model, err := scanGitTree(ctx, session, treeID, scan)
	if err != nil {
		return nil, err
	}
	return FromTree(model, FromTreeOptions{PortableTargets: true})
}

func scanGitTree(ctx context.Context, session *gitsource.Session, treeID string, scan tree.ScanOptions) (*tree.Node, error) {
	if scan.FilterPolicy != nil && scan.FilterPolicy.OutputPath != "" {
		return nil, artifact.InternalError("git source must not exclude a filesystem output path")
	}
	backend, err := session.Backend()
	if err != nil {
		return nil, gitObservationError(err)
	}
	scan.Backend = backend
	scan.RootLocation = treeID
	model, err := tree.NewScanner(scan).Scan(ctx)
	if err != nil {
		return nil, gitObservationError(err)
	}
	return model, nil
}

// gitObservationError keeps the code of a typed error, with the full wrapped
// message. Any other error is a source read failure, as for a filesystem
// source.
func gitObservationError(err error) error {
	var typed *artifact.Error
	if !errors.As(err, &typed) {
		return artifact.SourceReadFailure(err)
	}
	if typed == err { //nolint:errorlint // identity check: err is already the typed error.
		return err
	}
	return &artifact.Error{Code: typed.Code, Message: err.Error(), Paths: typed.Paths, Internal: typed.Internal}
}
