package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveLayersAndProvenance(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	userBase := t.TempDir()
	writeConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), `schemaVersion: 1
defaults:
  depth: 8
  dirsOnly: true
  hidden: true
  format: markdown
  style: ascii
filters:
  useDefaultIgnores: false
  useGitignore: false
ignore:
  - user-cache/**
  - shared/**
`)
	writeConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
defaults:
  depth: null
  hidden: false
  format: json
filters:
  useGitignore: true
ignore:
  - shared/**
  - generated/**
`)
	loader := NewLoader(WithUserConfigDir(func() (string, error) { return userBase, nil }))
	resolved, err := loader.Resolve(ResolveOptions{
		Root: root,
		Overrides: Overrides{
			Depth:             DepthOverride{Set: true, Value: 3},
			DirectoriesOnly:   Optional[bool]{Set: true, Value: false},
			UseDefaultIgnores: Optional[bool]{Set: true, Value: true},
			IgnorePatterns:    []string{"generated/**", "cli-only/**"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Effective.MaxDepth == nil || *resolved.Effective.MaxDepth != 3 {
		t.Fatalf("depth = %#v", resolved.Effective.MaxDepth)
	}
	if resolved.Effective.DirectoriesOnly || resolved.Effective.IncludeHidden {
		t.Fatalf("booleans = dirsOnly:%t hidden:%t", resolved.Effective.DirectoriesOnly, resolved.Effective.IncludeHidden)
	}
	if resolved.Effective.Format != FormatJSON || resolved.Effective.Style != StyleASCII {
		t.Fatalf("format/style = %q/%q", resolved.Effective.Format, resolved.Effective.Style)
	}
	if !resolved.Effective.UseDefaultIgnores || !resolved.Effective.UseGitIgnore {
		t.Fatalf("filters = defaults:%t git:%t", resolved.Effective.UseDefaultIgnores, resolved.Effective.UseGitIgnore)
	}
	wantIgnores := []string{"user-cache/**", "shared/**", "generated/**", "cli-only/**"}
	if !reflect.DeepEqual(resolved.Effective.IgnorePatterns, wantIgnores) {
		t.Fatalf("ignore = %#v, want %#v", resolved.Effective.IgnorePatterns, wantIgnores)
	}
	if resolved.Provenance["depth"].Source != SourceCLI || resolved.Provenance["hidden"].Source != SourceProject || resolved.Provenance["style"].Source != SourceUser {
		t.Fatalf("provenance = %#v", resolved.Provenance)
	}
	if len(resolved.Ignores) != 4 || resolved.Ignores[1].Origin.Source != SourceUser || resolved.Ignores[2].Origin.Source != SourceProject || resolved.Ignores[3].Origin.Source != SourceCLI {
		t.Fatalf("ignore origins = %#v", resolved.Ignores)
	}
}

func TestResolvePresentationPrecedenceResetAndPresetIndependence(t *testing.T) {
	root := t.TempDir()
	userBase := t.TempDir()
	writeConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), `schemaVersion: 1
presentation:
  color: always
  icons: nerd
  theme: midnight
`)
	writeConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
preset: ai
presentation:
  color: never
  theme: null
`)
	loader := NewLoader(WithUserConfigDir(func() (string, error) { return userBase, nil }))
	resolved, err := loader.Resolve(ResolveOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Effective.Color != "never" || resolved.Effective.Icons != "nerd" || resolved.Effective.Theme != "default" {
		t.Fatalf("effective presentation = %#v", resolved.Effective)
	}
	if resolved.Provenance["color"].Source != SourceProject || resolved.Provenance["icons"].Source != SourceUser || resolved.Provenance["theme"].Source != SourceProject {
		t.Fatalf("presentation provenance = %#v", resolved.Provenance)
	}
	if resolved.Provenance["color"].Preset != "" || resolved.Provenance["icons"].Preset != "" || resolved.Provenance["theme"].Preset != "" {
		t.Fatal("preset unexpectedly defined presentation")
	}

	resolved, err = loader.Resolve(ResolveOptions{Root: root, Overrides: Overrides{
		Color: Optional[string]{Set: true, Value: "auto"},
		Theme: ThemeSelection{Set: true, Value: "daylight"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Effective.Color != "auto" || resolved.Effective.Icons != "nerd" || resolved.Effective.Theme != "daylight" || resolved.Provenance["theme"].Source != SourceCLI {
		t.Fatalf("CLI presentation = %#v provenance=%#v", resolved.Effective, resolved.Provenance)
	}

	withoutConfig, err := loader.Resolve(ResolveOptions{Root: root, DisableAll: true, Overrides: Overrides{Preset: PresetSelection{Set: true, Name: "ai"}}})
	if err != nil {
		t.Fatal(err)
	}
	if withoutConfig.Effective.Color != "auto" || withoutConfig.Effective.Icons != "never" || withoutConfig.Effective.Theme != "default" {
		t.Fatalf("preset changed presentation = %#v", withoutConfig.Effective)
	}
}

func TestResolvePresetPrecedenceAndExplicitOverrides(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	userBase := t.TempDir()
	writeConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), `schemaVersion: 1
preset: compact
defaults:
  hidden: true
ignore:
  - user/**
`)
	writeConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
preset: ai
defaults:
  format: text
ignore:
  - project/**
`)
	loader := NewLoader(WithUserConfigDir(func() (string, error) { return userBase, nil }))
	resolved, err := loader.Resolve(ResolveOptions{
		Root: root,
		Overrides: Overrides{
			Preset:         PresetSelection{Set: true, Name: "docs"},
			Depth:          DepthOverride{Set: true, Value: 6},
			IgnorePatterns: []string{"cli/**"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Preset.Name == nil || *resolved.Preset.Name != "docs" || resolved.Preset.Origin.Source != SourceCLI {
		t.Fatalf("preset = %#v", resolved.Preset)
	}
	if resolved.Effective.MaxDepth == nil || *resolved.Effective.MaxDepth != 6 || resolved.Effective.DirectoriesOnly || resolved.Effective.IncludeHidden || resolved.Effective.Format != FormatMarkdown {
		t.Fatalf("effective = %#v", resolved.Effective)
	}
	if got := resolved.Provenance["format"]; got.Source != SourceCLI || got.Preset != "docs" {
		t.Fatalf("format origin = %#v", got)
	}
	if got := resolved.Provenance["depth"]; got.Source != SourceCLI || got.Preset != "" {
		t.Fatalf("depth origin = %#v", got)
	}
	wantIgnores := []string{"user/**", "project/**", "cli/**"}
	if !reflect.DeepEqual(resolved.Effective.IgnorePatterns, wantIgnores) {
		t.Fatalf("ignore = %#v, want %#v", resolved.Effective.IgnorePatterns, wantIgnores)
	}
}

func TestResolvePresetResetAndAdditiveIgnores(t *testing.T) {
	t.Run("project reset preserves explicit values", func(t *testing.T) {
		root := t.TempDir()
		userBase := t.TempDir()
		writeConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), `schemaVersion: 1
preset: compact
defaults:
  hidden: true
ignore:
  - user/**
`)
		writeConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
preset: null
defaults:
  depth: 2
`)
		resolved, err := NewLoader(WithUserConfigDir(func() (string, error) { return userBase, nil })).Resolve(ResolveOptions{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Preset.Name != nil || resolved.Preset.Origin.Source != SourceProject {
			t.Fatalf("preset = %#v", resolved.Preset)
		}
		if resolved.Effective.DirectoriesOnly || !resolved.Effective.IncludeHidden || resolved.Effective.MaxDepth == nil || *resolved.Effective.MaxDepth != 2 {
			t.Fatalf("effective = %#v", resolved.Effective)
		}
		if !reflect.DeepEqual(resolved.Effective.IgnorePatterns, []string{"user/**"}) {
			t.Fatalf("ignore = %#v", resolved.Effective.IgnorePatterns)
		}
	})

	t.Run("selected preset contributes before explicit layer ignores", func(t *testing.T) {
		root := t.TempDir()
		userBase := t.TempDir()
		writeConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), `schemaVersion: 1
ignore:
  - "**/dist"
  - user/**
`)
		writeConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
preset: ai
defaults:
  format: text
ignore:
  - "**/dist"
  - project/**
`)
		resolved, err := NewLoader(WithUserConfigDir(func() (string, error) { return userBase, nil })).Resolve(ResolveOptions{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"**/dist", "user/**", "**/build", "*.map", "project/**"}
		if !reflect.DeepEqual(resolved.Effective.IgnorePatterns, want) {
			t.Fatalf("ignore = %#v, want %#v", resolved.Effective.IgnorePatterns, want)
		}
		if resolved.Ignores[0].Origin.Source != SourceUser || resolved.Ignores[2].Origin.Preset != "ai" || resolved.Ignores[4].Origin.Preset != "" {
			t.Fatalf("ignore origins = %#v", resolved.Ignores)
		}
		if got := resolved.Provenance["style"]; got.Source != SourceProject || got.Preset != "ai" {
			t.Fatalf("style origin = %#v", got)
		}
		if got := resolved.Provenance["format"]; got.Source != SourceProject || got.Preset != "" {
			t.Fatalf("format origin = %#v", got)
		}
	})

	t.Run("CLI reset skips configured preset", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\npreset: ai\ndefaults:\n  depth: 7\n")
		resolved, err := loaderWithoutUserConfig().Resolve(ResolveOptions{
			Root:      root,
			Overrides: Overrides{Preset: PresetSelection{Set: true, Disabled: true}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Preset.Name != nil || resolved.Preset.Origin.Source != SourceCLI || resolved.Effective.Format != FormatText || resolved.Effective.MaxDepth == nil || *resolved.Effective.MaxDepth != 7 || len(resolved.Ignores) != 0 {
			t.Fatalf("resolution = %#v", resolved)
		}
	})

	t.Run("CLI preset remains active with no config", func(t *testing.T) {
		resolved, err := loaderWithoutUserConfig().Resolve(ResolveOptions{
			Root:       t.TempDir(),
			DisableAll: true,
			Overrides:  Overrides{Preset: PresetSelection{Set: true, Name: "ai"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Preset.Name == nil || *resolved.Preset.Name != "ai" || resolved.Effective.Format != FormatMarkdown || len(resolved.Ignores) != 3 {
			t.Fatalf("resolution = %#v", resolved)
		}
	})
}

func TestResolveProjectDiscoveryStopsAtNearestConfigWithinGitBoundary(t *testing.T) {
	repository := t.TempDir()
	if err := os.Mkdir(filepath.Join(repository, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, filepath.Join(repository, ".dirloom.yaml"), "schemaVersion: 1\ndefaults:\n  depth: 9\n")
	application := filepath.Join(repository, "packages", "app")
	root := filepath.Join(application, "src")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, filepath.Join(application, ".dirloom.yaml"), "schemaVersion: 1\ndefaults:\n  depth: 2\n")

	loader := loaderWithoutUserConfig()
	resolved, err := loader.Resolve(ResolveOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Effective.MaxDepth == nil || *resolved.Effective.MaxDepth != 2 {
		t.Fatalf("depth = %#v", resolved.Effective.MaxDepth)
	}
	if got := resolved.Sources[1].Path; got != filepath.Join(application, ".dirloom.yaml") {
		t.Fatalf("project path = %q", got)
	}
}

func TestResolveRecognizesGitFileBoundary(t *testing.T) {
	repository := t.TempDir()
	if err := os.WriteFile(filepath.Join(repository, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, filepath.Join(repository, ".dirloom.yaml"), "schemaVersion: 1\ndefaults:\n  hidden: true\n")
	root := filepath.Join(repository, "nested")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Effective.IncludeHidden {
		t.Fatal("project config at a .git file boundary was not loaded")
	}
}

func TestResolveOutsideGitOnlyChecksInspectedRoot(t *testing.T) {
	parent := t.TempDir()
	writeConfig(t, filepath.Join(parent, ".dirloom.yaml"), "schemaVersion: 1\ndefaults:\n  hidden: true\n")
	root := filepath.Join(parent, "child")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Effective.IncludeHidden || resolved.Sources[1].Status != StatusMissing || resolved.Sources[1].Path != filepath.Join(root, ".dirloom.yaml") {
		t.Fatalf("resolution = %#v", resolved)
	}
}

func TestResolveExplicitProjectPathAndSourceControls(t *testing.T) {
	workingDirectory := t.TempDir()
	root := t.TempDir()
	writeConfig(t, filepath.Join(workingDirectory, "ci.yaml"), "schemaVersion: 1\ndefaults:\n  depth: 4\n")
	loader := NewLoader(
		WithUserConfigDir(func() (string, error) { return t.TempDir(), nil }),
		WithWorkingDirectory(func() (string, error) { return workingDirectory, nil }),
	)
	resolved, err := loader.Resolve(ResolveOptions{Root: root, ExplicitProjectPath: "ci.yaml", DisableUser: true})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Sources[0].Status != StatusDisabled || resolved.Sources[1].Status != StatusLoaded || resolved.Effective.MaxDepth == nil || *resolved.Effective.MaxDepth != 4 {
		t.Fatalf("resolution = %#v", resolved)
	}

	_, err = loader.Resolve(ResolveOptions{Root: root, ExplicitProjectPath: "missing.yaml"})
	if err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing explicit config error = %v", err)
	}

	_, err = loader.Resolve(ResolveOptions{Root: root, ExplicitProjectPath: "ci.yaml", DisableAll: true})
	if err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("conflicting flags error = %v", err)
	}
}

func TestResolveDisableAllStillAppliesCLI(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 99\n")
	resolved, err := loaderWithoutUserConfig().Resolve(ResolveOptions{
		Root:       root,
		DisableAll: true,
		Overrides: Overrides{
			Depth:          DepthOverride{Set: true, Unlimited: true},
			IncludeHidden:  Optional[bool]{Set: true, Value: true},
			IgnorePatterns: []string{"tmp/**"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Effective.MaxDepth != nil || !resolved.Effective.IncludeHidden || resolved.Provenance["depth"].Source != SourceCLI {
		t.Fatalf("effective = %#v", resolved.Effective)
	}
	if len(resolved.Sources) != 2 || resolved.Sources[0].Status != StatusDisabled || resolved.Sources[1].Status != StatusDisabled {
		t.Fatalf("sources = %#v", resolved.Sources)
	}
}

func TestResolveClassifiesReadFailuresAsRuntimeErrors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".dirloom.yaml")
	loader := loaderWithoutUserConfig()
	loader.fs = permissionFileSystem{fileSystem: osFileSystem{}, deniedPath: path}
	_, err := loader.Resolve(ResolveOptions{Root: root})
	if err == nil || IsInvalid(err) || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("permission error = %v", err)
	}
}

func TestResolveUserDirectoryUnavailableIsNonFatal(t *testing.T) {
	loader := NewLoader(WithUserConfigDir(func() (string, error) { return "", errors.New("HOME is unset") }))
	resolved, err := loader.Resolve(ResolveOptions{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Sources[0].Status != StatusUnavailable {
		t.Fatalf("user source = %#v", resolved.Sources[0])
	}
}

func TestResolveRejectsOversizedConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".dirloom.yaml")
	if err := os.WriteFile(path, []byte("schemaVersion: 1\n#"+strings.Repeat("x", maxConfigSize)), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root})
	if err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("oversized error = %v", err)
	}
}

func TestResolveRejectsInvalidCLIValues(t *testing.T) {
	root := t.TempDir()
	tests := []Overrides{
		{Preset: PresetSelection{Set: true, Name: "unknown"}},
		{Preset: PresetSelection{Set: true}},
		{Preset: PresetSelection{Set: true, Disabled: true, Name: "ai"}},
		{Format: Optional[string]{Set: true, Value: "yaml"}},
		{Style: Optional[string]{Set: true, Value: "auto"}},
		{Depth: DepthOverride{Set: true, Value: -1}},
		{IgnorePatterns: []string{"../outside"}},
	}
	for _, overrides := range tests {
		if _, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root, Overrides: overrides}); err == nil || !IsInvalid(err) {
			t.Fatalf("Resolve(%#v) error = %v", overrides, err)
		}
	}
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loaderWithoutUserConfig() *Loader {
	return NewLoader(WithUserConfigDir(func() (string, error) { return "", errors.New("disabled in test") }))
}

type permissionFileSystem struct {
	fileSystem
	deniedPath string
}

func (fileSystem permissionFileSystem) Stat(path string) (os.FileInfo, error) {
	if path == fileSystem.deniedPath {
		return nil, os.ErrPermission
	}
	return fileSystem.fileSystem.Stat(path)
}
