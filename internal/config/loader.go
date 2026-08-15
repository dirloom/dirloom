package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dirloom/dirloom/internal/filter"
)

const maxConfigSize = 1 << 20

type fileSystem interface {
	ReadFile(string) ([]byte, error)
	Stat(string) (fs.FileInfo, error)
}

type osFileSystem struct{}

func (osFileSystem) ReadFile(path string) ([]byte, error) {
	// #nosec G304 -- reading the discovered or explicitly selected config path is this package's purpose.
	return os.ReadFile(path)
}
func (osFileSystem) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }

// Loader resolves filesystem-backed configuration with injectable host paths.
type Loader struct {
	fs            fileSystem
	userConfigDir func() (string, error)
	workingDir    func() (string, error)
}

type LoaderOption func(*Loader)

// WithUserConfigDir overrides user configuration discovery, primarily for tests.
func WithUserConfigDir(resolve func() (string, error)) LoaderOption {
	return func(loader *Loader) { loader.userConfigDir = resolve }
}

// WithWorkingDirectory overrides relative explicit configuration resolution.
func WithWorkingDirectory(resolve func() (string, error)) LoaderOption {
	return func(loader *Loader) { loader.workingDir = resolve }
}

// NewLoader creates a host-backed configuration loader.
func NewLoader(options ...LoaderOption) *Loader {
	loader := &Loader{
		fs:            osFileSystem{},
		userConfigDir: os.UserConfigDir,
		workingDir:    os.Getwd,
	}
	for _, option := range options {
		option(loader)
	}
	return loader
}

// Resolve loads enabled layers, applies CLI overrides, and records provenance.
func (loader *Loader) Resolve(options ResolveOptions) (Resolution, error) {
	if options.DisableAll && (options.DisableUser || options.ExplicitProjectPath != "") {
		return Resolution{}, invalidf("--no-config cannot be combined with --config or --no-user-config")
	}
	root, err := loader.resolveRoot(options.Root)
	if err != nil {
		return Resolution{}, err
	}

	resolution := defaultResolution(root)
	var userSource, projectSource Source
	var userValues, projectValues partial
	if options.DisableAll {
		userSource = Source{Kind: SourceUser, Status: StatusDisabled}
		projectSource = Source{Kind: SourceProject, Status: StatusDisabled}
	} else {
		userSource, userValues, err = loader.loadUser(options.DisableUser)
		if err != nil {
			return Resolution{}, err
		}
		projectSource, projectValues, err = loader.loadProject(root, options.ExplicitProjectPath)
		if err != nil {
			return Resolution{}, err
		}
	}

	resolution.Sources = []Source{userSource, projectSource}
	selected, err := resolvePresetSelection(userSource, userValues, projectSource, projectValues, options.Overrides)
	if err != nil {
		return Resolution{}, err
	}
	resolution.Preset = selected

	if userSource.Status == StatusLoaded {
		origin := Origin{Source: SourceUser, Path: userSource.Path}
		if err := applyLayer(&resolution, userValues, origin, selected); err != nil {
			return Resolution{}, err
		}
	}
	if projectSource.Status == StatusLoaded {
		origin := Origin{Source: SourceProject, Path: projectSource.Path}
		if err := applyLayer(&resolution, projectValues, origin, selected); err != nil {
			return Resolution{}, err
		}
	}
	if err := applyCLILayer(&resolution, options.Overrides, selected); err != nil {
		return Resolution{}, err
	}
	return resolution, nil
}

func (loader *Loader) resolveRoot(raw string) (string, error) {
	if raw == "" {
		raw = "."
	}
	absRoot, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("resolve directory %q: %w", raw, err)
	}
	absRoot = filepath.Clean(absRoot)
	info, err := loader.fs.Stat(absRoot)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return "", fmt.Errorf("directory %q does not exist", raw)
		case errors.Is(err, fs.ErrPermission):
			return "", fmt.Errorf("permission denied while opening directory %q", raw)
		default:
			return "", fmt.Errorf("open directory %q: %w", raw, err)
		}
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", raw)
	}
	return absRoot, nil
}

func (loader *Loader) loadUser(disabled bool) (Source, partial, error) {
	if disabled {
		return Source{Kind: SourceUser, Status: StatusDisabled}, partial{}, nil
	}
	directory, err := loader.userConfigDir()
	if err != nil || directory == "" {
		return Source{Kind: SourceUser, Status: StatusUnavailable}, partial{}, nil
	}
	path := filepath.Join(directory, "dirloom", "config.yaml")
	return loader.loadOptional(SourceUser, path, false)
}

func (loader *Loader) loadProject(root, explicit string) (Source, partial, error) {
	if explicit != "" {
		path := explicit
		if !filepath.IsAbs(path) {
			workingDirectory, err := loader.workingDir()
			if err != nil {
				return Source{}, partial{}, fmt.Errorf("resolve --config path %q: %w", explicit, err)
			}
			path = filepath.Join(workingDirectory, path)
		}
		path = filepath.Clean(path)
		return loader.loadOptional(SourceProject, path, true)
	}

	path, err := loader.discoverProject(root)
	if err != nil {
		return Source{}, partial{}, err
	}
	return loader.loadOptional(SourceProject, path, false)
}

func (loader *Loader) discoverProject(root string) (string, error) {
	boundary, found, err := loader.findGitBoundary(root)
	if err != nil {
		return "", err
	}
	if !found {
		return filepath.Join(root, ".dirloom.yaml"), nil
	}
	for directory := root; ; directory = filepath.Dir(directory) {
		candidate := filepath.Join(directory, ".dirloom.yaml")
		_, statErr := loader.fs.Stat(candidate)
		switch {
		case statErr == nil:
			return candidate, nil
		case errors.Is(statErr, fs.ErrNotExist):
		case statErr != nil:
			return "", fmt.Errorf("inspect project config %q: %w", candidate, statErr)
		}
		if samePath(directory, boundary) {
			break
		}
	}
	return filepath.Join(root, ".dirloom.yaml"), nil
}

func (loader *Loader) findGitBoundary(root string) (string, bool, error) {
	for directory := root; ; directory = filepath.Dir(directory) {
		marker := filepath.Join(directory, ".git")
		_, err := loader.fs.Stat(marker)
		switch {
		case err == nil:
			return directory, true, nil
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return "", false, fmt.Errorf("inspect Git boundary %q: %w", marker, err)
		}
		parent := filepath.Dir(directory)
		if samePath(parent, directory) {
			return "", false, nil
		}
	}
}

func (loader *Loader) loadOptional(kind SourceKind, path string, required bool) (Source, partial, error) {
	source := Source{Kind: kind, Path: path, Status: StatusMissing}
	info, err := loader.fs.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if required {
				return Source{}, partial{}, invalidf("config file %q does not exist", path)
			}
			return source, partial{}, nil
		}
		return Source{}, partial{}, fmt.Errorf("inspect config %q: %w", path, err)
	}
	if info.IsDir() {
		return Source{}, partial{}, fmt.Errorf("read config %q: path is a directory", path)
	}
	if info.Size() > maxConfigSize {
		return Source{}, partial{}, invalidf("invalid config %q: file exceeds the 1 MiB limit", path)
	}
	data, err := loader.fs.ReadFile(path)
	if err != nil {
		return Source{}, partial{}, fmt.Errorf("read config %q: %w", path, err)
	}
	if len(data) > maxConfigSize {
		return Source{}, partial{}, invalidf("invalid config %q: file exceeds the 1 MiB limit", path)
	}
	values, err := parseDocument(data, path)
	if err != nil {
		return Source{}, partial{}, err
	}
	source.Status = StatusLoaded
	return source, values, nil
}

func defaultResolution(root string) Resolution {
	builtIn := Origin{Source: SourceBuiltIn}
	return Resolution{
		Root:   root,
		Preset: ResolvedPreset{Origin: builtIn},
		Effective: Effective{
			Format:            FormatText,
			Style:             StyleUnicode,
			UseDefaultIgnores: true,
			UseGitIgnore:      true,
		},
		Provenance: map[string]Origin{
			"depth":             builtIn,
			"dirsOnly":          builtIn,
			"hidden":            builtIn,
			"format":            builtIn,
			"style":             builtIn,
			"useDefaultIgnores": builtIn,
			"useGitignore":      builtIn,
		},
	}
}

func resolvePresetSelection(userSource Source, userValues partial, projectSource Source, projectValues partial, overrides Overrides) (ResolvedPreset, error) {
	selection := PresetSelection{}
	origin := Origin{Source: SourceBuiltIn}
	if userSource.Status == StatusLoaded && userValues.Preset.Set {
		selection = userValues.Preset
		origin = Origin{Source: SourceUser, Path: userSource.Path}
	}
	if projectSource.Status == StatusLoaded && projectValues.Preset.Set {
		selection = projectValues.Preset
		origin = Origin{Source: SourceProject, Path: projectSource.Path}
	}
	if overrides.Preset.Set {
		selection = overrides.Preset
		origin = Origin{Source: SourceCLI}
	}
	if !selection.Set {
		return ResolvedPreset{Origin: origin}, nil
	}
	if selection.Disabled {
		if selection.Name != "" {
			return ResolvedPreset{}, invalidf("disabled preset selection must not include a name")
		}
		return ResolvedPreset{Origin: origin}, nil
	}
	if selection.Name == "" {
		return ResolvedPreset{}, invalidf("preset name must not be empty")
	}
	if _, ok := LookupPreset(selection.Name); !ok {
		allowed := append(PresetNames(), PresetNone)
		return ResolvedPreset{}, invalidf("unsupported preset %q (expected %s)", selection.Name, joinExpected(allowed))
	}
	name := selection.Name
	return ResolvedPreset{Name: &name, Origin: origin}, nil
}

func applyLayer(resolution *Resolution, values partial, origin Origin, selected ResolvedPreset) error {
	if selected.Name != nil && sameOrigin(selected.Origin, origin) {
		if err := applyNamedPreset(resolution, *selected.Name, origin); err != nil {
			return err
		}
	}
	return applyPartial(resolution, values, origin)
}

func applyCLILayer(resolution *Resolution, overrides Overrides, selected ResolvedPreset) error {
	origin := Origin{Source: SourceCLI}
	if selected.Name != nil && sameOrigin(selected.Origin, origin) {
		if err := applyNamedPreset(resolution, *selected.Name, origin); err != nil {
			return err
		}
	}
	return applyOverrides(resolution, overrides)
}

func applyNamedPreset(resolution *Resolution, name string, origin Origin) error {
	definition, ok := LookupPreset(name)
	if !ok {
		return invalidf("unsupported preset %q (expected %s)", name, joinExpected(PresetNames()))
	}
	origin.Preset = name
	return applyPartial(resolution, presetPartial(definition), origin)
}

func sameOrigin(left, right Origin) bool {
	return left.Source == right.Source && left.Path == right.Path
}

func applyPartial(resolution *Resolution, values partial, origin Origin) error {
	applyDepth(resolution, values.Depth, origin)
	applyOptional(&resolution.Effective.DirectoriesOnly, "dirsOnly", values.DirectoriesOnly, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.IncludeHidden, "hidden", values.IncludeHidden, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.Format, "format", values.Format, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.Style, "style", values.Style, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.UseDefaultIgnores, "useDefaultIgnores", values.UseDefaultIgnores, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.UseGitIgnore, "useGitignore", values.UseGitIgnore, origin, resolution.Provenance)
	appendIgnores(resolution, values.IgnorePatterns, origin)
	return validateEffective(*resolution)
}

func applyOverrides(resolution *Resolution, overrides Overrides) error {
	origin := Origin{Source: SourceCLI}
	applyDepth(resolution, overrides.Depth, origin)
	applyOptional(&resolution.Effective.DirectoriesOnly, "dirsOnly", overrides.DirectoriesOnly, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.IncludeHidden, "hidden", overrides.IncludeHidden, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.Format, "format", overrides.Format, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.Style, "style", overrides.Style, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.UseDefaultIgnores, "useDefaultIgnores", overrides.UseDefaultIgnores, origin, resolution.Provenance)
	applyOptional(&resolution.Effective.UseGitIgnore, "useGitignore", overrides.UseGitIgnore, origin, resolution.Provenance)
	appendIgnores(resolution, overrides.IgnorePatterns, origin)
	return validateEffective(*resolution)
}

func applyDepth(resolution *Resolution, value DepthOverride, origin Origin) {
	if !value.Set {
		return
	}
	if value.Unlimited {
		resolution.Effective.MaxDepth = nil
	} else {
		depth := value.Value
		resolution.Effective.MaxDepth = &depth
	}
	resolution.Provenance["depth"] = origin
}

func applyOptional[T any](target *T, field string, value Optional[T], origin Origin, provenance map[string]Origin) {
	if !value.Set {
		return
	}
	*target = value.Value
	provenance[field] = origin
}

func appendIgnores(resolution *Resolution, patterns []string, origin Origin) {
	seen := make(map[string]struct{}, len(resolution.Ignores)+len(patterns))
	for _, rule := range resolution.Ignores {
		seen[rule.Pattern] = struct{}{}
	}
	for _, pattern := range patterns {
		if _, exists := seen[pattern]; exists {
			continue
		}
		seen[pattern] = struct{}{}
		resolution.Ignores = append(resolution.Ignores, IgnoreRule{Pattern: pattern, Origin: origin})
		resolution.Effective.IgnorePatterns = append(resolution.Effective.IgnorePatterns, pattern)
	}
}

func validateEffective(resolution Resolution) error {
	if resolution.Effective.MaxDepth != nil && *resolution.Effective.MaxDepth < 0 {
		return invalidf("depth must be a non-negative integer or unlimited")
	}
	switch resolution.Effective.Format {
	case FormatText, FormatMarkdown, FormatJSON:
	default:
		return invalidf("unsupported format %q (expected text, markdown, or json)", resolution.Effective.Format)
	}
	switch resolution.Effective.Style {
	case StyleUnicode, StyleASCII:
	default:
		return invalidf("unsupported style %q (expected unicode or ascii)", resolution.Effective.Style)
	}
	if _, err := filter.NewIgnoreMatcher(resolution.Effective.IgnorePatterns); err != nil {
		return invalidf("%v", err)
	}
	return nil
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
