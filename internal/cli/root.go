// Package cli implements Dirloom's non-interactive command-line interface.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dirloom/dirloom/internal/app"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/output"
	"github.com/dirloom/dirloom/internal/render"
	"github.com/spf13/cobra"
)

type usageError struct {
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// Execute runs the CLI with explicit streams and returns a stable process exit
// code: 0 success, 1 runtime error, 2 invalid arguments.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer, version string) int {
	return executeWithLoader(ctx, args, stdout, stderr, version, configuration.NewLoader())
}

func executeWithLoader(ctx context.Context, args []string, stdout, stderr io.Writer, version string, loader *configuration.Loader) int {
	command := newRootCommand(stdout, stderr, version, loader)
	command.SetArgs(args)
	if err := command.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %s\n", err)
		var invalid *usageError
		if errors.As(err, &invalid) {
			return 2
		}
		return 1
	}
	return 0
}

// NewRootCommand constructs the root Cobra command without process globals.
func NewRootCommand(stdout, stderr io.Writer, version string) *cobra.Command {
	return newRootCommand(stdout, stderr, version, configuration.NewLoader())
}

func newRootCommand(stdout, stderr io.Writer, version string, loader *configuration.Loader) *cobra.Command {
	var opts options
	var sources sourceOptions
	command := &cobra.Command{
		Use:   "dirloom [directory]",
		Short: "Create clean, deterministic project trees",
		Long: "Dirloom turns a directory into a clean, deterministic and shareable\n" +
			"tree for terminals, documentation, CI pipelines, humans and tools.\n\n" +
			"Arguments:\n  directory   Directory to inspect (default: current directory)",
		Example: `  dirloom
  dirloom ./src
  dirloom --depth 3
  dirloom --preset docs
  dirloom --dirs-only
  dirloom --style ascii
  dirloom --format markdown
  dirloom --format markdown-tree
  dirloom --ignore node_modules --ignore dist
  dirloom --output structure.md --format markdown`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			resolved, overrides, err := resolveOptions(cmd, loader, root, sources, &opts)
			if err != nil {
				return err
			}
			if styleIsInactive(resolved.Effective.Format) && overrides.Style.Set {
				return &usageError{err: fmt.Errorf("--style cannot be used with --format %s", resolved.Effective.Format)}
			}

			model, err := app.Inspect(cmd.Context(), app.InspectRequest{
				Root:              resolved.Root,
				MaxDepth:          resolved.Effective.MaxDepth,
				DirectoriesOnly:   resolved.Effective.DirectoriesOnly,
				IncludeHidden:     resolved.Effective.IncludeHidden,
				IgnorePatterns:    resolved.Effective.IgnorePatterns,
				UseDefaultIgnores: resolved.Effective.UseDefaultIgnores,
				UseGitIgnore:      resolved.Effective.UseGitIgnore,
				OutputPath:        opts.output,
			})
			if err != nil {
				return err
			}

			renderer, err := render.New(resolved.Effective.Format, resolved.Effective.Style)
			if err != nil {
				return err
			}
			var rendered bytes.Buffer
			if err := renderer.Render(&rendered, model); err != nil {
				return fmt.Errorf("render tree: %w", err)
			}

			if opts.output != "" {
				return output.WriteFile(opts.output, rendered.Bytes())
			}
			if err := writeAll(stdout, rendered.Bytes()); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}

	command.SetOut(stdout)
	command.SetErr(stderr)
	command.CompletionOptions.DisableDefaultCmd = true
	command.SetVersionTemplate("dirloom {{.Version}}\n")
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	command.PersistentFlags().StringVar(&sources.path, "config", "", "use a project configuration file instead of automatic discovery")
	command.PersistentFlags().BoolVar(&sources.noUser, "no-user-config", false, "do not load the user configuration file")
	command.PersistentFlags().BoolVar(&sources.noConfig, "no-config", false, "do not load user or project configuration files")
	bindInspectFlags(command, &opts)
	command.Flags().StringVarP(&opts.output, "output", "o", "", "write transactionally to a file instead of stdout")
	command.AddCommand(newConfigCommand(stdout, loader, &sources))
	command.AddCommand(newPresetCommand(stdout, &sources))

	return command
}

func bindInspectFlags(command *cobra.Command, opts *options) {
	command.Flags().StringVar(&opts.preset, "preset", "", "built-in preset: ai, compact, docs, monorepo, or none")
	command.Flags().VarP(&opts.depth, "depth", "d", "maximum depth (0 shows only the root; unlimited removes the limit)")
	command.Flags().BoolVar(&opts.directoriesOnly, "dirs-only", false, "show directories only")
	command.Flags().BoolVar(&opts.includeHidden, "hidden", false, "include hidden entries that survive other filters")
	command.Flags().StringArrayVar(&opts.ignorePatterns, "ignore", nil, "exclude a pattern (repeatable)")
	command.Flags().BoolVar(&opts.noDefaultIgnore, "no-default-ignore", false, "disable built-in directory exclusions")
	command.Flags().BoolVar(&opts.noGitIgnore, "no-gitignore", false, "do not apply .gitignore files")
	command.Flags().StringVar(&opts.format, "format", "", "output format: text, markdown, markdown-tree, or json")
	command.Flags().StringVar(&opts.style, "style", "", "tree style: unicode or ascii")
}

func resolveOptions(command *cobra.Command, loader *configuration.Loader, root string, sources sourceOptions, opts *options) (configuration.Resolution, configuration.Overrides, error) {
	if command.Flags().Changed("config") && sources.path == "" {
		return configuration.Resolution{}, configuration.Overrides{}, &usageError{err: fmt.Errorf("--config requires a non-empty path")}
	}
	overrides, err := explicitOverrides(command, opts)
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

func explicitOverrides(command *cobra.Command, opts *options) (configuration.Overrides, error) {
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
	if command.Flags().Changed("format") {
		result.Format = configuration.Optional[string]{Set: true, Value: opts.format}
	}
	if command.Flags().Changed("style") {
		result.Style = configuration.Optional[string]{Set: true, Value: opts.style}
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
	return result, nil
}

func newConfigCommand(stdout io.Writer, loader *configuration.Loader, sources *sourceOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Inspect Dirloom configuration",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &usageError{err: fmt.Errorf("expected a config subcommand, received %d argument(s)", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	var opts options
	var outputFormat string
	explain := &cobra.Command{
		Use:   "explain [directory]",
		Short: "Explain effective values and their sources",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			resolved, overrides, err := resolveOptions(cmd, loader, root, *sources, &opts)
			if err != nil {
				return err
			}
			if styleIsInactive(resolved.Effective.Format) && overrides.Style.Set {
				return &usageError{err: fmt.Errorf("--style cannot be used with --format %s", resolved.Effective.Format)}
			}
			switch outputFormat {
			case "text":
				if err := resolved.WriteText(stdout); err != nil {
					return fmt.Errorf("write configuration explanation: %w", err)
				}
			case "json":
				if err := resolved.WriteJSON(stdout); err != nil {
					return fmt.Errorf("write configuration explanation: %w", err)
				}
			default:
				return &usageError{err: fmt.Errorf("unsupported explanation format %q (expected text or json)", outputFormat)}
			}
			return nil
		},
	}
	bindInspectFlags(explain, &opts)
	explain.Flags().StringVar(&outputFormat, "as", "text", "explanation format: text or json")
	command.AddCommand(explain)
	return command
}

func styleIsInactive(format string) bool {
	return format == render.FormatJSON || format == render.FormatMarkdownTree
}

func newPresetCommand(stdout io.Writer, sources *sourceOptions) *cobra.Command {
	names := strings.Join(configuration.PresetNames(), ", ")
	command := &cobra.Command{
		Use:   "preset",
		Short: "Inspect built-in presets",
		Long:  "Inspect Dirloom's built-in presets. Available presets: " + names + ".",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &usageError{err: fmt.Errorf("expected a preset subcommand, received %d argument(s)", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectPresetSourceFlags(cmd, sources); err != nil {
				return err
			}
			return cmd.Help()
		},
	}
	var outputFormat string
	explain := &cobra.Command{
		Use:   "explain <preset>",
		Short: "Explain one built-in preset",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{err: fmt.Errorf("expected exactly one preset name, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectPresetSourceFlags(cmd, sources); err != nil {
				return err
			}
			definition, ok := configuration.LookupPreset(args[0])
			if !ok {
				return &usageError{err: fmt.Errorf("unsupported preset %q (expected %s)", args[0], names)}
			}
			switch outputFormat {
			case "text":
				if err := definition.WriteText(stdout); err != nil {
					return fmt.Errorf("write preset explanation: %w", err)
				}
			case "json":
				if err := definition.WriteJSON(stdout); err != nil {
					return fmt.Errorf("write preset explanation: %w", err)
				}
			default:
				return &usageError{err: fmt.Errorf("unsupported explanation format %q (expected text or json)", outputFormat)}
			}
			return nil
		},
	}
	explain.Flags().StringVar(&outputFormat, "as", "text", "explanation format: text or json")
	command.AddCommand(explain)
	return command
}

func rejectPresetSourceFlags(command *cobra.Command, sources *sourceOptions) error {
	checks := []struct {
		name    string
		changed bool
	}{
		{name: "config", changed: command.Flags().Changed("config") || sources.path != ""},
		{name: "no-user-config", changed: command.Flags().Changed("no-user-config") || sources.noUser},
		{name: "no-config", changed: command.Flags().Changed("no-config") || sources.noConfig},
	}
	for _, check := range checks {
		if check.changed {
			return &usageError{err: fmt.Errorf("--%s cannot be used with preset inspection", check.name)}
		}
	}
	return nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
