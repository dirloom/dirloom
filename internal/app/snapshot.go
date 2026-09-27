package app

import (
	"context"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
)

// SnapshotResult is the application-level snapshot outcome.
type SnapshotResult struct {
	Document    snapshot.Document
	Artifact    artifact.Artifact
	Fingerprint identity.Fingerprint
	NodeCount   int
	SourceKind  source.Kind
	Bytes       []byte
}

// SnapshotFromSource observes src once, builds Snapshot v1, and encodes it.
func SnapshotFromSource(ctx context.Context, src source.Source, capture snapshot.CaptureV1) (SnapshotResult, error) {
	if err := ctx.Err(); err != nil {
		return SnapshotResult{}, err
	}
	if src == nil {
		return SnapshotResult{}, artifact.InternalError("snapshot source is nil")
	}
	art, err := src.Observe(ctx)
	if err != nil {
		return SnapshotResult{}, err
	}
	doc, err := snapshot.Build(*art, capture)
	if err != nil {
		return SnapshotResult{}, err
	}
	fp, err := identity.Parse(doc.Fingerprint)
	if err != nil {
		return SnapshotResult{}, err
	}
	encoded, err := snapshot.Marshal(doc)
	if err != nil {
		return SnapshotResult{}, err
	}
	return SnapshotResult{
		Document:    doc,
		Artifact:    *art,
		Fingerprint: fp,
		NodeCount:   art.CountNodes(),
		SourceKind:  src.Kind(),
		Bytes:       encoded,
	}, nil
}

// Snapshot inspects the filesystem once and produces Snapshot v1 bytes.
func Snapshot(ctx context.Context, request InspectRequest) (SnapshotResult, error) {
	capture := CaptureFromInspect(request)
	return SnapshotFromSource(ctx, NewFilesystemSource(request), capture)
}

// CaptureFromInspect maps effective InspectRequest structural settings into
// Capture Semantics v1. OutputPath is intentionally excluded.
func CaptureFromInspect(request InspectRequest) snapshot.CaptureV1 {
	ignore := append([]string(nil), request.IgnorePatterns...)
	if ignore == nil {
		ignore = []string{}
	}
	var depth *int
	if request.MaxDepth != nil {
		value := *request.MaxDepth
		depth = &value
	}
	return snapshot.CaptureV1{
		Depth:             depth,
		DirsOnly:          request.DirectoriesOnly,
		Hidden:            request.IncludeHidden,
		UseDefaultIgnores: request.UseDefaultIgnores,
		UseGitignore:      request.UseGitIgnore,
		Ignore:            ignore,
	}
}
