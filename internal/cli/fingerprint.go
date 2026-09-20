package cli

import (
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/artifact"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/spf13/cobra"
)

type fingerprintOptions struct {
	root            string
	format          string
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

func newFingerprintCommand(stdout io.Writer, loader *configuration.Loader, sources *sourceOptions) *cobra.Command {
	var opts fingerprintOptions
	command := &cobra.Command{
		Use:   "fingerprint [directory]",
		Short: "Fingerprint the structural view produced by Dirloom, not file contents",
		Long: "Compute a deterministic fingerprint of the structural view Dirloom observes\n" +
			"after applying the active filters. The fingerprint does not hash file contents.\n" +
			"Ignore rules, depth, hidden visibility and directories-only change that view\n" +
			"and therefore the fingerprint. Presentation (theme, color, icons, TTY) does not.",
		Example: `  dirloom fingerprint
  dirloom fingerprint --root .
  dirloom fingerprint --format json
  dirloom fingerprint ./src --depth 3`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args))}
			}
			return nil
		},
		ValidArgsFunction: completeInspectRoot,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := fingerprintRoot(cmd, args, opts.root)
			if err != nil {
				return err
			}
			if opts.format != "text" && opts.format != "json" {
				return &usageError{err: fmt.Errorf("unsupported fingerprint format %q (expected text or json)", opts.format)}
			}
			resolved, _, err := resolveFingerprintOptions(cmd, loader, root, *sources, &opts)
			if err != nil {
				return err
			}
			result, err := app.Fingerprint(cmd.Context(), app.InspectRequest{
				Root:              resolved.Root,
				MaxDepth:          resolved.Effective.MaxDepth,
				DirectoriesOnly:   resolved.Effective.DirectoriesOnly,
				IncludeHidden:     resolved.Effective.IncludeHidden,
				IgnorePatterns:    resolved.Effective.IgnorePatterns,
				UseDefaultIgnores: resolved.Effective.UseDefaultIgnores,
				UseGitIgnore:      resolved.Effective.UseGitIgnore,
			})
			if err != nil {
				return classifyFingerprintError(err)
			}
			if opts.format == "json" {
				document := identity.NewJSONDocument(result.Fingerprint, result.NodeCount)
				if err := document.WriteJSON(stdout); err != nil {
					return fmt.Errorf("write fingerprint JSON: %w", err)
				}
				return nil
			}
			if err := identity.WriteText(stdout, result.Fingerprint); err != nil {
				return fmt.Errorf("write fingerprint: %w", err)
			}
			return nil
		},
	}
	command.Flags().StringVar(&opts.root, "root", "", "directory to fingerprint (default: positional argument or current directory)")
	command.Flags().StringVar(&opts.format, "format", "text", "output format: text or json")
	command.Flags().StringVar(&opts.preset, "preset", "", "built-in preset: ai, compact, docs, monorepo, or none")
	command.Flags().VarP(&opts.depth, "depth", "d", "maximum depth (0 shows only the root; unlimited removes the limit)")
	command.Flags().BoolVar(&opts.directoriesOnly, "dirs-only", false, "include directories only")
	command.Flags().BoolVar(&opts.includeHidden, "hidden", false, "include hidden entries that survive other filters")
	command.Flags().StringArrayVar(&opts.ignorePatterns, "ignore", nil, "exclude a pattern (repeatable)")
	command.Flags().BoolVar(&opts.noDefaultIgnore, "no-default-ignore", false, "disable built-in directory exclusions")
	command.Flags().BoolVar(&opts.noGitIgnore, "no-gitignore", false, "do not apply .gitignore files")
	command.Flags().StringVar(&opts.color, "color", "", "accepted and ignored: fingerprint identity is presentation-independent")
	command.Flags().StringVar(&opts.icons, "icons", "", "accepted and ignored: fingerprint identity is presentation-independent")
	command.Flags().StringVar(&opts.theme, "theme", "", "accepted and ignored: fingerprint identity is presentation-independent")
	registerFingerprintCompletions(command)
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

func fingerprintRoot(command *cobra.Command, args []string, flag string) (string, error) {
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

func resolveFingerprintOptions(command *cobra.Command, loader *configuration.Loader, root string, sources sourceOptions, opts *fingerprintOptions) (configuration.Resolution, configuration.Overrides, error) {
	if command.Flags().Changed("config") && sources.path == "" {
		return configuration.Resolution{}, configuration.Overrides{}, &usageError{err: fmt.Errorf("--config requires a non-empty path")}
	}
	overrides, err := fingerprintOverrides(command, opts)
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

func fingerprintOverrides(command *cobra.Command, opts *fingerprintOptions) (configuration.Overrides, error) {
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

func registerFingerprintCompletions(command *cobra.Command) {
	fixed := func(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp)
	}
	_ = command.RegisterFlagCompletionFunc("format", fixed([]string{"text", "json"}))
	_ = command.RegisterFlagCompletionFunc("preset", fixed(append(append([]string{}, configuration.PresetNames()...), configuration.PresetNone)))
	_ = command.RegisterFlagCompletionFunc("depth", fixed([]string{"unlimited"}))
	_ = command.RegisterFlagCompletionFunc("color", fixed(presentation.ColorModes()))
	_ = command.RegisterFlagCompletionFunc("icons", fixed(presentation.IconModes()))
	_ = command.RegisterFlagCompletionFunc("theme", completeTheme)
	_ = command.RegisterFlagCompletionFunc("root", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
}

func classifyFingerprintError(err error) error {
	if configuration.IsInvalid(err) {
		return &usageError{err: err}
	}
	if artifact.IsInternal(err) {
		return fmt.Errorf("internal error: %w", err)
	}
	return err
}
