// Package config loads and resolves Dirloom's layered configuration.
package config

import (
	"errors"
	"fmt"
)

const (
	// SchemaVersion is the only supported .dirloom.yaml schema version.
	SchemaVersion = 1

	FormatText     = "text"
	FormatMarkdown = "markdown"
	FormatJSON     = "json"
	StyleUnicode   = "unicode"
	StyleASCII     = "ascii"
)

type SourceKind string

const (
	SourceBuiltIn SourceKind = "built-in"
	SourceUser    SourceKind = "user"
	SourceProject SourceKind = "project"
	SourceCLI     SourceKind = "cli"
)

type SourceStatus string

const (
	StatusLoaded      SourceStatus = "loaded"
	StatusMissing     SourceStatus = "missing"
	StatusDisabled    SourceStatus = "disabled"
	StatusUnavailable SourceStatus = "unavailable"
)

// Source describes one optional file-backed configuration layer.
type Source struct {
	Kind   SourceKind   `json:"kind"`
	Path   string       `json:"path,omitempty"`
	Status SourceStatus `json:"status"`
}

// Origin identifies the layer that supplied an effective value.
type Origin struct {
	Source SourceKind `json:"source"`
	Path   string     `json:"path,omitempty"`
}

// IgnoreRule is an effective exclusion together with its source.
type IgnoreRule struct {
	Pattern string `json:"pattern"`
	Origin  Origin `json:"origin"`
}

// Effective contains the fully resolved inspection and rendering settings.
type Effective struct {
	MaxDepth          *int
	DirectoriesOnly   bool
	IncludeHidden     bool
	Format            string
	Style             string
	UseDefaultIgnores bool
	UseGitIgnore      bool
	IgnorePatterns    []string
}

// Resolution contains effective values and enough metadata to explain them.
type Resolution struct {
	Root       string
	Sources    []Source
	Effective  Effective
	Provenance map[string]Origin
	Ignores    []IgnoreRule
}

// Optional represents a scalar CLI override, including explicit false values.
type Optional[T any] struct {
	Set   bool
	Value T
}

// DepthOverride distinguishes an omitted depth from an explicit unlimited one.
type DepthOverride struct {
	Set       bool
	Unlimited bool
	Value     int
}

// Overrides contains only options explicitly supplied on the command line.
type Overrides struct {
	Depth             DepthOverride
	DirectoriesOnly   Optional[bool]
	IncludeHidden     Optional[bool]
	Format            Optional[string]
	Style             Optional[string]
	UseDefaultIgnores Optional[bool]
	UseGitIgnore      Optional[bool]
	IgnorePatterns    []string
}

// ResolveOptions controls source selection and supplies explicit CLI values.
type ResolveOptions struct {
	Root                string
	ExplicitProjectPath string
	DisableUser         bool
	DisableAll          bool
	Overrides           Overrides
}

// InvalidError marks a configuration or option error as invalid user input.
type InvalidError struct {
	err error
}

func (e *InvalidError) Error() string { return e.err.Error() }
func (e *InvalidError) Unwrap() error { return e.err }

func invalidf(format string, args ...any) error {
	return &InvalidError{err: fmt.Errorf(format, args...)}
}

// IsInvalid reports whether an error represents invalid configuration input.
func IsInvalid(err error) bool {
	var invalid *InvalidError
	return errors.As(err, &invalid)
}
