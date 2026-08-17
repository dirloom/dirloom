// Package config loads and resolves Dirloom's layered configuration.
package config

import (
	"errors"
	"fmt"

	"github.com/dirloom/dirloom/internal/diagram"
	"github.com/dirloom/dirloom/internal/outputformat"
	"github.com/dirloom/dirloom/internal/presentation"
)

const (
	// SchemaVersion is the only supported .dirloom.yaml schema version.
	SchemaVersion = 1

	FormatText         = outputformat.Text
	FormatMarkdown     = outputformat.Markdown
	FormatMarkdownTree = outputformat.MarkdownTree
	FormatJSON         = outputformat.JSON
	FormatMermaid      = outputformat.Mermaid
	FormatGraphviz     = outputformat.Graphviz
	FormatD2           = outputformat.D2
	StyleUnicode       = "unicode"
	StyleASCII         = "ascii"

	DiagramViewStructure      = string(diagram.ViewStructure)
	DiagramDirectionTopDown   = string(diagram.DirectionTopDown)
	DiagramDirectionLeftRight = string(diagram.DirectionLeftRight)
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
	Preset string     `json:"preset,omitempty"`
}

// ResolvedPreset identifies the effective preset selection and its source.
// Name is nil when no preset is active, including after an explicit reset.
type ResolvedPreset struct {
	Name   *string `json:"name"`
	Origin Origin  `json:"origin"`
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
	Color             string
	Icons             string
	Theme             string
	DiagramView       string
	DiagramDirection  string
	DiagramMaxNodes   *int
}

// Resolution contains effective values and enough metadata to explain them.
type Resolution struct {
	Root       string
	Sources    []Source
	Preset     ResolvedPreset
	Effective  Effective
	Provenance map[string]Origin
	Ignores    []IgnoreRule
	ThemeInfo  *presentation.Theme
}

// SetThemeInfo attaches an already validated winning theme for diagnostics.
// The configuration loader deliberately does not read theme files itself.
func (resolution *Resolution) SetThemeInfo(theme presentation.Theme) {
	copyTheme := theme
	resolution.ThemeInfo = &copyTheme
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

// LimitOverride distinguishes an omitted limit from explicit unlimited.
type LimitOverride struct {
	Set       bool
	Unlimited bool
	Value     int
}

// PresetSelection distinguishes inheritance, a named preset, and an explicit
// reset. Disabled is meaningful only when Set is true.
type PresetSelection struct {
	Set      bool
	Disabled bool
	Name     string
}

// ThemeSelection distinguishes inheritance, a named/path theme, and null,
// which explicitly resets an inherited selection to the built-in default.
type ThemeSelection struct {
	Set   bool
	Reset bool
	Value string
}

// Overrides contains only options explicitly supplied on the command line.
type Overrides struct {
	Preset            PresetSelection
	Depth             DepthOverride
	DirectoriesOnly   Optional[bool]
	IncludeHidden     Optional[bool]
	Format            Optional[string]
	Style             Optional[string]
	UseDefaultIgnores Optional[bool]
	UseGitIgnore      Optional[bool]
	IgnorePatterns    []string
	Color             Optional[string]
	Icons             Optional[string]
	Theme             ThemeSelection
	DiagramView       Optional[string]
	DiagramDirection  Optional[string]
	DiagramMaxNodes   LimitOverride
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
