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
	runner, err := gitsource.NewRunner(ctx, g.Dir)
	if err != nil {
		return nil, gitObservationError(err)
	}
	resolved, err := runner.Resolve(ctx, g.Revision)
	if err != nil {
		return nil, gitObservationError(err)
	}
	batch, err := runner.StartBatch(ctx, resolved.Format)
	if err != nil {
		return nil, gitObservationError(err)
	}
	options := g.Scan
	options.Backend = gitsource.NewBackend(batch)
	options.RootLocation = resolved.Tree
	model, scanErr := tree.NewScanner(options).Scan(ctx)
	closeErr := batch.Close()
	if scanErr != nil {
		return nil, gitObservationError(scanErr)
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
