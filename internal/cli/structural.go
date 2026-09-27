package cli

import (
	"fmt"

	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/spf13/cobra"
)

// structuralFlagState holds structural CLI overrides shared by fingerprint and
// snapshot without depending on presentation initialization for identity.
type structuralFlagState struct {
	preset          string
	depth           optionalDepth
	directoriesOnly bool
	includeHidden   bool
	ignorePatterns  []string
	noDefaultIgnore bool
	noGitIgnore     bool
	color           string
	icons           string
	theme           string
}

func structuralRoot(command *cobra.Command, args []string, flag string) (string, error) {
	flagSet := command.Flags().Changed("root")
	if flagSet && len(args) == 1 {
		return "", &usageError{err: fmt.Errorf("--root and a positional directory cannot be used together")}
	}
	if flagSet {
		if flag == "" {
			return "", &usageError{err: fmt.Errorf("--root requires a non-empty path")}
		}
		return flag, nil
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return ".", nil
}

func resolveStructuralOptions(command *cobra.Command, loader *configuration.Loader, root string, sources sourceOptions, opts structuralFlagState) (configuration.Resolution, configuration.Overrides, error) {
	if command.Flags().Changed("config") && sources.path == "" {
		return configuration.Resolution{}, configuration.Overrides{}, &usageError{err: fmt.Errorf("--config requires a non-empty path")}
	}
	overrides, err := structuralOverrides(command, opts)
	if err != nil {
		return configuration.Resolution{}, configuration.Overrides{}, err
	}
	resolved, err := loader.Resolve(configuration.ResolveOptions{
		Root:                root,
		ExplicitProjectPath: sources.path,
		DisableUser:         sources.noUser,
		DisableAll:          sources.noConfig,
		Overrides:           overrides,
	})
	if err != nil {
		if configuration.IsInvalid(err) {
			return configuration.Resolution{}, configuration.Overrides{}, &usageError{err: err}
		}
		return configuration.Resolution{}, configuration.Overrides{}, err
	}
	return resolved, overrides, nil
}

func structuralOverrides(command *cobra.Command, opts structuralFlagState) (configuration.Overrides, error) {
	result := configuration.Overrides{}
	if command.Flags().Changed("preset") {
		if opts.preset == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--preset requires a non-empty value")}
		}
		if opts.preset == configuration.PresetNone {
			result.Preset = configuration.PresetSelection{Set: true, Disabled: true}
		} else {
			result.Preset = configuration.PresetSelection{Set: true, Name: opts.preset}
		}
	}
	if command.Flags().Changed("depth") {
		result.Depth = configuration.DepthOverride{Set: true, Unlimited: opts.depth.unlimited, Value: opts.depth.value}
	}
	if command.Flags().Changed("dirs-only") {
		result.DirectoriesOnly = configuration.Optional[bool]{Set: true, Value: opts.directoriesOnly}
	}
	if command.Flags().Changed("hidden") {
		result.IncludeHidden = configuration.Optional[bool]{Set: true, Value: opts.includeHidden}
	}
	if command.Flags().Changed("no-default-ignore") {
		result.UseDefaultIgnores = configuration.Optional[bool]{Set: true, Value: !opts.noDefaultIgnore}
	}
	if command.Flags().Changed("no-gitignore") {
		result.UseGitIgnore = configuration.Optional[bool]{Set: true, Value: !opts.noGitIgnore}
	}
	if command.Flags().Changed("ignore") {
		result.IgnorePatterns = append([]string(nil), opts.ignorePatterns...)
	}
	if command.Flags().Changed("color") {
		if opts.color == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--color requires a non-empty value")}
		}
		result.Color = configuration.Optional[string]{Set: true, Value: opts.color}
	}
	if command.Flags().Changed("icons") {
		if opts.icons == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--icons requires a non-empty value")}
		}
		result.Icons = configuration.Optional[string]{Set: true, Value: opts.icons}
	}
	if command.Flags().Changed("theme") {
		if opts.theme == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--theme requires a non-empty value")}
		}
		result.Theme = configuration.ThemeSelection{Set: true, Value: opts.theme}
	}
	return result, nil
}
