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
