// Package source abstracts Canonical Structural Artifact production.
package source

import (
	"context"

	"github.com/dirloom/dirloom/internal/artifact"
)

// Kind identifies a structural source without entering Identity Projection v1.
type Kind string

const (
	KindFilesystem Kind = "filesystem"
	KindMemory     Kind = "memory"
	// KindSnapshot identifies an already-validated snapshot artifact reused as
	// a structural source.
	KindSnapshot Kind = "snapshot"
)

// Source observes a structure and returns a canonical artifact.
type Source interface {
	Kind() Kind
	Observe(ctx context.Context) (*artifact.Artifact, error)
}

// Memory is a test source that returns a cloned artifact.
type Memory struct {
	Artifact artifact.Artifact
}

func (m Memory) Kind() Kind { return KindMemory }

func (m Memory) Observe(ctx context.Context) (*artifact.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	clone := m.Artifact.Clone()
	return &clone, nil
}

// Snapshot exposes an already-validated snapshot artifact as a structural
// source. It never parses Snapshot Schema: decoding and self-verifying
// validation happen in the snapshot package before this adapter is built.
type Snapshot struct {
	Artifact artifact.Artifact
}

func (s Snapshot) Kind() Kind { return KindSnapshot }

func (s Snapshot) Observe(ctx context.Context) (*artifact.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	clone := s.Artifact.Clone()
	return &clone, nil
}
